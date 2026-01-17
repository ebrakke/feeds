# Download-First Video Streaming Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace broken streaming-mux approach with progressive streaming + explicit HD download.

**Architecture:** Default playback uses YouTube's progressive streams with range header passthrough. HD quality requires explicit download that muxes video+audio to a cached MP4 file. Frontend shows download progress via SSE and allows switching to cached versions.

**Tech Stack:** Go backend with ffmpeg for muxing, SvelteKit frontend with SSE for progress.

---

## Task 1: Simplify Stream Handler to Progressive Proxy

**Files:**
- Modify: `internal/api/handlers.go:1077-1174` (handleStreamProxy, streamMuxedVideo)

**Step 1: Replace streamMuxedVideo with progressive proxy**

Replace the entire `handleStreamProxy` and `streamMuxedVideo` functions with a simple progressive proxy that passes range headers through:

```go
func (s *Server) handleStreamProxy(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	quality := r.URL.Query().Get("quality")
	if quality == "" {
		quality = "auto"
	}

	// For non-auto quality, check cache first
	if quality != "auto" {
		cacheKey := CacheKey(videoID, quality)
		if cachedPath, ok := s.videoCache.Get(cacheKey); ok {
			log.Printf("Serving cached video: %s", cacheKey)
			http.ServeFile(w, r, cachedPath)
			return
		}
		// No cache for this quality - return 404, client should use auto
		http.Error(w, "Quality not cached", http.StatusNotFound)
		return
	}

	// Auto quality: proxy progressive stream from YouTube
	s.proxyProgressiveStream(w, r, videoID)
}

func (s *Server) proxyProgressiveStream(w http.ResponseWriter, r *http.Request, videoID string) {
	videoURL := "https://www.youtube.com/watch?v=" + videoID

	// Get progressive stream URL (combined video+audio, typically up to 720p)
	streamURL, err := s.ytdlp.GetStreamURL(videoURL, "best")
	if err != nil {
		log.Printf("Failed to get stream URL for %s: %v", videoID, err)
		http.Error(w, "Failed to get stream URL", http.StatusInternalServerError)
		return
	}

	// Create request to YouTube with range header passthrough
	req, err := http.NewRequestWithContext(r.Context(), "GET", streamURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Forward range header for seeking support
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	client := &http.Client{Timeout: 0} // No timeout for streaming
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch stream for %s: %v", videoID, err)
		http.Error(w, "Failed to fetch stream", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream the response
	io.Copy(w, resp.Body)
}
```

**Step 2: Remove dead code**

Delete `streamMuxedVideo` and `muxInBackground` functions (lines ~1099-1240).

**Step 3: Delete serveProgressiveFallback**

Find and delete `serveProgressiveFallback` function - it's no longer needed as progressive is now the default.

**Step 4: Test manually**

Run: `make dev`
Test: Open browser to watch page, video should play with seeking working.

**Step 5: Commit**

```bash
git add internal/api/handlers.go
git commit -m "refactor: simplify stream handler to progressive proxy with range passthrough"
```

---

## Task 2: Add Download Manager Component

**Files:**
- Create: `internal/api/download.go`

**Step 1: Create download manager file**

