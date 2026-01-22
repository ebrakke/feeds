<script lang="ts">
	import { onMount } from 'svelte';
	import { getFeeds, reorderFeeds, getSmartFeeds } from '$lib/api';
	import type { Feed, SmartFeed } from '$lib/types';
	import { navigationOrigin } from '$lib/stores/navigation';

	let feeds = $state<Feed[]>([]);
	let smartFeeds = $state<SmartFeed[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Drag state (desktop)
	let draggedIndex = $state<number | null>(null);
	let dragOverIndex = $state<number | null>(null);

	// Edit mode state (mobile-friendly reordering)
	let editMode = $state(false);
	let selectedIndex = $state<number | null>(null);

	onMount(async () => {
		try {
			const [feedsData, smartData] = await Promise.all([
				getFeeds(),
				getSmartFeeds()
			]);
			feeds = feedsData;
			smartFeeds = smartData.feeds || [];
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

	// Desktop drag handlers
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

		await moveItem(draggedIndex, index);
		draggedIndex = null;
		dragOverIndex = null;
	}

	function handleDragEnd() {
		draggedIndex = null;
		dragOverIndex = null;
	}

	// Edit mode handlers
	function toggleEditMode() {
		editMode = !editMode;
		if (!editMode) {
			selectedIndex = null;
		}
	}

	function selectFeed(index: number) {
		if (!editMode) return;
		selectedIndex = selectedIndex === index ? null : index;
	}

	async function moveUp(index: number) {
		if (index === 0) return;
		await moveItem(index, index - 1);
		selectedIndex = index - 1;
	}

	async function moveDown(index: number) {
		if (index >= feeds.length - 1) return;
		await moveItem(index, index + 1);
		selectedIndex = index + 1;
	}

	async function moveItem(fromIndex: number, toIndex: number) {
		// Reorder locally first (optimistic update)
		const newFeeds = [...feeds];
		const [moved] = newFeeds.splice(fromIndex, 1);
		newFeeds.splice(toIndex, 0, moved);
		feeds = newFeeds;

		// Persist to backend
		try {
			const feedIds = newFeeds.map(f => f.id);
			await reorderFeeds(feedIds);
		} catch (e) {
			// Revert on error by re-fetching
			error = e instanceof Error ? e.message : 'Failed to reorder feeds';
			feeds = await getFeeds();
			selectedIndex = null;
		}
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
	<!-- Smart Feeds Section -->
	{#if smartFeeds.length > 0}
		<div class="mb-6">
			<h2 class="text-sm font-medium text-text-muted uppercase tracking-wider mb-3">Smart Feeds</h2>
			<div class="space-y-2">
				{#each smartFeeds as smartFeed (smartFeed.slug)}
					<a
						href="/smart/{smartFeed.slug}"
						class="card flex items-center justify-between p-4 hover:bg-elevated transition-colors group"
					>
						<div class="flex items-center gap-3 min-w-0">
							<!-- Smart feed icon -->
							<div class="w-10 h-10 rounded-lg bg-gradient-to-br {smartFeed.icon === 'flame' ? 'from-orange-500/20 to-orange-600/20' : 'from-emerald-500/20 to-emerald-600/20'} flex items-center justify-center flex-shrink-0">
								{#if smartFeed.icon === 'play'}
									<svg class="w-5 h-5 text-emerald-500" fill="currentColor" viewBox="0 0 24 24">
										<path d="M8 5v14l11-7z"/>
									</svg>
								{:else if smartFeed.icon === 'flame'}
									<svg class="w-5 h-5 text-orange-500" fill="currentColor" viewBox="0 0 24 24">
										<path d="M12 23c-3.866 0-7-3.358-7-7.5 0-2.84 1.5-5.5 3-7.5 1.5 2 3 3 5 3s3.5-1 5-3c1.5 2 3 4.66 3 7.5 0 4.142-3.134 7.5-7 7.5zm0-5c-1.657 0-3 1.343-3 3s1.343 3 3 3 3-1.343 3-3-1.343-3-3-3z"/>
									</svg>
								{:else}
									<svg class="w-5 h-5 text-emerald-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
									</svg>
								{/if}
							</div>
							<!-- Feed name -->
							<span class="font-medium text-text-primary truncate">{smartFeed.name}</span>
							{#if smartFeed.videoCount > 0}
								<span class="px-2 py-0.5 text-xs font-medium bg-emerald-500/20 text-emerald-400 rounded-full flex-shrink-0">
									{smartFeed.videoCount}
								</span>
							{/if}
						</div>
						<svg class="w-5 h-5 text-text-muted group-hover:text-text-secondary transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
						</svg>
					</a>
				{/each}
			</div>
		</div>
	{/if}

	<!-- Header with Edit button -->
	<div class="flex items-center justify-between mb-4">
		<h1 class="text-lg font-medium text-text-primary">Your Feeds</h1>
		<button
			onclick={toggleEditMode}
			class="text-sm font-medium {editMode ? 'text-emerald-500' : 'text-text-muted hover:text-text-secondary'} transition-colors"
		>
			{editMode ? 'Done' : 'Edit'}
		</button>
	</div>

	<div class="space-y-2">
		{#each feeds as feed, index (feed.id)}
			<div
				draggable={!editMode}
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

				{#if editMode}
					<!-- Edit mode: show reorder controls -->
					<div class="card flex items-center justify-between p-4">
						<div class="flex items-center gap-3 min-w-0">
							<!-- Feed icon -->
							<div class="w-10 h-10 rounded-lg bg-gradient-to-br {feed.is_system ? 'from-amber-500/20 to-amber-600/20' : 'from-violet-500/20 to-violet-600/20'} flex items-center justify-center flex-shrink-0">
								<svg class="w-5 h-5 {feed.is_system ? 'text-amber-500' : 'text-violet-500'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
								</svg>
							</div>
							<!-- Feed name -->
							<span class="font-medium text-text-primary truncate">{feed.name}</span>
						</div>
						<!-- Reorder buttons -->
						<div class="flex items-center gap-1">
							<button
								onclick={() => moveUp(index)}
								disabled={index === 0}
								class="p-2 rounded-lg hover:bg-elevated disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
								aria-label="Move up"
							>
								<svg class="w-5 h-5 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
								</svg>
							</button>
							<button
								onclick={() => moveDown(index)}
								disabled={index >= feeds.length - 1}
								class="p-2 rounded-lg hover:bg-elevated disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
								aria-label="Move down"
							>
								<svg class="w-5 h-5 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
								</svg>
							</button>
						</div>
					</div>
				{:else}
					<!-- Normal mode: clickable link -->
					<a
						href="/feeds/{feed.id}"
						class="card flex items-center justify-between p-4 hover:bg-elevated transition-colors"
					>
						<div class="flex items-center gap-3 min-w-0">
							<!-- Drag handle (desktop only) -->
							<div class="hidden sm:flex w-6 h-6 items-center justify-center text-text-muted cursor-grab active:cursor-grabbing flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
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
				{/if}
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
