package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/erik/feeds/internal/ai"
	"github.com/erik/feeds/internal/db"
	"github.com/erik/feeds/internal/models"
	"github.com/erik/feeds/internal/youtube"
	"github.com/erik/feeds/internal/ytdlp"
)

type Server struct {
	db        *db.DB
	ytdlp     *ytdlp.YTDLP
	ai        *ai.Client
	templates *template.Template
	packs     fs.FS
}

func NewServer(database *db.DB, yt *ytdlp.YTDLP, aiClient *ai.Client, templatesFS fs.FS, packsFS fs.FS) (*Server, error) {
	funcMap := template.FuncMap{
		"div": func(a, b int) int { return a / b },
		"mod": func(a, b int) int { return a % b },
		"mul": func(a, b int) int { return a * b },
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		db:        database,
		ytdlp:     yt,
		ai:        aiClient,
		templates: tmpl,
		packs:     packsFS,
	}, nil
}

// htmx helpers

func isHtmxRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func htmxRedirect(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Redirect", url)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Pages
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /import", s.handleImportPage)
	mux.HandleFunc("POST /import", s.handleImport)
	mux.HandleFunc("POST /import/url", s.handleImportURL)
	mux.HandleFunc("POST /import/file", s.handleImportFile)
	mux.HandleFunc("POST /import/organize", s.handleOrganize)
	mux.HandleFunc("POST /import/confirm", s.handleConfirmOrganize)
	mux.HandleFunc("GET /feeds/{id}", s.handleFeedPage)
	mux.HandleFunc("GET /feeds/{id}/export", s.handleExportFeed)
	mux.HandleFunc("POST /feeds/{id}/refresh", s.handleRefreshFeed)
	mux.HandleFunc("POST /feeds/{id}/delete", s.handleDeleteFeed)
	mux.HandleFunc("POST /channels/{id}/delete", s.handleDeleteChannel)
	mux.HandleFunc("POST /channels/{id}/move", s.handleMoveChannel)
	mux.HandleFunc("POST /feeds/{id}/add-channel", s.handleAddChannel)
	mux.HandleFunc("GET /channels/{id}", s.handleChannelPage)
	mux.HandleFunc("GET /download/{id}", s.handleDownload)
	mux.HandleFunc("GET /watch/{id}", s.handleWatchPage)
	mux.HandleFunc("GET /watch/{id}/info", s.handleWatchInfo)
	mux.HandleFunc("GET /all", s.handleAllRecent)
	mux.HandleFunc("GET /history", s.handleHistory)
	mux.HandleFunc("POST /watch/{id}/progress", s.handleUpdateWatchProgress)
	mux.HandleFunc("POST /watch/{id}/mark-watched", s.handleMarkWatched)
	mux.HandleFunc("POST /watch/{id}/mark-unwatched", s.handleMarkUnwatched)
	mux.HandleFunc("POST /open", s.handleOpenVideo)
	mux.HandleFunc("POST /watch/{id}/subscribe", s.handleSubscribeFromWatch)

	// SSE endpoints for htmx
	mux.HandleFunc("GET /feeds/{id}/refresh/stream", s.handleRefreshFeedStream)

	// Packs
	mux.HandleFunc("GET /packs", s.handlePacksList)
	mux.HandleFunc("GET /packs/{name}", s.handlePackFile)
}

// Page handlers

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	feeds, err := s.db.GetFeeds()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title": "Home",
		"Feeds": feeds,
	}
	s.templates.ExecuteTemplate(w, "index", data)
}

