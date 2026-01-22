package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/erik/feeds/internal/db"
	"github.com/erik/feeds/internal/models"
	"github.com/erik/feeds/internal/sponsorblock"
	"github.com/erik/feeds/internal/youtube"
	"github.com/erik/feeds/internal/ytdlp"
)

type Server struct {
	db           *db.DB
	ytdlp        *ytdlp.YTDLP
	sponsorblock *sponsorblock.Client
	templates       *template.Template
	packs           fs.FS
	videoCache      *VideoCache
	downloadManager *DownloadManager

	// Stream URL cache (video ID -> cached entry)
	streamCache   map[string]*streamCacheEntry
	streamCacheMu sync.RWMutex
}

type streamCacheEntry struct {
	streamURL  string
	title      string
	channel    string
	channelURL string
	thumbnail  string
	viewCount  int64
	expiresAt  time.Time
}

func NewServer(database *db.DB, yt *ytdlp.YTDLP, templatesFS fs.FS, packsFS fs.FS) (*Server, error) {
	funcMap := template.FuncMap{
		"div": func(a, b int) int { return a / b },
		"mod": func(a, b int) int { return a % b },
		"mul": func(a, b int) int { return a * b },
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	// Ensure Inbox system feed exists
	if _, err := database.EnsureInboxExists(); err != nil {
		return nil, fmt.Errorf("failed to create Inbox: %w", err)
	}

	videoCache := NewVideoCache()

	return &Server{
		db:              database,
		ytdlp:           yt,
		sponsorblock:    sponsorblock.NewClient(),
		templates:       tmpl,
		packs:           packsFS,
		videoCache:      videoCache,
		downloadManager: NewDownloadManager(videoCache, yt),
		streamCache:     make(map[string]*streamCacheEntry),
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
	// Legacy template-based routes (will be removed once SPA is complete)
	mux.HandleFunc("GET /legacy/{$}", s.handleIndex)
	mux.HandleFunc("GET /legacy/import", s.handleImportPage)
	mux.HandleFunc("POST /legacy/import", s.handleImport)
	mux.HandleFunc("POST /legacy/import/url", s.handleImportURL)
	mux.HandleFunc("POST /legacy/import/file", s.handleImportFile)
	mux.HandleFunc("POST /legacy/import/confirm", s.handleConfirmOrganize)
	mux.HandleFunc("GET /legacy/feeds/{id}", s.handleFeedPage)
	mux.HandleFunc("GET /legacy/channels/{id}", s.handleChannelPage)
	mux.HandleFunc("GET /legacy/watch/{id}", s.handleWatchPage)
	mux.HandleFunc("GET /legacy/all", s.handleAllRecent)
	mux.HandleFunc("GET /legacy/history", s.handleHistory)

	// JSON API routes for SPA
	mux.HandleFunc("GET /api/feeds", s.handleAPIGetFeeds)
	mux.HandleFunc("POST /api/feeds", s.handleAPICreateFeed)
	mux.HandleFunc("GET /api/feeds/{id}", s.handleAPIGetFeed)
	mux.HandleFunc("DELETE /api/feeds/{id}", s.handleAPIDeleteFeed)
	mux.HandleFunc("PUT /api/feeds/reorder", s.handleAPIReorderFeeds)
	mux.HandleFunc("GET /api/feeds/{id}/export", s.handleExportFeed)
	mux.HandleFunc("GET /api/feeds/{id}/shuffle", s.handleAPIGetShuffledVideos)
	mux.HandleFunc("POST /api/feeds/{id}/refresh", s.handleAPIRefreshFeed)
	mux.HandleFunc("GET /api/feeds/{id}/refresh/stream", s.handleRefreshFeedStream)

	mux.HandleFunc("GET /api/channels/{id}", s.handleAPIGetChannel)
	mux.HandleFunc("GET /api/channels/{id}/feeds", s.handleAPIGetChannelFeeds)
	mux.HandleFunc("POST /api/feeds/{id}/channels", s.handleAPIAddChannel)
	mux.HandleFunc("DELETE /api/channels/{id}", s.handleAPIDeleteChannel)
	mux.HandleFunc("DELETE /api/feeds/{feedId}/channels/{channelId}", s.handleAPIRemoveChannelFromFeed)
	mux.HandleFunc("POST /api/channels/{id}/feeds", s.handleAPIAddChannelToFeed)
	mux.HandleFunc("POST /api/channels/{id}/refresh", s.handleAPIRefreshChannel)
	mux.HandleFunc("GET /api/channels/{id}/fetch-more", s.handleAPIFetchMoreVideos)

	mux.HandleFunc("GET /api/videos/recent", s.handleAPIGetRecentVideos)
	mux.HandleFunc("GET /api/videos/history", s.handleAPIGetHistory)
	mux.HandleFunc("GET /api/videos/{id}/info", s.handleWatchInfo)
	mux.HandleFunc("GET /api/videos/{id}/nearby", s.handleAPINearbyVideos)
	mux.HandleFunc("GET /api/videos/{id}/segments", s.handleAPIGetSegments)
	mux.HandleFunc("POST /api/videos/{id}/progress", s.handleUpdateWatchProgress)
	mux.HandleFunc("POST /api/videos/{id}/watched", s.handleAPIMarkWatched)
	mux.HandleFunc("DELETE /api/videos/{id}/watched", s.handleAPIMarkUnwatched)

	mux.HandleFunc("GET /api/download/{id}", s.handleDownload)
	mux.HandleFunc("GET /api/stream/{id}", s.handleStreamProxy)

	mux.HandleFunc("POST /api/videos/{id}/download", s.handleStartDownload)
	mux.HandleFunc("GET /api/videos/{id}/download/status", s.handleDownloadStatus)
	mux.HandleFunc("GET /api/videos/{id}/qualities", s.handleGetQualities)

	mux.HandleFunc("POST /api/import/url", s.handleAPIImportURL)
	mux.HandleFunc("POST /api/import/file", s.handleAPIImportFile)
	mux.HandleFunc("POST /api/import/confirm", s.handleAPIConfirmOrganize)
	mux.HandleFunc("POST /api/import/watch-history", s.handleAPIImportWatchHistory)

	mux.HandleFunc("GET /api/packs", s.handlePacksList)
	mux.HandleFunc("GET /api/packs/{name}", s.handlePackFile)

	mux.HandleFunc("GET /api/config", s.handleAPIConfig)
	mux.HandleFunc("POST /api/config/ytdlp-cookies", s.handleAPISetYTDLPCookies)
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
	videos, total, err := s.db.GetAllRecentVideos(100, 0)
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
		"Total":       total,
	}
	s.templates.ExecuteTemplate(w, "all", data)
}

func (s *Server) handleImportPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title": "Import Feed",
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
		"Title": "Import Feed",
		"Error": errMsg,
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

	videos, _, err := s.db.GetVideosByFeed(feedID, 50, 0)
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
	var allVideos []models.Video
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
			for i := range videos {
				videos[i].ChannelID = channel.ID
				allVideos = append(allVideos, videos[i])
			}
			mu.Unlock()
		}(ch)
	}

	wg.Wait()

	// Check shorts status synchronously before saving
	var totalVideos int
	if len(allVideos) > 0 {
		videoIDs := make([]string, len(allVideos))
		for i, v := range allVideos {
			videoIDs[i] = v.ID
		}
		shortsStatus := youtube.CheckShortsStatus(videoIDs)

		for i := range allVideos {
			if isShort, ok := shortsStatus[allVideos[i].ID]; ok {
				allVideos[i].IsShort = &isShort
			}
			if _, err := s.db.UpsertVideo(&allVideos[i]); err != nil {
				log.Printf("Failed to save video %s: %v", allVideos[i].ID, err)
				continue
			}
			totalVideos++
		}
	}
	log.Printf("Refresh complete: %d total videos saved", totalVideos)

	// Fetch durations in background (shorts status is checked synchronously now)
	go s.fetchMissingDurations(feedID)

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

	// Use worker pool for parallel fetching
	const maxWorkers = 5

	type result struct {
		videos  []models.Video
		err     error
		chName  string
		chID    int64
	}

	jobs := make(chan *models.Channel, len(channels))
	results := make(chan result, len(channels))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ch := range jobs {
				videos, err := youtube.FetchLatestVideos(ch.URL, 5)
				results <- result{videos: videos, err: err, chName: ch.Name, chID: ch.ID}
			}
		}()
	}

	// Send jobs
	for i := range channels {
		jobs <- &channels[i]
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results and send progress
	var totalVideos int
	var completed int
	var errors []string

	for res := range results {
		completed++

		// Send progress event
		progress := map[string]any{
			"current": completed,
			"total":   total,
			"channel": res.chName,
		}
		data, _ := json.Marshal(progress)
		fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
		flusher.Flush()

		if res.err != nil {
			errors = append(errors, res.chName+": "+res.err.Error())
			log.Printf("Failed to fetch videos for %s: %v", res.chName, res.err)
			continue
		}

		if len(res.videos) > 0 {
			// Check shorts status only for videos that don't already have it
			videoIDs := make([]string, len(res.videos))
			for i, v := range res.videos {
				videoIDs[i] = v.ID
			}

			// Get existing shorts status from DB
			existingStatus, err := s.db.GetVideoShortsStatus(videoIDs)
			if err != nil {
				log.Printf("Failed to get existing shorts status: %v", err)
				existingStatus = map[string]bool{}
			}

			// Only check shorts for videos that don't have status yet
			var needsCheck []string
			for _, id := range videoIDs {
				if _, hasStatus := existingStatus[id]; !hasStatus {
					needsCheck = append(needsCheck, id)
				}
			}

			// Fetch shorts status only for new videos
			var shortsStatus map[string]bool
			if len(needsCheck) > 0 {
				shortsStatus = youtube.CheckShortsStatus(needsCheck)
			} else {
				shortsStatus = map[string]bool{}
			}

			// Merge existing status into results
			for id, isShort := range existingStatus {
				shortsStatus[id] = isShort
			}

			for i := range res.videos {
				res.videos[i].ChannelID = res.chID
				if isShort, ok := shortsStatus[res.videos[i].ID]; ok {
					res.videos[i].IsShort = &isShort
				}
				if _, err := s.db.UpsertVideo(&res.videos[i]); err != nil {
					log.Printf("Failed to save video %s: %v", res.videos[i].ID, err)
					continue
				}
				totalVideos++
			}
		}
	}

	// Fetch durations for videos that don't have them (in background)
	// Note: shorts status is checked synchronously now
	go s.fetchMissingDurations(feedID)

	// Send completion event
	complete := map[string]any{
		"totalVideos": totalVideos,
		"feedID":      feedID,
		"errors":      errors,
	}
	data, _ := json.Marshal(complete)
	fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
	flusher.Flush()

	log.Printf("SSE refresh complete: %d videos saved for feed %d", totalVideos, feedID)
}

