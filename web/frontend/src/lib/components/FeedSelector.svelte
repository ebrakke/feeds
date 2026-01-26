<script lang="ts">
	import type { Feed } from '$lib/types';
	import { createFeed } from '$lib/api';

	interface Props {
		feeds: Feed[];
		onSelect: (feedId: number) => void;
		onCancel: () => void;
		loading?: boolean;
		error?: string | null;
	}

	let { feeds, onSelect, onCancel, loading = false, error = null }: Props = $props();

	let selectedFeedId = $state<number | null>(null);
	let createNew = $state(false);
	let newFeedName = $state('');
	let creating = $state(false);
	let createError = $state<string | null>(null);

	async function handleConfirm() {
		if (createNew) {
			if (!newFeedName.trim()) {
				createError = 'Feed name is required';
				return;
			}
			creating = true;
			createError = null;
			try {
				const feed = await createFeed(newFeedName.trim());
				onSelect(feed.id);
			} catch (e) {
				createError = e instanceof Error ? e.message : 'Failed to create feed';
			} finally {
				creating = false;
			}
		} else {
			if (!selectedFeedId) {
				return;
			}
			onSelect(selectedFeedId);
		}
	}

	function selectFeed(feedId: number) {
		selectedFeedId = feedId;
		createNew = false;
		createError = null;
	}

	function selectCreateNew() {
		createNew = true;
		selectedFeedId = null;
		createError = null;
	}
</script>

<!-- Backdrop -->
<div
	class="fixed inset-0 bg-void/80 backdrop-blur-sm z-50 flex items-center justify-center p-4"
	onclick={onCancel}
	role="presentation"
>
	<!-- Modal -->
	<div
		class="bg-surface border border-white/10 rounded-2xl shadow-2xl max-w-md w-full max-h-[80vh] overflow-hidden flex flex-col"
		onclick={(e) => e.stopPropagation()}
		role="dialog"
		aria-modal="true"
		aria-labelledby="feed-selector-title"
	>
		<!-- Header -->
		<div class="p-6 border-b border-white/10">
			<h2 id="feed-selector-title" class="text-xl font-display font-bold">Add Channel to Feed</h2>
			<p class="text-text-muted text-sm mt-1">Choose a feed or create a new one</p>
		</div>

		<!-- Error -->
		{#if error}
			<div class="mx-6 mt-4 bg-crimson-500/10 border border-crimson-500/30 rounded-xl p-3">
				<p class="text-crimson-400 text-sm">{error}</p>
			</div>
		{/if}

		{#if createError}
			<div class="mx-6 mt-4 bg-crimson-500/10 border border-crimson-500/30 rounded-xl p-3">
				<p class="text-crimson-400 text-sm">{createError}</p>
			</div>
		{/if}

		<!-- Feed List -->
		<div class="flex-1 overflow-y-auto p-6 space-y-2">
			{#each feeds as feed}
				<button
					type="button"
					onclick={() => selectFeed(feed.id)}
					class="w-full text-left p-3 rounded-xl border transition-all {selectedFeedId === feed.id
						? 'bg-emerald-500/10 border-emerald-500/50'
						: 'bg-void border-white/5 hover:border-white/20'}"
				>
					<div class="flex items-center gap-3">
						<div
							class="w-5 h-5 rounded-full border-2 flex items-center justify-center {selectedFeedId ===
							feed.id
								? 'border-emerald-500'
								: 'border-white/20'}"
						>
							{#if selectedFeedId === feed.id}
								<div class="w-2.5 h-2.5 rounded-full bg-emerald-500"></div>
							{/if}
						</div>
						<span class="font-medium text-text-primary">{feed.name}</span>
					</div>
				</button>
			{/each}

			<!-- Create New Option -->
			<button
				type="button"
				onclick={selectCreateNew}
				class="w-full text-left p-3 rounded-xl border transition-all {createNew
					? 'bg-emerald-500/10 border-emerald-500/50'
					: 'bg-void border-white/5 hover:border-white/20'}"
			>
				<div class="flex items-center gap-3">
					<div
						class="w-5 h-5 rounded-full border-2 flex items-center justify-center {createNew
							? 'border-emerald-500'
							: 'border-white/20'}"
					>
						{#if createNew}
							<div class="w-2.5 h-2.5 rounded-full bg-emerald-500"></div>
						{/if}
					</div>
					<span class="font-medium text-emerald-400">Create New Feed</span>
				</div>
			</button>

			{#if createNew}
				<div class="pl-8 pt-2">
					<input
						type="text"
						bind:value={newFeedName}
						placeholder="Feed name"
						class="w-full bg-void border border-white/10 rounded-lg px-4 py-2 text-text-primary placeholder-text-dim focus:outline-none focus:border-emerald-500/50 transition-colors"
						autofocus
					/>
				</div>
			{/if}
		</div>

		<!-- Actions -->
		<div class="p-6 border-t border-white/10 flex gap-3">
			<button type="button" onclick={onCancel} class="btn btn-secondary flex-1">
				Cancel
			</button>
			<button
				type="button"
				onclick={handleConfirm}
				disabled={loading || creating || (!selectedFeedId && !createNew) || (createNew && !newFeedName.trim())}
				class="btn btn-primary flex-1"
			>
				{#if loading || creating}
					<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
						<circle
							class="opacity-25"
							cx="12"
							cy="12"
							r="10"
							stroke="currentColor"
							stroke-width="4"
						/>
						<path
							class="opacity-75"
							fill="currentColor"
							d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
						/>
					</svg>
					{creating ? 'Creating...' : 'Adding...'}
				{:else}
					Add Channel
				{/if}
			</button>
		</div>
	</div>
</div>
