# Video Streaming Quality Improvement

**Date:** 2026-01-17
**Status:** Approved

## Problem

Current video streaming has reliability issues:
- Progressive streaming caps at 360p in practice
- Adaptive streaming (experimental) rarely works well
- Seeking during adaptive playback falls back to progressive
- Cannot reliably scrub through videos
- Buffering issues at 2x playback speed

## Goals

1. Reliable 720p+ quality by default
2. Full video scrubbable from the start (hybrid: play immediately, buffer in background)
3. Stable enough for 2x playback
4. Simple UX - no experimental toggles or complex configuration

## Solution: Server-Side Muxing with Cached Temp Files

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Frontend                              │
│  Simple <video> element with src="/api/stream/{id}?q=720"   │
│  No MSE, no chunking, just standard HTML5 video             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Backend /api/stream                      │
│  1. Check cache for existing muxed file                     │
│  2. If miss: fetch video+audio URLs via yt-dlp              │
│  3. Mux with ffmpeg to temp file                            │
│  4. Serve file with full HTTP range request support         │
│  5. Cache for 1 hour                                        │
└─────────────────────────────────────────────────────────────┘
```

### Backend Flow

```
Request: GET /api/stream/{id}?quality=720
                    │
                    ▼
┌────────────────────────────────────────┐
│ 1. Generate cache key: "{id}_{quality}"│
└────────────────────────────────────────┘
                    │
                    ▼
┌────────────────────────────────────────┐
│ 2. Check cache directory for file      │
│    Path: /tmp/feeds-cache/{key}.mp4    │
│    Also check if still within 1hr TTL  │
└────────────────────────────────────────┘
                    │
          ┌────────┴────────┐
          │                 │
       HIT ▼              MISS ▼
┌──────────────┐    ┌─────────────────────────────┐
│ Serve file   │    │ 3. Fetch URLs via yt-dlp    │
│ with Range   │    │    - bestvideo[height<=Q]   │
│ support      │    │    - bestaudio              │
└──────────────┘    └─────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────────────────┐
                    │ 4. Mux with ffmpeg to file  │
                    │    ffmpeg -i video -i audio │
                    │    -c copy -movflags +faststart
                    │    output.mp4               │
                    └─────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────────────────┐
                    │ 5. Serve file with Range    │
                    │    request support          │
                    └─────────────────────────────┘
```

### Key Implementation Details

**ffmpeg flags:**
- `-c copy` - no re-encoding, just remux
- `-movflags +faststart` - moves moov atom to start for streaming

**Cache management:**
- Location: `/tmp/feeds-cache/`
- TTL: 1 hour
- Cleanup: periodic goroutine every 10 minutes
- Key format: `{videoId}_{quality}.mp4`

**Quality format string (yt-dlp):**
```
bestvideo[height<={q}][vcodec^=avc1]+bestaudio/bestvideo[height<={q}]+bestaudio
```

**Range request support:**
- Use `http.ServeFile()` or equivalent for automatic range handling
- Enables seeking and scrubbing

### Frontend Changes

**Remove:**
- MediaSource API code
- SourceBuffer management
- Chunk-by-chunk streaming (2MB segments)
- Fallback logic between adaptive/progressive
- "Adaptive streaming (experimental)" toggle
- 4K/8K quality options (H.264 caps at 1080p)
- `streamToBuffer()`, `cleanupMediaSource()`, abort controllers

**Simplified video element:**
```svelte
<video
  bind:this={videoElement}
  src="/api/stream/{videoId}?quality={selectedQuality}"
/>
```

**Quality dropdown (unchanged except defaults):**
```svelte
<select bind:value={selectedQuality}>
  <option value="1080">1080p</option>
  <option value="720">720p</option>  <!-- default -->
  <option value="480">480p</option>
  <option value="360">360p</option>
</select>
```

### Error Handling

| Scenario | Detection | Response |
|----------|-----------|----------|
| yt-dlp fails | Process exit code | Return 502, show "Video unavailable" |
| ffmpeg mux fails | Process exit code | Fall back to progressive stream |
| Age-restricted | yt-dlp error | Return 403, suggest cookies config |
| Quality unavailable | yt-dlp returns lower | Accept silently |
| Cache disk full | Write error | Clear oldest, retry |
| Client disconnects | Context cancelled | Keep muxing (cache for next viewer) |

**Progressive fallback:**
If muxing fails, redirect to progressive stream URL directly. Lower quality but something plays.

### Files Changed

**Modified:**
- `internal/api/handlers.go` - Rewrite `handleStreamProxy`
- `internal/ytdlp/ytdlp.go` - Simplify format selection
- `web/frontend/src/routes/watch/[id]/+page.svelte` - Remove MSE code
- `web/frontend/src/lib/api.ts` - Remove `getStreamURLs()`

**New:**
- `internal/api/cache.go` - Video cache manager

**Removed code:**
- ~300 lines of MSE/adaptive streaming frontend code
- Adaptive streaming toggle UI
- `/api/stream-urls/{id}` endpoint (no longer needed)
- `/api/proxy-stream/{id}/{type}` endpoint (no longer needed)

## Implementation Order

1. Backend cache system - temp file handling, TTL, cleanup goroutine
2. Backend streaming - mux to cache, serve with range support
3. Frontend cleanup - remove MSE code, simplify video element
4. Testing - verify seeking, 2x playback, quality changes
5. Cleanup - remove dead code and unused endpoints

## Success Criteria

- [ ] 720p plays reliably by default
- [ ] Full video is scrubbable after brief buffering
- [ ] 2x playback works without stuttering
- [ ] No "experimental" toggles visible to user
- [ ] Seeking works instantly on cached videos
- [ ] Graceful fallback if muxing fails
