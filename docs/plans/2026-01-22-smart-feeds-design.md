# Smart Feeds Design

Auto-generated feeds based on in-app watch history.

## Overview

Two dynamic feeds that surface content based on viewing behavior:

1. **Hot This Week** - Videos from channels you've been watching heavily in the last 7 days
2. **Continue Watching** - Videos you started (10%+ progress) but haven't finished

These feeds appear in a "Smart Feeds" section above regular feeds, are always visible, and cannot be modified by users.

## Data Model

No new database tables. Uses existing `watch_progress` data:

- **Hot This Week**: Join `watch_progress` → `videos` → `channels`, aggregate watches in last 7 days, return unwatched videos from top channels
- **Continue Watching**: Filter `watch_progress` where `progress_seconds / duration_seconds` is between 0.1 and 0.95

Feeds are "virtual" - they don't exist in the `feeds` table. The API returns them separately and the frontend renders them in a dedicated section.

## API

### GET /api/smart-feeds

Returns metadata for all smart feeds.

```json
{
  "feeds": [
    {
      "slug": "hot-this-week",
      "name": "Hot This Week",
      "icon": "flame",
      "videoCount": 12
    },
    {
      "slug": "continue-watching",
      "name": "Continue Watching",
      "icon": "play",
      "videoCount": 3
    }
  ]
}
```

### GET /api/smart-feeds/hot-this-week

Query params: `limit` (default 50), `offset` (default 0)

Logic:
1. Find channels with the most `watch_progress` entries where `watched_at` is within last 7 days
2. Return recent videos from those channels that user hasn't fully watched (< 95% progress)
3. Order by video publish date descending

### GET /api/smart-feeds/continue-watching

Query params: `limit` (default 50), `offset` (default 0)

Logic:
1. Find videos where `progress_seconds / duration_seconds` is between 0.1 and 0.95
2. Order by `watched_at` descending (most recently watched first)

## Background Refresh

A goroutine runs every 15 minutes to pre-compute and cache:
- Top channels from last 7 days with watch counts
- Continue watching video list with progress data

This keeps API responses fast while data stays reasonably fresh.

## Frontend

### Feed List Structure

```
Smart Feeds
  🔥 Hot This Week (12)
  ▶️ Continue Watching (3)

Your Feeds
  Inbox
  Heavy Rotation
  ...
```

- Smart Feeds section appears above "Your Feeds"
- Each shows a video count badge
- No edit/delete/reorder controls for smart feeds

### Routing

```
/smart/hot-this-week
/smart/continue-watching
```

The existing feed video grid component is reused - data source changes based on whether it's a regular feed (by ID) or smart feed (by slug).

## Edge Cases

### Empty States

- **Hot This Week** with no watch history in 7 days: "Watch some videos to see your hot channels here"
- **Continue Watching** with no partial videos: "No videos in progress"

### Video Lifecycle

- Videos reaching 95%+ progress are removed from Continue Watching on next refresh
- Fully watched videos are excluded from Hot This Week
- New users see empty smart feeds until they start watching

## Files to Modify

### Backend
- `internal/api/api.go` - Register new routes
- `internal/api/api_handlers.go` - Add smart feed handlers
- `internal/db/db.go` - Add queries for hot channels and continue watching

### Frontend
- `web/frontend/src/lib/api.ts` - Add API client functions
- `web/frontend/src/lib/components/Sidebar.svelte` - Add Smart Feeds section
- `web/frontend/src/routes/smart/[slug]/+page.svelte` - Smart feed view page
