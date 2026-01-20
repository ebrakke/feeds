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
