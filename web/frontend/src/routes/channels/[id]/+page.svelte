<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getChannel, refreshChannel } from '$lib/api';
	import type { Channel, Video, WatchProgress } from '$lib/types';
	import VideoGrid from '$lib/components/VideoGrid.svelte';

	let channel = $state<Channel | null>(null);
	let videos = $state<Video[]>([]);
	let progressMap = $state<Record<string, WatchProgress>>({});
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<string | null>(null);

	let id = $derived(parseInt($page.params.id));

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
</script>

<svelte:head>
	<title>{channel?.name ?? 'Channel'} - Feeds</title>
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
{:else if channel}
	<div class="mb-4 flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold">{channel.name}</h1>
			<a href={channel.url} target="_blank" rel="noopener" class="text-gray-400 text-sm hover:text-blue-400">
				View on YouTube
			</a>
		</div>
		<button
			onclick={handleRefresh}
			disabled={refreshing}
			class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-3 py-1.5 rounded text-sm inline-flex items-center gap-1 flex-shrink-0"
		>
			{#if refreshing}
				<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
			{/if}
			Refresh
		</button>
	</div>

	<VideoGrid {videos} {progressMap} showChannel={false} />
{/if}