// fetchMissingDurations fetches durations for videos that don't have them
func (s *Server) fetchMissingDurations(feedID int64) {
	// Get videos without duration (limit to most recent 50 to avoid long waits)
	videoIDs, err := s.db.GetVideosWithoutDuration(feedID, 50)
	if err != nil {
		log.Printf("Failed to get videos without duration: %v", err)
		return
	}

	if len(videoIDs) == 0 {
		return
	}

	log.Printf("Fetching durations for %d videos in feed %d", len(videoIDs), feedID)

	// Fetch durations in batches to avoid overwhelming yt-dlp
	batchSize := 10
	for i := 0; i < len(videoIDs); i += batchSize {
		end := i + batchSize
		if end > len(videoIDs) {
			end = len(videoIDs)
		}
		batch := videoIDs[i:end]

		durations, err := s.ytdlp.GetVideoDurations(batch)
		if err != nil {
			log.Printf("Failed to fetch durations for batch: %v", err)
			continue
		}

		for videoID, duration := range durations {
			if err := s.db.UpdateVideoDuration(videoID, duration); err != nil {
				log.Printf("Failed to update duration for %s: %v", videoID, err)
			}
		}
	}

	log.Printf("Finished fetching durations for feed %d", feedID)
}

// fetchMissingShortsStatus checks shorts status for videos that don't have it
func (s *Server) fetchMissingShortsStatus(feedID int64) {
	videoIDs, err := s.db.GetVideosWithoutShortStatus(feedID, 100)
	if err != nil {
		log.Printf("Failed to get videos without shorts status: %v", err)
		return
	}

	if len(videoIDs) == 0 {
		return
	}

	log.Printf("Checking shorts status for %d videos in feed %d", len(videoIDs), feedID)

	// Check in batches
	batchSize := 20
	for i := 0; i < len(videoIDs); i += batchSize {
		end := i + batchSize
		if end > len(videoIDs) {
			end = len(videoIDs)
		}
		batch := videoIDs[i:end]

		results := youtube.CheckShortsStatus(batch)
		for videoID, isShort := range results {
			if err := s.db.UpdateVideoIsShort(videoID, isShort); err != nil {
				log.Printf("Failed to update shorts status for %s: %v", videoID, err)
			}
		}
	}

	log.Printf("Finished checking shorts status for feed %d", feedID)
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

	// Check cache first
	s.streamCacheMu.RLock()
	cached, ok := s.streamCache[videoID]
	s.streamCacheMu.RUnlock()

	if ok && time.Now().Before(cached.expiresAt) {
		// Cache hit - check subscription status and return
		chInfo := s.getChannelInfo(cached.channelURL)

		// Get saved progress for resume
		var resumeFrom int
		if wp, err := s.db.GetWatchProgress(videoID); err == nil {
			resumeFrom = wp.ProgressSeconds
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"title":              cached.title,
			"channel":            cached.channel,
			"streamURL":          cached.streamURL,
			"channelURL":         cached.channelURL,
			"thumbnail":          cached.thumbnail,
			"channelId":          chInfo.ChannelID,
			"channelMemberships": chInfo.Memberships,
			"viewCount":          cached.viewCount,
			"resumeFrom":         resumeFrom,
		})
		return
	}

	// Cache miss - fetch from ytdlp
	videoURL := "https://www.youtube.com/watch?v=" + videoID

	// Get video info
	info, err := s.ytdlp.GetVideoInfo(videoURL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get video info"})
		return
	}

	// Use proxy URL to avoid IP-locked stream URLs
	streamURL := "/api/stream/" + videoID

	// Resolve channel URL to canonical form for subscription
	channelURL := ""
	if info.ChannelURL != "" {
		if channelInfo, err := youtube.ResolveChannelURL(info.ChannelURL); err == nil {
			channelURL = channelInfo.URL
		}
	}

	// Cache the result (5 minute TTL - stream URLs expire after ~6 hours but we keep it short)
	s.streamCacheMu.Lock()
	s.streamCache[videoID] = &streamCacheEntry{
		streamURL:  streamURL,
		title:      info.Title,
		channel:    info.Channel,
		channelURL: channelURL,
		thumbnail:  info.GetBestThumbnail(),
		viewCount:  info.ViewCount,
		expiresAt:  time.Now().Add(5 * time.Minute),
	}
	s.streamCacheMu.Unlock()

	// Get channel info (ID if known + memberships)
	chInfo := s.getChannelInfo(channelURL)

	// Get saved progress for resume
	var resumeFrom int
	if wp, err := s.db.GetWatchProgress(videoID); err == nil {
		resumeFrom = wp.ProgressSeconds
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"title":              info.Title,
		"channel":            info.Channel,
		"streamURL":          streamURL,
		"channelURL":         channelURL,
		"thumbnail":          info.GetBestThumbnail(),
		"channelId":          chInfo.ChannelID,
		"channelMemberships": chInfo.Memberships,
		"viewCount":          info.ViewCount,
		"resumeFrom":         resumeFrom,
	})
}

