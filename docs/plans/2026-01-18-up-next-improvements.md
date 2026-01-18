# Up Next Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve Up Next sidebar with shorts filtering, infinite scroll, and focus mode for independent scrolling.

**Architecture:** Add offset parameter to backend pagination, filter shorts in SQL query, add focus mode state and infinite scroll logic to frontend.

**Tech Stack:** Go (backend), SvelteKit/TypeScript (frontend), SQLite

---

### Task 1: Backend - Add offset and shorts filter to GetNearbyVideos

**Files:**
- Modify: `internal/db/db.go:829` (GetNearbyVideos function)

**Step 1: Update function signature and query**

Change `GetNearbyVideos` to accept offset parameter and filter shorts:

```go
// GetNearbyVideos returns videos from the same feed as the given video,
// positioned around the current video based on publish date.
// Returns up to `limit` videos that come after this video in the feed.
// Excludes shorts.
func (db *DB) GetNearbyVideos(videoID string, limit int, offset int) ([]models.Video, int64, error) {
	// First, get the video's feed and published date
	var feedID int64
	var published time.Time
	err := db.conn.QueryRow(`
		SELECT c.feed_id, v.published
		FROM videos v
		JOIN channels c ON v.channel_id = c.id
		WHERE v.id = ?
	`, videoID).Scan(&feedID, &published)
	if err != nil {
		return nil, 0, err
	}

	// Get videos from the same feed that are older than (or same as) the current video
	// excluding the current video itself and shorts, ordered by newest first
	rows, err := db.conn.Query(`
		SELECT v.id, v.channel_id, v.title, v.channel_name, v.thumbnail, v.duration, v.is_short, v.published, v.url
		FROM videos v
		JOIN channels c ON v.channel_id = c.id
		WHERE c.feed_id = ? AND v.published <= ? AND v.id != ? AND (v.is_short IS NULL OR v.is_short = 0)
		ORDER BY v.published DESC
		LIMIT ? OFFSET ?
	`, feedID, published, videoID, limit, offset)
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
	return videos, feedID, rows.Err()
}
```

**Step 2: Run backend to verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Compilation error (handler needs updating)

---

### Task 2: Backend - Update API handler for offset parameter

**Files:**
- Modify: `internal/api/handlers.go:962-973`

**Step 1: Parse offset and pass to database**

```go
// handleAPINearbyVideos returns videos from the same feed, positioned after the current video
func (s *Server) handleAPINearbyVideos(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 50 {
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

	videos, feedID, err := s.db.GetNearbyVideos(videoID, limit, offset)
```

**Step 2: Run backend to verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: SUCCESS

**Step 3: Commit backend changes**

```bash
git add internal/db/db.go internal/api/handlers.go
git commit -m "feat: add offset pagination and shorts filter to nearby videos API"
```

---

### Task 3: Frontend - Update API function signature

**Files:**
- Modify: `web/frontend/src/lib/api.ts:139-145`

**Step 1: Add offset parameter**

```typescript
export async function getNearbyVideos(id: string, limit = 20, offset = 0): Promise<{
	videos: Video[];
	feedId: number;
	progressMap: Record<string, WatchProgress>;
}> {
	return fetchJSON(`/videos/${id}/nearby?limit=${limit}&offset=${offset}`);
}
```

**Step 2: Verify no TypeScript errors**

Run: `cd /root/code/feeds/web/frontend && npm run check`
Expected: SUCCESS (existing call uses default offset=0)

---

### Task 4: Frontend - Add focus mode state and infinite scroll logic

**Files:**
- Modify: `web/frontend/src/routes/watch/[id]/+page.svelte`

**Step 1: Add new state variables after line 26**

After `let nearbyFeedId = $state(0);` add:

```typescript
// Up Next focus mode and infinite scroll
let upNextFocusMode = $state(false);
let upNextOffset = $state(0);
let upNextLoading = $state(false);
let upNextHasMore = $state(true);
let upNextContainer: HTMLDivElement | null = null;
```

**Step 2: Add loadMoreNearbyVideos function**

Add after the `loadVideo` function (around line 248):

```typescript
async function loadMoreNearbyVideos() {
	if (upNextLoading || !upNextHasMore) return;

	upNextLoading = true;
	try {
		const newOffset = upNextOffset + 20;
		const nearby = await getNearbyVideos(videoId, 20, newOffset);
		if (nearby.videos.length === 0) {
			upNextHasMore = false;
		} else {
			nearbyVideos = [...nearbyVideos, ...nearby.videos];
			nearbyProgressMap = { ...nearbyProgressMap, ...nearby.progressMap };
			upNextOffset = newOffset;
			if (nearby.videos.length < 20) {
				upNextHasMore = false;
			}
		}
	} catch (e) {
		console.warn('Failed to load more nearby videos:', e);
	} finally {
		upNextLoading = false;
	}
}
```

**Step 3: Add scroll handler for focus mode and infinite scroll**

Add after `loadMoreNearbyVideos`:

```typescript
function handleUpNextScroll(e: Event) {
	const container = e.target as HTMLDivElement;

	// Enter focus mode on first scroll
	if (!upNextFocusMode) {
		upNextFocusMode = true;
	}

	// Check if near bottom for infinite scroll
	const scrollBottom = container.scrollHeight - container.scrollTop - container.clientHeight;
	if (scrollBottom < 200) {
		loadMoreNearbyVideos();
	}
}

function exitFocusMode() {
	upNextFocusMode = false;
}
```

