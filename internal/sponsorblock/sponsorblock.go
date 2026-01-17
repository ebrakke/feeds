package sponsorblock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	BaseURL = "https://sponsor.ajay.app"
)

// Segment categories
const (
	CategorySponsor     = "sponsor"
	CategoryIntro       = "intro"
	CategoryOutro       = "outro"
	CategoryInteraction = "interaction"
	CategorySelfpromo   = "selfpromo"
	CategoryMusicOfftopic = "music_offtopic"
	CategoryPreview     = "preview"
	CategoryFiller      = "filler"
)

// DefaultCategories are the categories we fetch by default
var DefaultCategories = []string{
	CategorySponsor,
	CategoryIntro,
	CategoryOutro,
	CategoryInteraction,
	CategorySelfpromo,
	CategoryPreview,
}

// Segment represents a SponsorBlock segment
type Segment struct {
	UUID       string    `json:"UUID"`
	Segment    [2]float64 `json:"segment"`
	Category   string    `json:"category"`
	ActionType string    `json:"actionType"`
	Votes      int       `json:"votes"`
	Locked     int       `json:"locked"`
}

// Client is a SponsorBlock API client
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new SponsorBlock client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: BaseURL,
	}
}

// GetSegments fetches segments for a video from SponsorBlock API
func (c *Client) GetSegments(videoID string, categories []string) ([]Segment, error) {
	if len(categories) == 0 {
		categories = DefaultCategories
	}

	// Build URL with categories
	params := url.Values{}
	params.Set("videoID", videoID)

	// Add categories as JSON array
	categoriesJSON, _ := json.Marshal(categories)
	params.Set("categories", string(categoriesJSON))

	reqURL := fmt.Sprintf("%s/api/skipSegments?%s", c.baseURL, params.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Feeds/1.0 (https://github.com/erik/feeds)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 404 means no segments found - that's fine
	if resp.StatusCode == http.StatusNotFound {
		return []Segment{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SponsorBlock API returned status %d", resp.StatusCode)
	}

	var segments []Segment
	if err := json.NewDecoder(resp.Body).Decode(&segments); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return segments, nil
}

// CategoryInfo returns human-readable info about a category
func CategoryInfo(category string) (name string, color string) {
	switch category {
	case CategorySponsor:
		return "Sponsor", "#00d400"
	case CategoryIntro:
		return "Intro", "#00ffff"
	case CategoryOutro:
		return "Outro", "#0202ed"
	case CategoryInteraction:
		return "Interaction", "#cc00ff"
	case CategorySelfpromo:
		return "Self-Promotion", "#ffff00"
	case CategoryMusicOfftopic:
		return "Music (Off-topic)", "#ff9900"
	case CategoryPreview:
		return "Preview", "#008fd6"
	case CategoryFiller:
		return "Filler", "#7300ff"
	default:
		return strings.Title(category), "#888888"
	}
}
