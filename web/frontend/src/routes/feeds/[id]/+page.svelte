<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getFeed, deleteFeed, removeChannelFromFeed, getShuffledVideos } from '$lib/api';
	import type { Feed, Channel, Video, WatchProgress } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';

	const PAGE_SIZE = 100;

	let feed = $state<Feed | null>(null);
	let channels = $state<Channel[]>([]);
	let videos = $state<Video[]>([]);
	let progressMap = $state<Record<string, WatchProgress>>({});
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state<string | null>(null);
	let refreshing = $state(false);
	let refreshProgress = $state<{ current: number; total: number; channel: string } | null>(null);
	let activeTab = $state<'videos' | 'shorts' | 'shuffle' | 'channels'>('videos');
	let selectedChannels = $state<Set<number>>(new Set());
	let shuffledVideos = $state<Video[]>([]);
	let shuffleTotal = $state(0);
	let shuffleLoading = $state(false);
	let shuffleLoaded = $state(false);
	let total = $state(0);
	let hideWatched = $state(false);

	// Filter shorts - use is_short flag if available, fall back to duration heuristic
	let shortsVideos = $derived(videos.filter(v =>
		v.is_short === true || (v.is_short === null && v.duration > 0 && v.duration < 90)
	));
	let regularVideos = $derived(videos.filter(v =>
		v.is_short === false || (v.is_short === null && (v.duration === 0 || v.duration >= 90))
	));
	let deletingChannels = $state<Set<number>>(new Set());

	// Filter by watched status
	let displayRegularVideos = $derived(
		hideWatched ? regularVideos.filter(v => !progressMap[v.id]) : regularVideos
	);
	let displayShortsVideos = $derived(
		hideWatched ? shortsVideos.filter(v => !progressMap[v.id]) : shortsVideos
	);

	// Stats
	let watchedCount = $derived(videos.filter(v => progressMap[v.id]).length);
	let hasMore = $derived(videos.length < total);

	let id = $derived(parseInt($page.params.id));
	let scrollRestoreKey = $derived(`feed-${id}-last-video`);
	let allSelected = $derived(channels.length > 0 && selectedChannels.size === channels.length);

	// Inbox-specific behavior
	let isInbox = $derived(feed?.is_system === true);

	onMount(async () => {
		await loadFeed();
	});

	async function loadFeed() {
		loading = true;
		error = null;
		try {
			const data = await getFeed(id, PAGE_SIZE, 0);
			feed = data.feed;
			channels = data.channels || [];
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
			const newOffset = videos.length;
			const data = await getFeed(id, PAGE_SIZE, newOffset);
			videos = [...videos, ...(data.videos || [])];
			progressMap = { ...progressMap, ...data.progressMap };
			total = data.total || total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load more videos';
		} finally {
			loadingMore = false;
		}
	}

	async function loadShuffledVideos() {
		if (shuffleLoading) return;

		shuffleLoading = true;
		try {
			const data = await getShuffledVideos(id, PAGE_SIZE, 0);
			shuffledVideos = data.videos || [];
			shuffleTotal = data.total || 0;
			shuffleLoaded = true;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load shuffled videos';
		} finally {
			shuffleLoading = false;
		}
	}

	async function reshuffle() {
		shuffleLoaded = false;
		await loadShuffledVideos();
	}

	function switchTab(tab: typeof activeTab) {
		activeTab = tab;
		if (tab === 'shuffle' && !shuffleLoaded && !shuffleLoading) {
			loadShuffledVideos();
		}
	}

	// Infinite scroll handler
	function handleScroll() {
		if (!browser || loadingMore || !hasMore || activeTab === 'channels') return;

		const scrollHeight = document.documentElement.scrollHeight;
		const scrollTop = window.scrollY;
		const clientHeight = window.innerHeight;

		if (scrollHeight - scrollTop - clientHeight < 500) {
			loadMore();
		}
	}

	// Set up scroll listener
	$effect(() => {
		if (browser && !loading) {
			window.addEventListener('scroll', handleScroll);
			return () => window.removeEventListener('scroll', handleScroll);
		}
	});

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
			await removeChannelFromFeed(id, channelId);
			channels = channels.filter(c => c.id !== channelId);
			videos = videos.filter(v => v.channel_id !== channelId);
			selectedChannels.delete(channelId);
			selectedChannels = selectedChannels;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to remove channel from feed';
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
				await removeChannelFromFeed(id, channelId);
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

	function handleChannelRemovedFromFeed(channelId: number) {
		// Remove all videos from this channel from the view
		videos = videos.filter(v => v.channel_id !== channelId);
		// Remove from channels list
		channels = channels.filter(c => c.id !== channelId);
	}

	function handleWatchedToggle(videoId: string, watched: boolean) {
		if (watched) {
			// Mark as watched - set progress to 100/100
			progressMap = {
				...progressMap,
				[videoId]: {
					video_id: videoId,
					progress_seconds: 100,
					duration_seconds: 100,
					watched_at: new Date().toISOString()
				}
			};
		} else {
			// Mark as unwatched - remove from progress map
			const { [videoId]: _, ...rest } = progressMap;
			progressMap = rest;
		}
	}
