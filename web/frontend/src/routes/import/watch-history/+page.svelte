<script lang="ts">
	import { goto } from '$app/navigation';
	import { importWatchHistory, organizeWatchHistory, confirmOrganize, getConfig } from '$lib/api';
	import type { WatchHistoryChannel, GroupSuggestion, Config } from '$lib/types';

	type Step = 'upload' | 'preview' | 'organize' | 'confirm';

	let step = $state<Step>('upload');
	let file = $state<File | null>(null);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let config = $state<Config | null>(null);

	// Data from each step
	let channels = $state<WatchHistoryChannel[]>([]);
	let totalVideos = $state(0);
	let groups = $state<GroupSuggestion[]>([]);

	// Channel selection for organizing
	let selectedChannels = $state<Set<string>>(new Set());

	// Load config on mount
	$effect(() => {
		getConfig().then(c => config = c).catch(() => {});
	});

	function handleFileChange(e: Event) {
		const target = e.target as HTMLInputElement;
		file = target.files?.[0] ?? null;
		error = null;
	}

	async function handleUpload() {
		if (!file) return;

		loading = true;
		error = null;
		try {
			const result = await importWatchHistory(file);
			channels = result.channels;
			totalVideos = result.totalVideos;
			// Pre-select all channels
			selectedChannels = new Set(channels.map(c => c.url));
			step = 'preview';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to parse watch history';
		} finally {
			loading = false;
		}
	}

	function toggleChannel(url: string) {
		const newSet = new Set(selectedChannels);
		if (newSet.has(url)) {
			newSet.delete(url);
		} else {
			newSet.add(url);
		}
		selectedChannels = newSet;
	}

	function selectAll() {
		selectedChannels = new Set(channels.map(c => c.url));
	}

	function selectNone() {
		selectedChannels = new Set();
	}

	function selectTop(n: number) {
		selectedChannels = new Set(channels.slice(0, n).map(c => c.url));
	}

	async function handleOrganize() {
		if (selectedChannels.size === 0) {
			error = 'Please select at least one channel';
			return;
		}

		loading = true;
		error = null;
		try {
			const selectedList = channels.filter(c => selectedChannels.has(c.url));
			const result = await organizeWatchHistory(selectedList);
			groups = result.groups;
			step = 'organize';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to organize channels';
		} finally {
			loading = false;
		}
	}

	function handleQuickImport() {
		if (selectedChannels.size === 0) {
			error = 'Please select at least one channel';
			return;
		}

		const selectedList = channels.filter(c => selectedChannels.has(c.url));

		// Group by watch frequency tiers
		const heavy: { url: string; name: string }[] = [];      // 20+ views
		const regular: { url: string; name: string }[] = [];    // 5-19 views
		const occasional: { url: string; name: string }[] = []; // 2-4 views
		const oneTime: { url: string; name: string }[] = [];    // 1 view

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

		// Build groups (only include non-empty ones)
		const newGroups: GroupSuggestion[] = [];
		if (heavy.length > 0) newGroups.push({ name: 'Favorites (20+ views)', channels: heavy });
		if (regular.length > 0) newGroups.push({ name: 'Regular (5-19 views)', channels: regular });
		if (occasional.length > 0) newGroups.push({ name: 'Occasional (2-4 views)', channels: occasional });
		if (oneTime.length > 0) newGroups.push({ name: 'One-time (1 view)', channels: oneTime });

		groups = newGroups;
		step = 'organize';
	}

	async function handleConfirm() {
		loading = true;
		error = null;
		try {
			// Build the groups and channel names map
			const groupsToCreate = groups.map(g => ({
				name: g.name,
				channels: g.channels.map(c => c.url)
			}));
			const channelNames: Record<string, string> = {};
			for (const g of groups) {
				for (const c of g.channels) {
					channelNames[c.url] = c.name;
				}
			}

			await confirmOrganize(groupsToCreate, channelNames);
			goto('/');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create feeds';
		} finally {
			loading = false;
		}
	}

	function removeGroup(index: number) {
		groups = groups.filter((_, i) => i !== index);
	}

	function removeChannelFromGroup(groupIndex: number, channelUrl: string) {
		groups = groups.map((g, i) => {
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
	<title>Import Watch History - Feeds</title>
</svelte:head>

<div class="max-w-4xl mx-auto">
	<h1 class="text-2xl font-bold mb-2">Import Watch History</h1>
	<p class="text-gray-400 mb-6">
		Import your YouTube watch history from Google Takeout to discover channels you've watched.
	</p>

	{#if error}
		<div class="bg-red-900/50 border border-red-700 rounded-lg p-4 mb-6">
			<p class="text-red-400">{error}</p>
		</div>
	{/if}

	<!-- Progress indicator -->
	<div class="flex items-center gap-2 mb-6">
		<div class="flex items-center gap-2">
			<div class={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium ${step === 'upload' ? 'bg-blue-600' : 'bg-green-600'}`}>
				{step === 'upload' ? '1' : '✓'}
			</div>
			<span class={step === 'upload' ? 'text-white' : 'text-gray-400'}>Upload</span>
		</div>
		<div class="flex-1 h-0.5 bg-gray-700"></div>
		<div class="flex items-center gap-2">
			<div class={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium ${step === 'preview' ? 'bg-blue-600' : step === 'upload' ? 'bg-gray-700' : 'bg-green-600'}`}>
				{step === 'upload' || step === 'preview' ? '2' : '✓'}
			</div>
			<span class={step === 'preview' ? 'text-white' : 'text-gray-400'}>Select</span>
		</div>
		<div class="flex-1 h-0.5 bg-gray-700"></div>
		<div class="flex items-center gap-2">
			<div class={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium ${step === 'organize' ? 'bg-blue-600' : step === 'confirm' ? 'bg-green-600' : 'bg-gray-700'}`}>
				{step === 'confirm' ? '✓' : '3'}
			</div>
			<span class={step === 'organize' ? 'text-white' : 'text-gray-400'}>Organize</span>
		</div>
		<div class="flex-1 h-0.5 bg-gray-700"></div>
		<div class="flex items-center gap-2">
			<div class={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium ${step === 'confirm' ? 'bg-blue-600' : 'bg-gray-700'}`}>
				4
			</div>
			<span class={step === 'confirm' ? 'text-white' : 'text-gray-400'}>Confirm</span>
		</div>
	</div>

	<!-- Step 1: Upload -->
	{#if step === 'upload'}
		<div class="bg-gray-800 rounded-lg p-6">
			<h2 class="text-lg font-semibold mb-4">Upload Watch History</h2>
			<div class="text-gray-400 text-sm mb-4 space-y-2">
				<p>To export your YouTube watch history:</p>
				<ol class="list-decimal list-inside space-y-1 ml-2">
					<li>Go to <a href="https://takeout.google.com" target="_blank" rel="noopener" class="text-blue-400 hover:underline">Google Takeout</a></li>
					<li>Deselect all, then select only "YouTube and YouTube Music"</li>
					<li>Click "All YouTube data included" and select only "history"</li>
					<li>Change format from HTML to JSON</li>
					<li>Download and extract the archive</li>
					<li>Upload the <code class="bg-gray-700 px-1 rounded">watch-history.json</code> file</li>
				</ol>
			</div>
			<input
				type="file"
				accept=".json"
				onchange={handleFileChange}
				class="w-full bg-gray-900 border border-gray-700 rounded px-4 py-2 mb-4 text-white file:mr-4 file:py-1 file:px-3 file:rounded file:border-0 file:bg-blue-600 file:text-white file:cursor-pointer"
			/>
			<button
				onclick={handleUpload}
				disabled={loading || !file}
				class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
			>
				{#if loading}
					<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				{/if}
				Parse Watch History
			</button>
		</div>
	{/if}

	<!-- Step 2: Preview & Select -->
	{#if step === 'preview'}
		<div class="bg-gray-800 rounded-lg p-6">
			<div class="flex items-center justify-between mb-4">
				<div>
					<h2 class="text-lg font-semibold">Select Channels</h2>
					<p class="text-gray-400 text-sm">
						Found {channels.length} channels from {totalVideos.toLocaleString()} videos
					</p>
				</div>
				<div class="flex gap-2">
					<button onclick={selectAll} class="text-sm text-blue-400 hover:underline">All</button>
					<button onclick={selectNone} class="text-sm text-blue-400 hover:underline">None</button>
					<button onclick={() => selectTop(50)} class="text-sm text-blue-400 hover:underline">Top 50</button>
					<button onclick={() => selectTop(100)} class="text-sm text-blue-400 hover:underline">Top 100</button>
				</div>
			</div>

			<p class="text-gray-400 text-sm mb-4">
				{selectedChannels.size} of {channels.length} selected
			</p>

			<div class="max-h-96 overflow-y-auto border border-gray-700 rounded mb-4">
				{#each channels as channel, i}
					<label
						class="flex items-center gap-3 p-3 hover:bg-gray-700/50 cursor-pointer border-b border-gray-700 last:border-b-0"
					>
						<input
							type="checkbox"
							checked={selectedChannels.has(channel.url)}
							onchange={() => toggleChannel(channel.url)}
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
					onclick={() => { step = 'upload'; }}
					class="bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded"
				>
					Back
				</button>
				<button
					onclick={handleQuickImport}
					disabled={selectedChannels.size === 0}
					class="bg-green-600 hover:bg-green-700 disabled:opacity-50 text-white px-4 py-2 rounded"
				>
					Quick Import by Frequency
				</button>
				{#if config?.aiEnabled}
					<button
						onclick={handleOrganize}
						disabled={loading || selectedChannels.size === 0}
						class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
					>
						{#if loading}
							<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							Organizing...
						{:else}
							Organize with AI (slow)
						{/if}
					</button>
				{/if}
			</div>
		</div>
	{/if}

	<!-- Step 3: Organize (Review AI suggestions) -->
	{#if step === 'organize'}
		<div class="bg-gray-800 rounded-lg p-6">
			<h2 class="text-lg font-semibold mb-2">Review Groups</h2>
			<p class="text-gray-400 text-sm mb-4">
				AI has organized your channels into {groups.length} groups. Remove any you don't want.
			</p>

			<div class="space-y-4 mb-6">
				{#each groups as group, groupIndex}
					<div class="border border-gray-700 rounded-lg p-4">
						<div class="flex items-center justify-between mb-3">
							<h3 class="font-semibold text-lg">{group.name}</h3>
							<button
								onclick={() => removeGroup(groupIndex)}
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
										onclick={() => removeChannelFromGroup(groupIndex, channel.url)}
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
					onclick={() => { step = 'preview'; }}
					class="bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded"
				>
					Back
				</button>
				<button
					onclick={() => { step = 'confirm'; }}
					disabled={groups.length === 0}
					class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-4 py-2 rounded"
				>
					Continue
				</button>
			</div>
		</div>
	{/if}

	<!-- Step 4: Confirm -->
	{#if step === 'confirm'}
		<div class="bg-gray-800 rounded-lg p-6">
			<h2 class="text-lg font-semibold mb-2">Confirm Import</h2>
			<p class="text-gray-400 text-sm mb-4">
				This will create {groups.length} feeds with a total of {groups.reduce((acc, g) => acc + g.channels.length, 0)} channels.
			</p>

			<div class="space-y-3 mb-6">
				{#each groups as group}
					<div class="flex items-center justify-between p-3 bg-gray-700/50 rounded">
						<span class="font-medium">{group.name}</span>
						<span class="text-gray-400 text-sm">{group.channels.length} channels</span>
					</div>
				{/each}
			</div>

			<div class="flex gap-3">
				<button
					onclick={() => { step = 'organize'; }}
					class="bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded"
				>
					Back
				</button>
				<button
					onclick={handleConfirm}
					disabled={loading}
					class="bg-green-600 hover:bg-green-700 disabled:opacity-50 text-white px-4 py-2 rounded inline-flex items-center gap-2"
				>
					{#if loading}
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
</div>
