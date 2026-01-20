<script lang="ts">
	import { bottomSheet } from '$lib/stores/bottomSheet';
	import { toast } from '$lib/stores/toast';
	import { createFeed, addChannelToFeed } from '$lib/api';
	import type { Feed } from '$lib/types';

	let processing = $state(false);
	let creatingNew = $state(false);
	let newFeedName = $state('');
	let creating = $state(false);

	async function handleSelect(feed: Feed) {
		if (processing) return;
		processing = true;
		try {
			await addChannelToFeed($bottomSheet.channelId!, feed.id);
			toast.success(`Added to ${feed.name}`);
			bottomSheet.close();
		} catch (err) {
			console.error('Failed to add to feed:', err);
			toast.error('Failed to add to feed');
		} finally {
			processing = false;
		}
	}

	async function handleCreateAndAdd() {
		if (!newFeedName.trim() || !$bottomSheet.channelId) return;

		creating = true;
		try {
			const newFeed = await createFeed(newFeedName.trim());
			await addChannelToFeed($bottomSheet.channelId!, newFeed.id);
			toast.success(`Created "${newFeedName}" and added ${$bottomSheet.channelName}`);
			bottomSheet.close();
			newFeedName = '';
			creatingNew = false;
		} catch (err) {
			console.error('Failed to create feed:', err);
			toast.error('Failed to create feed');
		} finally {
			creating = false;
		}
	}

	function handleClose() {
		if (!processing && !creating) {
			bottomSheet.close();
			creatingNew = false;
			newFeedName = '';
		}
	}
</script>

{#if $bottomSheet.open}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="backdrop" onclick={handleClose}></div>

	<div class="sheet">
		<!-- Handle bar -->
		<div class="flex justify-center pt-3 pb-1">
			<div class="w-10 h-1 bg-white/20 rounded-full"></div>
		</div>

		<!-- Title -->
		<div class="px-4 pb-3 border-b border-white/10">
			<h3 class="text-lg font-display font-medium text-text-primary">{$bottomSheet.title}</h3>
		</div>

		<!-- Content -->
		<div class="max-h-[60vh] overflow-y-auto overscroll-contain py-2">
			{#each $bottomSheet.feeds.filter(f => !f.is_system) as feed}
				<button
					onclick={() => handleSelect(feed)}
					disabled={processing}
					class="w-full flex items-center justify-between px-4 py-3 hover:bg-elevated transition-colors disabled:opacity-50 text-left"
				>
					<span class="text-text-primary">{feed.name}</span>
					<div class="w-5 h-5 rounded-full border-2 flex items-center justify-center {$bottomSheet.memberFeedIds.includes(feed.id) ? 'bg-emerald-500 border-emerald-500' : 'border-text-muted'}">
						{#if $bottomSheet.memberFeedIds.includes(feed.id)}
							<svg class="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
							</svg>
						{/if}
					</div>
				</button>
			{/each}
			{#if $bottomSheet.feeds.filter(f => !f.is_system).length === 0}
				<p class="px-4 py-3 text-sm text-text-muted">No available feeds</p>
			{/if}

			<div class="border-t border-border-subtle">
				{#if creatingNew}
					<div class="p-4 space-y-3">
						<input
							type="text"
							bind:value={newFeedName}
							placeholder="New feed name..."
							class="input w-full"
							onkeydown={(e) => e.key === 'Enter' && handleCreateAndAdd()}
						/>
						<div class="flex gap-2">
							<button
								onclick={() => { creatingNew = false; newFeedName = ''; }}
								class="btn btn-ghost flex-1"
								disabled={creating}
							>
								Cancel
							</button>
							<button
								onclick={handleCreateAndAdd}
								class="btn btn-primary flex-1"
								disabled={creating || !newFeedName.trim()}
							>
								{creating ? 'Creating...' : 'Create'}
							</button>
						</div>
					</div>
				{:else}
					<button
						onclick={() => creatingNew = true}
						class="w-full flex items-center gap-3 px-4 py-3 text-emerald-500 hover:bg-elevated transition-colors"
					>
						<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
						</svg>
						<span>Create new feed</span>
					</button>
				{/if}
			</div>
		</div>
	</div>
{/if}

<style>
	.backdrop {
		position: fixed !important;
		top: 0 !important;
		left: 0 !important;
		right: 0 !important;
		bottom: 0 !important;
		background: rgba(0, 0, 0, 0.6);
		backdrop-filter: blur(4px);
		z-index: 9998 !important;
	}

	.sheet {
		position: fixed !important;
		bottom: 0 !important;
		left: 0 !important;
		right: 0 !important;
		top: auto !important;
		background: var(--color-surface);
		border-top: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 1rem 1rem 0 0;
		z-index: 9999 !important;
		animation: slide-up 0.3s var(--ease-out-expo);
		padding-bottom: env(safe-area-inset-bottom, 0);
	}

	@keyframes slide-up {
		from {
			transform: translateY(100%);
		}
		to {
			transform: translateY(0);
		}
	}
</style>
