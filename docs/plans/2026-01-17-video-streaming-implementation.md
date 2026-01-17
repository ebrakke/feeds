# Video Streaming Quality Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace MSE-based adaptive streaming with server-side muxing and temp file caching for reliable 720p+ playback with full scrubbing support.

**Architecture:** Backend muxes video+audio via ffmpeg to temp files (1-hour cache), serves with range request support. Frontend simplified to basic `<video>` element.

**Tech Stack:** Go, ffmpeg, SvelteKit, yt-dlp

---

## Task 1: Create Video Cache Manager

**Files:**
- Create: `internal/api/cache.go`

**Step 1: Create the cache manager file**

```go
package api

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	videoCacheDir = "/tmp/feeds-video-cache"
	videoCacheTTL = 1 * time.Hour
	cacheCleanupInterval = 10 * time.Minute
)

// VideoCache manages cached muxed video files
type VideoCache struct {
	mu       sync.RWMutex
	muxing   map[string]chan struct{} // tracks in-progress muxing operations
}

// NewVideoCache creates a new video cache manager
func NewVideoCache() *VideoCache {
	// Ensure cache directory exists
	if err := os.MkdirAll(videoCacheDir, 0755); err != nil {
		log.Printf("Warning: could not create video cache dir: %v", err)
	}

	vc := &VideoCache{
		muxing: make(map[string]chan struct{}),
	}

	// Start cleanup goroutine
	go vc.cleanupLoop()

	return vc
}

// CacheKey generates a cache key for a video
func CacheKey(videoID, quality string) string {
	return videoID + "_" + quality
}

// CachePath returns the file path for a cached video
func (vc *VideoCache) CachePath(key string) string {
	return filepath.Join(videoCacheDir, key+".mp4")
}

// Get returns the path to a cached video if it exists and is valid
func (vc *VideoCache) Get(key string) (string, bool) {
	path := vc.CachePath(key)

	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}

	// Check if file is too old
	if time.Since(info.ModTime()) > videoCacheTTL {
		os.Remove(path)
		return "", false
	}

	// Check if file has content
	if info.Size() == 0 {
		os.Remove(path)
		return "", false
	}

	return path, true
}

// WaitForMuxing waits for an in-progress muxing operation to complete
// Returns true if we should proceed with muxing, false if another goroutine is handling it
func (vc *VideoCache) WaitForMuxing(key string) (shouldMux bool, done func()) {
	vc.mu.Lock()

	if ch, exists := vc.muxing[key]; exists {
		// Another goroutine is muxing, wait for it
		vc.mu.Unlock()
		<-ch
		return false, nil
	}

	// We're the first, create a channel for others to wait on
	ch := make(chan struct{})
	vc.muxing[key] = ch
	vc.mu.Unlock()

	return true, func() {
		vc.mu.Lock()
		delete(vc.muxing, key)
		close(ch)
		vc.mu.Unlock()
	}
}

// cleanupLoop periodically removes expired cache files
func (vc *VideoCache) cleanupLoop() {
	ticker := time.NewTicker(cacheCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		vc.cleanup()
	}
}

func (vc *VideoCache) cleanup() {
	entries, err := os.ReadDir(videoCacheDir)
	if err != nil {
		return
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > videoCacheTTL {
			path := filepath.Join(videoCacheDir, entry.Name())
			if err := os.Remove(path); err == nil {
				log.Printf("Cleaned up expired cache file: %s", entry.Name())
			}
		}
	}
}
```

