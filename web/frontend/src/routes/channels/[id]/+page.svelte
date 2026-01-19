<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getChannel, refreshChannel, addChannelToFeed, removeChannelFromFeed } from '$lib/api';
	import type { Channel, Video, WatchProgress, Feed } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';

	let channel = $state<Channel | null>(null);
	let videos = $state<Video[]>([]);
	let progressMap = $state<Record<string, WatchProgress>>({});
	let feeds = $state<Feed[]>([]);
	let allFeeds = $state<Feed[]>([]);
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<string | null>(null);
	let showAddDropdown = $state(false);
	let addingToFeed = $state(false);
	let removingFromFeed = $state<number | null>(null);

	let id = $derived(parseInt($page.params.id));
	let scrollRestoreKey = $derived(`channel-${id}-last-video`);
	let availableFeeds = $derived(allFeeds.filter(f => !feeds.some(cf => cf.id === f.id)));

	onMount(async () => {
		await loadChannel();
	});

	async function loadChannel() {
		loading = true;
		error = null;
		try {
			const data = await getChannel(id);
			channel = data.channel;
			videos = data.videos;
			progressMap = data.progressMap || {};
			feeds = data.feeds || [];
			allFeeds = data.allFeeds || [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load channel';
		} finally {
			loading = false;
		}
	}

	async function handleRefresh() {
		refreshing = true;
		try {
			await refreshChannel(id);
			await loadChannel();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to refresh';
		} finally {
			refreshing = false;
		}
	}

	async function handleAddToFeed(feedId: number) {
		addingToFeed = true;
		try {
			const result = await addChannelToFeed(id, feedId);
			feeds = result.feeds;
			showAddDropdown = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add to feed';
		} finally {
			addingToFeed = false;
		}
	}

	async function handleRemoveFromFeed(feedId: number) {
		// Confirm if this is the last feed
		if (feeds.length === 1) {
			if (!confirm('This will delete the channel and all its videos. Continue?')) {
				return;
			}
		}

		removingFromFeed = feedId;
		try {
			await removeChannelFromFeed(feedId, id);
			feeds = feeds.filter(f => f.id !== feedId);

			// If no more feeds, redirect to home
			if (feeds.length === 0) {
				window.location.href = '/';
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to remove from feed';
		} finally {
			removingFromFeed = null;
		}
	}

	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		if (!target.closest('.add-feed-dropdown')) {
			showAddDropdown = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} />

<svelte:head>
	<title>{channel?.name ?? 'Channel'} - Feeds</title>
</svelte:head>

{#if loading}
	<div class="flex flex-col items-center justify-center py-20">
		<div class="w-12 h-12 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin mb-4"></div>
		<p class="text-text-muted font-display">Loading channel...</p>
	</div>
{:else if error}
	<div class="empty-state animate-fade-up" style="opacity: 0;">
		<div class="w-16 h-16 mx-auto mb-4 rounded-full bg-crimson-500/10 flex items-center justify-center">
			<svg class="w-8 h-8 text-crimson-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<circle cx="12" cy="12" r="10"/>
				<line x1="12" y1="8" x2="12" y2="12"/>
				<line x1="12" y1="16" x2="12.01" y2="16"/>
			</svg>
		</div>
		<p class="text-crimson-400 mb-4">{error}</p>
		<a href="/" class="btn btn-secondary">Go back home</a>
	</div>
{:else if channel}
	<!-- Channel Header -->
	<header class="mb-6 animate-fade-up" style="opacity: 0;">
		<div class="flex items-start justify-between gap-4">
			<div class="flex items-center gap-4">
				<div class="w-14 h-14 rounded-2xl bg-gradient-to-br from-emerald-500/20 to-crimson-500/20 flex items-center justify-center border border-white/5">
					<svg class="w-7 h-7 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
						<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
						<circle cx="9" cy="7" r="4"/>
						<path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
						<path d="M16 3.13a4 4 0 0 1 0 7.75"/>
					</svg>
				</div>
				<div>
					<h1 class="text-2xl font-display font-bold mb-1">{channel.name}</h1>
					<div class="flex items-center gap-3 text-sm">
						<span class="text-text-muted">{videos.length} videos</span>
						<span class="text-text-dim">·</span>
						<a
							href={channel.url}
							target="_blank"
							rel="noopener"
							class="text-text-secondary hover:text-emerald-400 transition-colors inline-flex items-center gap-1"
						>
							View on YouTube
							<svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
								<polyline points="15 3 21 3 21 9"/>
								<line x1="10" y1="14" x2="21" y2="3"/>
							</svg>
						</a>
					</div>
				</div>
			</div>

			<button
				onclick={handleRefresh}
				disabled={refreshing}
				class="btn btn-primary shrink-0"
			>
				{#if refreshing}
					<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
					</svg>
					Refreshing...
				{:else}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<polyline points="23 4 23 10 17 10"/>
						<polyline points="1 20 1 14 7 14"/>
						<path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
					</svg>
					Refresh
				{/if}
			</button>
		</div>

		<!-- Feed Chips -->
		<div class="mt-4 flex flex-wrap items-center gap-2">
			<span class="text-sm text-text-muted">Feeds:</span>
			{#each feeds as feed (feed.id)}
				<span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-surface border border-white/5 text-sm">
					<a href="/feeds/{feed.id}" class="hover:text-emerald-400 transition-colors">
						{feed.name}
					</a>
					<button
						onclick={() => handleRemoveFromFeed(feed.id)}
						disabled={removingFromFeed === feed.id}
						class="text-text-muted hover:text-crimson-400 transition-colors"
						title="Remove from this feed"
					>
						{#if removingFromFeed === feed.id}
							<svg class="w-3.5 h-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
							</svg>
						{:else}
							<svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M18 6L6 18M6 6l12 12"/>
							</svg>
						{/if}
					</button>
				</span>
			{/each}

			<!-- Add to Feed Button -->
			<div class="relative add-feed-dropdown">
				<button
					onclick={() => showAddDropdown = !showAddDropdown}
					disabled={availableFeeds.length === 0}
					class="inline-flex items-center gap-1 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-sm text-emerald-400 hover:bg-emerald-500/20 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
				>
					<svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M12 5v14M5 12h14"/>
					</svg>
					Add to feed
				</button>

				{#if showAddDropdown && availableFeeds.length > 0}
					<div class="absolute top-full left-0 mt-1 w-48 bg-surface border border-white/10 rounded-lg shadow-xl z-50">
						{#each availableFeeds as feed (feed.id)}
							<button
								onclick={() => handleAddToFeed(feed.id)}
								disabled={addingToFeed}
								class="w-full px-4 py-2 text-left text-sm hover:bg-white/5 transition-colors first:rounded-t-lg last:rounded-b-lg"
							>
								{feed.name}
							</button>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	</header>

	<!-- Videos Grid -->
	{#if videos.length === 0}
		<div class="empty-state animate-fade-up stagger-1" style="opacity: 0;">
			<div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-surface flex items-center justify-center">
				<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
					<rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18"/>
					<path d="M10 8l6 4-6 4V8z"/>
				</svg>
			</div>
			<p class="empty-state-title">No videos yet</p>
			<p class="empty-state-text">Try refreshing to fetch new videos</p>
		</div>
	{:else}
		<div class="animate-fade-up stagger-1" style="opacity: 0;">
			<VideoGrid {videos} {progressMap} showChannel={false} {scrollRestoreKey} />
		</div>
	{/if}
{/if}
