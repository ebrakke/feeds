# Synchronous Shorts Detection

**Date:** 2026-01-18
**Status:** Approved

## Problem

Videos appear in feeds before shorts status is known. When you:
1. Refresh a feed → Videos show up with `is_short = null`, then get categorized later in background
2. Load more via pagination → Already-stored videos that were never checked still have `is_short = null`

This causes shorts and regular videos to be mixed in the UI until the background job completes.

## Goal

By the time any video appears in the UI, it should already have `is_short = true` or `is_short = false` - never `null`.

## Approach

Check shorts status synchronously at two points:

1. **During feed refresh** - After fetching RSS, before returning response, check all new videos
2. **During pagination** - Before returning videos to frontend, check any with `is_short = null`

### Trade-offs

- **Latency:** Adds time to refresh/pagination requests
- **Benefit:** Clean separation between shorts and regular videos at all times
- **Mitigation:** Existing `CheckShortsStatus` handles concurrency (5 parallel requests)

## Implementation

### File: `internal/api/api_handlers.go`

#### 1. `handleAPIRefreshFeed` (~line 303-321)

Collect all videos first, check shorts status, then save:

```go
var allVideos []models.Video
for res := range results {
    // collect videos...
}

videoIDs := make([]string, len(allVideos))
for i, v := range allVideos {
    videoIDs[i] = v.ID
}
shortsStatus := yt.CheckShortsStatus(videoIDs)

for i := range allVideos {
    if isShort, ok := shortsStatus[allVideos[i].ID]; ok {
        allVideos[i].IsShort = &isShort
    }
    s.db.UpsertVideo(&allVideos[i])
}
```

#### 2. `handleAPIGetFeed` (~line 205-210)

After fetching videos, check any with null status:

```go
var uncheckedIDs []string
for _, v := range videos {
    if v.IsShort == nil {
        uncheckedIDs = append(uncheckedIDs, v.ID)
    }
}
if len(uncheckedIDs) > 0 {
    shortsStatus := yt.CheckShortsStatus(uncheckedIDs)
    for i := range videos {
        if isShort, ok := shortsStatus[videos[i].ID]; ok {
            videos[i].IsShort = &isShort
            s.db.UpdateVideoIsShort(videos[i].ID, isShort)
        }
    }
}
```

#### 3. `handleAPIAddChannel` (~line 397-404)

Check shorts before saving initial videos.

#### 4. `handleAPIRefreshChannel` (~line 461-476)

Check shorts before saving refreshed videos.

### File: `internal/api/handlers.go`

#### 5. Remove background job calls

Remove `go s.fetchMissingShortsStatus(feedID)` from:
- `handleRefreshFeed`
- `handleRefreshFeedStream`

## Endpoints Affected

- `POST /api/feeds/:id/refresh` - Refresh feed
- `GET /api/feeds/:id` - Get feed with pagination
- `POST /api/feeds/:id/channels` - Add channel
- `POST /api/channels/:id/refresh` - Refresh channel
- `GET /feeds/:id/refresh` (SSE) - Streaming refresh
