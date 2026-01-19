<script lang="ts">
	import type { Video, WatchProgress, Feed } from '$lib/types';
	import { removeChannelFromFeed, markWatched, markUnwatched, getFeeds, getChannel, addChannelToFeed, createFeed } from '$lib/api';
	import { toast } from '$lib/stores/toast';
	import { bottomSheet } from '$lib/stores/bottomSheet';

	interface Props {
		video: Video;
		progress?: WatchProgress;
		showChannel?: boolean;
		showRemoveFromFeed?: boolean;
		currentFeedId?: number;
		onChannelRemovedFromFeed?: () => void;
		onVideoClick?: () => void;
		onWatchedToggle?: (videoId: string, watched: boolean) => void;
	}

	let {
		video,
		progress,
		showChannel = true,
		showRemoveFromFeed = false,
		currentFeedId,
		onChannelRemovedFromFeed,
		onVideoClick,
		onWatchedToggle
	}: Props = $props();

	let showMenu = $state(false);
	let removingFromFeed = $state(false);
	let togglingWatched = $state(false);
	let showAddToFeedMenu = $state(false);
	let availableFeeds = $state<Feed[]>([]);
	let loadingFeeds = $state(false);
	let addingToFeed = $state(false);

	async function handleRemoveFromFeed(e: Event) {
		e.preventDefault();
		e.stopPropagation();
		if (!currentFeedId || removingFromFeed) return;

		removingFromFeed = true;
		try {
			await removeChannelFromFeed(currentFeedId, video.channel_id);
			showMenu = false;
			onChannelRemovedFromFeed?.();
		} catch (err) {
			console.error('Failed to remove channel from feed:', err);
			alert('Failed to remove channel');
		} finally {
			removingFromFeed = false;
		}
	}

	async function handleToggleWatched(e: Event) {
		e.preventDefault();
		e.stopPropagation();
		if (togglingWatched) return;

		togglingWatched = true;
		try {
			if (isWatched) {
				await markUnwatched(video.id);
				onWatchedToggle?.(video.id, false);
			} else {
				await markWatched(video.id);
				onWatchedToggle?.(video.id, true);
			}
			showMenu = false;
		} catch (err) {
			console.error('Failed to toggle watched status:', err);
			alert('Failed to update watched status');
		} finally {
			togglingWatched = false;
		}
	}

	async function handleOpenAddToFeed(e: Event) {
		e.preventDefault();
		e.stopPropagation();
		if (loadingFeeds || addingToFeed) return;

		// Check if mobile (no hover support = touch device)
		const isMobile = window.matchMedia('(hover: none)').matches;

		loadingFeeds = true;
		try {
			const [feedsResult, channelResult] = await Promise.all([
				getFeeds(),
				getChannel(video.channel_id)
			]);

			const channelFeedIds = new Set(channelResult.feeds.map((f) => f.id));
			availableFeeds = feedsResult.filter(
				(f) => !f.is_system && f.id !== currentFeedId && !channelFeedIds.has(f.id)
			);

			if (isMobile) {
				// Mobile: Show bottom sheet via store (rendered in layout)
				showMenu = false;
				bottomSheet.open({
					title: 'Add to feed',
					channelId: video.channel_id,
					channelName: video.channel_name,
					feeds: availableFeeds
				});
			} else {
				// Desktop: Show submenu
				showAddToFeedMenu = true;
			}
		} catch (err) {
			console.error('Failed to load feeds:', err);
			toast.error('Failed to load feeds');
			showAddToFeedMenu = false;
		} finally {
			loadingFeeds = false;
		}
	}

	async function handleAddToFeed(feed: Feed, e: Event) {
		e.preventDefault();
		e.stopPropagation();
		if (addingToFeed) return;

		addingToFeed = true;
		try {
			await addChannelToFeed(video.channel_id, feed.id);
			toast.success(`Added to ${feed.name}`);
			showMenu = false;
			showAddToFeedMenu = false;
		} catch (err) {
			console.error('Failed to add to feed:', err);
			toast.error('Failed to add to feed');
		} finally {
			addingToFeed = false;
		}
	}

	async function handleCreateNewFeed(e: Event) {
		e.preventDefault();
		e.stopPropagation();

		const name = prompt('Enter feed name:');
		if (!name?.trim()) return;

		addingToFeed = true;
		try {
			const newFeed = await createFeed(name.trim());
			await addChannelToFeed(video.channel_id, newFeed.id);
			toast.success(`Added to ${newFeed.name}`);
			showMenu = false;
			showAddToFeedMenu = false;
		} catch (err) {
			console.error('Failed to create feed:', err);
			toast.error('Failed to create feed');
		} finally {
			addingToFeed = false;
		}
	}

	function toggleMenu(e: Event) {
		e.preventDefault();
		e.stopPropagation();
		showMenu = !showMenu;
	}

	function closeMenu() {
		showMenu = false;
		showAddToFeedMenu = false;
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
	onmouseleave={closeMenu}
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

	<!-- Action Menu -->
	<button
		onclick={toggleMenu}
		class="absolute top-2 right-2 p-2 rounded-lg bg-void/80 backdrop-blur-sm text-text-secondary hover:text-text-primary hover:bg-white/10 transition-all sm:opacity-0 sm:group-hover:opacity-100 z-30"
		title="More options"
	>
		<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
			<circle cx="12" cy="5" r="2"/>
			<circle cx="12" cy="12" r="2"/>
			<circle cx="12" cy="19" r="2"/>
		</svg>
	</button>

	{#if showMenu}
		<div class="absolute top-12 right-2 w-56 bg-surface border border-white/10 rounded-lg shadow-xl z-50">
			<a
				href={video.url}
				target="_blank"
				rel="noopener"
				class="flex items-center w-full px-4 py-2 text-sm hover:bg-white/5 transition-colors rounded-t-lg"
				onclick={() => showMenu = false}
			>
				<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
					<polyline points="15 3 21 3 21 9"/>
					<line x1="10" y1="14" x2="21" y2="3"/>
				</svg>
				Open in YouTube
			</a>
			<div class="border-t border-white/10"></div>
			<button
				onclick={handleToggleWatched}
				disabled={togglingWatched}
				class="flex items-center w-full px-4 py-2 text-sm hover:bg-white/5 transition-colors disabled:opacity-50"
			>
				{#if togglingWatched}
					<svg class="w-4 h-4 mr-2 animate-spin" viewBox="0 0 24 24" fill="none">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
					</svg>
				{:else if isWatched}
					<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
						<circle cx="12" cy="12" r="3"/>
						<line x1="1" y1="1" x2="23" y2="23"/>
					</svg>
				{:else}
					<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
						<circle cx="12" cy="12" r="3"/>
					</svg>
				{/if}
				{isWatched ? 'Mark as unwatched' : 'Mark as watched'}
			</button>
			<div class="border-t border-white/10"></div>
			<div class="relative">
				<button
					onclick={handleOpenAddToFeed}
					disabled={loadingFeeds || addingToFeed}
					class="flex items-center justify-between w-full px-4 py-2 text-sm hover:bg-white/5 transition-colors disabled:opacity-50"
				>
					<span class="flex items-center">
						{#if loadingFeeds}
							<svg class="w-4 h-4 mr-2 animate-spin" viewBox="0 0 24 24" fill="none">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
							</svg>
						{:else}
							<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M12 5v14M5 12h14"/>
							</svg>
						{/if}
						Add to feed
					</span>
					<svg class="w-3 h-3 hidden sm:block" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M9 18l6-6-6-6"/>
					</svg>
				</button>
				{#if showAddToFeedMenu}
					<div class="absolute left-full top-0 ml-1 w-48 bg-surface border border-white/10 rounded-lg shadow-xl z-50">
						{#if availableFeeds.length > 0}
							{#each availableFeeds as feed}
								<button
									onclick={(e) => handleAddToFeed(feed, e)}
									disabled={addingToFeed}
									class="flex items-center w-full px-4 py-2 text-sm hover:bg-white/5 transition-colors first:rounded-t-lg disabled:opacity-50"
								>
									{feed.name}
								</button>
							{/each}
							<div class="border-t border-white/10"></div>
						{/if}
						<button
							onclick={handleCreateNewFeed}
							disabled={addingToFeed}
							class="flex items-center w-full px-4 py-2 text-sm text-emerald-400 hover:bg-emerald-500/10 transition-colors rounded-b-lg {availableFeeds.length === 0 ? 'rounded-t-lg' : ''} disabled:opacity-50"
						>
							<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M12 5v14M5 12h14"/>
							</svg>
							Create new feed...
						</button>
					</div>
				{/if}
			</div>
			{#if showRemoveFromFeed && currentFeedId}
				<div class="border-t border-white/10"></div>
				<button
					onclick={handleRemoveFromFeed}
					disabled={removingFromFeed}
					class="flex items-center w-full px-4 py-2 text-sm text-crimson-400 hover:bg-crimson-500/10 transition-colors rounded-b-lg disabled:opacity-50"
				>
					{#if removingFromFeed}
						<svg class="w-4 h-4 mr-2 animate-spin" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
						</svg>
					{:else}
						<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M18 6L6 18M6 6l12 12"/>
						</svg>
					{/if}
					Remove channel from feed
				</button>
			{/if}
		</div>
	{/if}
</article>
