# Kodi Plugin Design: Feeds TV Experience

**Date:** 2026-01-20
**Status:** Draft

## Overview

A Kodi video add-on that connects to a self-hosted Feeds server, enabling a living room TV experience for browsing and watching YouTube subscriptions organized by feed.

## Goals

- Browse feeds on TV with remote control navigation
- Play videos with quality selection (download-then-stream for stability)
- Sync watch progress bidirectionally with the Feeds server
- Auto-skip sponsor segments via SponsorBlock
- Hide Shorts (not suitable for horizontal TV viewing)

## Architecture

```
┌─────────────┐         ┌─────────────────┐         ┌──────────────┐
│   Kodi UI   │ ──────► │  Feeds Plugin   │ ──────► │ Feeds Server │
│  (Remote)   │ ◄────── │  (Python)       │ ◄────── │   (Go API)   │
└─────────────┘         └─────────────────┘         └──────────────┘
```

The plugin is a standard Kodi Python video add-on that:
1. Calls the Feeds server REST API
2. Presents feeds/videos as browsable Kodi directories
3. Downloads videos server-side before playback for stability
4. Streams from the server's cached file
5. Reports watch progress back to the server

The Feeds server handles all yt-dlp operations - the plugin just makes HTTP requests.

## Navigation Structure

### Main Menu

```
Feeds
├── Tech
│   ├── Video 1 (3 days ago)
│   ├── Video 2 (5 days ago)
│   └── ...
├── Music
│   └── ...
├── Cooking
│   └── ...
└── History
```

### Video List Display

Each video item shows:
- Thumbnail (from YouTube)
- Title
- Channel name
- Published date (relative: "3 days ago")
- Duration
- Resume indicator (if partially watched)

Shorts are filtered out server-side and never displayed.

Pagination: 50 videos per page with "Next page" navigation item.

## Playback Flow

### Quality Selection & Download

1. User selects a video
2. Plugin fetches available qualities: `GET /api/videos/{id}/qualities`
3. Quality picker dialog appears (720p, 1080p, etc.)
4. Plugin initiates download: `POST /api/videos/{id}/download` with chosen quality
5. Shows progress dialog: "Preparing video... 45%"
   - Polls download status via SSE: `GET /api/videos/{id}/download/status`
6. Once complete, plays via: `GET /api/stream/{id}`

This download-then-stream approach provides stable playback without mid-video quality issues.

### Resume Support

Before starting playback:
1. Plugin checks server for existing watch progress
2. If progress exists, shows dialog: "Resume from 12:34?"
3. On confirm, starts Kodi playback at saved position

### Progress Reporting

- Every 30 seconds during playback: `POST /api/videos/{id}/progress`
- On pause or stop: immediately reports current position
- On reaching ~90% completion: `POST /api/videos/{id}/watched`

This enables seamless handoff between TV and other devices.

### SponsorBlock Integration

Before playback starts:
1. Plugin fetches segments: `GET /api/videos/{id}/segments`
2. Background monitor runs during playback
3. When player position enters sponsor segment, auto-seeks past it
4. Brief notification: "Skipped sponsor"

```python
class SponsorBlockMonitor:
    def run(self):
        while self.playing:
            position = self.player.getTime()
            for segment in self.segments:
                if segment.start <= position < segment.end:
                    self.player.seekTime(segment.end)
                    self.notify("Skipped sponsor")
            xbmc.sleep(1000)  # Check every second
```

## Settings

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| Server URL | text | (required) | Feeds server address (e.g., `http://192.168.1.50:8080`) |
| Default Quality | select | Ask each time | Can set 720p/1080p/Best to skip picker |
| SponsorBlock | boolean | On | Toggle sponsor auto-skip |
| Videos per page | number | 50 | Items per page (25-100) |

### First Run

- Plugin detects no server configured
- Prompts user for server URL
- Tests connection before saving

## Plugin Structure

```
plugin.video.feeds/
├── addon.xml                 # Kodi metadata & dependencies
├── main.py                   # Entry point & URL routing
├── resources/
│   ├── settings.xml          # Settings schema
│   ├── language/
│   │   └── resource.language.en_gb/
│   │       └── strings.po    # Translatable strings
│   └── lib/
│       ├── feeds_api.py      # HTTP client for Feeds server
│       ├── player.py         # Playback monitor (progress sync, SponsorBlock)
│       └── utils.py          # Time formatting, pagination helpers
├── icon.png                  # Addon icon (512x512)
└── fanart.jpg                # Background art (1920x1080)
```

## API Usage

### Endpoints Used

| Endpoint | Purpose |
|----------|---------|
| `GET /api/feeds` | List all feeds for main menu |
| `GET /api/feeds/{id}` | Get videos in a feed (with pagination) |
| `GET /api/videos/history` | Get watch history |
| `GET /api/videos/{id}/qualities` | Available quality options |
| `POST /api/videos/{id}/download` | Start server-side download |
| `GET /api/videos/{id}/download/status` | SSE stream for download progress |
| `GET /api/stream/{id}` | Stream the cached video file |
| `GET /api/videos/{id}/segments` | SponsorBlock segments |
| `POST /api/videos/{id}/progress` | Report watch progress |
| `POST /api/videos/{id}/watched` | Mark video as watched |

### Request Flow Example

```
User selects "Tech" feed
  → GET /api/feeds/3?limit=50&offset=0
  ← { videos: [...], total: 127 }

User selects a video
  → GET /api/videos/dQw4w9WgXcQ/qualities
  ← { available: ["720p", "1080p"], cached: ["720p"] }

User picks 1080p
  → POST /api/videos/dQw4w9WgXcQ/download { quality: "1080p" }
  → GET /api/videos/dQw4w9WgXcQ/download/status (SSE)
  ← progress events... done!

Playback starts
  → GET /api/videos/dQw4w9WgXcQ/segments
  → GET /api/stream/dQw4w9WgXcQ

During playback (every 30s)
  → POST /api/videos/dQw4w9WgXcQ/progress { progress: 180, duration: 600 }
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Server unreachable | "Cannot connect to Feeds server" dialog with retry |
| Download fails | "Failed to prepare video" with retry option |
| Stream interrupted | Attempt reconnection, resume from last position |
| Invalid server URL | Settings prompt with validation message |

## Future Considerations

Not in initial scope, but possible additions:

- **Search:** Search across all videos
- **Refresh feeds:** Trigger feed refresh from Kodi
- **Channel view:** Browse by channel instead of feed
- **Shuffle mode:** Random video from a feed
- **Keyboard support:** For users with wireless keyboards

## Dependencies

- Python 3.x (Kodi 19+ requirement)
- Kodi video add-on APIs (`xbmc`, `xbmcgui`, `xbmcplugin`, `xbmcaddon`)
- Standard library `urllib` or bundled `requests`

## Compatibility

- Kodi 19 (Matrix) and later
- All platforms: Linux, Windows, macOS, Android, LibreELEC, OSMC, CoreELEC
