<script lang="ts">
	import { onMount } from 'svelte';
	import { getRecentVideos } from '$lib/api';
	import type { Video, WatchProgress } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';

	let videos = $state<Video[]>([]);
	let progressMap = $state<Record<string, WatchProgress>>({});
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			const data = await getRecentVideos();
			videos = data.videos;
			progressMap = data.progressMap || {};
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load videos';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Everything - Feeds</title>
</svelte:head>

<div class="mb-4">
	<h1 class="text-2xl font-bold">Everything</h1>
	<p class="text-gray-400 text-sm">Recent videos from all feeds</p>
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
	<VideoGrid {videos} {progressMap} />
{/if}
