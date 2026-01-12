<script lang="ts">
	import { goto } from '$app/navigation';
	import { importFromURL, importFromFile, getPacks } from '$lib/api';
	import type { Feed } from '$lib/types';

	let url = $state('');
	let file = $state<File | null>(null);
	let loading = $state(false);
	let error = $state<string | null>(null);

	let packs = $state<{ name: string; description: string; author: string; tags: string[] }[]>([]);

	// Load packs on mount
	$effect(() => {
		getPacks().then(p => packs = p).catch(() => {});
	});

	async function handleImportURL(e: Event) {
		e.preventDefault();
		if (!url.trim()) return;

		loading = true;
		error = null;
		try {
			const feed = await importFromURL(url);
			goto(`/feeds/${feed.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to import';
		} finally {
			loading = false;
		}
	}

	async function handleImportFile(e: Event) {
		e.preventDefault();
		if (!file) return;

		loading = true;
		error = null;
		try {
			const feed = await importFromFile(file);
			goto(`/feeds/${feed.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to import';
		} finally {
			loading = false;
		}
	}

	function handleFileChange(e: Event) {
		const target = e.target as HTMLInputElement;
		file = target.files?.[0] ?? null;
	}

	async function handlePackImport(packName: string) {
		loading = true;
		error = null;
		try {
			const packUrl = window.location.origin + `/api/packs/${packName}`;
			const feed = await importFromURL(packUrl);
			goto(`/feeds/${feed.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to import pack';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Import - Feeds</title>
</svelte:head>

<div class="max-w-2xl mx-auto">
	<h1 class="text-2xl font-bold mb-6">Import Feed</h1>

	{#if error}
		<div class="bg-red-900/50 border border-red-700 rounded-lg p-4 mb-6">
			<p class="text-red-400">{error}</p>
		</div>
	{/if}

	<!-- Import from URL -->
	<div class="bg-gray-800 rounded-lg p-6 mb-6">
		<h2 class="text-lg font-semibold mb-4">Import from URL</h2>
		<form onsubmit={handleImportURL}>
			<input
				type="url"
				bind:value={url}
				placeholder="https://example.com/feed.json"
				class="w-full bg-gray-900 border border-gray-700 rounded px-4 py-2 mb-4 text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
			/>
			<button
				type="submit"
				disabled={loading || !url.trim()}
				class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
			>
				{#if loading}
					<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				{/if}
				Import
			</button>
		</form>
	</div>

	<!-- Import from file -->
	<div class="bg-gray-800 rounded-lg p-6 mb-6">
		<h2 class="text-lg font-semibold mb-4">Import from File</h2>
		<p class="text-gray-400 text-sm mb-4">
			Upload a NewPipe export (JSON) or Feeds export file.
		</p>
		<form onsubmit={handleImportFile}>
			<input
				type="file"
				accept=".json"
				onchange={handleFileChange}
				class="w-full bg-gray-900 border border-gray-700 rounded px-4 py-2 mb-4 text-white file:mr-4 file:py-1 file:px-3 file:rounded file:border-0 file:bg-blue-600 file:text-white file:cursor-pointer"
			/>
			<button
				type="submit"
				disabled={loading || !file}
				class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
			>
				{#if loading}
					<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				{/if}
				Upload & Import
			</button>
		</form>
	</div>

	<!-- Subscription Packs -->
	{#if packs.length > 0}
		<div class="bg-gray-800 rounded-lg p-6">
			<h2 class="text-lg font-semibold mb-4">Subscription Packs</h2>
			<p class="text-gray-400 text-sm mb-4">
				Quick-start with curated channel collections.
			</p>
			<div class="grid gap-3">
				{#each packs as pack}
					<button
						onclick={() => handlePackImport(pack.name)}
						disabled={loading}
						class="text-left bg-gray-700 hover:bg-gray-600 disabled:opacity-50 rounded-lg p-4 transition-colors"
					>
						<h3 class="font-semibold">{pack.name}</h3>
						<p class="text-sm text-gray-400">{pack.description}</p>
						{#if pack.tags.length > 0}
							<div class="flex gap-1 mt-2">
								{#each pack.tags as tag}
									<span class="text-xs bg-gray-600 rounded px-2 py-0.5">{tag}</span>
								{/each}
							</div>
						{/if}
					</button>
				{/each}
			</div>
		</div>
	{/if}
</div>
