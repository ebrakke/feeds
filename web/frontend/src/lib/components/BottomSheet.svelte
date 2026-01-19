<script lang="ts">
	import { bottomSheet } from '$lib/stores/bottomSheet';
	import { toast } from '$lib/stores/toast';
	import { createFeed, addChannelToFeed } from '$lib/api';
	import type { Feed } from '$lib/types';

	let processing = $state(false);

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

	async function handleCreate() {
		if (processing) return;

		const name = prompt('Enter feed name:');
		if (!name?.trim()) return;

		processing = true;
		try {
			const newFeed = await createFeed(name.trim());
			await addChannelToFeed($bottomSheet.channelId!, newFeed.id);
			toast.success(`Added to ${newFeed.name}`);
			bottomSheet.close();
		} catch (err) {
			console.error('Failed to create feed:', err);
			toast.error('Failed to create feed');
		} finally {
			processing = false;
		}
	}

	function handleClose() {
		if (!processing) {
			bottomSheet.close();
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
			{#each $bottomSheet.feeds as feed}
				<button
					onclick={() => handleSelect(feed)}
					disabled={processing}
					class="flex items-center w-full px-4 py-3 text-base text-text-primary hover:bg-white/5 active:bg-white/10 transition-colors disabled:opacity-50"
				>
					{feed.name}
				</button>
			{/each}
			{#if $bottomSheet.feeds.length === 0}
				<p class="px-4 py-3 text-sm text-text-muted">No available feeds</p>
			{/if}
			<div class="border-t border-white/10 mt-2"></div>
			<button
				onclick={handleCreate}
				disabled={processing}
				class="flex items-center w-full px-4 py-3 text-base text-emerald-400 hover:bg-emerald-500/10 active:bg-emerald-500/20 transition-colors disabled:opacity-50"
			>
				<svg class="w-5 h-5 mr-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M12 5v14M5 12h14"/>
				</svg>
				Create new feed
			</button>
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
