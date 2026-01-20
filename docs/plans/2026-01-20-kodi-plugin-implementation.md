# Kodi Plugin Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Kodi video add-on that browses Feeds, plays videos with quality selection, syncs watch progress, and auto-skips sponsors.

**Architecture:** Python plugin calls Feeds server REST API, presents directory listings, triggers server-side downloads before playback, streams cached files, and monitors playback for progress sync + SponsorBlock.

**Tech Stack:** Python 3.x, Kodi xbmc/xbmcgui/xbmcplugin APIs, urllib for HTTP

**Design Doc:** `docs/plans/2026-01-20-kodi-plugin-design.md`

---

## Task 1: Plugin Skeleton & Addon Metadata

**Files:**
- Create: `plugins/kodi/plugin.video.feeds/addon.xml`
- Create: `plugins/kodi/plugin.video.feeds/main.py`
- Create: `plugins/kodi/README.md`

**Step 1: Create addon.xml**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<addon id="plugin.video.feeds"
       name="Feeds"
       version="0.1.0"
       provider-name="feeds">
    <requires>
        <import addon="xbmc.python" version="3.0.0"/>
    </requires>
    <extension point="xbmc.python.pluginsource" library="main.py">
        <provides>video</provides>
    </extension>
    <extension point="xbmc.addon.metadata">
        <platform>all</platform>
        <summary lang="en_GB">Browse your Feeds subscriptions</summary>
        <description lang="en_GB">Connect to your self-hosted Feeds server to browse YouTube subscriptions organized by feed. Features quality selection, watch progress sync, and SponsorBlock integration.</description>
        <license>MIT</license>
    </extension>
</addon>
```

**Step 2: Create minimal main.py**

```python
#!/usr/bin/env python3
"""Feeds Kodi Plugin - Entry Point"""

import sys
import xbmcgui
import xbmcplugin
import xbmcaddon

ADDON = xbmcaddon.Addon()
HANDLE = int(sys.argv[1])


def main():
    """Main entry point."""
    xbmcgui.Dialog().ok("Feeds", "Plugin loaded successfully!")
    xbmcplugin.endOfDirectory(HANDLE)


if __name__ == "__main__":
    main()
```

**Step 3: Create README.md**

```markdown
# Feeds Kodi Plugin

A Kodi video add-on for browsing and watching videos from your self-hosted Feeds server.

## Installation

1. Zip the `plugin.video.feeds` folder
2. In Kodi: Settings → Add-ons → Install from zip file
3. Configure your Feeds server URL in addon settings

## Features

- Browse feeds organized by topic
- Quality selection before playback
- Watch progress sync across devices
- SponsorBlock auto-skip
- Shorts filtered out

## Requirements

- Kodi 19 (Matrix) or later
- A running Feeds server
```

**Step 4: Commit**

```bash
git add plugins/kodi/
git commit -m "feat(kodi): add plugin skeleton with addon.xml"
```

---

## Task 2: Settings Configuration

**Files:**
- Create: `plugins/kodi/plugin.video.feeds/resources/settings.xml`
- Modify: `plugins/kodi/plugin.video.feeds/addon.xml` (add settings extension)

**Step 1: Create settings.xml**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<settings version="2">
    <section id="connection">
        <category id="server" label="Server">
            <group id="1">
                <setting id="server_url" type="string" label="Server URL" help="Your Feeds server address (e.g., http://192.168.1.50:8080)">
                    <default></default>
                    <control type="edit" format="string"/>
                </setting>
            </group>
        </category>
    </section>
    <section id="playback">
        <category id="quality" label="Playback">
            <group id="1">
                <setting id="default_quality" type="integer" label="Default Quality" help="Quality to use for video playback">
                    <default>0</default>
                    <constraints>
                        <options>
                            <option label="Ask each time">0</option>
                            <option label="720p">720</option>
                            <option label="1080p">1080</option>
                            <option label="Best available">9999</option>
                        </options>
                    </constraints>
                    <control type="spinner" format="string"/>
                </setting>
                <setting id="sponsorblock_enabled" type="boolean" label="SponsorBlock" help="Auto-skip sponsor segments">
                    <default>true</default>
                    <control type="toggle"/>
                </setting>
                <setting id="videos_per_page" type="integer" label="Videos per page" help="Number of videos to load at once">
                    <default>50</default>
                    <constraints>
                        <minimum>25</minimum>
                        <maximum>100</maximum>
                        <step>25</step>
                    </constraints>
                    <control type="slider" format="integer"/>
                </setting>
            </group>
        </category>
    </section>
</settings>
```