**Step 2: Verify the file compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/api/cache.go
git commit -m "feat: add video cache manager for muxed files"
```

---

## Task 2: Rewrite Stream Handler with Muxing

**Files:**
- Modify: `internal/api/handlers.go:1211-1309`

**Step 1: Add VideoCache to Server struct**

In `internal/api/handlers.go`, find the Server struct (around line 40) and add the cache field:

```go
type Server struct {
	db         *db.DB
	ytdlp      *ytdlp.YTDLP
	templates  *template.Template
	videoCache *VideoCache  // Add this line
}
```

**Step 2: Initialize VideoCache in NewServer**

Find the NewServer function and add cache initialization:

```go
func NewServer(database *db.DB, ytdlpInstance *ytdlp.YTDLP) *Server {
	s := &Server{
		db:         database,
		ytdlp:      ytdlpInstance,
		videoCache: NewVideoCache(),  // Add this line
	}
	// ... rest of function
}
```

**Step 3: Rewrite handleStreamProxy function**

Replace the entire `handleStreamProxy` function (lines 1211-1309) with:

```go
func (s *Server) handleStreamProxy(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	quality := r.URL.Query().Get("quality")
	if quality == "" {
		quality = "720"
	}

	cacheKey := CacheKey(videoID, quality)

	// Check cache first
	if cachedPath, ok := s.videoCache.Get(cacheKey); ok {
		log.Printf("Serving cached video: %s", cacheKey)
		http.ServeFile(w, r, cachedPath)
		return
	}

	// Check if another request is already muxing this video
	shouldMux, done := s.videoCache.WaitForMuxing(cacheKey)
	if !shouldMux {
		// Another goroutine finished muxing, check cache again
		if cachedPath, ok := s.videoCache.Get(cacheKey); ok {
			http.ServeFile(w, r, cachedPath)
			return
		}
		// Muxing failed, fall through to try again or fallback
	}

	if done != nil {
		defer done()
	}

	// Get video and audio URLs
	videoURL := "https://www.youtube.com/watch?v=" + videoID
	videoStreamURL, audioStreamURL, err := s.ytdlp.GetAdaptiveStreamURLs(videoURL, quality)
	if err != nil {
		log.Printf("Failed to get adaptive URLs for %s: %v, trying progressive", videoID, err)
		s.serveProgressiveFallback(w, r, videoID, quality)
		return
	}

	// If no separate audio, try progressive
	if audioStreamURL == "" {
		log.Printf("No separate audio for %s, using progressive", videoID)
		s.serveProgressiveFallback(w, r, videoID, quality)
		return
	}

	// Mux video and audio to cache file
	outputPath := s.videoCache.CachePath(cacheKey)
	tempPath := outputPath + ".tmp"

	log.Printf("Muxing video %s at quality %s", videoID, quality)

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", videoStreamURL,
		"-i", audioStreamURL,
		"-c", "copy",
		"-movflags", "+faststart",
		"-f", "mp4",
		tempPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("ffmpeg mux failed for %s: %v, stderr: %s", videoID, err, stderr.String())
		os.Remove(tempPath)
		s.serveProgressiveFallback(w, r, videoID, quality)
		return
	}

	// Move temp file to final location
	if err := os.Rename(tempPath, outputPath); err != nil {
		log.Printf("Failed to rename temp file for %s: %v", videoID, err)
		os.Remove(tempPath)
		s.serveProgressiveFallback(w, r, videoID, quality)
		return
	}

	log.Printf("Successfully muxed video %s, serving from cache", videoID)
	http.ServeFile(w, r, outputPath)
}