```go
package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/erik/feeds/internal/ytdlp"
)

// DownloadManager handles background video downloads and muxing
type DownloadManager struct {
	cache     *VideoCache
	ytdlp     *ytdlp.YTDLP
	active    map[string]*Download
	mu        sync.RWMutex
	listeners map[string][]chan DownloadProgress
}

// Download represents an in-progress download
type Download struct {
	VideoID   string
	Quality   string
	Status    string // "downloading", "muxing", "complete", "error"
	Progress  float64
	Error     string
	StartedAt time.Time
}

// DownloadProgress is sent to SSE clients
type DownloadProgress struct {
	Quality         string  `json:"quality"`
	Percent         float64 `json:"percent"`
	BytesDownloaded int64   `json:"bytesDownloaded"`
	TotalBytes      int64   `json:"totalBytes"`
	Status          string  `json:"status"`
	Error           string  `json:"error,omitempty"`
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(cache *VideoCache, yt *ytdlp.YTDLP) *DownloadManager {
	return &DownloadManager{
		cache:     cache,
		ytdlp:     yt,
		active:    make(map[string]*Download),
		listeners: make(map[string][]chan DownloadProgress),
	}
}

func downloadKey(videoID, quality string) string {
	return videoID + ":" + quality
}

// StartDownload initiates a background download for the given video and quality
func (dm *DownloadManager) StartDownload(videoID, quality string) (*Download, error) {
	key := downloadKey(videoID, quality)
	cacheKey := CacheKey(videoID, quality)

	// Check if already cached
	if _, ok := dm.cache.Get(cacheKey); ok {
		return &Download{
			VideoID: videoID,
			Quality: quality,
			Status:  "complete",
		}, nil
	}

	dm.mu.Lock()
	// Check if already downloading
	if d, exists := dm.active[key]; exists {
		dm.mu.Unlock()
		return d, nil
	}

	// Create new download
	d := &Download{
		VideoID:   videoID,
		Quality:   quality,
		Status:    "downloading",
		StartedAt: time.Now(),
	}
	dm.active[key] = d
	dm.mu.Unlock()

	// Start download in background
	go dm.runDownload(videoID, quality, key, cacheKey)

	return d, nil
}

// GetStatus returns current download status for a video
func (dm *DownloadManager) GetStatus(videoID string) map[string]*Download {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make(map[string]*Download)
	for key, d := range dm.active {
		if d.VideoID == videoID {
			result[d.Quality] = d
		}
	}
	return result
}

// Subscribe returns a channel that receives progress updates for a video
func (dm *DownloadManager) Subscribe(videoID string) chan DownloadProgress {
	ch := make(chan DownloadProgress, 10)

	dm.mu.Lock()
	dm.listeners[videoID] = append(dm.listeners[videoID], ch)
	dm.mu.Unlock()

	return ch
}

// Unsubscribe removes a progress listener
func (dm *DownloadManager) Unsubscribe(videoID string, ch chan DownloadProgress) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	listeners := dm.listeners[videoID]
	for i, listener := range listeners {
		if listener == ch {
			dm.listeners[videoID] = append(listeners[:i], listeners[i+1:]...)
			close(ch)
			break
		}
	}
}

func (dm *DownloadManager) broadcast(videoID string, progress DownloadProgress) {
	dm.mu.RLock()
	listeners := dm.listeners[videoID]
	dm.mu.RUnlock()

	for _, ch := range listeners {
		select {
		case ch <- progress:
		default:
			// Skip if channel buffer is full
		}
	}
}

func (dm *DownloadManager) runDownload(videoID, quality, key, cacheKey string) {
	defer func() {
		dm.mu.Lock()
		delete(dm.active, key)
		dm.mu.Unlock()
	}()

	videoURL := "https://www.youtube.com/watch?v=" + videoID

	// Get adaptive stream URLs
	videoStreamURL, audioStreamURL, err := dm.ytdlp.GetAdaptiveStreamURLs(videoURL, quality)
	if err != nil {
		dm.setError(key, videoID, quality, fmt.Sprintf("Failed to get stream URLs: %v", err))
		return
	}

	if audioStreamURL == "" {
		dm.setError(key, videoID, quality, "No separate audio stream available for this quality")
		return
	}

	// Create temp directory for this download
	tempDir := filepath.Join(os.TempDir(), "feeds-download-"+videoID+"-"+quality)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		dm.setError(key, videoID, quality, fmt.Sprintf("Failed to create temp dir: %v", err))
		return
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, "video.mp4")
	audioPath := filepath.Join(tempDir, "audio.m4a")

	// Download video and audio in parallel
	var wg sync.WaitGroup
	var videoErr, audioErr error
	var videoSize, audioSize int64

	wg.Add(2)

	go func() {
		defer wg.Done()
		videoSize, videoErr = dm.downloadFile(videoStreamURL, videoPath)
	}()

	go func() {
		defer wg.Done()
		audioSize, audioErr = dm.downloadFile(audioStreamURL, audioPath)
	}()

	// Monitor progress while downloading
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				dm.mu.RLock()
				d, exists := dm.active[key]
				dm.mu.RUnlock()
				if !exists {
					return
				}

				var downloaded int64
				if info, err := os.Stat(videoPath); err == nil {
					downloaded += info.Size()
				}
				if info, err := os.Stat(audioPath); err == nil {
					downloaded += info.Size()
				}

				// Estimate total (we don't know exact size upfront)
				// Update with rough progress
				progress := DownloadProgress{
					Quality:         quality,
					BytesDownloaded: downloaded,
					Status:          d.Status,
				}
				dm.broadcast(videoID, progress)
			}
		}
	}()

	wg.Wait()

	if videoErr != nil {
		dm.setError(key, videoID, quality, fmt.Sprintf("Video download failed: %v", videoErr))
		return
	}
	if audioErr != nil {
		dm.setError(key, videoID, quality, fmt.Sprintf("Audio download failed: %v", audioErr))
		return
	}

	// Update status to muxing
	dm.mu.Lock()
	if d, exists := dm.active[key]; exists {
		d.Status = "muxing"
	}
	dm.mu.Unlock()

	dm.broadcast(videoID, DownloadProgress{
		Quality:         quality,
		BytesDownloaded: videoSize + audioSize,
		TotalBytes:      videoSize + audioSize,
		Status:          "muxing",
	})

	// Mux with ffmpeg
	outputPath := dm.cache.CachePath(cacheKey)
	tempOutput := outputPath + ".tmp"

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-c", "copy",
		"-movflags", "+faststart",
		"-f", "mp4",
		tempOutput,
	)

	if err := cmd.Run(); err != nil {
		dm.setError(key, videoID, quality, fmt.Sprintf("Muxing failed: %v", err))
		return
	}

	// Move to final location
	if err := os.Rename(tempOutput, outputPath); err != nil {
		dm.setError(key, videoID, quality, fmt.Sprintf("Failed to save file: %v", err))
		return
	}

	log.Printf("Download complete: %s quality %s", videoID, quality)

	dm.broadcast(videoID, DownloadProgress{
		Quality: quality,
		Percent: 100,
		Status:  "complete",
	})
}

func (dm *DownloadManager) setError(key, videoID, quality, errMsg string) {
	dm.mu.Lock()
	if d, exists := dm.active[key]; exists {
		d.Status = "error"
		d.Error = errMsg
	}
	dm.mu.Unlock()

	log.Printf("Download error for %s quality %s: %s", videoID, quality, errMsg)

	dm.broadcast(videoID, DownloadProgress{
		Quality: quality,
		Status:  "error",
		Error:   errMsg,
	})
}

func (dm *DownloadManager) downloadFile(url, destPath string) (int64, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	return n, err
}
```

