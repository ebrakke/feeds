package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/erik/feeds/internal/models"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

var ErrSystemFeed = errors.New("cannot delete system feed")

type DB struct {
	conn *sql.DB
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for concurrent reads/writes
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}
	// Wait up to 5 seconds for locks instead of failing immediately
	if _, err := conn.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

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

func (db *DB) Close() error {
	return db.conn.Close()
}

// Inbox operations

// EnsureInboxExists creates the Inbox system feed if it doesn't exist
func (db *DB) EnsureInboxExists() (*models.Feed, error) {
	// Check if Inbox already exists
	var f models.Feed
	err := db.conn.QueryRow(
		"SELECT id, name, description, author, tags, is_system, sort_order, new_video_count, created_at, updated_at FROM feeds WHERE is_system = TRUE AND name = 'Inbox'",
	).Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt)
	if err == nil {
		return &f, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create Inbox - get max sort_order first
	var maxOrder int
	if err := db.conn.QueryRow("SELECT COALESCE(MAX(sort_order), -1) FROM feeds").Scan(&maxOrder); err != nil {
		return nil, err
	}

	now := time.Now()
	result, err := db.conn.Exec(
		"INSERT INTO feeds (name, description, author, tags, is_system, sort_order, created_at, updated_at) VALUES ('Inbox', '', '', '', TRUE, ?, ?, ?)",
		maxOrder+1, now, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.Feed{
		ID:        id,
		Name:      "Inbox",
		IsSystem:  true,
		SortOrder: maxOrder + 1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetInbox returns the Inbox system feed
func (db *DB) GetInbox() (*models.Feed, error) {
	var f models.Feed
	err := db.conn.QueryRow(
		"SELECT id, name, description, author, tags, is_system, sort_order, new_video_count, created_at, updated_at FROM feeds WHERE is_system = TRUE AND name = 'Inbox'",
	).Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// Feed operations

func (db *DB) CreateFeed(name string) (*models.Feed, error) {
	return db.CreateFeedWithMetadata(name, "", "", "")
}

func (db *DB) CreateFeedWithMetadata(name, description, author, tags string) (*models.Feed, error) {
	// Get max sort_order to put new feed at end
	var maxOrder int
	if err := db.conn.QueryRow("SELECT COALESCE(MAX(sort_order), -1) FROM feeds").Scan(&maxOrder); err != nil {
		return nil, err
	}

	now := time.Now()
	result, err := db.conn.Exec(
		"INSERT INTO feeds (name, description, author, tags, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		name, description, author, tags, maxOrder+1, now, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.Feed{
		ID:          id,
		Name:        name,
		Description: description,
		Author:      author,
		Tags:        tags,
		SortOrder:   maxOrder + 1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (db *DB) GetFeeds() ([]models.Feed, error) {
	rows, err := db.conn.Query("SELECT id, name, description, author, tags, is_system, sort_order, new_video_count, created_at, updated_at FROM feeds ORDER BY sort_order ASC, name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []models.Feed
	for rows.Next() {
		var f models.Feed
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

func (db *DB) GetFeed(id int64) (*models.Feed, error) {
	var f models.Feed
	err := db.conn.QueryRow(
		"SELECT id, name, description, author, tags, is_system, sort_order, new_video_count, created_at, updated_at FROM feeds WHERE id = ?", id,
	).Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (db *DB) UpdateFeed(id int64, name string) error {
	_, err := db.conn.Exec(
		"UPDATE feeds SET name = ?, updated_at = ? WHERE id = ?",
		name, time.Now(), id,
	)
	return err
}

func (db *DB) UpdateFeedMetadata(id int64, name, description, author, tags string) error {
	_, err := db.conn.Exec(
		"UPDATE feeds SET name = ?, description = ?, author = ?, tags = ?, updated_at = ? WHERE id = ?",
		name, description, author, tags, time.Now(), id,
	)
	return err
}

func (db *DB) DeleteFeed(id int64) error {
	// Check if this is a system feed
	var isSystem bool
	err := db.conn.QueryRow("SELECT is_system FROM feeds WHERE id = ?", id).Scan(&isSystem)
	if err != nil {
		return err
	}
	if isSystem {
		return ErrSystemFeed
	}
	_, err = db.conn.Exec("DELETE FROM feeds WHERE id = ?", id)
	return err
}

// GetOrCreateFeed returns an existing feed by name or creates it if it doesn't exist
func (db *DB) GetOrCreateFeed(name string) (*models.Feed, error) {
	var f models.Feed
	err := db.conn.QueryRow(
		"SELECT id, name, description, author, tags, is_system, sort_order, new_video_count, created_at, updated_at FROM feeds WHERE name = ?", name,
	).Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt)
	if err == nil {
		return &f, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	// Feed doesn't exist, create it
	return db.CreateFeed(name)
}

// Channel operations

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

// DeleteChannel removes a channel completely (from all feeds)
func (db *DB) DeleteChannel(channelID int64) error {
	// CASCADE will handle feed_channels and videos
	_, err := db.conn.Exec("DELETE FROM channels WHERE id = ?", channelID)
	return err
}

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

// GetChannelByURL returns a channel by its URL, or nil if not found
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

// GetChannelsByURL returns the channel with the given URL (now unique)
func (db *DB) GetChannelsByURL(url string) ([]models.Channel, error) {
	channel, err := db.GetChannelByURL(url)
	if err != nil || channel == nil {
		return nil, err
	}
	return []models.Channel{*channel}, nil
}

// GetFeedsByChannel returns all feeds that contain a channel
func (db *DB) GetFeedsByChannel(channelID int64) ([]models.Feed, error) {
	rows, err := db.conn.Query(`
		SELECT f.id, f.name, f.description, f.author, f.tags, f.is_system, f.sort_order, f.new_video_count, f.created_at, f.updated_at
		FROM feeds f
		JOIN feed_channels fc ON f.id = fc.feed_id
		WHERE fc.channel_id = ?
		ORDER BY f.sort_order ASC, f.name ASC
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []models.Feed
	for rows.Next() {
		var f models.Feed
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

// ReorderFeeds updates sort_order for feeds based on the provided order.
// feedIDs should contain all feed IDs in the desired display order.
func (db *DB) ReorderFeeds(feedIDs []int64) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE feeds SET sort_order = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, id := range feedIDs {
		if _, err := stmt.Exec(i, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateNewVideoCount sets the new_video_count for a feed
func (db *DB) UpdateNewVideoCount(feedID int64, count int) error {
	_, err := db.conn.Exec("UPDATE feeds SET new_video_count = ? WHERE id = ?", count, feedID)
	return err
}

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

// Video operations

// UpsertVideo inserts or updates a video. Returns true if this was a new insert.
func (db *DB) UpsertVideo(v *models.Video) (bool, error) {
	var isShort *int
	if v.IsShort != nil {
		val := 0
		if *v.IsShort {
			val = 1
		}
		isShort = &val
	}

	// Check if video exists first
	var exists bool
	err := db.conn.QueryRow("SELECT 1 FROM videos WHERE id = ?", v.ID).Scan(&exists)
	isInsert := err == sql.ErrNoRows

	_, err = db.conn.Exec(`
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
	return isInsert, err
}

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

func (db *DB) GetVideosByChannel(channelID int64, limit, offset int) ([]models.Video, error) {
	rows, err := db.conn.Query(`
		SELECT id, channel_id, title, channel_name, thumbnail, duration, is_short, published, url
		FROM videos
		WHERE channel_id = ?
		ORDER BY published DESC
		LIMIT ? OFFSET ?
	`, channelID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.Video
	for rows.Next() {
		var v models.Video
		var isShort sql.NullBool
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &isShort, &v.Published, &v.URL); err != nil {
			return nil, err
		}
		if isShort.Valid {
			v.IsShort = &isShort.Bool
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

// GetVideoCountByChannel returns the total number of videos for a channel
func (db *DB) GetVideoCountByChannel(channelID int64) (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM videos WHERE channel_id = ?`, channelID).Scan(&count)
	return count, err
}

// GetVideosWithoutDuration returns video IDs that have duration = 0
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

// UpdateVideoDuration updates the duration for a single video
func (db *DB) UpdateVideoDuration(videoID string, duration int) error {
	_, err := db.conn.Exec(`UPDATE videos SET duration = ? WHERE id = ?`, duration, videoID)
	return err
}

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

func (db *DB) GetAllRecentVideos(limit, offset int) ([]models.Video, int, error) {
	// Get total count first
	var total int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM videos").Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.conn.Query(`
		SELECT id, channel_id, title, channel_name, thumbnail, duration, is_short, published, url
		FROM videos
		ORDER BY published DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
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

// Channel metadata operations

type ChannelMetadata struct {
	URL         string
	Name        string
	VideoTitles string // comma-separated
	FetchedAt   time.Time
}

func (db *DB) GetChannelMetadata(url string) (*ChannelMetadata, error) {
	var m ChannelMetadata
	err := db.conn.QueryRow(
		"SELECT url, name, video_titles, fetched_at FROM channel_metadata WHERE url = ?", url,
	).Scan(&m.URL, &m.Name, &m.VideoTitles, &m.FetchedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (db *DB) GetAllChannelMetadata() (map[string]*ChannelMetadata, error) {
	rows, err := db.conn.Query("SELECT url, name, video_titles, fetched_at FROM channel_metadata")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metadata := make(map[string]*ChannelMetadata)
	for rows.Next() {
		var m ChannelMetadata
		if err := rows.Scan(&m.URL, &m.Name, &m.VideoTitles, &m.FetchedAt); err != nil {
			return nil, err
		}
		metadata[m.URL] = &m
	}
	return metadata, rows.Err()
}

func (db *DB) UpsertChannelMetadata(url, name, videoTitles string) error {
	_, err := db.conn.Exec(`
		INSERT INTO channel_metadata (url, name, video_titles, fetched_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
			name = excluded.name,
			video_titles = excluded.video_titles,
			fetched_at = excluded.fetched_at
	`, url, name, videoTitles, time.Now())
	return err
}

// Watch progress operations

type WatchProgress struct {
	VideoID         string    `json:"video_id"`
	ProgressSeconds int       `json:"progress_seconds"`
	DurationSeconds int       `json:"duration_seconds"`
	WatchedAt       time.Time `json:"watched_at"`
}

func (db *DB) UpdateWatchProgress(videoID string, progressSeconds, durationSeconds int) error {
	// Only update if:
	// 1. No existing record, OR
	// 2. New progress is higher than existing, OR
	// 3. New progress is at least 10 seconds (to allow restarting from beginning intentionally)
	_, err := db.conn.Exec(`
		INSERT INTO watch_progress (video_id, progress_seconds, duration_seconds, watched_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET
			progress_seconds = CASE
				WHEN excluded.progress_seconds > watch_progress.progress_seconds THEN excluded.progress_seconds
				WHEN excluded.progress_seconds >= 10 THEN excluded.progress_seconds
				ELSE watch_progress.progress_seconds
			END,
			duration_seconds = CASE
				WHEN excluded.duration_seconds > watch_progress.duration_seconds THEN excluded.duration_seconds
				ELSE watch_progress.duration_seconds
			END,
			watched_at = excluded.watched_at
	`, videoID, progressSeconds, durationSeconds, time.Now())
	return err
}

func (db *DB) GetWatchProgress(videoID string) (*WatchProgress, error) {
	var wp WatchProgress
	err := db.conn.QueryRow(
		"SELECT video_id, progress_seconds, duration_seconds, watched_at FROM watch_progress WHERE video_id = ?", videoID,
	).Scan(&wp.VideoID, &wp.ProgressSeconds, &wp.DurationSeconds, &wp.WatchedAt)
	if err != nil {
		return nil, err
	}
	return &wp, nil
}

func (db *DB) GetWatchProgressMap(videoIDs []string) (map[string]*WatchProgress, error) {
	if len(videoIDs) == 0 {
		return make(map[string]*WatchProgress), nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(videoIDs))
	args := make([]any, len(videoIDs))
	for i, id := range videoIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT video_id, progress_seconds, duration_seconds, watched_at FROM watch_progress WHERE video_id IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*WatchProgress)
	for rows.Next() {
		var wp WatchProgress
		if err := rows.Scan(&wp.VideoID, &wp.ProgressSeconds, &wp.DurationSeconds, &wp.WatchedAt); err != nil {
			return nil, err
		}
		result[wp.VideoID] = &wp
	}
	return result, rows.Err()
}

func (db *DB) GetWatchHistory(limit int) ([]models.Video, error) {
	rows, err := db.conn.Query(`
		SELECT v.id, v.channel_id, v.title, v.channel_name, v.thumbnail, v.duration, v.is_short, v.published, v.url
		FROM videos v
		JOIN watch_progress wp ON v.id = wp.video_id
		ORDER BY wp.watched_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.Video
	for rows.Next() {
		var v models.Video
		var isShort sql.NullBool
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &isShort, &v.Published, &v.URL); err != nil {
			return nil, err
		}
		if isShort.Valid {
			v.IsShort = &isShort.Bool
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

func (db *DB) MarkAsWatched(videoID string) error {
	// Mark as fully watched (100% = progress equals duration)
	// Use 100/100 as a marker for "manually marked watched"
	_, err := db.conn.Exec(`
		INSERT INTO watch_progress (video_id, progress_seconds, duration_seconds, watched_at)
		VALUES (?, 100, 100, ?)
		ON CONFLICT(video_id) DO UPDATE SET
			progress_seconds = 100,
			duration_seconds = 100,
			watched_at = excluded.watched_at
	`, videoID, time.Now())
	return err
}

func (db *DB) DeleteWatchProgress(videoID string) error {
	_, err := db.conn.Exec("DELETE FROM watch_progress WHERE video_id = ?", videoID)
	return err
}

// SponsorBlock segment operations

type SponsorBlockSegment struct {
	VideoID    string    `json:"video_id"`
	SegmentUUID string   `json:"segment_uuid"`
	StartTime  float64   `json:"start_time"`
	EndTime    float64   `json:"end_time"`
	Category   string    `json:"category"`
	ActionType string    `json:"action_type"`
	Votes      int       `json:"votes"`
	FetchedAt  time.Time `json:"fetched_at"`
}

// GetSponsorBlockSegments returns cached segments for a video
func (db *DB) GetSponsorBlockSegments(videoID string) ([]SponsorBlockSegment, error) {
	rows, err := db.conn.Query(`
		SELECT video_id, segment_uuid, start_time, end_time, category, action_type, votes, fetched_at
		FROM sponsorblock_segments
		WHERE video_id = ?
		ORDER BY start_time
	`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segments []SponsorBlockSegment
	for rows.Next() {
		var s SponsorBlockSegment
		if err := rows.Scan(&s.VideoID, &s.SegmentUUID, &s.StartTime, &s.EndTime, &s.Category, &s.ActionType, &s.Votes, &s.FetchedAt); err != nil {
			return nil, err
		}
		segments = append(segments, s)
	}
	return segments, rows.Err()
}

// HasSponsorBlockSegments checks if we have cached segments for a video (even if empty)
func (db *DB) HasSponsorBlockSegments(videoID string) (bool, time.Time, error) {
	var fetchedAt time.Time
	err := db.conn.QueryRow(`
		SELECT fetched_at FROM sponsorblock_segments WHERE video_id = ? LIMIT 1
	`, videoID).Scan(&fetchedAt)
	if err == sql.ErrNoRows {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}
	return true, fetchedAt, nil
}

// SaveSponsorBlockSegments saves segments for a video (replaces existing)
func (db *DB) SaveSponsorBlockSegments(videoID string, segments []SponsorBlockSegment) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing segments for this video
	if _, err := tx.Exec("DELETE FROM sponsorblock_segments WHERE video_id = ?", videoID); err != nil {
		return err
	}

	// Insert new segments
	stmt, err := tx.Prepare(`
		INSERT INTO sponsorblock_segments (video_id, segment_uuid, start_time, end_time, category, action_type, votes, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, s := range segments {
		if _, err := stmt.Exec(videoID, s.SegmentUUID, s.StartTime, s.EndTime, s.Category, s.ActionType, s.Votes, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// MarkSponsorBlockFetched marks that we've fetched segments for a video (even if none found)
func (db *DB) MarkSponsorBlockFetched(videoID string) error {
	_, err := db.conn.Exec(`
		INSERT INTO sponsorblock_segments (video_id, segment_uuid, start_time, end_time, category, action_type, votes, fetched_at)
		VALUES (?, '__no_segments__', 0, 0, 'none', 'none', 0, ?)
		ON CONFLICT(video_id, segment_uuid) DO UPDATE SET fetched_at = excluded.fetched_at
	`, videoID, time.Now())
	return err
}

// GetShuffledVideosByFeed returns unwatched, non-short videos in random order
func (db *DB) GetShuffledVideosByFeed(feedID int64, limit, offset int) ([]models.Video, int, error) {
	// Get total count of unwatched, non-short videos
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

// GetNearbyVideos returns videos from the same feed as the given video,
// positioned around the current video based on publish date.
// Returns up to `limit` videos that come after this video in the feed.
// Excludes shorts.
func (db *DB) GetNearbyVideos(videoID string, limit int, offset int) ([]models.Video, int64, error) {
	// First, get a feed for this video and published date
	// Note: With multi-feed channels, a video could be in multiple feeds.
	// We pick the first one found for now.
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

	// Get videos from the same feed that are older than (or same as) the current video
	// excluding the current video itself and shorts, ordered by newest first
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

// GetVideoShortsStatus returns existing shorts status for given video IDs.
// Returns a map of videoID -> isShort for videos that have is_short set (not NULL).
func (db *DB) GetVideoShortsStatus(videoIDs []string) (map[string]bool, error) {
	if len(videoIDs) == 0 {
		return map[string]bool{}, nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(videoIDs))
	args := make([]any, len(videoIDs))
	for i, id := range videoIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, is_short FROM videos
		WHERE id IN (%s) AND is_short IS NOT NULL
	`, strings.Join(placeholders, ","))

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var id string
		var isShort int
		if err := rows.Scan(&id, &isShort); err != nil {
			return nil, err
		}
		result[id] = isShort == 1
	}
	return result, rows.Err()
}