func (s *Server) handleAllRecent(w http.ResponseWriter, r *http.Request) {
	videos, err := s.db.GetAllRecentVideos(100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get watch progress for all videos
	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	data := map[string]any{
		"Title":       "Everything",
		"Videos":      videos,
		"ProgressMap": progressMap,
	}
	s.templates.ExecuteTemplate(w, "all", data)
}

func (s *Server) handleImportPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":     "Import Feed",
		"AIEnabled": s.ai != nil,
	}
	s.templates.ExecuteTemplate(w, "import", data)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderImportError(w, "Failed to parse form")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		s.renderImportError(w, "Feed name is required")
		return
	}

	jsonData := r.FormValue("json")
	if jsonData == "" {
		s.renderImportError(w, "JSON data is required")
		return
	}

	var export models.NewPipeExport
	if err := json.Unmarshal([]byte(jsonData), &export); err != nil {
		s.renderImportError(w, "Invalid NewPipe JSON format: "+err.Error())
		return
	}

	// Filter to YouTube only (service_id 0)
	var subs []models.NewPipeSubscription
	for _, sub := range export.Subscriptions {
		if sub.ServiceID == 0 {
			subs = append(subs, sub)
		}
	}

	if len(subs) == 0 {
		s.renderImportError(w, "No YouTube subscriptions found in file")
		return
	}

	// Create feed
	feed, err := s.db.CreateFeed(name)
	if err != nil {
		s.renderImportError(w, "Failed to create feed: "+err.Error())
		return
	}

	// Add channels
	for _, sub := range subs {
		if _, err := s.db.AddChannel(feed.ID, sub.URL, sub.Name); err != nil {
			log.Printf("Failed to add channel %s: %v", sub.URL, err)
		}
	}

	// Redirect to new feed
	http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feed.ID, 10), http.StatusSeeOther)
}

func (s *Server) renderImportError(w http.ResponseWriter, errMsg string) {
	data := map[string]any{
		"Title":     "Import Feed",
		"Error":     errMsg,
		"AIEnabled": s.ai != nil,
	}
	s.templates.ExecuteTemplate(w, "import", data)
}

// handleImportURL imports a feed from a remote URL
func (s *Server) handleImportURL(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderImportError(w, "Failed to parse form")
		return
	}

	feedURL := strings.TrimSpace(r.FormValue("url"))
	if feedURL == "" {
		s.renderImportError(w, "URL is required")
		return
	}

	// Fetch the URL with timeout and size limit
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(feedURL)
	if err != nil {
		s.renderImportError(w, "Failed to fetch URL: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.renderImportError(w, "URL returned status: "+resp.Status)
		return
	}

	// Limit read to 5MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		s.renderImportError(w, "Failed to read response: "+err.Error())
		return
	}

	// Try to parse as Feeds format first
	var feedExport models.FeedExport
	if err := json.Unmarshal(body, &feedExport); err == nil && len(feedExport.Channels) > 0 {
		// Feeds format detected
		tags := ""
		if len(feedExport.Tags) > 0 {
			tags = strings.Join(feedExport.Tags, ", ")
		}

		feed, err := s.db.CreateFeedWithMetadata(feedExport.Name, feedExport.Description, feedExport.Author, tags)
		if err != nil {
			s.renderImportError(w, "Failed to create feed: "+err.Error())
			return
		}

		for _, ch := range feedExport.Channels {
			if _, err := s.db.AddChannel(feed.ID, ch.URL, ch.Name); err != nil {
				log.Printf("Failed to add channel %s: %v", ch.URL, err)
			}
		}

		log.Printf("Imported feed '%s' from URL with %d channels (Feeds format)", feed.Name, len(feedExport.Channels))
		http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feed.ID, 10), http.StatusSeeOther)
		return
	}

	// Try NewPipe format
	var newPipeExport models.NewPipeExport
	if err := json.Unmarshal(body, &newPipeExport); err == nil && len(newPipeExport.Subscriptions) > 0 {
		// Filter to YouTube only (service_id 0)
		var subs []models.NewPipeSubscription
		for _, sub := range newPipeExport.Subscriptions {
			if sub.ServiceID == 0 {
				subs = append(subs, sub)
			}
		}

		if len(subs) == 0 {
			s.renderImportError(w, "No YouTube subscriptions found in file")
			return
		}

		// Use filename from URL or default name
		feedName := "Imported Feed"
		if parts := strings.Split(feedURL, "/"); len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			if strings.HasSuffix(lastPart, ".json") {
				feedName = strings.TrimSuffix(lastPart, ".json")
			}
		}

		feed, err := s.db.CreateFeed(feedName)
		if err != nil {
			s.renderImportError(w, "Failed to create feed: "+err.Error())
			return
		}

		for _, sub := range subs {
			if _, err := s.db.AddChannel(feed.ID, sub.URL, sub.Name); err != nil {
				log.Printf("Failed to add channel %s: %v", sub.URL, err)
			}
		}

		log.Printf("Imported feed '%s' from URL with %d channels (NewPipe format)", feed.Name, len(subs))
		http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feed.ID, 10), http.StatusSeeOther)
		return
	}

	s.renderImportError(w, "Unrecognized format - expected Feeds or NewPipe JSON")
}

