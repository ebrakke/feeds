# Mobile UX Redesign - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Streamline mobile UX by adding contextual navigation, hamburger menu, improved feed management, and inline feed creation.

**Architecture:** Frontend-first implementation with minimal backend changes. Add navigation store for origin tracking, new menu component, enhanced bottom sheet with inline feed creation, and new feed creation flow.

**Tech Stack:** SvelteKit 2, Svelte 5 (runes), Tailwind CSS 4, TypeScript

---

## Task 1: Add Navigation Origin Store

**Files:**
- Create: `/root/code/feeds/web/frontend/src/lib/stores/navigation.ts`

**Step 1: Create the navigation store**

```typescript
import { writable, get } from 'svelte/store';

export interface NavigationOrigin {
  feedId: number;
  feedName: string;
  path: string;
}

function createNavigationStore() {
  const { subscribe, set, update } = writable<NavigationOrigin | null>(null);

  return {
    subscribe,
    setOrigin: (origin: NavigationOrigin) => set(origin),
    clear: () => set(null),
    get: () => get({ subscribe })
  };
}

export const navigationOrigin = createNavigationStore();
```

**Step 2: Commit**

```bash
git add web/frontend/src/lib/stores/navigation.ts
git commit -m "feat: add navigation origin store for contextual back navigation"
```

---

## Task 2: Create Hamburger Menu Component

**Files:**
- Create: `/root/code/feeds/web/frontend/src/lib/components/HamburgerMenu.svelte`

**Step 1: Create the hamburger menu component**

```svelte
<script lang="ts">
  import { fly } from 'svelte/transition';

  let { open = $bindable(false) } = $props();

  function handleClose() {
    open = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      handleClose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 bg-black/60 backdrop-blur-sm z-[9998]"
    onclick={handleClose}
    role="button"
    tabindex="-1"
    aria-label="Close menu"
    transition:fly={{ duration: 200, opacity: 0 }}
  ></div>

  <!-- Menu Panel -->
  <div
    class="fixed top-0 left-0 bottom-0 w-72 max-w-[80vw] bg-surface border-r border-border-subtle z-[9999] flex flex-col"
    transition:fly={{ x: -288, duration: 300, opacity: 1 }}
  >
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-border-subtle">
      <span class="text-lg font-display font-semibold text-text-primary">Menu</span>
      <button
        onclick={handleClose}
        class="p-2 -m-2 text-text-muted hover:text-text-primary transition-colors"
        aria-label="Close menu"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Menu Items -->
    <nav class="flex-1 overflow-y-auto py-2">
      <!-- Import/Export Section -->
      <div class="px-3 py-2">
        <div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Import & Export</div>
        <a
          href="/import"
          onclick={handleClose}
          class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
          </svg>
          <span>Import watch history</span>
        </a>
        <a
          href="/import#packs"
          onclick={handleClose}
          class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
          </svg>
          <span>Subscription packs</span>
        </a>
      </div>

      <div class="my-2 border-t border-border-subtle"></div>

      <!-- Views Section -->
      <div class="px-3 py-2">
        <div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Views</div>
        <a
          href="/all"
          onclick={handleClose}
          class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
          </svg>
          <span>All videos</span>
        </a>
        <a
          href="/history"
          onclick={handleClose}
          class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span>Watch history</span>
        </a>
      </div>
    </nav>

    <!-- Footer with safe area -->
    <div class="p-4 border-t border-border-subtle" style="padding-bottom: max(1rem, env(safe-area-inset-bottom));">
      <a
        href="/settings"
        onclick={handleClose}
        class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <span>Settings</span>
      </a>
    </div>
  </div>
{/if}
```

**Step 2: Commit**

```bash
git add web/frontend/src/lib/components/HamburgerMenu.svelte
git commit -m "feat: add hamburger menu component for infrequent actions"
```

---

## Task 3: Update Layout Header

