# Stream Quality Switching Design

## Problem

Once a higher quality stream is selected (e.g., 1080p), users cannot downgrade to a lower quality. The current UI only shows quality buttons for resolutions higher than what's currently playing, plus cached qualities.

Additionally, the current "Download HD" buttons conflate two concerns: selecting playback quality and downloading for offline use.

## Solution

Separate stream quality selection from downloads with two distinct controls:

1. **Quality dropdown** - Select what quality to stream (can always upgrade or downgrade)
2. **Download button** - Download the currently selected quality for offline use

## UI Design

### Layout

Below the video, replace the current "Download HD" row with:

```
┌─────────────────────────────────────────────────────┐
│  Quality: [720p ▼]          [⬇ Download 720p]      │
└─────────────────────────────────────────────────────┘
```

### Quality Dropdown

- **Always shows all available qualities** (360p, 480p, 720p, 1080p, 1440p, 2160p)
- **Indicators:**
  - Checkmark (✓) next to cached qualities
  - Current selection highlighted
- **Behavior:**
  - Selecting any quality immediately starts streaming at that quality
  - If not cached, buffers in background then switches (existing streaming behavior)
  - No restrictions on downgrading - user can always select lower quality

### Download Button

- **Label:** "Download {quality}p" where quality is the currently selected quality
- **States:**
  - Default: Shows download icon + "Download 720p"
  - Downloading: Shows progress percentage
  - Cached: Shows checkmark + "Downloaded" (disabled or visually distinct)
- **Behavior:**
  - Clicking starts an explicit download of the selected quality
  - Uses existing download infrastructure (POST /api/videos/{id}/download)
  - Shows progress via existing SSE subscription

## Implementation Changes

### Frontend (`/web/frontend/src/routes/watch/[id]/+page.svelte`)

1. **Replace "Download HD" buttons with new UI:**
   - Remove the `download-hd-row` section
   - Add quality dropdown (`<select>` or custom dropdown)
   - Add single download button

2. **Update quality selection logic:**
   - `handleQualitySelect()` should work for any quality (remove restrictions)
   - Always allow selecting qualities lower than current playback
   - Remove the `qNum > actualHeight` filter from display logic

3. **Add download button handler:**
   - New function to download currently selected quality
   - Reuse existing `startDownload()` API call
   - Show progress on the download button itself

### No Backend Changes Required

The existing API already supports:
- Streaming any quality via `/api/stream/{id}?quality=X`
- Downloading any quality via `POST /api/videos/{id}/download`
- Quality status via `GET /api/videos/{id}/qualities`

## Migration

- Remove the old "Download HD" button row entirely
- Users who want to download specific qualities use the new dropdown + download flow
- Auto quality behavior remains available as a dropdown option

## Edge Cases

1. **Download while streaming different quality:** User streams 480p but downloads 1080p - both should work independently
2. **Quality not available:** If a quality isn't in `availableQualities`, don't show it in dropdown
3. **Download already in progress:** Disable download button and show progress
4. **Switch quality during download:** Allow - streaming and downloading are independent operations