**Step 2: Update addon.xml to include settings**

Add this extension before the closing `</addon>`:

```xml
    <extension point="xbmc.service" library="resources/lib/service.py" start="startup"/>
```

Actually, skip that for now - settings work automatically. Just commit the settings.xml.

**Step 3: Commit**

```bash
git add plugins/kodi/plugin.video.feeds/resources/settings.xml
git commit -m "feat(kodi): add settings configuration"
```

---

## Task 3: Feeds API Client

**Files:**
- Create: `plugins/kodi/plugin.video.feeds/resources/lib/__init__.py`
- Create: `plugins/kodi/plugin.video.feeds/resources/lib/feeds_api.py`

**Step 1: Create __init__.py (empty)**

```python
# Feeds Kodi Plugin Library
```

**Step 2: Create feeds_api.py**

```python
"""Feeds Server API Client"""

import json
import urllib.request
import urllib.error
import urllib.parse
from typing import Optional


class FeedsAPIError(Exception):
    """Error from Feeds API"""
    pass


class FeedsAPI:
    """HTTP client for Feeds server API."""

    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")

    def _request(self, method: str, path: str, data: Optional[dict] = None) -> dict:
        """Make HTTP request to API."""
        url = f"{self.base_url}{path}"
        headers = {"Content-Type": "application/json"}

        body = json.dumps(data).encode("utf-8") if data else None
        req = urllib.request.Request(url, data=body, headers=headers, method=method)

        try:
            with urllib.request.urlopen(req, timeout=30) as response:
                return json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            error_body = e.read().decode("utf-8")
            try:
                error_data = json.loads(error_body)
                raise FeedsAPIError(error_data.get("error", str(e)))
            except json.JSONDecodeError:
                raise FeedsAPIError(str(e))
        except urllib.error.URLError as e:
            raise FeedsAPIError(f"Cannot connect to server: {e.reason}")

    def get_feeds(self) -> list:
        """Get all feeds."""
        return self._request("GET", "/api/feeds")

    def get_feed_videos(self, feed_id: int, limit: int = 50, offset: int = 0) -> dict:
        """Get videos in a feed with pagination."""
        return self._request("GET", f"/api/feeds/{feed_id}?limit={limit}&offset={offset}")

    def get_history(self, limit: int = 50, offset: int = 0) -> dict:
        """Get watch history."""
        return self._request("GET", f"/api/videos/history?limit={limit}&offset={offset}")

    def get_video_qualities(self, video_id: str) -> dict:
        """Get available qualities for a video."""
        return self._request("GET", f"/api/videos/{video_id}/qualities")

    def start_download(self, video_id: str, quality: str) -> dict:
        """Start server-side download of video."""
        return self._request("POST", f"/api/videos/{video_id}/download", {"quality": quality})

    def get_stream_url(self, video_id: str) -> str:
        """Get stream URL for a video."""
        return f"{self.base_url}/api/stream/{video_id}"

    def get_segments(self, video_id: str) -> list:
        """Get SponsorBlock segments."""
        try:
            result = self._request("GET", f"/api/videos/{video_id}/segments")
            return result if isinstance(result, list) else result.get("segments", [])
        except FeedsAPIError:
            return []

    def report_progress(self, video_id: str, progress: int, duration: int) -> None:
        """Report watch progress."""
        self._request("POST", f"/api/videos/{video_id}/progress", {
            "progress": progress,
            "duration": duration
        })

    def mark_watched(self, video_id: str) -> None:
        """Mark video as watched."""
        self._request("POST", f"/api/videos/{video_id}/watched", {})

    def test_connection(self) -> bool:
        """Test if server is reachable."""
        try:
            self.get_feeds()
            return True
        except FeedsAPIError:
            return False
```

