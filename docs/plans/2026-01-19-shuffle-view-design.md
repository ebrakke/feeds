# Shuffle View Design

## Overview

Add a "Shuffle" tab to each feed page that displays videos in random order, providing an alternative to the chronological "newest first" browsing pattern.

## Requirements

- New "Shuffle" tab on the feed detail page (`/feeds/[id]`)
- Shows only **unwatched, non-short videos** from that feed
- Videos displayed in **random order**
- Reshuffles on page refresh
- Same video card layout as current feed view
- Infinite scroll/pagination support

## UI Design

Tab placement on feed page:
```
[Videos]  [Shorts]  [Shuffle]  [Channels]
```

## Technical Approach

### Backend

Add a new API endpoint or query parameter to support random ordering:

**Option: Query parameter on existing endpoint**
- `GET /api/feeds/{id}?sort=random&hideWatched=true&hideShorts=true`
- Simpler, reuses existing infrastructure

**Database query:**
```sql
SELECT * FROM videos
WHERE channel_id IN (SELECT id FROM channels WHERE feed_id = ?)
  AND is_short = false OR is_short IS NULL
  AND id NOT IN (SELECT video_id FROM watch_progress WHERE progress_seconds > 0)
ORDER BY RANDOM()
LIMIT ? OFFSET ?
```

Note: Random pagination is tricky - subsequent pages may repeat or skip items. Options:
1. Seed the random with a session value (consistent within session)
2. Fetch larger batches client-side
3. Accept some repetition on "load more"

### Frontend

- Add "Shuffle" tab to `/web/frontend/src/routes/feeds/[id]/+page.svelte`
- Reuse existing video grid component
- Pass sort parameter to API calls
- Consider adding a "Reshuffle" button for re-randomizing without page refresh

## Out of Scope

- Global shuffle across all feeds
- Including watched videos
- Including shorts
- Fetching older/popular videos from YouTube (future enhancement)
- View count display

## Future Enhancements

- Back-catalog fetching: Fetch top N popular videos per channel for richer shuffle pool
- View counts: Display and optionally sort by popularity
- Weighted random: Popular videos appear more frequently
