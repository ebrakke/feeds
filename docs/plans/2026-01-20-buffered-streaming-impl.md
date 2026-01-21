# Buffered Streaming Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the current "wait for full download" streaming with a buffered approach that starts playback within 10-15 seconds.

**Architecture:** Start yt-dlp download in background, wait for ~10 seconds of video data to buffer, then begin serving the partial file while download continues. Remove the unreliable direct proxy mode.

**Tech Stack:** Go, yt-dlp, HTTP range requests

---

## Task 1: Add Buffer Threshold Constants and Helper Functions

**Files:**
- Modify: `internal/api/cache.go`

**Step 1: Add buffer threshold constants**

Add after existing constants (line ~14):

```go
const (
	// Buffer thresholds for streaming (bytes needed for ~10 sec of video)
	bufferThreshold1080p = 8 * 1024 * 1024  // ~8 MB for 1080p
	bufferThreshold720p  = 4 * 1024 * 1024  // ~4 MB for 720p
	bufferThreshold480p  = 2 * 1024 * 1024  // ~2 MB for 480p
	bufferThreshold360p  = 1 * 1024 * 1024  // ~1 MB for 360p
	bufferThreshold4K    = 20 * 1024 * 1024 // ~20 MB for 4K
)

// GetBufferThreshold returns the bytes needed to buffer ~10 seconds of video
func GetBufferThreshold(quality string) int64 {
	switch quality {
	case "2160", "4K", "best":
		return bufferThreshold4K
	case "1080":
		return bufferThreshold1080p
	case "720":
		return bufferThreshold720p
	case "480":
		return bufferThreshold480p
	case "360":
		return bufferThreshold360p
	default:
		return bufferThreshold720p // Default to 720p threshold
	}
}
```

**Step 2: Add helper to get file size safely**

Add to `cache.go`:

```go
// GetFileSize returns the current size of a file, or 0 if it doesn't exist
func GetFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
```

**Step 3: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/api/cache.go
git commit -m "feat(streaming): add buffer threshold constants and helpers"
```

---

## Task 2: Extend Download Struct for Buffered Streaming

**Files:**
- Modify: `internal/api/download.go`

**Step 1: Add fields to Download struct for buffer tracking**

Modify the `Download` struct (around line 23-30) to add:

```go
// Download represents an in-progress download
type Download struct {
	VideoID     string
	Quality     string
	Status      string // "downloading", "muxing", "complete", "error"
	Progress    float64
	Error       string
	StartedAt   time.Time
	FilePath    string        // Path to the file being downloaded
	FileSize    int64         // Current file size (updated during download)
	TotalSize   int64         // Expected total size (if known)
	bufferReady chan struct{} // Closed when buffer threshold is reached
	mu          sync.Mutex    // Protects FileSize updates
}
```

**Step 2: Add WaitForBuffer method**

Add after the Download struct:

```go
// WaitForBuffer blocks until the download has buffered enough data or context is cancelled.
// Returns nil when buffer is ready, or an error if cancelled/failed.
func (d *Download) WaitForBuffer(ctx context.Context, threshold int64) error {
	// Check if already have enough data
	d.mu.Lock()
	if d.FileSize >= threshold {
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()

	// Wait for buffer to be ready
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.bufferReady:
		return nil
	}
}

// UpdateFileSize updates the current file size and signals buffer ready if threshold met
func (d *Download) UpdateFileSize(size int64, threshold int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.FileSize = size

	// Signal buffer ready if we've hit the threshold and haven't already
	if size >= threshold && d.bufferReady != nil {
		select {
		case <-d.bufferReady:
			// Already closed
		default:
			close(d.bufferReady)
		}
	}
}

// GetFilePath returns the path to the download file
func (d *Download) GetFilePath() string {
	return d.FilePath
}

// GetFileSize returns the current downloaded size
func (d *Download) GetFileSize() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.FileSize
}
```

**Step 3: Add context import**

Add `"context"` to the imports at the top of `download.go`.

**Step 4: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/api/download.go
git commit -m "feat(streaming): add buffer tracking to Download struct"
```

---

## Task 3: Update StartDownload to Initialize Buffer Tracking

**Files:**
- Modify: `internal/api/download.go`

**Step 1: Update StartDownload to initialize new fields**

Modify the `StartDownload` function. Find where the Download struct is created (around line 78):

