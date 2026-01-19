# YouTube Shorts Detection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reliably detect and filter YouTube Shorts with persistent storage that survives feed refreshes.

**Architecture:** Add `is_short` boolean column to videos table. Fix duration preservation bug in UpsertVideo. Check shorts status via URL HEAD request during background processing (similar to duration fetching). Frontend uses `is_short` flag instead of duration heuristics.

**Tech Stack:** Go (backend), SQLite (database), SvelteKit (frontend)

---

## Task 1: Fix Duration Preservation Bug

**Files:**
- Modify: `internal/db/db.go:387-398`

**Step 1: Update UpsertVideo to preserve existing duration**

Change the ON CONFLICT clause to only update duration if the new value is non-zero:

```go
func (db *DB) UpsertVideo(v *models.Video) error {
	_, err := db.conn.Exec(`
		INSERT INTO videos (id, channel_id, title, channel_name, thumbnail, duration, published, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			channel_id = excluded.channel_id,
			title = excluded.title,
			thumbnail = excluded.thumbnail,
			duration = CASE
				WHEN excluded.duration > 0 THEN excluded.duration
				ELSE videos.duration
			END
	`, v.ID, v.ChannelID, v.Title, v.ChannelName, v.Thumbnail, v.Duration, v.Published, v.URL)
	return err
}
```

**Step 2: Verify manually**

Run `make dev`, refresh a feed, check that durations are preserved.

**Step 3: Commit**

```bash
git add internal/db/db.go
git commit -m "fix: preserve video duration on feed refresh"
```

---

## Task 2: Add is_short Column to Database

**Files:**
- Modify: `internal/db/db.go` (schema and queries)
- Modify: `internal/models/models.go`

**Step 1: Add is_short to Video model**

In `internal/models/models.go`, add the field:

```go
type Video struct {
	ID          string    `json:"id"`
	ChannelID   int64     `json:"channel_id"`
	Title       string    `json:"title"`
	ChannelName string    `json:"channel_name"`
	Thumbnail   string    `json:"thumbnail"`
	Duration    int       `json:"duration"`
	IsShort     *bool     `json:"is_short"` // nil = unknown, true = short, false = not short
	Published   time.Time `json:"published"`
	URL         string    `json:"url"`
}
```

**Step 2: Add column to schema and migration**

In `internal/db/db.go`, update the schema (around line 54-64):

```go
CREATE TABLE IF NOT EXISTS videos (
	id TEXT PRIMARY KEY,
	channel_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	channel_name TEXT NOT NULL,
	thumbnail TEXT,
	duration INTEGER,
	is_short INTEGER,
	published DATETIME,
	url TEXT NOT NULL,
	FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE
);
```

Add migration in `initSchema()` after the CREATE TABLE statements:

```go
// Migration: add is_short column if it doesn't exist
_, _ = db.conn.Exec(`ALTER TABLE videos ADD COLUMN is_short INTEGER`)
```

**Step 3: Update all SELECT queries to include is_short**

Update these functions to select and scan `is_short`:
- `GetVideosByFeed` (line ~414)
- `GetAllRecentVideos` (line ~439)
- `GetVideosByChannel` (line ~509)
- `GetVideoByID` (line ~656)
- Any other queries selecting from videos

Example scan pattern:
```go
var isShort sql.NullBool
// In Scan: &isShort
// After scan:
if isShort.Valid {
	v.IsShort = &isShort.Bool
}
```

**Step 4: Update UpsertVideo to handle is_short**

```go
func (db *DB) UpsertVideo(v *models.Video) error {
	var isShort *int
	if v.IsShort != nil {
		val := 0
		if *v.IsShort {
			val = 1
		}
		isShort = &val
	}

	_, err := db.conn.Exec(`
		INSERT INTO videos (id, channel_id, title, channel_name, thumbnail, duration, is_short, published, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			channel_id = excluded.channel_id,
			title = excluded.title,
			thumbnail = excluded.thumbnail,
			duration = CASE
				WHEN excluded.duration > 0 THEN excluded.duration
				ELSE videos.duration
			END,
			is_short = CASE
				WHEN excluded.is_short IS NOT NULL THEN excluded.is_short
				ELSE videos.is_short
			END
	`, v.ID, v.ChannelID, v.Title, v.ChannelName, v.Thumbnail, v.Duration, isShort, v.Published, v.URL)
	return err
}
```

**Step 5: Add UpdateVideoIsShort function**

```go
// UpdateVideoIsShort updates the is_short flag for a video
func (db *DB) UpdateVideoIsShort(videoID string, isShort bool) error {
	val := 0
	if isShort {
		val = 1
	}
	_, err := db.conn.Exec(`UPDATE videos SET is_short = ? WHERE id = ?`, val, videoID)
	return err
}

// GetVideosWithoutShortStatus returns video IDs that have is_short = NULL
func (db *DB) GetVideosWithoutShortStatus(feedID int64, limit int) ([]string, error) {
	rows, err := db.conn.Query(`
		SELECT v.id FROM videos v
		JOIN channels c ON v.channel_id = c.id
		WHERE c.feed_id = ? AND v.is_short IS NULL
		ORDER BY v.published DESC
		LIMIT ?
	`, feedID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
```

**Step 6: Commit**

```bash
git add internal/db/db.go internal/models/models.go
git commit -m "feat: add is_short column to videos table"
```

---

## Task 3: Add Shorts Detection to YouTube Package

**Files:**
- Modify: `internal/youtube/rss.go`

**Step 1: Add batch shorts checking function**

Add this function to `internal/youtube/rss.go`:

```go
// CheckShortsStatus checks multiple video IDs and returns a map of videoID -> isShort
// Uses concurrent requests with a limit to avoid overwhelming the server
func CheckShortsStatus(videoIDs []string) map[string]bool {
	results := make(map[string]bool)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent requests
	sem := make(chan struct{}, 5)

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, videoID := range videoIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			shortsURL := fmt.Sprintf("https://www.youtube.com/shorts/%s", id)
			resp, err := client.Head(shortsURL)
			if err != nil {
				return // Skip on error, will retry later
			}
			resp.Body.Close()

			mu.Lock()
			results[id] = (resp.StatusCode == 200)
			mu.Unlock()
		}(videoID)
	}

	wg.Wait()
	return results
}
```

Add `"sync"` to imports.

**Step 2: Commit**

```bash
git add internal/youtube/rss.go
git commit -m "feat: add batch shorts status checking"
```

---

## Task 4: Add Background Shorts Detection to API

**Files:**
- Modify: `internal/api/handlers.go`

**Step 1: Add fetchMissingShortsStatus function**

Add after `fetchMissingDurations` function (around line 742):

```go
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
```

**Step 2: Call it after feed refresh**

In `handleRefreshFeed` (around line 689), add the call after `fetchMissingDurations`:

```go
// Fetch durations for videos that don't have them (in background)
go s.fetchMissingDurations(feedID)

// Check shorts status for videos that don't have it (in background)
go s.fetchMissingShortsStatus(feedID)
```

**Step 3: Commit**

```bash
git add internal/api/handlers.go
git commit -m "feat: add background shorts status detection on feed refresh"
```

---

## Task 5: Update Frontend to Use is_short Flag

**Files:**
- Modify: `web/frontend/src/routes/feeds/[id]/+page.svelte`
- Modify: `web/frontend/src/routes/all/+page.svelte` (if it has similar logic)

**Step 1: Update shorts filtering logic in feed page**

In `web/frontend/src/routes/feeds/[id]/+page.svelte`, change lines 27-29:

```typescript
// Filter shorts - use is_short flag if available, fall back to duration heuristic
let shortsVideos = $derived(videos.filter(v =>
	v.is_short === true || (v.is_short === null && v.duration > 0 && v.duration < 90)
));
let regularVideos = $derived(videos.filter(v =>
	v.is_short === false || (v.is_short === null && (v.duration === 0 || v.duration >= 90))
));
```

**Step 2: Check and update all/+page.svelte if needed**

Check if `web/frontend/src/routes/all/+page.svelte` has similar filtering and update it the same way.

**Step 3: Test manually**

- Run `make dev`
- Refresh a feed
- Check that shorts tab shows correctly
- Verify shorts don't appear in Videos tab after status is detected

**Step 4: Commit**

```bash
git add web/frontend/src/routes/feeds/[id]/+page.svelte web/frontend/src/routes/all/+page.svelte
git commit -m "feat: use is_short flag for shorts filtering in frontend"
```

---

## Task 6: Final Testing and Cleanup

**Step 1: End-to-end test**

1. Delete the database to start fresh: `rm feeds.db`
2. Run `make dev`
3. Add a channel known to have shorts
4. Refresh the feed
5. Wait for background processing
6. Verify:
   - Shorts appear in Shorts tab
   - Shorts don't appear in Videos tab
   - Refreshing again preserves the detection

**Step 2: Final commit**

```bash
git add -A
git commit -m "feat: reliable YouTube Shorts detection with persistent storage

- Fix bug where duration was reset on feed refresh
- Add is_short column to track shorts status permanently
- Detect shorts via URL check (youtube.com/shorts/{id} returns 200)
- Background processing similar to duration fetching
- Frontend uses is_short flag with duration fallback"
```

---

## Summary

This implementation:
1. **Fixes the duration reset bug** - existing durations are preserved on refresh
2. **Adds persistent shorts detection** - `is_short` flag stored in database
3. **Uses reliable URL-based detection** - HEAD request to `/shorts/{id}` is ~99% accurate
4. **Runs in background** - doesn't slow down feed refresh
5. **Graceful fallback** - frontend falls back to duration heuristic if `is_short` is null
