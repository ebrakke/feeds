package youtube

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/erik/feeds/internal/models"
)

// RSS feed structures
type Feed struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	Author  Author   `xml:"author"`
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
var handleRegex = regexp.MustCompile(`/@([^/]+)`)
var videoIDRegex = regexp.MustCompile(`(?:v=|youtu\.be/|shorts/)([a-zA-Z0-9_-]{11})`)

// ChannelInfo contains basic channel metadata
type ChannelInfo struct {
	ID   string
	Name string
	URL  string
}

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

// ExtractVideoID extracts the video ID from various YouTube URL formats
// Supports: youtube.com/watch?v=ID, youtu.be/ID, youtube.com/shorts/ID
func ExtractVideoID(url string) string {
	matches := videoIDRegex.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ResolveChannelURL takes any YouTube channel URL format and returns channel info
// Supports: /channel/ID, /@handle, /c/customname, /user/username
func ResolveChannelURL(inputURL string) (*ChannelInfo, error) {
	// Normalize the URL
	inputURL = strings.TrimSpace(inputURL)
	if !strings.HasPrefix(inputURL, "http") {
		inputURL = "https://www.youtube.com/" + strings.TrimPrefix(inputURL, "/")
	}

	// If it's already a /channel/ URL, try RSS directly
	if channelID := ExtractChannelID(inputURL); channelID != "" {
		return fetchChannelInfoByID(channelID)
	}

	// For handles and other formats, we need to resolve the actual channel ID
	// by fetching the page and looking for the channel ID
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(inputURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("channel page returned status %d", resp.StatusCode)
	}

	// The final URL after redirects should contain the channel ID or we can extract from page
	finalURL := resp.Request.URL.String()
	if channelID := ExtractChannelID(finalURL); channelID != "" {
		return fetchChannelInfoByID(channelID)
	}

	// Read body and look for channel ID in the HTML
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	body := string(bodyBytes)

	// Look for channel ID in various places in the HTML
	patterns := []string{
		`"channelId":"([^"]+)"`,
		`/channel/([^"/?]+)`,
		`"externalId":"([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(body); len(matches) > 1 {
			return fetchChannelInfoByID(matches[1])
		}
	}

	return nil, fmt.Errorf("could not find channel ID for URL: %s", inputURL)
}

// fetchChannelInfoByID fetches channel info from RSS feed
func fetchChannelInfoByID(channelID string) (*ChannelInfo, error) {
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

	channelName := feed.Author.Name
	if channelName == "" && len(feed.Entries) > 0 {
		channelName = feed.Entries[0].Author.Name
	}
	if channelName == "" {
		// Try to get name from title (format: "YouTube channel name")
		channelName = strings.TrimSuffix(feed.Title, " - YouTube")
	}

	return &ChannelInfo{
		ID:   channelID,
		Name: channelName,
		URL:  fmt.Sprintf("https://www.youtube.com/channel/%s", channelID),
	}, nil
}

// FetchLatestVideos fetches latest videos from a channel's RSS feed
// If filterShorts is true, it will make additional HTTP requests to check each video
func FetchLatestVideos(channelURL string, limit int) ([]models.Video, error) {
	return FetchLatestVideosFiltered(channelURL, limit, false)
}

// FetchLatestVideosFiltered fetches latest videos with optional shorts filtering
func FetchLatestVideosFiltered(channelURL string, limit int, checkShortsURL bool) ([]models.Video, error) {
	// First try direct channel ID extraction
	channelID := ExtractChannelID(channelURL)

	// If that fails, resolve the URL (handles @handles, /c/, /user/, etc.)
	if channelID == "" {
		info, err := ResolveChannelURL(channelURL)
		if err != nil {
			return nil, fmt.Errorf("could not resolve channel URL %s: %w", channelURL, err)
		}
		channelID = info.ID
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

// CheckShortsStatus checks multiple video IDs and returns a map of videoID -> isShort
// Uses concurrent requests with a limit to avoid overwhelming the server
func CheckShortsStatus(videoIDs []string) map[string]bool {
	results := make(map[string]bool)
	if len(videoIDs) == 0 {
		return results
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent requests to 5 to avoid overwhelming YouTube's servers
	sem := make(chan struct{}, 5)

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, videoID := range videoIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			shortsURL := fmt.Sprintf("https://www.youtube.com/shorts/%s", id)
			resp, err := client.Head(shortsURL)
			if err != nil {
				return // Skip on error, will retry later
			}
			defer resp.Body.Close()

			mu.Lock()
			results[id] = (resp.StatusCode == 200)
			mu.Unlock()
		}(videoID)
	}

	wg.Wait()
	return results
}