**Step 4: Reset focus mode state when video changes**

In `loadVideo` function, after `nearbyFeedId = nearby.feedId;` (around line 237), add:

```typescript
upNextOffset = 0;
upNextHasMore = true;
upNextFocusMode = false;
```

---

### Task 5: Frontend - Update desktop sidebar HTML with focus mode

**Files:**
- Modify: `web/frontend/src/routes/watch/[id]/+page.svelte:808-847`

**Step 1: Replace the desktop sidebar section**

Replace lines 808-847 with:

```svelte
<!-- Desktop Sidebar - Up Next -->
{#if nearbyVideos.length > 0}
	<aside
		class="hidden lg:block animate-fade-up stagger-2 transition-all duration-300"
		class:up-next-focus-mode={upNextFocusMode}
		style="opacity: 0;"
	>
		<div class="sticky top-20" class:up-next-focus-sticky={upNextFocusMode}>
			<div class="flex items-center justify-between mb-4">
				<h2 class="font-display font-semibold">Up Next</h2>
				<div class="flex items-center gap-3">
					{#if upNextFocusMode}
						<button
							onclick={exitFocusMode}
							class="text-sm text-text-muted hover:text-white transition-colors"
						>
							Exit Focus
						</button>
					{/if}
					{#if nearbyFeedId > 0}
						<a href="/feeds/{nearbyFeedId}" class="text-sm text-emerald-400 hover:text-emerald-300 transition-colors">
							View Feed
						</a>
					{/if}
				</div>
			</div>
			<div
				class="up-next-sidebar space-y-2 pr-2"
				class:up-next-sidebar-focus={upNextFocusMode}
				onscroll={handleUpNextScroll}
				bind:this={upNextContainer}
			>
				{#each nearbyVideos as video}
					<a href="/watch/{video.id}" class="up-next-item group">
						<div class="video-thumbnail w-36 flex-shrink-0">
							{#if video.thumbnail}
								<img src={video.thumbnail} alt="" />
							{/if}
							{#if video.duration > 0}
								<span class="duration-badge">{formatDuration(video.duration)}</span>
							{/if}
							{#if getWatchedPercent(video) > 0}
								<div class="watch-progress">
									<div class="watch-progress-fill" style="width: {getWatchedPercent(video)}%"></div>
								</div>
							{/if}
						</div>
						<div class="flex-1 min-w-0">
							<h3 class="text-sm font-medium line-clamp-2 group-hover:text-emerald-400 transition-colors">
								{video.title}
							</h3>
							<p class="text-xs text-text-muted mt-1">{video.channel_name}</p>
						</div>
					</a>
				{/each}
				{#if upNextLoading}
					<div class="flex justify-center py-4">
						<div class="animate-spin rounded-full h-6 w-6 border-2 border-emerald-500 border-t-transparent"></div>
					</div>
				{/if}
				{#if !upNextHasMore && nearbyVideos.length > 0}
					<p class="text-center text-text-muted text-sm py-4">No more videos</p>
				{/if}
			</div>
		</div>
	</aside>
{/if}
```

**Step 2: Add click handler to video player for exiting focus mode**

Find the video element container (around line 430) and add onclick:

```svelte
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="video-container" onclick={exitFocusMode}>
```

---

### Task 6: Frontend - Add focus mode CSS

**Files:**
- Modify: `web/frontend/src/app.css` (after line 788)

**Step 1: Add focus mode styles**

```css
/* Up Next Focus Mode */
.up-next-focus-mode {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: 400px;
  background: var(--bg-primary);
  z-index: 40;
  padding: 1rem;
  border-left: 1px solid var(--border-subtle);
}

.up-next-focus-sticky {
  position: static;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.up-next-sidebar-focus {
  max-height: none;
  flex: 1;
  overflow-y: auto;
}
```

---

### Task 7: Frontend - Update main layout for focus mode

**Files:**
- Modify: `web/frontend/src/routes/watch/[id]/+page.svelte`

**Step 1: Add focus mode class to main container**

Find the main content wrapper (around line 420) and add conditional class:

```svelte
<div class="flex-1 max-w-4xl" class:video-focus-mode={upNextFocusMode}>
```

**Step 2: Hide description when in focus mode**

Wrap the description/comments section (everything after the video player inside the left column, before the mobile Up Next section) with:

```svelte
{#if !upNextFocusMode}
  <!-- existing description/comments content -->
{/if}
```

**Step 3: Add video focus mode CSS to app.css**

```css
.video-focus-mode {
  position: fixed;
  top: 5rem;
  left: 1rem;
  right: 420px;
  z-index: 30;
}
```

---

### Task 8: Test and commit frontend changes

**Step 1: Run TypeScript check**

Run: `cd /root/code/feeds/web/frontend && npm run check`
Expected: SUCCESS

**Step 2: Test in browser**

Run: `make dev`
Test:
1. Navigate to a watch page
2. Scroll in Up Next sidebar → should enter focus mode
3. Video should stay fixed, description hidden
4. Scroll to bottom → should load more videos
5. Click video player → should exit focus mode
6. Verify no shorts appear in Up Next

**Step 3: Commit all changes**

```bash
git add -A
git commit -m "feat: add up next focus mode with infinite scroll and shorts filter"
```
