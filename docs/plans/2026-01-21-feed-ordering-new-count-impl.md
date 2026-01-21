# Feed Ordering and New Video Counts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add manual drag-and-drop feed ordering and show new video count badges from last refresh.

**Architecture:** Add `sort_order` and `new_video_count` columns to feeds table. New reorder API endpoint. Frontend uses HTML5 drag-and-drop to reorder, displays badge when new_video_count > 0.

**Tech Stack:** Go/SQLite backend, SvelteKit frontend with Svelte 5 runes

---

### Task 1: Database Migration

**Files:**
- Create: `internal/db/migrations/003_feed_ordering.sql`

**Step 1: Create migration file**

```sql
-- +goose Up

-- Add sort_order column (lower = higher in list)
ALTER TABLE feeds ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

-- Add new_video_count column (videos added in last refresh)
ALTER TABLE feeds ADD COLUMN new_video_count INTEGER NOT NULL DEFAULT 0;

-- Initialize sort_order based on current display order (is_system DESC, name ASC)
-- This preserves existing order for users
WITH ordered_feeds AS (
    SELECT id, ROW_NUMBER() OVER (ORDER BY is_system DESC, name ASC) - 1 AS new_order
    FROM feeds
)
UPDATE feeds SET sort_order = (
    SELECT new_order FROM ordered_feeds WHERE ordered_feeds.id = feeds.id
);

-- +goose Down

-- SQLite doesn't support DROP COLUMN easily, but goose can handle it
-- For simplicity, we'll just ignore these columns if rolling back
```

**Step 2: Verify migration applies**

Run: `cd /root/code/feeds && go run ./cmd/feeds`
Expected: Server starts without migration errors

**Step 3: Commit**

```bash
git add internal/db/migrations/003_feed_ordering.sql
git commit -m "feat(db): add sort_order and new_video_count columns to feeds"
```

---

### Task 2: Update Go Models

**Files:**
- Modify: `internal/models/models.go:5-14`

**Step 1: Add fields to Feed struct**

Change the Feed struct from:

```go
type Feed struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Author      string    `json:"author,omitempty"`
	Tags        string    `json:"tags,omitempty"` // comma-separated
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
```

To:

```go
type Feed struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Author        string    `json:"author,omitempty"`
	Tags          string    `json:"tags,omitempty"` // comma-separated
	IsSystem      bool      `json:"is_system"`
	SortOrder     int       `json:"sort_order"`
	NewVideoCount int       `json:"new_video_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
```

**Step 2: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Build succeeds (may have scan errors until DB layer updated)

**Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "feat(models): add SortOrder and NewVideoCount to Feed"
```

---

### Task 3: Update Database Layer - Feed Queries

**Files:**
- Modify: `internal/db/db.go`

**Step 1: Update GetFeeds to return new fields and use sort_order**

Find `GetFeeds` function (around line 151) and update:

```go
func (db *DB) GetFeeds() ([]models.Feed, error) {
	rows, err := db.conn.Query("SELECT id, name, description, author, tags, is_system, sort_order, new_video_count, created_at, updated_at FROM feeds ORDER BY sort_order ASC, name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []models.Feed
	for rows.Next() {
		var f models.Feed
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}
```

**Step 2: Update GetFeed to return new fields**

Find `GetFeed` function (around line 169) and update:

```go
func (db *DB) GetFeed(id int64) (*models.Feed, error) {
	var f models.Feed
	err := db.conn.QueryRow(
		"SELECT id, name, description, author, tags, is_system, sort_order, new_video_count, created_at, updated_at FROM feeds WHERE id = ?", id,
	).Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
```

**Step 3: Update GetFeedsByChannel to return new fields**

Find `GetFeedsByChannel` function (around line 371) and update:

