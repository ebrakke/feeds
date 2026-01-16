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
				// Small delay to ensure DOM is rendered
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
		// Also save on any link click
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
			// Merge progress maps
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

		// Load more when within 500px of bottom
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
</script>

<svelte:head>
	<title>Everything - Feeds</title>
</svelte:head>

<div class="mb-4">
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold">Everything</h1>
			<p class="text-gray-400 text-sm">
				{#if loading}
					Loading...
				{:else}
					{total.toLocaleString()} videos total
					{#if watchedCount > 0}
						&middot; {watchedCount.toLocaleString()} watched
					{/if}
					{#if videos.length < total}
						&middot; {videos.length.toLocaleString()} loaded
					{/if}
				{/if}
			</p>
		</div>
		<div class="flex items-center gap-2">
			<label class="flex items-center gap-2 text-sm text-gray-400 cursor-pointer">
				<input
					type="checkbox"
					bind:checked={hideWatched}
					class="w-4 h-4 rounded border-gray-500 bg-gray-600 text-blue-500 focus:ring-blue-500 focus:ring-offset-0"
				/>
				Hide watched
			</label>
		</div>
	</div>
</div>

{#if loading}
	<div class="flex justify-center py-12">
		<svg class="animate-spin h-8 w-8 text-blue-500" fill="none" viewBox="0 0 24 24">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
		</svg>
	</div>
{:else if error}
	<div class="text-center py-12">
		<p class="text-red-400">{error}</p>
	</div>
{:else}
	<VideoGrid videos={displayVideos} {progressMap} />

	{#if loadingMore}
		<div class="flex justify-center py-8">
			<svg class="animate-spin h-6 w-6 text-blue-500" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
			</svg>
		</div>
	{:else if hasMore}
		<div class="flex justify-center py-8">
			<button
				onclick={loadMore}
				class="bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded-lg text-sm"
			>
				Load more ({total - videos.length} remaining)
			</button>
		</div>
	{/if}
{/if}
