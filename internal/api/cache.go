package api

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	videoCacheDir        = "/tmp/feeds-video-cache"
	videoCacheTTL        = 1 * time.Hour
	cacheCleanupInterval = 10 * time.Minute
)

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

// GetFileSize returns the current size of a file, or 0 if it doesn't exist
func GetFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// VideoCache manages cached muxed video files
type VideoCache struct{}

// NewVideoCache creates a new video cache manager
func NewVideoCache() *VideoCache {
	// Ensure cache directory exists
	if err := os.MkdirAll(videoCacheDir, 0755); err != nil {
		log.Printf("Warning: could not create video cache dir: %v", err)
	}

	vc := &VideoCache{}

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
