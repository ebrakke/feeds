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

    def get_stream_url(self, video_id: str, quality: str = None) -> str:
        """Get stream URL for a video."""
        if quality:
            return f"{self.base_url}/api/stream/{video_id}?quality={quality}"
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
