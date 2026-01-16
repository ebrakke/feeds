<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getFeed, deleteFeed, deleteChannel } from '$lib/api';
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
	let refreshProgress = $state<{ current: number; total: number; channel: string } | null>(null);
	let activeTab = $state<'videos' | 'shorts' | 'channels'>('videos');
	let selectedChannels = $state<Set<number>>(new Set());

	// Filter shorts (videos under 90 seconds with known duration)
	let shortsVideos = $derived(videos.filter(v => v.duration > 0 && v.duration < 90));
	let regularVideos = $derived(videos.filter(v => v.duration === 0 || v.duration >= 90));
	let deletingChannels = $state<Set<number>>(new Set());

	let id = $derived(parseInt($page.params.id));
	let allSelected = $derived(channels.length > 0 && selectedChannels.size === channels.length);

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
			channels = data.channels || [];
			videos = data.videos || [];
			progressMap = data.progressMap || {};
			allFeeds = data.allFeeds || [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load feed';
		} finally {
			loading = false;
		}
	}

	async function handleRefresh() {
		refreshing = true;
		refreshProgress = null;
		error = null;

		try {
			const eventSource = new EventSource(`/api/feeds/${id}/refresh/stream`);

			eventSource.addEventListener('progress', (e) => {
				const data = JSON.parse(e.data);
				refreshProgress = {
					current: data.current,
					total: data.total,
					channel: data.channel
				};
			});

			eventSource.addEventListener('complete', async (e) => {
				eventSource.close();
				const data = JSON.parse(e.data);
				if (data.errors && data.errors.length > 0) {
					console.warn('Some channels failed:', data.errors);
				}
				await loadFeed();
				refreshing = false;
				refreshProgress = null;
			});

			eventSource.onerror = () => {
				eventSource.close();
				error = 'Connection lost during refresh';
				refreshing = false;
				refreshProgress = null;
			};
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to refresh';
			refreshing = false;
			refreshProgress = null;
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
		deletingChannels.add(channelId);
		deletingChannels = deletingChannels;
		try {
			await deleteChannel(channelId);
			// Remove from local state instead of reloading
			channels = channels.filter(c => c.id !== channelId);
			videos = videos.filter(v => v.channel_id !== channelId);
			selectedChannels.delete(channelId);
			selectedChannels = selectedChannels;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete channel';
		} finally {
			deletingChannels.delete(channelId);
			deletingChannels = deletingChannels;
		}
	}

	function toggleChannel(channelId: number) {
		if (selectedChannels.has(channelId)) {
			selectedChannels.delete(channelId);
		} else {
			selectedChannels.add(channelId);
		}
		selectedChannels = selectedChannels;
	}

	function toggleAllChannels() {
		if (allSelected) {
			selectedChannels = new Set();
		} else {
			selectedChannels = new Set(channels.map(c => c.id));
		}
	}

	async function handleDeleteSelected() {
		if (selectedChannels.size === 0) return;
		const count = selectedChannels.size;
		if (!confirm(`Remove ${count} channel${count > 1 ? 's' : ''} from this feed?`)) return;

		const toDelete = [...selectedChannels];
		for (const channelId of toDelete) {
			deletingChannels.add(channelId);
		}
		deletingChannels = deletingChannels;

		let failed = 0;
		for (const channelId of toDelete) {
			try {
				await deleteChannel(channelId);
				channels = channels.filter(c => c.id !== channelId);
				videos = videos.filter(v => v.channel_id !== channelId);
				selectedChannels.delete(channelId);
			} catch (e) {
				failed++;
			} finally {
				deletingChannels.delete(channelId);
				deletingChannels = deletingChannels;
			}
		}
		selectedChannels = selectedChannels;

		if (failed > 0) {
			error = `Failed to remove ${failed} channel${failed > 1 ? 's' : ''}`;
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
					{#if refreshProgress}
						{refreshProgress.current}/{refreshProgress.total}
					{:else}
						Refreshing...
					{/if}
				{:else}
					Refresh
				{/if}
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

	<!-- Tabs -->
	<div class="border-b border-gray-700 mb-4">
		<nav class="flex gap-4">
			<button
				onclick={() => activeTab = 'videos'}
				class="pb-2 px-1 text-sm font-medium border-b-2 transition-colors {activeTab === 'videos' ? 'border-blue-500 text-blue-400' : 'border-transparent text-gray-400 hover:text-gray-300'}"
			>
				Videos ({regularVideos.length})
			</button>
			{#if shortsVideos.length > 0}
				<button
					onclick={() => activeTab = 'shorts'}
					class="pb-2 px-1 text-sm font-medium border-b-2 transition-colors {activeTab === 'shorts' ? 'border-blue-500 text-blue-400' : 'border-transparent text-gray-400 hover:text-gray-300'}"
				>
					Shorts ({shortsVideos.length})
				</button>
			{/if}
			<button
				onclick={() => activeTab = 'channels'}
				class="pb-2 px-1 text-sm font-medium border-b-2 transition-colors {activeTab === 'channels' ? 'border-blue-500 text-blue-400' : 'border-transparent text-gray-400 hover:text-gray-300'}"
			>
				Channels ({channels.length})
			</button>
		</nav>
	</div>

	{#if refreshProgress}
		<div class="mb-4 bg-gray-800 rounded-lg p-3">
			<div class="flex justify-between text-sm text-gray-400 mb-1">
				<span>Refreshing: {refreshProgress.channel}</span>
				<span>{refreshProgress.current} / {refreshProgress.total}</span>
			</div>
			<div class="w-full bg-gray-700 rounded-full h-2">
				<div
					class="bg-blue-500 h-2 rounded-full transition-all duration-200"
					style="width: {(refreshProgress.current / refreshProgress.total) * 100}%"
				></div>
			</div>
		</div>
	{/if}

	{#if activeTab === 'videos'}
		{#if isInbox && videos.length === 0 && channels.length === 0}
			<div class="text-center py-12 text-gray-400">
				<p class="mb-2">Your inbox is empty!</p>
				<p class="text-sm">New channels you subscribe to will appear here for triage.</p>
			</div>
		{:else}
			<VideoGrid
				videos={regularVideos}
				{progressMap}
				showMoveAction={isInbox}
				availableFeeds={moveTargetFeeds}
				onChannelMoved={loadFeed}
			/>
		{/if}
	{:else if activeTab === 'shorts'}
		<VideoGrid
			videos={shortsVideos}
			{progressMap}
			showMoveAction={isInbox}
			availableFeeds={moveTargetFeeds}
			onChannelMoved={loadFeed}
		/>
	{:else if activeTab === 'channels'}
		<div class="space-y-2">
			{#if channels.length > 0}
				<div class="flex items-center justify-between mb-4">
					<label class="flex items-center gap-3 cursor-pointer">
						<input
							type="checkbox"
							checked={allSelected}
							onchange={toggleAllChannels}
							class="w-4 h-4 rounded border-gray-500 bg-gray-600 text-blue-500 focus:ring-blue-500 focus:ring-offset-0"
						/>
						<span class="text-gray-400 text-sm">Select all</span>
					</label>
					{#if selectedChannels.size > 0}
						<button
							onclick={handleDeleteSelected}
							class="bg-red-600 hover:bg-red-700 text-white px-3 py-1.5 rounded text-sm"
						>
							Remove {selectedChannels.size} selected
						</button>
					{/if}
				</div>
				{#each channels as channel}
					<div class="flex items-center gap-3 bg-gray-800 hover:bg-gray-750 rounded-lg px-4 py-3 {deletingChannels.has(channel.id) ? 'opacity-50' : ''}">
						<input
							type="checkbox"
							checked={selectedChannels.has(channel.id)}
							onchange={() => toggleChannel(channel.id)}
							disabled={deletingChannels.has(channel.id)}
							class="w-4 h-4 rounded border-gray-500 bg-gray-600 text-blue-500 focus:ring-blue-500 focus:ring-offset-0"
						/>
						<a href="/channels/{channel.id}" class="hover:text-blue-400 flex-1">
							{channel.name}
						</a>
						<button
							onclick={() => handleDeleteChannel(channel.id, channel.name)}
							disabled={deletingChannels.has(channel.id)}
							class="text-red-400 hover:text-red-300 text-sm disabled:opacity-50"
						>
							{deletingChannels.has(channel.id) ? 'Removing...' : 'Remove'}
						</button>
					</div>
				{/each}
			{:else}
				<div class="text-center py-12 text-gray-400">
					<p>No channels in this feed yet.</p>
				</div>
			{/if}
		</div>
	{/if}
{/if}