**Step 3: Commit**

```bash
git add plugins/kodi/plugin.video.feeds/resources/lib/
git commit -m "feat(kodi): add Feeds API client"
```

---

## Task 4: Utility Functions

**Files:**
- Create: `plugins/kodi/plugin.video.feeds/resources/lib/utils.py`

**Step 1: Create utils.py**

```python
"""Utility functions for Feeds Kodi Plugin"""

from datetime import datetime, timezone
from typing import Optional


def format_duration(seconds: Optional[int]) -> str:
    """Format duration in seconds to HH:MM:SS or MM:SS."""
    if not seconds:
        return ""
    hours, remainder = divmod(seconds, 3600)
    minutes, secs = divmod(remainder, 60)
    if hours:
        return f"{hours}:{minutes:02d}:{secs:02d}"
    return f"{minutes}:{secs:02d}"


def format_relative_date(iso_date: str) -> str:
    """Format ISO date string to relative time (e.g., '3 days ago')."""
    try:
        # Parse ISO format
        if "Z" in iso_date:
            dt = datetime.fromisoformat(iso_date.replace("Z", "+00:00"))
        elif "+" in iso_date or iso_date.endswith("-00:00"):
            dt = datetime.fromisoformat(iso_date)
        else:
            dt = datetime.fromisoformat(iso_date).replace(tzinfo=timezone.utc)

        now = datetime.now(timezone.utc)
        diff = now - dt

        days = diff.days
        if days == 0:
            hours = diff.seconds // 3600
            if hours == 0:
                minutes = diff.seconds // 60
                if minutes == 0:
                    return "Just now"
                return f"{minutes} minute{'s' if minutes != 1 else ''} ago"
            return f"{hours} hour{'s' if hours != 1 else ''} ago"
        elif days == 1:
            return "Yesterday"
        elif days < 7:
            return f"{days} days ago"
        elif days < 30:
            weeks = days // 7
            return f"{weeks} week{'s' if weeks != 1 else ''} ago"
        elif days < 365:
            months = days // 30
            return f"{months} month{'s' if months != 1 else ''} ago"
        else:
            years = days // 365
            return f"{years} year{'s' if years != 1 else ''} ago"
    except (ValueError, TypeError):
        return ""


def build_plugin_url(base_url: str, **params) -> str:
    """Build a plugin:// URL with parameters."""
    import urllib.parse
    if params:
        return f"{base_url}?{urllib.parse.urlencode(params)}"
    return base_url
```

**Step 2: Commit**

```bash
git add plugins/kodi/plugin.video.feeds/resources/lib/utils.py
git commit -m "feat(kodi): add utility functions"
```

---

## Task 5: Main Navigation (Feeds List)

**Files:**
- Modify: `plugins/kodi/plugin.video.feeds/main.py`

**Step 1: Rewrite main.py with routing and feeds list**

