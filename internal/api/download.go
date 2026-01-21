package api

import (
	"context"
	"fmt"
	"log"
	"os"
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
	for _, d := range dm.active {
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
		// For terminal states (complete/error), use blocking send with timeout
		// to ensure the message is delivered
		if progress.Status == "complete" || progress.Status == "error" {
			select {
			case ch <- progress:
			case <-time.After(5 * time.Second):
				log.Printf("Timeout sending %s status to listener for %s", progress.Status, videoID)
			}
		} else {
			// For progress updates, non-blocking is fine
			select {
			case ch <- progress:
			default:
				// Skip if channel buffer is full
			}
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
	outputPath := dm.cache.CachePath(cacheKey)
	tempOutput := outputPath + ".tmp"

	log.Printf("Starting yt-dlp download for %s quality %s", videoID, quality)

	// Use yt-dlp's native downloader with progress callback
	// This is MUCH faster than HTTP download (~20 MB/s vs ~100 KB/s)
	// Progress is mapped in ytdlp: video=0-80%, audio=80-95%, complete=100%
	var lastBroadcast time.Time
	size, err := dm.ytdlp.DownloadVideoWithProgress(videoURL, quality, tempOutput, func(downloaded, total int64, percent float64) {
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

	log.Printf("Download complete: %s quality %s, %d bytes saved to %s", videoID, quality, size, outputPath)

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
