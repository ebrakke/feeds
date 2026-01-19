<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { getRecentVideos } from '$lib/api';
	import type { Video, WatchProgress } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';

	const SCROLL_KEY = 'all-scroll-position';
	const PAGE_SIZE = 100;

	let videos = $state<Video[]>([]);
	let progressMap = $state<Record<string, WatchProgress>>({});
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state<string | null>(null);
	let total = $state(0);
	let offset = $state(0);
	let hideWatched = $state(false);

	// Filter videos based on watched state
	let displayVideos = $derived(
		hideWatched ? videos.filter(v => !progressMap[v.id]) : videos
	);

	// Stats for display
	let watchedCount = $derived(
		videos.filter(v => progressMap[v.id]).length
	);
	let hasMore = $derived(offset + videos.length < total);

	onMount(async () => {
		await loadInitialVideos();

		// Restore scroll position after videos load
		if (browser) {
			const savedPosition = sessionStorage.getItem(SCROLL_KEY);
			if (savedPosition) {
				requestAnimationFrame(() => {
					window.scrollTo(0, parseInt(savedPosition, 10));
				});
			}
		}
	});

	// Save scroll position when navigating away
	function saveScrollPosition() {
		if (browser) {
			sessionStorage.setItem(SCROLL_KEY, String(window.scrollY));
		}
	}

	// Listen for navigation
	if (browser) {
		window.addEventListener('beforeunload', saveScrollPosition);
		document.addEventListener('click', (e) => {
			const target = e.target as HTMLElement;
			if (target.closest('a')) {
				saveScrollPosition();
			}
		});
	}

	onDestroy(() => {
		if (browser) {
			saveScrollPosition();
			window.removeEventListener('beforeunload', saveScrollPosition);
		}
	});

	async function loadInitialVideos() {
		loading = true;
		error = null;
		try {
			const data = await getRecentVideos(PAGE_SIZE, 0);
			videos = data.videos;
			progressMap = data.progressMap || {};
			total = data.total;
			offset = 0;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load videos';
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		if (loadingMore || !hasMore) return;

		loadingMore = true;
		try {
			const newOffset = videos.length;
			const data = await getRecentVideos(PAGE_SIZE, newOffset);
			videos = [...videos, ...data.videos];
			progressMap = { ...progressMap, ...data.progressMap };
			total = data.total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load more videos';
		} finally {
			loadingMore = false;
		}
	}

	// Infinite scroll handler
	function handleScroll() {
		if (!browser || loadingMore || !hasMore) return;

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

	function handleWatchedToggle(videoId: string, watched: boolean) {
		if (watched) {
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
			const { [videoId]: _, ...rest } = progressMap;
			progressMap = rest;
		}
	}
</script>

<svelte:head>
	<title>Everything - Feeds</title>
</svelte:head>

<!-- Header -->
<header class="mb-6 animate-fade-up" style="opacity: 0;">
	<div class="flex items-start justify-between gap-4 flex-wrap">
		<div>
			<h1 class="text-2xl font-display font-bold mb-1">Everything</h1>
			<p class="text-text-muted text-sm">
				{#if loading}
					Loading...
				{:else}
					{total.toLocaleString()} videos
					{#if watchedCount > 0}
						<span class="text-text-dim mx-1">·</span>
						{watchedCount.toLocaleString()} watched
					{/if}
					{#if videos.length < total}
						<span class="text-text-dim mx-1">·</span>
						{videos.length.toLocaleString()} loaded
					{/if}
				{/if}
			</p>
		</div>

		<label class="flex items-center gap-2 text-sm text-text-secondary cursor-pointer">
			<input
				type="checkbox"
				bind:checked={hideWatched}
				class="checkbox"
			/>
			Hide watched
		</label>
	</div>
</header>

{#if loading}
	<div class="flex flex-col items-center justify-center py-20">
		<div class="w-12 h-12 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin mb-4"></div>
		<p class="text-text-muted font-display">Loading videos...</p>
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
		<p class="text-crimson-400">{error}</p>
	</div>
{:else}
	<div class="animate-fade-up stagger-1" style="opacity: 0;">
		<VideoGrid videos={displayVideos} {progressMap} onWatchedToggle={handleWatchedToggle} />
	</div>

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