```python
#!/usr/bin/env python3
"""Feeds Kodi Plugin - Entry Point"""

import sys
import urllib.parse

import xbmc
import xbmcgui
import xbmcplugin
import xbmcaddon

# Add lib to path
ADDON = xbmcaddon.Addon()
sys.path.insert(0, xbmc.translatePath(ADDON.getAddonInfo("path") + "/resources/lib"))

from feeds_api import FeedsAPI, FeedsAPIError
from utils import format_duration, format_relative_date, build_plugin_url

HANDLE = int(sys.argv[1])
BASE_URL = sys.argv[0]


def get_api() -> FeedsAPI:
    """Get configured API client."""
    server_url = ADDON.getSetting("server_url")
    if not server_url:
        xbmcgui.Dialog().ok("Feeds", "Please configure your server URL in addon settings.")
        ADDON.openSettings()
        server_url = ADDON.getSetting("server_url")
        if not server_url:
            raise FeedsAPIError("No server configured")
    return FeedsAPI(server_url)


def get_settings() -> dict:
    """Get addon settings."""
    return {
        "server_url": ADDON.getSetting("server_url"),
        "default_quality": int(ADDON.getSetting("default_quality") or 0),
        "sponsorblock_enabled": ADDON.getSettingBool("sponsorblock_enabled"),
        "videos_per_page": int(ADDON.getSetting("videos_per_page") or 50),
    }


def list_feeds():
    """Show main menu with all feeds."""
    try:
        api = get_api()
        feeds = api.get_feeds()
    except FeedsAPIError as e:
        xbmcgui.Dialog().ok("Feeds", f"Error: {e}")
        return

    xbmcplugin.setPluginCategory(HANDLE, "Feeds")
    xbmcplugin.setContent(HANDLE, "files")

    for feed in feeds:
        feed_id = feed.get("id")
        name = feed.get("name", "Unknown")

        li = xbmcgui.ListItem(label=name)
        li.setInfo("video", {"title": name})

        url = build_plugin_url(BASE_URL, action="list_videos", feed_id=feed_id)
        xbmcplugin.addDirectoryItem(HANDLE, url, li, isFolder=True)

    # Add History
    li = xbmcgui.ListItem(label="[History]")
    li.setInfo("video", {"title": "History"})
    url = build_plugin_url(BASE_URL, action="history")
    xbmcplugin.addDirectoryItem(HANDLE, url, li, isFolder=True)

    xbmcplugin.endOfDirectory(HANDLE)


def list_videos(feed_id: int, offset: int = 0):
    """Show videos in a feed."""
    settings = get_settings()
    limit = settings["videos_per_page"]

    try:
        api = get_api()
        result = api.get_feed_videos(feed_id, limit=limit, offset=offset)
    except FeedsAPIError as e:
        xbmcgui.Dialog().ok("Feeds", f"Error: {e}")
        return

    feed_name = result.get("name", "Videos")
    videos = result.get("videos", [])
    total = result.get("total", 0)

    xbmcplugin.setPluginCategory(HANDLE, feed_name)
    xbmcplugin.setContent(HANDLE, "videos")

    for video in videos:
        # Skip shorts
        if video.get("is_short"):
            continue

        video_id = video.get("id")
        title = video.get("title", "Unknown")
        channel = video.get("channel_name", "")
        thumbnail = video.get("thumbnail", "")
        duration = video.get("duration", 0)
        published = video.get("published", "")

        # Format label with channel and date
        date_str = format_relative_date(published)
        label = f"{title}"
        label2 = f"{channel} • {date_str}" if date_str else channel

        li = xbmcgui.ListItem(label=label, label2=label2)
        li.setArt({"thumb": thumbnail, "poster": thumbnail})
        li.setInfo("video", {
            "title": title,
            "plot": f"{channel}\n{date_str}",
            "duration": duration,
            "mediatype": "video",
        })
        li.setProperty("IsPlayable", "true")

        url = build_plugin_url(BASE_URL, action="play", video_id=video_id)
        xbmcplugin.addDirectoryItem(HANDLE, url, li, isFolder=False)

    # Pagination
    if offset + limit < total:
        li = xbmcgui.ListItem(label="[Next Page]")
        url = build_plugin_url(BASE_URL, action="list_videos", feed_id=feed_id, offset=offset + limit)
        xbmcplugin.addDirectoryItem(HANDLE, url, li, isFolder=True)

    xbmcplugin.endOfDirectory(HANDLE)


def list_history(offset: int = 0):
    """Show watch history."""
    settings = get_settings()
    limit = settings["videos_per_page"]

    try:
        api = get_api()
        result = api.get_history(limit=limit, offset=offset)
    except FeedsAPIError as e:
        xbmcgui.Dialog().ok("Feeds", f"Error: {e}")
        return

    videos = result.get("videos", [])
    total = result.get("total", 0)

    xbmcplugin.setPluginCategory(HANDLE, "History")
    xbmcplugin.setContent(HANDLE, "videos")

    for video in videos:
        video_id = video.get("id")
        title = video.get("title", "Unknown")
        channel = video.get("channel_name", "")
        thumbnail = video.get("thumbnail", "")
        duration = video.get("duration", 0)

        li = xbmcgui.ListItem(label=title, label2=channel)
        li.setArt({"thumb": thumbnail, "poster": thumbnail})
        li.setInfo("video", {
            "title": title,
            "plot": channel,
            "duration": duration,
            "mediatype": "video",
        })
        li.setProperty("IsPlayable", "true")

        url = build_plugin_url(BASE_URL, action="play", video_id=video_id)
        xbmcplugin.addDirectoryItem(HANDLE, url, li, isFolder=False)

    # Pagination
    if offset + limit < total:
        li = xbmcgui.ListItem(label="[Next Page]")
        url = build_plugin_url(BASE_URL, action="history", offset=offset + limit)
        xbmcplugin.addDirectoryItem(HANDLE, url, li, isFolder=True)

    xbmcplugin.endOfDirectory(HANDLE)


def play_video(video_id: str):
    """Play a video with quality selection and download."""
    settings = get_settings()

    try:
        api = get_api()

        # Get available qualities
        qualities_data = api.get_video_qualities(video_id)
        available = qualities_data.get("available", [])
        cached = qualities_data.get("cached", [])

        if not available and not cached:
            xbmcgui.Dialog().ok("Feeds", "No qualities available for this video.")
            return

        # Determine quality to use
        default_quality = settings["default_quality"]
        selected_quality = None

        if default_quality == 0:  # Ask each time
            # Build options list, mark cached ones
            options = []
            quality_values = []
            for q in available:
                label = q
                if q in cached:
                    label += " (cached)"
                options.append(label)
                quality_values.append(q)

            choice = xbmcgui.Dialog().select("Select Quality", options)
            if choice < 0:
                return
            selected_quality = quality_values[choice]
        elif default_quality == 9999:  # Best
            selected_quality = available[-1] if available else cached[-1]
        else:
            # Find matching quality or closest
            target = str(default_quality) + "p"
            if target in available:
                selected_quality = target
            elif target in cached:
                selected_quality = target
            else:
                selected_quality = available[-1] if available else cached[-1]

        # Check if already cached
        if selected_quality not in cached:
            # Need to download first
            pDialog = xbmcgui.DialogProgress()
            pDialog.create("Preparing Video", "Starting download...")

            api.start_download(video_id, selected_quality)

            # Poll for completion (simplified - in practice would use SSE)
            import time
            for i in range(120):  # Max 2 minutes
                if pDialog.iscanceled():
                    return
                pDialog.update(int(i * 100 / 120), f"Downloading {selected_quality}...")
                time.sleep(1)

                # Check if now cached
                new_qualities = api.get_video_qualities(video_id)
                if selected_quality in new_qualities.get("cached", []):
                    break

            pDialog.close()

        # Play the video
        stream_url = api.get_stream_url(video_id)
        li = xbmcgui.ListItem(path=stream_url)
        xbmcplugin.setResolvedUrl(HANDLE, True, li)

    except FeedsAPIError as e:
        xbmcgui.Dialog().ok("Feeds", f"Error: {e}")


def router(paramstring: str):
    """Route to appropriate function based on parameters."""
    params = dict(urllib.parse.parse_qsl(paramstring))
    action = params.get("action")

    if action is None:
        list_feeds()
    elif action == "list_videos":
        list_videos(int(params["feed_id"]), int(params.get("offset", 0)))
    elif action == "history":
        list_history(int(params.get("offset", 0)))
    elif action == "play":
        play_video(params["video_id"])
    else:
        raise ValueError(f"Unknown action: {action}")


if __name__ == "__main__":
    router(sys.argv[2][1:])  # Strip leading '?'
```

