<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getFeed, deleteFeed, refreshFeed, deleteChannel } from '$lib/api';
	import type { Feed, Channel, Video, WatchProgress } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';

	let feed = $state<Feed | null>(null);
	let channels = $state<Channel[]>([]);
	let videos = $state<Video[]>([]);
	let progressMap = $state<Record<string, WatchProgress>>({});
	let allFeeds = $state<Feed[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let refreshing = $state(false);
	let showChannels = $state(false);

	let id = $derived(parseInt($page.params.id));

	// Inbox-specific behavior
	let isInbox = $derived(feed?.is_system === true);
	let moveTargetFeeds = $derived(
		allFeeds.filter(f => f.id !== feed?.id && !f.is_system)
	);

	onMount(async () => {
		await loadFeed();
	});

	async function loadFeed() {
		loading = true;
		error = null;
		try {
			const data = await getFeed(id);
			feed = data.feed;
			channels = data.channels;
			videos = data.videos;
			progressMap = data.progressMap || {};
			allFeeds = data.allFeeds;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load feed';
		} finally {
			loading = false;
		}
	}

	async function handleRefresh() {
		refreshing = true;
		try {
			const result = await refreshFeed(id);
			await loadFeed();
			if (result.errors.length > 0) {
				console.warn('Some channels failed:', result.errors);
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to refresh';
		} finally {
			refreshing = false;
		}
	}

	async function handleDelete() {
		if (!confirm(`Delete "${feed?.name}"? This cannot be undone.`)) return;
		try {
			await deleteFeed(id);
			goto('/');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete feed';
		}
	}

	async function handleDeleteChannel(channelId: number, channelName: string) {
		if (!confirm(`Remove "${channelName}" from this feed?`)) return;
		try {
			await deleteChannel(channelId);
			await loadFeed();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete channel';
		}
	}
</script>

<svelte:head>
	<title>{feed?.name ?? 'Feed'} - Feeds</title>
</svelte:head>

{#if loading}
	<div class="flex justify-center py-12">
		<svg class="animate-spin h-8 w-8 text-blue-500" fill="none" viewBox="0 0 24 24">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
		</svg>
	</div>
{:else if error}
	<div class="text-center py-12">
		<p class="text-red-400 mb-4">{error}</p>
		<a href="/" class="text-blue-400 hover:underline">Go back home</a>
	</div>
{:else if feed}
	<div class="mb-4 flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold">{feed.name}</h1>
			{#if feed.description}
				<p class="text-gray-400 text-sm mt-1">{feed.description}</p>
			{/if}
			<p class="text-gray-500 text-xs mt-1">{channels.length} channels</p>
		</div>
		<div class="flex gap-2 flex-shrink-0">
			<button
				onclick={handleRefresh}
				disabled={refreshing}
				class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-3 py-1.5 rounded text-sm inline-flex items-center gap-1"
			>
				{#if refreshing}
					<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				{/if}
				Refresh
			</button>
			<button
				onclick={() => showChannels = !showChannels}
				class="bg-gray-700 hover:bg-gray-600 text-white px-3 py-1.5 rounded text-sm"
			>
				{showChannels ? 'Hide' : 'Show'} Channels
			</button>
			{#if !isInbox}
				<button
					onclick={handleDelete}
					class="bg-red-600 hover:bg-red-700 text-white px-3 py-1.5 rounded text-sm"
				>
					Delete
				</button>
			{/if}
		</div>
	</div>

	{#if isInbox && videos.length === 0 && channels.length === 0}
		<div class="text-center py-12 text-gray-400">
			<p class="mb-2">Your inbox is empty!</p>
			<p class="text-sm">New channels you subscribe to will appear here for triage.</p>
		</div>
	{/if}

	{#if showChannels}
		<div class="mb-6 bg-gray-800 rounded-lg p-4">
			<h2 class="font-semibold mb-3">Channels ({channels.length})</h2>
			<div class="space-y-2">
				{#each channels as channel}
					<div class="flex items-center justify-between bg-gray-700 rounded px-3 py-2">
						<a href="/channels/{channel.id}" class="hover:text-blue-400">
							{channel.name}
						</a>
						<button
							onclick={() => handleDeleteChannel(channel.id, channel.name)}
							class="text-red-400 hover:text-red-300 text-sm"
						>
							Remove
						</button>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	<VideoGrid
		{videos}
		{progressMap}
		showMoveAction={isInbox}
		availableFeeds={moveTargetFeeds}
		onChannelMoved={loadFeed}
	/>
{/if}