**Files:**
- Modify: `/root/code/feeds/web/frontend/src/routes/+layout.svelte`

**Step 1: Update the layout to use hamburger menu and contextual navigation**

Replace the entire `<header>` section and add the hamburger menu import. The key changes:
1. Add hamburger menu button on the left
2. Show origin feed name when navigating from a feed
3. Replace Import button with Settings button
4. Add HamburgerMenu component

Update imports at top:
```svelte
<script lang="ts">
  import '../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import BottomSheet from '$lib/components/BottomSheet.svelte';
  import HamburgerMenu from '$lib/components/HamburgerMenu.svelte';
  import { navigationOrigin } from '$lib/stores/navigation';
  import { page } from '$app/stores';

  let { children } = $props();

  let menuOpen = $state(false);

  // Determine if we should show origin navigation
  let showOrigin = $derived(
    $navigationOrigin !== null &&
    ($page.url.pathname.startsWith('/watch/') || $page.url.pathname.startsWith('/channels/'))
  );
</script>
```

Replace header:
```svelte
<header class="app-header">
  <div class="container">
    <nav class="flex items-center justify-between h-14 sm:h-16">
      {#if showOrigin && $navigationOrigin}
        <!-- Contextual back navigation -->
        <a
          href={$navigationOrigin.path}
          class="flex items-center gap-2 -ml-1 p-1 rounded-lg text-text-primary hover:bg-elevated transition-colors min-w-0"
        >
          <svg class="w-5 h-5 flex-shrink-0 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
          </svg>
          <span class="font-medium truncate">{$navigationOrigin.feedName}</span>
        </a>
      {:else}
        <!-- Default navigation -->
        <div class="flex items-center gap-2">
          <button
            onclick={() => menuOpen = true}
            class="p-2 -ml-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-elevated transition-colors"
            aria-label="Open menu"
          >
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
          <a href="/" class="group flex items-center gap-2 p-1 rounded-lg hover:bg-elevated transition-colors">
            <div class="w-8 h-8 sm:w-9 sm:h-9 rounded-lg bg-gradient-to-br from-emerald-400 to-emerald-600 flex items-center justify-center shadow-lg shadow-emerald-500/20">
              <svg class="w-4 h-4 sm:w-5 sm:h-5 text-white" viewBox="0 0 24 24" fill="currentColor">
                <path d="M8 5v14l11-7z"/>
              </svg>
            </div>
            <span class="text-lg font-display font-semibold text-text-primary">Feeds</span>
          </a>
        </div>
      {/if}

      <!-- Right side buttons -->
      <div class="flex items-center gap-1 -mr-1">
        <a href="/settings" class="btn btn-ghost btn-sm" aria-label="Settings">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </a>
      </div>
    </nav>
  </div>
</header>
```

Add menu component before closing div:
```svelte
<HamburgerMenu bind:open={menuOpen} />
```

**Step 2: Commit**

```bash
git add web/frontend/src/routes/+layout.svelte
git commit -m "feat: update header with hamburger menu and contextual navigation"
```

---

## Task 4: Set Navigation Origin in Feed Page

**Files:**
- Modify: `/root/code/feeds/web/frontend/src/routes/feeds/[id]/+page.svelte`

**Step 1: Import and set navigation origin on mount**

Add to the script section near the top imports:
```typescript
import { navigationOrigin } from '$lib/stores/navigation';
import { onMount } from 'svelte';
```

Add after `feed` variable is set (after the data loading):
```typescript
onMount(() => {
  if (feed) {
    navigationOrigin.setOrigin({
      feedId: feed.id,
      feedName: feed.name,
      path: `/feeds/${feed.id}`
    });
  }
});
```

**Step 2: Commit**

```bash
git add web/frontend/src/routes/feeds/[id]/+page.svelte
git commit -m "feat: set navigation origin when entering feed page"
```

---

## Task 5: Clear Navigation Origin on Home Page

