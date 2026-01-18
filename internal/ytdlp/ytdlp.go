package ytdlp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/erik/feeds/internal/models"
)

type YTDLP struct {
	BinPath     string
	CookiesPath string
}

func New(binPath string, cookiesPath string) *YTDLP {
	if binPath == "" {
		binPath = "yt-dlp"
	}
	return &YTDLP{BinPath: binPath, CookiesPath: cookiesPath}
}

func (y *YTDLP) appendCookiesArgs(args []string) []string {
	if y.CookiesPath == "" {
		return args
	}
	if _, err := os.Stat(y.CookiesPath); err != nil {
		return args
	}
	return append(args, "--cookies", y.CookiesPath)
}

// Thumbnail represents a single thumbnail option
type Thumbnail struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

// VideoInfo represents yt-dlp JSON output for a video
type VideoInfo struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Channel     string      `json:"channel"`
	ChannelURL  string      `json:"channel_url"`
	Thumbnail   string      `json:"thumbnail"`
	Thumbnails  []Thumbnail `json:"thumbnails"`
	Duration    int         `json:"duration"`
	UploadDate  string      `json:"upload_date"`
	WebpageURL  string      `json:"webpage_url"`
	URL         string      `json:"url"`
	Description string      `json:"description"`
	ViewCount   int64       `json:"view_count"`
}

// GetBestThumbnail returns the best available thumbnail URL
func (v *VideoInfo) GetBestThumbnail() string {
	if v.Thumbnail != "" {
		return v.Thumbnail
	}
	// Find the largest thumbnail
	var best Thumbnail
	for _, t := range v.Thumbnails {
		if t.Width > best.Width {
			best = t
		}
	}
	if best.URL != "" {
		return best.URL
	}
	// Fallback to standard YouTube thumbnail URL
	if v.ID != "" {
		return fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", v.ID)
	}
	return ""
}

// GetLatestVideos fetches the latest videos from a channel (fast mode)
func (y *YTDLP) GetLatestVideos(channelURL string, limit int) ([]VideoInfo, error) {
	args := []string{
		"--flat-playlist",
		"--playlist-end", fmt.Sprintf("%d", limit),
		"--dump-json",
		"--no-warnings",
	}
	args = y.appendCookiesArgs(args)
	args = append(args, channelURL)
	cmd := exec.Command(y.BinPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp error: %v, stderr: %s", err, stderr.String())
	}

	var videos []VideoInfo
	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var v VideoInfo
		if err := decoder.Decode(&v); err != nil {
			continue
		}
		videos = append(videos, v)
	}

	return videos, nil
}

func formatForQuality(quality string, adaptive bool) string {
	if adaptive {
		// For higher resolutions (4K+), allow VP9/AV1 codecs since H.264 maxes at 1080p
		// For 1080p and below, prefer H.264 (avc1) for better compatibility
		switch quality {
		case "4320": // 8K
			return "bestvideo[height<=4320]+bestaudio/best[height<=4320]"
		case "2160": // 4K
			return "bestvideo[height<=2160]+bestaudio/best[height<=2160]"
		case "1440":
			return "bestvideo[height<=1440]+bestaudio/best[height<=1440]"
		case "1080":
			return "bestvideo[height<=1080][vcodec^=avc1]+bestaudio/bestvideo[height<=1080]+bestaudio/best[height<=1080]"
		case "720":
			return "bestvideo[height<=720][vcodec^=avc1]+bestaudio/bestvideo[height<=720]+bestaudio/best[height<=720]"
		case "480":
			return "bestvideo[height<=480][vcodec^=avc1]+bestaudio/bestvideo[height<=480]+bestaudio/best[height<=480]"
		case "360":
			return "bestvideo[height<=360][vcodec^=avc1]+bestaudio/bestvideo[height<=360]+bestaudio/best[height<=360]"
		case "best":
			return "bestvideo+bestaudio/best"
		default:
			return "bestvideo[height<=720][vcodec^=avc1]+bestaudio/bestvideo[height<=720]+bestaudio/best[height<=720]"
		}
	}

	// For progressive (combined) streams, YouTube typically only offers up to 720p
	// Higher quality requires separate video+audio streams (adaptive mode)
	// We use "best" to get the highest available combined stream
	switch quality {
	case "360":
		return "best[ext=mp4][height<=360]/best[height<=360]/best"
	case "480":
		return "best[ext=mp4][height<=480]/best[height<=480]/best"
	default:
		// For 720p+, just get the best available combined stream
		// YouTube combined streams rarely exceed 720p
		return "best[ext=mp4]/best"
	}
}