```go
	// Create new download
	d := &Download{
		VideoID:     videoID,
		Quality:     quality,
		Status:      "downloading",
		StartedAt:   time.Now(),
		FilePath:    dm.cache.CachePath(cacheKey),
		bufferReady: make(chan struct{}),
	}
```

**Step 2: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/api/download.go
git commit -m "feat(streaming): initialize buffer tracking in StartDownload"
```

---

## Task 4: Update runDownload to Track File Size During Download

**Files:**
- Modify: `internal/api/download.go`

**Step 1: Update runDownload to track file size**

Modify the `runDownload` function. Find the progress callback in `DownloadVideoWithProgress` call (around line 175) and update it:

```go
func (dm *DownloadManager) runDownload(videoID, quality, key, cacheKey string) {
	dm.mu.RLock()
	d := dm.active[key]
	dm.mu.RUnlock()

	defer func() {
		dm.mu.Lock()
		delete(dm.active, key)
		dm.mu.Unlock()
	}()

	videoURL := "https://www.youtube.com/watch?v=" + videoID
	outputPath := dm.cache.CachePath(cacheKey)
	tempOutput := outputPath + ".tmp"

	// Store the temp path for serving partial file
	if d != nil {
		d.FilePath = tempOutput
	}

	log.Printf("Starting yt-dlp download for %s quality %s", videoID, quality)

	// Get buffer threshold for this quality
	threshold := GetBufferThreshold(quality)

	// Use yt-dlp's native downloader with progress callback
	var lastBroadcast time.Time
	size, err := dm.ytdlp.DownloadVideoWithProgress(videoURL, quality, tempOutput, func(downloaded, total int64, percent float64) {
		// Update file size for buffer tracking
		if d != nil {
			currentSize := GetFileSize(tempOutput)
			d.UpdateFileSize(currentSize, threshold)
			d.TotalSize = total
		}

		// Throttle broadcasts to every 250ms for smoother progress
		if time.Since(lastBroadcast) < 250*time.Millisecond {
			return
		}
		lastBroadcast = time.Now()

		dm.broadcast(videoID, DownloadProgress{
			Quality:         quality,
			Percent:         percent,
			BytesDownloaded: downloaded,
			TotalBytes:      total,
			Status:          "downloading",
		})
	})

	if err != nil {
		// Clean up temp file on error
		os.Remove(tempOutput)
		dm.setError(key, videoID, quality, fmt.Sprintf("Download failed: %v", err))
		return
	}

	// Move to final location
	if err := os.Rename(tempOutput, outputPath); err != nil {
		os.Remove(tempOutput)
		dm.setError(key, videoID, quality, fmt.Sprintf("Failed to save file: %v", err))
		return
	}

	// Update file path to final location
	if d != nil {
		d.FilePath = outputPath
		d.FileSize = size
	}

	log.Printf("Download complete: %s quality %s, %d bytes saved to %s", videoID, quality, size, outputPath)

	dm.broadcast(videoID, DownloadProgress{
		Quality: quality,
		Percent: 100,
		Status:  "complete",
	})
}
```

**Step 2: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/api/download.go
git commit -m "feat(streaming): track file size during download for buffered streaming"
```

---

## Task 5: Add GetOrStartDownload Method

**Files:**
- Modify: `internal/api/download.go`

**Step 1: Add GetOrStartDownload method**

Add this method to DownloadManager:

```go
// GetOrStartDownload returns an existing download or starts a new one.
// Unlike StartDownload, this always returns the Download pointer for tracking.
func (dm *DownloadManager) GetOrStartDownload(videoID, quality string) *Download {
	key := downloadKey(videoID, quality)
	cacheKey := CacheKey(videoID, quality)

	// Check if already cached (complete)
	if cachedPath, ok := dm.cache.Get(cacheKey); ok {
		return &Download{
			VideoID:  videoID,
			Quality:  quality,
			Status:   "complete",
			FilePath: cachedPath,
			FileSize: GetFileSize(cachedPath),
		}
	}

	dm.mu.Lock()
	// Check if already downloading
	if d, exists := dm.active[key]; exists {
		dm.mu.Unlock()
		return d
	}

	// Create new download
	d := &Download{
		VideoID:     videoID,
		Quality:     quality,
		Status:      "downloading",
		StartedAt:   time.Now(),
		FilePath:    dm.cache.CachePath(cacheKey) + ".tmp",
		bufferReady: make(chan struct{}),
	}
	dm.active[key] = d
	dm.mu.Unlock()

	// Start download in background
	go dm.runDownload(videoID, quality, key, cacheKey)

	return d
}
```

