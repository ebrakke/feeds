<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getFeeds } from '$lib/api';
	import type { Feed } from '$lib/types';

	let feeds = $state<Feed[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let videoUrl = $state('');
	let opening = $state(false);

	onMount(async () => {
		try {
			feeds = await getFeeds();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load feeds';
		} finally {
			loading = false;
		}
	});

	async function openVideo(e: Event) {
		e.preventDefault();
		if (!videoUrl.trim()) return;

		opening = true;
		// Extract video ID from various YouTube URL formats
		const match = videoUrl.match(/(?:v=|youtu\.be\/|shorts\/)([a-zA-Z0-9_-]{11})/);
		if (match) {
			goto(`/watch/${match[1]}`);
		} else {
			alert('Invalid YouTube URL');
			opening = false;
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}
</script>

<svelte:head>
	<title>Home - Feeds</title>
</svelte:head>

<!-- Open video URL -->
<form onsubmit={openVideo} class="mb-4">
	<div class="flex gap-2">
		<input
			type="text"
			bind:value={videoUrl}
			placeholder="Paste YouTube URL to watch..."
			class="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
		/>
		<button
			type="submit"
			disabled={opening}
			class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded-lg inline-flex items-center gap-2"
		>
			{#if opening}
				<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<span>Opening...</span>
			{:else}
				<span>Open</span>
			{/if}
		</button>
	</div>
</form>

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
{:else if feeds.length === 0}
	<div class="text-center py-12">
		<p class="text-gray-400 mb-4">No feeds yet.</p>
		<a href="/import" class="inline-block bg-blue-600 hover:bg-blue-700 text-white py-3 px-6 rounded">
			Import Feed
		</a>
	</div>
{:else}
	{@const inboxFeed = feeds.find(f => f.is_system)}
	{@const regularFeeds = feeds.filter(f => !f.is_system)}

	<!-- Inbox (triage) -->
	{#if inboxFeed}
		<a href="/feeds/{inboxFeed.id}" class="block bg-amber-900/40 border border-amber-700/50 rounded-lg p-4 mb-4 hover:bg-amber-900/60">
			<h3 class="font-semibold text-amber-200">Inbox</h3>
			<p class="text-sm text-amber-200/70">New channels to organize</p>
		</a>
	{/if}

	<!-- Everything feed link -->
	<a href="/all" class="block bg-gradient-to-r from-blue-600 to-purple-600 rounded-lg p-4 mb-4 hover:from-blue-500 hover:to-purple-500">
		<h3 class="font-semibold">Everything</h3>
		<p class="text-sm text-blue-100">Recent videos from all feeds</p>
	</a>

	<div class="space-y-2">
		{#each regularFeeds as feed}
			<a href="/feeds/{feed.id}" class="block bg-gray-800 rounded-lg p-4 hover:bg-gray-700">
				<h3 class="font-semibold">{feed.name}</h3>
				<p class="text-sm text-gray-400">Updated {formatDate(feed.updated_at)}</p>
			</a>
		{/each}
	</div>
{/if}
