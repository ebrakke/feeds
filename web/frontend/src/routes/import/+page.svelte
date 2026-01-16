<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		importFromURL,
		importFromFile,
		importWatchHistory,
		organizeWatchHistory,
		confirmOrganize,
		getPacks,
		getConfig,
		setYtdlpCookies,
		clearYtdlpCookies
	} from '$lib/api';
	import type { Feed, Config, WatchHistoryChannel, GroupSuggestion } from '$lib/types';

	type Step = 'upload' | 'preview' | 'organize' | 'confirm';

	let importURL = $state('');
	let importFile = $state<File | null>(null);
	let importLoading = $state(false);
	let importError = $state<string | null>(null);

	let watchStep = $state<Step>('upload');
	let watchFile = $state<File | null>(null);
	let watchLoading = $state(false);
	let watchError = $state<string | null>(null);
	let watchChannels = $state<WatchHistoryChannel[]>([]);
	let watchTotalVideos = $state(0);
	let watchGroups = $state<GroupSuggestion[]>([]);
	let watchSelectedChannels = $state<Set<string>>(new Set());
	let config = $state<Config | null>(null);

	let cookiesText = $state('');
	let cookiesSaving = $state(false);
	let cookiesError = $state<string | null>(null);
	let cookiesMessage = $state<string | null>(null);

	let packs = $state<{ name: string; description: string; author: string; tags: string[] }[]>([]);

	// Load packs on mount
	$effect(() => {
		getPacks().then(p => packs = p).catch(() => {});
		getConfig().then(c => config = c).catch(() => {});
	});

	async function handleImportURL(e: Event) {
		e.preventDefault();
		if (!importURL.trim()) return;

		importLoading = true;
		importError = null;
		try {
			const feed = await importFromURL(importURL);
			goto(`/feeds/${feed.id}`);
		} catch (e) {
			importError = e instanceof Error ? e.message : 'Failed to import';
		} finally {
			importLoading = false;
		}
	}

	async function handleImportFile(e: Event) {
		e.preventDefault();
		if (!importFile) return;

		importLoading = true;
		importError = null;
		try {
			const feed = await importFromFile(importFile);
			goto(`/feeds/${feed.id}`);
		} catch (e) {
			importError = e instanceof Error ? e.message : 'Failed to import';
		} finally {
			importLoading = false;
		}
	}

	function handleFileChange(e: Event) {
		const target = e.target as HTMLInputElement;
		importFile = target.files?.[0] ?? null;
	}

	function handleWatchFileChange(e: Event) {
		const target = e.target as HTMLInputElement;
		watchFile = target.files?.[0] ?? null;
		watchError = null;
	}

	async function handleSaveCookies(e: Event) {
		e.preventDefault();
		cookiesError = null;
		cookiesMessage = null;

		const cookies = cookiesText.trim();
		if (!cookies) {
			cookiesError = 'Paste cookies.txt contents first.';
			return;
		}

		cookiesSaving = true;
		try {
			await setYtdlpCookies(cookies);
			cookiesText = '';
			cookiesMessage = 'Cookies saved. Streaming should work immediately.';
			config = await getConfig();
		} catch (e) {
			cookiesError = e instanceof Error ? e.message : 'Failed to save cookies.';
		} finally {
			cookiesSaving = false;
		}
	}

	async function handleClearCookies() {
		cookiesError = null;
		cookiesMessage = null;

		cookiesSaving = true;
		try {
			await clearYtdlpCookies();
			cookiesMessage = 'Cookies cleared.';
			config = await getConfig();
		} catch (e) {
			cookiesError = e instanceof Error ? e.message : 'Failed to clear cookies.';
		} finally {
			cookiesSaving = false;
		}
	}

	async function handlePackImport(packName: string) {
		importLoading = true;
		importError = null;
		try {
			const packUrl = window.location.origin + `/api/packs/${packName}`;
			const feed = await importFromURL(packUrl);
			goto(`/feeds/${feed.id}`);
		} catch (e) {
			importError = e instanceof Error ? e.message : 'Failed to import pack';
		} finally {
			importLoading = false;
		}
	}

	async function handleWatchUpload() {
		if (!watchFile) return;

		watchLoading = true;
		watchError = null;
		try {
			const result = await importWatchHistory(watchFile);
			watchChannels = result.channels;
			watchTotalVideos = result.totalVideos;
			watchSelectedChannels = new Set(watchChannels.map(c => c.url));
			watchStep = 'preview';
		} catch (e) {
			watchError = e instanceof Error ? e.message : 'Failed to parse watch history';
		} finally {
			watchLoading = false;
		}
	}

	function toggleWatchChannel(url: string) {
		const newSet = new Set(watchSelectedChannels);
		if (newSet.has(url)) {
			newSet.delete(url);
		} else {
			newSet.add(url);
		}
		watchSelectedChannels = newSet;
	}

	function selectAllWatchChannels() {
		watchSelectedChannels = new Set(watchChannels.map(c => c.url));
	}

	function selectNoneWatchChannels() {
		watchSelectedChannels = new Set();
	}

	function selectTopWatchChannels(n: number) {
		watchSelectedChannels = new Set(watchChannels.slice(0, n).map(c => c.url));
	}

	async function handleWatchOrganize() {
		if (watchSelectedChannels.size === 0) {
			watchError = 'Please select at least one channel';
			return;
		}

		watchLoading = true;
		watchError = null;
		try {
			const selectedList = watchChannels.filter(c => watchSelectedChannels.has(c.url));
			const result = await organizeWatchHistory(selectedList);
			watchGroups = result.groups;
			watchStep = 'organize';
		} catch (e) {
			watchError = e instanceof Error ? e.message : 'Failed to organize channels';
		} finally {
			watchLoading = false;
		}
	}

	function handleWatchQuickImport() {
		if (watchSelectedChannels.size === 0) {
			watchError = 'Please select at least one channel';
			return;
		}

		const selectedList = watchChannels.filter(c => watchSelectedChannels.has(c.url));
		const heavy: { url: string; name: string }[] = [];
		const regular: { url: string; name: string }[] = [];
		const occasional: { url: string; name: string }[] = [];
		const oneTime: { url: string; name: string }[] = [];

		for (const ch of selectedList) {
			const item = { url: ch.url, name: ch.name };
			if (ch.watch_count >= 20) {
				heavy.push(item);
			} else if (ch.watch_count >= 5) {
				regular.push(item);
			} else if (ch.watch_count >= 2) {
				occasional.push(item);
			} else {
				oneTime.push(item);
			}
		}

		const newGroups: GroupSuggestion[] = [];
		if (heavy.length > 0) newGroups.push({ name: 'Favorites (20+ views)', channels: heavy });
		if (regular.length > 0) newGroups.push({ name: 'Regular (5-19 views)', channels: regular });
		if (occasional.length > 0) newGroups.push({ name: 'Occasional (2-4 views)', channels: occasional });
		if (oneTime.length > 0) newGroups.push({ name: 'One-time (1 view)', channels: oneTime });

		watchGroups = newGroups;
		watchStep = 'organize';
	}

	async function handleWatchConfirm() {
		watchLoading = true;
		watchError = null;
		try {
			const groupsToCreate = watchGroups.map(g => ({
				name: g.name,
				channels: g.channels.map(c => c.url)
			}));
			const channelNames: Record<string, string> = {};
			for (const g of watchGroups) {
				for (const c of g.channels) {
					channelNames[c.url] = c.name;
				}
			}

			await confirmOrganize(groupsToCreate, channelNames);
			goto('/');
		} catch (e) {
			watchError = e instanceof Error ? e.message : 'Failed to create feeds';
		} finally {
			watchLoading = false;
		}
	}

	function removeWatchGroup(index: number) {
		watchGroups = watchGroups.filter((_, i) => i !== index);
	}

	function removeWatchChannelFromGroup(groupIndex: number, channelUrl: string) {
		watchGroups = watchGroups.map((g, i) => {
			if (i === groupIndex) {
				return {
					...g,
					channels: g.channels.filter(c => c.url !== channelUrl)
				};
			}
			return g;
		}).filter(g => g.channels.length > 0);
	}