func (s *Server) serveProgressiveFallback(w http.ResponseWriter, r *http.Request, videoID, quality string) {
	videoURL := "https://www.youtube.com/watch?v=" + videoID
	streamURL, err := s.ytdlp.GetStreamURL(videoURL, quality)
	if err != nil {
		log.Printf("Failed to get progressive stream URL for %s: %v", videoID, err)
		http.Error(w, "Failed to get stream URL", http.StatusBadGateway)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, streamURL, nil)
	if err != nil {
		log.Printf("Failed to create upstream request for %s: %v", videoID, err)
		http.Error(w, "Failed to start stream", http.StatusInternalServerError)
		return
	}

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Feeds/1.0)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		log.Printf("Upstream stream request failed for %s: %v", videoID, err)
		http.Error(w, "Failed to start stream", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		if strings.EqualFold(key, "Transfer-Encoding") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
```

**Step 4: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/api/handlers.go internal/api/cache.go
git commit -m "feat: implement server-side muxing with temp file cache"
```

---

## Task 3: Remove Unused Backend Endpoints

**Files:**
- Modify: `internal/api/handlers.go`

**Step 1: Remove route registrations**

Find lines 131-132 in handlers.go and remove:

```go
// DELETE these two lines:
mux.HandleFunc("GET /api/stream-urls/{id}", s.handleStreamURLs)
mux.HandleFunc("GET /api/proxy-stream/{id}/{type}", s.handleProxyStream)
```

**Step 2: Remove handleStreamURLs function**

Delete the entire `handleStreamURLs` function (lines 1076-1091):

```go
// DELETE this entire function
func (s *Server) handleStreamURLs(w http.ResponseWriter, r *http.Request) {
	// ...
}
```

**Step 3: Remove handleProxyStream function**

Delete the entire `handleProxyStream` function (lines 1141-1209):

```go
// DELETE this entire function
func (s *Server) handleProxyStream(w http.ResponseWriter, r *http.Request) {
	// ...
}
```

**Step 4: Remove streamURLCache and related code**

Delete the cache struct and helper function (lines 1093-1139):

```go
// DELETE all of this:
var streamURLCache = struct {
	sync.RWMutex
	urls map[string]cachedStreamURLs
}{urls: make(map[string]cachedStreamURLs)}

type cachedStreamURLs struct {
	// ...
}

func (s *Server) getOrFetchStreamURLs(videoID, quality string) (string, string, error) {
	// ...
}
```

**Step 5: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 6: Commit**

```bash
git add internal/api/handlers.go
git commit -m "refactor: remove unused stream-urls and proxy-stream endpoints"
```

---

## Task 4: Simplify Frontend - Remove MSE Code

**Files:**
- Modify: `web/frontend/src/routes/watch/[id]/+page.svelte`

**Step 1: Remove MSE-related state variables**

Find and delete these state variables (around lines 20, 26-36):

```typescript
// DELETE these lines:
let useAdaptiveStreaming = $state(false);
let currentVideoURL = $state('');
let currentAudioURL = $state('');
let usingMSE = $state(false);
let progressiveURL = $state('');
let pendingSeekTime = $state<number | null>(null);
let adaptiveLoadKey = '';

let mediaSource: MediaSource | null = null;
let mediaObjectURL = '';
let videoAbort: AbortController | null = null;
let audioAbort: AbortController | null = null;
```

**Step 2: Add simple streamURL state**

Add this single state variable instead:

```typescript
let streamURL = $derived(`/api/stream/${videoId}?quality=${selectedQuality}`);
```

**Step 3: Remove MSE helper functions**

Delete these functions (around lines 256-464):
- `cleanupMediaSource()`
- `buildMimeType()`
- `getContentType()`
- `waitForUpdateEnd()`
- `appendChunk()`
- `streamToBuffer()`
- `handleLoadStream()`

**Step 4: Simplify the quality change effect**

Replace the effect at lines 474-493 with:

```typescript
$effect(() => {
	if (loading || error || !videoElement) return;

	// Quality changed, update video source
	const newURL = `/api/stream/${videoId}?quality=${selectedQuality}`;
	if (videoElement.src !== newURL && videoElement.src !== location.origin + newURL) {
		const currentTime = videoElement.currentTime;
		const wasPlaying = !videoElement.paused;

		videoElement.src = newURL;
		videoElement.load();

		// Restore position after load
		videoElement.addEventListener('loadedmetadata', () => {
			if (currentTime > 0) {
				videoElement.currentTime = currentTime;
			}
			if (wasPlaying) {
				videoElement.play().catch(() => {});
			}
		}, { once: true });
	}
});
```

**Step 5: Remove the second effect block**

Delete the effect block that handled adaptive streaming (the one checking `useAdaptiveStreaming`).

**Step 6: Simplify handleSeeking**

Replace `handleSeeking` function with empty or remove entirely:

```typescript
function handleSeeking() {
	// No longer needed - range requests handle seeking
}
```

**Step 7: Simplify loadVideo cleanup**

In `loadVideo()`, remove these cleanup lines:

```typescript
// DELETE these from loadVideo():
cleanupMediaSource();
currentVideoURL = '';
currentAudioURL = '';
progressiveURL = '';
pendingSeekTime = null;
adaptiveLoadKey = '';
usingMSE = false;
```

**Step 8: Simplify onDestroy**

Replace onDestroy:

```typescript
onDestroy(() => {
	saveProgress();
	if (videoElement) {
		videoElement.pause();
	}
});
```

**Step 9: Commit**

```bash
git add web/frontend/src/routes/watch/[id]/+page.svelte
git commit -m "refactor: remove MSE/adaptive streaming code from frontend"
```

---

## Task 5: Simplify Frontend - Remove Adaptive UI

**Files:**
- Modify: `web/frontend/src/routes/watch/[id]/+page.svelte`

**Step 1: Update video element**

Find the video element (around line 657) and simplify:

```svelte
<video
	bind:this={videoElement}
	class="w-full h-full"
	controls
	preload="auto"
	playsinline
	poster={thumbnailURL || undefined}
	src={`/api/stream/${videoId}?quality=${selectedQuality}`}
	onloadedmetadata={handleLoadedMetadata}
	onloadeddata={handleVideoLoaded}
	ontimeupdate={handleTimeUpdate}
	onpause={handlePause}
>
	Your browser does not support the video tag.
</video>
```

Remove `onseeking={handleSeeking}` since it's no longer needed.

**Step 2: Remove adaptive streaming toggle from settings**

Find the settings dropdown (around lines 805-831) and remove the adaptive streaming checkbox:

```svelte
<!-- DELETE this entire label block: -->
<label class="settings-option">
	<input
		type="checkbox"
		bind:checked={useAdaptiveStreaming}
		class="checkbox"
	/>
	<div class="settings-option-text">
		<span class="settings-option-label">Adaptive streaming</span>
		<span class="settings-option-desc">Higher quality (experimental)</span>
	</div>
</label>
{#if useAdaptiveStreaming}
	<button
		onclick={handleLoadStream}
		disabled={streamLoading}
		class="btn btn-secondary btn-sm w-full mt-2"
	>
		<!-- ... -->
	</button>
{/if}
```

**Step 3: Remove 4K/8K quality options**

Find the quality dropdown (around lines 748-763) and remove the conditional 4K/8K options:

```svelte
<!-- DELETE these lines inside the select: -->
{#if useAdaptiveStreaming}
	<option value="4320">8K</option>
	<option value="2160">4K</option>
	<option value="1440">1440p</option>
{/if}
```

**Step 4: Verify build**

Run: `cd /root/code/feeds/web/frontend && npm run build`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add web/frontend/src/routes/watch/[id]/+page.svelte
git commit -m "refactor: remove adaptive streaming toggle and 4K/8K options from UI"
```

---

## Task 6: Remove Unused API Function

**Files:**
- Modify: `web/frontend/src/lib/api.ts`

**Step 1: Remove getStreamURLs function**

Delete the entire function (lines 222-229):

```typescript
// DELETE this function:
export async function getStreamURLs(id: string, quality: string): Promise<{
	videoURL: string;
	audioURL: string;
	dashURL?: string;
}> {
	return fetchJSON(`/stream-urls/${id}?quality=${encodeURIComponent(quality)}`);
}
```

**Step 2: Remove import from watch page**

In `+page.svelte`, update the import at line 4:

```typescript
// Change this:
import { getVideoInfo, updateProgress, getFeeds, addChannel, deleteChannel, getNearbyVideos, getStreamURLs, getSegments } from '$lib/api';

// To this:
import { getVideoInfo, updateProgress, getFeeds, addChannel, deleteChannel, getNearbyVideos, getSegments } from '$lib/api';
```

**Step 3: Verify build**

Run: `cd /root/code/feeds/web/frontend && npm run build`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add web/frontend/src/lib/api.ts web/frontend/src/routes/watch/[id]/+page.svelte
git commit -m "refactor: remove unused getStreamURLs API function"
```

---

## Task 7: Final Cleanup and Testing

**Files:**
- Review: All modified files

**Step 1: Run full build**

```bash
cd /root/code/feeds && make build
```

Expected: Build succeeds

**Step 2: Start the server**

```bash
cd /root/code/feeds && make dev
```

Expected: Server starts without errors

**Step 3: Manual testing checklist**

Test in browser at http://localhost:8080:

1. [ ] Navigate to a video - should load at 720p
2. [ ] Video plays without errors
3. [ ] Seek to middle of video - should work
4. [ ] Change quality to 1080p - video reloads at new quality
5. [ ] Play at 2x speed - should be smooth
6. [ ] Check server logs - should see "Muxing video" then "Serving from cache"
7. [ ] Reload same video - should see "Serving cached video" (no muxing)
8. [ ] No "Adaptive streaming" toggle visible in settings

**Step 4: Check for TypeScript errors**

```bash
cd /root/code/feeds/web/frontend && npm run check
```

Expected: No errors

**Step 5: Final commit**

```bash
git add -A
git commit -m "chore: final cleanup for video streaming improvements"
```

---

## Summary of Changes

**Backend (`internal/api/`):**
- New `cache.go`: Video cache manager with 1-hour TTL
- Modified `handlers.go`:
  - Added VideoCache to Server
  - Rewrote handleStreamProxy to use muxing + caching
  - Removed handleStreamURLs, handleProxyStream, streamURLCache

**Frontend (`web/frontend/src/`):**
- Modified `routes/watch/[id]/+page.svelte`:
  - Removed ~200 lines of MSE code
  - Removed adaptive streaming toggle
  - Removed 4K/8K options
  - Simplified video element to basic src assignment
- Modified `lib/api.ts`:
  - Removed getStreamURLs function

**Net result:** Simpler, more reliable video streaming with server-side quality handling.