```go
func (db *DB) GetFeedsByChannel(channelID int64) ([]models.Feed, error) {
	rows, err := db.conn.Query(`
		SELECT f.id, f.name, f.description, f.author, f.tags, f.is_system, f.sort_order, f.new_video_count, f.created_at, f.updated_at
		FROM feeds f
		JOIN feed_channels fc ON f.id = fc.feed_id
		WHERE fc.channel_id = ?
		ORDER BY f.sort_order ASC, f.name ASC
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []models.Feed
	for rows.Next() {
		var f models.Feed
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}
```

**Step 4: Update EnsureInboxExists to scan new fields**

Find `EnsureInboxExists` function (around line 70) and update the QueryRow scan:

```go
func (db *DB) EnsureInboxExists() (*models.Feed, error) {
	// Check if Inbox already exists
	var f models.Feed
	err := db.conn.QueryRow(
		"SELECT id, name, description, author, tags, is_system, sort_order, new_video_count, created_at, updated_at FROM feeds WHERE is_system = TRUE AND name = 'Inbox'",
	).Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt)
	if err == nil {
		return &f, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create Inbox - get max sort_order first
	var maxOrder int
	db.conn.QueryRow("SELECT COALESCE(MAX(sort_order), -1) FROM feeds").Scan(&maxOrder)

	now := time.Now()
	result, err := db.conn.Exec(
		"INSERT INTO feeds (name, description, author, tags, is_system, sort_order, created_at, updated_at) VALUES ('Inbox', '', '', '', TRUE, ?, ?, ?)",
		maxOrder+1, now, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.Feed{
		ID:        id,
		Name:      "Inbox",
		IsSystem:  true,
		SortOrder: maxOrder + 1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
```

**Step 5: Update GetOrCreateFeed to scan new fields**

Find `GetOrCreateFeed` function (around line 211) and update:

```go
func (db *DB) GetOrCreateFeed(name string) (*models.Feed, error) {
	var f models.Feed
	err := db.conn.QueryRow(
		"SELECT id, name, description, author, tags, is_system, sort_order, new_video_count, created_at, updated_at FROM feeds WHERE name = ?", name,
	).Scan(&f.ID, &f.Name, &f.Description, &f.Author, &f.Tags, &f.IsSystem, &f.SortOrder, &f.NewVideoCount, &f.CreatedAt, &f.UpdatedAt)
	if err == nil {
		return &f, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	// Feed doesn't exist, create it
	return db.CreateFeed(name)
}
```

**Step 6: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Build succeeds

**Step 7: Commit**

```bash
git add internal/db/db.go
git commit -m "feat(db): update feed queries to include sort_order and new_video_count"
```

---

### Task 4: Update Database Layer - CreateFeed Functions

**Files:**
- Modify: `internal/db/db.go`

**Step 1: Update CreateFeedWithMetadata to set sort_order**

Find `CreateFeedWithMetadata` function (around line 125) and update:

```go
func (db *DB) CreateFeedWithMetadata(name, description, author, tags string) (*models.Feed, error) {
	// Get max sort_order to put new feed at end
	var maxOrder int
	db.conn.QueryRow("SELECT COALESCE(MAX(sort_order), -1) FROM feeds").Scan(&maxOrder)

	now := time.Now()
	result, err := db.conn.Exec(
		"INSERT INTO feeds (name, description, author, tags, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		name, description, author, tags, maxOrder+1, now, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.Feed{
		ID:          id,
		Name:        name,
		Description: description,
		Author:      author,
		Tags:        tags,
		SortOrder:   maxOrder + 1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
```

**Step 2: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/db/db.go
git commit -m "feat(db): CreateFeedWithMetadata assigns sort_order at end of list"
```

---

### Task 5: Add ReorderFeeds Database Function

**Files:**
- Modify: `internal/db/db.go`

**Step 1: Add ReorderFeeds function**

Add this function after `GetFeedsByChannel` (around line 393):

```go
// ReorderFeeds updates sort_order for feeds based on the provided order.
// feedIDs should contain all feed IDs in the desired display order.
func (db *DB) ReorderFeeds(feedIDs []int64) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE feeds SET sort_order = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, id := range feedIDs {
		if _, err := stmt.Exec(i, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}
```

**Step 2: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/db/db.go
git commit -m "feat(db): add ReorderFeeds function"
```

---

### Task 6: Add UpdateNewVideoCount Database Function

**Files:**
- Modify: `internal/db/db.go`

**Step 1: Add UpdateNewVideoCount function**

Add this function after `ReorderFeeds`:

```go
// UpdateNewVideoCount sets the new_video_count for a feed
func (db *DB) UpdateNewVideoCount(feedID int64, count int) error {
	_, err := db.conn.Exec("UPDATE feeds SET new_video_count = ? WHERE id = ?", count, feedID)
	return err
}
```

**Step 2: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/db/db.go
git commit -m "feat(db): add UpdateNewVideoCount function"
```

---

### Task 7: Modify UpsertVideo to Return Insert Status

**Files:**
- Modify: `internal/db/db.go`

**Step 1: Update UpsertVideo to return whether it was an insert**

Find `UpsertVideo` function (around line 443) and change return type:

```go
// UpsertVideo inserts or updates a video. Returns true if this was a new insert.
func (db *DB) UpsertVideo(v *models.Video) (bool, error) {
	var isShort *int
	if v.IsShort != nil {
		val := 0
		if *v.IsShort {
			val = 1
		}
		isShort = &val
	}

	// Check if video exists first
	var exists bool
	err := db.conn.QueryRow("SELECT 1 FROM videos WHERE id = ?", v.ID).Scan(&exists)
	isInsert := err == sql.ErrNoRows

	_, err = db.conn.Exec(`
		INSERT INTO videos (id, channel_id, title, channel_name, thumbnail, duration, is_short, published, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			channel_id = excluded.channel_id,
			title = excluded.title,
			thumbnail = excluded.thumbnail,
			duration = CASE
				WHEN excluded.duration > 0 THEN excluded.duration
				ELSE videos.duration
			END,
			is_short = CASE
				WHEN excluded.is_short IS NOT NULL THEN excluded.is_short
				ELSE videos.is_short
			END
	`, v.ID, v.ChannelID, v.Title, v.ChannelName, v.Thumbnail, v.Duration, isShort, v.Published, v.URL)
	return isInsert, err
}
```

**Step 2: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Build fails - callers need to be updated (expected)

**Step 3: Commit (partial)**

```bash
git add internal/db/db.go
git commit -m "feat(db): UpsertVideo returns whether video was newly inserted"
```

---

### Task 8: Update UpsertVideo Callers

**Files:**
- Modify: `internal/api/api_handlers.go`

**Step 1: Find and update all UpsertVideo calls**

In `handleAPIRefreshFeed` (around line 376), change:

```go
if err := s.db.UpsertVideo(&allVideos[i]); err != nil {
```

To:

```go
if _, err := s.db.UpsertVideo(&allVideos[i]); err != nil {
```

Search for any other UpsertVideo calls in the codebase and update them similarly.

**Step 2: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/api/api_handlers.go
git commit -m "fix(api): update UpsertVideo callers for new return type"
```

---

### Task 9: Track New Video Count in Refresh Handler

**Files:**
- Modify: `internal/api/api_handlers.go`

**Step 1: Update handleAPIRefreshFeed to track inserts and update new_video_count**

In `handleAPIRefreshFeed` (starting around line 267), modify the video upsert loop and add the count update:

Find this section (around line 372-382):

```go
	for i := range allVideos {
		if isShort, ok := newShortsStatus[allVideos[i].ID]; ok {
			allVideos[i].IsShort = &isShort
		}
		if _, err := s.db.UpsertVideo(&allVideos[i]); err != nil {
			log.Printf("Failed to save video %s: %v", allVideos[i].ID, err)
			continue
		}
		totalVideos++
	}
```

Replace with:

```go
	var newVideos int
	for i := range allVideos {
		if isShort, ok := newShortsStatus[allVideos[i].ID]; ok {
			allVideos[i].IsShort = &isShort
		}
		isNew, err := s.db.UpsertVideo(&allVideos[i])
		if err != nil {
			log.Printf("Failed to save video %s: %v", allVideos[i].ID, err)
			continue
		}
		totalVideos++
		if isNew {
			newVideos++
		}
	}

	// Update new video count for this feed
	if err := s.db.UpdateNewVideoCount(id, newVideos); err != nil {
		log.Printf("Failed to update new video count for feed %d: %v", id, err)
	}
```

Also update the response to include newVideos:

```go
	jsonResponse(w, map[string]any{
		"videosFound": totalVideos,
		"newVideos":   newVideos,
		"channels":    len(channels),
		"errors":      errors,
	})
```

**Step 2: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/api/api_handlers.go
git commit -m "feat(api): track and store new video count on feed refresh"
```

---

### Task 10: Add Reorder API Endpoint

**Files:**
- Modify: `internal/api/api_handlers.go`
- Modify: `internal/api/handlers.go`

**Step 1: Add handler function in api_handlers.go**

Add this function (can go after `handleAPIDeleteFeed`):

```go
func (s *Server) handleAPIReorderFeeds(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeedIDs []int64 `json:"feed_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.FeedIDs) == 0 {
		jsonError(w, "feed_ids is required", http.StatusBadRequest)
		return
	}

	if err := s.db.ReorderFeeds(req.FeedIDs); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated feeds list
	feeds, err := s.db.GetFeeds()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, feeds)
}
```

**Step 2: Register the route in handlers.go**

Find the API routes section (around line 106) and add after `DELETE /api/feeds/{id}`:

```go
	mux.HandleFunc("PUT /api/feeds/reorder", s.handleAPIReorderFeeds)
```

**Step 3: Verify compilation**

Run: `cd /root/code/feeds && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/api/api_handlers.go internal/api/handlers.go
git commit -m "feat(api): add PUT /api/feeds/reorder endpoint"
```

---

### Task 11: Update Frontend Types

**Files:**
- Modify: `web/frontend/src/lib/types.ts`

**Step 1: Add new fields to Feed interface**

Update the Feed interface:

```typescript
export interface Feed {
	id: number;
	name: string;
	description?: string;
	author?: string;
	tags?: string;
	is_system?: boolean;
	sort_order: number;
	new_video_count: number;
	created_at: string;
	updated_at: string;
}
```

**Step 2: Commit**

```bash
git add web/frontend/src/lib/types.ts
git commit -m "feat(frontend): add sort_order and new_video_count to Feed type"
```

---

### Task 12: Add Frontend API Function

**Files:**
- Modify: `web/frontend/src/lib/api.ts`

**Step 1: Add reorderFeeds function**

Add after `deleteFeed` function:

```typescript
export async function reorderFeeds(feedIds: number[]): Promise<Feed[]> {
	return fetchJSON('/feeds/reorder', {
		method: 'PUT',
		body: JSON.stringify({ feed_ids: feedIds })
	});
}
```

**Step 2: Update refreshFeed return type to include newVideos**

Update the `refreshFeed` function signature:

```typescript
export async function refreshFeed(id: number): Promise<{
	videosFound: number;
	newVideos: number;
	channels: number;
	errors: string[];
}> {
	return fetchJSON(`/feeds/${id}/refresh`, { method: 'POST' });
}
```

**Step 3: Commit**

```bash
git add web/frontend/src/lib/api.ts
git commit -m "feat(frontend): add reorderFeeds API function"
```

---

### Task 13: Update Home Page with Drag-and-Drop and Badges

**Files:**
- Modify: `web/frontend/src/routes/+page.svelte`

**Step 1: Replace the entire file with new implementation**

```svelte
<script lang="ts">
	import { onMount } from 'svelte';
	import { getFeeds, reorderFeeds } from '$lib/api';
	import type { Feed } from '$lib/types';
	import { navigationOrigin } from '$lib/stores/navigation';

	let feeds = $state<Feed[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Drag state
	let draggedIndex = $state<number | null>(null);
	let dragOverIndex = $state<number | null>(null);

	onMount(async () => {
		try {
			feeds = await getFeeds();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load feeds';
		} finally {
			loading = false;
		}
	});

	// Clear navigation origin when returning to home
	$effect(() => {
		navigationOrigin.clear();
	});

	function handleDragStart(index: number) {
		draggedIndex = index;
	}

	function handleDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		dragOverIndex = index;
	}

	function handleDragLeave() {
		dragOverIndex = null;
	}

	async function handleDrop(index: number) {
		if (draggedIndex === null || draggedIndex === index) {
			draggedIndex = null;
			dragOverIndex = null;
			return;
		}

		// Reorder locally first (optimistic update)
		const newFeeds = [...feeds];
		const [moved] = newFeeds.splice(draggedIndex, 1);
		newFeeds.splice(index, 0, moved);
		feeds = newFeeds;

		draggedIndex = null;
		dragOverIndex = null;

		// Persist to backend
		try {
			const feedIds = newFeeds.map(f => f.id);
			await reorderFeeds(feedIds);
		} catch (e) {
			// Revert on error by re-fetching
			error = e instanceof Error ? e.message : 'Failed to reorder feeds';
			feeds = await getFeeds();
		}
	}

	function handleDragEnd() {
		draggedIndex = null;
		dragOverIndex = null;
	}
</script>

<svelte:head>
	<title>Home - Feeds</title>
</svelte:head>

{#if loading}
	<!-- Loading State -->
	<div class="flex flex-col items-center justify-center py-20">
		<div class="w-12 h-12 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin mb-4"></div>
		<p class="text-text-muted font-display">Loading your feeds...</p>
	</div>
{:else if error}
	<!-- Error State -->
	<div class="empty-state animate-fade-up" style="opacity: 0;">
		<svg class="empty-state-icon mx-auto" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
			<circle cx="12" cy="12" r="10"/>
			<line x1="12" y1="8" x2="12" y2="12"/>
			<line x1="12" y1="16" x2="12.01" y2="16"/>
		</svg>
		<p class="text-crimson-400 mb-2">{error}</p>
		<button onclick={() => location.reload()} class="btn btn-secondary btn-sm">
			Try Again
		</button>
	</div>
{:else if feeds.length === 0}
	<!-- Empty State -->
	<div class="empty-state animate-fade-up" style="opacity: 0;">
		<div class="w-20 h-20 mx-auto mb-6 rounded-2xl bg-surface flex items-center justify-center">
			<svg class="w-10 h-10 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
				<rect x="3" y="3" width="18" height="18" rx="2"/>
				<path d="M9 9h6v6H9z"/>
			</svg>
		</div>
		<h2 class="empty-state-title">No feeds yet</h2>
		<p class="empty-state-text mb-6">Import your subscriptions to get started</p>
		<a href="/import" class="btn btn-primary">
			<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
				<line x1="12" y1="5" x2="12" y2="19"/>
				<line x1="5" y1="12" x2="19" y2="12"/>
			</svg>
			Import Feed
		</a>
	</div>
{:else}
	<div class="space-y-2">
		{#each feeds as feed, index (feed.id)}
			<div
				draggable="true"
				ondragstart={() => handleDragStart(index)}
				ondragover={(e) => handleDragOver(e, index)}
				ondragleave={handleDragLeave}
				ondrop={() => handleDrop(index)}
				ondragend={handleDragEnd}
				class="group"
				class:opacity-50={draggedIndex === index}
			>
				{#if dragOverIndex === index && draggedIndex !== null && draggedIndex !== index}
					<div class="h-1 bg-emerald-500 rounded-full mb-2 transition-all"></div>
				{/if}
				<a
					href="/feeds/{feed.id}"
					class="card flex items-center justify-between p-4 hover:bg-elevated transition-colors"
				>
					<div class="flex items-center gap-3 min-w-0">
						<!-- Drag handle -->
						<div class="w-6 h-6 flex items-center justify-center text-text-muted cursor-grab active:cursor-grabbing flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
							<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
								<circle cx="9" cy="6" r="1.5"/>
								<circle cx="15" cy="6" r="1.5"/>
								<circle cx="9" cy="12" r="1.5"/>
								<circle cx="15" cy="12" r="1.5"/>
								<circle cx="9" cy="18" r="1.5"/>
								<circle cx="15" cy="18" r="1.5"/>
							</svg>
						</div>
						<!-- Feed icon -->
						<div class="w-10 h-10 rounded-lg bg-gradient-to-br {feed.is_system ? 'from-amber-500/20 to-amber-600/20' : 'from-violet-500/20 to-violet-600/20'} flex items-center justify-center flex-shrink-0">
							<svg class="w-5 h-5 {feed.is_system ? 'text-amber-500' : 'text-violet-500'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
							</svg>
						</div>
						<!-- Feed name and badge -->
						<span class="font-medium text-text-primary truncate">{feed.name}</span>
						{#if feed.new_video_count > 0}
							<span class="px-2 py-0.5 text-xs font-medium bg-emerald-500/20 text-emerald-400 rounded-full flex-shrink-0">
								{feed.new_video_count}
							</span>
						{/if}
					</div>
					<svg class="w-5 h-5 text-text-muted group-hover:text-text-secondary transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
					</svg>
				</a>
			</div>
		{/each}

		<!-- New Feed Button -->
		<div class="flex justify-center pt-6">
			<a
				href="/feeds/new"
				class="btn btn-primary flex items-center gap-2"
			>
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				<span>New Feed</span>
			</a>
		</div>
	</div>
{/if}
```

**Step 2: Verify frontend builds**

Run: `cd /root/code/feeds/web/frontend && npm run build`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add web/frontend/src/routes/+page.svelte
git commit -m "feat(frontend): add drag-and-drop reordering and new video badges to home page"
```

---

### Task 14: End-to-End Testing

**Step 1: Start the application**

Run: `cd /root/code/feeds && make dev`

**Step 2: Manual verification checklist**

1. Open browser to the app
2. Verify feeds display in order (should preserve previous alphabetical order initially)
3. Drag a feed to a new position - verify it stays in new position after refresh
4. Refresh a feed with channels - verify badge shows count of newly fetched videos
5. Refresh page - verify badge persists
6. Refresh same feed again - verify badge updates to new count (possibly 0)

**Step 3: Commit any fixes if needed**

---

### Task 15: Final Commit

**Step 1: Verify all changes are committed**

Run: `git status`
Expected: Clean working tree

**Step 2: Create summary commit if there are uncommitted changes**

If clean, done. Otherwise:

```bash
git add -A
git commit -m "feat: feed ordering and new video count badges"
```
