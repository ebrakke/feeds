# YouTube Channel Import from Video URLs - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable users to paste YouTube video or channel URLs into the `/import` page and add the channel to a selected feed.

**Architecture:** Extend backend with video-to-channel resolution in `youtube/rss.go`, add new `/api/import/youtube` endpoint in `api_handlers.go`, and enhance frontend `/import` page with URL detection and feed selector modal.

**Tech Stack:** Go (backend), SvelteKit (frontend), YouTube HTML scraping (no official API)

---

## Task 1: Backend - Add Video URL Resolution Function

**Files:**
- Modify: `internal/youtube/rss.go` (add new function after `ResolveChannelURL`)

**Step 1: Add the ResolveVideoToChannel function**

Add this function after line 126 (after `ResolveChannelURL`):

```go
// ResolveVideoToChannel extracts channel information from a YouTube video URL
// Supports: /watch?v=ID, youtu.be/ID, /shorts/ID
func ResolveVideoToChannel(videoURL string) (*ChannelInfo, error) {
	// Normalize the URL
	videoURL = strings.TrimSpace(videoURL)

	// Extract video ID using existing regex
	matches := videoIDRegex.FindStringSubmatch(videoURL)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not extract video ID from URL: %s", videoURL)
	}
	videoID := matches[1]

	// Construct canonical watch URL
	watchURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	// Fetch video page
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(watchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch video page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("video page returned status %d", resp.StatusCode)
	}

	// Read body and look for channel ID
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	body := string(bodyBytes)

	// Look for channel ID in the HTML
	patterns := []string{
		`"channelId":"([^"]+)"`,
		`"externalChannelId":"([^"]+)"`,
		`/channel/([^"/?]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(body); len(matches) > 1 {
			return fetchChannelInfoByID(matches[1])
		}
	}

	return nil, fmt.Errorf("could not find channel ID for video: %s", videoURL)
}
```

**Step 2: Test the function manually (no automated tests yet)**

Build and verify it compiles:

```bash
cd /root/code/feeds
go build ./...
```

Expected: No compilation errors

**Step 3: Commit**

