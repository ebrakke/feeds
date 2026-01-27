<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import { getSmartFeed } from '$lib/api';
	import type { Video, WatchProgress } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';
	import { navigationOrigin } from '$lib/stores/navigation';

	const PAGE_SIZE = 50;

	let name = $state('');
	let videos = $state<Video[]>([]);
	let progressMap = $state<Record<string, WatchProgress>>({});
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state<string | null>(null);
	let total = $state(0);

	let slug = $derived($page.params.slug!);
	let hasMore = $derived(videos.length < total);

	// Empty state messages
	let emptyTitle = $derived(
		slug === 'continue-watching'
			? 'No videos in progress'
			: 'No hot channels this week'
	);
	let emptyMessage = $derived(
		slug === 'continue-watching'
			? 'Videos you start watching will appear here'
			: 'Watch some videos to see your hot channels here'
	);

	onMount(async () => {
		await loadFeed();
	});

	async function loadFeed() {
		loading = true;
		error = null;
		try {
			const data = await getSmartFeed(slug, PAGE_SIZE, 0);
			name = data.name;
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
			const data = await getSmartFeed(slug, PAGE_SIZE, videos.length);
			videos = [...videos, ...(data.videos || [])];
			progressMap = { ...progressMap, ...data.progressMap };
			total = data.total || total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load more videos';
		} finally {
			loadingMore = false;
		}
	}

	// Infinite scroll
	function handleScroll() {
		if (!browser || loadingMore || !hasMore) return;

		const scrollHeight = document.documentElement.scrollHeight;
		const scrollTop = window.scrollY;
		const clientHeight = window.innerHeight;

		if (scrollHeight - scrollTop - clientHeight < 500) {
			loadMore();
		}
	}

	$effect(() => {
		if (browser && !loading) {
			window.addEventListener('scroll', handleScroll);
			return () => window.removeEventListener('scroll', handleScroll);
		}
	});

	// Set navigation origin
	$effect(() => {
		if (name) {
			navigationOrigin.setOrigin({
				feedId: 0,
				feedName: name,
				path: `/smart/${slug}`
			});
		}
	});
</script>

<svelte:head>
	<title>{name || 'Smart Feed'} - Feeds</title>
</svelte:head>

{#if loading}
	<div class="flex flex-col items-center justify-center py-20">
		<div class="w-12 h-12 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin mb-4"></div>
		<p class="text-text-muted font-display">Loading...</p>
	</div>
{:else if error}
	<div class="empty-state animate-fade-up" style="opacity: 0;">
		<svg class="empty-state-icon mx-auto" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
			<circle cx="12" cy="12" r="10"/>
			<line x1="12" y1="8" x2="12" y2="12"/>
			<line x1="12" y1="16" x2="12.01" y2="16"/>
		</svg>
		<p class="text-crimson-400 mb-2">{error}</p>
		<button onclick={() => loadFeed()} class="btn btn-secondary btn-sm">
			Try Again
		</button>
	</div>
{:else}
	<!-- Header -->
	<div class="flex items-center justify-between mb-6">
		<div class="flex items-center gap-3">
			<a href="/" class="p-2 -ml-2 rounded-lg hover:bg-elevated transition-colors" aria-label="Back to home">
				<svg class="w-5 h-5 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
				</svg>
			</a>
			<div>
				<h1 class="text-xl font-semibold text-text-primary">{name}</h1>
				<p class="text-sm text-text-muted">{total} video{total === 1 ? '' : 's'}</p>
			</div>
		</div>
	</div>

	{#if videos.length === 0}
		<div class="empty-state animate-fade-up" style="opacity: 0;">
			<div class="w-20 h-20 mx-auto mb-6 rounded-2xl bg-surface flex items-center justify-center">
				{#if slug === 'continue-watching'}
					<svg class="w-10 h-10 text-text-muted" fill="currentColor" viewBox="0 0 24 24">
						<path d="M8 5v14l11-7z"/>
					</svg>
				{:else}
					<svg class="w-10 h-10 text-text-muted" fill="currentColor" viewBox="0 0 24 24">
						<path d="M12 23c-3.866 0-7-3.358-7-7.5 0-2.84 1.5-5.5 3-7.5 1.5 2 3 3 5 3s3.5-1 5-3c1.5 2 3 4.66 3 7.5 0 4.142-3.134 7.5-7 7.5zm0-5c-1.657 0-3 1.343-3 3s1.343 3 3 3 3-1.343 3-3-1.343-3-3-3z"/>
					</svg>
				{/if}
			</div>
			<h2 class="empty-state-title">{emptyTitle}</h2>
			<p class="empty-state-text">{emptyMessage}</p>
		</div>
	{:else}
		<VideoGrid {videos} {progressMap} />

		{#if loadingMore}
			<div class="flex justify-center py-8">
				<div class="w-8 h-8 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin"></div>
			</div>
		{/if}
	{/if}
{/if}