**Step 2: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/api/download.go
git commit -m "feat(streaming): add GetOrStartDownload for buffered streaming"
```

---

## Task 6: Add selectBestQuality Helper

**Files:**
- Modify: `internal/api/handlers.go`

**Step 1: Add selectBestQuality function**

Add this function before `handleStreamProxy`:

```go
// selectBestQuality returns the best quality to use for "auto" mode.
// Defaults to 1080p as a good balance of quality and download speed.
func selectBestQuality() string {
	return "1080"
}
```

**Step 2: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/api/handlers.go
git commit -m "feat(streaming): add selectBestQuality helper"
```

---

## Task 7: Implement Buffered Stream Handler

**Files:**
- Modify: `internal/api/handlers.go`

**Step 1: Replace handleStreamProxy with buffered implementation**

Replace the `handleStreamProxy` function (around line 1185-1207):

```go
func (s *Server) handleStreamProxy(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	quality := r.URL.Query().Get("quality")
	if quality == "" || quality == "auto" {
		quality = selectBestQuality()
	}

	// Check if already fully cached
	cacheKey := CacheKey(videoID, quality)
	if cachedPath, ok := s.videoCache.Get(cacheKey); ok {
		log.Printf("Serving fully cached video: %s quality %s", videoID, quality)
		http.ServeFile(w, r, cachedPath)
		return
	}

	// Start or get existing download
	download := s.downloadManager.GetOrStartDownload(videoID, quality)

	// If already complete, serve the file
	if download.Status == "complete" {
		log.Printf("Serving completed download: %s quality %s", videoID, quality)
		http.ServeFile(w, r, download.GetFilePath())
		return
	}

	// Wait for buffer threshold
	threshold := GetBufferThreshold(quality)
	log.Printf("Waiting for buffer (%d bytes) for %s quality %s", threshold, videoID, quality)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if err := download.WaitForBuffer(ctx, threshold); err != nil {
		log.Printf("Buffer wait failed for %s: %v", videoID, err)
		http.Error(w, "Buffering failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	log.Printf("Buffer ready for %s quality %s, serving partial file", videoID, quality)

	// Serve the partial file
	s.servePartialFile(w, r, download)
}
```

**Step 2: Add context import if not present**

Ensure `"context"` is in the imports.

**Step 3: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: Error about missing `servePartialFile` - this is expected, we'll add it next.

**Step 4: Commit (partial - will complete in next task)**

Don't commit yet - we need to add `servePartialFile` first.

---

## Task 8: Implement servePartialFile

**Files:**
- Modify: `internal/api/handlers.go`

**Step 1: Add servePartialFile function**

Add after `handleStreamProxy`:

```go
// servePartialFile serves a file that may still be downloading.
// It handles range requests and serves available data.
func (s *Server) servePartialFile(w http.ResponseWriter, r *http.Request, d *Download) {
	filePath := d.GetFilePath()

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get current file size
	currentSize := d.GetFileSize()
	if currentSize == 0 {
		info, err := file.Stat()
		if err != nil {
			http.Error(w, "Failed to stat file", http.StatusInternalServerError)
			return
		}
		currentSize = info.Size()
	}

	// Set content type
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")

	// Handle range request
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		s.servePartialFileRange(w, file, currentSize, rangeHeader)
		return
	}

	// No range request - serve from beginning
	// Use current size as content length (client will handle incomplete)
	w.Header().Set("Content-Length", strconv.FormatInt(currentSize, 10))
	w.WriteHeader(http.StatusOK)

	// Copy available data
	io.CopyN(w, file, currentSize)
}

// servePartialFileRange handles range requests for partial files
func (s *Server) servePartialFileRange(w http.ResponseWriter, file *os.File, fileSize int64, rangeHeader string) {
	// Parse range header: "bytes=start-end" or "bytes=start-"
	var start, end int64
	_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
	if err != nil {
		// Try without end
		_, err = fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
		if err != nil {
			http.Error(w, "Invalid range", http.StatusBadRequest)
			return
		}
		end = fileSize - 1
	}

	// Validate range
	if start < 0 || start >= fileSize {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		http.Error(w, "Range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// Clamp end to available data
	if end >= fileSize {
		end = fileSize - 1
	}

	// Seek to start position
	if _, err := file.Seek(start, 0); err != nil {
		http.Error(w, "Seek failed", http.StatusInternalServerError)
		return
	}

	length := end - start + 1

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(http.StatusPartialContent)

	io.CopyN(w, file, length)
}
```