**Step 2: Commit**

```bash
git add plugins/kodi/plugin.video.feeds/main.py
git commit -m "feat(kodi): add navigation and video listing"
```

---

## Task 6: Playback Monitor (Progress Sync & SponsorBlock)

**Files:**
- Create: `plugins/kodi/plugin.video.feeds/resources/lib/player.py`

**Step 1: Create player.py**

```python
"""Playback monitor for progress sync and SponsorBlock."""

import xbmc
import xbmcgui


class FeedsPlayer(xbmc.Player):
    """Custom player with progress tracking and SponsorBlock support."""

    def __init__(self, api, video_id: str, segments: list, sponsorblock_enabled: bool):
        super().__init__()
        self.api = api
        self.video_id = video_id
        self.segments = segments
        self.sponsorblock_enabled = sponsorblock_enabled
        self.last_progress_report = 0
        self.duration = 0
        self.playing = False

    def onAVStarted(self):
        """Called when playback actually starts."""
        self.playing = True
        try:
            self.duration = int(self.getTotalTime())
        except RuntimeError:
            self.duration = 0
        self._start_monitor()

    def onPlayBackStopped(self):
        """Called when playback is stopped."""
        self.playing = False
        self._report_progress()

    def onPlayBackPaused(self):
        """Called when playback is paused."""
        self._report_progress()

    def onPlayBackEnded(self):
        """Called when playback ends naturally."""
        self.playing = False
        # Mark as watched if we reached near the end
        try:
            if self.duration > 0:
                self.api.mark_watched(self.video_id)
        except Exception:
            pass

    def _report_progress(self):
        """Report current progress to server."""
        try:
            position = int(self.getTime())
            if self.duration > 0:
                self.api.report_progress(self.video_id, position, self.duration)
        except Exception:
            pass

    def _start_monitor(self):
        """Start the playback monitoring loop."""
        report_interval = 30  # seconds

        while self.playing and not xbmc.Monitor().abortRequested():
            try:
                position = self.getTime()
                current_time = int(position)

                # SponsorBlock: check if in sponsor segment
                if self.sponsorblock_enabled and self.segments:
                    for seg in self.segments:
                        start = seg.get("start_time", 0)
                        end = seg.get("end_time", 0)
                        if start <= position < end:
                            # Skip to end of segment
                            self.seekTime(end)
                            xbmcgui.Dialog().notification(
                                "SponsorBlock",
                                "Skipped sponsor",
                                xbmcgui.NOTIFICATION_INFO,
                                2000
                            )
                            break

                # Progress reporting every 30 seconds
                if current_time - self.last_progress_report >= report_interval:
                    self._report_progress()
                    self.last_progress_report = current_time

                # Check if 90% complete -> mark as watched
                if self.duration > 0 and position > self.duration * 0.9:
                    try:
                        self.api.mark_watched(self.video_id)
                    except Exception:
                        pass

            except RuntimeError:
                # Player no longer active
                break

            xbmc.sleep(1000)  # Check every second
```