**Files:**
- Modify: `/root/code/feeds/web/frontend/src/routes/+page.svelte`

**Step 1: Clear navigation origin on mount**

Add to imports:
```typescript
import { navigationOrigin } from '$lib/stores/navigation';
import { onMount } from 'svelte';
```

Add onMount:
```typescript
onMount(() => {
  navigationOrigin.clear();
});
```

**Step 2: Commit**

```bash
git add web/frontend/src/routes/+page.svelte
git commit -m "feat: clear navigation origin when returning to home"
```

---

## Task 6: Update Home Page Layout

**Files:**
- Modify: `/root/code/feeds/web/frontend/src/routes/+page.svelte`

**Step 1: Refactor home page to separate system feeds from user feeds**

This is a larger change. The key updates:
1. Remove the Inbox card display
2. Remove the "Quick open" / paste URL section
3. Separate feeds into two groups: frequency (system-like) and user feeds
4. Add "+ New Feed" button at the bottom

Find the feeds display section and update it to:

```svelte
<!-- Frequency Feeds (system-generated from watch history) -->
{#if frequencyFeeds.length > 0}
  <div class="space-y-2">
    {#each frequencyFeeds as feed}
      <a
        href="/feeds/{feed.id}"
        class="card flex items-center justify-between p-4 hover:bg-elevated transition-colors group"
      >
        <div class="flex items-center gap-3 min-w-0">
          <div class="w-10 h-10 rounded-lg bg-gradient-to-br from-emerald-500/20 to-emerald-600/20 flex items-center justify-center flex-shrink-0">
            <svg class="w-5 h-5 text-emerald-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </div>
          <span class="font-medium text-text-primary truncate">{feed.name}</span>
        </div>
        <svg class="w-5 h-5 text-text-muted group-hover:text-text-secondary transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
        </svg>
      </a>
    {/each}
  </div>
{/if}

<!-- Divider -->
{#if frequencyFeeds.length > 0 && userFeeds.length > 0}
  <div class="flex items-center gap-4 py-4">
    <div class="flex-1 border-t border-border-subtle"></div>
    <span class="text-sm text-text-muted">Your Feeds</span>
    <div class="flex-1 border-t border-border-subtle"></div>
  </div>
{/if}

<!-- User Feeds -->
{#if userFeeds.length > 0}
  <div class="space-y-2">
    {#each userFeeds as feed}
      <a
        href="/feeds/{feed.id}"
        class="card flex items-center justify-between p-4 hover:bg-elevated transition-colors group"
      >
        <div class="flex items-center gap-3 min-w-0">
          <div class="w-10 h-10 rounded-lg bg-gradient-to-br from-violet-500/20 to-violet-600/20 flex items-center justify-center flex-shrink-0">
            <svg class="w-5 h-5 text-violet-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </div>
          <span class="font-medium text-text-primary truncate">{feed.name}</span>
        </div>
        <svg class="w-5 h-5 text-text-muted group-hover:text-text-secondary transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
        </svg>
      </a>
    {/each}
  </div>
{/if}

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
```

Add to the script section to separate feeds:
```typescript
// Separate frequency feeds (from watch history import) from user-created feeds
// Frequency feeds typically have names like "Heavy Rotation", "Regulars", etc.
const FREQUENCY_FEED_NAMES = ['Heavy Rotation', 'Regulars', 'Frequent', 'Occasional', 'A Few Times', 'Discovered'];

let frequencyFeeds = $derived(
  feeds.filter(f => !f.is_system && FREQUENCY_FEED_NAMES.includes(f.name))
);

let userFeeds = $derived(
  feeds.filter(f => !f.is_system && !FREQUENCY_FEED_NAMES.includes(f.name))
);
```

**Step 2: Commit**

```bash
git add web/frontend/src/routes/+page.svelte
git commit -m "feat: reorganize home page with frequency feeds and user feeds sections"
```

---

## Task 7: Create Settings Page

