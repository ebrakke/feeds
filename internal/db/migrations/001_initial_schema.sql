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