**Step 2: Commit**

```bash
git add plugins/kodi/plugin.video.feeds/resources/lib/player.py
git commit -m "feat(kodi): add playback monitor with progress sync and SponsorBlock"
```

---

## Task 7: Integrate Player into Main

**Files:**
- Modify: `plugins/kodi/plugin.video.feeds/main.py`

**Step 1: Update play_video function to use FeedsPlayer**

Replace the `play_video` function with this updated version:

```python
def play_video(video_id: str):
    """Play a video with quality selection and download."""
    from player import FeedsPlayer

    settings = get_settings()

    try:
        api = get_api()

        # Get available qualities
        qualities_data = api.get_video_qualities(video_id)
        available = qualities_data.get("available", [])
        cached = qualities_data.get("cached", [])

        if not available and not cached:
            xbmcgui.Dialog().ok("Feeds", "No qualities available for this video.")
            return

        # Determine quality to use
        default_quality = settings["default_quality"]
        selected_quality = None

        if default_quality == 0:  # Ask each time
            # Build options list, mark cached ones
            options = []
            quality_values = []
            for q in available:
                label = q
                if q in cached:
                    label += " (cached)"
                options.append(label)
                quality_values.append(q)

            choice = xbmcgui.Dialog().select("Select Quality", options)
            if choice < 0:
                return
            selected_quality = quality_values[choice]
        elif default_quality == 9999:  # Best
            selected_quality = available[-1] if available else cached[-1]
        else:
            # Find matching quality or closest
            target = str(default_quality) + "p"
            if target in available:
                selected_quality = target
            elif target in cached:
                selected_quality = target
            else:
                selected_quality = available[-1] if available else cached[-1]

        # Check if already cached
        if selected_quality not in cached:
            # Need to download first
            pDialog = xbmcgui.DialogProgress()
            pDialog.create("Preparing Video", "Starting download...")

            api.start_download(video_id, selected_quality)

            # Poll for completion
            import time
            for i in range(120):  # Max 2 minutes
                if pDialog.iscanceled():
                    return
                pDialog.update(int(i * 100 / 120), f"Downloading {selected_quality}...")
                time.sleep(1)

                # Check if now cached
                new_qualities = api.get_video_qualities(video_id)
                if selected_quality in new_qualities.get("cached", []):
                    break

            pDialog.close()

        # Get SponsorBlock segments if enabled
        segments = []
        if settings["sponsorblock_enabled"]:
            segments = api.get_segments(video_id)

        # Play the video
        stream_url = api.get_stream_url(video_id)
        li = xbmcgui.ListItem(path=stream_url)
        xbmcplugin.setResolvedUrl(HANDLE, True, li)

        # Start custom player for monitoring
        player = FeedsPlayer(api, video_id, segments, settings["sponsorblock_enabled"])

    except FeedsAPIError as e:
        xbmcgui.Dialog().ok("Feeds", f"Error: {e}")
```