// handleImportFile imports a feed from an uploaded file
func (s *Server) handleImportFile(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form with 5MB limit
	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		s.renderImportError(w, "Failed to parse form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.renderImportError(w, "No file uploaded")
		return
	}
	defer file.Close()

	// Read file contents
	body, err := io.ReadAll(io.LimitReader(file, 5*1024*1024))
	if err != nil {
		s.renderImportError(w, "Failed to read file: "+err.Error())
		return
	}

	// Try to parse as Feeds format first
	var feedExport models.FeedExport
	if err := json.Unmarshal(body, &feedExport); err == nil && len(feedExport.Channels) > 0 {
		// Feeds format detected
		tags := ""
		if len(feedExport.Tags) > 0 {
			tags = strings.Join(feedExport.Tags, ", ")
		}

		feed, err := s.db.CreateFeedWithMetadata(feedExport.Name, feedExport.Description, feedExport.Author, tags)
		if err != nil {
			s.renderImportError(w, "Failed to create feed: "+err.Error())
			return
		}

		for _, ch := range feedExport.Channels {
			if _, err := s.db.AddChannel(feed.ID, ch.URL, ch.Name); err != nil {
				log.Printf("Failed to add channel %s: %v", ch.URL, err)
			}
		}

		log.Printf("Imported feed '%s' from file with %d channels (Feeds format)", feed.Name, len(feedExport.Channels))
		http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feed.ID, 10), http.StatusSeeOther)
		return
	}

	// Try NewPipe format
	var newPipeExport models.NewPipeExport
	if err := json.Unmarshal(body, &newPipeExport); err == nil && len(newPipeExport.Subscriptions) > 0 {
		// Filter to YouTube only (service_id 0)
		var subs []models.NewPipeSubscription
		for _, sub := range newPipeExport.Subscriptions {
			if sub.ServiceID == 0 {
				subs = append(subs, sub)
			}
		}

		if len(subs) == 0 {
			s.renderImportError(w, "No YouTube subscriptions found in file")
			return
		}

		// Use filename without extension as feed name
		feedName := strings.TrimSuffix(header.Filename, ".json")
		if feedName == "" {
			feedName = "Imported Feed"
		}

		feed, err := s.db.CreateFeed(feedName)
		if err != nil {
			s.renderImportError(w, "Failed to create feed: "+err.Error())
			return
		}

		for _, sub := range subs {
			if _, err := s.db.AddChannel(feed.ID, sub.URL, sub.Name); err != nil {
				log.Printf("Failed to add channel %s: %v", sub.URL, err)
			}
		}

		log.Printf("Imported feed '%s' from file with %d channels (NewPipe format)", feed.Name, len(subs))
		http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feed.ID, 10), http.StatusSeeOther)
		return
	}

	s.renderImportError(w, "Unrecognized format - expected Feeds or NewPipe JSON")
}

func (s *Server) handleFeedPage(w http.ResponseWriter, r *http.Request) {
	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	feed, err := s.db.GetFeed(feedID)
	if err != nil {
		http.Error(w, "Feed not found", http.StatusNotFound)
		return
	}

	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "videos"
	}

	channels, err := s.db.GetChannelsByFeed(feedID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	videos, err := s.db.GetVideosByFeed(feedID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get watch progress for all videos
	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	// Get all feeds for the move dropdown
	allFeeds, err := s.db.GetFeeds()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check for error query param (from add channel)
	errorMsg := r.URL.Query().Get("error")

	data := map[string]any{
		"Title":       feed.Name,
		"Feed":        feed,
		"Tab":         tab,
		"Channels":    channels,
		"Videos":      videos,
		"ProgressMap": progressMap,
		"AllFeeds":    allFeeds,
		"Error":       errorMsg,
	}
	s.templates.ExecuteTemplate(w, "feed", data)
}

func (s *Server) handleRefreshFeed(w http.ResponseWriter, r *http.Request) {
	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	channels, err := s.db.GetChannelsByFeed(feedID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Refreshing feed %d with %d channels", feedID, len(channels))

	// Fetch videos via RSS - fast and no rate limiting
	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalVideos int
	semaphore := make(chan struct{}, 20) // RSS is lightweight, can do more concurrent

	for _, ch := range channels {
		wg.Add(1)
		go func(channel models.Channel) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			videos, err := youtube.FetchLatestVideos(channel.URL, 5)
			if err != nil {
				log.Printf("Failed to fetch videos for %s: %v", channel.Name, err)
				return
			}

			log.Printf("Fetched %d videos from %s", len(videos), channel.Name)

			mu.Lock()
			defer mu.Unlock()

			for _, v := range videos {
				v.ChannelID = channel.ID
				if err := s.db.UpsertVideo(&v); err != nil {
					log.Printf("Failed to save video %s: %v", v.ID, err)
					continue
				}
				totalVideos++
			}
		}(ch)
	}

	wg.Wait()
	log.Printf("Refresh complete: %d total videos saved", totalVideos)

	// Redirect back to feed page
	http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feedID, 10), http.StatusSeeOther)
}

