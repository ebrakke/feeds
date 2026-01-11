package models

import "time"

type Feed struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Channel struct {
	ID     int64  `json:"id"`
	FeedID int64  `json:"feed_id"`
	URL    string `json:"url"`
	Name   string `json:"name"`
}

type Video struct {
	ID          string    `json:"id"`
	ChannelID   int64     `json:"channel_id"`
	Title       string    `json:"title"`
	ChannelName string    `json:"channel_name"`
	Thumbnail   string    `json:"thumbnail"`
	Duration    int       `json:"duration"`
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
