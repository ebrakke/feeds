package api

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/erik/yt-app/internal/ai"
	"github.com/erik/yt-app/internal/db"
	"github.com/erik/yt-app/internal/models"
	"github.com/erik/yt-app/internal/youtube"
	"github.com/erik/yt-app/internal/ytdlp"
)

type Server struct {
	db        *db.DB
	ytdlp     *ytdlp.YTDLP
	ai        *ai.Client
	templates *template.Template
}

func NewServer(database *db.DB, yt *ytdlp.YTDLP, aiClient *ai.Client, templatesDir string) (*Server, error) {
	funcMap := template.FuncMap{
		"div": func(a, b int) int { return a / b },
		"mod": func(a, b int) int { return a % b },
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(templatesDir + "/*.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		db:        database,
		ytdlp:     yt,
		ai:        aiClient,
		templates: tmpl,
	}, nil
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Pages
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /import", s.handleImportPage)
	mux.HandleFunc("POST /import", s.handleImport)
	mux.HandleFunc("POST /import/organize", s.handleOrganize)
	mux.HandleFunc("POST /import/confirm", s.handleConfirmOrganize)
	mux.HandleFunc("GET /feeds/{id}", s.handleFeedPage)
	mux.HandleFunc("GET /feeds/{id}/export", s.handleExportFeed)
	mux.HandleFunc("POST /feeds/{id}/refresh", s.handleRefreshFeed)
	mux.HandleFunc("POST /feeds/{id}/delete", s.handleDeleteFeed)
	mux.HandleFunc("GET /watch/{id}", s.handleWatchPage)
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

	data := map[string]any{
		"Title":    feed.Name,
		"Feed":     feed,
		"Tab":      tab,
		"Channels": channels,
		"Videos":   videos,
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

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleWatchPage(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	videoURL := "https://www.youtube.com/watch?v=" + videoID

	// Get video info
	info, err := s.ytdlp.GetVideoInfo(videoURL)
	if err != nil {
		http.Error(w, "Failed to get video info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get stream URL
	streamURL, err := s.ytdlp.GetStreamURL(videoURL)
	if err != nil {
		http.Error(w, "Failed to get stream URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":     info.Title,
		"Video":     info,
		"StreamURL": streamURL,
	}
	s.templates.ExecuteTemplate(w, "watch", data)
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

// handleExportFeed exports a feed as NewPipe-compatible JSON
func (s *Server) handleExportFeed(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=subscriptions.json")
	json.NewEncoder(w).Encode(export)
}