**Step 2: Verify it compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/api/download.go
git commit -m "feat: add download manager for background video muxing"
```

---

## Task 3: Wire Download Manager into Server

**Files:**
- Modify: `internal/api/handlers.go:29-80` (Server struct and NewServer)

**Step 1: Add downloadManager field to Server struct**

In `internal/api/handlers.go`, find the Server struct (around line 29) and add:

```go
type Server struct {
	db              *db.DB
	ytdlp           *ytdlp.YTDLP
	ai              *ai.Client
	sponsorblock    *sponsorblock.Client
	templates       *template.Template
	packs           fs.FS
	videoCache      *VideoCache
	downloadManager *DownloadManager  // Add this line

	// Stream URL cache (video ID -> cached entry)
	streamCache   map[string]*streamCacheEntry
	streamCacheMu sync.RWMutex
}
```

**Step 2: Initialize downloadManager in NewServer**

In the `NewServer` function (around line 53), add initialization:

```go
	videoCache := NewVideoCache()

	return &Server{
		db:              database,
		ytdlp:           yt,
		ai:              aiClient,
		sponsorblock:    sponsorblock.NewClient(),
		templates:       tmpl,
		packs:           packsFS,
		videoCache:      videoCache,
		downloadManager: NewDownloadManager(videoCache, yt),  // Add this line
		streamCache:     make(map[string]*streamCacheEntry),
	}, nil
