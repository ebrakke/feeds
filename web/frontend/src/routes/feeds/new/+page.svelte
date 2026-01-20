<script lang="ts">
  import { goto } from '$app/navigation';
  import { getFeeds } from '$lib/api';
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

  const ICONS = ['🎮', '🎵', '📱', '🎬', '📚', '🔬', '💻', '🎨', '🍳', '⚽', '🎯', '🌍', '💰', '🏠', '🚗', '✈️'];

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
            allChannels = [...allChannels, { channel, feedId: feed.id, feedName: feed.name }];
          }
        }
      }
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
      const res = await fetch('/api/feeds', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: feedName.trim() })
      });
      const feed = await res.json();

      // Add selected channels to the feed
      for (const channelId of selectedChannelIds) {
        await fetch(`/api/channels/${channelId}/feeds`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ feed_id: feed.id })
        });
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
