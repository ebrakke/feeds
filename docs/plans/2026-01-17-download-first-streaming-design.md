# Download-First Video Streaming Design

## Problem

YouTube provides high-quality video (720p+) as separate video and audio streams that require muxing. Server-side muxing via pipe breaks HTTP range requests because the total content length is unknown, which prevents video seeking. Previous attempts at fragmented MP4 and Matroska streaming failed to provide a usable experience.

## Solution

A "podcast app" model inspired by AntennaPod: stream immediately at whatever quality YouTube provides as a combined stream, then offer explicit HD download for higher quality.

## User Flow

1. **Page Load**: Video plays immediately using progressive "Auto" stream (whatever YouTube provides combined, typically up to 720p). Seeking works because YouTube supports range requests on progressive streams.

2. **HD Request**: User taps a quality button (720p/1080p/etc) to initiate download. Button shows downloading state.

3. **Progress Display**: Progress bar overlays the video thumbnail area during download.

4. **Completion**: Quality button gets a badge indicating cached version is ready. User continues watching current stream.

5. **Switch**: User taps quality button to switch to cached HD version when they choose.

## API Design

### Stream Endpoint (existing, simplified)

```
GET /api/stream/{id}?quality=auto
```

Returns progressive stream from YouTube with range header passthrough. This is the default playback path.

```
GET /api/stream/{id}?quality=720
```

If cached file exists, serve it with proper range support. Otherwise, return 404 (client should use auto quality).

### Download Initiation

```
POST /api/videos/{id}/download
Content-Type: application/json

{
  "quality": "720"
}
```

Response:
```json
{
  "status": "started",
  "downloadId": "abc123"
}
```

Or if already cached:
```json
{
  "status": "ready",
  "quality": "720"
}
```

### Download Progress (SSE)

```
GET /api/videos/{id}/download/status
```

Server-Sent Events stream:
```
event: progress
data: {"quality": "720", "percent": 45, "bytesDownloaded": 150000000, "totalBytes": 333000000}

event: progress
data: {"quality": "720", "percent": 100, "bytesDownloaded": 333000000, "totalBytes": 333000000}

event: complete
data: {"quality": "720", "path": "/cache/abc123.mp4"}

event: error
data: {"quality": "720", "error": "ffmpeg failed"}
```

### Available Qualities

```
GET /api/videos/{id}/qualities
```

Response:
```json
{
  "available": ["360", "480", "720", "1080"],
  "cached": ["720"],
  "downloading": null
}
```

## Backend Implementation

### Download Manager

New component to manage background downloads:

```go
type DownloadManager struct {
    cache      *VideoCache
    ytdlp      *ytdlp.YTDLP
    active     map[string]*Download  // videoID:quality -> Download
    mu         sync.RWMutex
    progressCh map[string][]chan DownloadProgress
}

type Download struct {
    VideoID   string
    Quality   string
    Status    string  // "downloading", "muxing", "complete", "error"
    Progress  float64
    Error     error
    StartedAt time.Time
}

type DownloadProgress struct {
    Quality         string  `json:"quality"`
    Percent         float64 `json:"percent"`
    BytesDownloaded int64   `json:"bytesDownloaded"`
    TotalBytes      int64   `json:"totalBytes"`
    Status          string  `json:"status"`
    Error           string  `json:"error,omitempty"`
}
```

### Download Process

1. Get video and audio URLs from yt-dlp
2. Download both to temp files (track progress via file size vs expected)
3. Mux with ffmpeg using `-movflags +faststart` for seekability
4. Move to cache directory
5. Notify subscribers via SSE

### Cache Structure

```
/cache/
  {videoID}/
    720.mp4
    1080.mp4
    metadata.json  # timestamps, sizes
```

### Stream Handler Changes

Remove all muxing-on-the-fly code. The stream handler becomes:

1. Check if quality is cached → serve file with range support
2. Otherwise → proxy progressive stream from YouTube with range passthrough

## Frontend Implementation

### Quality Selector State

```typescript
type QualityState = {
  available: string[];      // from /qualities endpoint
  cached: string[];         // ready to play
  downloading: string | null;
  progress: number;         // 0-100 when downloading
};
```

### UI Components

**Quality Button**:
- Default: Shows current quality (e.g., "Auto")
- With cached versions: Shows badge count
- When downloading: Shows spinner

**Quality Menu**:
- Lists available qualities
- Cached qualities show checkmark
- Downloading quality shows progress bar
- Tap uncached quality to start download

**Progress Overlay**:
- Appears over video thumbnail during download
- Shows percentage and estimated time
- Can be dismissed (download continues in background)

### SSE Connection

```typescript
function subscribeToProgress(videoId: string) {
  const es = new EventSource(`/api/videos/${videoId}/download/status`);

  es.addEventListener('progress', (e) => {
    const data = JSON.parse(e.data);
    updateProgress(data);
  });

  es.addEventListener('complete', (e) => {
    const data = JSON.parse(e.data);
    markQualityCached(data.quality);
    es.close();
  });

  es.addEventListener('error', (e) => {
    showError(e.data);
    es.close();
  });
}
```

## Files to Modify

### Backend

- `internal/api/handlers.go`: Simplify stream handler, add download endpoints
- `internal/api/server.go`: Add DownloadManager dependency
- `internal/api/download.go`: New file for download manager
- `internal/api/cache.go`: Update cache structure for per-quality files

### Frontend

- `web/frontend/src/routes/watch/[id]/+page.svelte`: Quality selector UI, progress overlay
- `web/frontend/src/lib/api.ts`: Add download and qualities API functions
- `web/frontend/src/lib/stores/downloads.ts`: New store for download state

## Migration

1. Remove all streaming-mux code (fragmented MP4, Matroska attempts)
2. Implement progressive proxy with range passthrough
3. Add download manager and endpoints
4. Update frontend quality selector
5. Add progress overlay

## Edge Cases

- **Download interrupted**: Resume not supported initially; restart download
- **Disk space**: Check available space before download; configurable limit
- **Concurrent downloads**: Allow one download per video at a time
- **Cache eviction**: LRU based on last access time, configurable max size
- **Video expired**: YouTube URLs expire; re-fetch if download fails

## Success Criteria

- Video plays immediately on page load with working seek
- User can request HD download with visible progress
- Cached HD version plays with full seek support
- No broken streaming states or empty responses
- Works reliably at 2x playback speed