```

**Step 3: Verify it compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/api/handlers.go
git commit -m "feat: wire download manager into server"
```

---

## Task 4: Add Download API Endpoints

**Files:**
- Modify: `internal/api/handlers.go` (add routes and handlers)

**Step 1: Register new routes**

In `RegisterRoutes` function, add after the existing video routes:

```go
	mux.HandleFunc("POST /api/videos/{id}/download", s.handleStartDownload)
	mux.HandleFunc("GET /api/videos/{id}/download/status", s.handleDownloadStatus)
	mux.HandleFunc("GET /api/videos/{id}/qualities", s.handleGetQualities)
```

**Step 2: Add handleStartDownload handler**

```go
func (s *Server) handleStartDownload(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	var req struct {
		Quality string `json:"quality"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Quality == "" || req.Quality == "auto" {
		http.Error(w, "Quality must be specified (e.g., 720, 1080)", http.StatusBadRequest)
		return
	}

	download, err := s.downloadManager.StartDownload(videoID, req.Quality)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  download.Status,
		"quality": download.Quality,
	})
}
```

**Step 3: Add handleDownloadStatus SSE handler**

```go
func (s *Server) handleDownloadStatus(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	ch := s.downloadManager.Subscribe(videoID)
	defer s.downloadManager.Unsubscribe(videoID, ch)

	// Send current status first
	status := s.downloadManager.GetStatus(videoID)
	for quality, d := range status {
		data, _ := json.Marshal(DownloadProgress{
			Quality: quality,
			Status:  d.Status,
			Error:   d.Error,
		})
		fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case progress, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(progress)
			fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
			flusher.Flush()

			if progress.Status == "complete" || progress.Status == "error" {
				return
			}
		}
	}
}
```

**Step 4: Add handleGetQualities handler**

```go
func (s *Server) handleGetQualities(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	// Available qualities (hardcoded for now, could query yt-dlp)
	available := []string{"360", "480", "720", "1080"}

	// Check which are cached
	var cached []string
	for _, q := range available {
		cacheKey := CacheKey(videoID, q)
		if _, ok := s.videoCache.Get(cacheKey); ok {
			cached = append(cached, q)
		}
	}

	// Check which is downloading
	var downloading *string
	status := s.downloadManager.GetStatus(videoID)
	for quality, d := range status {
		if d.Status == "downloading" || d.Status == "muxing" {
			downloading = &quality
			break
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"available":   available,
		"cached":      cached,
		"downloading": downloading,
	})
}
```

**Step 5: Verify it compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 6: Commit**

```bash
git add internal/api/handlers.go
git commit -m "feat: add download API endpoints"
```

---

## Task 5: Add Frontend API Functions

**Files:**
- Modify: `web/frontend/src/lib/api.ts`

**Step 1: Add download API functions**

Add at the end of the file:

```typescript
// Video Downloads
export async function startDownload(videoId: string, quality: string): Promise<{
	status: string;
	quality: string;
}> {
	return fetchJSON(`/videos/${videoId}/download`, {
		method: 'POST',
		body: JSON.stringify({ quality })
	});
}

export async function getQualities(videoId: string): Promise<{
	available: string[];
	cached: string[];
	downloading: string | null;
}> {
	return fetchJSON(`/videos/${videoId}/qualities`);
}

