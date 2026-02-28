# Smart Feeds Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add auto-generated "Hot This Week" and "Continue Watching" feeds based on in-app watch history.

**Architecture:** Virtual feeds (no database changes) with backend queries computing results on-demand. Background goroutine caches results every 15 minutes. Frontend adds Smart Feeds section above regular feeds on home page.

**Tech Stack:** Go backend, SvelteKit frontend, SQLite database (existing schema)

---

## Task 1: Add Database Queries for Smart Feeds

**Files:**
- Modify: `internal/db/db.go`

**Step 1: Add GetContinueWatching query**

Add this method to `internal/db/db.go` after the existing watch progress methods (~line 848):

```go
// GetContinueWatching returns videos with 10-95% progress, ordered by most recently watched
func (db *DB) GetContinueWatching(limit, offset int) ([]models.Video, map[string]*WatchProgress, int, error) {
	// Get total count first
	var total int
	err := db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM watch_progress wp
		JOIN videos v ON wp.video_id = v.id
		WHERE wp.duration_seconds > 0
		  AND CAST(wp.progress_seconds AS REAL) / wp.duration_seconds >= 0.1
		  AND CAST(wp.progress_seconds AS REAL) / wp.duration_seconds < 0.95
	`).Scan(&total)
	if err != nil {
		return nil, nil, 0, err
	}

	rows, err := db.conn.Query(`
		SELECT v.id, v.channel_id, v.title, v.channel_name, v.thumbnail, v.duration, v.is_short, v.published, v.url,
		       wp.video_id, wp.progress_seconds, wp.duration_seconds, wp.watched_at
		FROM watch_progress wp
		JOIN videos v ON wp.video_id = v.id
		WHERE wp.duration_seconds > 0
		  AND CAST(wp.progress_seconds AS REAL) / wp.duration_seconds >= 0.1
		  AND CAST(wp.progress_seconds AS REAL) / wp.duration_seconds < 0.95
		ORDER BY wp.watched_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, nil, 0, err
	}
	defer rows.Close()

	var videos []models.Video
	progressMap := make(map[string]*WatchProgress)
	for rows.Next() {
		var v models.Video
		var isShort sql.NullBool
		var wp WatchProgress
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &isShort, &v.Published, &v.URL,
			&wp.VideoID, &wp.ProgressSeconds, &wp.DurationSeconds, &wp.WatchedAt); err != nil {
			return nil, nil, 0, err
		}
		if isShort.Valid {
			v.IsShort = &isShort.Bool
		}
		videos = append(videos, v)
		progressMap[v.ID] = &wp
	}
	return videos, progressMap, total, rows.Err()
}
```

**Step 2: Add GetHotThisWeek query**

Add this method after GetContinueWatching:

```go
// GetHotThisWeek returns unwatched videos from channels watched in the last 7 days
func (db *DB) GetHotThisWeek(limit, offset int) ([]models.Video, int, error) {
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	// Get total count first
	var total int
	err := db.conn.QueryRow(`
		SELECT COUNT(DISTINCT v.id)
		FROM videos v
		JOIN channels c ON v.channel_id = c.id
		WHERE c.id IN (
			SELECT DISTINCT v2.channel_id
			FROM watch_progress wp
			JOIN videos v2 ON wp.video_id = v2.id
			WHERE wp.watched_at >= ?
		)
		AND (v.is_short IS NULL OR v.is_short = 0)
		AND v.id NOT IN (
			SELECT video_id FROM watch_progress
			WHERE duration_seconds > 0
			  AND CAST(progress_seconds AS REAL) / duration_seconds >= 0.95
		)
	`, sevenDaysAgo).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.conn.Query(`
		SELECT v.id, v.channel_id, v.title, v.channel_name, v.thumbnail, v.duration, v.is_short, v.published, v.url
		FROM videos v
		JOIN channels c ON v.channel_id = c.id
		WHERE c.id IN (
			SELECT DISTINCT v2.channel_id
			FROM watch_progress wp
			JOIN videos v2 ON wp.video_id = v2.id
			WHERE wp.watched_at >= ?
		)
		AND (v.is_short IS NULL OR v.is_short = 0)
		AND v.id NOT IN (
			SELECT video_id FROM watch_progress
			WHERE duration_seconds > 0
			  AND CAST(progress_seconds AS REAL) / duration_seconds >= 0.95
		)
		ORDER BY v.published DESC
		LIMIT ? OFFSET ?
	`, sevenDaysAgo, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var videos []models.Video
	for rows.Next() {
		var v models.Video
		var isShort sql.NullBool
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.ChannelName, &v.Thumbnail, &v.Duration, &isShort, &v.Published, &v.URL); err != nil {
			return nil, 0, err
		}
		if isShort.Valid {
			v.IsShort = &isShort.Bool
		}
		videos = append(videos, v)
	}
	return videos, total, rows.Err()
}
```

**Step 3: Add GetSmartFeedCounts query**

Add this method for the feed list metadata:

```go
// GetSmartFeedCounts returns video counts for smart feeds
func (db *DB) GetSmartFeedCounts() (continueWatching int, hotThisWeek int, error error) {
	// Continue watching count
	err := db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM watch_progress wp
		JOIN videos v ON wp.video_id = v.id
		WHERE wp.duration_seconds > 0
		  AND CAST(wp.progress_seconds AS REAL) / wp.duration_seconds >= 0.1
		  AND CAST(wp.progress_seconds AS REAL) / wp.duration_seconds < 0.95
	`).Scan(&continueWatching)
	if err != nil {
		return 0, 0, err
	}

	// Hot this week count
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	err = db.conn.QueryRow(`
		SELECT COUNT(DISTINCT v.id)
		FROM videos v
		JOIN channels c ON v.channel_id = c.id
		WHERE c.id IN (
			SELECT DISTINCT v2.channel_id
			FROM watch_progress wp
			JOIN videos v2 ON wp.video_id = v2.id
			WHERE wp.watched_at >= ?
		)
		AND (v.is_short IS NULL OR v.is_short = 0)
		AND v.id NOT IN (
			SELECT video_id FROM watch_progress
			WHERE duration_seconds > 0
			  AND CAST(progress_seconds AS REAL) / duration_seconds >= 0.95
		)
	`, sevenDaysAgo).Scan(&hotThisWeek)
	if err != nil {
		return 0, 0, err
	}

	return continueWatching, hotThisWeek, nil
}
```

**Step 4: Verify it compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/db/db.go
git commit -m "feat(db): add queries for smart feeds

Add GetContinueWatching, GetHotThisWeek, and GetSmartFeedCounts
methods to support virtual smart feeds based on watch history.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 2: Add Smart Feeds API Handlers

**Files:**
- Modify: `internal/api/handlers.go`

**Step 1: Add handleAPIGetSmartFeeds handler**

Add this handler after the existing API handlers (around line 1842):

```go
// handleAPIGetSmartFeeds returns metadata for all smart feeds
func (s *Server) handleAPIGetSmartFeeds(w http.ResponseWriter, r *http.Request) {
	continueWatching, hotThisWeek, err := s.db.GetSmartFeedCounts()
	if err != nil {
		log.Printf("Failed to get smart feed counts: %v", err)
		// Return zeros on error rather than failing
		continueWatching, hotThisWeek = 0, 0
	}

	feeds := []map[string]any{
		{
			"slug":       "continue-watching",
			"name":       "Continue Watching",
			"icon":       "play",
			"videoCount": continueWatching,
		},
		{
			"slug":       "hot-this-week",
			"name":       "Hot This Week",
			"icon":       "flame",
			"videoCount": hotThisWeek,
		},
	}

	jsonResponse(w, map[string]any{"feeds": feeds})
}
```

**Step 2: Add handleAPIGetSmartFeed handler**

Add this handler after handleAPIGetSmartFeeds:

```go
// handleAPIGetSmartFeed returns videos for a specific smart feed
func (s *Server) handleAPIGetSmartFeed(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	switch slug {
	case "continue-watching":
		videos, progressMap, total, err := s.db.GetContinueWatching(limit, offset)
		if err != nil {
			log.Printf("Failed to get continue watching: %v", err)
			http.Error(w, "Failed to load videos", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]any{
			"slug":        slug,
			"name":        "Continue Watching",
			"videos":      videos,
			"progressMap": progressMap,
			"total":       total,
			"offset":      offset,
			"limit":       limit,
		})

	case "hot-this-week":
		videos, total, err := s.db.GetHotThisWeek(limit, offset)
		if err != nil {
			log.Printf("Failed to get hot this week: %v", err)
			http.Error(w, "Failed to load videos", http.StatusInternalServerError)
			return
		}
		// Get watch progress for videos
		videoIDs := make([]string, len(videos))
		for i, v := range videos {
			videoIDs[i] = v.ID
		}
		progressMap, _ := s.db.GetWatchProgressMap(videoIDs)

		jsonResponse(w, map[string]any{
			"slug":        slug,
			"name":        "Hot This Week",
			"videos":      videos,
			"progressMap": progressMap,
			"total":       total,
			"offset":      offset,
			"limit":       limit,
		})

	default:
		http.Error(w, "Unknown smart feed", http.StatusNotFound)
	}
}
```

**Step 3: Register the routes**

In `internal/api/handlers.go`, find the `RegisterRoutes` function and add these lines after the `/api/search` route (around line 152):

```go
	mux.HandleFunc("GET /api/smart-feeds", s.handleAPIGetSmartFeeds)
	mux.HandleFunc("GET /api/smart-feeds/{slug}", s.handleAPIGetSmartFeed)
