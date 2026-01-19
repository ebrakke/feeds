# Quick Feed Creation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add "Add to feed" submenu to video card menu, allowing users to add channels to existing feeds or create new feeds on the fly.

**Architecture:** Toast store + component for notifications. VideoCard gets new submenu that fetches feeds/channel data on open, handles add and create flows.

**Tech Stack:** SvelteKit 5 (runes), TypeScript, existing API client

---

## Task 1: Create Toast Store

**Files:**
- Create: `web/frontend/src/lib/stores/toast.ts`

**Step 1: Create the toast store**

```typescript
import { writable } from 'svelte/store';

export interface Toast {
	id: number;
	message: string;
	type: 'success' | 'error';
}

function createToastStore() {
	const { subscribe, update } = writable<Toast[]>([]);
	let nextId = 0;

	return {
		subscribe,
		show(message: string, type: 'success' | 'error' = 'success') {
			const id = nextId++;
			update((toasts) => [...toasts, { id, message, type }]);
			setTimeout(() => {
				update((toasts) => toasts.filter((t) => t.id !== id));
			}, 3000);
		},
		success(message: string) {
			this.show(message, 'success');
		},
		error(message: string) {
			this.show(message, 'error');
		}
	};
}

export const toast = createToastStore();
```

**Step 2: Commit**

```bash
git add web/frontend/src/lib/stores/toast.ts
git commit -m "feat: add toast notification store"
```

---

## Task 2: Create Toast Component

**Files:**
- Create: `web/frontend/src/lib/components/Toast.svelte`

**Step 1: Create the toast component**

```svelte
<script lang="ts">
	import { toast } from '$lib/stores/toast';
</script>

{#if $toast.length > 0}
	<div class="fixed bottom-4 right-4 z-[100] flex flex-col gap-2">
		{#each $toast as t (t.id)}
			<div
				class="px-4 py-3 rounded-lg shadow-lg backdrop-blur-sm text-sm font-medium animate-slide-in
					{t.type === 'success' ? 'bg-emerald-500/90 text-white' : 'bg-crimson-500/90 text-white'}"
			>
				{t.message}
			</div>
		{/each}
	</div>
{/if}

<style>
	@keyframes slide-in {
		from {
			opacity: 0;
			transform: translateX(100%);
		}
		to {
			opacity: 1;
			transform: translateX(0);
		}
	}
	.animate-slide-in {
		animation: slide-in 0.2s ease-out;
	}
</style>
```

**Step 2: Commit**

```bash
git add web/frontend/src/lib/components/Toast.svelte
git commit -m "feat: add toast notification component"
```

---

## Task 3: Mount Toast in Layout

**Files:**
- Modify: `web/frontend/src/routes/+layout.svelte`

**Step 1: Add toast component import and mount**

Add import at top of script:
```typescript
import Toast from '$lib/components/Toast.svelte';
```

Add `<Toast />` at the end of the template, just before the closing `</div>`:
```svelte
	</footer>
	<Toast />
</div>
```

**Step 2: Commit**

```bash
git add web/frontend/src/routes/+layout.svelte
git commit -m "feat: mount toast component in layout"
```

---

## Task 4: Add "Add to feed" Submenu to VideoCard

**Files:**
- Modify: `web/frontend/src/lib/components/VideoCard.svelte`

**Step 1: Add imports and state**

Add to imports at top:
```typescript
import { getFeeds, getChannel, addChannelToFeed, createFeed } from '$lib/api';
import { toast } from '$lib/stores/toast';
import type { Feed } from '$lib/types';
```

Add new state variables after existing state:
```typescript
let showAddToFeedMenu = $state(false);
let availableFeeds = $state<Feed[]>([]);
let loadingFeeds = $state(false);
let addingToFeed = $state(false);
```

**Step 2: Add handler functions**

Add after existing handler functions:

