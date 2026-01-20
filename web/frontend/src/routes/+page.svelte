<script lang="ts">
	import { onMount } from 'svelte';
	import { getFeeds } from '$lib/api';
	import type { Feed } from '$lib/types';
	import { navigationOrigin } from '$lib/stores/navigation';

	let feeds = $state<Feed[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Separate frequency feeds (from watch history import) from user-created feeds
	// Frequency feeds typically have names like "Heavy Rotation", "Regulars", etc.
	const FREQUENCY_FEED_NAMES = ['Heavy Rotation', 'Regulars', 'Frequent', 'Occasional', 'A Few Times', 'Discovered'];

	let frequencyFeeds = $derived(
		feeds.filter(f => !f.is_system && FREQUENCY_FEED_NAMES.includes(f.name))
	);

	let userFeeds = $derived(
		feeds.filter(f => !f.is_system && !FREQUENCY_FEED_NAMES.includes(f.name))
	);

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
	<div class="space-y-4">
		<!-- Frequency Feeds (system-generated from watch history) -->
		{#if frequencyFeeds.length > 0}
			<div class="space-y-2">
				{#each frequencyFeeds as feed}
					<a
						href="/feeds/{feed.id}"
						class="card flex items-center justify-between p-4 hover:bg-elevated transition-colors group"
					>
						<div class="flex items-center gap-3 min-w-0">
							<div class="w-10 h-10 rounded-lg bg-gradient-to-br from-emerald-500/20 to-emerald-600/20 flex items-center justify-center flex-shrink-0">
								<svg class="w-5 h-5 text-emerald-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
								</svg>
							</div>
							<span class="font-medium text-text-primary truncate">{feed.name}</span>
						</div>
						<svg class="w-5 h-5 text-text-muted group-hover:text-text-secondary transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
						</svg>
					</a>
				{/each}
			</div>
		{/if}

		<!-- Divider -->
		{#if frequencyFeeds.length > 0 && userFeeds.length > 0}
			<div class="flex items-center gap-4 py-4">
				<div class="flex-1 border-t border-border-subtle"></div>
				<span class="text-sm text-text-muted">Your Feeds</span>
				<div class="flex-1 border-t border-border-subtle"></div>
			</div>
		{/if}

		<!-- User Feeds -->
		{#if userFeeds.length > 0}
			<div class="space-y-2">
				{#each userFeeds as feed}
					<a
						href="/feeds/{feed.id}"
						class="card flex items-center justify-between p-4 hover:bg-elevated transition-colors group"
					>
						<div class="flex items-center gap-3 min-w-0">
							<div class="w-10 h-10 rounded-lg bg-gradient-to-br from-violet-500/20 to-violet-600/20 flex items-center justify-center flex-shrink-0">
								<svg class="w-5 h-5 text-violet-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
								</svg>
							</div>
							<span class="font-medium text-text-primary truncate">{feed.name}</span>
						</div>
						<svg class="w-5 h-5 text-text-muted group-hover:text-text-secondary transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
						</svg>
					</a>
				{/each}
			</div>
		{/if}

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