**Step 2: Add io import if not present**

Ensure `"io"` is in the imports.

**Step 3: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/api/handlers.go internal/api/download.go
git commit -m "feat(streaming): implement buffered streaming with partial file serving"
```

---

## Task 9: Remove Unused proxyProgressiveStream

**Files:**
- Modify: `internal/api/handlers.go`

**Step 1: Remove proxyProgressiveStream function**

Delete the `proxyProgressiveStream` function (around line 1209-1251) as it's no longer used.

**Step 2: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/api/handlers.go
git commit -m "refactor(streaming): remove unused proxyProgressiveStream"
```

---

## Task 10: Test Manually

**Step 1: Start the server**

Run: `cd /root/code/feeds && make dev`

**Step 2: Test via curl**

In another terminal:

```bash
# Test 1080p streaming - should start within 15 seconds
time curl -v "http://localhost:8080/api/stream/dQw4w9WgXcQ?quality=1080" -o /dev/null --max-time 30

# Test auto quality (should use 1080p)
time curl -v "http://localhost:8080/api/stream/dQw4w9WgXcQ" -o /dev/null --max-time 30
```

Expected: Response headers arrive within 15 seconds, streaming begins.

**Step 3: Test via mobile browser**

1. Open the app on mobile
2. Navigate to a video
3. Select 1080p quality
4. Verify playback starts within 15 seconds

**Step 4: Commit integration test notes**

No code changes, just verification.

---

## Task 11: Handle Edge Case - Download Error During Buffer Wait

**Files:**
- Modify: `internal/api/download.go`

**Step 1: Update Download struct to track errors during buffer wait**

Add error signaling to the Download struct. Modify `WaitForBuffer`:

```go
// WaitForBuffer blocks until the download has buffered enough data or context is cancelled.
// Returns nil when buffer is ready, or an error if cancelled/failed.
func (d *Download) WaitForBuffer(ctx context.Context, threshold int64) error {
	// Check if already have enough data
	d.mu.Lock()
	if d.FileSize >= threshold {
		d.mu.Unlock()
		return nil
	}
	if d.Status == "error" {
		err := d.Error
		d.mu.Unlock()
		return fmt.Errorf("download failed: %s", err)
	}
	if d.Status == "complete" {
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()

	// Poll for buffer ready with timeout
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.bufferReady:
			return nil
		case <-ticker.C:
			d.mu.Lock()
			if d.Status == "error" {
				err := d.Error
				d.mu.Unlock()
				return fmt.Errorf("download failed: %s", err)
			}
			if d.Status == "complete" || d.FileSize >= threshold {
				d.mu.Unlock()
				return nil
			}
			d.mu.Unlock()
		}
	}
}
```

**Step 2: Add fmt import if not present**

Ensure `"fmt"` is in the imports for download.go.

**Step 3: Verify the code compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/api/download.go
git commit -m "fix(streaming): handle download errors during buffer wait"
```

---

## Task 12: Final Integration Commit

**Step 1: Run full test**

```bash
cd /root/code/feeds && go build ./... && go test ./...
```

**Step 2: Create final commit with all changes**

```bash
git add -A
git commit -m "feat(streaming): buffered streaming for fast 1080p+ playback

- Replace direct proxy with buffered download approach
- Start playback after ~10 seconds of buffering instead of full download
- Support range requests for seeking within downloaded portion
- Auto quality now defaults to 1080p with buffered streaming
- Remove unreliable progressive stream proxy mode

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Summary

This implementation:

1. **Adds buffer threshold constants** - ~10 seconds of video data per quality level
2. **Extends Download struct** - Tracks file size and buffer readiness
3. **Implements buffered streaming** - Waits for buffer, then serves partial file
4. **Handles range requests** - Seeking works within downloaded portion
5. **Removes unreliable proxy** - No more flaky 360p "auto" mode
6. **Defaults auto to 1080p** - Best quality with buffered approach

The key insight is that yt-dlp writes to the temp file progressively, so we can start serving it as soon as enough data is buffered, while the download continues in the background.
