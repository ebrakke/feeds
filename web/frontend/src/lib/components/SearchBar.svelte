<script lang="ts">
	import { search } from '$lib/api';
	import type { Video, Channel } from '$lib/types';
	import { goto } from '$app/navigation';

	let query = $state('');
	let isOpen = $state(false);
	let isLoading = $state(false);
	let videos = $state<Video[]>([]);
	let channels = $state<Channel[]>([]);
	let activeTab = $state<'videos' | 'channels'>('videos');
	let debounceTimeout: ReturnType<typeof setTimeout> | null = null;
	let abortController: AbortController | null = null;
	let inputRef: HTMLInputElement;

	async function doSearch(q: string) {
		if (q.length < 2) {
			videos = [];
			channels = [];
			return;
		}

		// Abort previous request
		if (abortController) {
			abortController.abort();
		}
		abortController = new AbortController();

		isLoading = true;
		try {
			const result = await search(q, 10);
			videos = result.videos || [];
			channels = result.channels || [];
			// Default to tab with results
			if (videos.length === 0 && channels.length > 0) {
				activeTab = 'channels';
			} else {
				activeTab = 'videos';
			}
		} catch (err) {
			if ((err as Error).name !== 'AbortError') {
				console.error('Search failed:', err);
			}
		} finally {
			isLoading = false;
		}
	}

	function handleInput() {
		if (debounceTimeout) {
			clearTimeout(debounceTimeout);
		}
		debounceTimeout = setTimeout(() => {
			doSearch(query);
		}, 300);
	}

	function handleFocus() {
		isOpen = true;
		if (query.length >= 2) {
			doSearch(query);
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			isOpen = false;
			inputRef?.blur();
		}
	}

	function handleClickOutside(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (!target.closest('.search-container')) {
			isOpen = false;
		}
	}

	function navigateTo(path: string) {
		isOpen = false;
		query = '';
		videos = [];
		channels = [];
		goto(path);
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
		if (diffDays < 7) return `${diffDays}d ago`;
		if (diffDays < 30) return `${Math.floor(diffDays / 7)}w ago`;
		if (diffDays < 365) return `${Math.floor(diffDays / 30)}mo ago`;
		return `${Math.floor(diffDays / 365)}y ago`;
	}

	let hasResults = $derived(videos.length > 0 || channels.length > 0);
	let showDropdown = $derived(isOpen && query.length >= 2);
</script>

<svelte:window on:click={handleClickOutside} on:keydown={handleKeydown} />

