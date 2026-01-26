# YouTube Channel Import from Video URLs - Design Document

**Date:** 2026-01-25
**Status:** Approved
**Author:** User + Claude

## Problem Statement

The current "Paste a Link" import feature on `/import` only handles feed export JSON files. When users paste YouTube channel or video URLs, the import fails because the system attempts to parse them as JSON. Users need a simple way to add channels to feeds by pasting either a channel URL or a video URL (which would extract and add the channel that published the video).

## Goals

1. Support pasting YouTube channel URLs in any format (handles, IDs, custom names, etc.)
2. Support pasting YouTube video URLs and automatically extract the channel
3. Allow users to choose which feed to add the channel to (or create a new feed)
4. Maintain the existing feed import functionality
5. Provide clear feedback during the import process

## Non-Goals

- Bookmarking or saving specific videos (only extract and add the channel)
- Batch video URL processing
- Support for non-YouTube platforms
- Modifying the existing feed page channel input

## Solution Overview

Enhance the existing `/import` page to intelligently detect YouTube URLs and handle them with a new backend endpoint and feed selector UI flow.

### User Flow

```
User pastes URL into "Paste a Link" input
    ↓
Frontend detects URL type
    ├─ YouTube URL? → Show feed selector modal
    └─ Other URL? → Use existing JSON import flow
    ↓
User selects existing feed or creates new feed
    ↓
POST /api/import/youtube { url, feedId }
    ↓
Backend resolves channel (from channel or video URL)
    ↓
Backend adds channel to feed
    ↓
Navigate to feed page with success toast
```

## Backend Design

### New Function: `ResolveVideoToChannel`

**File:** `/internal/youtube/rss.go`

```go
// ResolveVideoToChannel extracts channel information from a YouTube video URL
// Supports: /watch?v=ID, youtu.be/ID, /shorts/ID
func ResolveVideoToChannel(videoURL string) (*ChannelInfo, error)
```

**Implementation approach:**
1. Extract video ID from URL using regex patterns
2. Construct video page URL: `https://www.youtube.com/watch?v={VIDEO_ID}`
3. Fetch video page HTML
4. Search for channel ID in HTML using regex patterns:
   - `"channelId":"([^"]+)"`
   - `"externalChannelId":"([^"]+)"`
   - `/channel/([^"/?]+)`
5. Call `fetchChannelInfoByID(channelID)` to get full channel details
6. Return `ChannelInfo{ID, Name, URL}`

**Supported video URL formats:**
- `https://www.youtube.com/watch?v=VIDEO_ID`
- `https://youtu.be/VIDEO_ID`
- `https://www.youtube.com/shorts/VIDEO_ID`
- `https://m.youtube.com/watch?v=VIDEO_ID`

### New API Endpoint: `handleAPIImportYouTube`

**File:** `/internal/api/api_handlers.go`

**Route:** `POST /api/import/youtube`

**Request:**
```json
{
  "url": "https://youtube.com/watch?v=... or channel URL",
  "feedId": 123
}
```

**Response (201 Created):**
```json
{
  "channel": {
    "id": 1,
    "url": "https://youtube.com/channel/UC...",
    "name": "Channel Name"
  },
  "feed": {
    "id": 123,
    "name": "Feed Name"
  }
}
```

**Error Response (400 Bad Request):**
```json
{
  "error": "Invalid YouTube URL"
}
```

**Implementation logic:**
1. Parse request body to get `url` and `feedId`
2. Validate feedId exists
3. Detect URL type:
   - Contains `/watch?v=` or `youtu.be/` or `/shorts/` → video URL
   - Otherwise → channel URL
4. Resolve to channel:
   - Video URL → `youtube.ResolveVideoToChannel(url)`
   - Channel URL → `youtube.ResolveChannelURL(url)`
5. Add channel to feed:
   - `db.AddChannelToFeed(feedId, channelInfo.URL, channelInfo.Name)`
6. If new channel, fetch initial videos (5 latest)
7. Get feed details from database
8. Return channel and feed objects

**Error handling:**
- Invalid feedId → 400 "Feed not found"
- Invalid URL format → 400 "Invalid YouTube URL"
- Channel resolution failure → 400 "Could not resolve channel from URL"
- Database errors → 500 "Failed to add channel"

### Router Update

**File:** `/internal/api/server.go` (or wherever routes are defined)

Add route:
```go
apiRouter.HandleFunc("/import/youtube", s.handleAPIImportYouTube).Methods("POST")
```

## Frontend Design

### URL Detection Logic

**File:** `/web/frontend/src/routes/import/+page.svelte`

Add helper function:
```typescript
function isYouTubeURL(url: string): boolean {
  const patterns = [
    /youtube\.com\/watch\?v=/,
    /youtu\.be\//,
    /youtube\.com\/shorts\//,
    /youtube\.com\/channel\//,
    /youtube\.com\/@/,
    /youtube\.com\/c\//,
    /youtube\.com\/user\//
  ];
  return patterns.some(pattern => pattern.test(url));
}
```

### Feed Selector Modal

**New Component or Inline Section:**

**UI Elements:**
- Modal/dialog backdrop with blur effect
- Title: "Add Channel to Feed"
- Subtitle: "Choose a feed or create a new one"
- List of existing feeds with radio button selection
- "Create New Feed" option with inline name input
- Action buttons: "Cancel" and "Add Channel"
- Loading spinner during API call