**Files:**
- Create: `/root/code/feeds/web/frontend/src/routes/settings/+page.svelte`

**Step 1: Create a basic settings page**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';

  let cookiesValue = $state('');
  let saving = $state(false);
  let message = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  onMount(async () => {
    try {
      const res = await fetch('/api/config');
      const data = await res.json();
      if (data.hasCookies) {
        cookiesValue = '(cookies configured)';
      }
    } catch (e) {
      console.error('Failed to load config:', e);
    }
  });

  async function saveCookies() {
    if (!cookiesValue || cookiesValue === '(cookies configured)') return;

    saving = true;
    message = null;

    try {
      const res = await fetch('/api/config/ytdlp-cookies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ cookies: cookiesValue })
      });

      if (res.ok) {
        message = { type: 'success', text: 'Cookies saved successfully' };
        cookiesValue = '(cookies configured)';
      } else {
        const data = await res.json();
        message = { type: 'error', text: data.error || 'Failed to save cookies' };
      }
    } catch (e) {
      message = { type: 'error', text: 'Failed to save cookies' };
    } finally {
      saving = false;
    }
  }
</script>

<svelte:head>
  <title>Settings - Feeds</title>
</svelte:head>

<div class="space-y-8">
  <div>
    <h1 class="text-2xl font-display font-bold text-text-primary">Settings</h1>
    <p class="text-text-secondary mt-1">Configure your Feeds app</p>
  </div>

  <!-- YouTube Cookies Section -->
  <div class="card p-6 space-y-4">
    <div>
      <h2 class="text-lg font-semibold text-text-primary">YouTube Cookies</h2>
      <p class="text-sm text-text-secondary mt-1">
        Provide cookies for age-restricted or member-only content
      </p>
    </div>

    <div class="space-y-3">
      <textarea
        bind:value={cookiesValue}
        placeholder="Paste your cookies.txt content here..."
        class="input w-full h-32 font-mono text-sm resize-none"
        disabled={saving}
      ></textarea>

      {#if message}
        <p class="text-sm {message.type === 'success' ? 'text-emerald-500' : 'text-crimson-500'}">
          {message.text}
        </p>
      {/if}

      <button
        onclick={saveCookies}
        disabled={saving || !cookiesValue || cookiesValue === '(cookies configured)'}
        class="btn btn-primary"
      >
        {saving ? 'Saving...' : 'Save Cookies'}
      </button>
    </div>
  </div>

  <!-- About Section -->
  <div class="card p-6 space-y-4">
    <div>
      <h2 class="text-lg font-semibold text-text-primary">About</h2>
      <p class="text-sm text-text-secondary mt-1">
        Feeds is a personal YouTube feed aggregator
      </p>
    </div>
  </div>
</div>
```

**Step 2: Commit**

```bash
git add web/frontend/src/routes/settings/+page.svelte
git commit -m "feat: add settings page with cookies configuration"
```

---

## Task 8: Create New Feed Page (Step 1 - Name & Icon)

**Files:**
- Create: `/root/code/feeds/web/frontend/src/routes/feeds/new/+page.svelte`

**Step 1: Create the new feed page with name input and icon picker**

```svelte
<script lang="ts">
  import { goto } from '$app/navigation';
  import { getFeeds, createFeed } from '$lib/api';
  import type { Feed, Channel } from '$lib/types';
  import { onMount } from 'svelte';

  // Step state
  let step = $state<1 | 2>(1);

  // Step 1: Name and icon
  let feedName = $state('');
  let selectedIcon = $state('');

  // Step 2: Channel selection
  let feeds = $state<Feed[]>([]);
  let allChannels = $state<{ channel: Channel; feedId: number; feedName: string }[]>([]);
  let selectedFeedFilter = $state<number | 'all'>('all');
  let searchQuery = $state('');
  let selectedChannelIds = $state<Set<number>>(new Set());

  let creating = $state(false);
  let error = $state('');

  const ICONS = ['', '', '', '', '', '', '', '', '', '', '', '', '', '', '', ''];

  onMount(async () => {
    try {
      feeds = await getFeeds();
      // Build channel list with feed info
      for (const feed of feeds) {
        if (feed.is_system) continue;
        const res = await fetch(`/api/feeds/${feed.id}`);
        const data = await res.json();
        for (const channel of data.channels || []) {
          // Avoid duplicates
          if (!allChannels.some(c => c.channel.id === channel.id)) {
            allChannels.push({ channel, feedId: feed.id, feedName: feed.name });
          }
        }
      }
      allChannels = allChannels;
    } catch (e) {
      error = 'Failed to load feeds';
    }
  });

  function goToStep2() {
    if (!feedName.trim()) {
      error = 'Please enter a feed name';
      return;
    }
    error = '';
    step = 2;
  }

  let filteredChannels = $derived(() => {
    let result = allChannels;

    if (selectedFeedFilter !== 'all') {
      result = result.filter(c => c.feedId === selectedFeedFilter);
    }

    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      result = result.filter(c => c.channel.name.toLowerCase().includes(query));
    }

    return result.sort((a, b) => a.channel.name.localeCompare(b.channel.name));
  });

  function toggleChannel(channelId: number) {
    const newSet = new Set(selectedChannelIds);
    if (newSet.has(channelId)) {
      newSet.delete(channelId);
    } else {
      newSet.add(channelId);
    }
    selectedChannelIds = newSet;
  }

  async function handleCreate() {
    if (selectedChannelIds.size === 0) {
      error = 'Please select at least one channel';
      return;
    }

    creating = true;
    error = '';

    try {
      // Create the feed
      const feed = await createFeed(feedName.trim());

      // Add selected channels
      for (const channelId of selectedChannelIds) {
        const channel = allChannels.find(c => c.channel.id === channelId);
        if (channel) {
          await fetch(`/api/channels/${channelId}/feeds`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ feed_id: feed.id })
          });
        }
      }

      goto(`/feeds/${feed.id}`);
    } catch (e) {
      error = 'Failed to create feed';
      creating = false;
    }
  }
