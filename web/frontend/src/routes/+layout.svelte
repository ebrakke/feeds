<script lang="ts">
	import '../app.css';
	import Toast from '$lib/components/Toast.svelte';
	import BottomSheet from '$lib/components/BottomSheet.svelte';
	import HamburgerMenu from '$lib/components/HamburgerMenu.svelte';
	import SearchBar from '$lib/components/SearchBar.svelte';
	import { navigationOrigin } from '$lib/stores/navigation';
	import { page } from '$app/stores';

	let { children } = $props();

	let menuOpen = $state(false);
	let mobileSearchOpen = $state(false);

	// Determine if we should show origin navigation
	let showOrigin = $derived(
		$navigationOrigin !== null &&
		($page.url.pathname.startsWith('/watch/') || $page.url.pathname.startsWith('/channels/'))
	);
</script>

<svelte:head>
	<title>Feeds</title>
</svelte:head>

<div class="min-h-screen flex flex-col">
	<!-- Header -->
	<header class="app-header">
		<div class="container">
			<nav class="flex items-center justify-between h-14 sm:h-16">
				{#if showOrigin && $navigationOrigin}
					<!-- Contextual back navigation -->
					<a
						href={$navigationOrigin.path}
						class="flex items-center gap-2 -ml-1 p-1 rounded-lg text-text-primary hover:bg-elevated transition-colors min-w-0"
					>
						<svg class="w-5 h-5 flex-shrink-0 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
						</svg>
						<span class="font-medium truncate">{$navigationOrigin.feedName}</span>
					</a>
				{:else}
					<!-- Default navigation -->
					<div class="flex items-center gap-2">
						<button
							onclick={() => menuOpen = true}
							class="p-2 -ml-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-elevated transition-colors"
							aria-label="Open menu"
						>
							<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
							</svg>
						</button>
						<a href="/" class="group flex items-center gap-2 p-1 rounded-lg hover:bg-elevated transition-colors">
							<div class="w-8 h-8 sm:w-9 sm:h-9 rounded-lg bg-gradient-to-br from-emerald-400 to-emerald-600 flex items-center justify-center shadow-lg shadow-emerald-500/20">
								<svg class="w-4 h-4 sm:w-5 sm:h-5 text-white" viewBox="0 0 24 24" fill="currentColor">
									<path d="M8 5v14l11-7z"/>
								</svg>
							</div>
							<span class="text-lg font-display font-semibold text-text-primary">Feeds</span>
						</a>
					</div>
				{/if}

				<!-- Search and right side buttons -->
				<div class="flex items-center gap-1 -mr-1">
					<!-- Desktop search -->
					<div class="hidden sm:block">
						<SearchBar />
					</div>
					<!-- Mobile search toggle -->
					<button
						onclick={() => mobileSearchOpen = !mobileSearchOpen}
						class="sm:hidden btn btn-ghost btn-sm"
						aria-label="Search"
					>
						<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
						</svg>
					</button>
					<a href="/settings" class="btn btn-ghost btn-sm" aria-label="Settings">
						<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
						</svg>
					</a>
				</div>
			</nav>
			<!-- Mobile search bar (shown when toggle is clicked) -->
			{#if mobileSearchOpen}
				<div class="sm:hidden pb-3 animate-slide-down">
					<SearchBar />
				</div>
			{/if}
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
	<HamburgerMenu bind:open={menuOpen} />
</div>
