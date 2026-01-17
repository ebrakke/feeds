<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import type { Video, WatchProgress, Feed } from '$lib/types';
	import VideoCard from './VideoCard.svelte';

	interface Props {
		videos: Video[];
		progressMap: Record<string, WatchProgress>;
		showChannel?: boolean;
		showMoveAction?: boolean;
		showRemoveAction?: boolean;
		availableFeeds?: Feed[];
		onChannelMoved?: () => void;
		onChannelRemoved?: () => void;
		scrollRestoreKey?: string;
	}

	let {
		videos,
		progressMap,
		showChannel = true,
		showMoveAction = false,
		showRemoveAction = false,
		availableFeeds = [],
		onChannelMoved,
		onChannelRemoved,
		scrollRestoreKey
	}: Props = $props();

	let gridElement: HTMLDivElement;

	onMount(() => {
		if (browser && scrollRestoreKey) {
			const savedVideoId = sessionStorage.getItem(scrollRestoreKey);
			if (savedVideoId) {
				sessionStorage.removeItem(scrollRestoreKey);
				requestAnimationFrame(() => {
					const card = gridElement?.querySelector(`[data-video-id="${savedVideoId}"]`);
					if (card) {
						card.scrollIntoView({ block: 'center' });
					}
				});
			}
		}
	});

	function handleVideoClick(videoId: string) {
		if (browser && scrollRestoreKey) {
			sessionStorage.setItem(scrollRestoreKey, videoId);
		}
	}
</script>

{#if videos.length === 0}
	<div class="empty-state">
		<div class="w-16 h-16 mx-auto mb-4 rounded-2xl bg-surface flex items-center justify-center">
			<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
				<rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18"/>
				<path d="M10 8l6 4-6 4V8z"/>
			</svg>
		</div>
		<p class="empty-state-title">No videos yet</p>
		<p class="empty-state-text">Videos will appear here once channels are added</p>
	</div>
{:else}
	<div bind:this={gridElement} class="video-grid">
		{#each videos as video, i (video.id)}
			<div
				class="animate-fade-up"
				style="opacity: 0; animation-delay: {Math.min(i * 0.03, 0.3)}s;"
			>
				<VideoCard
					{video}
					progress={progressMap[video.id]}
					{showChannel}
					{showMoveAction}
					{showRemoveAction}
					{availableFeeds}
					{onChannelMoved}
					{onChannelRemoved}
					onVideoClick={() => handleVideoClick(video.id)}
				/>
			</div>
		{/each}
	</div>
{/if}
