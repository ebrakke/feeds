package api

import (
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
	// Use a done channel to signal completion to the progress monitor
	done := make(chan struct{})
	defer close(done)

	defer func() {
		dm.mu.Lock()
		delete(dm.active, key)
		dm.mu.Unlock()
	}()

	videoURL := "https://www.youtube.com/watch?v=" + videoID

	log.Printf("Starting download for %s quality %s", videoID, quality)

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

	log.Printf("Got stream URLs for %s, starting downloads", videoID)

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
		log.Printf("Video download finished for %s: %d bytes, err=%v", videoID, videoSize, videoErr)
	}()

	go func() {
		defer wg.Done()
		audioSize, audioErr = dm.downloadFile(audioStreamURL, audioPath)
		log.Printf("Audio download finished for %s: %d bytes, err=%v", videoID, audioSize, audioErr)
	}()

	// Monitor progress while downloading
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
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

				// Update with rough progress - estimate ~15MB total for a typical video
				estimatedTotal := int64(15 * 1024 * 1024)
				percent := float64(downloaded) / float64(estimatedTotal) * 100
				if percent > 95 {
					percent = 95 // Cap at 95% until muxing is done
				}

				progress := DownloadProgress{
					Quality:         quality,
					Percent:         percent,
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

	log.Printf("Downloads complete for %s, starting mux (%d + %d bytes)", videoID, videoSize, audioSize)

	dm.broadcast(videoID, DownloadProgress{
		Quality:         quality,
		Percent:         95,
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

	log.Printf("Download complete: %s quality %s, saved to %s", videoID, quality, outputPath)

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
