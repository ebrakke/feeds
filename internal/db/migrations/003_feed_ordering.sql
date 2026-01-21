-- +goose Up

-- Add sort_order column (lower = higher in list)
ALTER TABLE feeds ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

-- Add new_video_count column (videos added in last refresh)
ALTER TABLE feeds ADD COLUMN new_video_count INTEGER NOT NULL DEFAULT 0;

-- Initialize sort_order based on current display order (is_system DESC, name ASC)
-- This preserves existing order for users
WITH ordered_feeds AS (
    SELECT id, ROW_NUMBER() OVER (ORDER BY is_system DESC, name ASC) - 1 AS new_order
    FROM feeds
)
UPDATE feeds SET sort_order = (
    SELECT new_order FROM ordered_feeds WHERE ordered_feeds.id = feeds.id
);

-- +goose Down

-- SQLite doesn't support DROP COLUMN easily, but goose can handle it
-- For simplicity, we'll just ignore these columns if rolling back
