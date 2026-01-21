# Buffered Streaming Design

## Problem

The current streaming implementation has two modes, both with issues:

1. **"Auto" (direct proxy)**: Unreliable, often only delivers 360p, breaks frequently
2. **Specific quality (download-first)**: Reliable but requires waiting 2+ minutes for full download before playback starts

Users want reliable 1080p/4K streaming that starts in 5-15 seconds, not minutes.

## Solution

Replace both modes with a single **buffered streaming** approach: start downloading immediately, wait for a 10-second buffer threshold, then begin serving the partial file while download continues in background.

## How It Works

```
Request: GET /api/stream/{id}?quality=1080
                    │
                    ▼
        ┌─────────────────────┐
        │ Check if cached?    │
        └─────────────────────┘
                    │
        ┌───────────┴───────────┐
        │                       │
     Cached               Not Cached
        │                       │
        ▼                       ▼
   Serve file           Start download
   immediately          (if not running)
                               │
                               ▼
                    Wait for buffer threshold
                    (~10 sec of video, 5-10 MB)
                               │
                               ▼
                    Begin serving partial file
                    (download continues)
                               │
                               ▼
                    Client plays video
                               │
                               ▼
                ┌──────────────┴──────────────┐
                │                             │
        Download completes           Client disconnects
                │                     (download incomplete)
                ▼                             │
        Keep in cache                         ▼
        (1-hour TTL)                   Delete partial file
```

## Key Behaviors

### Buffer Threshold

- Target: 10 seconds of video content
- Calculation: `bitrate * 10 / 8` bytes (bitrate from yt-dlp metadata)
- Fallback estimates if metadata unavailable:
  - 1080p: ~8 MB (assume ~6 Mbps bitrate)
  - 4K: ~20 MB (assume ~15 Mbps bitrate)
  - 720p: ~4 MB (assume ~3 Mbps bitrate)

### Quality Selection

- "auto" quality: Select best available up to 1080p using buffered approach
- Specific quality (720, 1080, 4K): Use buffered approach for that quality
- Remove the unreliable direct proxy mode entirely

### Seeking

- Seeking within downloaded portion: Works immediately via HTTP range requests
- Seeking past downloaded portion: Client waits for data (natural HTTP behavior)
- No special handling needed - existing range request support handles this

### Cleanup

- Fully downloaded files: Keep in cache with 1-hour TTL (existing behavior)
- Partial downloads with active readers: Keep serving
- Partial downloads with no readers: Delete immediately

## API Changes

### Stream Endpoint

```
GET /api/stream/{id}?quality={quality}
```

**Behavior changes:**
- No longer returns 404 for uncached qualities
- Blocks briefly (5-15 seconds) while buffering, then streams
- Returns appropriate Content-Length (estimated if still downloading)
- Supports range requests for seeking within downloaded portion

**Response headers:**
```
Content-Type: video/mp4
Content-Length: {estimated or actual size}
Accept-Ranges: bytes
X-Feeds-Buffered: true  # indicates buffered streaming mode
X-Feeds-Download-Progress: 45  # percentage downloaded (optional)
```

### Qualities Endpoint (unchanged)

```
GET /api/videos/{id}/qualities
```

Still returns available/cached/downloading status for UI purposes.

### Download Endpoint (unchanged)

```
POST /api/videos/{id}/download
```

Still works for explicit pre-download requests. The stream endpoint now implicitly triggers downloads.

## Implementation Details

### Modified Files

**`internal/api/stream.go`**

```go
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
    videoID := chi.URLParam(r, "id")
    quality := r.URL.Query().Get("quality")
    if quality == "" || quality == "auto" {
        quality = s.selectBestQuality(videoID) // New: pick best available
    }

    // Check cache first
    if cachedPath := s.videoCache.GetPath(videoID, quality); cachedPath != "" {
        s.serveFile(w, r, cachedPath)
        return
    }

    // Start or get existing download
    download := s.downloadManager.StartOrGet(videoID, quality)

    // Wait for buffer threshold
    threshold := s.calculateBufferThreshold(videoID, quality)
    if err := download.WaitForBytes(r.Context(), threshold); err != nil {
        http.Error(w, "Buffering failed", http.StatusServiceUnavailable)
        return
    }

    // Serve partial file with range support
    s.servePartialFile(w, r, download)
}
```

**`internal/api/download.go`**

Add to existing DownloadManager:

```go
// WaitForBytes blocks until the download has at least n bytes or context cancels
func (d *Download) WaitForBytes(ctx context.Context, n int64) error

// GetCurrentSize returns current downloaded size
func (d *Download) GetCurrentSize() int64

// GetEstimatedSize returns expected final size (from yt-dlp metadata)
func (d *Download) GetEstimatedSize() int64

// TrackReader registers an active reader (for cleanup tracking)
func (d *Download) TrackReader() func()
```

**`internal/api/cleanup.go`** (new)

```go
// PartialFileCleanup runs periodically and removes:
// - Partial downloads with no active readers
// - Downloads that have been stalled for > 5 minutes
func (s *Server) startPartialFileCleanup()
```

### New Helper Functions

```go
// selectBestQuality picks the highest available quality up to 1080p
func (s *Server) selectBestQuality(videoID string) string

// calculateBufferThreshold returns bytes needed for 10 sec buffer
func (s *Server) calculateBufferThreshold(videoID, quality string) int64

// servePartialFile serves a file that's still being written
func (s *Server) servePartialFile(w http.ResponseWriter, r *http.Request, d *Download)
```

## Testing Plan

1. **Mobile browser**: Select 1080p, verify playback starts in ~10-15 seconds
2. **Seeking**: Verify seeking within buffered content works
3. **Disconnect**: Close browser mid-stream, verify partial file is cleaned up
4. **Complete playback**: Watch to end, verify file stays in cache
5. **Re-watch**: Play same video again, verify instant start from cache

## Success Criteria

- Playback starts within 10-15 seconds for 1080p
- No more unreliable 360p "auto" streams
- Seeking works within downloaded portion
- Partial files don't accumulate on disk
- Existing Kodi plugin works without changes (just faster)

## Not In Scope

- Kodi plugin changes (will benefit automatically)
- Frontend UI changes (existing quality selector works)
- Resume interrupted downloads
- Seeking past downloaded portion (user waits naturally)
