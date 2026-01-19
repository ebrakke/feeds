# Multi-Feed Channels Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable channels to belong to multiple feeds simultaneously, with shared videos and easy management from both channel pages and feed video browsing.

**Architecture:** Many-to-many relationship between feeds and channels via junction table. Channels become unique by URL, videos stay attached to channels. Goose migrations for schema changes.

**Tech Stack:** Go backend, SQLite with goose migrations, SvelteKit frontend

---

## Task 1: Add Goose Migration Tool

**Files:**
- Modify: `go.mod`
- Modify: `internal/db/db.go`
- Create: `internal/db/migrations/001_initial_schema.sql`

**Step 1: Add goose dependency**

Run:
```bash
cd /root/code/feeds && go get github.com/pressly/goose/v3
```

**Step 2: Create migrations directory**

Run:
```bash
mkdir -p /root/code/feeds/internal/db/migrations
```

**Step 3: Create baseline migration (001_initial_schema.sql)**

Create `/root/code/feeds/internal/db/migrations/001_initial_schema.sql`:

```sql
-- +goose Up
-- This is the baseline schema, tables already exist
-- Using IF NOT EXISTS to be safe for new databases

CREATE TABLE IF NOT EXISTS feeds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    author TEXT DEFAULT '',
    tags TEXT DEFAULT '',
    is_system BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id INTEGER NOT NULL,
    url TEXT NOT NULL,
    name TEXT NOT NULL,
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
);

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

CREATE INDEX IF NOT EXISTS idx_channels_feed ON channels(feed_id);
CREATE INDEX IF NOT EXISTS idx_videos_channel ON videos(channel_id);
CREATE INDEX IF NOT EXISTS idx_videos_published ON videos(published DESC);

CREATE TABLE IF NOT EXISTS channel_metadata (
    url TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    video_titles TEXT,
    fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS watch_progress (
    video_id TEXT PRIMARY KEY,
    progress_seconds INTEGER NOT NULL DEFAULT 0,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    watched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (video_id) REFERENCES videos(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_watch_progress_watched_at ON watch_progress(watched_at DESC);

CREATE TABLE IF NOT EXISTS sponsorblock_segments (
    video_id TEXT NOT NULL,
    segment_uuid TEXT NOT NULL,
    start_time REAL NOT NULL,
    end_time REAL NOT NULL,
    category TEXT NOT NULL,
    action_type TEXT NOT NULL DEFAULT 'skip',
    votes INTEGER DEFAULT 0,
    fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (video_id, segment_uuid)
);

CREATE INDEX IF NOT EXISTS idx_sponsorblock_video ON sponsorblock_segments(video_id);

-- +goose Down
-- Cannot drop tables as this is baseline
SELECT 1;
```

**Step 4: Update db.go to use goose**

Modify `/root/code/feeds/internal/db/db.go`. Replace the `migrate()` method with goose-based migration:

```go
// Add to imports
import (
    "embed"
    "github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func (db *DB) migrate() error {
    goose.SetBaseFS(embedMigrations)

    if err := goose.SetDialect("sqlite3"); err != nil {
        return err
    }

    if err := goose.Up(db.conn, "migrations"); err != nil {
        return err
    }

    return nil
}
```

Remove the old schema string and ALTER TABLE migrations from the migrate() function.

**Step 5: Verify build**

Run:
```bash
cd /root/code/feeds && go build ./...
```
Expected: Build succeeds

**Step 6: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
feat: add goose for database migrations

Replace hand-rolled migrations with goose for versioned,
reversible database schema management.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 2: Create Multi-Feed Channels Migration

**Files:**
- Create: `internal/db/migrations/002_multi_feed_channels.sql`

**Step 1: Create the migration file**

Create `/root/code/feeds/internal/db/migrations/002_multi_feed_channels.sql`:

```sql
-- +goose Up

-- Create new deduplicated channels table
CREATE TABLE channels_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

-- Create junction table for many-to-many relationship
CREATE TABLE feed_channels (
    feed_id INTEGER NOT NULL,
    channel_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (feed_id, channel_id),
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES channels_new(id) ON DELETE CASCADE
);

CREATE INDEX idx_feed_channels_channel ON feed_channels(channel_id);

-- Migrate: insert unique channels (dedupe by URL, keep first name)
INSERT INTO channels_new (url, name)
SELECT url, name FROM channels GROUP BY url;

-- Migrate: create feed-channel links from existing relationships
INSERT INTO feed_channels (feed_id, channel_id)
SELECT c.feed_id, cn.id
FROM channels c
JOIN channels_new cn ON cn.url = c.url;

-- Create mapping table for video channel_id updates
CREATE TEMP TABLE channel_id_map AS
SELECT c.id AS old_id, cn.id AS new_id
FROM channels c
JOIN channels_new cn ON cn.url = c.url;

-- Migrate: update video references to new channel IDs
UPDATE videos SET channel_id = (
    SELECT new_id FROM channel_id_map WHERE old_id = videos.channel_id
);

-- Drop old table and rename new one
DROP TABLE channels;
ALTER TABLE channels_new RENAME TO channels;

-- +goose Down
-- Note: This migration is not fully reversible as it deduplicates channels
-- Rollback would require picking one feed per channel

-- Recreate old channels table structure
CREATE TABLE channels_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id INTEGER NOT NULL,
    url TEXT NOT NULL,
    name TEXT NOT NULL,
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
);

-- For each feed-channel relationship, create a channel entry
INSERT INTO channels_old (feed_id, url, name)
SELECT fc.feed_id, c.url, c.name
FROM feed_channels fc
JOIN channels c ON c.id = fc.channel_id;

-- Update videos (will pick arbitrary channel for videos in multiple feeds)
UPDATE videos SET channel_id = (
    SELECT co.id FROM channels_old co
    JOIN channels c ON c.url = co.url
    WHERE c.id = videos.channel_id
    LIMIT 1
);

DROP TABLE feed_channels;
DROP TABLE channels;
ALTER TABLE channels_old RENAME TO channels;
CREATE INDEX idx_channels_feed ON channels(feed_id);
```