```bash
git add internal/youtube/rss.go
git commit -m "feat(youtube): add ResolveVideoToChannel function

Extracts channel information from YouTube video URLs by:
- Parsing video ID from URL patterns (watch, youtu.be, shorts)
- Fetching video page HTML
- Extracting channel ID from page metadata
- Returning ChannelInfo via existing fetchChannelInfoByID

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 2: Backend - Add YouTube Import API Endpoint

**Files:**
- Modify: `internal/api/api_handlers.go` (add new function before line 950, before "Import endpoints" comment)

**Step 1: Add helper function to detect video vs channel URL**

Add this helper function around line 948 (right before the "Import endpoints" comment):

```go
// isVideoURL checks if a URL is a YouTube video URL vs a channel URL
func isVideoURL(url string) bool {
	return strings.Contains(url, "/watch?v=") ||
		strings.Contains(url, "youtu.be/") ||
		strings.Contains(url, "/shorts/")
}
```

**Step 2: Add the handleAPIImportYouTube endpoint handler**

Add this function after the helper (around line 955):

```go
func (s *Server) handleAPIImportYouTube(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		FeedID int64  `json:"feedId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate inputs
	if strings.TrimSpace(req.URL) == "" {
		jsonError(w, "URL is required", http.StatusBadRequest)
		return
	}
	if req.FeedID == 0 {
		jsonError(w, "feedId is required", http.StatusBadRequest)
		return
	}

	// Verify feed exists
	feed, err := s.db.GetFeed(req.FeedID)
	if err != nil {
		jsonError(w, "Feed not found", http.StatusBadRequest)
		return
	}

	// Resolve to channel (detect video vs channel URL)
	var channelInfo *yt.ChannelInfo
	if isVideoURL(req.URL) {
		channelInfo, err = yt.ResolveVideoToChannel(req.URL)
		if err != nil {
			jsonError(w, "Could not resolve channel from video URL: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		channelInfo, err = yt.ResolveChannelURL(req.URL)
		if err != nil {
			jsonError(w, "Could not resolve channel from URL: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Add channel to feed
	channel, isNew, err := s.db.AddChannelToFeed(req.FeedID, channelInfo.URL, channelInfo.Name)
	if err != nil {
		jsonError(w, "Failed to add channel: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If new channel, fetch initial videos
	if isNew {
		videos, err := yt.FetchLatestVideos(channel.URL, 5)
		if err != nil {
			log.Printf("Failed to fetch initial videos for channel %s: %v", channel.URL, err)
		} else {
			// Check shorts status
			var videoIDs []string
			for _, v := range videos {
				videoIDs = append(videoIDs, v.ID)
			}
			shortsMap := yt.CheckShortsStatus(videoIDs)

			// Upsert videos
			for _, video := range videos {
				video.IsShort = shortsMap[video.ID]
				if err := s.db.UpsertVideo(&video); err != nil {
					log.Printf("Failed to upsert video %s: %v", video.ID, err)
				}
			}
		}
	}

	// Return channel and feed info
	w.WriteHeader(http.StatusCreated)
	jsonResponse(w, map[string]any{
		"channel": channel,
		"feed":    feed,
	})
}
```

**Step 3: Register the route in RegisterRoutes**

Modify: `internal/api/handlers.go` at line 144 (after `handleAPIImportWatchHistory` route)

Add this line:

```go
mux.HandleFunc("POST /api/import/youtube", s.handleAPIImportYouTube)
```

Insert it after line 144, so it becomes:

```go
	mux.HandleFunc("POST /api/import/watch-history", s.handleAPIImportWatchHistory)
	mux.HandleFunc("POST /api/import/youtube", s.handleAPIImportYouTube)
```

**Step 4: Test the endpoint compiles**

```bash
cd /root/code/feeds
go build ./...
```

Expected: No compilation errors

**Step 5: Commit**

```bash
git add internal/api/api_handlers.go internal/api/handlers.go
git commit -m "feat(api): add YouTube import endpoint

Add POST /api/import/youtube endpoint that:
- Accepts YouTube channel or video URLs
- Detects URL type (video vs channel)
- Resolves to channel using appropriate function
- Adds channel to specified feed
- Fetches initial 5 videos for new channels
- Returns channel and feed info

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 3: Frontend - Add API Function

**Files:**
- Modify: `web/frontend/src/lib/api.ts` (add function around line 189, after fetchMoreChannelVideos)

**Step 1: Add importYouTubeChannel function**

Add this after the `fetchMoreChannelVideos` function (around line 187):

```typescript
// Import
export async function importYouTubeChannel(url: string, feedId: number): Promise<{
	channel: Channel;
	feed: Feed;
}> {
	return fetchJSON('/import/youtube', {
		method: 'POST',
		body: JSON.stringify({ url, feedId })
	});
}
```

**Step 2: Verify TypeScript compiles**

```bash
cd /root/code/feeds/web/frontend
npm run check
```

Expected: No type errors

**Step 3: Commit**

```bash
git add web/frontend/src/lib/api.ts
git commit -m "feat(frontend): add importYouTubeChannel API function

Add API client function for YouTube import endpoint.
Takes URL and feedId, returns channel and feed objects.

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 4: Frontend - Add Feed Selector Component

**Files:**
- Create: `web/frontend/src/lib/components/FeedSelector.svelte`

**Step 1: Create the FeedSelector component**

```svelte
<script lang="ts">
	import type { Feed } from '$lib/types';
	import { createFeed } from '$lib/api';

	interface Props {
		feeds: Feed[];
		onSelect: (feedId: number) => void;
		onCancel: () => void;
		loading?: boolean;
		error?: string | null;
	}

	let { feeds, onSelect, onCancel, loading = false, error = null }: Props = $props();

	let selectedFeedId = $state<number | null>(null);
	let createNew = $state(false);
	let newFeedName = $state('');
	let creating = $state(false);
	let createError = $state<string | null>(null);

	async function handleConfirm() {
		if (createNew) {
			if (!newFeedName.trim()) {
				createError = 'Feed name is required';
				return;
			}
			creating = true;
			createError = null;
			try {
				const feed = await createFeed(newFeedName.trim());
				onSelect(feed.id);
			} catch (e) {
				createError = e instanceof Error ? e.message : 'Failed to create feed';
			} finally {
				creating = false;
			}
		} else {
			if (!selectedFeedId) {
				return;
			}
			onSelect(selectedFeedId);
		}
	}

	function selectFeed(feedId: number) {
		selectedFeedId = feedId;
		createNew = false;
		createError = null;
	}

	function selectCreateNew() {
		createNew = true;
		selectedFeedId = null;
		createError = null;
	}
</script>

<!-- Backdrop -->
<div
	class="fixed inset-0 bg-void/80 backdrop-blur-sm z-50 flex items-center justify-center p-4"
	onclick={onCancel}
	role="presentation"
>
	<!-- Modal -->
	<div
		class="bg-surface border border-white/10 rounded-2xl shadow-2xl max-w-md w-full max-h-[80vh] overflow-hidden flex flex-col"
		onclick={(e) => e.stopPropagation()}
		role="dialog"
		aria-modal="true"
		aria-labelledby="feed-selector-title"
	>
		<!-- Header -->
		<div class="p-6 border-b border-white/10">
			<h2 id="feed-selector-title" class="text-xl font-display font-bold">Add Channel to Feed</h2>
			<p class="text-text-muted text-sm mt-1">Choose a feed or create a new one</p>
		</div>

		<!-- Error -->
		{#if error}
			<div class="mx-6 mt-4 bg-crimson-500/10 border border-crimson-500/30 rounded-xl p-3">
				<p class="text-crimson-400 text-sm">{error}</p>
			</div>
		{/if}

		{#if createError}
			<div class="mx-6 mt-4 bg-crimson-500/10 border border-crimson-500/30 rounded-xl p-3">
				<p class="text-crimson-400 text-sm">{createError}</p>
			</div>
		{/if}

		<!-- Feed List -->
		<div class="flex-1 overflow-y-auto p-6 space-y-2">
			{#each feeds as feed}
				<button
					type="button"
					onclick={() => selectFeed(feed.id)}
					class="w-full text-left p-3 rounded-xl border transition-all {selectedFeedId === feed.id
						? 'bg-emerald-500/10 border-emerald-500/50'
						: 'bg-void border-white/5 hover:border-white/20'}"
				>
					<div class="flex items-center gap-3">
						<div
							class="w-5 h-5 rounded-full border-2 flex items-center justify-center {selectedFeedId ===
							feed.id
								? 'border-emerald-500'
								: 'border-white/20'}"
						>
							{#if selectedFeedId === feed.id}
								<div class="w-2.5 h-2.5 rounded-full bg-emerald-500"></div>
							{/if}
						</div>
						<span class="font-medium text-text-primary">{feed.name}</span>
					</div>
				</button>
			{/each}

			<!-- Create New Option -->
			<button
				type="button"
				onclick={selectCreateNew}
				class="w-full text-left p-3 rounded-xl border transition-all {createNew
					? 'bg-emerald-500/10 border-emerald-500/50'
					: 'bg-void border-white/5 hover:border-white/20'}"
			>
				<div class="flex items-center gap-3">
					<div
						class="w-5 h-5 rounded-full border-2 flex items-center justify-center {createNew
							? 'border-emerald-500'
							: 'border-white/20'}"
					>
						{#if createNew}
							<div class="w-2.5 h-2.5 rounded-full bg-emerald-500"></div>
						{/if}
					</div>
					<span class="font-medium text-emerald-400">Create New Feed</span>
				</div>
			</button>

			{#if createNew}
				<div class="pl-8 pt-2">
					<input
						type="text"
						bind:value={newFeedName}
						placeholder="Feed name"
						class="w-full bg-void border border-white/10 rounded-lg px-4 py-2 text-text-primary placeholder-text-dim focus:outline-none focus:border-emerald-500/50 transition-colors"
						autofocus
					/>
				</div>
			{/if}
		</div>

		<!-- Actions -->
		<div class="p-6 border-t border-white/10 flex gap-3">
			<button type="button" onclick={onCancel} class="btn btn-secondary flex-1">
				Cancel
			</button>
			<button
				type="button"
				onclick={handleConfirm}
				disabled={loading || creating || (!selectedFeedId && !createNew) || (createNew && !newFeedName.trim())}
				class="btn btn-primary flex-1"
			>
				{#if loading || creating}
					<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
						<circle
							class="opacity-25"
							cx="12"
							cy="12"
							r="10"
							stroke="currentColor"
							stroke-width="4"
						/>
						<path
							class="opacity-75"
							fill="currentColor"
							d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
						/>
					</svg>
					{creating ? 'Creating...' : 'Adding...'}
				{:else}
					Add Channel
				{/if}
			</button>
		</div>
	</div>
</div>
```

**Step 2: Verify component compiles**

```bash
cd /root/code/feeds/web/frontend
npm run check
```

Expected: No type errors

**Step 3: Commit**

```bash
git add web/frontend/src/lib/components/FeedSelector.svelte
git commit -m "feat(frontend): add FeedSelector modal component

Modal component for selecting destination feed when importing channels.
Features:
- Radio button selection of existing feeds
- Inline new feed creation
- Loading and error states
- Backdrop with blur effect

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 5: Frontend - Integrate Feed Selector into Import Page

**Files:**
- Modify: `web/frontend/src/routes/import/+page.svelte`

**Step 1: Add import for FeedSelector component**

At line 2, after the `goto` import, add:

```typescript
import FeedSelector from '$lib/components/FeedSelector.svelte';
```

**Step 2: Add import for getFeeds and importYouTubeChannel**

At line 3-8, update the imports from `$lib/api` to include the new functions:

```typescript
import {
	importFromURL,
	importWatchHistory,
	confirmOrganize,
	getPacks,
	getFeeds,
	importYouTubeChannel
} from '$lib/api';
```

**Step 3: Add state variables for YouTube import**

After line 26 (after `watchSelectedChannels`), add:

```typescript
// YouTube import
let showFeedSelector = $state(false);
let pendingYouTubeURL = $state('');
let allFeeds = $state<Feed[]>([]);
let youtubeImportLoading = $state(false);
let youtubeImportError = $state<string | null>(null);
```

**Step 4: Add helper function to detect YouTube URLs**

After line 34 (after the `$effect` block), add:

```typescript
function isYouTubeURL(url: string): boolean {
	const patterns = [
		/youtube\.com\/watch\?v=/,
		/youtu\.be\//,
		/youtube\.com\/shorts\//,
		/youtube\.com\/channel\//,
		/youtube\.com\/@/,
		/youtube\.com\/c\//,
		/youtube\.com\/user\//
	];
	return patterns.some(pattern => pattern.test(url));
}
```

**Step 5: Update handleImportURL to detect YouTube URLs**

Replace the `handleImportURL` function (lines 36-50) with:

```typescript
async function handleImportURL(e: Event) {
	e.preventDefault();
	if (!importURL.trim()) return;

	// Check if it's a YouTube URL
	if (isYouTubeURL(importURL)) {
		pendingYouTubeURL = importURL;
		youtubeImportError = null;
		try {
			allFeeds = await getFeeds();
			showFeedSelector = true;
		} catch (e) {
			importError = e instanceof Error ? e.message : 'Failed to load feeds';
		}
		return;
	}

	// Existing JSON import flow
	importLoading = true;
	importError = null;
	try {
		const feed = await importFromURL(importURL);
		goto(`/feeds/${feed.id}`);
	} catch (e) {
		importError = e instanceof Error ? e.message : 'Failed to import';
	} finally {
		importLoading = false;
	}
}
```

**Step 6: Add YouTube import handler functions**

After the `handleImportURL` function (around line 66), add:

```typescript
async function handleYouTubeImportConfirm(feedId: number) {
	youtubeImportLoading = true;
	youtubeImportError = null;

	try {
		const result = await importYouTubeChannel(pendingYouTubeURL, feedId);
		showFeedSelector = false;
		importURL = '';
		goto(`/feeds/${result.feed.id}`);
	} catch (e) {
		youtubeImportError = e instanceof Error ? e.message : 'Failed to add channel';
	} finally {
		youtubeImportLoading = false;
	}
}

function handleYouTubeImportCancel() {
	showFeedSelector = false;
	youtubeImportError = null;
}
```

**Step 7: Update placeholder text**

At line 245, change the placeholder from:

```svelte
placeholder="https://youtube.com/channel/... or video URL"
```

to:

```svelte
placeholder="YouTube channel or video URL, or feed export link"
```

**Step 8: Add FeedSelector modal at the end of the file**

At line 585 (just before the closing `</div>`), add:

```svelte
<!-- Feed Selector Modal -->
{#if showFeedSelector}
	<FeedSelector
		feeds={allFeeds}
		onSelect={handleYouTubeImportConfirm}
		onCancel={handleYouTubeImportCancel}
		loading={youtubeImportLoading}
		error={youtubeImportError}
	/>
{/if}
```

**Step 9: Add Feed type import**

At line 9, add `Feed` to the type imports:

```typescript
import type { WatchHistoryChannel, GroupSuggestion, Feed } from '$lib/types';
```

**Step 10: Verify TypeScript compiles**

```bash
cd /root/code/feeds/web/frontend
npm run check
```

Expected: No type errors

**Step 11: Commit**

```bash
git add web/frontend/src/routes/import/+page.svelte
git commit -m "feat(frontend): integrate YouTube import with feed selector

Enhance /import page to detect YouTube URLs and show feed selector.
Changes:
- Detect YouTube URLs (channels and videos) before submission
- Show FeedSelector modal for YouTube URLs
- Use existing import flow for non-YouTube URLs
- Update placeholder text to reflect new capability
- Handle import confirmation and navigation

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

---

## Task 6: Manual Testing & Verification

**Files:**
- None (testing only)

**Step 1: Start the development server**

```bash
cd /root/code/feeds
make dev
```

This will run in the background per the CLAUDE.md instructions.

**Step 2: Test backend endpoint with curl**

Open a new terminal and test the endpoint:

```bash
# Test with a video URL
curl -X POST http://localhost:8080/api/import/youtube \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "feedId": 1}'

# Test with a channel URL
curl -X POST http://localhost:8080/api/import/youtube \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.youtube.com/@mkbhd", "feedId": 1}'
```

Expected: 201 Created with channel and feed JSON (or 400 if feed doesn't exist)

**Step 3: Test frontend in browser**

1. Navigate to `http://localhost:8080/import`
2. Paste a YouTube video URL: `https://www.youtube.com/watch?v=dQw4w9WgXcQ`
3. Click "Add"
4. Verify FeedSelector modal appears
5. Select a feed
6. Click "Add Channel"
7. Verify navigation to feed page
8. Verify channel appears in the feed

**Step 4: Test edge cases**

Test these scenarios:
- Invalid YouTube URL → should show error
- Private/deleted video → should show error
- Channel URL (@handle, /channel/, /c/, /user/) → should work
- youtu.be short URL → should work
- /shorts/ URL → should work
- JSON feed URL → should use existing import flow (not show modal)

**Step 5: No commit needed (testing only)**

---

## Task 7: Documentation & Final Commit

**Files:**
- Modify: `README.md` (if import section exists)
- Create: None

**Step 1: Update README if needed**

If there's a section about importing channels, add a note about video URL support.

**Step 2: Final verification build**

```bash
cd /root/code/feeds
go build ./...
cd web/frontend
npm run build
```

Expected: Both build successfully

**Step 3: Final commit (if README was updated)**

```bash
git add README.md
git commit -m "docs: update import documentation for video URL support

Generated with [Claude Code](https://claude.ai/code)
via [Happy](https://happy.engineering)

Co-Authored-By: Claude <noreply@anthropic.com>
Co-Authored-By: Happy <yesreply@happy.engineering>"
```

**Step 4: Push all changes**

```bash
git push origin main
```

---

## Testing Checklist

After implementation, verify:

**Channel URL Formats:**
- [ ] `https://youtube.com/channel/UCxxxxxx`
- [ ] `https://youtube.com/@handle`
- [ ] `https://youtube.com/c/customname`
- [ ] `https://youtube.com/user/username`

**Video URL Formats:**
- [ ] `https://youtube.com/watch?v=VIDEO_ID`
- [ ] `https://youtu.be/VIDEO_ID`
- [ ] `https://youtube.com/shorts/VIDEO_ID`
- [ ] `https://m.youtube.com/watch?v=VIDEO_ID`

**Feed Selection:**
- [ ] Add to existing feed
- [ ] Create new feed inline
- [ ] Cancel and return to import page
- [ ] Validation: no feed selected shows appropriate state

**Edge Cases:**
- [ ] Invalid YouTube URL
- [ ] Private/deleted video
- [ ] Private/deleted channel
- [ ] Network timeout/failure

**Existing Functionality:**
- [ ] JSON feed import still works
- [ ] NewPipe import still works
- [ ] Watch history import still works
- [ ] Subscription packs still work

---

## Notes

- No database migrations required
- No breaking changes to existing functionality
- All new code is additive
- Uses existing channel resolution and database patterns
- Follows DRY principle by reusing existing functions
- YAGNI: Only implements requested features, no extras