**Step 2: Commit**

```bash
git add plugins/kodi/plugin.video.feeds/main.py
git commit -m "feat(kodi): integrate playback monitor into video player"
```

---

## Task 8: Add Icon and Fanart Placeholders

**Files:**
- Create: `plugins/kodi/plugin.video.feeds/icon.png` (placeholder note)
- Create: `plugins/kodi/plugin.video.feeds/fanart.jpg` (placeholder note)

**Step 1: Create .gitkeep files for assets**

Since we can't create actual images, create a README noting what's needed:

```markdown
<!-- plugins/kodi/plugin.video.feeds/resources/media/README.md -->
# Media Assets Needed

- `icon.png` - 512x512 addon icon
- `fanart.jpg` - 1920x1080 background art

Place these files in the plugin root directory (next to addon.xml).
```

Actually, for Kodi to work without crashing, we need placeholder images. Skip this step - Kodi will work without them, just won't have icons.

**Step 2: Commit README update**

```bash
git add plugins/kodi/README.md
git commit -m "docs(kodi): note about icon and fanart assets"
```

---

## Task 9: Final Integration Test Checklist

**Manual Testing Steps:**

1. **Install in Kodi:**
   - Zip `plugin.video.feeds` folder
   - Kodi → Settings → Add-ons → Install from zip

2. **Configure:**
   - Opens settings on first run (no server URL)
   - Enter server URL
   - Test connection works

3. **Browse:**
   - Main menu shows feeds
   - Selecting feed shows videos
   - History shows watched videos
   - Pagination works

4. **Playback:**
   - Quality picker appears
   - Download progress shows
   - Video plays
   - SponsorBlock skips work
   - Progress saves on pause/stop

5. **Cross-device:**
   - Watch partially on TV
   - Check web app shows same progress

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Plugin skeleton | addon.xml, main.py, README.md |
| 2 | Settings | settings.xml |
| 3 | API client | feeds_api.py |
| 4 | Utilities | utils.py |
| 5 | Navigation | main.py (routing, feeds, videos) |
| 6 | Player monitor | player.py |
| 7 | Player integration | main.py update |
| 8 | Assets | placeholder docs |
| 9 | Manual testing | checklist |

Total: 8 implementation tasks + 1 testing task
