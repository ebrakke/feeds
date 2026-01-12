<script lang="ts">
	import type { Video, WatchProgress } from '$lib/types';

	interface Props {
		video: Video;
		progress?: WatchProgress;
		showChannel?: boolean;
	}

	let { video, progress, showChannel = true }: Props = $props();

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
