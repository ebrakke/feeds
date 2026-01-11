# Feeds

**Non-algorithmic video subscriptions, self-hosted.**

Feeds is a self-hosted YouTube subscription manager that puts you back in control. No algorithms deciding what you see—just chronological updates from creators you choose, organized into feeds you curate.

## Why Feeds?

YouTube's algorithm optimizes for engagement, not your interests. Feeds takes a different approach:

- **Chronological, always** — See what your subscriptions posted, in order, nothing more
- **Curated feeds** — Group channels by mood, topic, or whatever makes sense to you
- **Shareable lists** — Export a feed as JSON and share it; others can import without polluting their main subscriptions
- **Self-hosted persistence** — Your watch history, progress, and feeds live on your server, synced across all your devices
- **yt-dlp powered** — Stream or download videos however you want

## How it compares

| | Feeds | YouTube | Piped | Grayjay |
|---|-------|---------|-------|---------|
| Algorithm-free | Yes | No | Yes | Yes |
| Self-hosted | Yes | No | Yes | No |
| Persistent history | Yes | Yes | No | Device-only |
| Multi-device sync | Yes | Yes | No | No |
| Shareable feed lists | Yes | No | No | No |
| Download support | Yes | No | Some | Yes |

Piped is great for privacy-first proxying for many users. Grayjay is great as a mobile app. Feeds is for when you want a single server that holds your subscriptions, history, and curated lists—your own YouTube replacement that you control completely.

## Features

- **Feed management** — Create feeds like "Tech", "Music", "Politics", "Cooking"
- **Channel organization** — Add channels to feeds, move them around, AI-assisted bulk organization
- **Import subscriptions** — Import from NewPipe exports or add channels directly
- **Watch history** — Track what you've watched with progress saved across devices
- **Everything view** — See all recent videos across all feeds in one place
- **Streaming & downloads** — Uses yt-dlp under the hood for reliable playback

## Quick start

### Prerequisites

- Go 1.21+
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) installed and in PATH

### Run it

```bash
# Clone and build
git clone https://github.com/erik/feeds.git
cd feeds
go build -o feeds ./cmd/server

# Run (creates feeds.db in current directory)
./feeds
```

Open `http://localhost:8080` in your browser.

### Configuration

Create a `.env` file or set environment variables:

```bash
PORT=8080              # Server port
DB_PATH=./feeds.db     # SQLite database location
OPENAI_API_KEY=...     # Optional: for AI-powered feed organization
```

## Sharing feeds

Export any feed as JSON from the UI. Share the file however you want—email, paste in a forum, host it somewhere. Recipients import it as a new feed without affecting their existing subscriptions.

This makes it easy to share curated lists: "Here's my favorite woodworking channels" or "Political commentators I follow across the spectrum."

## License

MIT
