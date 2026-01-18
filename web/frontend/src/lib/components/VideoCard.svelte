<script lang="ts">
	import type { Video, WatchProgress, Feed } from '$lib/types';
	import { moveChannel, deleteChannel } from '$lib/api';

	interface Props {
		video: Video;
		progress?: WatchProgress;
		showChannel?: boolean;
		showMoveAction?: boolean;
		showRemoveAction?: boolean;
		availableFeeds?: Feed[];
		onChannelMoved?: () => void;
		onChannelRemoved?: () => void;
		onVideoClick?: () => void;
	}

	let {
		video,
		progress,
		showChannel = true,
		showMoveAction = false,
		showRemoveAction = false,
		availableFeeds = [],
		onChannelMoved,
		onChannelRemoved,
		onVideoClick
	}: Props = $props();

	let showMoveDropdown = $state(false);
	let moving = $state(false);
	let removing = $state(false);

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

	async function handleRemove(e: Event) {
		e.preventDefault();
		e.stopPropagation();
		if (removing) return;
		removing = true;
		try {
			await deleteChannel(video.channel_id);
			onChannelRemoved?.();
		} catch (err) {
			console.error('Failed to remove channel:', err);
			alert('Failed to remove channel');
		} finally {
			removing = false;
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

	let isWatched = $derived(progress ? progressPercent >= 90 : false);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<article
	class="group relative card overflow-hidden {isWatched ? 'opacity-50' : ''}"
	data-video-id={video.id}
	onmouseleave={closeDropdown}
>
	<!-- Thumbnail -->
	<a href="/watch/{video.id}" class="block" onclick={onVideoClick}>
		<div class="video-thumbnail">
			<img
				src={video.thumbnail}
				alt=""
				loading="lazy"
			/>

			<!-- Duration Badge -->
			{#if video.duration}
				<span class="duration-badge z-10">
					{formatDuration(video.duration)}
				</span>
			{/if}

			<!-- Watch Progress Bar -->
			{#if progress && progressPercent > 0 && progressPercent < 100}
				<div class="watch-progress z-10">
					<div class="watch-progress-fill" style="width: {progressPercent}%"></div>
				</div>
			{/if}

			<!-- Hover Play Icon -->
			<div class="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity z-10">
				<div class="w-14 h-14 rounded-full bg-emerald-500/90 flex items-center justify-center shadow-lg shadow-emerald-500/20 transform scale-90 group-hover:scale-100 transition-transform">
					<svg class="w-6 h-6 text-void ml-0.5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M8 5v14l11-7z"/>
					</svg>
				</div>
			</div>
		</div>
	</a>

	<!-- Content - optimized for mobile readability -->
	<div class="p-3 sm:p-3.5">
		<a href="/watch/{video.id}" class="block group/title" onclick={onVideoClick}>
			<h3 class="font-display font-medium text-[0.9375rem] sm:text-sm leading-snug line-clamp-2 text-text-primary group-hover/title:text-emerald-400 transition-colors">
				{video.title}
			</h3>
		</a>

		<div class="mt-2.5 sm:mt-2 flex items-center gap-2 text-[0.8125rem] sm:text-sm">
			{#if showChannel}
				<a
					href="/channels/{video.channel_id}"
					class="text-text-secondary hover:text-emerald-400 transition-colors truncate min-w-0"
				>
					{video.channel_name}
				</a>
				<span class="text-text-dim flex-shrink-0">·</span>
			{/if}
			<span class="text-text-muted whitespace-nowrap flex-shrink-0">{formatRelativeTime(video.published)}</span>
		</div>
	</div>

	<!-- Action Buttons -->
	{#if showRemoveAction}
		<button
			onclick={handleRemove}
			disabled={removing}
			class="absolute top-2 left-2 p-2 rounded-lg bg-void/80 backdrop-blur-sm text-text-secondary hover:text-crimson-400 hover:bg-crimson-500/20 disabled:opacity-50 transition-all opacity-0 group-hover:opacity-100 z-20"
			title="Remove channel from this feed"
		>
			{#if removing}
				<svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
				</svg>
			{:else}
				<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M18 6L6 18M6 6l12 12"/>
				</svg>
			{/if}
		</button>
	{/if}

	{#if showMoveAction && availableFeeds.length > 0}
		<button
			onclick={toggleDropdown}
			class="absolute top-2 right-2 p-2 rounded-lg bg-void/80 backdrop-blur-sm text-text-secondary hover:text-emerald-400 hover:bg-emerald-500/20 transition-all opacity-0 group-hover:opacity-100 z-20"
			title="Move channel to..."
		>
			<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
				<path d="M12 11v6M9 14l3-3 3 3"/>
			</svg>
		</button>

		{#if showMoveDropdown}
			<div class="dropdown top-12 right-2 z-30">
				<div class="dropdown-header">Move to feed</div>
				{#each availableFeeds as feed}
					<button
						onclick={(e) => handleMove(e, feed.id)}
						disabled={moving}
						class="dropdown-item"
					>
						{feed.name}
					</button>
				{/each}
			</div>
		{/if}
	{/if}
</article>
