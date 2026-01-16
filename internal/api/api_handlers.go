package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/erik/feeds/internal/ai"
	"github.com/erik/feeds/internal/db"
	"github.com/erik/feeds/internal/models"
	yt "github.com/erik/feeds/internal/youtube"
)

// JSON response helpers

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// Config endpoint

func (s *Server) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]any{
		"aiEnabled": s.ai != nil,
	})
}

// Feed endpoints

func (s *Server) handleAPIGetFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := s.db.GetFeeds()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, feeds)
}

func (s *Server) handleAPICreateFeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		jsonError(w, "Name is required", http.StatusBadRequest)
		return
	}

	feed, err := s.db.CreateFeed(req.Name)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonResponse(w, feed)
}

func (s *Server) handleAPIGetFeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	feed, err := s.db.GetFeed(id)
	if err != nil {
		jsonError(w, "Feed not found", http.StatusNotFound)
		return
	}

	channels, err := s.db.GetChannelsByFeed(id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	videos, err := s.db.GetVideosByFeed(id, 100)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get watch progress for videos
	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	// Get all feeds for move dialog
	allFeeds, _ := s.db.GetFeeds()

	jsonResponse(w, map[string]any{
		"feed":        feed,
		"channels":    channels,
		"videos":      videos,
		"progressMap": progressMap,
		"allFeeds":    allFeeds,
	})
}