**Step 2: Verify migration syntax**

Run:
```bash
cd /root/code/feeds && go build ./...
```
Expected: Build succeeds (migration is embedded)

**Step 3: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
feat: add multi-feed channels migration

Migration 002 transforms schema from one-to-many (channel belongs to
one feed) to many-to-many (channel can be in multiple feeds).

- Creates feed_channels junction table
- Deduplicates channels by URL
- Updates video references to new channel IDs

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 3: Update Go Models

**Files:**
- Modify: `internal/models/models.go`

**Step 1: Update Channel struct**

Modify `/root/code/feeds/internal/models/models.go`:

```go
type Channel struct {
    ID   int64  `json:"id"`
    URL  string `json:"url"`
    Name string `json:"name"`
}
```

Remove the `FeedID` field entirely.

**Step 2: Verify build**

Run:
```bash
cd /root/code/feeds && go build ./...
```
Expected: Build fails with errors about FeedID references - this is expected, we'll fix in next tasks

**Step 3: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
refactor: remove FeedID from Channel model

Channels are now independent entities that can belong to multiple
feeds via the feed_channels junction table.

Note: This breaks compilation - DB layer updates follow.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 4: Update Database Layer - Channel Queries

**Files:**
- Modify: `internal/db/db.go`

**Step 1: Update AddChannel to handle deduplication**

Replace the `AddChannel` function in `db.go`:

```go
// AddChannelToFeed adds a channel to a feed. If the channel URL doesn't exist,
// creates it first. Returns the channel and whether it was newly created.
func (db *DB) AddChannelToFeed(feedID int64, url, name string) (*models.Channel, bool, error) {
    tx, err := db.conn.Begin()
    if err != nil {
        return nil, false, err
    }
    defer tx.Rollback()

    // Check if channel exists
    var channel models.Channel
    err = tx.QueryRow(
        "SELECT id, url, name FROM channels WHERE url = ?", url,
    ).Scan(&channel.ID, &channel.URL, &channel.Name)

    isNew := false
    if err == sql.ErrNoRows {
        // Create new channel
        result, err := tx.Exec(
            "INSERT INTO channels (url, name) VALUES (?, ?)",
            url, name,
        )
        if err != nil {
            return nil, false, err
        }
        channel.ID, _ = result.LastInsertId()
        channel.URL = url
        channel.Name = name
        isNew = true
    } else if err != nil {
        return nil, false, err
    }

    // Add to feed (ignore if already exists)
    _, err = tx.Exec(
        "INSERT OR IGNORE INTO feed_channels (feed_id, channel_id) VALUES (?, ?)",
        feedID, channel.ID,
    )
    if err != nil {
        return nil, false, err
    }

    if err := tx.Commit(); err != nil {
        return nil, false, err
    }

    return &channel, isNew, nil
}

// AddChannel is a compatibility wrapper for AddChannelToFeed
func (db *DB) AddChannel(feedID int64, url, name string) (*models.Channel, error) {
    channel, _, err := db.AddChannelToFeed(feedID, url, name)
    return channel, err
}
```

**Step 2: Update GetChannelsByFeed**

Replace the `GetChannelsByFeed` function:

```go
func (db *DB) GetChannelsByFeed(feedID int64) ([]models.Channel, error) {
    rows, err := db.conn.Query(`
        SELECT c.id, c.url, c.name
        FROM channels c
        JOIN feed_channels fc ON c.id = fc.channel_id
        WHERE fc.feed_id = ?
        ORDER BY c.name
    `, feedID)
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

**Step 3: Update GetChannel**

Replace the `GetChannel` function:

```go
func (db *DB) GetChannel(channelID int64) (*models.Channel, error) {
    var c models.Channel
    err := db.conn.QueryRow(
        "SELECT id, url, name FROM channels WHERE id = ?", channelID,
    ).Scan(&c.ID, &c.URL, &c.Name)
    if err != nil {
        return nil, err
    }
    return &c, nil
}
```

**Step 4: Update GetChannelByURL**

Replace the `GetChannelByURL` function:

```go
func (db *DB) GetChannelByURL(url string) (*models.Channel, error) {
    var c models.Channel
    err := db.conn.QueryRow(
        "SELECT id, url, name FROM channels WHERE url = ?", url,
    ).Scan(&c.ID, &c.URL, &c.Name)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return &c, nil
}
```

**Step 5: Update GetChannelsByURL to return feed info**

Replace the `GetChannelsByURL` function:

```go
// ChannelWithFeeds includes the feeds a channel belongs to
type ChannelWithFeeds struct {
    models.Channel
    Feeds []models.Feed `json:"feeds"`
}