export function subscribeToDownloadProgress(
	videoId: string,
	onProgress: (data: { quality: string; percent: number; status: string; error?: string }) => void
): () => void {
	const es = new EventSource(`/api/videos/${videoId}/download/status`);

	es.addEventListener('progress', (e) => {
		const data = JSON.parse(e.data);
		onProgress(data);
	});

	es.addEventListener('status', (e) => {
		const data = JSON.parse(e.data);
		onProgress(data);
	});

	es.onerror = () => {
		es.close();
	};

	return () => es.close();
}
```

**Step 2: Verify TypeScript compiles**

Run: `cd /root/code/feeds/web/frontend && npm run check`
Expected: No errors

**Step 3: Commit**

```bash
git add web/frontend/src/lib/api.ts
git commit -m "feat: add download API functions to frontend"
```

---

## Task 6: Update Watch Page Quality Selector

**Files:**
- Modify: `web/frontend/src/routes/watch/[id]/+page.svelte`

**Step 1: Add import and state for downloads**

At the top of the script section, update imports:

```typescript
import { getVideoInfo, updateProgress, getFeeds, addChannel, deleteChannel, getNearbyVideos, getSegments, getQualities, startDownload, subscribeToDownloadProgress } from '$lib/api';
```

Add new state variables after the existing ones (around line 30):

```typescript
	// Download state
	let availableQualities = $state<string[]>([]);
	let cachedQualities = $state<string[]>([]);
	let downloadingQuality = $state<string | null>(null);
	let downloadProgress = $state(0);
	let downloadError = $state<string | null>(null);
	let unsubscribeProgress: (() => void) | null = null;
```

**Step 2: Load qualities when video loads**

In the `loadVideo` function, after loading the video info successfully (around line 220), add:

```typescript
		// Load available qualities
		try {
			const qualities = await getQualities(id);
			availableQualities = qualities.available;
			cachedQualities = qualities.cached || [];
			downloadingQuality = qualities.downloading;
		} catch (e) {
			console.warn('Failed to load qualities:', e);
			availableQualities = ['360', '480', '720', '1080'];
		}
```

**Step 3: Add download functions**

Add after the existing functions (around line 560):

```typescript
	async function handleQualitySelect(quality: string) {
		// If this quality is cached, switch to it
		if (cachedQualities.includes(quality)) {
			selectedQuality = quality;
			return;
		}

		// If auto, just switch
		if (quality === 'auto') {
			selectedQuality = 'auto';
			return;
		}

		// Otherwise, start download
		downloadError = null;
		try {
			const result = await startDownload(videoId, quality);
			if (result.status === 'complete') {
				cachedQualities = [...cachedQualities, quality];
				selectedQuality = quality;
			} else {
				downloadingQuality = quality;
				downloadProgress = 0;

				// Subscribe to progress updates
				if (unsubscribeProgress) {
					unsubscribeProgress();
				}
				unsubscribeProgress = subscribeToDownloadProgress(videoId, (data) => {
					if (data.status === 'complete') {
						cachedQualities = [...cachedQualities, data.quality];
						downloadingQuality = null;
						downloadProgress = 0;
						if (unsubscribeProgress) {
							unsubscribeProgress();
							unsubscribeProgress = null;
						}
					} else if (data.status === 'error') {
						downloadError = data.error || 'Download failed';
						downloadingQuality = null;
						downloadProgress = 0;
						if (unsubscribeProgress) {
							unsubscribeProgress();
							unsubscribeProgress = null;
						}
					} else {
						downloadProgress = data.percent || 0;
					}
				});
			}
		} catch (e) {
			downloadError = e instanceof Error ? e.message : 'Failed to start download';
		}
	}
```

**Step 4: Update onDestroy to clean up**

Update the `onDestroy` function:

```typescript
	onDestroy(() => {
		saveProgress();
		if (videoElement) {
			videoElement.pause();
		}
		if (unsubscribeProgress) {
			unsubscribeProgress();
		}
	});
