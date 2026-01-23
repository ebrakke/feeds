package api

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	videoCacheDir        = "/tmp/feeds-video-cache"
	videoCacheTTL        = 1 * time.Hour
	cacheCleanupInterval = 5 * time.Minute
	maxCacheSize         = 5 * 1024 * 1024 * 1024  // 5GB max cache size
	orphanedTmpTTL       = 30 * time.Minute        // Clean .tmp files older than 30 min (stale downloads)
)

const (
	// Buffer thresholds for streaming (bytes needed for ~10 sec of video)
	bufferThreshold4K    = 20 * 1024 * 1024 // ~20 MB for 4K (2160p)
	bufferThreshold1440p = 12 * 1024 * 1024 // ~12 MB for 1440p
	bufferThreshold1080p = 8 * 1024 * 1024  // ~8 MB for 1080p
	bufferThreshold720p  = 4 * 1024 * 1024  // ~4 MB for 720p
	bufferThreshold480p  = 2 * 1024 * 1024  // ~2 MB for 480p
	bufferThreshold360p  = 1 * 1024 * 1024  // ~1 MB for 360p
)

// GetBufferThreshold returns the bytes needed to buffer ~10 seconds of video
func GetBufferThreshold(quality string) int64 {
	switch quality {
	case "2160", "4K", "best":
		return bufferThreshold4K
	case "1440":
		return bufferThreshold1440p
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

	// Run cleanup immediately on startup to clear stale files from previous sessions
	log.Printf("Running initial cache cleanup...")
	vc.cleanup()

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

// cacheFileInfo holds info about a cached file for cleanup decisions
type cacheFileInfo struct {
	path    string
	size    int64
	modTime time.Time
}

func (vc *VideoCache) cleanup() {
	entries, err := os.ReadDir(videoCacheDir)
	if err != nil {
		log.Printf("Cache cleanup: failed to read directory: %v", err)
		return
	}

	now := time.Now()
	var files []cacheFileInfo
	var totalSize int64
	var removedCount int

	// Collect file info and clean orphaned .tmp files
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		path := filepath.Join(videoCacheDir, entry.Name())
		name := entry.Name()

		// Clean orphaned .tmp files (stale downloads from crashes or timeouts)
		if strings.HasSuffix(name, ".tmp") && now.Sub(info.ModTime()) > orphanedTmpTTL {
			if err := os.Remove(path); err == nil {
				log.Printf("Cleaned up orphaned temp file: %s (age: %v)", name, now.Sub(info.ModTime()).Round(time.Minute))
				removedCount++
			}
			continue
		}

		files = append(files, cacheFileInfo{
			path:    path,
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		totalSize += info.Size()
	}

	// First pass: remove files older than TTL
	for i := len(files) - 1; i >= 0; i-- {
		if now.Sub(files[i].modTime) > videoCacheTTL {
			if err := os.Remove(files[i].path); err == nil {
				log.Printf("Cleaned up expired cache file: %s (age: %v)", filepath.Base(files[i].path), now.Sub(files[i].modTime).Round(time.Minute))
				totalSize -= files[i].size
				removedCount++
				// Remove from slice
				files = append(files[:i], files[i+1:]...)
			}
		}
	}

	// Second pass: if still over max size, remove oldest files until under limit
	if totalSize > maxCacheSize {
		log.Printf("Cache size %.2f GB exceeds max %.2f GB, cleaning oldest files", float64(totalSize)/(1024*1024*1024), float64(maxCacheSize)/(1024*1024*1024))

		// Sort by modification time (oldest first)
		for i := 0; i < len(files)-1; i++ {
			for j := i + 1; j < len(files); j++ {
				if files[i].modTime.After(files[j].modTime) {
					files[i], files[j] = files[j], files[i]
				}
			}
		}

		// Remove oldest files until under limit
		for _, f := range files {
			if totalSize <= maxCacheSize {
				break
			}
			if err := os.Remove(f.path); err == nil {
				log.Printf("Cleaned up cache file to reduce size: %s (%.2f MB)", filepath.Base(f.path), float64(f.size)/(1024*1024))
				totalSize -= f.size
				removedCount++
			}
		}
	}

	if removedCount > 0 || totalSize > 0 {
		log.Printf("Cache cleanup complete: removed %d files, current size: %.2f GB", removedCount, float64(totalSize)/(1024*1024*1024))
	}
}
