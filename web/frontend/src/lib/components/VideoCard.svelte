<script lang="ts">
	import type { Video, WatchProgress, Feed } from '$lib/types';
	import { moveChannel } from '$lib/api';

	interface Props {
		video: Video;
		progress?: WatchProgress;
		showChannel?: boolean;
		showMoveAction?: boolean;
		availableFeeds?: Feed[];
		onChannelMoved?: () => void;
	}

	let { video, progress, showChannel = true, showMoveAction = false, availableFeeds = [], onChannelMoved }: Props = $props();

	let showMoveDropdown = $state(false);
	let moving = $state(false);

	async function handleMove(e: Event, feedId: number) {
		e.preventDefault();
		e.stopPropagation();
		moving = true;
		try {
			await moveChannel(video.channel_id, feedId);
			showMoveDropdown = false;
			onChannelMoved?.();
		} catch (err) {
			console.error('Failed to move channel:', err);
			alert('Failed to move channel');
		} finally {
			moving = false;
		}
	}

	function toggleDropdown(e: Event) {
		e.preventDefault();
		e.stopPropagation();
		showMoveDropdown = !showMoveDropdown;
	}

	function closeDropdown() {
		showMoveDropdown = false;
	}

	function formatDuration(seconds: number): string {
		if (!seconds) return '';
		const h = Math.floor(seconds / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = seconds % 60;
		if (h > 0) {
			return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
		}
		return `${m}:${s.toString().padStart(2, '0')}`;
	}

	function formatRelativeTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

		if (diffDays === 0) return 'Today';
		if (diffDays === 1) return 'Yesterday';
		if (diffDays < 7) return `${diffDays} days ago`;
		if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`;
		if (diffDays < 365) return `${Math.floor(diffDays / 30)} months ago`;
		return `${Math.floor(diffDays / 365)} years ago`;
	}

	let progressPercent = $derived(
		progress ? Math.round((progress.progress_seconds / progress.duration_seconds) * 100) : 0
	);

	let isWatched = $derived(progress?.completed ?? false);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="relative" onmouseleave={closeDropdown}>
	<a
		href="/watch/{video.id}"
		class="block bg-gray-800 rounded-lg overflow-hidden hover:bg-gray-700 transition-colors {isWatched ? 'opacity-60' : ''}"
	>
		<div class="relative">
			<img
				src={video.thumbnail}
				alt={video.title}
				class="w-full aspect-video object-cover"
				loading="lazy"
			/>
			{#if video.duration}
				<span class="absolute bottom-1 right-1 bg-black/80 text-white text-xs px-1 rounded">
					{formatDuration(video.duration)}
				</span>
			{/if}
			{#if progress && progressPercent > 0 && progressPercent < 100}
				<div class="absolute bottom-0 left-0 right-0 h-1 bg-gray-700">
					<div class="h-full bg-red-600" style="width: {progressPercent}%"></div>
				</div>
			{/if}
		</div>
		<div class="p-3">
			<h3 class="font-medium text-sm line-clamp-2 mb-1">{video.title}</h3>
			{#if showChannel}
				<p class="text-xs text-gray-400">{video.channel_name}</p>
			{/if}
			<p class="text-xs text-gray-500">{formatRelativeTime(video.published)}</p>
		</div>
	</a>

	{#if showMoveAction && availableFeeds.length > 0}
		<button
			onclick={toggleDropdown}
			class="absolute top-2 right-2 bg-black/70 hover:bg-black/90 text-white p-1.5 rounded-full"
			title="Move channel to..."
		>
			<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
			</svg>
		</button>

		{#if showMoveDropdown}
			<div class="absolute top-10 right-2 bg-gray-800 border border-gray-700 rounded-lg shadow-lg z-20 min-w-40">
				<p class="text-xs text-gray-400 px-3 py-2 border-b border-gray-700">Move channel to...</p>
				{#each availableFeeds as feed}
					<button
						onclick={(e) => handleMove(e, feed.id)}
						disabled={moving}
						class="w-full text-left px-3 py-2 text-sm hover:bg-gray-700 disabled:opacity-50"
					>
						{feed.name}
					</button>
				{/each}
			</div>
		{/if}
	{/if}
</div>
