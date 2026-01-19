# Changelog

All notable changes to Feeds will be documented in this file.

## [0.2.0] - 2025-01-19

### Added

- **Add to feed from any video** - Click the menu on any video card to add its channel to a feed. On desktop, hover to see a submenu of available feeds. On mobile, a native bottom sheet slides up.
- **Toast notifications** - Visual feedback when adding channels to feeds, with success/error states.
- **Mark as watched/unwatched** - Toggle watched status directly from the video card menu.
- **Shuffle tab** - Each feed now has a "Shuffle" tab that shows videos in random order.
- **Up Next focus mode** - On the watch page, a focused "Up Next" panel with infinite scroll and shorts filtering.
- **Shorts detection** - Videos are automatically tagged as shorts during feed refresh, with an option to filter them out.
- **HD downloads** - Download videos in 720p for offline viewing, with progress indicator.
- **Multi-feed channels** - Channels can now belong to multiple feeds simultaneously. Move or copy channels between feeds freely.
- **Feed chips on channel pages** - See which feeds a channel belongs to and manage membership.

### Changed

- **Redesigned quality selector** - Cleaner UI separating "Now Playing" from "Download HD" option.
- **Color scheme** - Updated from amber to emerald green for a fresher look.
- **Improved onboarding** - More granular frequency tiers when importing YouTube history, with clearer Google Takeout instructions.

### Fixed

- **Video stream cleanup** - Fixed phantom audio when navigating quickly between videos.
- **Mobile menu visibility** - Video card menu button is now always visible on mobile (no hover needed).
- **Z-index layering** - Fixed dropdown menus appearing behind other elements.
- **Channel removal** - "Remove from feed" now correctly removes the channel from the feed instead of deleting it entirely.

## [0.1.0] - 2025-01-11

Initial release.

- Feed management with custom categories
- Channel organization with import from NewPipe
- Watch history tracking with cross-device sync
- yt-dlp powered streaming
- Self-hosted SQLite database