</script>

<svelte:head>
  <title>New Feed - Feeds</title>
</svelte:head>

<div class="max-w-lg mx-auto space-y-6">
  {#if step === 1}
    <!-- Step 1: Name and Icon -->
    <div>
      <h1 class="text-2xl font-display font-bold text-text-primary">New Feed</h1>
      <p class="text-text-secondary mt-1">Give your feed a name</p>
    </div>

    <div class="space-y-4">
      <div>
        <label for="feed-name" class="block text-sm font-medium text-text-secondary mb-2">Feed name</label>
        <input
          id="feed-name"
          type="text"
          bind:value={feedName}
          placeholder="e.g., Cooking, Gaming, Music"
          class="input w-full"
          onkeydown={(e) => e.key === 'Enter' && goToStep2()}
        />
      </div>

      <div>
        <label class="block text-sm font-medium text-text-secondary mb-2">Icon (optional)</label>
        <div class="flex flex-wrap gap-2">
          {#each ICONS as icon}
            <button
              onclick={() => selectedIcon = selectedIcon === icon ? '' : icon}
              class="w-10 h-10 rounded-lg text-xl flex items-center justify-center transition-colors {selectedIcon === icon ? 'bg-emerald-500/20 ring-2 ring-emerald-500' : 'bg-elevated hover:bg-surface'}"
            >
              {icon}
            </button>
          {/each}
        </div>
      </div>

      {#if error}
        <p class="text-sm text-crimson-500">{error}</p>
      {/if}

      <button onclick={goToStep2} class="btn btn-primary w-full">
        Next
      </button>
    </div>

  {:else}
    <!-- Step 2: Channel Selection -->
    <div class="flex items-center justify-between">
      <div>
        <button onclick={() => step = 1} class="text-text-muted hover:text-text-primary transition-colors">
          <svg class="w-5 h-5 inline mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
          </svg>
          Back
        </button>
        <h1 class="text-2xl font-display font-bold text-text-primary mt-2">Add channels</h1>
      </div>
      <button
        onclick={handleCreate}
        disabled={creating || selectedChannelIds.size === 0}
        class="btn btn-primary"
      >
        {creating ? 'Creating...' : `Create (${selectedChannelIds.size})`}
      </button>
    </div>

    <div class="space-y-4">
      <!-- Search -->
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Search channels..."
        class="input w-full"
      />

      <!-- Feed Filter -->
      <div>
        <label for="feed-filter" class="block text-sm font-medium text-text-secondary mb-2">From feed</label>
        <select id="feed-filter" bind:value={selectedFeedFilter} class="select w-full">
          <option value="all">All feeds</option>
          {#each feeds.filter(f => !f.is_system) as feed}
            <option value={feed.id}>{feed.name}</option>
          {/each}
        </select>
      </div>

      <!-- Channel List -->
      <div class="card divide-y divide-border-subtle max-h-[50vh] overflow-y-auto">
        {#each filteredChannels() as { channel }}
          <button
            onclick={() => toggleChannel(channel.id)}
            class="w-full flex items-center gap-3 p-3 hover:bg-elevated transition-colors text-left"
          >
            <div class="w-6 h-6 rounded border-2 flex items-center justify-center flex-shrink-0 transition-colors {selectedChannelIds.has(channel.id) ? 'bg-emerald-500 border-emerald-500' : 'border-text-muted'}">
              {#if selectedChannelIds.has(channel.id)}
                <svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
              {/if}
            </div>
            <span class="text-text-primary truncate">{channel.name}</span>
          </button>
        {:else}
          <div class="p-8 text-center text-text-muted">
            {#if searchQuery}
              No channels match your search
            {:else}
              No channels available
            {/if}
          </div>
        {/each}
      </div>

      {#if error}
        <p class="text-sm text-crimson-500">{error}</p>
      {/if}
    </div>
  {/if}
</div>
```

**Step 2: Commit**

```bash
git add web/frontend/src/routes/feeds/new/+page.svelte
git commit -m "feat: add new feed creation page with channel picker"
```

---

## Task 9: Add createFeed API Function

**Files:**
- Modify: `/root/code/feeds/web/frontend/src/lib/api.ts`

**Step 1: Add createFeed function**

Add this function to the API file:

```typescript
export async function createFeed(name: string): Promise<Feed> {
  return fetchJSON('/feeds', {
    method: 'POST',
    body: JSON.stringify({ name })
  });
}
```

**Step 2: Commit**

```bash
git add web/frontend/src/lib/api.ts
git commit -m "feat: add createFeed API function"
```

---

## Task 10: Enhance Bottom Sheet with Feed Membership Display

**Files:**
- Modify: `/root/code/feeds/web/frontend/src/lib/stores/bottomSheet.ts`
- Modify: `/root/code/feeds/web/frontend/src/lib/components/BottomSheet.svelte`

**Step 1: Update bottom sheet store to include channel's current feed memberships**

Update the store interface in `bottomSheet.ts`:

```typescript
import { writable } from 'svelte/store';
import type { Feed } from '$lib/types';

export interface BottomSheetState {
  open: boolean;
  title: string;
  channelId: number | null;
  channelName: string;
  feeds: Feed[];
  memberFeedIds: number[];  // NEW: feeds this channel is already in
}

const initialState: BottomSheetState = {
  open: false,
  title: '',
  channelId: null,
  channelName: '',
  feeds: [],
  memberFeedIds: []
};

function createBottomSheetStore() {
  const { subscribe, set, update } = writable<BottomSheetState>(initialState);

  return {
    subscribe,
    open: (data: Omit<BottomSheetState, 'open'>) => {
      set({ ...data, open: true });
    },
    close: () => {
      set(initialState);
    }
  };
}

export const bottomSheet = createBottomSheetStore();
```

**Step 2: Update BottomSheet.svelte to show membership status**

Update the feed list rendering to show a filled indicator for feeds the channel is already in:

```svelte
{#each $bottomSheet.feeds.filter(f => !f.is_system) as feed}
  <button
    onclick={() => handleAddToFeed(feed.id)}
    disabled={processing}
    class="w-full flex items-center justify-between px-4 py-3 hover:bg-elevated transition-colors disabled:opacity-50 text-left"
  >
    <span class="text-text-primary">{feed.name}</span>
    <div class="w-5 h-5 rounded-full border-2 flex items-center justify-center {$bottomSheet.memberFeedIds.includes(feed.id) ? 'bg-emerald-500 border-emerald-500' : 'border-text-muted'}">
      {#if $bottomSheet.memberFeedIds.includes(feed.id)}
        <svg class="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
      {/if}
    </div>
  </button>
{/each}
```

**Step 3: Commit**

```bash
git add web/frontend/src/lib/stores/bottomSheet.ts web/frontend/src/lib/components/BottomSheet.svelte
git commit -m "feat: show feed membership status in bottom sheet"
```

---

## Task 11: Add Inline Feed Creation to Bottom Sheet

**Files:**
- Modify: `/root/code/feeds/web/frontend/src/lib/components/BottomSheet.svelte`

**Step 1: Add inline feed creation mode**

Add state variables:
```typescript
let creatingNew = $state(false);
let newFeedName = $state('');
let creating = $state(false);
```

Add create new feed handler:
```typescript
async function handleCreateAndAdd() {
  if (!newFeedName.trim() || !$bottomSheet.channelId) return;

  creating = true;
  try {
    // Create feed
    const res = await fetch('/api/feeds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: newFeedName.trim() })
    });
    const feed = await res.json();

    // Add channel to new feed
    await fetch(`/api/channels/${$bottomSheet.channelId}/feeds`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ feed_id: feed.id })
    });

    toast.success(`Created "${newFeedName}" and added ${$bottomSheet.channelName}`);
    bottomSheet.close();
    newFeedName = '';
    creatingNew = false;
  } catch (e) {
    toast.error('Failed to create feed');
  } finally {
    creating = false;
  }
}
```

Add UI for create new mode after the feed list:
```svelte
<div class="border-t border-border-subtle">
  {#if creatingNew}
    <div class="p-4 space-y-3">
      <input
        type="text"
        bind:value={newFeedName}
        placeholder="New feed name..."
        class="input w-full"
        onkeydown={(e) => e.key === 'Enter' && handleCreateAndAdd()}
        autofocus
      />
      <div class="flex gap-2">
        <button
          onclick={() => { creatingNew = false; newFeedName = ''; }}
          class="btn btn-ghost flex-1"
          disabled={creating}
        >
          Cancel
        </button>
        <button
          onclick={handleCreateAndAdd}
          class="btn btn-primary flex-1"
          disabled={creating || !newFeedName.trim()}
        >
          {creating ? 'Creating...' : 'Create'}
        </button>
      </div>
    </div>
  {:else}
    <button
      onclick={() => creatingNew = true}
      class="w-full flex items-center gap-3 px-4 py-3 text-emerald-500 hover:bg-elevated transition-colors"
    >
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
      </svg>
      <span>Create new feed</span>
    </button>
  {/if}
</div>
```

**Step 2: Commit**

```bash
git add web/frontend/src/lib/components/BottomSheet.svelte
git commit -m "feat: add inline feed creation to bottom sheet"
```

---

## Task 12: Update VideoCard to Pass Member Feed IDs

**Files:**
- Modify: `/root/code/feeds/web/frontend/src/lib/components/VideoCard.svelte`

**Step 1: Fetch channel's feed memberships when opening bottom sheet**

Update the `handleAddToFeed` function to also fetch memberships:

```typescript
async function handleAddToFeed() {
  menuOpen = false;

  // Get all feeds
  const feeds = await getFeeds();

  // Get which feeds this channel is already in
  let memberFeedIds: number[] = [];
  try {
    const res = await fetch(`/api/channels/${video.channel_id}/feeds`);
    if (res.ok) {
      const memberships = await res.json();
      memberFeedIds = memberships.map((m: { feedId: number }) => m.feedId);
    }
  } catch (e) {
    // Ignore - just won't show membership status
  }

  bottomSheet.open({
    title: `Add "${video.channel_name}" to feed`,
    channelId: video.channel_id,
    channelName: video.channel_name,
    feeds,
    memberFeedIds
  });
}
```

**Step 2: Commit**

```bash
git add web/frontend/src/lib/components/VideoCard.svelte
git commit -m "feat: fetch channel feed memberships for bottom sheet display"
```

---

## Task 13: Add Backend API for Channel Feed Memberships

**Files:**
- Modify: `/root/code/feeds/internal/api/handlers.go`
- Modify: `/root/code/feeds/internal/api/api_handlers.go`

**Step 1: Add handler for getting a channel's feed memberships**

Add route in `RegisterRoutes`:
```go
mux.HandleFunc("GET /api/channels/{id}/feeds", s.handleAPIGetChannelFeeds)
```

Add handler in `api_handlers.go`:
```go
// handleAPIGetChannelFeeds returns all feeds that contain a channel
func (s *Server) handleAPIGetChannelFeeds(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	feeds, err := s.db.GetFeedsByChannel(channelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type membership struct {
		ChannelID int64  `json:"channelId"`
		FeedID    int64  `json:"feedId"`
		FeedName  string `json:"feedName"`
	}

	memberships := make([]membership, 0, len(feeds))
	for _, feed := range feeds {
		memberships = append(memberships, membership{
			ChannelID: channelID,
			FeedID:    feed.ID,
			FeedName:  feed.Name,
		})
	}

	jsonResponse(w, memberships)
}
```

**Step 2: Commit**

```bash
git add internal/api/handlers.go internal/api/api_handlers.go
git commit -m "feat: add API endpoint for channel feed memberships"
```

---

## Task 14: Test the Implementation

**Step 1: Start the development server**

```bash
cd /root/code/feeds && make dev
```

**Step 2: Manual testing checklist**

1. [ ] Home page shows hamburger menu icon
2. [ ] Clicking hamburger opens slide-out menu
3. [ ] Menu contains Import, Watch History, All Videos, Settings links
4. [ ] Settings page loads and shows cookies configuration
5. [ ] Home page separates frequency feeds from user feeds
6. [ ] "+ New Feed" button appears at bottom of home
7. [ ] New feed page allows entering name and selecting channels
8. [ ] Creating a new feed works and redirects to the feed
9. [ ] Clicking into a feed sets navigation origin
10. [ ] Watching a video from a feed shows "← FeedName" in header
11. [ ] Clicking origin navigation returns to the feed
12. [ ] Returning to home clears the origin navigation
13. [ ] Bottom sheet shows which feeds channel is already in
14. [ ] Inline feed creation in bottom sheet works

**Step 3: Commit any fixes**

```bash
git add -A
git commit -m "fix: address issues found during testing"
```

---

## Summary

This implementation plan covers:

1. **Navigation store** for tracking origin feed
2. **Hamburger menu** for infrequent actions
3. **Updated header** with contextual "back to feed" navigation
4. **Reorganized home page** with frequency/user feed sections
5. **Settings page** for app configuration
6. **New feed creation flow** with channel picker
7. **Enhanced bottom sheet** showing feed membership and inline creation
8. **Backend API** for channel feed memberships

Total tasks: 14
Estimated implementation: Each task is 5-15 minutes
