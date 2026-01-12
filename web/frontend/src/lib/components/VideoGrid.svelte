<script lang="ts">
	import type { Video, WatchProgress } from '$lib/types';
	import VideoCard from './VideoCard.svelte';

	interface Props {
		videos: Video[];
		progressMap: Record<string, WatchProgress>;
		showChannel?: boolean;
	}

	let { videos, progressMap, showChannel = true }: Props = $props();
</script>

{#if videos.length === 0}
	<div class="text-center py-12">
		<p class="text-gray-400">No videos yet.</p>
	</div>
{:else}
	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
		{#each videos as video (video.id)}
			<VideoCard {video} progress={progressMap[video.id]} {showChannel} />
		{/each}
	</div>
{/if}