// handleRefreshFeedStream provides SSE progress updates during feed refresh
func (s *Server) handleRefreshFeedStream(w http.ResponseWriter, r *http.Request) {
	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	channels, err := s.db.GetChannelsByFeed(feedID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // For nginx proxies

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	total := len(channels)
	log.Printf("SSE refresh: feed %d with %d channels", feedID, total)

	// Handle empty feed
	if total == 0 {
		complete := map[string]any{
			"totalVideos": 0,
			"feedID":      feedID,
		}
		data, _ := json.Marshal(complete)
		fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Process channels sequentially to ensure ordered progress updates
	var totalVideos int
	for i, ch := range channels {
		// Send progress event
		progress := map[string]any{
			"current": i + 1,
			"total":   total,
			"channel": ch.Name,
		}
		data, _ := json.Marshal(progress)
		fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
		flusher.Flush()

		// Fetch videos
		videos, err := youtube.FetchLatestVideos(ch.URL, 5)
		if err != nil {
			log.Printf("Failed to fetch videos for %s: %v", ch.Name, err)
			continue
		}

		for _, v := range videos {
			v.ChannelID = ch.ID
			if err := s.db.UpsertVideo(&v); err != nil {
				log.Printf("Failed to save video %s: %v", v.ID, err)
				continue
			}
			totalVideos++
		}
	}

	// Send completion event
	complete := map[string]any{
		"totalVideos": totalVideos,
		"feedID":      feedID,
	}
	data, _ := json.Marshal(complete)
	fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
	flusher.Flush()

	log.Printf("SSE refresh complete: %d videos saved for feed %d", totalVideos, feedID)
}

func (s *Server) handleDeleteFeed(w http.ResponseWriter, r *http.Request) {
	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteFeed(feedID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isHtmxRequest(r) {
		htmxRedirect(w, "/")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleWatchPage(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	// Get watch progress for resume functionality
	var startTime int
	if wp, err := s.db.GetWatchProgress(videoID); err == nil {
		// Only resume if not near the end (within 30 seconds)
		if wp.DurationSeconds > 0 && wp.ProgressSeconds < wp.DurationSeconds-30 {
			startTime = wp.ProgressSeconds
		}
	}

	// Get all feeds for subscribe dropdown
	feeds, _ := s.db.GetFeeds()

	// Check query params for subscription status
	subscribed := r.URL.Query().Get("subscribed")

	// Render page immediately - video info will be fetched async via /watch/{id}/info
	data := map[string]any{
		"Title":      "Loading...",
		"VideoID":    videoID,
		"StartTime":  startTime,
		"Feeds":      feeds,
		"Subscribed": subscribed,
	}
	s.templates.ExecuteTemplate(w, "watch", data)
}

// handleWatchInfo returns video info and stream URL as JSON (called async from watch page)
func (s *Server) handleWatchInfo(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	videoURL := "https://www.youtube.com/watch?v=" + videoID

	// Get video info
	info, err := s.ytdlp.GetVideoInfo(videoURL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get video info"})
		return
	}

	// Get stream URL
	streamURL, err := s.ytdlp.GetStreamURL(videoURL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get stream URL"})
		return
	}

	// Resolve channel URL to canonical form for subscription
	channelURL := ""
	if info.ChannelURL != "" {
		if channelInfo, err := youtube.ResolveChannelURL(info.ChannelURL); err == nil {
			channelURL = channelInfo.URL
		}
	}

	// Check if already subscribed
	var existingChannelID int64
	if channelURL != "" {
		if existing, _ := s.db.GetChannelByURL(channelURL); existing != nil {
			existingChannelID = existing.ID
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"title":             info.Title,
		"channel":           info.Channel,
		"streamURL":         streamURL,
		"channelURL":        channelURL,
		"existingChannelID": existingChannelID,
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	quality := r.URL.Query().Get("quality")
	if quality == "" {
		quality = "best"
	}

	videoURL := "https://www.youtube.com/watch?v=" + videoID

	downloadURL, ext, err := s.ytdlp.GetDownloadURL(videoURL, quality)
	if err != nil {
		log.Printf("Failed to get download URL: %v", err)
		http.Error(w, "Failed to get download URL", http.StatusInternalServerError)
		return
	}

	// Set headers to trigger download in browser
	filename := videoID + "." + ext
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	// Redirect to the direct URL - browser will download
	http.Redirect(w, r, downloadURL, http.StatusFound)
}

func (s *Server) handleChannelPage(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	channel, err := s.db.GetChannel(channelID)
	if err != nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	feed, err := s.db.GetFeed(channel.FeedID)
	if err != nil {
		http.Error(w, "Feed not found", http.StatusNotFound)
		return
	}

	videos, err := s.db.GetVideosByChannel(channelID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get watch progress for all videos
	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	data := map[string]any{
		"Title":       channel.Name,
		"Channel":     channel,
		"Feed":        feed,
		"Videos":      videos,
		"ProgressMap": progressMap,
	}
	s.templates.ExecuteTemplate(w, "channel", data)
}

func (s *Server) handleAddChannel(w http.ResponseWriter, r *http.Request) {
	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	channelURL := strings.TrimSpace(r.FormValue("url"))
	if channelURL == "" {
		http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feedID, 10)+"?tab=channels&error=url_required", http.StatusSeeOther)
		return
	}

	// Resolve the channel URL to get channel info
	info, err := youtube.ResolveChannelURL(channelURL)
	if err != nil {
		log.Printf("Failed to resolve channel URL %s: %v", channelURL, err)
		http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feedID, 10)+"?tab=channels&error=invalid_channel", http.StatusSeeOther)
		return
	}

	// Add the channel to the feed
	if _, err := s.db.AddChannel(feedID, info.URL, info.Name); err != nil {
		log.Printf("Failed to add channel: %v", err)
		http.Error(w, "Failed to add channel", http.StatusInternalServerError)
		return
	}

	log.Printf("Added channel %s (%s) to feed %d", info.Name, info.URL, feedID)
	http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feedID, 10)+"?tab=channels", http.StatusSeeOther)
}

func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	// Get channel to know which feed to redirect to
	channel, err := s.db.GetChannel(channelID)
	if err != nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	feedID := channel.FeedID

	if err := s.db.DeleteChannel(channelID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// For htmx, return empty response - element will be removed via hx-swap="delete"
	if isHtmxRequest(r) {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/feeds/"+strconv.FormatInt(feedID, 10)+"?tab=channels", http.StatusSeeOther)
}

func (s *Server) handleMoveChannel(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	newFeedID, err := strconv.ParseInt(r.FormValue("feed_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	// Get channel to know which feed to redirect to
	channel, err := s.db.GetChannel(channelID)
	if err != nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	originalFeedID := channel.FeedID

	if err := s.db.MoveChannel(channelID, newFeedID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// For htmx, return empty response - element will be removed via hx-swap="delete"
	if isHtmxRequest(r) {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/feeds/"+strconv.FormatInt(originalFeedID, 10)+"?tab=channels", http.StatusSeeOther)
}

// handleOrganize uses AI to suggest groups for subscriptions
func (s *Server) handleOrganize(w http.ResponseWriter, r *http.Request) {
	if s.ai == nil {
		s.renderImportError(w, "AI grouping not available - set OPENAI_API_KEY")
		return
	}

	if err := r.ParseForm(); err != nil {
		s.renderImportError(w, "Failed to parse form")
		return
	}

	jsonData := r.FormValue("json")
	if jsonData == "" {
		s.renderImportError(w, "JSON data is required")
		return
	}

	var export models.NewPipeExport
	if err := json.Unmarshal([]byte(jsonData), &export); err != nil {
		s.renderImportError(w, "Invalid NewPipe JSON format: "+err.Error())
		return
	}

	// Filter to YouTube only
	var subs []models.NewPipeSubscription
	for _, sub := range export.Subscriptions {
		if sub.ServiceID == 0 {
			subs = append(subs, sub)
		}
	}

	if len(subs) == 0 {
		s.renderImportError(w, "No YouTube subscriptions found")
		return
	}

	// Fetch metadata (recent video titles) for each channel concurrently
	log.Printf("Fetching metadata for %d channels...", len(subs))
	metadata := s.fetchChannelMetadata(subs)
	log.Printf("Fetched metadata for %d channels", len(metadata))

	// Call AI to suggest groups with metadata
	suggestions, err := s.ai.SuggestGroupsWithMetadata(subs, metadata)
	if err != nil {
		log.Printf("AI grouping failed: %v", err)
		s.renderImportError(w, "AI grouping failed: "+err.Error())
		return
	}

	data := map[string]any{
		"Title":       "Review Groups",
		"Suggestions": suggestions,
	}
	s.templates.ExecuteTemplate(w, "organize", data)
}

// fetchChannelMetadata fetches recent video titles for channels to help AI categorization
// Uses cached data from DB when available, only fetches missing channels
func (s *Server) fetchChannelMetadata(subs []models.NewPipeSubscription) map[string]ai.ChannelInfo {
	metadata := make(map[string]ai.ChannelInfo)

	// Load cached metadata from DB
	cached, err := s.db.GetAllChannelMetadata()
	if err != nil {
		log.Printf("Failed to load cached metadata: %v", err)
		cached = make(map[string]*db.ChannelMetadata)
	}

	// Identify channels that need fetching
	var toFetch []models.NewPipeSubscription
	for _, sub := range subs {
		if cm, ok := cached[sub.URL]; ok {
			// Use cached data
			titles := strings.Split(cm.VideoTitles, "|||")
			metadata[sub.URL] = ai.ChannelInfo{
				Name:        sub.Name,
				URL:         sub.URL,
				VideoTitles: titles,
			}
		} else {
			toFetch = append(toFetch, sub)
		}
	}

	log.Printf("Using %d cached, fetching %d new channels", len(subs)-len(toFetch), len(toFetch))

	if len(toFetch) == 0 {
		return metadata
	}

	// Fetch missing channels concurrently
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 20) // Limit concurrent requests

	for _, sub := range toFetch {
		wg.Add(1)
		go func(sub models.NewPipeSubscription) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			videos, err := youtube.FetchLatestVideos(sub.URL, 3)
			if err != nil {
				log.Printf("Failed to fetch videos for %s: %v", sub.Name, err)
				return
			}

			var titles []string
			for _, v := range videos {
				titles = append(titles, v.Title)
			}

			// Cache to DB
			if err := s.db.UpsertChannelMetadata(sub.URL, sub.Name, strings.Join(titles, "|||")); err != nil {
				log.Printf("Failed to cache metadata for %s: %v", sub.Name, err)
			}

			mu.Lock()
			metadata[sub.URL] = ai.ChannelInfo{
				Name:        sub.Name,
				URL:         sub.URL,
				VideoTitles: titles,
			}
			mu.Unlock()
		}(sub)
	}

	wg.Wait()
	return metadata
}

// handleConfirmOrganize creates feeds from the confirmed groups
func (s *Server) handleConfirmOrganize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Parse the groups JSON from the form
	groupsJSON := r.FormValue("groups")
	if groupsJSON == "" {
		http.Error(w, "No groups data", http.StatusBadRequest)
		return
	}

	var groups []struct {
		Name     string `json:"name"`
		Channels []struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		} `json:"channels"`
	}

	if err := json.Unmarshal([]byte(groupsJSON), &groups); err != nil {
		http.Error(w, "Invalid groups data", http.StatusBadRequest)
		return
	}

	// Create a feed for each group
	for _, g := range groups {
		if len(g.Channels) == 0 {
			continue
		}

		feed, err := s.db.CreateFeed(g.Name)
		if err != nil {
			log.Printf("Failed to create feed %s: %v", g.Name, err)
			continue
		}

		for _, ch := range g.Channels {
			if _, err := s.db.AddChannel(feed.ID, ch.URL, ch.Name); err != nil {
				log.Printf("Failed to add channel %s: %v", ch.Name, err)
			}
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleExportFeed exports a feed as JSON
// Use ?format=newpipe for NewPipe-compatible format, otherwise uses Feeds format
func (s *Server) handleExportFeed(w http.ResponseWriter, r *http.Request) {
	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	feed, err := s.db.GetFeed(feedID)
	if err != nil {
		http.Error(w, "Feed not found", http.StatusNotFound)
		return
	}

	channels, err := s.db.GetChannelsByFeed(feedID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	format := r.URL.Query().Get("format")

	w.Header().Set("Content-Type", "application/json")

	if format == "newpipe" {
		// Build NewPipe-compatible export
		export := models.NewPipeExport{
			Subscriptions: make([]models.NewPipeSubscription, 0, len(channels)),
		}
		for _, ch := range channels {
			export.Subscriptions = append(export.Subscriptions, models.NewPipeSubscription{
				ServiceID: 0,
				URL:       ch.URL,
				Name:      ch.Name,
			})
		}
		w.Header().Set("Content-Disposition", "attachment; filename=subscriptions.json")
		json.NewEncoder(w).Encode(export)
		return
	}

	// Default: Feeds format
	var tags []string
	if feed.Tags != "" {
		tags = strings.Split(feed.Tags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	export := models.FeedExport{
		Version:     1,
		Name:        feed.Name,
		Description: feed.Description,
		Author:      feed.Author,
		Tags:        tags,
		Updated:     feed.UpdatedAt,
		Channels:    make([]models.ExportChannel, 0, len(channels)),
	}
	for _, ch := range channels {
		export.Channels = append(export.Channels, models.ExportChannel{
			URL:  ch.URL,
			Name: ch.Name,
		})
	}

	// Use feed name as filename, sanitized
	filename := strings.ReplaceAll(feed.Name, " ", "-") + ".json"
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	json.NewEncoder(w).Encode(export)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	videos, err := s.db.GetWatchHistory(100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get watch progress for all videos
	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	data := map[string]any{
		"Title":       "History",
		"Videos":      videos,
		"ProgressMap": progressMap,
	}
	s.templates.ExecuteTemplate(w, "history", data)
}

func (s *Server) handleUpdateWatchProgress(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	var req struct {
		Progress int `json:"progress"`
		Duration int `json:"duration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.db.UpdateWatchProgress(videoID, req.Progress, req.Duration); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleMarkWatched(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	// Mark as 100% watched (use a placeholder duration if we don't know the real one)
	if err := s.db.MarkAsWatched(videoID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleMarkUnwatched(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	if err := s.db.DeleteWatchProgress(videoID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOpenVideo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	if url == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	videoID := youtube.ExtractVideoID(url)
	if videoID == "" {
		http.Redirect(w, r, "/?error=invalid_url", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/watch/"+videoID, http.StatusSeeOther)
}

func (s *Server) handleSubscribeFromWatch(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	feedID, err := strconv.ParseInt(r.FormValue("feed_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	channelURL := r.FormValue("channel_url")
	channelName := r.FormValue("channel_name")

	// Check if already subscribed
	existing, err := s.db.GetChannelByURL(channelURL)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		// Already subscribed, just redirect back
		http.Redirect(w, r, "/watch/"+videoID+"?subscribed=already", http.StatusSeeOther)
		return
	}

	// Handle "Uncategorized" feed (feed_id=0)
	if feedID == 0 {
		feed, err := s.db.GetOrCreateFeed("Uncategorized")
		if err != nil {
			log.Printf("Failed to create Uncategorized feed: %v", err)
			http.Error(w, "Failed to create feed", http.StatusInternalServerError)
			return
		}
		feedID = feed.ID
	}

	// Add the channel
	if _, err := s.db.AddChannel(feedID, channelURL, channelName); err != nil {
		log.Printf("Failed to add channel: %v", err)
		http.Error(w, "Failed to subscribe", http.StatusInternalServerError)
		return
	}

	log.Printf("Subscribed to %s (%s) in feed %d from watch page", channelName, channelURL, feedID)
	http.Redirect(w, r, "/watch/"+videoID+"?subscribed=true", http.StatusSeeOther)
}

// handlePacksList returns a JSON list of available packs
func (s *Server) handlePacksList(w http.ResponseWriter, r *http.Request) {
	entries, err := fs.ReadDir(s.packs, "packs")
	if err != nil {
		http.Error(w, "Failed to read packs", http.StatusInternalServerError)
		return
	}

	type packInfo struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	var packs []packInfo
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			packs = append(packs, packInfo{
				Name: strings.TrimSuffix(entry.Name(), ".json"),
				URL:  "/packs/" + entry.Name(),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(packs)
}

// handlePackFile serves a specific pack file
func (s *Server) handlePackFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !strings.HasSuffix(name, ".json") {
		name += ".json"
	}

	data, err := fs.ReadFile(s.packs, "packs/"+name)
	if err != nil {
		http.Error(w, "Pack not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
