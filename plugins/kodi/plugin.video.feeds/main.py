#!/usr/bin/env python3
"""Feeds Kodi Plugin - Entry Point"""

import sys
import urllib.parse

import xbmc
import xbmcgui
import xbmcplugin
import xbmcaddon
import xbmcvfs

# Add lib to path
ADDON = xbmcaddon.Addon()
sys.path.insert(0, xbmcvfs.translatePath(ADDON.getAddonInfo("path") + "/resources/lib"))

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
    # Map enum index to quality value: 0=Ask, 1=720p, 2=1080p, 3=Best
    quality_map = {0: 0, 1: 720, 2: 1080, 3: 9999}
    quality_index = int(ADDON.getSetting("default_quality") or 0)
    return {
        "server_url": ADDON.getSetting("server_url"),
        "default_quality": quality_map.get(quality_index, 0),
        "sponsorblock_enabled": ADDON.getSetting("sponsorblock_enabled") == "true",
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
    from player import FeedsPlayer

    settings = get_settings()

    try:
        api = get_api()

        # Get available qualities
        qualities_data = api.get_video_qualities(video_id)
        available = qualities_data.get("available") or []
        cached = qualities_data.get("cached") or []

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
