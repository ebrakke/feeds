package models

import "time"

type Feed struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Author      string    `json:"author,omitempty"`
	Tags        string    `json:"tags,omitempty"` // comma-separated
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Channel struct {
	ID   int64  `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

type Video struct {
	ID          string    `json:"id"`
	ChannelID   int64     `json:"channel_id"`
	Title       string    `json:"title"`
	ChannelName string    `json:"channel_name"`
	Thumbnail   string    `json:"thumbnail"`
	Duration    int       `json:"duration"`
	IsShort     *bool     `json:"is_short"` // nil = unknown, true = short, false = not short
	Published   time.Time `json:"published"`
	URL         string    `json:"url"`
}

// NewPipe import format
type NewPipeExport struct {
	Subscriptions []NewPipeSubscription `json:"subscriptions"`
}

type NewPipeSubscription struct {
	ServiceID int    `json:"service_id"`
	URL       string `json:"url"`
	Name      string `json:"name"`
}

// FeedExport is the shareable feed format
type FeedExport struct {
	Version     int              `json:"version"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Author      string           `json:"author,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Updated     time.Time        `json:"updated,omitempty"`
	Channels    []ExportChannel  `json:"channels"`
}

type ExportChannel struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

// YouTube Watch History (Google Takeout format)
type WatchHistoryEntry struct {
	Header    string              `json:"header"`
	Title     string              `json:"title"`
	TitleURL  string              `json:"titleUrl"`
	Subtitles []WatchHistoryActor `json:"subtitles"`
	Time      time.Time           `json:"time"`
}

type WatchHistoryActor struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// WatchHistoryChannel represents a channel extracted from watch history with view stats
type WatchHistoryChannel struct {
	URL        string `json:"url"`
	Name       string `json:"name"`
	WatchCount int    `json:"watch_count"`
}