```

**Step 4: Verify it compiles**

Run: `cd /root/code/feeds && go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/api/handlers.go
git commit -m "feat(api): add smart feeds endpoints

Add GET /api/smart-feeds for feed metadata and
GET /api/smart-feeds/{slug} for feed videos.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 3: Add Frontend API Client Functions

**Files:**
- Modify: `web/frontend/src/lib/api.ts`

**Step 1: Add SmartFeed type**

Add this type after the existing imports in `api.ts`:

```typescript
export interface SmartFeed {
	slug: string;
	name: string;
	icon: string;
	videoCount: number;
}
```

**Step 2: Add getSmartFeeds function**

Add after the existing feed functions:

```typescript
// Smart Feeds
export async function getSmartFeeds(): Promise<{ feeds: SmartFeed[] }> {
	return fetchJSON('/smart-feeds');
}

export async function getSmartFeed(slug: string, limit = 50, offset = 0): Promise<{
	slug: string;
	name: string;
	videos: Video[];
	progressMap: Record<string, WatchProgress>;
	total: number;
	offset: number;
	limit: number;
}> {
	return fetchJSON(`/smart-feeds/${slug}?limit=${limit}&offset=${offset}`);
}
```

**Step 3: Export SmartFeed type from types.ts**

Modify `web/frontend/src/lib/types.ts` to add the SmartFeed interface:

```typescript
export interface SmartFeed {
	slug: string;
	name: string;
	icon: string;
	videoCount: number;
}
```

**Step 4: Commit**

```bash
git add web/frontend/src/lib/api.ts web/frontend/src/lib/types.ts
git commit -m "feat(frontend): add smart feeds API client

Add getSmartFeeds and getSmartFeed functions with SmartFeed type.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 4: Update Home Page with Smart Feeds Section

**Files:**
- Modify: `web/frontend/src/routes/+page.svelte`

**Step 1: Import getSmartFeeds and update state**

Update the script section to add smart feeds:

```typescript
import { onMount } from 'svelte';
import { getFeeds, reorderFeeds, getSmartFeeds } from '$lib/api';
import type { Feed, SmartFeed } from '$lib/types';
import { navigationOrigin } from '$lib/stores/navigation';

let feeds = $state<Feed[]>([]);
let smartFeeds = $state<SmartFeed[]>([]);
let loading = $state(true);
let error = $state<string | null>(null);
// ... rest of existing state

onMount(async () => {
	try {
		const [feedsData, smartData] = await Promise.all([
			getFeeds(),
			getSmartFeeds()
		]);
		feeds = feedsData;
		smartFeeds = smartData.feeds || [];
	} catch (e) {
		error = e instanceof Error ? e.message : 'Failed to load feeds';
	} finally {
		loading = false;
	}
});
```

**Step 2: Add Smart Feeds section to template**

After the `{:else}` for feeds.length > 0, add the Smart Feeds section before the header:

```svelte
{:else}
	<!-- Smart Feeds Section -->
	{#if smartFeeds.length > 0}
		<div class="mb-6">
			<h2 class="text-sm font-medium text-text-muted uppercase tracking-wider mb-3">Smart Feeds</h2>
			<div class="space-y-2">
				{#each smartFeeds as smartFeed (smartFeed.slug)}
					<a
						href="/smart/{smartFeed.slug}"
						class="card flex items-center justify-between p-4 hover:bg-elevated transition-colors"
					>
						<div class="flex items-center gap-3 min-w-0">
							<!-- Smart feed icon -->
							<div class="w-10 h-10 rounded-lg bg-gradient-to-br from-emerald-500/20 to-emerald-600/20 flex items-center justify-center flex-shrink-0">
								{#if smartFeed.icon === 'play'}
									<svg class="w-5 h-5 text-emerald-500" fill="currentColor" viewBox="0 0 24 24">
										<path d="M8 5v14l11-7z"/>
									</svg>
								{:else if smartFeed.icon === 'flame'}
									<svg class="w-5 h-5 text-orange-500" fill="currentColor" viewBox="0 0 24 24">
										<path d="M12 23c-3.866 0-7-3.358-7-7.5 0-2.84 1.5-5.5 3-7.5 1.5 2 3 3 5 3s3.5-1 5-3c1.5 2 3 4.66 3 7.5 0 4.142-3.134 7.5-7 7.5zm0-5c-1.657 0-3 1.343-3 3s1.343 3 3 3 3-1.343 3-3-1.343-3-3-3z"/>
									</svg>
								{:else}
									<svg class="w-5 h-5 text-emerald-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
									</svg>
								{/if}
							</div>
							<!-- Feed name -->
							<span class="font-medium text-text-primary truncate">{smartFeed.name}</span>
							{#if smartFeed.videoCount > 0}
								<span class="px-2 py-0.5 text-xs font-medium bg-emerald-500/20 text-emerald-400 rounded-full flex-shrink-0">
									{smartFeed.videoCount}
								</span>
							{/if}
						</div>
						<svg class="w-5 h-5 text-text-muted group-hover:text-text-secondary transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
						</svg>
					</a>
				{/each}
			</div>
		</div>
	{/if}

	<!-- Header with Edit button -->
	<div class="flex items-center justify-between mb-4">
		<h1 class="text-lg font-medium text-text-primary">Your Feeds</h1>
		<!-- ... existing edit button ... -->
	</div>
```

**Step 3: Commit**

```bash
git add web/frontend/src/routes/+page.svelte
git commit -m "feat(frontend): add smart feeds section to home page

Display Smart Feeds (Continue Watching, Hot This Week) above
regular feeds with appropriate icons and video counts.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 5: Create Smart Feed View Page

**Files:**
- Create: `web/frontend/src/routes/smart/[slug]/+page.svelte`

**Step 1: Create the directory**

```bash
mkdir -p web/frontend/src/routes/smart/\[slug\]
```

**Step 2: Create the page component**

Create `web/frontend/src/routes/smart/[slug]/+page.svelte`:

```svelte
<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import { getSmartFeed } from '$lib/api';
	import type { Video, WatchProgress } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';
	import { navigationOrigin } from '$lib/stores/navigation';

	const PAGE_SIZE = 50;

	let name = $state('');
	let videos = $state<Video[]>([]);
	let progressMap = $state<Record<string, WatchProgress>>({});
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state<string | null>(null);
	let total = $state(0);

	let slug = $derived($page.params.slug);
	let hasMore = $derived(videos.length < total);

	// Empty state messages
	let emptyTitle = $derived(
		slug === 'continue-watching'
			? 'No videos in progress'
			: 'No hot channels this week'
	);
	let emptyMessage = $derived(
		slug === 'continue-watching'
			? 'Videos you start watching will appear here'
			: 'Watch some videos to see your hot channels here'
	);

	onMount(async () => {
		await loadFeed();
	});

	async function loadFeed() {
		loading = true;
		error = null;
		try {
			const data = await getSmartFeed(slug, PAGE_SIZE, 0);
			name = data.name;
			videos = data.videos || [];
			progressMap = data.progressMap || {};
			total = data.total || 0;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load feed';
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		if (loadingMore || !hasMore) return;

		loadingMore = true;
		try {
			const data = await getSmartFeed(slug, PAGE_SIZE, videos.length);
			videos = [...videos, ...(data.videos || [])];
			progressMap = { ...progressMap, ...data.progressMap };
			total = data.total || total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load more videos';
		} finally {
			loadingMore = false;
		}
	}

	// Infinite scroll
	function handleScroll() {
		if (!browser || loadingMore || !hasMore) return;

		const scrollHeight = document.documentElement.scrollHeight;
		const scrollTop = window.scrollY;
		const clientHeight = window.innerHeight;

		if (scrollHeight - scrollTop - clientHeight < 500) {
			loadMore();
		}
	}

	$effect(() => {
		if (browser && !loading) {
			window.addEventListener('scroll', handleScroll);
			return () => window.removeEventListener('scroll', handleScroll);
		}
	});

	// Set navigation origin
	$effect(() => {
		if (name) {
			navigationOrigin.setOrigin({
				feedId: 0, // Smart feeds don't have IDs
				feedName: name,
				path: `/smart/${slug}`
			});
		}
	});
</script>

<svelte:head>
	<title>{name || 'Smart Feed'} - Feeds</title>
</svelte:head>

{#if loading}
	<div class="flex flex-col items-center justify-center py-20">
		<div class="w-12 h-12 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin mb-4"></div>
		<p class="text-text-muted font-display">Loading...</p>
	</div>
{:else if error}
	<div class="empty-state animate-fade-up" style="opacity: 0;">
		<svg class="empty-state-icon mx-auto" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
			<circle cx="12" cy="12" r="10"/>
			<line x1="12" y1="8" x2="12" y2="12"/>
			<line x1="12" y1="16" x2="12.01" y2="16"/>
		</svg>
		<p class="text-crimson-400 mb-2">{error}</p>
		<button onclick={() => loadFeed()} class="btn btn-secondary btn-sm">
			Try Again
		</button>
	</div>
{:else}
	<!-- Header -->
	<div class="flex items-center justify-between mb-6">
		<div class="flex items-center gap-3">
			<a href="/" class="p-2 -ml-2 rounded-lg hover:bg-elevated transition-colors">
				<svg class="w-5 h-5 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
				</svg>
			</a>
			<div>
				<h1 class="text-xl font-semibold text-text-primary">{name}</h1>
				<p class="text-sm text-text-muted">{total} video{total === 1 ? '' : 's'}</p>
			</div>
		</div>
	</div>

	{#if videos.length === 0}
		<div class="empty-state animate-fade-up" style="opacity: 0;">
			<div class="w-20 h-20 mx-auto mb-6 rounded-2xl bg-surface flex items-center justify-center">
				{#if slug === 'continue-watching'}
					<svg class="w-10 h-10 text-text-muted" fill="currentColor" viewBox="0 0 24 24">
						<path d="M8 5v14l11-7z"/>
					</svg>
				{:else}
					<svg class="w-10 h-10 text-text-muted" fill="currentColor" viewBox="0 0 24 24">
						<path d="M12 23c-3.866 0-7-3.358-7-7.5 0-2.84 1.5-5.5 3-7.5 1.5 2 3 3 5 3s3.5-1 5-3c1.5 2 3 4.66 3 7.5 0 4.142-3.134 7.5-7 7.5zm0-5c-1.657 0-3 1.343-3 3s1.343 3 3 3 3-1.343 3-3-1.343-3-3-3z"/>
					</svg>
				{/if}
			</div>
			<h2 class="empty-state-title">{emptyTitle}</h2>
			<p class="empty-state-text">{emptyMessage}</p>
		</div>
	{:else}
		<VideoGrid {videos} {progressMap} />

		{#if loadingMore}
			<div class="flex justify-center py-8">
				<div class="w-8 h-8 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin"></div>
			</div>
		{/if}
	{/if}
{/if}
```

**Step 3: Commit**

```bash
git add web/frontend/src/routes/smart/
git commit -m "feat(frontend): add smart feed view page

Create /smart/[slug] route to display Continue Watching and
Hot This Week videos with infinite scroll and empty states.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 6: Manual Testing

**Step 1: Start the dev server**

```bash
cd /root/code/feeds && make dev
```

**Step 2: Test the API endpoints**

Open a new terminal and test:

```bash
curl http://localhost:8080/api/smart-feeds | jq
curl http://localhost:8080/api/smart-feeds/continue-watching | jq
curl http://localhost:8080/api/smart-feeds/hot-this-week | jq
```

**Step 3: Test the frontend**

1. Open http://localhost:8080 in a browser
2. Verify "Smart Feeds" section appears above "Your Feeds"
3. Click on "Continue Watching" - should show empty state or videos
4. Click on "Hot This Week" - should show empty state or videos
5. Watch a video partially (>10%) and verify it appears in Continue Watching
6. Watch videos from a channel and verify Hot This Week updates

**Step 4: Verify edge cases**

1. New user with no watch history - both smart feeds should show empty states
2. Complete a video (>95%) - should not appear in Continue Watching
3. Watch videos from multiple channels - Hot This Week should include all

---

## Task 7: Final Commit

**Step 1: Verify everything works**

Run: `cd /root/code/feeds && go build ./... && cd web/frontend && npm run check`
Expected: No errors

**Step 2: Create summary commit if needed**

If there were any fixes during testing, commit them:

```bash
git add -A
git commit -m "fix: address smart feeds testing feedback

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```