```typescript
async function handleOpenAddToFeed(e: Event) {
	e.preventDefault();
	e.stopPropagation();
	if (loadingFeeds) return;

	showAddToFeedMenu = !showAddToFeedMenu;
	if (!showAddToFeedMenu) return;

	loadingFeeds = true;
	try {
		const [feedsResult, channelResult] = await Promise.all([
			getFeeds(),
			getChannel(video.channel_id)
		]);

		const channelFeedIds = new Set(channelResult.feeds.map((f) => f.id));
		availableFeeds = feedsResult.filter(
			(f) => !f.is_system && f.id !== currentFeedId && !channelFeedIds.has(f.id)
		);
	} catch (err) {
		console.error('Failed to load feeds:', err);
		toast.error('Failed to load feeds');
		showAddToFeedMenu = false;
	} finally {
		loadingFeeds = false;
	}
}

async function handleAddToFeed(feed: Feed, e: Event) {
	e.preventDefault();
	e.stopPropagation();
	if (addingToFeed) return;

	addingToFeed = true;
	try {
		await addChannelToFeed(video.channel_id, feed.id);
		toast.success(`Added to ${feed.name}`);
		showMenu = false;
		showAddToFeedMenu = false;
	} catch (err) {
		console.error('Failed to add to feed:', err);
		toast.error('Failed to add to feed');
	} finally {
		addingToFeed = false;
	}
}

async function handleCreateNewFeed(e: Event) {
	e.preventDefault();
	e.stopPropagation();

	const name = prompt('Enter feed name:');
	if (!name?.trim()) return;

	addingToFeed = true;
	try {
		const newFeed = await createFeed(name.trim());
		await addChannelToFeed(video.channel_id, newFeed.id);
		toast.success(`Added to ${newFeed.name}`);
		showMenu = false;
		showAddToFeedMenu = false;
	} catch (err) {
		console.error('Failed to create feed:', err);
		toast.error('Failed to create feed');
	} finally {
		addingToFeed = false;
	}
}
```

**Step 3: Update closeMenu to also close submenu**

Update the `closeMenu` function:
```typescript
function closeMenu() {
	showMenu = false;
	showAddToFeedMenu = false;
}
```

**Step 4: Add "Add to feed" menu item with submenu**

In the template, add the new menu item after the "Mark as watched" button and before the "Remove from feed" section. Find this line:
```svelte
{#if showRemoveFromFeed && currentFeedId}
```

Insert this block BEFORE that line:
```svelte
<div class="border-t border-white/10"></div>
<div class="relative">
	<button
		onclick={handleOpenAddToFeed}
		disabled={loadingFeeds || addingToFeed}
		class="flex items-center justify-between w-full px-4 py-2 text-sm hover:bg-white/5 transition-colors disabled:opacity-50"
	>
		<span class="flex items-center">
			{#if loadingFeeds}
				<svg class="w-4 h-4 mr-2 animate-spin" viewBox="0 0 24 24" fill="none">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
				</svg>
			{:else}
				<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M12 5v14M5 12h14"/>
				</svg>
			{/if}
			Add to feed
		</span>
		<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
			<path d="M9 18l6-6-6-6"/>
		</svg>
	</button>
	{#if showAddToFeedMenu}
		<div class="absolute left-full top-0 ml-1 w-48 bg-surface border border-white/10 rounded-lg shadow-xl z-50">
			{#if availableFeeds.length > 0}
				{#each availableFeeds as feed}
					<button
						onclick={(e) => handleAddToFeed(feed, e)}
						disabled={addingToFeed}
						class="flex items-center w-full px-4 py-2 text-sm hover:bg-white/5 transition-colors first:rounded-t-lg disabled:opacity-50"
					>
						{feed.name}
					</button>
				{/each}
				<div class="border-t border-white/10"></div>
			{/if}
			<button
				onclick={handleCreateNewFeed}
				disabled={addingToFeed}
				class="flex items-center w-full px-4 py-2 text-sm text-emerald-400 hover:bg-emerald-500/10 transition-colors rounded-b-lg {availableFeeds.length === 0 ? 'rounded-t-lg' : ''} disabled:opacity-50"
			>
				<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M12 5v14M5 12h14"/>
				</svg>
				Create new feed...
			</button>
		</div>
	{/if}
</div>
```

**Step 5: Commit**

```bash
git add web/frontend/src/lib/components/VideoCard.svelte
git commit -m "feat: add 'Add to feed' submenu to video card menu"
```

---

## Task 5: Manual Testing

**Step 1: Start the dev server**

```bash
make dev
```

**Step 2: Test the feature**

1. Navigate to a feed with videos
2. Click the three-dot menu on a video card
3. Click "Add to feed" - verify submenu appears with available feeds
4. Click a feed name - verify toast shows "Added to [Feed Name]"
5. Repeat and click "Create new feed..." - verify prompt appears
6. Enter a name - verify feed is created and toast shows
7. Verify the channel now appears in the new feed

**Step 3: Test edge cases**

1. Test when channel is already in all feeds (only "Create new feed..." should show)
2. Test canceling the prompt (nothing should happen)
3. Test with empty feed name (should do nothing)

---

## Task 6: Final Commit

**Step 1: Verify all changes are committed**

```bash
git status
git log --oneline -5
```

All changes should be committed across the previous tasks.