// channelMembership represents a channel's membership in a feed
type channelMembership struct {
	ChannelID int64  `json:"channelId"`
	FeedID    int64  `json:"feedId"`
	FeedName  string `json:"feedName"`
}

// channelInfo contains the channel ID (if known) and feed memberships
type channelInfo struct {
	ChannelID   *int64             `json:"channelId"` // nil if channel not yet in system
	Memberships []channelMembership `json:"memberships"`
}

// getChannelInfo returns the channel ID (if known) and all feeds that contain a channel with the given URL
func (s *Server) getChannelInfo(channelURL string) channelInfo {
	if channelURL == "" {
		return channelInfo{Memberships: []channelMembership{}}
	}

	channel, err := s.db.GetChannelByURL(channelURL)
	if err != nil || channel == nil {
		return channelInfo{Memberships: []channelMembership{}}
	}

	feeds, err := s.db.GetFeedsByChannel(channel.ID)
	if err != nil {
		return channelInfo{ChannelID: &channel.ID, Memberships: []channelMembership{}}
	}

	memberships := make([]channelMembership, 0, len(feeds))
	for _, feed := range feeds {
		name := feed.Name
		if feed.IsSystem {
			name = "Inbox"
		}
		memberships = append(memberships, channelMembership{
			ChannelID: channel.ID,
			FeedID:    feed.ID,
			FeedName:  name,
		})
	}
	return channelInfo{ChannelID: &channel.ID, Memberships: memberships}
}