<div class="search-container relative">
	<div class="relative">
		<input
			bind:this={inputRef}
			bind:value={query}
			oninput={handleInput}
			onfocus={handleFocus}
			type="text"
			placeholder="Search videos & channels..."
			class="search-input"
		/>
		<div class="absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none">
			{#if isLoading}
				<svg class="w-4 h-4 text-text-muted animate-spin" viewBox="0 0 24 24" fill="none">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
				</svg>
			{:else}
				<svg class="w-4 h-4 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
				</svg>
			{/if}
		</div>
		{#if query.length > 0}
			<button
				onclick={() => { query = ''; videos = []; channels = []; }}
				class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-text-muted hover:text-text-primary transition-colors"
				aria-label="Clear search"
			>
				<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		{/if}
	</div>

	{#if showDropdown}
		<div class="search-dropdown">
			{#if query.length < 2}
				<div class="search-hint">Type 2+ characters to search</div>
			{:else if isLoading}
				<div class="search-hint">Searching...</div>
			{:else if !hasResults}
				<div class="search-hint">No results for "{query}"</div>
			{:else}
				<!-- Tabs -->
				<div class="search-tabs">
					<button
						class="search-tab"
						class:active={activeTab === 'videos'}
						onclick={() => activeTab = 'videos'}
					>
						Videos ({videos.length})
					</button>
					<button
						class="search-tab"
						class:active={activeTab === 'channels'}
						onclick={() => activeTab = 'channels'}
					>
						Channels ({channels.length})
					</button>
				</div>

				<!-- Results -->
				<div class="search-results">
					{#if activeTab === 'videos'}
						{#if videos.length === 0}
							<div class="search-hint">No videos found</div>
						{:else}
							{#each videos as video}
								<button
									class="search-result-video"
									onclick={() => navigateTo(`/watch/${video.id}`)}
								>
									<div class="search-thumbnail">
										<img src={video.thumbnail} alt="" loading="lazy" />
										{#if video.duration}
											<span class="search-duration">{formatDuration(video.duration)}</span>
										{/if}
									</div>
									<div class="search-video-info">
										<div class="search-video-title">{video.title}</div>
										<div class="search-video-meta">
											<span>{video.channel_name}</span>
											<span class="text-text-dim">·</span>
											<span>{formatRelativeTime(video.published)}</span>
										</div>
									</div>
								</button>
							{/each}
						{/if}
					{:else}
						{#if channels.length === 0}
							<div class="search-hint">No channels found</div>
						{:else}
							{#each channels as channel}
								<button
									class="search-result-channel"
									onclick={() => navigateTo(`/channels/${channel.id}`)}
								>
									<div class="search-channel-icon">
										<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
										</svg>
									</div>
									<div class="search-channel-name">{channel.name}</div>
								</button>
							{/each}
						{/if}
					{/if}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.search-container {
		width: 100%;
		max-width: 400px;
	}

	.search-input {
		width: 100%;
		padding: 0.5rem 2rem 0.5rem 2.25rem;
		background: var(--color-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: var(--radius-md);
		color: var(--color-text-primary);
		font-family: var(--font-body);
		font-size: 0.875rem;
		transition: all 0.2s var(--ease-out-expo);
	}

	.search-input::placeholder {
		color: var(--color-text-muted);
	}

	.search-input:focus {
		outline: none;
		border-color: var(--color-emerald-500);
		box-shadow: 0 0 0 3px var(--color-emerald-glow);
	}

	.search-dropdown {
		position: absolute;
		top: calc(100% + 0.5rem);
		left: 0;
		right: 0;
		background: var(--color-elevated);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		box-shadow: var(--shadow-elevated);
		z-index: 100;
		overflow: hidden;
		animation: scale-in 0.2s var(--ease-out-expo);
	}

	.search-hint {
		padding: 1rem;
		text-align: center;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.search-tabs {
		display: flex;
		border-bottom: 1px solid var(--color-border-subtle);
	}

	.search-tab {
		flex: 1;
		padding: 0.75rem;
		font-family: var(--font-display);
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-muted);
		background: transparent;
		border: none;
		cursor: pointer;
		transition: all 0.15s;
	}

	.search-tab:hover {
		color: var(--color-text-secondary);
		background: var(--color-surface);
	}

	.search-tab.active {
		color: var(--color-emerald-400);
		background: var(--color-surface);
		box-shadow: inset 0 -2px 0 var(--color-emerald-500);
	}

	.search-results {
		max-height: 400px;
		overflow-y: auto;
	}

	.search-result-video {
		display: flex;
		gap: 0.75rem;
		width: 100%;
		padding: 0.75rem;
		text-align: left;
		background: transparent;
		border: none;
		cursor: pointer;
		transition: background 0.15s;
	}

	.search-result-video:hover {
		background: var(--color-surface);
	}

	.search-thumbnail {
		position: relative;
		width: 100px;
		aspect-ratio: 16 / 9;
		flex-shrink: 0;
		border-radius: var(--radius-sm);
		overflow: hidden;
		background: var(--color-surface);
	}

	.search-thumbnail img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.search-duration {
		position: absolute;
		bottom: 0.25rem;
		right: 0.25rem;
		padding: 0.125rem 0.25rem;
		background: rgba(0, 0, 0, 0.85);
		color: white;
		font-family: var(--font-display);
		font-size: 0.6875rem;
		font-weight: 500;
		border-radius: 2px;
	}

	.search-video-info {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.search-video-title {
		font-family: var(--font-display);
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-primary);
		line-height: 1.3;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.search-video-meta {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		display: flex;
		gap: 0.375rem;
		align-items: center;
	}

	.search-video-meta span:first-child {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.search-result-channel {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		width: 100%;
		padding: 0.75rem;
		text-align: left;
		background: transparent;
		border: none;
		cursor: pointer;
		transition: background 0.15s;
	}

	.search-result-channel:hover {
		background: var(--color-surface);
	}

	.search-channel-icon {
		width: 2.5rem;
		height: 2.5rem;
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--color-surface);
		border-radius: 50%;
		color: var(--color-text-muted);
	}

	.search-channel-name {
		font-family: var(--font-display);
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	@media (max-width: 640px) {
		.search-container {
			max-width: 100%;
		}

		.search-dropdown {
			position: fixed;
			top: 4rem;
			left: 0.5rem;
			right: 0.5rem;
			max-height: calc(100vh - 5rem);
		}

		.search-results {
			max-height: calc(100vh - 10rem);
		}

		.search-result-video,
		.search-result-channel {
			padding: 1rem;
		}
	}
</style>