**State variables:**
```typescript
let showFeedSelector = $state(false);
let pendingYouTubeURL = $state('');
let selectedFeedId = $state<number | null>(null);
let createNewFeedName = $state('');
let feeds = $state<Feed[]>([]);
let youtubeImportLoading = $state(false);
let youtubeImportError = $state<string | null>(null);
```

### Modified Form Handler

**Update `handleImportURL` function:**

```typescript
async function handleImportURL(e: Event) {
  e.preventDefault();
  if (!importURL.trim()) return;

  // Check if it's a YouTube URL
  if (isYouTubeURL(importURL)) {
    pendingYouTubeURL = importURL;
    // Fetch feeds for selector
    feeds = await getFeeds();
    showFeedSelector = true;
    return;
  }

  // Existing JSON import flow
  importLoading = true;
  importError = null;
  try {
    const feed = await importFromURL(importURL);
    goto(`/feeds/${feed.id}`);
  } catch (e) {
    importError = e instanceof Error ? e.message : 'Failed to import';
  } finally {
    importLoading = false;
  }
}
```

### New API Function

**File:** `/web/frontend/src/lib/api.ts`

```typescript
export async function importYouTubeChannel(url: string, feedId: number) {
  const res = await fetch('/api/import/youtube', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url, feedId })
  });
  if (!res.ok) {
    const data = await res.json();
    throw new Error(data.error || 'Failed to import channel');
  }
  return res.json();
}
```

### Feed Selector Confirmation Handler

```typescript
async function handleYouTubeImportConfirm() {
  if (!selectedFeedId && !createNewFeedName.trim()) {
    youtubeImportError = 'Please select a feed or create a new one';
    return;
  }

  youtubeImportLoading = true;
  youtubeImportError = null;

  try {
    let feedId = selectedFeedId;

    // Create new feed if needed
    if (!feedId && createNewFeedName.trim()) {
      const newFeed = await createFeed(createNewFeedName.trim());
      feedId = newFeed.id;
    }

    // Import channel
    const result = await importYouTubeChannel(pendingYouTubeURL, feedId!);

    // Show success toast
    showToast(`Added ${result.channel.name} to ${result.feed.name}`);

    // Navigate to feed
    goto(`/feeds/${feedId}`);
  } catch (e) {
    youtubeImportError = e instanceof Error ? e.message : 'Failed to add channel';
  } finally {
    youtubeImportLoading = false;
  }
}
```

### UI Changes Summary

1. **Input placeholder update:**
   - Old: "https://youtube.com/channel/..."
   - New: "YouTube channel or video URL, or feed export link"

2. **Section description update:**
   - Old: "Add a YouTube channel or video URL"
   - Keep as is (already accurate)

3. **New modal component** (after input section, conditionally rendered)

## Testing Considerations

### Manual Testing Checklist

**Channel URL Formats:**
- [ ] `https://youtube.com/channel/UCxxxxxx`
- [ ] `https://youtube.com/@handle`
- [ ] `https://youtube.com/c/customname`
- [ ] `https://youtube.com/user/username`

**Video URL Formats:**
- [ ] `https://youtube.com/watch?v=VIDEO_ID`
- [ ] `https://youtu.be/VIDEO_ID`
- [ ] `https://youtube.com/shorts/VIDEO_ID`
- [ ] `https://m.youtube.com/watch?v=VIDEO_ID`

**Feed Selection:**
- [ ] Add to existing feed
- [ ] Create new feed inline
- [ ] Cancel and return to import page
- [ ] Validation: no feed selected shows error

**Edge Cases:**
- [ ] Invalid YouTube URL
- [ ] Private/deleted video
- [ ] Private/deleted channel
- [ ] Video from a channel you're already subscribed to
- [ ] Network timeout/failure
- [ ] Very long channel names

**Existing Functionality:**
- [ ] JSON feed import still works
- [ ] NewPipe import still works
- [ ] Watch history import still works
- [ ] Subscription packs still work

## Security Considerations

1. **URL validation:** Ensure only YouTube URLs are processed (prevent SSRF)
2. **Rate limiting:** Consider adding rate limits to prevent abuse of YouTube scraping
3. **Input sanitization:** Validate and sanitize all user inputs
4. **Error messages:** Don't expose internal system details in error messages

## Performance Considerations

1. **HTTP timeouts:** Use existing 10-second timeout for YouTube requests
2. **Caching:** No caching needed for one-off imports
3. **Database transactions:** Use existing transaction pattern for channel addition
4. **Initial video fetch:** Keep existing 5-video limit to avoid slow imports

## Migration & Deployment

**No database migrations required** - uses existing tables and schema.

**Deployment steps:**
1. Deploy backend changes first
2. Deploy frontend changes
3. No feature flag needed (additive change)
4. Monitor error logs for resolution failures

## Future Enhancements (Out of Scope)

- Batch video URL import (paste multiple videos, extract unique channels)
- Support for playlist URLs (add all channels from playlist)
- Support for other video platforms (Vimeo, Twitch, etc.)
- Save specific videos as bookmarks in addition to adding channel
- Browser extension for one-click "Add to Feeds"

## Implementation Order

1. Backend: Add `ResolveVideoToChannel` function to `youtube/rss.go`
2. Backend: Add `handleAPIImportYouTube` endpoint to `api_handlers.go`
3. Backend: Update router to include new endpoint
4. Backend: Test with curl/Postman
5. Frontend: Add URL detection helper function
6. Frontend: Add `importYouTubeChannel` API function
7. Frontend: Build feed selector modal UI
8. Frontend: Update form handler to show modal for YouTube URLs
9. Frontend: Add confirmation handler
10. Frontend: Update placeholder text
11. Integration testing
12. Deploy
