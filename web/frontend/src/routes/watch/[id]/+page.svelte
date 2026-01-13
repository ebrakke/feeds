<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { getVideoInfo, updateProgress, getFeeds, addChannel } from '$lib/api';
	import type { Feed } from '$lib/types';

	let videoId = $derived($page.params.id);

	let title = $state('');
	let channelName = $state('');
	let channelURL = $state('');
	let streamURL = $state('');
	let existingChannelID = $state(0);
	let feeds = $state<Feed[]>([]);

	let loading = $state(true);
	let error = $state<string | null>(null);
	let subscribing = $state(false);
	let subscribed = $state(false);
	let selectedFeedId = $state<string>('');

	let player: HTMLVideoElement | null = null;
	let lastSavedTime = 0;

	onMount(async () => {
		// Load feeds
		try {
			feeds = await getFeeds();
			// Pre-select Inbox if it exists
			const inbox = feeds.find(f => f.is_system);
			if (inbox) {
				selectedFeedId = inbox.id.toString();
			}
		} catch (e) {
			console.warn('Failed to load feeds:', e);
		}

		// Load video info
		try {
			const data = await getVideoInfo(videoId);
			title = data.title;
			channelName = data.channel;
			channelURL = data.channelURL;
			streamURL = data.streamURL;
			existingChannelID = data.existingChannelID;
			if (existingChannelID > 0) {
				subscribed = true;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load video';
		} finally {
			loading = false;
		}
	});

	onDestroy(() => {
		if (player) {
			saveProgress();
			player.pause();
		}
	});

	function handleVideoLoaded() {
		if (player) {
			player.play().catch(() => {});
		}
	}

	function saveProgress() {
		if (!player) return;
		const currentTime = Math.floor(player.currentTime);
		const duration = Math.floor(player.duration) || 0;

		if (Math.abs(currentTime - lastSavedTime) >= 5 && duration > 0) {
			lastSavedTime = currentTime;
			updateProgress(videoId, currentTime, duration).catch(() => {});
		}
	}

	function handleTimeUpdate() {
		saveProgress();
	}

	function handlePause() {
		if (!player) return;
		const currentTime = Math.floor(player.currentTime);
		const duration = Math.floor(player.duration) || 0;
		if (duration > 0) {
			updateProgress(videoId, currentTime, duration).catch(() => {});
		}
	}

	async function handleSubscribe() {
		if (!selectedFeedId || !channelURL) return;

		subscribing = true;
		try {
			await addChannel(parseInt(selectedFeedId), channelURL);
			subscribed = true;
		} catch (e) {
			console.error('Failed to subscribe:', e);
			alert(e instanceof Error ? e.message : 'Failed to subscribe');
		} finally {
			subscribing = false;
		}
	}
</script>

<svelte:head>
	<title>{title || 'Watch'} - Feeds</title>
</svelte:head>

<div class="max-w-4xl mx-auto">
	<!-- Video container -->
	<div class="aspect-video bg-black rounded-lg overflow-hidden mb-4 relative">
		{#if loading}
			<div class="absolute inset-0 flex items-center justify-center bg-gray-900">
				<div class="text-center">
					<svg class="animate-spin h-10 w-10 text-blue-500 mx-auto mb-3" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
					<p class="text-gray-400 text-sm">Loading video...</p>
				</div>
			</div>
		{:else if error}
			<div class="absolute inset-0 flex items-center justify-center bg-gray-900">
				<div class="text-center px-4">
					<svg class="h-10 w-10 text-red-500 mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
					</svg>
					<p class="text-red-400 mb-2">{error}</p>
					<a
						href="https://www.youtube.com/watch?v={videoId}"
						target="_blank"
						rel="noopener"
						class="text-blue-400 hover:text-blue-300 text-sm"
					>
						Watch on YouTube instead
					</a>
				</div>
			</div>
		{:else}
			<!-- svelte-ignore a11y_media_has_caption -->
			<video
				bind:this={player}
				class="w-full h-full"
				controls
				playsinline
				src={streamURL}
				onloadeddata={handleVideoLoaded}
				ontimeupdate={handleTimeUpdate}
				onpause={handlePause}
			>
				Your browser does not support the video tag.
			</video>
		{/if}
	</div>

	<!-- Title -->
	<h1 class="text-lg font-bold mb-2">
		{#if loading}
			<span class="inline-block bg-gray-700 rounded h-6 w-64 animate-pulse"></span>
		{:else}
			{title}
		{/if}
	</h1>

	<div class="flex items-center justify-between mb-4">
		<!-- Channel name -->
		<p class="text-gray-400">
			{#if loading}
				<span class="inline-block bg-gray-700 rounded h-4 w-32 animate-pulse"></span>
			{:else}
				{channelName}
			{/if}
		</p>

		<!-- Subscribe section -->
		<div>
			{#if subscribed}
				<span class="text-green-400 text-sm">Subscribed!</span>
			{:else if channelURL && feeds.length > 0}
				<div class="flex items-center gap-2">
					<select
						bind:value={selectedFeedId}
						class="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm"
					>
						<option value="" disabled>Add to...</option>
						{#each feeds as feed}
							<option value={feed.id.toString()}>
								{feed.is_system ? 'Inbox (default)' : feed.name}
							</option>
						{/each}
					</select>
					<button
						onclick={handleSubscribe}
						disabled={subscribing || !selectedFeedId}
						class="bg-red-600 hover:bg-red-700 disabled:opacity-50 text-white px-3 py-1 rounded text-sm inline-flex items-center gap-1"
					>
						{#if subscribing}
							<svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
						{/if}
						Subscribe
					</button>
				</div>
			{:else if feeds.length === 0 && !loading}
				<a href="/import" class="text-sm text-blue-400 hover:text-blue-300">
					Create a feed to subscribe
				</a>
			{/if}
		</div>
	</div>

	<a
		href="https://www.youtube.com/watch?v={videoId}"
		target="_blank"
		rel="noopener"
		class="text-sm text-gray-500 hover:text-blue-400"
	>
		Watch on YouTube
	</a>
</div>
