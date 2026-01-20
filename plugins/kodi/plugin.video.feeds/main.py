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
