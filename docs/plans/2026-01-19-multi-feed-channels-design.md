# Multi-Feed Channel Management

## Overview

Enable channels to belong to multiple feeds simultaneously, with easy management from both the channel page and while browsing feed videos.

## Goals

1. Channels can be in multiple feeds at once
2. Videos are shared (single source of truth) - watching marks it watched everywhere
3. Easy to add/remove channels from feeds on the channel page
4. Easy to remove a channel from a feed while browsing videos

## Data Model Changes

### Current Model

```
Feed (1) → (many) Channel (1) → (many) Video
```

Channels have a `feed_id` foreign key, limiting them to one feed.

### New Model

```
Feed (many) ←→ (many) Channel (1) → (many) Video
```

Channels become independent entities linked to feeds via a junction table.

### Schema Changes

**Remove `feed_id` from channels, add unique constraint on URL:**
```sql
CREATE TABLE channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);
```

**New junction table:**
```sql
CREATE TABLE feed_channels (
    feed_id INTEGER NOT NULL,
    channel_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (feed_id, channel_id),
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE
);
```

### Deletion Behavior

- Deleting a feed removes `feed_channels` rows; channel and videos remain if channel is in other feeds
- Removing a channel from its last feed deletes the channel and its videos (orphan cleanup)

## Migration Strategy

### Goose Setup

Add goose for versioned migrations:

```
/internal/db/
  migrations/
    001_initial_schema.sql       # Current schema as baseline
    002_multi_feed_channels.sql  # Many-to-many transformation
```

### Migration 002: Multi-Feed Channels

```sql
-- +goose Up

-- Create new deduplicated channels table
CREATE TABLE channels_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

-- Create junction table
CREATE TABLE feed_channels (
    feed_id INTEGER NOT NULL,
    channel_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (feed_id, channel_id),
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES channels_new(id) ON DELETE CASCADE
);

-- Migrate: insert unique channels (dedupe by URL)
INSERT INTO channels_new (url, name)
SELECT url, name FROM channels GROUP BY url;

-- Migrate: create feed-channel links
INSERT INTO feed_channels (feed_id, channel_id)
SELECT c.feed_id, cn.id
FROM channels c
JOIN channels_new cn ON cn.url = c.url;

-- Migrate: update video references to new channel IDs
UPDATE videos SET channel_id = (
    SELECT cn.id FROM channels_new cn
    JOIN channels c ON c.url = cn.url
    WHERE c.id = videos.channel_id
);

-- Swap tables
DROP TABLE channels;
ALTER TABLE channels_new RENAME TO channels;

-- +goose Down
-- Note: Rollback would require picking one feed per channel
-- This is a one-way migration in practice
```

## API Changes

### Modified Endpoints

| Endpoint | Change |
|----------|--------|
| `POST /api/feeds/{id}/channels` | Creates channel if new URL, adds to feed if not already in it |
| `GET /api/channels/{id}` | Also returns `feeds[]` array of feeds the channel belongs to |
| `GET /api/feeds/{id}` | Query joins through `feed_channels` instead of direct `feed_id` |

### Removed Endpoints

| Endpoint | Reason |
|----------|--------|
| `DELETE /api/channels/{id}` | Replaced by feed-specific removal |
| `POST /api/channels/{id}/move` | Replaced by add/remove pattern |

### New Endpoints

| Endpoint | Request | Response | Purpose |
|----------|---------|----------|---------|
| `DELETE /api/feeds/{feedId}/channels/{channelId}` | - | `204` or `200 {deleted: true}` | Remove channel from feed; deletes channel if orphaned |
| `POST /api/channels/{id}/feeds` | `{feedId: number}` | `{feeds: Feed[]}` | Add existing channel to another feed |
| `GET /api/channels/{id}/feeds` | - | `{feeds: Feed[]}` | Get all feeds containing this channel |

### Behavior Details

**Adding channel to feed (`POST /api/feeds/{id}/channels`):**
1. Resolve URL to canonical form
2. If channel with URL doesn't exist → create channel, fetch videos
3. If channel exists → reuse existing channel and videos
4. Add `feed_channels` row (ignore if already exists)

**Removing channel from feed (`DELETE /api/feeds/{feedId}/channels/{channelId}`):**
1. Remove `feed_channels` row
2. Check if channel has any remaining feed associations
3. If orphaned → delete channel and cascade delete videos
4. Return `{deleted: true}` if channel was fully deleted, `{deleted: false}` if just unlinked