// handleAPINearbyVideos returns videos from the same feed, positioned after the current video
func (s *Server) handleAPINearbyVideos(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	videos, feedID, err := s.db.GetNearbyVideos(videoID, limit, offset)
	if err != nil {
		// Video might not be in our database (e.g., watching from URL)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"videos":      []models.Video{},
			"feedId":      0,
			"progressMap": map[string]any{},
		})
		return
	}

	// Get watch progress for all videos
	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"videos":      videos,
		"feedId":      feedID,
		"progressMap": progressMap,
	})
}

// handleAPIGetSegments returns SponsorBlock segments for a video
func (s *Server) handleAPIGetSegments(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	// Check cache first
	hasCached, fetchedAt, err := s.db.HasSponsorBlockSegments(videoID)
	if err != nil {
		log.Printf("Failed to check SponsorBlock cache: %v", err)
	}

	// Cache TTL: 24 hours
	cacheTTL := 24 * time.Hour

	// If we have fresh cached data, return it
	if hasCached && time.Since(fetchedAt) < cacheTTL {
		segments, err := s.db.GetSponsorBlockSegments(videoID)
		if err != nil {
			log.Printf("Failed to get cached segments: %v", err)
		} else {
			// Filter out the placeholder segment
			var result []map[string]any
			for _, seg := range segments {
				if seg.SegmentUUID == "__no_segments__" {
					continue
				}
				result = append(result, map[string]any{
					"uuid":       seg.SegmentUUID,
					"startTime":  seg.StartTime,
					"endTime":    seg.EndTime,
					"category":   seg.Category,
					"actionType": seg.ActionType,
					"votes":      seg.Votes,
				})
			}
			jsonResponse(w, map[string]any{
				"segments": result,
				"cached":   true,
			})
			return
		}
	}

	// Fetch from SponsorBlock API
	apiSegments, err := s.sponsorblock.GetSegments(videoID, nil)
	if err != nil {
		log.Printf("Failed to fetch SponsorBlock segments for %s: %v", videoID, err)
		// Return empty array, don't cache failure
		jsonResponse(w, map[string]any{
			"segments": []any{},
			"error":    "Failed to fetch segments",
		})
		return
	}

	// Convert to DB format and save
	if len(apiSegments) > 0 {
		dbSegments := make([]db.SponsorBlockSegment, len(apiSegments))
		for i, seg := range apiSegments {
			dbSegments[i] = db.SponsorBlockSegment{
				VideoID:     videoID,
				SegmentUUID: seg.UUID,
				StartTime:   seg.Segment[0],
				EndTime:     seg.Segment[1],
				Category:    seg.Category,
				ActionType:  seg.ActionType,
				Votes:       seg.Votes,
			}
		}
		if err := s.db.SaveSponsorBlockSegments(videoID, dbSegments); err != nil {
			log.Printf("Failed to cache SponsorBlock segments: %v", err)
		}
	} else {
		// Mark that we've fetched but found no segments
		if err := s.db.MarkSponsorBlockFetched(videoID); err != nil {
			log.Printf("Failed to mark SponsorBlock fetch: %v", err)
		}
	}

	// Return API response format
	var result []map[string]any
	for _, seg := range apiSegments {
		result = append(result, map[string]any{
			"uuid":       seg.UUID,
			"startTime":  seg.Segment[0],
			"endTime":    seg.Segment[1],
			"category":   seg.Category,
			"actionType": seg.ActionType,
			"votes":      seg.Votes,
		})
	}

	jsonResponse(w, map[string]any{
		"segments": result,
		"cached":   false,
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

// selectBestQuality returns the best quality to use for "auto" mode.
// Defaults to 1080p as a good balance of quality and download speed.
func selectBestQuality() string {
	return "1080"
}

func (s *Server) handleStreamProxy(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	quality := r.URL.Query().Get("quality")
	if quality == "" || quality == "auto" {
		quality = selectBestQuality()
	}

	// Check if already fully cached
	cacheKey := CacheKey(videoID, quality)
	if cachedPath, ok := s.videoCache.Get(cacheKey); ok {
		log.Printf("Serving fully cached video: %s quality %s", videoID, quality)
		http.ServeFile(w, r, cachedPath)
		return
	}

	// Start or get existing download
	download := s.downloadManager.GetOrStartDownload(videoID, quality)

	// If already complete, serve the file
	if download.Status == "complete" {
		log.Printf("Serving completed download: %s quality %s", videoID, quality)
		http.ServeFile(w, r, download.GetFilePath())
		return
	}

	// Wait for buffer threshold
	threshold := GetBufferThreshold(quality)
	log.Printf("Waiting for buffer (%d bytes) for %s quality %s", threshold, videoID, quality)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if err := download.WaitForBuffer(ctx, threshold); err != nil {
		log.Printf("Buffer wait failed for %s: %v", videoID, err)
		http.Error(w, "Buffering failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	log.Printf("Buffer ready for %s quality %s, serving partial file", videoID, quality)

	// Serve the partial file
	s.servePartialFile(w, r, download)
}

// servePartialFile serves a file that may still be downloading.
// It handles range requests and serves available data.
func (s *Server) servePartialFile(w http.ResponseWriter, r *http.Request, d *Download) {
	filePath := d.GetFilePath()

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get current file size
	currentSize := d.GetFileSize()
	if currentSize == 0 {
		info, err := file.Stat()
		if err != nil {
			http.Error(w, "Failed to stat file", http.StatusInternalServerError)
			return
		}
		currentSize = info.Size()
	}

	// Set content type
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")

	// Handle range request
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		s.servePartialFileRange(w, file, currentSize, rangeHeader)
		return
	}

	// No range request - serve from beginning
	// Use current size as content length (client will handle incomplete)
	w.Header().Set("Content-Length", strconv.FormatInt(currentSize, 10))
	w.WriteHeader(http.StatusOK)

	// Copy available data
	io.CopyN(w, file, currentSize)
}

// servePartialFileRange handles range requests for partial files
func (s *Server) servePartialFileRange(w http.ResponseWriter, file *os.File, fileSize int64, rangeHeader string) {
	// Parse range header: "bytes=start-end" or "bytes=start-"
	var start, end int64
	_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
	if err != nil {
		// Try without end
		_, err = fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
		if err != nil {
			http.Error(w, "Invalid range", http.StatusBadRequest)
			return
		}
		end = fileSize - 1
	}

	// Validate range
	if start < 0 || start >= fileSize {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		http.Error(w, "Range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// Clamp end to available data
	if end >= fileSize {
		end = fileSize - 1
	}

	// Seek to start position
	if _, err := file.Seek(start, 0); err != nil {
		http.Error(w, "Seek failed", http.StatusInternalServerError)
		return
	}

	length := end - start + 1

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(http.StatusPartialContent)

	io.CopyN(w, file, length)
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

	// Get feeds this channel belongs to (pick first for legacy template)
	feeds, err := s.db.GetFeedsByChannel(channelID)
	if err != nil || len(feeds) == 0 {
		http.Error(w, "Channel not in any feed", http.StatusNotFound)
		return
	}
	feed := feeds[0]

	videos, err := s.db.GetVideosByChannel(channelID, 50, 0)
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

	if err := s.db.DeleteChannel(channelID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isHtmxRequest(r) {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

	// Also update the video's duration in the videos table if we have it
	if req.Duration > 0 {
		s.db.UpdateVideoDuration(videoID, req.Duration)
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
		// Already subscribed - return success (idempotent)
		w.WriteHeader(http.StatusOK)
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
	w.WriteHeader(http.StatusOK)
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

// handleStartDownload starts a background download for a specific quality
func (s *Server) handleStartDownload(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	var req struct {
		Quality string `json:"quality"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Quality == "" || req.Quality == "auto" {
		http.Error(w, "Quality must be specified (e.g., 720, 1080)", http.StatusBadRequest)
		return
	}

	download, err := s.downloadManager.StartDownload(videoID, req.Quality)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  download.Status,
		"quality": download.Quality,
	})
}

// handleDownloadStatus provides SSE progress updates for downloads
func (s *Server) handleDownloadStatus(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	ch := s.downloadManager.Subscribe(videoID)
	defer s.downloadManager.Unsubscribe(videoID, ch)

	// Send current status first
	status := s.downloadManager.GetStatus(videoID)
	for quality, d := range status {
		data, _ := json.Marshal(DownloadProgress{
			Quality: quality,
			Status:  d.Status,
			Error:   d.Error,
		})
		fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case progress, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(progress)
			fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
			flusher.Flush()

			if progress.Status == "complete" || progress.Status == "error" {
				return
			}
		}
	}
}

// handleGetQualities returns available, cached, and downloading qualities for a video
func (s *Server) handleGetQualities(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	// Available qualities (hardcoded for now, could query yt-dlp)
	available := []string{"360", "480", "720", "1080", "1440", "2160"}

	// Check which are cached
	var cached []string
	for _, q := range available {
		cacheKey := CacheKey(videoID, q)
		if _, ok := s.videoCache.Get(cacheKey); ok {
			cached = append(cached, q)
		}
	}

	// Check which is downloading (only explicit downloads, not streaming)
	var downloading *string
	status := s.downloadManager.GetStatus(videoID)
	for quality, d := range status {
		if (d.Status == "downloading" || d.Status == "muxing") && !d.IsStreaming {
			downloading = &quality
			break
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"available":   available,
		"cached":      cached,
		"downloading": downloading,
	})
}
