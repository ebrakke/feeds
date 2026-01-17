<script lang="ts">
	import { onMount } from 'svelte';
	import { getHistory } from '$lib/api';
	import type { Video, WatchProgress } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';

	let videos = $state<Video[]>([]);
	let progressMap = $state<Record<string, WatchProgress>>({});
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			const data = await getHistory();
			videos = data.videos;
			progressMap = data.progressMap || {};
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load history';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>History - Feeds</title>
</svelte:head>

<!-- Header -->
<header class="mb-6 animate-fade-up" style="opacity: 0;">
	<div class="flex items-center gap-3 mb-1">
		<div class="w-10 h-10 rounded-xl bg-crimson-500/10 flex items-center justify-center">
			<svg class="w-5 h-5 text-crimson-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<circle cx="12" cy="12" r="10"/>
				<polyline points="12 6 12 12 16 14"/>
			</svg>
		</div>
		<div>
			<h1 class="text-2xl font-display font-bold">Watch History</h1>
			<p class="text-text-muted text-sm">
				{#if loading}
					Loading...
				{:else}
					{videos.length.toLocaleString()} videos watched
				{/if}
			</p>
		</div>
	</div>
</header>

{#if loading}
	<div class="flex flex-col items-center justify-center py-20">
		<div class="w-12 h-12 rounded-full border-2 border-amber-500/20 border-t-amber-500 animate-spin mb-4"></div>
		<p class="text-text-muted font-display">Loading history...</p>
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
{:else if videos.length === 0}
	<div class="empty-state animate-fade-up" style="opacity: 0;">
		<div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-surface flex items-center justify-center">
			<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
				<circle cx="12" cy="12" r="10"/>
				<polyline points="12 6 12 12 16 14"/>
			</svg>
		</div>
		<p class="empty-state-title">No watch history yet</p>
		<p class="empty-state-text">Videos you watch will appear here</p>
	</div>
{:else}
	<div class="animate-fade-up stagger-1" style="opacity: 0;">
		<VideoGrid {videos} {progressMap} />
	</div>
{/if}
