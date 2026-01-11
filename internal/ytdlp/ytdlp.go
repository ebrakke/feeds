package ytdlp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/erik/yt-app/internal/models"
)

type YTDLP struct {
	BinPath string
}

func New(binPath string) *YTDLP {
	if binPath == "" {
		binPath = "yt-dlp"
	}
	return &YTDLP{BinPath: binPath}
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
	cmd := exec.Command(y.BinPath,
		"--flat-playlist",
		"--playlist-end", fmt.Sprintf("%d", limit),
		"--dump-json",
		"--no-warnings",
		channelURL,
	)

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

// GetStreamURL extracts the direct stream URL for a video
func (y *YTDLP) GetStreamURL(videoURL string) (string, error) {
	cmd := exec.Command(y.BinPath,
		"--get-url",
		"--format", "best[ext=mp4]/best",
		videoURL,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp error: %v, stderr: %s", err, stderr.String())
	}

	return string(bytes.TrimSpace(stdout.Bytes())), nil
}

// GetVideoInfo fetches full metadata for a single video
func (y *YTDLP) GetVideoInfo(videoURL string) (*VideoInfo, error) {
	cmd := exec.Command(y.BinPath,
		"--dump-json",
		"--no-playlist",
		videoURL,
	)

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

	cmd := exec.Command(y.BinPath,
		"--get-url",
		"--format", format,
		videoURL,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("yt-dlp error: %v, stderr: %s", err, stderr.String())
	}

	return string(bytes.TrimSpace(stdout.Bytes())), ext, nil
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