</script>

<svelte:head>
	<title>Import - Feeds</title>
</svelte:head>

<div class="max-w-4xl mx-auto space-y-8">
	<div>
		<h1 class="text-2xl font-bold mb-2">Import</h1>
		<p class="text-gray-400">
			Bring in existing feeds or build new ones from your YouTube watch history.
		</p>
	</div>

	<section class="bg-gray-800 rounded-lg p-6">
		<div class="flex items-start justify-between gap-4 mb-4">
			<div>
				<h2 class="text-lg font-semibold mb-1">Watch History</h2>
				<p class="text-gray-400 text-sm">
					Upload Google Takeout history to discover channels you've actually watched.
				</p>
			</div>
			<span class="text-xs text-gray-400">
				Step {watchStep === 'upload' ? '1' : watchStep === 'preview' ? '2' : watchStep === 'organize' ? '3' : '4'} of 4
			</span>
		</div>

		{#if watchError}
			<div class="bg-red-900/50 border border-red-700 rounded-lg p-3 mb-4">
				<p class="text-red-400 text-sm">{watchError}</p>
			</div>
		{/if}

		<div class="flex items-center gap-2 mb-6">
			<div class="flex items-center gap-2">
				<div class={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-medium ${watchStep === 'upload' ? 'bg-blue-600' : 'bg-green-600'}`}>
					{watchStep === 'upload' ? '1' : '✓'}
				</div>
				<span class={watchStep === 'upload' ? 'text-white text-sm' : 'text-gray-400 text-sm'}>Upload</span>
			</div>
			<div class="flex-1 h-0.5 bg-gray-700"></div>
			<div class="flex items-center gap-2">
				<div class={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-medium ${watchStep === 'preview' ? 'bg-blue-600' : watchStep === 'upload' ? 'bg-gray-700' : 'bg-green-600'}`}>
					{watchStep === 'upload' || watchStep === 'preview' ? '2' : '✓'}
				</div>
				<span class={watchStep === 'preview' ? 'text-white text-sm' : 'text-gray-400 text-sm'}>Select</span>
			</div>
			<div class="flex-1 h-0.5 bg-gray-700"></div>
			<div class="flex items-center gap-2">
				<div class={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-medium ${watchStep === 'organize' ? 'bg-blue-600' : watchStep === 'confirm' ? 'bg-green-600' : 'bg-gray-700'}`}>
					{watchStep === 'confirm' ? '✓' : '3'}
				</div>
				<span class={watchStep === 'organize' ? 'text-white text-sm' : 'text-gray-400 text-sm'}>Organize</span>
			</div>
			<div class="flex-1 h-0.5 bg-gray-700"></div>
			<div class="flex items-center gap-2">
				<div class={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-medium ${watchStep === 'confirm' ? 'bg-blue-600' : 'bg-gray-700'}`}>
					4
				</div>
				<span class={watchStep === 'confirm' ? 'text-white text-sm' : 'text-gray-400 text-sm'}>Confirm</span>
			</div>
		</div>

		{#if watchStep === 'upload'}
			<div class="bg-gray-900/40 border border-gray-700 rounded-lg p-4">
				<h3 class="text-sm font-semibold mb-2 text-gray-200">Upload watch-history.json</h3>
				<div class="text-gray-400 text-sm mb-3 space-y-2">
					<p>Export from Google Takeout with YouTube history set to JSON.</p>
					<ol class="list-decimal list-inside space-y-1 ml-2">
						<li>Go to <a href="https://takeout.google.com" target="_blank" rel="noopener" class="text-blue-400 hover:underline">Google Takeout</a></li>
						<li>Select only "YouTube and YouTube Music"</li>
						<li>Pick history and change format to JSON</li>
						<li>Upload <code class="bg-gray-700 px-1 rounded">watch-history.json</code></li>
					</ol>
				</div>
				<input
					type="file"
					accept=".json"
					onchange={handleWatchFileChange}
					class="w-full bg-gray-900 border border-gray-700 rounded px-4 py-2 mb-3 text-white file:mr-4 file:py-1 file:px-3 file:rounded file:border-0 file:bg-blue-600 file:text-white file:cursor-pointer"
				/>
				<button
					onclick={handleWatchUpload}
					disabled={watchLoading || !watchFile}
					class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
				>
					{#if watchLoading}
						<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
					{/if}
					Parse Watch History
				</button>
			</div>
		{/if}

		{#if watchStep === 'preview'}
			<div class="bg-gray-900/40 border border-gray-700 rounded-lg p-4">
				<div class="flex flex-wrap items-center justify-between gap-3 mb-3">
					<div>
						<h3 class="text-sm font-semibold">Select Channels</h3>
						<p class="text-gray-400 text-sm">
							Found {watchChannels.length} channels from {watchTotalVideos.toLocaleString()} videos
						</p>
					</div>
					<div class="flex gap-2 text-sm">
						<button onclick={selectAllWatchChannels} class="text-blue-400 hover:underline">All</button>
						<button onclick={selectNoneWatchChannels} class="text-blue-400 hover:underline">None</button>
						<button onclick={() => selectTopWatchChannels(50)} class="text-blue-400 hover:underline">Top 50</button>
						<button onclick={() => selectTopWatchChannels(100)} class="text-blue-400 hover:underline">Top 100</button>
					</div>
				</div>

				<p class="text-gray-400 text-sm mb-3">
					{watchSelectedChannels.size} of {watchChannels.length} selected
				</p>

				<div class="max-h-80 overflow-y-auto border border-gray-700 rounded mb-4">
					{#each watchChannels as channel, i}
						<label
							class="flex items-center gap-3 p-3 hover:bg-gray-800/80 cursor-pointer border-b border-gray-700 last:border-b-0"
						>
							<input
								type="checkbox"
								checked={watchSelectedChannels.has(channel.url)}
								onchange={() => toggleWatchChannel(channel.url)}
								class="w-4 h-4 rounded border-gray-600 bg-gray-900 text-blue-600 focus:ring-blue-500"
							/>
							<div class="flex-1 min-w-0">
								<div class="font-medium truncate">{channel.name}</div>
								<div class="text-sm text-gray-400">{channel.watch_count} videos watched</div>
							</div>
							<div class="text-sm text-gray-500">#{i + 1}</div>
						</label>
					{/each}
				</div>

				<div class="flex flex-wrap gap-3">
					<button
						onclick={() => { watchStep = 'upload'; }}
						class="bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded"
					>
						Back
					</button>
					<button
						onclick={handleWatchQuickImport}
						disabled={watchSelectedChannels.size === 0}
						class="bg-green-600 hover:bg-green-700 disabled:opacity-50 text-white px-4 py-2 rounded"
					>
						Quick Import by Frequency
					</button>
					{#if config?.aiEnabled}
						<button
							onclick={handleWatchOrganize}
							disabled={watchLoading || watchSelectedChannels.size === 0}
							class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
						>
							{#if watchLoading}
								<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
								</svg>
								Organizing...
							{:else}
								Organize with AI
							{/if}
						</button>
					{/if}
				</div>
			</div>
		{/if}

		{#if watchStep === 'organize'}
			<div class="bg-gray-900/40 border border-gray-700 rounded-lg p-4">
				<h3 class="text-sm font-semibold mb-2">Review Groups</h3>
				<p class="text-gray-400 text-sm mb-4">
					AI has organized your channels into {watchGroups.length} groups. Remove any you don't want.
				</p>

				<div class="space-y-4 mb-6">
					{#each watchGroups as group, groupIndex}
						<div class="border border-gray-700 rounded-lg p-4">
							<div class="flex items-center justify-between mb-3">
								<h4 class="font-semibold">{group.name}</h4>
								<button
									onclick={() => removeWatchGroup(groupIndex)}
									class="text-red-400 hover:text-red-300 text-sm"
								>
									Remove group
								</button>
							</div>
							<div class="flex flex-wrap gap-2">
								{#each group.channels as channel}
									<span class="inline-flex items-center gap-1 bg-gray-700 rounded px-2 py-1 text-sm">
										{channel.name}
										<button
											onclick={() => removeWatchChannelFromGroup(groupIndex, channel.url)}
											class="text-gray-400 hover:text-white ml-1"
										>
											×
										</button>
									</span>
								{/each}
							</div>
						</div>
					{/each}
				</div>

				<div class="flex gap-3">
					<button
						onclick={() => { watchStep = 'preview'; }}
						class="bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded"
					>
						Back
					</button>
					<button
						onclick={() => { watchStep = 'confirm'; }}
						disabled={watchGroups.length === 0}
						class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded"
					>
						Continue
					</button>
				</div>
			</div>
		{/if}

		{#if watchStep === 'confirm'}
			<div class="bg-gray-900/40 border border-gray-700 rounded-lg p-4">
				<h3 class="text-sm font-semibold mb-2">Confirm Import</h3>
				<p class="text-gray-400 text-sm mb-4">
					This will create {watchGroups.length} feeds with a total of {watchGroups.reduce((acc, g) => acc + g.channels.length, 0)} channels.
				</p>

				<div class="space-y-3 mb-6">
					{#each watchGroups as group}
						<div class="flex items-center justify-between p-3 bg-gray-800/80 rounded">
							<span class="font-medium">{group.name}</span>
							<span class="text-gray-400 text-sm">{group.channels.length} channels</span>
						</div>
					{/each}
				</div>

				<div class="flex gap-3">
					<button
						onclick={() => { watchStep = 'organize'; }}
						class="bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded"
					>
						Back
					</button>
					<button
						onclick={handleWatchConfirm}
						disabled={watchLoading}
						class="bg-green-600 hover:bg-green-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
					>
						{#if watchLoading}
							<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							Creating feeds...
						{:else}
							Create Feeds
						{/if}
					</button>
				</div>
			</div>
		{/if}
	</section>

	<!-- YouTube cookies -->
	<div class="bg-gray-800 rounded-lg p-6 mb-6">
		<h2 class="text-lg font-semibold mb-2">YouTube Cookies (optional)</h2>
		<p class="text-gray-400 text-sm mb-4">
			Paste a Netscape-format cookies.txt export here to unlock streaming when YouTube blocks your IP.
			This stays on your server.
		</p>

		{#if cookiesError}
			<div class="bg-red-900/50 border border-red-700 rounded-lg p-3 mb-4">
				<p class="text-red-400 text-sm">{cookiesError}</p>
			</div>
		{/if}
		{#if cookiesMessage}
			<div class="bg-green-900/40 border border-green-700 rounded-lg p-3 mb-4">
				<p class="text-green-300 text-sm">{cookiesMessage}</p>
			</div>
		{/if}

		<form onsubmit={handleSaveCookies}>
			<textarea
				bind:value={cookiesText}
				rows="6"
				placeholder="# Netscape HTTP Cookie File&#10;.youtube.com&#9;TRUE&#9;/&#9;FALSE&#9;0&#9;VISITOR_INFO1_LIVE&#9;..."
				class="w-full bg-gray-900 border border-gray-700 rounded px-4 py-2 mb-3 text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
			></textarea>
			<div class="flex items-center gap-3">
				<button
					type="submit"
					disabled={cookiesSaving}
					class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
				>
					{#if cookiesSaving}
						<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
					{/if}
					Save Cookies
				</button>
				<button
					type="button"
					onclick={handleClearCookies}
					disabled={cookiesSaving || !config?.ytdlpCookiesConfigured}
					class="bg-gray-700 hover:bg-gray-600 disabled:opacity-50 text-white px-4 py-2 rounded"
				>
					Clear
				</button>
				<span class="text-xs text-gray-400">
					Status: {config?.ytdlpCookiesConfigured ? 'configured' : 'not set'}
				</span>
			</div>
		</form>
	</div>

	{#if importError}
		<div class="bg-red-900/50 border border-red-700 rounded-lg p-4">
			<p class="text-red-400">{importError}</p>
		</div>
	{/if}

	<div class="grid gap-6 md:grid-cols-2">
		<div class="bg-gray-800 rounded-lg p-6">
			<h2 class="text-lg font-semibold mb-4">Import from URL</h2>
			<form onsubmit={handleImportURL}>
				<input
					type="url"
					bind:value={importURL}
					placeholder="https://example.com/feed.json"
					class="w-full bg-gray-900 border border-gray-700 rounded px-4 py-2 mb-4 text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
				/>
				<button
					type="submit"
					disabled={importLoading || !importURL.trim()}
					class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
				>
					{#if importLoading}
						<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
					{/if}
					Import
				</button>
			</form>
		</div>

		<div class="bg-gray-800 rounded-lg p-6">
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
					disabled={importLoading || !importFile}
					class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
				>
					{#if importLoading}
						<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
					{/if}
					Upload & Import
				</button>
			</form>
		</div>
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
						disabled={importLoading}
						class="text-left bg-gray-700 hover:bg-gray-600 disabled:opacity-50 rounded-lg p-4 transition-colors"
					>
						<h3 class="font-semibold">{pack.name}</h3>
						<p class="text-sm text-gray-400">{pack.description}</p>
							{#if pack.tags && pack.tags.length > 0}
								<div class="flex gap-1 mt-2">
									{#each pack.tags ?? [] as tag}
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
