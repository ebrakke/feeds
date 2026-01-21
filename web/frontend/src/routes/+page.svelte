<script lang="ts">
	import { onMount } from 'svelte';
	import { getFeeds, reorderFeeds } from '$lib/api';
	import type { Feed } from '$lib/types';
	import { navigationOrigin } from '$lib/stores/navigation';

	let feeds = $state<Feed[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Drag state
	let draggedIndex = $state<number | null>(null);
	let dragOverIndex = $state<number | null>(null);

	onMount(async () => {
		try {
			feeds = await getFeeds();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load feeds';
		} finally {
			loading = false;
		}
	});

	// Clear navigation origin when returning to home
	$effect(() => {
		navigationOrigin.clear();
	});

	function handleDragStart(index: number) {
		draggedIndex = index;
	}

	function handleDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		dragOverIndex = index;
	}

	function handleDragLeave() {
		dragOverIndex = null;
	}

	async function handleDrop(index: number) {
		if (draggedIndex === null || draggedIndex === index) {
			draggedIndex = null;
			dragOverIndex = null;
			return;
		}

		// Reorder locally first (optimistic update)
		const newFeeds = [...feeds];
		const [moved] = newFeeds.splice(draggedIndex, 1);
		newFeeds.splice(index, 0, moved);
		feeds = newFeeds;

		draggedIndex = null;
		dragOverIndex = null;

		// Persist to backend
		try {
			const feedIds = newFeeds.map(f => f.id);
			await reorderFeeds(feedIds);
		} catch (e) {
			// Revert on error by re-fetching
			error = e instanceof Error ? e.message : 'Failed to reorder feeds';
			feeds = await getFeeds();
		}
	}

	function handleDragEnd() {
		draggedIndex = null;
		dragOverIndex = null;
	}
</script>

<svelte:head>
	<title>Home - Feeds</title>
</svelte:head>

{#if loading}
	<!-- Loading State -->
	<div class="flex flex-col items-center justify-center py-20">
		<div class="w-12 h-12 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin mb-4"></div>
		<p class="text-text-muted font-display">Loading your feeds...</p>
	</div>
{:else if error}
	<!-- Error State -->
	<div class="empty-state animate-fade-up" style="opacity: 0;">
		<svg class="empty-state-icon mx-auto" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
			<circle cx="12" cy="12" r="10"/>
			<line x1="12" y1="8" x2="12" y2="12"/>
			<line x1="12" y1="16" x2="12.01" y2="16"/>
		</svg>
		<p class="text-crimson-400 mb-2">{error}</p>
		<button onclick={() => location.reload()} class="btn btn-secondary btn-sm">
			Try Again
		</button>
	</div>
{:else if feeds.length === 0}
	<!-- Empty State -->
	<div class="empty-state animate-fade-up" style="opacity: 0;">
		<div class="w-20 h-20 mx-auto mb-6 rounded-2xl bg-surface flex items-center justify-center">
			<svg class="w-10 h-10 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
				<rect x="3" y="3" width="18" height="18" rx="2"/>
				<path d="M9 9h6v6H9z"/>
			</svg>
		</div>
		<h2 class="empty-state-title">No feeds yet</h2>
		<p class="empty-state-text mb-6">Import your subscriptions to get started</p>
		<a href="/import" class="btn btn-primary">
			<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
				<line x1="12" y1="5" x2="12" y2="19"/>
				<line x1="5" y1="12" x2="19" y2="12"/>
			</svg>
			Import Feed
		</a>
	</div>
{:else}
	<div class="space-y-2">
		{#each feeds as feed, index (feed.id)}
			<div
				draggable="true"
				ondragstart={() => handleDragStart(index)}
				ondragover={(e) => handleDragOver(e, index)}
				ondragleave={handleDragLeave}
				ondrop={() => handleDrop(index)}
				ondragend={handleDragEnd}
				class="group"
				class:opacity-50={draggedIndex === index}
			>
				{#if dragOverIndex === index && draggedIndex !== null && draggedIndex !== index}
					<div class="h-1 bg-emerald-500 rounded-full mb-2 transition-all"></div>
				{/if}
				<a
					href="/feeds/{feed.id}"
					class="card flex items-center justify-between p-4 hover:bg-elevated transition-colors"
				>
					<div class="flex items-center gap-3 min-w-0">
						<!-- Drag handle -->
						<div class="w-6 h-6 flex items-center justify-center text-text-muted cursor-grab active:cursor-grabbing flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
							<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
								<circle cx="9" cy="6" r="1.5"/>
								<circle cx="15" cy="6" r="1.5"/>
								<circle cx="9" cy="12" r="1.5"/>
								<circle cx="15" cy="12" r="1.5"/>
								<circle cx="9" cy="18" r="1.5"/>
								<circle cx="15" cy="18" r="1.5"/>
							</svg>
						</div>
						<!-- Feed icon -->
						<div class="w-10 h-10 rounded-lg bg-gradient-to-br {feed.is_system ? 'from-amber-500/20 to-amber-600/20' : 'from-violet-500/20 to-violet-600/20'} flex items-center justify-center flex-shrink-0">
							<svg class="w-5 h-5 {feed.is_system ? 'text-amber-500' : 'text-violet-500'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
							</svg>
						</div>
						<!-- Feed name and badge -->
						<span class="font-medium text-text-primary truncate">{feed.name}</span>
						{#if feed.new_video_count > 0}
							<span class="px-2 py-0.5 text-xs font-medium bg-emerald-500/20 text-emerald-400 rounded-full flex-shrink-0">
								{feed.new_video_count}
							</span>
						{/if}
					</div>
					<svg class="w-5 h-5 text-text-muted group-hover:text-text-secondary transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
					</svg>
				</a>
			</div>
		{/each}

		<!-- New Feed Button -->
		<div class="flex justify-center pt-6">
			<a
				href="/feeds/new"
				class="btn btn-primary flex items-center gap-2"
			>
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				<span>New Feed</span>
			</a>
		</div>
	</div>
{/if}
