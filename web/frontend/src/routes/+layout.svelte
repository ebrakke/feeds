<script lang="ts">
	import '../app.css';
	import Toast from '$lib/components/Toast.svelte';
	import BottomSheet from '$lib/components/BottomSheet.svelte';

	let { children } = $props();
</script>

<svelte:head>
	<title>Feeds</title>
</svelte:head>

<div class="min-h-screen flex flex-col">
	<!-- Header -->
	<header class="app-header">
		<div class="container">
			<nav class="flex items-center justify-between h-14 sm:h-16">
				<!-- Logo -->
				<a href="/" class="group flex items-center gap-2 sm:gap-2.5 -ml-1 p-1 rounded-lg">
					<div class="w-9 h-9 sm:w-8 sm:h-8 rounded-lg bg-gradient-to-br from-emerald-500 to-emerald-600 flex items-center justify-center shadow-[0_0_20px_rgba(52,211,153,0.2)] group-hover:shadow-[0_0_30px_rgba(52,211,153,0.3)] transition-shadow">
						<svg class="w-4.5 h-4.5 sm:w-4 sm:h-4 text-[#050505]" viewBox="0 0 24 24" fill="currentColor">
							<path d="M8 5v14l11-7z"/>
						</svg>
					</div>
					<span class="text-lg font-display font-semibold tracking-tight text-text-primary group-hover:text-emerald-400 transition-colors">
						Feeds
					</span>
				</a>

				<!-- Navigation - touch-optimized -->
				<div class="flex items-center gap-2 sm:gap-1 -mr-1">
					<a
						href="/history"
						class="btn btn-ghost btn-sm flex items-center justify-center"
						aria-label="History"
					>
						<svg class="w-5 h-5 sm:w-4 sm:h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<circle cx="12" cy="12" r="10"/>
							<polyline points="12,6 12,12 16,14"/>
						</svg>
						<span class="hidden sm:inline ml-1.5">History</span>
					</a>
					<a
						href="/import"
						class="btn btn-primary btn-sm flex items-center justify-center"
					>
						<svg class="w-5 h-5 sm:w-4 sm:h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
							<line x1="12" y1="5" x2="12" y2="19"/>
							<line x1="5" y1="12" x2="19" y2="12"/>
						</svg>
						<span class="ml-1.5">Import</span>
					</a>
				</div>
			</nav>
		</div>
	</header>

	<!-- Main Content -->
	<main class="flex-1 container py-4 sm:py-6">
		{@render children()}
	</main>

	<!-- Footer -->
	<footer class="border-t border-border-subtle">
		<div class="container py-4">
			<p class="text-center text-xs text-text-dim">
				A personal YouTube feed aggregator
			</p>
		</div>
	</footer>
	<Toast />
	<BottomSheet />
</div>