// GetStreamURL extracts the direct stream URL for a video at a desired quality.
// quality is a height like "1080", "720", etc. Use "best" to let yt-dlp decide.
func (y *YTDLP) GetStreamURL(videoURL string, quality string) (string, error) {
	format := formatForQuality(quality, false)

	args := []string{
		"--force-ipv4",
		"--get-url",
		"--format", format,
	}
	args = y.appendCookiesArgs(args)
	args = append(args, videoURL)
	cmd := exec.Command(y.BinPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp error: %v, stderr: %s", err, stderr.String())
	}

	return string(bytes.TrimSpace(stdout.Bytes())), nil
}

// GetAdaptiveStreamURLs returns separate video+audio URLs when available.
// If yt-dlp only returns a single URL, audioURL will be empty.
func (y *YTDLP) GetAdaptiveStreamURLs(videoURL string, quality string) (string, string, error) {
	format := formatForQuality(quality, true)
	args := []string{
		"--force-ipv4",
		"--get-url",
		"--format", format,
		"--no-playlist",
	}
	args = y.appendCookiesArgs(args)
	args = append(args, videoURL)
	cmd := exec.Command(y.BinPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("yt-dlp error: %v, stderr: %s", err, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return "", "", fmt.Errorf("yt-dlp returned empty stream URLs")
	}
	videoURLOut := strings.TrimSpace(lines[0])
	audioURLOut := ""
	if len(lines) > 1 {
		audioURLOut = strings.TrimSpace(lines[1])
	}
	return videoURLOut, audioURLOut, nil
}

// GetDASHManifest returns the DASH manifest URL for a video
func (y *YTDLP) GetDASHManifest(videoURL string) (string, error) {
	args := []string{
		"--force-ipv4",
		"--dump-json",
		"--no-playlist",
	}
	args = y.appendCookiesArgs(args)
	args = append(args, videoURL)
	cmd := exec.Command(y.BinPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp error: %v, stderr: %s", err, stderr.String())
	}

	// Parse JSON to extract manifest URL
	var result struct {
		ManifestURL     string `json:"manifest_url"`
		RequestedFormats []struct {
			ManifestURL string `json:"manifest_url"`
		} `json:"requested_formats"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return "", fmt.Errorf("failed to parse yt-dlp output: %v", err)
	}

	// Try direct manifest URL first, then check requested_formats
	if result.ManifestURL != "" {
		return result.ManifestURL, nil
	}
	for _, f := range result.RequestedFormats {
		if f.ManifestURL != "" {
			return f.ManifestURL, nil
		}
	}

	return "", fmt.Errorf("no DASH manifest URL found")
}

// Version returns the yt-dlp version string.
func (y *YTDLP) Version() (string, error) {
	cmd := exec.Command(y.BinPath, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp --version error: %v, stderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}


// GetVideoInfo fetches full metadata for a single video
func (y *YTDLP) GetVideoInfo(videoURL string) (*VideoInfo, error) {
	args := []string{
		"--dump-json",
		"--no-playlist",
	}
	args = y.appendCookiesArgs(args)
	args = append(args, videoURL)
	cmd := exec.Command(y.BinPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp error: %v, stderr: %s", err, stderr.String())
	}

	var info VideoInfo
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// Format represents available video formats
type Format struct {
	FormatID   string `json:"format_id"`
	Ext        string `json:"ext"`
	Resolution string `json:"resolution"`
	Height     int    `json:"height"`
	Filesize   int64  `json:"filesize"`
	VCodec     string `json:"vcodec"`
	ACodec     string `json:"acodec"`
}

// GetDownloadURL gets the direct download URL for a specific quality
// Quality options: "best", "720", "480", "360", "audio"
func (y *YTDLP) GetDownloadURL(videoURL string, quality string) (string, string, error) {
	var format string
	var ext string

	switch quality {
	case "audio":
		format = "bestaudio[ext=m4a]/bestaudio"
		ext = "m4a"
	case "360":
		format = "best[height<=360][ext=mp4]/best[height<=360]"
		ext = "mp4"
	case "480":
		format = "best[height<=480][ext=mp4]/best[height<=480]"
		ext = "mp4"
	case "720":
		format = "best[height<=720][ext=mp4]/best[height<=720]"
		ext = "mp4"
	default: // "best"
		format = "best[ext=mp4]/best"
		ext = "mp4"
	}

	args := []string{
		"--get-url",
		"--format", format,
	}
	args = y.appendCookiesArgs(args)
	args = append(args, videoURL)
	cmd := exec.Command(y.BinPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("yt-dlp error: %v, stderr: %s", err, stderr.String())
	}

	return string(bytes.TrimSpace(stdout.Bytes())), ext, nil
}

// GetVideoDurations fetches durations for multiple videos in a single yt-dlp call
// Returns a map of video ID to duration in seconds
func (y *YTDLP) GetVideoDurations(videoIDs []string) (map[string]int, error) {
	if len(videoIDs) == 0 {
		return make(map[string]int), nil
	}

	// Build playlist URL with all video IDs
	// yt-dlp can fetch multiple videos at once using comma-separated IDs
	urls := make([]string, len(videoIDs))
	for i, id := range videoIDs {
		urls[i] = "https://www.youtube.com/watch?v=" + id
	}

	args := []string{
		"--dump-json",
		"--no-warnings",
		"--skip-download",
		"--no-playlist",
	}
	args = y.appendCookiesArgs(args)
	args = append(args, urls...)

	cmd := exec.Command(y.BinPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp error: %v, stderr: %s", err, stderr.String())
	}

	durations := make(map[string]int)
	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var v VideoInfo
		if err := decoder.Decode(&v); err != nil {
			continue
		}
		if v.ID != "" && v.Duration > 0 {
			durations[v.ID] = v.Duration
		}
	}

	return durations, nil
}

// DownloadVideo downloads a video to the specified path using yt-dlp's native downloader.
// This is much faster than using GetAdaptiveStreamURLs + HTTP download because yt-dlp
// handles YouTube's throttling, uses multiple connections, and has proper retry logic.
// Returns the final file size.
func (y *YTDLP) DownloadVideo(videoURL string, quality string, outputPath string) (int64, error) {
	format := formatForQuality(quality, true)

	args := []string{
		"--force-ipv4",
		"--format", format,
		"--merge-output-format", "mp4",
		"--output", outputPath,
		"--no-playlist",
		"--no-warnings",
		"--newline", // Progress on separate lines
	}
	args = y.appendCookiesArgs(args)
	args = append(args, videoURL)

	cmd := exec.Command(y.BinPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("yt-dlp download error: %v, stderr: %s", err, stderr.String())
	}

	// Get the final file size
	info, err := os.Stat(outputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat output file: %v", err)
	}

	return info.Size(), nil
}

// DownloadVideoWithProgress downloads a video and reports progress via callback.
// The callback receives (downloaded bytes, total bytes, percent).
// For adaptive streams, video+audio are downloaded separately and then merged.
// Progress is mapped: video=0-80%, audio=80-95%, merging=95-100%
// outputPath should be the desired final path (e.g., "/tmp/video.mp4")
func (y *YTDLP) DownloadVideoWithProgress(videoURL string, quality string, outputPath string, progressFn func(downloaded, total int64, percent float64)) (int64, error) {
	format := formatForQuality(quality, true)

	// yt-dlp adds extension based on merge format, so we strip .mp4 if present
	outputTemplate := strings.TrimSuffix(outputPath, ".mp4")

	args := []string{
		"--force-ipv4",
		"--format", format,
		"--merge-output-format", "mp4",
		"--output", outputTemplate + ".%(ext)s",
		"--no-playlist",
		"--no-warnings",
		"--newline", // Progress on separate lines
		"--progress-template", "%(progress._percent_str)s %(progress._total_bytes_str)s %(progress._downloaded_bytes_str)s",
	}
	args = y.appendCookiesArgs(args)
	args = append(args, videoURL)

	cmd := exec.Command(y.BinPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start yt-dlp: %v", err)
	}

	// Track which download phase we're in (video=0, audio=1)
	// Video download maps to 0-80%, audio to 80-95%
	downloadPhase := 0
	var lastTotal int64

	// Read progress output line by line using bufio.Scanner for proper buffering
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse progress lines - format is "  0.0%    3.17MiB    1.00KiB" with variable whitespace
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.HasSuffix(fields[0], "%") {
			// First field is percent like "50.5%"
			percentStr := strings.TrimSuffix(fields[0], "%")
			var rawPercent float64
			if _, err := fmt.Sscanf(percentStr, "%f", &rawPercent); err == nil {
				total := parseSize(fields[1])
				downloaded := parseSize(fields[2])

				// Detect phase change: when total changes significantly or percent resets from 100 to 0
				if lastTotal > 0 && total > 0 && total != lastTotal && downloadPhase == 0 {
					downloadPhase = 1
				}
				lastTotal = total

				// Map raw percent to overall progress
				// Video (phase 0): 0-80%, Audio (phase 1): 80-95%
				var mappedPercent float64
				switch downloadPhase {
				case 0: // Video download
					mappedPercent = rawPercent * 0.80
				case 1: // Audio download
					mappedPercent = 80.0 + (rawPercent * 0.15)
				}

				log.Printf("yt-dlp progress (phase %d): raw=%.1f%% mapped=%.1f%% (%d / %d bytes)", downloadPhase, rawPercent, mappedPercent, downloaded, total)
				if progressFn != nil {
					progressFn(downloaded, total, mappedPercent)
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return 0, fmt.Errorf("yt-dlp download error: %v, stderr: %s", err, stderr.String())
	}

	// The actual output file will be outputTemplate + ".mp4"
	actualOutput := outputTemplate + ".mp4"

	// Rename to the requested path if different
	if actualOutput != outputPath {
		if err := os.Rename(actualOutput, outputPath); err != nil {
			return 0, fmt.Errorf("failed to rename output file: %v", err)
		}
	}

	// Get the final file size
	info, err := os.Stat(outputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat output file: %v", err)
	}

	return info.Size(), nil
}

// parseSize parses size strings like "10.5MiB", "1.2GiB", "500KiB"
func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" {
		return 0
	}

	var value float64
	var unit string
	if _, err := fmt.Sscanf(s, "%f%s", &value, &unit); err != nil {
		return 0
	}

	multiplier := int64(1)
	switch strings.ToLower(unit) {
	case "kib", "kb":
		multiplier = 1024
	case "mib", "mb":
		multiplier = 1024 * 1024
	case "gib", "gb":
		multiplier = 1024 * 1024 * 1024
	}

	return int64(value * float64(multiplier))
}

// ToModel converts VideoInfo to our Video model
func (v *VideoInfo) ToModel(channelID int64, channelName string) *models.Video {
	published := time.Now()
	if v.UploadDate != "" {
		if t, err := time.Parse("20060102", v.UploadDate); err == nil {
			published = t
		}
	}

	// Get video URL - flat-playlist uses "url" field, full info uses "webpage_url"
	videoURL := v.WebpageURL
	if videoURL == "" {
		videoURL = v.URL
	}

	return &models.Video{
		ID:          v.ID,
		ChannelID:   channelID,
		Title:       v.Title,
		ChannelName: channelName,
		Thumbnail:   v.GetBestThumbnail(),
		Duration:    v.Duration,
		Published:   published,
		URL:         videoURL,
	}
}
