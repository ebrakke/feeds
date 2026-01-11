package api

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/erik/yt-app/internal/db"
	"github.com/erik/yt-app/internal/models"
	"github.com/erik/yt-app/internal/youtube"
	"github.com/erik/yt-app/internal/ytdlp"
)

type Server struct {
	db        *db.DB
	ytdlp     *ytdlp.YTDLP
	templates *template.Template
}

func NewServer(database *db.DB, yt *ytdlp.YTDLP, templatesDir string) (*Server, error) {
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
		templates: tmpl,
	}, nil
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Pages
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /import", s.handleImportPage)
	mux.HandleFunc("POST /import", s.handleImport)
	mux.HandleFunc("GET /feeds/{id}", s.handleFeedPage)
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
