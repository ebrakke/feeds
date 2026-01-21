# Feed Ordering and New Video Counts

## Overview

Two enhancements to the feed list:
1. **Manual drag-and-drop ordering** - Arrange feeds in any order you want
2. **New video badges** - Show how many videos were added in the last refresh

## Data Model Changes

### Feed Table Additions

Add two columns to the `feeds` table:

```sql
ALTER TABLE feeds ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feeds ADD COLUMN new_video_count INTEGER NOT NULL DEFAULT 0;
```

- `sort_order`: Lower numbers appear first. New feeds get `MAX(sort_order) + 1`
- `new_video_count`: Count of videos inserted during the most recent refresh

### Query Changes

Feed list query changes from:
```sql
ORDER BY is_system DESC, name ASC
```

To:
```sql
ORDER BY sort_order ASC, name ASC
```

All feeds (system and user) are now mixed together and fully reorderable.

## API Changes

### New Endpoint: Reorder Feeds

```
PUT /api/feeds/reorder
```

Request body:
```json
{
  "feed_ids": [3, 1, 5, 2]
}
```

Backend assigns `sort_order` values (0, 1, 2, 3...) based on array position. Returns updated feeds list.

### Modified: Feed Response

Add fields to feed objects in `GET /api/feeds`:
```json
{
  "id": 1,
  "name": "Tech News",
  "sort_order": 0,
  "new_video_count": 5,
  ...
}
```

### Modified: Refresh Endpoint

`POST /api/feeds/{id}/refresh` now:
1. Tracks how many videos were **inserted** (not updated)
2. Stores that count in `feeds.new_video_count`
3. Returns the count in the response

## Frontend Changes

### Home Page Feed List

- Display feeds in `sort_order` order (no system/user grouping)
- Show badge after feed name when `new_video_count > 0`: "Tech News **(5)**"
- Badge styled subtly (muted color, smaller text)

### Drag-and-Drop Reordering

- Drag handle (grip icon) on left of each feed row
- HTML5 drag-and-drop for reordering
- On drop, call `PUT /api/feeds/reorder` with new order
- Optimistic UI update, revert on failure

### Visual Layout

```
[drag handle] Feed Name (5)
[drag handle] Another Feed
[drag handle] Third Feed (12)
```

## Migration Strategy

New migration file initializes `sort_order` based on current display order (system feeds first by name, then user feeds by name) so existing users see feeds in the same initial order.

## Files to Modify

| Layer | File | Changes |
|-------|------|---------|
| DB | `internal/db/migrations/` | New migration file |
| DB | `internal/db/db.go` | Add `ReorderFeeds()`, update `GetFeeds()` ordering, track inserts in refresh |
| API | `internal/api/api_handlers.go` | Add reorder handler, modify refresh to return count |
| API | `internal/api/routes.go` | Register new endpoint |
| Frontend | `src/lib/types.ts` | Add `sort_order`, `new_video_count` to Feed type |
| Frontend | `src/lib/api.ts` | Add `reorderFeeds()` function |
| Frontend | `src/routes/+page.svelte` | Add drag-and-drop, show badges, remove grouping |

## Behavior Summary

1. Feeds display in `sort_order` order
2. Drag a feed to reorder; changes persist immediately
3. Refreshing a feed updates its `new_video_count` badge
4. Badge shows "(N)" after feed name when N > 0
5. Badge resets only when the feed is refreshed again
