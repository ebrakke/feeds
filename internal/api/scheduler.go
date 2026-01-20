package api

import (
	"log"
	"time"

	"github.com/erik/feeds/internal/db"
	"github.com/erik/feeds/internal/youtube"
)

const (
	refreshInterval = 4 * time.Hour
)

// VideoRefreshScheduler handles periodic video refresh for all channels
type VideoRefreshScheduler struct {
	db *db.DB
}

// NewVideoRefreshScheduler creates and starts a new video refresh scheduler
func NewVideoRefreshScheduler(database *db.DB) *VideoRefreshScheduler {
	s := &VideoRefreshScheduler{db: database}
	go s.run()
	return s
}

func (s *VideoRefreshScheduler) run() {
	// Run immediately on startup
	s.refreshAllChannels()

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.refreshAllChannels()
	}
}

func (s *VideoRefreshScheduler) refreshAllChannels() {
	channels, err := s.db.GetAllChannels()
	if err != nil {
		log.Printf("[scheduler] Failed to get channels: %v", err)
		return
	}

	if len(channels) == 0 {
		log.Printf("[scheduler] No channels to refresh")
		return
	}

	log.Printf("[scheduler] Starting refresh for %d channels", len(channels))
	startTime := time.Now()

	var totalVideos int
	var errorCount int

	for i, ch := range channels {
		log.Printf("[scheduler] Refreshing channel %d/%d: %s", i+1, len(channels), ch.Name)

		videos, err := youtube.FetchLatestVideos(ch.URL, 5)
		if err != nil {
			log.Printf("[scheduler] Failed to fetch videos for %s: %v", ch.Name, err)
			errorCount++
			continue
		}

		if len(videos) == 0 {
			continue
		}

		// Check shorts status for new videos
		videoIDs := make([]string, len(videos))
		for j, v := range videos {
			videoIDs[j] = v.ID
		}

		// Get existing shorts status to avoid re-checking
		existingStatus, err := s.db.GetVideoShortsStatus(videoIDs)
		if err != nil {
			log.Printf("[scheduler] Failed to get existing shorts status: %v", err)
			existingStatus = map[string]bool{}
		}

		// Only check shorts for videos without status
		var needsCheck []string
		for _, id := range videoIDs {
			if _, hasStatus := existingStatus[id]; !hasStatus {
				needsCheck = append(needsCheck, id)
			}
		}

		var shortsStatus map[string]bool
		if len(needsCheck) > 0 {
			shortsStatus = youtube.CheckShortsStatus(needsCheck)
		} else {
			shortsStatus = map[string]bool{}
		}

		// Merge existing status
		for id, isShort := range existingStatus {
			shortsStatus[id] = isShort
		}

		// Save videos
		for j := range videos {
			videos[j].ChannelID = ch.ID
			if isShort, ok := shortsStatus[videos[j].ID]; ok {
				videos[j].IsShort = &isShort
			}
			if err := s.db.UpsertVideo(&videos[j]); err != nil {
				log.Printf("[scheduler] Failed to save video %s: %v", videos[j].ID, err)
				continue
			}
			totalVideos++
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("[scheduler] Refresh complete: %d videos saved, %d errors, took %v", totalVideos, errorCount, elapsed)
}
