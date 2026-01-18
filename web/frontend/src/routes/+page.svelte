<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getFeeds } from '$lib/api';
	import type { Feed } from '$lib/types';

	let feeds = $state<Feed[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let videoUrl = $state('');
	let opening = $state(false);

	onMount(async () => {
		try {
			feeds = await getFeeds();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load feeds';
		} finally {
			loading = false;
		}
	});

	async function openVideo(e: Event) {
		e.preventDefault();
		if (!videoUrl.trim()) return;

		opening = true;
		// Extract video ID from various YouTube URL formats
		const match = videoUrl.match(/(?:v=|youtu\.be\/|shorts\/)([a-zA-Z0-9_-]{11})/);
		if (match) {
			goto(`/watch/${match[1]}`);
		} else {
			alert('Invalid YouTube URL');
			opening = false;
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}
</script>

<svelte:head>
	<title>Home - Feeds</title>
</svelte:head>

<!-- Quick Open Video -->
<section class="mb-8 animate-fade-up" style="opacity: 0;">
	<form onsubmit={openVideo}>
		<div class="relative">
			<input
				type="text"
				bind:value={videoUrl}
				placeholder="Paste a YouTube URL to watch..."
				class="input pl-12 pr-24"
			/>
			<svg class="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
				<path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
			</svg>
			<button
				type="submit"
				disabled={opening || !videoUrl.trim()}
				class="absolute right-2 top-1/2 -translate-y-1/2 btn btn-primary btn-sm"
			>
				{#if opening}
					<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
					</svg>
				{:else}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M8 5v14l11-7z"/>
					</svg>
					<span>Watch</span>
				{/if}
			</button>
		</div>
	</form>
</section>

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
	{@const inboxFeed = feeds.find(f => f.is_system)}
	{@const regularFeeds = feeds.filter(f => !f.is_system)}

	<div class="space-y-4">
		<!-- Inbox (triage) -->
		{#if inboxFeed}
			<a
				href="/feeds/{inboxFeed.id}"
				class="feed-card feed-card-inbox group animate-fade-up stagger-1"
				style="opacity: 0;"
			>
				<div class="flex items-center gap-3">
					<div class="w-10 h-10 rounded-lg bg-emerald-500/20 flex items-center justify-center group-hover:scale-105 transition-transform">
						<svg class="w-5 h-5 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<polyline points="22,12 16,12 14,15 10,15 8,12 2,12"/>
							<path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>
						</svg>
					</div>
					<div>
						<h3 class="font-display font-semibold text-emerald-300">Inbox</h3>
						<p class="text-sm text-emerald-400/60">New channels to organize</p>
					</div>
					<svg class="w-5 h-5 text-emerald-400/40 ml-auto group-hover:translate-x-1 transition-transform" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M9 18l6-6-6-6"/>
					</svg>
				</div>
			</a>
		{/if}

		<!-- Everything feed link -->
		<a
			href="/all"
			class="feed-card feed-card-featured group animate-fade-up stagger-2"
			style="opacity: 0;"
		>
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 rounded-lg bg-black/20 flex items-center justify-center group-hover:scale-105 transition-transform">
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M8 5v14l11-7z"/>
					</svg>
				</div>
				<div>
					<h3 class="font-display font-semibold">Everything</h3>
					<p class="text-sm opacity-80">Recent videos from all feeds</p>
				</div>
				<svg class="w-5 h-5 opacity-60 ml-auto group-hover:translate-x-1 transition-transform" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M9 18l6-6-6-6"/>
				</svg>
			</div>
		</a>

		<!-- Regular Feeds -->
		{#if regularFeeds.length > 0}
			<div class="pt-4">
				<h2 class="text-xs font-display font-medium text-text-muted uppercase tracking-wider mb-3 px-1">Your Feeds</h2>
				<div class="space-y-2">
					{#each regularFeeds as feed, i}
						<a
							href="/feeds/{feed.id}"
							class="feed-card group animate-fade-up"
							style="opacity: 0; animation-delay: {0.15 + i * 0.05}s;"
						>
							<div class="flex items-center gap-3">
								<div class="w-10 h-10 rounded-lg bg-elevated flex items-center justify-center group-hover:bg-border transition-colors">
									<svg class="w-5 h-5 text-text-muted group-hover:text-emerald-400 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
										<path d="M4 11a9 9 0 0 1 9 9"/>
										<path d="M4 4a16 16 0 0 1 16 16"/>
										<circle cx="5" cy="19" r="1"/>
									</svg>
								</div>
								<div class="flex-1 min-w-0">
									<h3 class="font-display font-medium truncate group-hover:text-emerald-400 transition-colors">{feed.name}</h3>
									<p class="text-sm text-text-muted">Updated {formatDate(feed.updated_at)}</p>
								</div>
								<svg class="w-5 h-5 text-text-dim group-hover:text-text-muted group-hover:translate-x-1 transition-all" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
									<path d="M9 18l6-6-6-6"/>
								</svg>
							</div>
						</a>
					{/each}
				</div>
			</div>
		{/if}
	</div>
{/if}
