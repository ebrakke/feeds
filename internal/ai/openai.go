package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/erik/feeds/internal/models"
)

// truncateList joins up to n items from a slice
func truncateList(items []string, n int) string {
	if len(items) > n {
		items = items[:n]
	}
	return strings.Join(items, ", ")
}

type Client struct {
	APIKey string
}

func New(apiKey string) *Client {
	return &Client{APIKey: apiKey}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type GroupSuggestion struct {
	Name     string                       `json:"name"`
	Channels []models.NewPipeSubscription `json:"channels"`
}

// ChannelInfo holds channel name and recent video titles for AI context
type ChannelInfo struct {
	Name        string
	URL         string
	VideoTitles []string
}

func (c *Client) SuggestGroups(subs []models.NewPipeSubscription) ([]GroupSuggestion, error) {
	return c.SuggestGroupsWithMetadata(subs, nil)
}

func (c *Client) SuggestGroupsWithMetadata(subs []models.NewPipeSubscription, metadata map[string]ChannelInfo) ([]GroupSuggestion, error) {
	// Build channel list for prompt with metadata if available
	var channelEntries []string
	for _, sub := range subs {
		entry := sub.Name
		if metadata != nil {
			if info, ok := metadata[sub.URL]; ok && len(info.VideoTitles) > 0 {
				entry = fmt.Sprintf("%s (recent videos: %s)", sub.Name, truncateList(info.VideoTitles, 3))
			}
		}
		channelEntries = append(channelEntries, entry)
	}

	prompt := fmt.Sprintf(`You are organizing YouTube channel subscriptions into logical groups.

Given this list of YouTube channels (with recent video titles for context), group them into 5-12 categories based on their content type/topic.

Channels:
%s

Respond with ONLY valid JSON in this exact format (no markdown, no explanation):
{
  "groups": [
    {
      "name": "Group Name",
      "channels": ["Channel Name 1", "Channel Name 2"]
    }
  ]
}

CRITICAL RULES:
- Copy channel names EXACTLY as given (before the "(recent videos:" part)
- Every single channel MUST appear in exactly one group
- DO NOT use "Other" or "Miscellaneous" - find a specific category for each channel
- Group names should be short (1-3 words)
- If unsure, use the video titles to determine the category
- Good categories: Tech, Gaming, Music, Education, News, Entertainment, Sports, Cooking, Science, Art, Finance, Comedy, Lifestyle, DIY, Automotive, Travel, Health, Politics, History, Animation, Reviews, Podcasts, etc.
- It's okay to have small groups with just 1-2 channels if they don't fit elsewhere`, channelEntries)

	req := chatRequest{
		Model: "gpt-5.2-2025-12-11",
		Messages: []message{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	log.Printf("Calling OpenAI API (model: gpt-5.2, channels: %d)...", len(subs))
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("OpenAI API responded with status %d", resp.StatusCode)

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	// Parse the AI response
	var aiResult struct {
		Groups []struct {
			Name     string   `json:"name"`
			Channels []string `json:"channels"`
		} `json:"groups"`
	}

	if err := json.Unmarshal([]byte(chatResp.Choices[0].Message.Content), &aiResult); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	// Map channel names back to full subscription data
	// Use both exact name and lowercase for matching
	channelMap := make(map[string]models.NewPipeSubscription)
	channelMapLower := make(map[string]string) // lowercase -> original name
	for _, sub := range subs {
		channelMap[sub.Name] = sub
		channelMapLower[strings.ToLower(sub.Name)] = sub.Name
	}

	var suggestions []GroupSuggestion
	for _, g := range aiResult.Groups {
		// Skip "Other" or "Miscellaneous" groups from AI
		nameLower := strings.ToLower(g.Name)
		if nameLower == "other" || nameLower == "miscellaneous" || nameLower == "misc" {
			continue
		}

		suggestion := GroupSuggestion{Name: g.Name}
		for _, chName := range g.Channels {
			// Try exact match first
			if sub, ok := channelMap[chName]; ok {
				suggestion.Channels = append(suggestion.Channels, sub)
				delete(channelMap, chName)
				delete(channelMapLower, strings.ToLower(chName))
				continue
			}
			// Try case-insensitive match
			if originalName, ok := channelMapLower[strings.ToLower(chName)]; ok {
				if sub, ok := channelMap[originalName]; ok {
					suggestion.Channels = append(suggestion.Channels, sub)
					delete(channelMap, originalName)
					delete(channelMapLower, strings.ToLower(chName))
				}
			}
		}
		if len(suggestion.Channels) > 0 {
			suggestions = append(suggestions, suggestion)
		}
	}

	// For unmatched channels, try to assign them to the most appropriate existing group
	// or create a catch-all "Uncategorized" group
	if len(channelMap) > 0 {
		uncategorized := GroupSuggestion{Name: "Uncategorized"}
		for _, sub := range channelMap {
			uncategorized.Channels = append(uncategorized.Channels, sub)
		}
		suggestions = append(suggestions, uncategorized)
	}

	return suggestions, nil
}