</script>

<svelte:head>
	<title>{feed?.name ?? 'Feed'} - Feeds</title>
</svelte:head>

{#if loading}
	<div class="flex flex-col items-center justify-center py-20">
		<div class="w-12 h-12 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin mb-4"></div>
		<p class="text-text-muted font-display">Loading feed...</p>
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
		<a href="/" class="btn btn-secondary btn-sm">Go back home</a>
	</div>
{:else if feed}
	<!-- Header - mobile optimized with stacked layout -->
	<header class="mb-4 sm:mb-6 animate-fade-up" style="opacity: 0;">
		<div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3 sm:gap-4">
			<div class="min-w-0">
				<h1 class="text-xl sm:text-2xl font-display font-bold mb-1 truncate">{feed.name}</h1>
				<p class="text-text-muted text-sm">
					{total.toLocaleString()} videos
					{#if watchedCount > 0}
						<span class="text-text-dim mx-1">·</span>
						{watchedCount.toLocaleString()} watched
					{/if}
					{#if videos.length < total}
						<span class="text-text-dim mx-1">·</span>
						{videos.length.toLocaleString()} loaded
					{/if}
				</p>
			</div>

			<!-- Controls - responsive grid on mobile -->
			<div class="flex flex-wrap items-center gap-2 sm:gap-2">
				<!-- Hide Watched Toggle - touch-friendly -->
				<label class="flex items-center gap-2.5 text-sm text-text-secondary cursor-pointer py-2 px-1 -ml-1 rounded-lg active:bg-elevated">
					<input
						type="checkbox"
						bind:checked={hideWatched}
						class="checkbox"
					/>
					<span class="select-none">Hide watched</span>
				</label>

				<div class="flex items-center gap-2">
					<button
						onclick={handleRefresh}
						disabled={refreshing}
						class="btn btn-primary btn-sm"
					>
						{#if refreshing}
							<svg class="animate-spin h-4 w-4 sm:h-4 sm:w-4" viewBox="0 0 24 24" fill="none">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
							</svg>
							{#if refreshProgress}
								<span class="tabular-nums">{refreshProgress.current}/{refreshProgress.total}</span>
							{:else}
								<span class="hidden sm:inline">Refreshing</span>
							{/if}
						{:else}
							<svg class="w-5 h-5 sm:w-4 sm:h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M23 4v6h-6M1 20v-6h6"/>
								<path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
							</svg>
							<span>Refresh</span>
						{/if}
					</button>

					{#if !isInbox}
						<button
							onclick={handleDelete}
							class="btn btn-danger btn-sm"
						>
							Delete
						</button>
					{/if}
				</div>
			</div>
		</div>
	</header>

	<!-- Tabs - horizontally scrollable on mobile -->
	<nav class="flex gap-4 sm:gap-6 border-b border-border-subtle mb-4 sm:mb-6 -mx-4 px-4 sm:mx-0 sm:px-0 overflow-x-auto scrollbar-none animate-fade-up stagger-1" style="opacity: 0;">
		<button
			onclick={() => switchTab('videos')}
			class="tab {activeTab === 'videos' ? 'active' : ''}"
		>
			Videos
			<span class="ml-1 text-text-dim">
				({displayRegularVideos.length}{hideWatched && regularVideos.length !== displayRegularVideos.length ? `/${regularVideos.length}` : ''})
			</span>
		</button>
		{#if shortsVideos.length > 0}
			<button
				onclick={() => switchTab('shorts')}
				class="tab {activeTab === 'shorts' ? 'active' : ''}"
			>
				Shorts
				<span class="ml-1 text-text-dim">
					({displayShortsVideos.length}{hideWatched && shortsVideos.length !== displayShortsVideos.length ? `/${shortsVideos.length}` : ''})
				</span>
			</button>
		{/if}
		<button
			onclick={() => switchTab('shuffle')}
			class="tab {activeTab === 'shuffle' ? 'active' : ''}"
		>
			Shuffle
			{#if shuffleLoaded}
				<span class="ml-1 text-text-dim">({shuffleTotal})</span>
			{/if}
		</button>
		<button
			onclick={() => switchTab('channels')}
			class="tab {activeTab === 'channels' ? 'active' : ''}"
		>
			Channels
			<span class="ml-1 text-text-dim">({channels.length})</span>
		</button>
	</nav>

	<!-- Refresh Progress -->
	{#if refreshProgress}
		<div class="card-elevated p-4 mb-6 animate-fade-in">
			<div class="flex justify-between text-sm mb-2">
				<span class="text-text-secondary truncate">{refreshProgress.channel}</span>
				<span class="text-text-muted">{refreshProgress.current} / {refreshProgress.total}</span>
			</div>
			<div class="progress-bar">
				<div
					class="progress-bar-fill"
					style="width: {(refreshProgress.current / refreshProgress.total) * 100}%"
				></div>
			</div>
		</div>
	{/if}

	<!-- Tab Content -->
	<div class="animate-fade-up stagger-2" style="opacity: 0;">
		{#if activeTab === 'videos'}
			{#if isInbox && videos.length === 0 && channels.length === 0}
				<div class="empty-state">
					<div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-emerald-500/10 flex items-center justify-center">
						<svg class="w-8 h-8 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
							<polyline points="22,12 16,12 14,15 10,15 8,12 2,12"/>
							<path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>
						</svg>
					</div>
					<h3 class="empty-state-title">Your inbox is empty</h3>
					<p class="empty-state-text">New channels you subscribe to will appear here for triage</p>
				</div>
			{:else}
				<VideoGrid
					videos={displayRegularVideos}
					{progressMap}
					showChannel={true}
					showRemoveFromFeed={true}
					currentFeedId={feed?.id}
					onChannelRemovedFromFeed={handleChannelRemovedFromFeed}
					onWatchedToggle={handleWatchedToggle}
					{scrollRestoreKey}
				/>
			{/if}
		{:else if activeTab === 'shorts'}
			<VideoGrid
				videos={displayShortsVideos}
				{progressMap}
				showChannel={true}
				showRemoveFromFeed={true}
				currentFeedId={feed?.id}
				onChannelRemovedFromFeed={handleChannelRemovedFromFeed}
				onWatchedToggle={handleWatchedToggle}
				scrollRestoreKey={`${scrollRestoreKey}-shorts`}
			/>
		{:else if activeTab === 'shuffle'}
			{#if shuffleLoading}
				<div class="flex flex-col items-center justify-center py-20">
					<div class="w-12 h-12 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin mb-4"></div>
					<p class="text-text-muted font-display">Shuffling videos...</p>
				</div>
			{:else if shuffledVideos.length === 0}
				<div class="empty-state">
					<div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-surface flex items-center justify-center">
						<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
							<path d="M16 3h5v5M4 20L20.2 3.8M21 16v5h-5M15 15l5.1 5.1M4 4l5 5"/>
						</svg>
					</div>
					<h3 class="empty-state-title">No unwatched videos</h3>
					<p class="empty-state-text">Watch some videos to clear the deck, or check back after a refresh</p>
				</div>
			{:else}
				<div class="flex justify-end mb-4">
					<button
						onclick={reshuffle}
						class="btn btn-secondary btn-sm"
					>
						<svg class="w-4 h-4 mr-1.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M16 3h5v5M4 20L20.2 3.8M21 16v5h-5M15 15l5.1 5.1M4 4l5 5"/>
						</svg>
						Reshuffle
					</button>
				</div>
				<VideoGrid
					videos={shuffledVideos}
					{progressMap}
					showRemoveFromFeed={false}
					onWatchedToggle={handleWatchedToggle}
					scrollRestoreKey={`${scrollRestoreKey}-shuffle`}
				/>
			{/if}
		{:else if activeTab === 'channels'}
			{#if channels.length > 0}
				<!-- Bulk Actions -->
				<div class="flex items-center justify-between mb-4">
					<label class="flex items-center gap-3 cursor-pointer">
						<input
							type="checkbox"
							checked={allSelected}
							onchange={toggleAllChannels}
							class="checkbox"
						/>
						<span class="text-sm text-text-secondary">Select all</span>
					</label>
					{#if selectedChannels.size > 0}
						<button
							onclick={handleDeleteSelected}
							class="btn btn-danger btn-sm"
						>
							Remove {selectedChannels.size} selected
						</button>
					{/if}
				</div>

				<!-- Channel List -->
				<div class="space-y-2">
					{#each channels as channel, i}
						<div
							class="channel-item {deletingChannels.has(channel.id) ? 'opacity-50' : ''} animate-fade-up"
							style="opacity: 0; animation-delay: {Math.min(i * 0.03, 0.3)}s;"
						>
							<input
								type="checkbox"
								checked={selectedChannels.has(channel.id)}
								onchange={() => toggleChannel(channel.id)}
								disabled={deletingChannels.has(channel.id)}
								class="checkbox"
							/>
							<a href="/channels/{channel.id}" class="flex-1 hover:text-emerald-400 transition-colors truncate">
								{channel.name}
							</a>
							<button
								onclick={() => handleDeleteChannel(channel.id, channel.name)}
								disabled={deletingChannels.has(channel.id)}
								class="text-sm text-text-muted hover:text-crimson-400 transition-colors disabled:opacity-50"
							>
								{deletingChannels.has(channel.id) ? 'Removing...' : 'Remove'}
							</button>
						</div>
					{/each}
				</div>
			{:else}
				<div class="empty-state">
					<div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-surface flex items-center justify-center">
						<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
							<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
							<circle cx="9" cy="7" r="4"/>
							<path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
							<path d="M16 3.13a4 4 0 0 1 0 7.75"/>
						</svg>
					</div>
					<h3 class="empty-state-title">No channels yet</h3>
					<p class="empty-state-text">Add channels to this feed to see their videos</p>
				</div>
			{/if}
		{/if}
	</div>

	<!-- Load More -->
	{#if activeTab !== 'channels'}
		{#if loadingMore}
			<div class="flex justify-center py-8">
				<div class="w-8 h-8 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin"></div>
			</div>
		{:else if hasMore}
			<div class="flex justify-center py-8">
				<button
					onclick={loadMore}
					class="btn btn-secondary"
				>
					Load more ({total - videos.length} remaining)
				</button>
			</div>
		{/if}
	{/if}
{/if}
