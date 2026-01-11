package db

import (
	"database/sql"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/erik/feeds/internal/models"
)

type DB struct {
	conn *sql.DB
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS feeds (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		author TEXT DEFAULT '',
		tags TEXT DEFAULT '',
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
	`
	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Run migrations for existing databases
	migrations := []string{
		"ALTER TABLE feeds ADD COLUMN description TEXT DEFAULT ''",
		"ALTER TABLE feeds ADD COLUMN author TEXT DEFAULT ''",
		"ALTER TABLE feeds ADD COLUMN tags TEXT DEFAULT ''",
	}
	for _, m := range migrations {
		// Ignore errors (column may already exist)
		db.conn.Exec(m)
	}

	return nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// Feed operations

func (db *DB) CreateFeed(name string) (*models.Feed, error) {
	return db.CreateFeedWithMetadata(name, "", "", "")
}

func (db *DB) CreateFeedWithMetadata(name, description, author, tags string) (*models.Feed, error) {
	now := time.Now()
	result, err := db.conn.Exec(
		"INSERT INTO feeds (name, description, author, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		name, description, author, tags, now, now,
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
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (db *DB) GetFeeds() ([]models.Feed, error) {
	rows, err := db.conn.Query("SELECT id, name, description, author, tags, created_at, updated_at FROM feeds ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []models.Feed
	for rows.Next() {
		var f models.Feed
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

func (db *DB) GetFeed(id int64) (*models.Feed, error) {
	var f models.Feed
	err := db.conn.QueryRow(
		"SELECT id, name, description, author, tags, created_at, updated_at FROM feeds WHERE id = ?", id,
	).Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.CreatedAt, &f.UpdatedAt)
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
	_, err := db.conn.Exec("DELETE FROM feeds WHERE id = ?", id)
	return err
}

// GetOrCreateFeed returns an existing feed by name or creates it if it doesn't exist
func (db *DB) GetOrCreateFeed(name string) (*models.Feed, error) {
	var f models.Feed
	err := db.conn.QueryRow(
		"SELECT id, name, description, author, tags, created_at, updated_at FROM feeds WHERE name = ?", name,
	).Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.CreatedAt, &f.UpdatedAt)
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

func (db *DB) AddChannel(feedID int64, url, name string) (*models.Channel, error) {
	result, err := db.conn.Exec(
		"INSERT INTO channels (feed_id, url, name) VALUES (?, ?, ?)",
		feedID, url, name,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.Channel{
		ID:     id,
		FeedID: feedID,
		URL:    url,
		Name:   name,
	}, nil
}

func (db *DB) GetChannelsByFeed(feedID int64) ([]models.Channel, error) {
	rows, err := db.conn.Query(
		"SELECT id, feed_id, url, name FROM channels WHERE feed_id = ? ORDER BY name", feedID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.Channel
	for rows.Next() {
		var c models.Channel
		if err := rows.Scan(&c.ID, &c.FeedID, &c.URL, &c.Name); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

func (db *DB) DeleteChannel(channelID int64) error {
	_, err := db.conn.Exec("DELETE FROM channels WHERE id = ?", channelID)
	return err
}

func (db *DB) MoveChannel(channelID, newFeedID int64) error {
	_, err := db.conn.Exec("UPDATE channels SET feed_id = ? WHERE id = ?", newFeedID, channelID)
	return err
}

func (db *DB) GetChannel(channelID int64) (*models.Channel, error) {
	var c models.Channel
	err := db.conn.QueryRow(
		"SELECT id, feed_id, url, name FROM channels WHERE id = ?", channelID,
	).Scan(&c.ID, &c.FeedID, &c.URL, &c.Name)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetChannelByURL returns a channel by its URL, or nil if not found
func (db *DB) GetChannelByURL(url string) (*models.Channel, error) {
	var c models.Channel
	err := db.conn.QueryRow(
		"SELECT id, feed_id, url, name FROM channels WHERE url = ?", url,
	).Scan(&c.ID, &c.FeedID, &c.URL, &c.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Video operations

func (db *DB) UpsertVideo(v *models.Video) error {
	_, err := db.conn.Exec(`
		INSERT INTO videos (id, channel_id, title, channel_name, thumbnail, duration, published, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			channel_id = excluded.channel_id,
			title = excluded.title,
			thumbnail = excluded.thumbnail,
			duration = excluded.duration
	`, v.ID, v.ChannelID, v.Title, v.ChannelName, v.Thumbnail, v.Duration, v.Published, v.URL)
	return err
}

func (db *DB) GetVideosByFeed(feedID int64, limit int) ([]models.Video, error) {
	rows, err := db.conn.Query(`
		SELECT v.id, v.channel_id, v.title, v.channel_name, v.thumbnail, v.duration, v.published, v.url
		FROM videos v
		JOIN channels c ON v.channel_id = c.id
		WHERE c.feed_id = ?
		ORDER BY v.published DESC
		LIMIT ?
	`, feedID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.Video
	for rows.Next() {
		var v models.Video
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &v.Published, &v.URL); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

func (db *DB) GetVideosByChannel(channelID int64, limit int) ([]models.Video, error) {
	rows, err := db.conn.Query(`
		SELECT id, channel_id, title, channel_name, thumbnail, duration, published, url
		FROM videos
		WHERE channel_id = ?
		ORDER BY published DESC
		LIMIT ?
	`, channelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.Video
	for rows.Next() {
		var v models.Video
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &v.Published, &v.URL); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

func (db *DB) DeleteVideosByFeed(feedID int64) error {
	_, err := db.conn.Exec(`
		DELETE FROM videos WHERE channel_id IN (
			SELECT id FROM channels WHERE feed_id = ?
		)
	`, feedID)
	return err
}

func (db *DB) GetAllRecentVideos(limit int) ([]models.Video, error) {
	rows, err := db.conn.Query(`
		SELECT id, channel_id, title, channel_name, thumbnail, duration, published, url
		FROM videos
		ORDER BY published DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.Video
	for rows.Next() {
		var v models.Video
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &v.Published, &v.URL); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
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
	VideoID         string
	ProgressSeconds int
	DurationSeconds int
	WatchedAt       time.Time
}

func (db *DB) UpdateWatchProgress(videoID string, progressSeconds, durationSeconds int) error {
	_, err := db.conn.Exec(`
		INSERT INTO watch_progress (video_id, progress_seconds, duration_seconds, watched_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET
			progress_seconds = excluded.progress_seconds,
			duration_seconds = excluded.duration_seconds,
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
		SELECT v.id, v.channel_id, v.title, v.channel_name, v.thumbnail, v.duration, v.published, v.url
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
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &v.Published, &v.URL); err != nil {
			return nil, err
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
