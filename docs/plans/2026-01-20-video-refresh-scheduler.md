# Video Refresh Scheduler Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a background goroutine that refreshes all channels every 4 hours, fetching latest videos and checking shorts status.

**Architecture:** In-process goroutine with ticker loop. Reuses existing `youtube.FetchLatestVideos()` and `youtube.CheckShortsStatus()` functions. Scheduler lives in `internal/api/scheduler.go` and is started from `cmd/server/main.go`.

**Tech Stack:** Go standard library (time.Ticker), existing youtube package

---

### Task 1: Add GetAllChannels DB function

**Files:**
- Modify: `/root/code/feeds/internal/db/db.go`

**Step 1: Add GetAllChannels function after GetChannelsByFeed (around line 296)**

Add this function to get all channels from the database:

```go
// GetAllChannels returns all channels in the database
func (db *DB) GetAllChannels() ([]models.Channel, error) {
	rows, err := db.conn.Query(`
		SELECT id, url, name FROM channels ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.Channel
	for rows.Next() {
		var c models.Channel
		if err := rows.Scan(&c.ID, &c.URL, &c.Name); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}
```

**Step 2: Verify the function compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/db/db.go
git commit -m "feat: add GetAllChannels DB function for scheduler"
```

---

### Task 2: Create scheduler.go

**Files:**
- Create: `/root/code/feeds/internal/api/scheduler.go`

**Step 1: Create the scheduler file**

```go
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
```

**Step 2: Verify it compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/api/scheduler.go
git commit -m "feat: add video refresh scheduler (4-hour interval)"
```

---

### Task 3: Integrate scheduler into main.go

**Files:**
- Modify: `/root/code/feeds/cmd/server/main.go`

**Step 1: Start scheduler after server creation (around line 134)**

After the server is created and before the mux routes, add:

```go
	// Start background video refresh scheduler
	_ = api.NewVideoRefreshScheduler(database)
```

**Step 2: Verify it compiles and runs**

Run: `cd /root/code/feeds && go build ./... && ./feeds -h`
Expected: No errors, help text displays

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: start video refresh scheduler on server startup"
```

---

### Task 4: Test the scheduler manually

**Step 1: Run the server and verify scheduler starts**

Run: `cd /root/code/feeds && make dev`

Expected: Logs show:
- `[scheduler] Starting refresh for N channels` (immediately on startup)
- `[scheduler] Refreshing channel 1/N: ChannelName`
- `[scheduler] Refresh complete: X videos saved, Y errors, took Zs`

**Step 2: Verify no errors in the logs**

The scheduler should complete its first run without panics or critical errors.