// GetChannelsByURL returns the channel and all feeds it belongs to
func (db *DB) GetChannelsByURL(url string) ([]models.Channel, error) {
    // First get the channel
    channel, err := db.GetChannelByURL(url)
    if err != nil || channel == nil {
        return nil, err
    }

    // Return as slice for backwards compatibility
    return []models.Channel{*channel}, nil
}

// GetFeedsByChannel returns all feeds that contain a channel
func (db *DB) GetFeedsByChannel(channelID int64) ([]models.Feed, error) {
    rows, err := db.conn.Query(`
        SELECT f.id, f.name, f.description, f.author, f.tags, f.is_system, f.created_at, f.updated_at
        FROM feeds f
        JOIN feed_channels fc ON f.id = fc.feed_id
        WHERE fc.channel_id = ?
        ORDER BY f.name
    `, channelID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var feeds []models.Feed
    for rows.Next() {
        var f models.Feed
        if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.CreatedAt, &f.UpdatedAt); err != nil {
            return nil, err
        }
        feeds = append(feeds, f)
    }
    return feeds, rows.Err()
}
```

**Step 6: Add RemoveChannelFromFeed**

Add new function:

```go
// RemoveChannelFromFeed removes a channel from a feed.
// If the channel has no more feeds, it and its videos are deleted.
// Returns true if the channel was completely deleted.
func (db *DB) RemoveChannelFromFeed(feedID, channelID int64) (bool, error) {
    tx, err := db.conn.Begin()
    if err != nil {
        return false, err
    }
    defer tx.Rollback()

    // Remove the feed-channel link
    _, err = tx.Exec(
        "DELETE FROM feed_channels WHERE feed_id = ? AND channel_id = ?",
        feedID, channelID,
    )
    if err != nil {
        return false, err
    }

    // Check if channel has any remaining feeds
    var count int
    err = tx.QueryRow(
        "SELECT COUNT(*) FROM feed_channels WHERE channel_id = ?",
        channelID,
    ).Scan(&count)
    if err != nil {
        return false, err
    }

    deleted := false
    if count == 0 {
        // No more feeds - delete channel (videos cascade)
        _, err = tx.Exec("DELETE FROM channels WHERE id = ?", channelID)
        if err != nil {
            return false, err
        }
        deleted = true
    }

    if err := tx.Commit(); err != nil {
        return false, err
    }

    return deleted, nil
}
```

**Step 7: Remove MoveChannel (no longer needed)**

Delete the `MoveChannel` function entirely - moving is now add + remove.

**Step 8: Update DeleteChannel**

The DeleteChannel function now only makes sense as "delete from all feeds":

```go
// DeleteChannel removes a channel completely (from all feeds)
func (db *DB) DeleteChannel(channelID int64) error {
    // CASCADE will handle feed_channels and videos
    _, err := db.conn.Exec("DELETE FROM channels WHERE id = ?", channelID)
    return err
}
```

**Step 9: Verify build**

Run:
```bash
cd /root/code/feeds && go build ./...
```
Expected: Still errors in api_handlers.go - we'll fix those next

**Step 10: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
refactor: update DB layer for multi-feed channels

- AddChannelToFeed: creates or reuses channel, adds to feed
- GetChannelsByFeed: joins through feed_channels
- RemoveChannelFromFeed: removes link, cleans up orphans
- GetFeedsByChannel: returns all feeds for a channel
- Remove MoveChannel (replaced by add/remove pattern)

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 5: Update Video Queries for Junction Table

**Files:**
- Modify: `internal/db/db.go`

**Step 1: Update GetVideosByFeed**

Replace the query to join through feed_channels:

```go
func (db *DB) GetVideosByFeed(feedID int64, limit, offset int) ([]models.Video, int, error) {
    // Get total count first
    var total int
    err := db.conn.QueryRow(`
        SELECT COUNT(*)
        FROM videos v
        JOIN channels c ON v.channel_id = c.id
        JOIN feed_channels fc ON c.id = fc.channel_id
        WHERE fc.feed_id = ?
    `, feedID).Scan(&total)
    if err != nil {
        return nil, 0, err
    }

    rows, err := db.conn.Query(`
        SELECT v.id, v.channel_id, v.title, v.channel_name, v.thumbnail, v.duration, v.is_short, v.published, v.url
        FROM videos v
        JOIN channels c ON v.channel_id = c.id
        JOIN feed_channels fc ON c.id = fc.channel_id
        WHERE fc.feed_id = ?
        ORDER BY v.published DESC
        LIMIT ? OFFSET ?
    `, feedID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var videos []models.Video
    for rows.Next() {
        var v models.Video
        var isShort sql.NullBool
        if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &isShort, &v.Published, &v.URL); err != nil {
            return nil, 0, err
        }
        if isShort.Valid {
            v.IsShort = &isShort.Bool
        }
        videos = append(videos, v)
    }
    return videos, total, rows.Err()
}
```

**Step 2: Update GetVideosWithoutDuration**

```go
func (db *DB) GetVideosWithoutDuration(feedID int64, limit int) ([]string, error) {
    rows, err := db.conn.Query(`
        SELECT v.id FROM videos v
        JOIN channels c ON v.channel_id = c.id
        JOIN feed_channels fc ON c.id = fc.channel_id
        WHERE fc.feed_id = ? AND v.duration = 0
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

**Step 3: Update DeleteVideosByFeed**

```go
func (db *DB) DeleteVideosByFeed(feedID int64) error {
    _, err := db.conn.Exec(`
        DELETE FROM videos WHERE channel_id IN (
            SELECT c.id FROM channels c
            JOIN feed_channels fc ON c.id = fc.channel_id
            WHERE fc.feed_id = ?
        )
    `, feedID)
    return err
}
```

**Step 4: Update GetShuffledVideosByFeed**

```go
func (db *DB) GetShuffledVideosByFeed(feedID int64, limit, offset int) ([]models.Video, int, error) {
    var total int
    err := db.conn.QueryRow(`
        SELECT COUNT(*)
        FROM videos v
        JOIN channels c ON v.channel_id = c.id
        JOIN feed_channels fc ON c.id = fc.channel_id
        WHERE fc.feed_id = ?
          AND (v.is_short IS NULL OR v.is_short = 0)
          AND v.id NOT IN (SELECT video_id FROM watch_progress)
    `, feedID).Scan(&total)
    if err != nil {
        return nil, 0, err
    }

    rows, err := db.conn.Query(`
        SELECT v.id, v.channel_id, v.title, v.channel_name, v.thumbnail, v.duration, v.is_short, v.published, v.url
        FROM videos v
        JOIN channels c ON v.channel_id = c.id
        JOIN feed_channels fc ON c.id = fc.channel_id
        WHERE fc.feed_id = ?
          AND (v.is_short IS NULL OR v.is_short = 0)
          AND v.id NOT IN (SELECT video_id FROM watch_progress)
        ORDER BY RANDOM()
        LIMIT ? OFFSET ?
    `, feedID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var videos []models.Video
    for rows.Next() {
        var v models.Video
        var isShort sql.NullBool
        if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &isShort, &v.Published, &v.URL); err != nil {
            return nil, 0, err
        }
        if isShort.Valid {
            v.IsShort = &isShort.Bool
        }
        videos = append(videos, v)
    }
    return videos, total, rows.Err()
}
```

**Step 5: Update GetNearbyVideos**

```go
func (db *DB) GetNearbyVideos(videoID string, limit int, offset int) ([]models.Video, int64, error) {
    // Get the video's feed(s) and published date
    // Pick the first feed if in multiple
    var feedID int64
    var published time.Time
    err := db.conn.QueryRow(`
        SELECT fc.feed_id, v.published
        FROM videos v
        JOIN channels c ON v.channel_id = c.id
        JOIN feed_channels fc ON c.id = fc.channel_id
        WHERE v.id = ?
        LIMIT 1
    `, videoID).Scan(&feedID, &published)
    if err != nil {
        return nil, 0, err
    }

    rows, err := db.conn.Query(`
        SELECT v.id, v.channel_id, v.title, v.channel_name, v.thumbnail, v.duration, v.is_short, v.published, v.url
        FROM videos v
        JOIN channels c ON v.channel_id = c.id
        JOIN feed_channels fc ON c.id = fc.channel_id
        WHERE fc.feed_id = ? AND v.published <= ? AND v.id != ? AND (v.is_short IS NULL OR v.is_short = 0)
        ORDER BY v.published DESC
        LIMIT ? OFFSET ?
    `, feedID, published, videoID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var videos []models.Video
    for rows.Next() {
        var v models.Video
        var isShort sql.NullBool
        if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &isShort, &v.Published, &v.URL); err != nil {
            return nil, 0, err
        }
        if isShort.Valid {
            v.IsShort = &isShort.Bool
        }
        videos = append(videos, v)
    }
    return videos, feedID, rows.Err()
}
```

**Step 6: Update GetVideosWithoutShortStatus**

```go
func (db *DB) GetVideosWithoutShortStatus(feedID int64, limit int) ([]string, error) {
    rows, err := db.conn.Query(`
        SELECT v.id FROM videos v
        JOIN channels c ON v.channel_id = c.id
        JOIN feed_channels fc ON c.id = fc.channel_id
        WHERE fc.feed_id = ? AND v.is_short IS NULL
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

**Step 7: Verify build**

Run:
```bash
cd /root/code/feeds && go build ./...
```

**Step 8: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
refactor: update video queries for junction table

All video queries now join through feed_channels instead of
directly referencing feed_id on channels.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 6: Update API Handlers

**Files:**
- Modify: `internal/api/api_handlers.go`
- Modify: `internal/api/handlers.go`

**Step 1: Update handleAPIGetChannel to include feeds**

In `api_handlers.go`, update `handleAPIGetChannel`:

```go
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
    })
}
```

**Step 2: Add handleAPIRemoveChannelFromFeed**

Add new handler in `api_handlers.go`:

```go
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
```

**Step 3: Add handleAPIAddChannelToFeed (for existing channels)**

Add new handler in `api_handlers.go`:

```go
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
```

**Step 4: Update handleAPIAddChannel**

The existing handler should work mostly as-is, but update to use new return:

```go
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
                s.db.UpsertVideo(&videos[i])
            }
        }
    }

    w.WriteHeader(http.StatusCreated)
    jsonResponse(w, channel)
}
```

**Step 5: Remove handleAPIMoveChannel**

Delete the `handleAPIMoveChannel` function - it's replaced by add/remove.

**Step 6: Update handleAPIDeleteChannel**

This now only deletes if removing from last feed. Update to be explicit:

```go
func (s *Server) handleAPIDeleteChannel(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
    if err != nil {
        jsonError(w, "Invalid channel ID", http.StatusBadRequest)
        return
    }

    // This deletes the channel from all feeds
    if err := s.db.DeleteChannel(id); err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
```

**Step 7: Register new routes in handlers.go**

Update `RegisterRoutes` in `handlers.go`:

```go
// Replace the channel routes section:
mux.HandleFunc("GET /api/channels/{id}", s.handleAPIGetChannel)
mux.HandleFunc("POST /api/feeds/{id}/channels", s.handleAPIAddChannel)
mux.HandleFunc("DELETE /api/feeds/{feedId}/channels/{channelId}", s.handleAPIRemoveChannelFromFeed)
mux.HandleFunc("POST /api/channels/{id}/feeds", s.handleAPIAddChannelToFeed)
mux.HandleFunc("DELETE /api/channels/{id}", s.handleAPIDeleteChannel)
mux.HandleFunc("POST /api/channels/{id}/refresh", s.handleAPIRefreshChannel)
```

Remove the move channel route:
```go
// DELETE THIS LINE:
// mux.HandleFunc("POST /api/channels/{id}/move", s.handleAPIMoveChannel)
```

**Step 8: Update getChannelMemberships**

The existing function in `handlers.go` should work but update to use new query:

```go
func (s *Server) getChannelMemberships(channelURL string) []channelMembership {
    if channelURL == "" {
        return []channelMembership{}
    }

    channel, err := s.db.GetChannelByURL(channelURL)
    if err != nil || channel == nil {
        return []channelMembership{}
    }

    feeds, err := s.db.GetFeedsByChannel(channel.ID)
    if err != nil {
        return []channelMembership{}
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
    return memberships
}
```

**Step 9: Verify build**

Run:
```bash
cd /root/code/feeds && go build ./...
```
Expected: Build succeeds

**Step 10: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
feat: update API for multi-feed channels

New endpoints:
- DELETE /api/feeds/{feedId}/channels/{channelId} - remove from feed
- POST /api/channels/{id}/feeds - add existing channel to feed

Updated endpoints:
- GET /api/channels/{id} - now includes feeds[] and allFeeds[]
- POST /api/feeds/{id}/channels - reuses existing channels

Removed:
- POST /api/channels/{id}/move - replaced by add/remove

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 7: Update Frontend Types

**Files:**
- Modify: `web/frontend/src/lib/types.ts`

**Step 1: Update Channel interface**

```typescript
export interface Channel {
    id: number;
    url: string;
    name: string;
}
```

Remove `feed_id` from the interface.

**Step 2: Verify frontend compiles**

Run:
```bash
cd /root/code/feeds/web/frontend && npm run check
```
Expected: Errors about feed_id references - we'll fix in next tasks

**Step 3: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
refactor: remove feed_id from Channel type

Channels are now independent entities that can belong to multiple feeds.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 8: Update Frontend API Functions

**Files:**
- Modify: `web/frontend/src/lib/api.ts`

**Step 1: Update getChannel response type**

```typescript
export async function getChannel(id: number): Promise<{
    channel: Channel;
    videos: Video[];
    progressMap: Record<string, WatchProgress>;
    feeds: Feed[];
    allFeeds: Feed[];
}> {
    return fetchJSON(`/channels/${id}`);
}
```

**Step 2: Add removeChannelFromFeed**

```typescript
export async function removeChannelFromFeed(feedId: number, channelId: number): Promise<{ deleted: boolean }> {
    return fetchJSON(`/feeds/${feedId}/channels/${channelId}`, { method: 'DELETE' });
}
```

**Step 3: Add addChannelToFeed**

```typescript
export async function addChannelToFeed(channelId: number, feedId: number): Promise<{ feeds: Feed[] }> {
    return fetchJSON(`/channels/${channelId}/feeds`, {
        method: 'POST',
        body: JSON.stringify({ feedId })
    });
}
```

**Step 4: Remove moveChannel function**

Delete the `moveChannel` function - it's replaced by add/remove.

**Step 5: Verify frontend compiles**

Run:
```bash
cd /root/code/feeds/web/frontend && npm run check
```

**Step 6: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
feat: update frontend API for multi-feed channels

- getChannel now returns feeds[] and allFeeds[]
- Add removeChannelFromFeed(feedId, channelId)
- Add addChannelToFeed(channelId, feedId)
- Remove moveChannel (replaced by add/remove)

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 9: Update Channel Page with Feed Chips

**Files:**
- Modify: `web/frontend/src/routes/channels/[id]/+page.svelte`

**Step 1: Update script to handle feeds**

Replace the script section:

```svelte
<script lang="ts">
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { getChannel, refreshChannel, addChannelToFeed, removeChannelFromFeed } from '$lib/api';
    import type { Channel, Video, WatchProgress, Feed } from '$lib/types';
    import VideoGrid from '$lib/components/VideoGrid.svelte';

    let channel = $state<Channel | null>(null);
    let videos = $state<Video[]>([]);
    let progressMap = $state<Record<string, WatchProgress>>({});
    let feeds = $state<Feed[]>([]);
    let allFeeds = $state<Feed[]>([]);
    let loading = $state(true);
    let refreshing = $state(false);
    let error = $state<string | null>(null);
    let showAddDropdown = $state(false);
    let addingToFeed = $state(false);
    let removingFromFeed = $state<number | null>(null);

    let id = $derived(parseInt($page.params.id));
    let scrollRestoreKey = $derived(`channel-${id}-last-video`);
    let availableFeeds = $derived(allFeeds.filter(f => !feeds.some(cf => cf.id === f.id)));

    onMount(async () => {
        await loadChannel();
    });

    async function loadChannel() {
        loading = true;
        error = null;
        try {
            const data = await getChannel(id);
            channel = data.channel;
            videos = data.videos;
            progressMap = data.progressMap || {};
            feeds = data.feeds || [];
            allFeeds = data.allFeeds || [];
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to load channel';
        } finally {
            loading = false;
        }
    }

    async function handleRefresh() {
        refreshing = true;
        try {
            await refreshChannel(id);
            await loadChannel();
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to refresh';
        } finally {
            refreshing = false;
        }
    }

    async function handleAddToFeed(feedId: number) {
        addingToFeed = true;
        try {
            const result = await addChannelToFeed(id, feedId);
            feeds = result.feeds;
            showAddDropdown = false;
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to add to feed';
        } finally {
            addingToFeed = false;
        }
    }

    async function handleRemoveFromFeed(feedId: number) {
        // Confirm if this is the last feed
        if (feeds.length === 1) {
            if (!confirm('This will delete the channel and all its videos. Continue?')) {
                return;
            }
        }

        removingFromFeed = feedId;
        try {
            await removeChannelFromFeed(feedId, id);
            feeds = feeds.filter(f => f.id !== feedId);

            // If no more feeds, redirect to home
            if (feeds.length === 0) {
                window.location.href = '/';
            }
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to remove from feed';
        } finally {
            removingFromFeed = null;
        }
    }
</script>
```

**Step 2: Add feed chips UI**

Update the header section after the channel name/URL:

```svelte
{:else if channel}
    <!-- Channel Header -->
    <header class="mb-6 animate-fade-up" style="opacity: 0;">
        <div class="flex items-start justify-between gap-4">
            <div class="flex items-center gap-4">
                <div class="w-14 h-14 rounded-2xl bg-gradient-to-br from-emerald-500/20 to-crimson-500/20 flex items-center justify-center border border-white/5">
                    <svg class="w-7 h-7 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                        <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
                        <circle cx="9" cy="7" r="4"/>
                        <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
                        <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
                    </svg>
                </div>
                <div>
                    <h1 class="text-2xl font-display font-bold mb-1">{channel.name}</h1>
                    <div class="flex items-center gap-3 text-sm">
                        <span class="text-text-muted">{videos.length} videos</span>
                        <span class="text-text-dim">·</span>
                        <a
                            href={channel.url}
                            target="_blank"
                            rel="noopener"
                            class="text-text-secondary hover:text-emerald-400 transition-colors inline-flex items-center gap-1"
                        >
                            View on YouTube
                            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
                                <polyline points="15 3 21 3 21 9"/>
                                <line x1="10" y1="14" x2="21" y2="3"/>
                            </svg>
                        </a>
                    </div>
                </div>
            </div>

            <button
                onclick={handleRefresh}
                disabled={refreshing}
                class="btn btn-primary shrink-0"
            >
                {#if refreshing}
                    <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
                    </svg>
                    Refreshing...
                {:else}
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="23 4 23 10 17 10"/>
                        <polyline points="1 20 1 14 7 14"/>
                        <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                    </svg>
                    Refresh
                {/if}
            </button>
        </div>

        <!-- Feed Chips -->
        <div class="mt-4 flex flex-wrap items-center gap-2">
            <span class="text-sm text-text-muted">Feeds:</span>
            {#each feeds as feed (feed.id)}
                <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-surface border border-white/5 text-sm">
                    <a href="/feeds/{feed.id}" class="hover:text-emerald-400 transition-colors">
                        {feed.name}
                    </a>
                    <button
                        onclick={() => handleRemoveFromFeed(feed.id)}
                        disabled={removingFromFeed === feed.id}
                        class="text-text-muted hover:text-crimson-400 transition-colors"
                        title="Remove from this feed"
                    >
                        {#if removingFromFeed === feed.id}
                            <svg class="w-3.5 h-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                            </svg>
                        {:else}
                            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M18 6L6 18M6 6l12 12"/>
                            </svg>
                        {/if}
                    </button>
                </span>
            {/each}

            <!-- Add to Feed Button -->
            <div class="relative">
                <button
                    onclick={() => showAddDropdown = !showAddDropdown}
                    disabled={availableFeeds.length === 0}
                    class="inline-flex items-center gap-1 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-sm text-emerald-400 hover:bg-emerald-500/20 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M12 5v14M5 12h14"/>
                    </svg>
                    Add to feed
                </button>

                {#if showAddDropdown && availableFeeds.length > 0}
                    <div class="absolute top-full left-0 mt-1 w-48 bg-surface border border-white/10 rounded-lg shadow-xl z-20">
                        {#each availableFeeds as feed (feed.id)}
                            <button
                                onclick={() => handleAddToFeed(feed.id)}
                                disabled={addingToFeed}
                                class="w-full px-4 py-2 text-left text-sm hover:bg-white/5 transition-colors first:rounded-t-lg last:rounded-b-lg"
                            >
                                {feed.name}
                            </button>
                        {/each}
                    </div>
                {/if}
            </div>
        </div>
    </header>
```

**Step 3: Close dropdown on outside click**

Add to script section:

```typescript
function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.relative')) {
        showAddDropdown = false;
    }
}
```

Add to component:

```svelte
<svelte:window on:click={handleClickOutside} />
```

**Step 4: Verify frontend compiles**

Run:
```bash
cd /root/code/feeds/web/frontend && npm run check
```

**Step 5: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
feat: add feed chips to channel page

Channel page now shows all feeds the channel belongs to as chips.
Users can:
- Click chip to visit feed
- Click X to remove from feed (with confirmation if last)
- Click "+ Add to feed" to add to another feed

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 10: Update VideoCard for Remove From Feed

**Files:**
- Modify: `web/frontend/src/lib/components/VideoCard.svelte`

**Step 1: Update props and imports**

Update the Props interface:

```typescript
interface Props {
    video: Video;
    progress?: WatchProgress;
    showChannel?: boolean;
    showRemoveFromFeed?: boolean;
    currentFeedId?: number;
    onChannelRemovedFromFeed?: () => void;
    onVideoClick?: () => void;
}
```

Update the destructuring:

```typescript
let {
    video,
    progress,
    showChannel = true,
    showRemoveFromFeed = false,
    currentFeedId,
    onChannelRemovedFromFeed,
    onVideoClick
}: Props = $props();
```

**Step 2: Add menu state and handler**

```typescript
let showMenu = $state(false);
let removingFromFeed = $state(false);

async function handleRemoveFromFeed(e: Event) {
    e.preventDefault();
    e.stopPropagation();
    if (!currentFeedId || removingFromFeed) return;

    removingFromFeed = true;
    try {
        await removeChannelFromFeed(currentFeedId, video.channel_id);
        showMenu = false;
        onChannelRemovedFromFeed?.();
    } catch (err) {
        console.error('Failed to remove channel from feed:', err);
        alert('Failed to remove channel');
    } finally {
        removingFromFeed = false;
    }
}

function toggleMenu(e: Event) {
    e.preventDefault();
    e.stopPropagation();
    showMenu = !showMenu;
}

function closeMenu() {
    showMenu = false;
}
```

Add import:

```typescript
import { removeChannelFromFeed } from '$lib/api';
```

**Step 3: Add menu UI**

Replace the Action Buttons section:

```svelte
<!-- Action Menu -->
{#if showRemoveFromFeed && currentFeedId}
    <button
        onclick={toggleMenu}
        class="absolute top-2 right-2 p-2 rounded-lg bg-void/80 backdrop-blur-sm text-text-secondary hover:text-text-primary hover:bg-white/10 transition-all opacity-0 group-hover:opacity-100 z-20"
        title="More options"
    >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
            <circle cx="12" cy="5" r="2"/>
            <circle cx="12" cy="12" r="2"/>
            <circle cx="12" cy="19" r="2"/>
        </svg>
    </button>

    {#if showMenu}
        <div class="dropdown top-12 right-2 z-30 w-56">
            <a
                href={video.url}
                target="_blank"
                rel="noopener"
                class="dropdown-item"
                onclick={() => showMenu = false}
            >
                <svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
                    <polyline points="15 3 21 3 21 9"/>
                    <line x1="10" y1="14" x2="21" y2="3"/>
                </svg>
                Open in YouTube
            </a>
            <div class="border-t border-white/10 my-1"></div>
            <button
                onclick={handleRemoveFromFeed}
                disabled={removingFromFeed}
                class="dropdown-item text-crimson-400 hover:bg-crimson-500/10"
            >
                {#if removingFromFeed}
                    <svg class="w-4 h-4 mr-2 animate-spin" viewBox="0 0 24 24" fill="none">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                    </svg>
                {:else}
                    <svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M18 6L6 18M6 6l12 12"/>
                    </svg>
                {/if}
                Remove channel from feed
            </button>
        </div>
    {/if}
{/if}
```

Add to article tag: `onmouseleave={closeMenu}`

**Step 4: Remove old move/remove action code**

Remove the old `showMoveAction`, `showRemoveAction`, `availableFeeds`, `onChannelMoved`, `onChannelRemoved` props and related code.

**Step 5: Verify frontend compiles**

Run:
```bash
cd /root/code/feeds/web/frontend && npm run check
```

**Step 6: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
feat: add remove from feed menu to VideoCard

VideoCard now has a three-dot menu that appears on hover with:
- Open in YouTube
- Remove channel from this feed

The old move/remove action props are replaced with
showRemoveFromFeed and currentFeedId.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 11: Update Feed Page to Use New VideoCard Props

**Files:**
- Modify: `web/frontend/src/routes/feeds/[id]/+page.svelte`
- Modify: `web/frontend/src/lib/components/VideoGrid.svelte`

**Step 1: Update VideoGrid props**

In `VideoGrid.svelte`, update the Props interface to pass through the new props:

```typescript
interface Props {
    videos: Video[];
    progressMap: Record<string, WatchProgress>;
    showChannel?: boolean;
    showRemoveFromFeed?: boolean;
    currentFeedId?: number;
    onChannelRemovedFromFeed?: (channelId: number) => void;
    scrollRestoreKey?: string;
}
```

Pass them to VideoCard:

```svelte
<VideoCard
    {video}
    progress={progressMap[video.id]}
    {showChannel}
    {showRemoveFromFeed}
    {currentFeedId}
    onChannelRemovedFromFeed={() => onChannelRemovedFromFeed?.(video.channel_id)}
    onVideoClick={saveScrollPosition}
/>
```

**Step 2: Update feed page to handle removal**

In the feed page, add handler and pass props:

```typescript
function handleChannelRemovedFromFeed(channelId: number) {
    // Remove all videos from this channel from the view
    videos = videos.filter(v => v.channel_id !== channelId);
    // Remove from channels list
    channels = channels.filter(c => c.id !== channelId);
}
```

Pass to VideoGrid:

```svelte
<VideoGrid
    {videos}
    {progressMap}
    showChannel={true}
    showRemoveFromFeed={true}
    currentFeedId={feed?.id}
    onChannelRemovedFromFeed={handleChannelRemovedFromFeed}
    {scrollRestoreKey}
/>
```

**Step 3: Remove old move/remove logic**

Remove the old `showMoveAction`, `showRemoveAction`, `availableFeeds`, `moveTargetFeeds` logic from the feed page.

**Step 4: Verify frontend compiles**

Run:
```bash
cd /root/code/feeds/web/frontend && npm run check
```

**Step 5: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
feat: wire up remove from feed in feed page

Feed page now passes showRemoveFromFeed and currentFeedId to VideoGrid.
When a channel is removed, all its videos disappear from the view.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Task 12: Test and Verify

**Step 1: Build backend**

Run:
```bash
cd /root/code/feeds && go build ./...
```
Expected: Build succeeds

**Step 2: Build frontend**

Run:
```bash
cd /root/code/feeds/web/frontend && npm run build
```
Expected: Build succeeds

**Step 3: Run smoke tests**

Run:
```bash
cd /root/code/feeds && make smoke
```
Expected: Tests pass (or identify issues to fix)

**Step 4: Manual testing**

Start the app:
```bash
cd /root/code/feeds && make dev
```

Test scenarios:
1. Visit a channel page - should see feed chips
2. Add channel to another feed - chip appears
3. Remove channel from feed - chip disappears
4. Browse feed videos - three-dot menu appears on hover
5. Click "Remove channel from feed" - videos disappear
6. Add same channel URL to different feed - should reuse channel

**Step 5: Final commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
feat: complete multi-feed channel management

Channels can now belong to multiple feeds:
- Channel page shows feed chips with add/remove
- Feed video browsing has remove channel menu
- Videos are shared across feeds (single source of truth)
- Watch progress syncs across all feeds

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>
EOF
)"
```

---

## Summary of Changes

### Database
- Added goose migrations
- Created `feed_channels` junction table
- Channels now unique by URL
- Videos shared across feeds

### Backend API
- `GET /api/channels/{id}` returns feeds[] and allFeeds[]
- `DELETE /api/feeds/{feedId}/channels/{channelId}` removes from feed
- `POST /api/channels/{id}/feeds` adds to feed
- Removed `POST /api/channels/{id}/move`

### Frontend
- Channel page has feed chips with add/remove
- VideoCard has three-dot menu with "Remove channel from feed"
- Videos disappear immediately when channel removed from feed
