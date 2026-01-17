package api

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	videoCacheDir        = "/tmp/feeds-video-cache"
	videoCacheTTL        = 1 * time.Hour
	cacheCleanupInterval = 10 * time.Minute
)

// VideoCache manages cached muxed video files
type VideoCache struct {
	mu     sync.RWMutex
	muxing map[string]chan struct{} // tracks in-progress muxing operations
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
