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
