package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

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
		"ytdlpCookiesConfigured": s.ytdlpCookiesConfigured(),
	})
}

func (s *Server) handleAPISetYTDLPCookies(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cookies string `json:"cookies"`
		Clear   bool   `json:"clear"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cookiesPath := s.ytdlp.CookiesPath
	if cookiesPath == "" {
		jsonError(w, "Cookies path not configured", http.StatusInternalServerError)
		return
	}

	if req.Clear {
		if err := os.Remove(cookiesPath); err != nil && !os.IsNotExist(err) {
			jsonError(w, "Failed to clear cookies", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	cookies := strings.TrimSpace(req.Cookies)
	if cookies == "" {
		jsonError(w, "Cookies are required", http.StatusBadRequest)
		return
	}
	firstLine := ""
	for _, line := range strings.Split(cookies, "\n") {
		if strings.TrimSpace(line) != "" {
			firstLine = strings.TrimSpace(line)
			break
		}
	}
	if firstLine == "" {
		jsonError(w, "Cookies are required", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(firstLine, "# Netscape HTTP Cookie File") {
		cookies = "# Netscape HTTP Cookie File\n" + cookies
	}
	cookies = normalizeNetscapeCookies(cookies)

	if err := os.MkdirAll(filepath.Dir(cookiesPath), 0o755); err != nil {
		jsonError(w, "Failed to prepare cookies directory", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(cookiesPath, []byte(cookies+"\n"), 0o600); err != nil {
		jsonError(w, "Failed to save cookies", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ytdlpCookiesConfigured() bool {
	if s.ytdlp == nil || s.ytdlp.CookiesPath == "" {
		return false
	}
	info, err := os.Stat(s.ytdlp.CookiesPath)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Size() > 0
}

func normalizeNetscapeCookies(contents string) string {
	lines := strings.Split(contents, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 7 {
			continue
		}
		domain := strings.TrimSpace(parts[0])
		flag := strings.TrimSpace(parts[1])
		if strings.HasPrefix(domain, ".") && strings.EqualFold(flag, "false") {
			parts[1] = "TRUE"
		} else if !strings.HasPrefix(domain, ".") && strings.EqualFold(flag, "true") {
			parts[1] = "FALSE"
		}
		lines[i] = strings.Join(parts, "\t")
	}
	return strings.Join(lines, "\n")
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

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	videos, total, err := s.db.GetVideosByFeed(id, limit, offset)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check shorts status for any videos with null is_short
	var uncheckedIDs []string
	for _, v := range videos {
		if v.IsShort == nil {
			uncheckedIDs = append(uncheckedIDs, v.ID)
		}
	}
	if len(uncheckedIDs) > 0 {
		shortsStatus := yt.CheckShortsStatus(uncheckedIDs)
		for i := range videos {
			if isShort, ok := shortsStatus[videos[i].ID]; ok {
				videos[i].IsShort = &isShort
				s.db.UpdateVideoIsShort(videos[i].ID, isShort)
			}
		}
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
		"total":       total,
		"offset":      offset,
		"limit":       limit,
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

func (s *Server) handleAPIReorderFeeds(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeedIDs []int64 `json:"feed_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.FeedIDs) == 0 {
		jsonError(w, "feed_ids is required", http.StatusBadRequest)
		return
	}

	if err := s.db.ReorderFeeds(req.FeedIDs); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated feeds list
	feeds, err := s.db.GetFeeds()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, feeds)
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
	var allVideos []models.Video

	for res := range results {
		if res.err != nil {
			errors = append(errors, res.chName+": "+res.err.Error())
			continue
		}

		for _, v := range res.videos {
			v.ChannelID = res.chID
			allVideos = append(allVideos, v)
		}
	}

	var newVideos int

	// Check shorts status only for videos that don't already have it
	if len(allVideos) > 0 {
		videoIDs := make([]string, len(allVideos))
		for i, v := range allVideos {
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
		var newShortsStatus map[string]bool
		if len(needsCheck) > 0 {
			log.Printf("Checking shorts status for %d new videos (skipping %d with existing status)", len(needsCheck), len(existingStatus))
			newShortsStatus = yt.CheckShortsStatus(needsCheck)
		} else {
			log.Printf("All %d videos already have shorts status, skipping check", len(videoIDs))
			newShortsStatus = map[string]bool{}
		}

		// Merge existing and new status
		for id, isShort := range existingStatus {
			newShortsStatus[id] = isShort
		}
		for i := range allVideos {
			if isShort, ok := newShortsStatus[allVideos[i].ID]; ok {
				allVideos[i].IsShort = &isShort
			}
			isNew, err := s.db.UpsertVideo(&allVideos[i])
			if err != nil {
				log.Printf("Failed to save video %s: %v", allVideos[i].ID, err)
				continue
			}
			totalVideos++
			if isNew {
				newVideos++
			}
		}

		// Update new video count for this feed
		if err := s.db.UpdateNewVideoCount(id, newVideos); err != nil {
			log.Printf("Failed to update new video count for feed %d: %v", id, err)
		}
	}

	jsonResponse(w, map[string]any{
		"videosFound": totalVideos,
		"newVideos":   newVideos,
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

	// Parse pagination params
	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	channel, err := s.db.GetChannel(id)
	if err != nil {
		jsonError(w, "Channel not found", http.StatusNotFound)
		return
	}

	// Fetch one extra to determine if there are more videos
	videos, err := s.db.GetVideosByChannel(id, limit+1, offset)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hasMore := len(videos) > limit
	if hasMore {
		videos = videos[:limit]
	}

	// Get all feeds this channel belongs to
	feeds, err := s.db.GetFeedsByChannel(id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get all feeds for "add to feed" dropdown
	allFeeds, _ := s.db.GetFeeds()

	videoIDs := make([]string, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

	jsonResponse(w, map[string]any{
		"channel":     channel,
		"videos":      videos,
		"progressMap": progressMap,
		"feeds":       feeds,
		"allFeeds":    allFeeds,
		"hasMore":     hasMore,
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

	channel, isNew, err := s.db.AddChannelToFeed(feedID, channelInfo.URL, channelInfo.Name)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch initial videos only if channel is new
	if isNew {
		videos, err := yt.FetchLatestVideos(channelInfo.URL, 5)
		if err == nil && len(videos) > 0 {
			videoIDs := make([]string, len(videos))
			for i, v := range videos {
				videoIDs[i] = v.ID
			}
			shortsStatus := yt.CheckShortsStatus(videoIDs)

			for i := range videos {
				videos[i].ChannelID = channel.ID
				if isShort, ok := shortsStatus[videos[i].ID]; ok {
					videos[i].IsShort = &isShort
				}
				_, _ = s.db.UpsertVideo(&videos[i])
			}
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

func (s *Server) handleAPIRemoveChannelFromFeed(w http.ResponseWriter, r *http.Request) {
	feedID, err := strconv.ParseInt(r.PathValue("feedId"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	channelID, err := strconv.ParseInt(r.PathValue("channelId"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	deleted, err := s.db.RemoveChannelFromFeed(feedID, channelID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]any{
		"deleted": deleted,
	})
}

func (s *Server) handleAPIAddChannelToFeed(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
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

	// Get channel to get its URL
	channel, err := s.db.GetChannel(channelID)
	if err != nil {
		jsonError(w, "Channel not found", http.StatusNotFound)
		return
	}

	// Add to feed (reuses existing channel)
	_, _, err = s.db.AddChannelToFeed(req.FeedID, channel.URL, channel.Name)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated feeds list
	feeds, _ := s.db.GetFeedsByChannel(channelID)

	jsonResponse(w, map[string]any{
		"feeds": feeds,
	})
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
	if len(videos) > 0 {
		// Check shorts status before saving
		videoIDs := make([]string, len(videos))
		for i, v := range videos {
			videoIDs[i] = v.ID
		}
		shortsStatus := yt.CheckShortsStatus(videoIDs)

		for i := range videos {
			videos[i].ChannelID = channel.ID
			if isShort, ok := shortsStatus[videos[i].ID]; ok {
				videos[i].IsShort = &isShort
			}
			if _, err := s.db.UpsertVideo(&videos[i]); err != nil {
				log.Printf("Failed to save video %s: %v", videos[i].ID, err)
				continue
			}
			savedCount++
		}
	}

	jsonResponse(w, map[string]any{
		"videosFound": savedCount,
		"channel":     channel.Name,
	})
}

// handleAPIFetchMoreVideos uses yt-dlp to fetch older videos from a channel's history
// It streams progress updates via Server-Sent Events (SSE)
func (s *Server) handleAPIFetchMoreVideos(w http.ResponseWriter, r *http.Request) {
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

	// Get the current video count to determine offset
	currentCount, err := s.db.GetVideoCountByChannel(id)
	if err != nil {
		jsonError(w, "Failed to get video count", http.StatusInternalServerError)
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Helper to send SSE events
	sendEvent := func(eventType string, data any) {
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
		flusher.Flush()
	}

	// Fetch 10 videos at a time, up to 50 total
	const batchSize = 10
	const maxBatches = 5
	totalSaved := 0
	hasMore := true

	for batch := 0; batch < maxBatches && hasMore; batch++ {
		start := currentCount + (batch * batchSize) + 1
		end := start + batchSize - 1

		// Send progress update
		sendEvent("progress", map[string]any{
			"batch":      batch + 1,
			"maxBatches": maxBatches,
			"fetching":   batchSize,
			"totalSaved": totalSaved,
			"status":     "fetching",
		})

		log.Printf("Fetching videos for channel %s (ID: %d), positions %d-%d", channel.URL, id, start, end)

		videos, err := s.ytdlp.GetChannelVideos(channel.URL, start, end)
		if err != nil {
			log.Printf("Failed to fetch videos: %v", err)
			sendEvent("error", map[string]any{
				"message": "Failed to fetch videos: " + err.Error(),
			})
			return
		}

		log.Printf("yt-dlp returned %d videos in batch %d", len(videos), batch+1)

		// If we got fewer videos than requested, there are no more
		if len(videos) < batchSize {
			hasMore = false
		}

		if len(videos) == 0 {
			break
		}

		// Check shorts status before saving
		videoIDs := make([]string, len(videos))
		for i, v := range videos {
			videoIDs[i] = v.ID
		}
		shortsStatus := yt.CheckShortsStatus(videoIDs)

		batchSaved := 0
		for _, v := range videos {
			video := v.ToModel(channel.ID, channel.Name)
			if isShort, ok := shortsStatus[video.ID]; ok {
				video.IsShort = &isShort
			}
			if _, err := s.db.UpsertVideo(video); err != nil {
				log.Printf("Failed to save video %s: %v", video.ID, err)
				continue
			}
			batchSaved++
		}
		totalSaved += batchSaved

		// Send batch complete update
		sendEvent("progress", map[string]any{
			"batch":      batch + 1,
			"maxBatches": maxBatches,
			"saved":      batchSaved,
			"totalSaved": totalSaved,
			"status":     "saved",
		})
	}

	// Send final complete event
	sendEvent("complete", map[string]any{
		"videosFound": totalSaved,
		"channel":     channel.Name,
		"hasMore":     hasMore,
	})
}

// handleAPIGetChannelFeeds returns all feeds that contain a channel
func (s *Server) handleAPIGetChannelFeeds(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	feeds, err := s.db.GetFeedsByChannel(channelID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type membership struct {
		ChannelID int64  `json:"channelId"`
		FeedID    int64  `json:"feedId"`
		FeedName  string `json:"feedName"`
	}

	memberships := make([]membership, 0, len(feeds))
	for _, feed := range feeds {
		memberships = append(memberships, membership{
			ChannelID: channelID,
			FeedID:    feed.ID,
			FeedName:  feed.Name,
		})
	}

	jsonResponse(w, memberships)
}

// Video endpoints

func (s *Server) handleAPIGetRecentVideos(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	videos, total, err := s.db.GetAllRecentVideos(limit, offset)
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
		"total":       total,
		"offset":      offset,
		"limit":       limit,
	})
}

func (s *Server) handleAPIGetShuffledVideos(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	videos, total, err := s.db.GetShuffledVideosByFeed(id, limit, offset)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]any{
		"videos": videos,
		"total":  total,
		"offset": offset,
		"limit":  limit,
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

// isVideoURL checks if a URL is a YouTube video URL vs a channel URL
func isVideoURL(url string) bool {
	return strings.Contains(url, "/watch?v=") ||
		strings.Contains(url, "youtu.be/") ||
		strings.Contains(url, "/shorts/")
}

// Import endpoints

func (s *Server) handleAPIImportYouTube(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		FeedID int64  `json:"feedId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate inputs
	if strings.TrimSpace(req.URL) == "" {
		jsonError(w, "URL is required", http.StatusBadRequest)
		return
	}
	if req.FeedID == 0 {
		jsonError(w, "feedId is required", http.StatusBadRequest)
		return
	}

	// Verify feed exists
	feed, err := s.db.GetFeed(req.FeedID)
	if err != nil {
		jsonError(w, "Feed not found", http.StatusBadRequest)
		return
	}

	// Resolve to channel (detect video vs channel URL)
	var channelInfo *yt.ChannelInfo
	if isVideoURL(req.URL) {
		channelInfo, err = yt.ResolveVideoToChannel(req.URL)
		if err != nil {
			jsonError(w, "Could not resolve channel from video URL: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		channelInfo, err = yt.ResolveChannelURL(req.URL)
		if err != nil {
			jsonError(w, "Could not resolve channel from URL: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Add channel to feed
	channel, isNew, err := s.db.AddChannelToFeed(req.FeedID, channelInfo.URL, channelInfo.Name)
	if err != nil {
		jsonError(w, "Failed to add channel: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If new channel, fetch initial videos
	if isNew {
		videos, err := yt.FetchLatestVideos(channel.URL, 5)
		if err != nil {
			log.Printf("Failed to fetch initial videos for channel %s: %v", channel.URL, err)
		} else {
			// Check shorts status
			var videoIDs []string
			for _, v := range videos {
				videoIDs = append(videoIDs, v.ID)
			}
			shortsMap := yt.CheckShortsStatus(videoIDs)

			// Upsert videos
			for _, video := range videos {
				isShort := shortsMap[video.ID]
				video.IsShort = &isShort
				if _, err := s.db.UpsertVideo(&video); err != nil {
					log.Printf("Failed to upsert video %s: %v", video.ID, err)
				}
			}
		}
	}

	// Return channel and feed info
	w.WriteHeader(http.StatusCreated)
	jsonResponse(w, map[string]any{
		"channel": channel,
		"feed":    feed,
	})
}

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

// Search endpoint

func (s *Server) handleAPISearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) < 2 {
		jsonError(w, "Query must be at least 2 characters", http.StatusBadRequest)
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	// Search videos
	videos, err := s.db.SearchVideos(query, limit)
	if err != nil {
		jsonError(w, "Failed to search videos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Search channels
	channels, err := s.db.SearchChannels(query, limit)
	if err != nil {
		jsonError(w, "Failed to search channels: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]any{
		"videos":   videos,
		"channels": channels,
		"query":    query,
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
