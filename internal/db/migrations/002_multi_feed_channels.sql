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