```

**Step 5: Commit**

```bash
git add web/frontend/src/routes/watch/[id]/+page.svelte
git commit -m "feat: add quality download logic to watch page"
```

---

## Task 7: Update Quality Selector UI

**Files:**
- Modify: `web/frontend/src/routes/watch/[id]/+page.svelte`

**Step 1: Replace quality selector HTML**

Find the quality selector section (around line 746) and replace:

```svelte
						<!-- Quality Selector -->
						<div class="control-group">
							<label class="control-label">Quality</label>
							<div class="quality-selector">
								<button
									class="quality-btn"
									class:active={selectedQuality === 'auto'}
									onclick={() => handleQualitySelect('auto')}
								>
									Auto
								</button>
								{#each availableQualities as q}
									<button
										class="quality-btn"
										class:active={selectedQuality === q}
										class:cached={cachedQualities.includes(q)}
										class:downloading={downloadingQuality === q}
										onclick={() => handleQualitySelect(q)}
										disabled={downloadingQuality !== null && downloadingQuality !== q}
									>
										{q}p
										{#if cachedQualities.includes(q)}
											<span class="quality-badge">✓</span>
										{:else if downloadingQuality === q}
											<span class="quality-progress">{Math.round(downloadProgress)}%</span>
										{/if}
									</button>
								{/each}
							</div>
							{#if downloadError}
								<p class="text-red-500 text-sm mt-1">{downloadError}</p>
							{/if}
						</div>
```

**Step 2: Add quality selector styles**

Add to the style section at the end of the file:

```svelte
<style>
	/* ... existing styles ... */

	.quality-selector {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.quality-btn {
		padding: 0.375rem 0.75rem;
		border-radius: 0.375rem;
		font-size: 0.875rem;
		background: var(--surface-secondary);
		color: var(--text-secondary);
		border: 1px solid var(--border);
		cursor: pointer;
		transition: all 0.15s;
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}

	.quality-btn:hover:not(:disabled) {
		background: var(--surface-tertiary);
		color: var(--text-primary);
	}

	.quality-btn.active {
		background: var(--primary);
		color: white;
		border-color: var(--primary);
	}

	.quality-btn.cached {
		border-color: var(--success);
	}

	.quality-btn.downloading {
		background: var(--surface-tertiary);
	}

	.quality-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.quality-badge {
		font-size: 0.75rem;
		color: var(--success);
	}

	.quality-progress {
		font-size: 0.75rem;
		color: var(--primary);
	}
</style>
```

**Step 3: Verify it renders**

Run: `make dev`
Test: Open watch page, verify quality buttons display correctly.

**Step 4: Commit**

```bash
git add web/frontend/src/routes/watch/[id]/+page.svelte
git commit -m "feat: update quality selector UI with download state"
```

---

## Task 8: Add Download Progress Overlay

**Files:**
- Modify: `web/frontend/src/routes/watch/[id]/+page.svelte`

**Step 1: Add progress overlay HTML**

Find the video element wrapper (around line 656) and add the overlay inside the video container:

```svelte
				<div class="video-container relative">
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						bind:this={videoElement}
						class="w-full h-full"
						controls
						preload="auto"
						playsinline
						poster={thumbnailURL || undefined}
						onloadedmetadata={handleLoadedMetadata}
						onloadeddata={handleVideoLoaded}
						ontimeupdate={handleTimeUpdate}
						onpause={handlePause}
					>
						Your browser does not support the video tag.
					</video>

					<!-- Download Progress Overlay -->
					{#if downloadingQuality}
						<div class="download-overlay">
							<div class="download-overlay-content">
								<div class="download-spinner"></div>
								<span class="download-text">Downloading {downloadingQuality}p...</span>
								<div class="download-progress-bar">
									<div class="download-progress-fill" style="width: {downloadProgress}%"></div>
								</div>
								<span class="download-percent">{Math.round(downloadProgress)}%</span>
							</div>
						</div>
					{/if}

					<!-- Skip Notice -->
					{#if showSkipNotice}
						<!-- ... existing skip notice ... -->
					{/if}
				</div>
```

**Step 2: Add overlay styles**

Add to the style section:

```css
	.download-overlay {
		position: absolute;
		bottom: 0;
		left: 0;
		right: 0;
		background: linear-gradient(to top, rgba(0,0,0,0.8), transparent);
		padding: 1rem;
		pointer-events: none;
	}

	.download-overlay-content {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		color: white;
	}

	.download-spinner {
		width: 1rem;
		height: 1rem;
		border: 2px solid rgba(255,255,255,0.3);
		border-top-color: white;
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.download-text {
		font-size: 0.875rem;
		font-weight: 500;
	}

	.download-progress-bar {
		flex: 1;
		height: 4px;
		background: rgba(255,255,255,0.2);
		border-radius: 2px;
		overflow: hidden;
	}

	.download-progress-fill {
		height: 100%;
		background: var(--primary);
		transition: width 0.3s ease;
	}

	.download-percent {
		font-size: 0.875rem;
		font-weight: 600;
		min-width: 3rem;
		text-align: right;
	}
```

**Step 3: Test the overlay**

Run: `make dev`
Test: Start a download and verify overlay appears at bottom of video.

**Step 4: Commit**

```bash
git add web/frontend/src/routes/watch/[id]/+page.svelte
git commit -m "feat: add download progress overlay on video"
```

---

## Task 9: Update Stream Effect to Use Quality State

**Files:**
- Modify: `web/frontend/src/routes/watch/[id]/+page.svelte`

**Step 1: Update stream URL effect**

Find the effect that builds the stream URL (around line 265) and update:

```typescript
	$effect(() => {
		if (loading || error || !videoElement) return;

		// For non-auto qualities, only load if cached
		const effectiveQuality = selectedQuality === 'auto' || cachedQualities.includes(selectedQuality)
			? selectedQuality
			: 'auto';

		// Build the stream URL
		const newURL = `/api/stream/${videoId}?quality=${effectiveQuality}`;

		// Skip if we already loaded this exact URL
		if (lastLoadedURL === newURL) return;
		lastLoadedURL = newURL;

		const currentTime = videoElement.currentTime;
		const wasPlaying = !videoElement.paused;
		const video = videoElement;

		video.src = newURL;
		video.load();

		// Restore position after load
		video.addEventListener('loadedmetadata', () => {
			if (currentTime > 0) {
				video.currentTime = currentTime;
			}
			if (wasPlaying) {
				video.play().catch(() => {});
			}
		}, { once: true });
	});
```

**Step 2: Verify it works**

Run: `make dev`
Test:
- Page load plays auto quality
- Selecting uncached quality starts download but keeps playing auto
- When download completes, selecting that quality switches video

**Step 3: Commit**

```bash
git add web/frontend/src/routes/watch/[id]/+page.svelte
git commit -m "feat: stream URL effect respects cached quality state"
```

---

## Task 10: Clean Up Dead Code

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/cache.go`

**Step 1: Remove muxInBackground and related code from handlers.go**

Delete the `muxInBackground` function and `serveProgressiveFallback` if still present.

**Step 2: Remove WaitForMuxing from cache.go**

The `WaitForMuxing` method is no longer needed. Remove it from `cache.go`.

**Step 3: Verify everything compiles**

Run: `cd /root/code/feeds && go build ./... && cd web/frontend && npm run check`
Expected: No errors

**Step 4: Test full flow**

Run: `make dev`
Test:
1. Open watch page - video plays immediately with seeking
2. Click 1080p - download starts, progress shown
3. When complete, click 1080p again - switches to cached version
4. Refresh page - 1080p shows as cached, plays immediately when selected

**Step 5: Commit**

```bash
git add internal/api/handlers.go internal/api/cache.go
git commit -m "refactor: remove dead muxing code"
```

---

## Summary

This plan implements the download-first streaming approach in 10 tasks:

1. Simplify stream handler to progressive proxy
2. Add download manager component
3. Wire download manager into server
4. Add download API endpoints
5. Add frontend API functions
6. Update watch page quality logic
7. Update quality selector UI
8. Add download progress overlay
9. Update stream URL effect
10. Clean up dead code

Each task is atomic and can be committed independently. The order ensures the app remains functional throughout implementation.

