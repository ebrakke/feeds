<script lang="ts">
	import { toast } from '$lib/stores/toast';
</script>

{#if $toast.length > 0}
	<div class="fixed bottom-4 right-4 z-[100] flex flex-col gap-2">
		{#each $toast as t (t.id)}
			<div
				class="px-4 py-3 rounded-lg shadow-lg backdrop-blur-sm text-sm font-medium animate-slide-in
					{t.type === 'success' ? 'bg-emerald-500/90 text-white' : 'bg-crimson-500/90 text-white'}"
			>
				{t.message}
			</div>
		{/each}
	</div>
{/if}

<style>
	@keyframes slide-in {
		from {
			opacity: 0;
			transform: translateX(100%);
		}
		to {
			opacity: 1;
			transform: translateX(0);
		}
	}
	.animate-slide-in {
		animation: slide-in 0.2s ease-out;
	}
</style>
