package youtube

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/erik/yt-app/internal/models"
)

// RSS feed structures
type Feed struct {
	XMLName xml.Name `xml:"feed"`
	Entries []Entry  `xml:"entry"`
}

type Entry struct {
	VideoID   string    `xml:"videoId"`
	Title     string    `xml:"title"`
	Published time.Time `xml:"published"`
	Author    Author    `xml:"author"`
}

type Author struct {
	Name string `xml:"name"`
}

var channelIDRegex = regexp.MustCompile(`/channel/([^/]+)`)

// Common shorts indicators in titles
var shortsIndicators = []string{"#shorts", "#short", "#Shorts", "#Short"}

// ExtractChannelID extracts the channel ID from a YouTube channel URL
func ExtractChannelID(channelURL string) string {
	matches := channelIDRegex.FindStringSubmatch(channelURL)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// FetchLatestVideos fetches latest videos from a channel's RSS feed
// If filterShorts is true, it will make additional HTTP requests to check each video
func FetchLatestVideos(channelURL string, limit int) ([]models.Video, error) {
	return FetchLatestVideosFiltered(channelURL, limit, false)
}

// FetchLatestVideosFiltered fetches latest videos with optional shorts filtering
func FetchLatestVideosFiltered(channelURL string, limit int, checkShortsURL bool) ([]models.Video, error) {
	channelID := ExtractChannelID(channelURL)
	if channelID == "" {
		return nil, fmt.Errorf("could not extract channel ID from URL: %s", channelURL)
	}

	rssURL := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", channelID)

	resp, err := http.Get(rssURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RSS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("RSS returned status %d", resp.StatusCode)
	}

	var feed Feed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("failed to parse RSS: %w", err)
	}

	var videos []models.Video
	for _, entry := range feed.Entries {
		if len(videos) >= limit {
			break
		}

		// Handle the yt: namespace prefix in videoId
		videoID := strings.TrimPrefix(entry.VideoID, "yt:")

		// Skip shorts based on title indicators
		if hasShortsHashtag(entry.Title) {
			continue
		}
		// Optionally check URL (slower but more accurate)
		if checkShortsURL && IsShort(videoID) {
			continue
		}

		videos = append(videos, models.Video{
			ID:          videoID,
			Title:       entry.Title,
			ChannelName: entry.Author.Name,
			Thumbnail:   fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", videoID),
			Published:   entry.Published,
			URL:         fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
		})
	}

	return videos, nil
}

// hasShortsHashtag checks if title contains shorts hashtags
func hasShortsHashtag(title string) bool {
	titleLower := strings.ToLower(title)
	for _, indicator := range shortsIndicators {
		if strings.Contains(titleLower, strings.ToLower(indicator)) {
			return true
		}
	}
	return false
}

// IsShort checks if a video ID is a YouTube Short
func IsShort(videoID string) bool {
	shortsURL := fmt.Sprintf("https://www.youtube.com/shorts/%s", videoID)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}
	resp, err := client.Head(shortsURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// If we get 200, it's a short. If redirect (to /watch), it's not a short.
	return resp.StatusCode == 200
}