## UI Changes

### Channel Page - Feed Chips

**Location:** `/channels/[id]` page, below channel name/URL

**Layout:**
```
┌─────────────────────────────────────────────────┐
│ TechReviews                                     │
│ youtube.com/@techreviews                        │
│                                                 │
│ Feeds: [Tech ✕] [Favorites ✕] [+ Add to feed]  │
│                                                 │
│ [Refresh]                                       │
└─────────────────────────────────────────────────┘
```

**Components:**
- Feed chips with ✕ button for each feed the channel belongs to
- "+ Add to feed" button that opens a dropdown of available feeds

**Behavior:**
- Click ✕ on chip → API call to remove from feed → chip disappears
- If removing from last feed → show confirmation dialog ("This will delete the channel and all its videos")
- Click "+ Add to feed" → dropdown shows feeds not already containing this channel
- Select feed from dropdown → API call to add → new chip appears

**State:**
```typescript
let channelFeeds: Feed[] = [];  // Feeds this channel belongs to
let availableFeeds: Feed[] = []; // Feeds not containing this channel
```

### Feed Page - Video Card Menu

**Location:** Three-dot menu on each video card in feed view

**Menu Addition:**
```
┌─────────────────────────┐
│ Open in YouTube         │
│ Mark as watched         │
│ ─────────────────────── │
│ Remove channel from     │
│ this feed               │
└─────────────────────────┘
```

**Behavior:**
- Click "Remove channel from this feed" → API call → all videos from that channel disappear from current view
- No confirmation modal (easily reversible by re-adding from channel page)
- Show toast notification: "Removed [ChannelName] from [FeedName]"

**Implementation:**
```typescript
async function removeChannelFromFeed(channelId: number) {
    await api.delete(`/api/feeds/${feed.id}/channels/${channelId}`);
    // Filter out all videos from this channel
    videos = videos.filter(v => v.channelId !== channelId);
    // Update channels list if on Channels tab
    channels = channels.filter(c => c.id !== channelId);
    showToast(`Removed ${channelName} from ${feed.name}`);
}
```

## Database Query Changes

### Get Videos by Feed

**Before:**
```sql
SELECT v.* FROM videos v
JOIN channels c ON v.channel_id = c.id
WHERE c.feed_id = ?
ORDER BY v.published DESC
```

**After:**
```sql
SELECT v.* FROM videos v
JOIN channels c ON v.channel_id = c.id
JOIN feed_channels fc ON c.id = fc.channel_id
WHERE fc.feed_id = ?
ORDER BY v.published DESC
```

### Get Channels by Feed

**Before:**
```sql
SELECT * FROM channels WHERE feed_id = ? ORDER BY name
```

**After:**
```sql
SELECT c.* FROM channels c
JOIN feed_channels fc ON c.id = fc.channel_id
WHERE fc.feed_id = ?
ORDER BY c.name
```

### Get Feeds by Channel

**New query:**
```sql
SELECT f.* FROM feeds f
JOIN feed_channels fc ON f.id = fc.feed_id
WHERE fc.channel_id = ?
ORDER BY f.name
```

## Implementation Order

1. **Add goose** - Set up migration tooling, create baseline migration
2. **Run migration** - Create `feed_channels` table, migrate existing data
3. **Update Go models** - Remove `FeedID` from `Channel`, add relationship queries
4. **Update DB layer** - New queries for many-to-many relationships
5. **Update API handlers** - Implement new endpoints, modify existing ones
6. **Update frontend types** - Add `feeds` to channel response type
7. **Channel page UI** - Add feed chips component
8. **Feed page UI** - Add menu option to video cards
9. **Test migration** - Verify data integrity on copy of real database

## Files to Modify

### Backend
- `go.mod` - Add goose dependency
- `internal/db/db.go` - Replace hand-rolled migrations with goose, update all channel/feed queries
- `internal/db/migrations/*.sql` - New migration files
- `internal/models/models.go` - Update `Channel` struct
- `internal/api/api_handlers.go` - New and modified endpoints

### Frontend
- `web/frontend/src/lib/api.ts` - New API functions
- `web/frontend/src/lib/types.ts` - Update types
- `web/frontend/src/routes/channels/[id]/+page.svelte` - Feed chips UI
- `web/frontend/src/routes/feeds/[id]/+page.svelte` - Video card menu option
- `web/frontend/src/lib/components/VideoCard.svelte` - Add menu if not exists, or modify existing