func (s *Server) handleAPIDeleteFeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteFeed(id); err != nil {
		if errors.Is(err, db.ErrSystemFeed) {
			jsonError(w, "Cannot delete system feed", http.StatusForbidden)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIRefreshFeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	channels, err := s.db.GetChannelsByFeed(id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use worker pool for parallel fetching with rate limiting
	const maxWorkers = 5

	type result struct {
		videos []models.Video
		err    error
		chName string
		chID   int64
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
				videos, err := yt.FetchLatestVideos(ch.URL, 5)
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

	// Collect results
	var totalVideos int
	var errors []string

	for res := range results {
		if res.err != nil {
			errors = append(errors, res.chName+": "+res.err.Error())
			continue
		}

		for _, v := range res.videos {
			v.ChannelID = res.chID
			if err := s.db.UpsertVideo(&v); err != nil {
				log.Printf("Failed to save video %s: %v", v.ID, err)
				continue
			}
			totalVideos++
		}
	}

	jsonResponse(w, map[string]any{
		"videosFound": totalVideos,
		"channels":    len(channels),
		"errors":      errors,
	})
}

// Channel endpoints

func (s *Server) handleAPIGetChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	channel, err := s.db.GetChannel(id)
	if err != nil {
		jsonError(w, "Channel not found", http.StatusNotFound)
		return
	}

	videos, err := s.db.GetVideosByChannel(id, 100)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	jsonResponse(w, map[string]any{
		"channel":     channel,
		"videos":      videos,
		"progressMap": progressMap,
	})
}

func (s *Server) handleAPIAddChannel(w http.ResponseWriter, r *http.Request) {
	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		jsonError(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Resolve channel info
	channelInfo, err := yt.ResolveChannelURL(req.URL)
	if err != nil {
		jsonError(w, "Invalid YouTube channel URL: "+err.Error(), http.StatusBadRequest)
		return
	}

	channel, err := s.db.AddChannel(feedID, channelInfo.URL, channelInfo.Name)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch initial videos
	videos, err := yt.FetchLatestVideos(channelInfo.URL, 5)
	if err == nil {
		for _, v := range videos {
			v.ChannelID = channel.ID
			s.db.UpsertVideo(&v)
		}
	}

	w.WriteHeader(http.StatusCreated)
	jsonResponse(w, channel)
}

func (s *Server) handleAPIDeleteChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteChannel(id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIMoveChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	var req struct {
		FeedID int64 `json:"feedId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.db.MoveChannel(id, req.FeedID); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIRefreshChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	channel, err := s.db.GetChannel(id)
	if err != nil {
		jsonError(w, "Channel not found", http.StatusNotFound)
		return
	}

	videos, err := yt.FetchLatestVideos(channel.URL, 20)
	if err != nil {
		jsonError(w, "Failed to fetch videos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var savedCount int
	for _, v := range videos {
		v.ChannelID = channel.ID
		if err := s.db.UpsertVideo(&v); err != nil {
			log.Printf("Failed to save video %s: %v", v.ID, err)
			continue
		}
		savedCount++
	}

	jsonResponse(w, map[string]any{
		"videosFound": savedCount,
		"channel":     channel.Name,
	})
}

// Video endpoints

func (s *Server) handleAPIGetRecentVideos(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	videos, err := s.db.GetAllRecentVideos(limit)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	jsonResponse(w, map[string]any{
		"videos":      videos,
		"progressMap": progressMap,
	})
}

func (s *Server) handleAPIGetHistory(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	videos, err := s.db.GetWatchHistory(limit)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get progress map for these videos
	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	jsonResponse(w, map[string]any{
		"videos":      videos,
		"progressMap": progressMap,
	})
}

func (s *Server) handleAPIMarkWatched(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	if err := s.db.MarkAsWatched(videoID); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIMarkUnwatched(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	if err := s.db.DeleteWatchProgress(videoID); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Import endpoints

func (s *Server) handleAPIImportURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	feedURL := strings.TrimSpace(req.URL)
	if feedURL == "" {
		jsonError(w, "URL is required", http.StatusBadRequest)
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(feedURL)
	if err != nil {
		jsonError(w, "Failed to fetch URL: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, "URL returned status: "+resp.Status, http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		jsonError(w, "Failed to read response: "+err.Error(), http.StatusBadRequest)
		return
	}

	feed, err := s.importFeedFromJSON(body, feedURL)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonResponse(w, feed)
}

func (s *Server) handleAPIImportFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		jsonError(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	body, err := io.ReadAll(io.LimitReader(file, 5*1024*1024))
	if err != nil {
		jsonError(w, "Failed to read file: "+err.Error(), http.StatusBadRequest)
		return
	}

	feed, err := s.importFeedFromJSON(body, header.Filename)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonResponse(w, feed)
}

func (s *Server) importFeedFromJSON(body []byte, source string) (*models.Feed, error) {
	// Try Feeds format first
	var feedExport models.FeedExport
	if err := json.Unmarshal(body, &feedExport); err == nil && len(feedExport.Channels) > 0 {
		tags := ""
		if len(feedExport.Tags) > 0 {
			tags = strings.Join(feedExport.Tags, ", ")
		}

		feed, err := s.db.CreateFeedWithMetadata(feedExport.Name, feedExport.Description, feedExport.Author, tags)
		if err != nil {
			return nil, err
		}

		for _, ch := range feedExport.Channels {
			if _, err := s.db.AddChannel(feed.ID, ch.URL, ch.Name); err != nil {
				log.Printf("Failed to add channel %s: %v", ch.URL, err)
			}
		}

		return feed, nil
	}

	// Try NewPipe format - add channels to Inbox
	var newPipeExport models.NewPipeExport
	if err := json.Unmarshal(body, &newPipeExport); err == nil && len(newPipeExport.Subscriptions) > 0 {
		var subs []models.NewPipeSubscription
		for _, sub := range newPipeExport.Subscriptions {
			if sub.ServiceID == 0 {
				subs = append(subs, sub)
			}
		}

		if len(subs) == 0 {
			return nil, &importError{"No YouTube subscriptions found in file"}
		}

		// Add to Inbox instead of creating a new feed
		inbox, err := s.db.GetInbox()
		if err != nil {
			return nil, err
		}

		for _, sub := range subs {
			if _, err := s.db.AddChannel(inbox.ID, sub.URL, sub.Name); err != nil {
				log.Printf("Failed to add channel %s: %v", sub.URL, err)
			}
		}

		return inbox, nil
	}

	return nil, &importError{"Unrecognized format - expected Feeds or NewPipe JSON"}
}

type importError struct {
	message string
}

func (e *importError) Error() string {
	return e.message
}

// AI organize endpoints

func (s *Server) handleAPIOrganize(w http.ResponseWriter, r *http.Request) {
	if s.ai == nil {
		jsonError(w, "AI organization is not enabled", http.StatusBadRequest)
		return
	}

	var req struct {
		Channels []struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Channels) == 0 {
		jsonError(w, "No channels to organize", http.StatusBadRequest)
		return
	}

	// Convert to NewPipeSubscription format for AI
	var subs []models.NewPipeSubscription
	for _, ch := range req.Channels {
		subs = append(subs, models.NewPipeSubscription{
			ServiceID: 0,
			URL:       ch.URL,
			Name:      ch.Name,
		})
	}

	// Call AI to organize (without metadata for simplicity)
	groups, err := s.ai.SuggestGroups(subs)
	if err != nil {
		jsonError(w, "AI organization failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]any{
		"groups": groups,
	})
}

func (s *Server) handleAPIConfirmOrganize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Groups []struct {
			Name     string   `json:"name"`
			Channels []string `json:"channels"` // URLs
		} `json:"groups"`
		ChannelNames map[string]string `json:"channelNames"` // URL -> Name
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var createdFeeds []*models.Feed

	for _, group := range req.Groups {
		if len(group.Channels) == 0 {
			continue
		}

		feed, err := s.db.CreateFeed(group.Name)
		if err != nil {
			jsonError(w, "Failed to create feed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		for _, url := range group.Channels {
			name := req.ChannelNames[url]
			if name == "" {
				name = url
			}
			if _, err := s.db.AddChannel(feed.ID, url, name); err != nil {
				log.Printf("Failed to add channel %s: %v", url, err)
			}
		}

		createdFeeds = append(createdFeeds, feed)
	}

	jsonResponse(w, map[string]any{
		"feeds": createdFeeds,
	})
}

// Watch History Import endpoints

func (s *Server) handleAPIImportWatchHistory(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 * 1024 * 1024); err != nil { // 50MB limit for large histories
		jsonError(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	body, err := io.ReadAll(io.LimitReader(file, 50*1024*1024))
	if err != nil {
		jsonError(w, "Failed to read file: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Parse watch history
	channels, totalVideos, err := parseWatchHistory(body)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]any{
		"channels":    channels,
		"totalVideos": totalVideos,
	})
}

func (s *Server) handleAPIOrganizeWatchHistory(w http.ResponseWriter, r *http.Request) {
	if s.ai == nil {
		jsonError(w, "AI organization is not enabled", http.StatusBadRequest)
		return
	}

	var req struct {
		Channels []models.WatchHistoryChannel `json:"channels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Channels) == 0 {
		jsonError(w, "No channels to organize", http.StatusBadRequest)
		return
	}

	// Convert to NewPipeSubscription format and build metadata with watch counts
	var subs []models.NewPipeSubscription
	metadata := make(map[string]ai.ChannelInfo)

	for _, ch := range req.Channels {
		subs = append(subs, models.NewPipeSubscription{
			ServiceID: 0,
			URL:       ch.URL,
			Name:      ch.Name,
		})
		metadata[ch.URL] = ai.ChannelInfo{
			Name:       ch.Name,
			URL:        ch.URL,
			WatchCount: ch.WatchCount,
		}
	}

	// Call AI with watch count metadata
	groups, err := s.ai.SuggestGroupsWithMetadata(subs, metadata)
	if err != nil {
		jsonError(w, "AI organization failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]any{
		"groups": groups,
	})
}

// parseWatchHistory extracts unique channels from YouTube watch history JSON
func parseWatchHistory(data []byte) ([]models.WatchHistoryChannel, int, error) {
	var entries []models.WatchHistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, 0, &importError{"Invalid watch history format: " + err.Error()}
	}

	if len(entries) == 0 {
		return nil, 0, &importError{"No watch history entries found"}
	}

	// Count watches per channel
	channelCounts := make(map[string]*models.WatchHistoryChannel)
	totalVideos := 0

	for _, entry := range entries {
		// Skip non-YouTube entries and entries without channel info
		if entry.Header != "YouTube" {
			continue
		}
		if len(entry.Subtitles) == 0 {
			continue
		}

		totalVideos++

		// Get channel info from subtitles
		channelURL := entry.Subtitles[0].URL
		channelName := entry.Subtitles[0].Name

		if channelURL == "" {
			continue
		}

		if existing, ok := channelCounts[channelURL]; ok {
			existing.WatchCount++
		} else {
			channelCounts[channelURL] = &models.WatchHistoryChannel{
				URL:        channelURL,
				Name:       channelName,
				WatchCount: 1,
			}
		}
	}

	if len(channelCounts) == 0 {
		return nil, 0, &importError{"No YouTube channels found in watch history"}
	}

	// Convert to slice and sort by watch count (descending)
	channels := make([]models.WatchHistoryChannel, 0, len(channelCounts))
	for _, ch := range channelCounts {
		channels = append(channels, *ch)
	}

	// Sort by watch count descending
	for i := 0; i < len(channels)-1; i++ {
		for j := i + 1; j < len(channels); j++ {
			if channels[j].WatchCount > channels[i].WatchCount {
				channels[i], channels[j] = channels[j], channels[i]
			}
		}
	}

	return channels, totalVideos, nil
}
