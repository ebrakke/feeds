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
		const heavyRotation: { url: string; name: string }[] = [];
		const regulars: { url: string; name: string }[] = [];
		const frequent: { url: string; name: string }[] = [];
		const occasional: { url: string; name: string }[] = [];
		const fewTimes: { url: string; name: string }[] = [];
		const discovered: { url: string; name: string }[] = [];

		for (const ch of selectedList) {
			const item = { url: ch.url, name: ch.name };
			if (ch.watch_count >= 50) {
				heavyRotation.push(item);
			} else if (ch.watch_count >= 20) {
				regulars.push(item);
			} else if (ch.watch_count >= 10) {
				frequent.push(item);
			} else if (ch.watch_count >= 5) {
				occasional.push(item);
			} else if (ch.watch_count >= 2) {
				fewTimes.push(item);
			} else {
				discovered.push(item);
			}
		}

		const newGroups: GroupSuggestion[] = [];
		if (heavyRotation.length > 0) newGroups.push({ name: 'Heavy Rotation (50+)', channels: heavyRotation });
		if (regulars.length > 0) newGroups.push({ name: 'Regulars (20-49)', channels: regulars });
		if (frequent.length > 0) newGroups.push({ name: 'Frequent (10-19)', channels: frequent });
		if (occasional.length > 0) newGroups.push({ name: 'Occasional (5-9)', channels: occasional });
		if (fewTimes.length > 0) newGroups.push({ name: 'A Few Times (2-4)', channels: fewTimes });
		if (discovered.length > 0) newGroups.push({ name: 'Discovered (1)', channels: discovered });

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

	let stepNumber = $derived(
		watchStep === 'upload' ? 1 : watchStep === 'preview' ? 2 : watchStep === 'organize' ? 3 : 4
	);
</script>

<svelte:head>
	<title>Import - Feeds</title>
</svelte:head>

<!-- Header -->
<header class="mb-6 sm:mb-8 animate-fade-up" style="opacity: 0;">
	<h1 class="text-xl sm:text-2xl font-display font-bold mb-1.5 sm:mb-2">Import</h1>
	<p class="text-text-muted text-sm sm:text-base">
		Bring in existing feeds or build new ones from your YouTube watch history.
	</p>
</header>

<!-- Global Import Error -->
{#if importError}
	<div class="card bg-crimson-500/10 border border-crimson-500/30 p-4 mb-6 animate-fade-up" style="opacity: 0;">
		<div class="flex items-center gap-3">
			<svg class="w-5 h-5 text-crimson-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<circle cx="12" cy="12" r="10"/>
				<line x1="12" y1="8" x2="12" y2="12"/>
				<line x1="12" y1="16" x2="12.01" y2="16"/>
			</svg>
			<p class="text-crimson-400">{importError}</p>
		</div>
	</div>
{/if}

<div class="space-y-6 sm:space-y-8">
	<!-- Watch History Import Section -->
	<section class="card p-4 sm:p-6 animate-fade-up stagger-1" style="opacity: 0;">
		<div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3 sm:gap-4 mb-5 sm:mb-6">
			<div>
				<div class="flex items-center gap-3 mb-1">
					<div class="w-10 h-10 rounded-xl bg-emerald-500/10 flex items-center justify-center flex-shrink-0">
						<svg class="w-5 h-5 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<circle cx="12" cy="12" r="10"/>
							<polyline points="12 6 12 12 16 14"/>
						</svg>
					</div>
					<h2 class="text-lg font-display font-semibold">Watch History</h2>
				</div>
				<p class="text-text-muted text-sm mt-2 sm:mt-0 sm:ml-13">
					Upload Google Takeout history to discover channels you've actually watched.
				</p>
			</div>
			<span class="text-xs text-text-dim font-mono bg-surface px-2.5 py-1.5 rounded self-start whitespace-nowrap">
				Step {stepNumber}/4
			</span>
		</div>

		{#if watchError}
			<div class="bg-crimson-500/10 border border-crimson-500/30 rounded-xl p-3 mb-4">
				<p class="text-crimson-400 text-sm">{watchError}</p>
			</div>
		{/if}

		<!-- Progress Steps - mobile optimized with compact view -->
		<div class="flex items-center gap-1 sm:gap-2 mb-5 sm:mb-6 overflow-x-auto pb-1 -mx-1 px-1">
			{#each ['Upload', 'Select', 'Organize', 'Confirm'] as label, i}
				{@const stepNum = i + 1}
				{@const isActive = stepNumber === stepNum}
				{@const isComplete = stepNumber > stepNum}
				<div class="flex items-center gap-1.5 sm:gap-2 flex-shrink-0">
					<div class="w-8 h-8 sm:w-9 sm:h-9 rounded-full flex items-center justify-center text-xs sm:text-sm font-semibold transition-all
						{isActive ? 'bg-emerald-500 text-void shadow-lg shadow-emerald-500/20' : isComplete ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'bg-surface text-text-dim border border-white/5'}">
						{#if isComplete}
							<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
								<polyline points="20 6 9 17 4 12"/>
							</svg>
						{:else}
							{stepNum}
						{/if}
					</div>
					<span class="text-xs sm:text-sm {isActive ? 'text-text-primary font-medium' : 'text-text-dim'} hidden xs:inline sm:inline">{label}</span>
				</div>
				{#if i < 3}
					<div class="flex-1 min-w-3 sm:min-w-6 h-px bg-border"></div>
				{/if}
			{/each}
		</div>

		<!-- Step Content -->
		{#if watchStep === 'upload'}
			<div class="bg-surface rounded-xl p-5 border border-white/5">
				<h3 class="text-sm font-display font-semibold mb-3">Get Your YouTube Watch History</h3>
				<p class="text-text-muted text-sm mb-4">
					Import your watch history to discover channels you actually care about. We'll organize them by how often you watch.
				</p>

				<div class="bg-void rounded-xl p-4 mb-5 border border-white/5">
					<h4 class="text-xs font-semibold text-text-secondary uppercase tracking-wide mb-3">How to export from Google Takeout</h4>
					<ol class="space-y-3 text-sm">
						<li class="flex gap-3">
							<span class="flex-shrink-0 w-6 h-6 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center text-xs font-semibold">1</span>
							<div>
								<span class="text-text-primary">Go to </span>
								<a href="https://takeout.google.com" target="_blank" rel="noopener" class="text-emerald-400 hover:text-emerald-300 transition-colors font-medium">takeout.google.com</a>
							</div>
						</li>
						<li class="flex gap-3">
							<span class="flex-shrink-0 w-6 h-6 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center text-xs font-semibold">2</span>
							<div class="text-text-primary">
								Click <span class="text-text-secondary">"Deselect all"</span>, then scroll down and check only <span class="text-text-secondary">"YouTube and YouTube Music"</span>
							</div>
						</li>
						<li class="flex gap-3">
							<span class="flex-shrink-0 w-6 h-6 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center text-xs font-semibold">3</span>
							<div class="text-text-primary">
								Click <span class="text-text-secondary">"All YouTube data included"</span> → deselect everything except <span class="text-text-secondary">"history"</span>
							</div>
						</li>
						<li class="flex gap-3">
							<span class="flex-shrink-0 w-6 h-6 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center text-xs font-semibold">4</span>
							<div class="text-text-primary">
								Click <span class="text-text-secondary">"Multiple formats"</span> → find history and change format from <span class="text-crimson-400">HTML</span> to <span class="text-emerald-400 font-medium">JSON</span>
								<span class="block text-text-dim text-xs mt-1">This step is important - the default HTML format won't work</span>
							</div>
						</li>
						<li class="flex gap-3">
							<span class="flex-shrink-0 w-6 h-6 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center text-xs font-semibold">5</span>
							<div class="text-text-primary">
								Click <span class="text-text-secondary">"Next step"</span> → <span class="text-text-secondary">"Create export"</span> → wait for email → download and unzip
							</div>
						</li>
						<li class="flex gap-3">
							<span class="flex-shrink-0 w-6 h-6 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center text-xs font-semibold">6</span>
							<div class="text-text-primary">
								Find <code class="bg-surface px-1.5 py-0.5 rounded text-emerald-400 text-xs">watch-history.json</code> in the <code class="bg-surface px-1.5 py-0.5 rounded text-text-secondary text-xs">YouTube and YouTube Music/history/</code> folder
							</div>
						</li>
					</ol>
				</div>

				<input
					type="file"
					accept=".json"
					onchange={handleWatchFileChange}
					class="w-full bg-void border border-white/10 rounded-lg px-4 py-3 mb-4 text-text-primary
						file:mr-4 file:py-1.5 file:px-4 file:rounded-lg file:border-0 file:bg-emerald-500 file:text-void file:font-medium file:cursor-pointer
						file:hover:bg-emerald-400 file:transition-colors focus:outline-none focus:border-emerald-500/50"
				/>
				<button
					onclick={handleWatchUpload}
					disabled={watchLoading || !watchFile}
					class="btn btn-primary"
				>
					{#if watchLoading}
						<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
						</svg>
					{/if}
					Parse Watch History
				</button>
			</div>
		{/if}

		{#if watchStep === 'preview'}
			<div class="bg-surface rounded-xl p-5 border border-white/5">
				<div class="flex flex-wrap items-center justify-between gap-3 mb-4">
					<div>
						<h3 class="text-sm font-display font-semibold">Select Channels</h3>
						<p class="text-text-muted text-sm">
							Found {watchChannels.length} channels from {watchTotalVideos.toLocaleString()} videos
						</p>
					</div>
					<div class="flex gap-3 text-sm">
						<button onclick={selectAllWatchChannels} class="text-emerald-400 hover:text-emerald-300 transition-colors">All</button>
						<button onclick={selectNoneWatchChannels} class="text-emerald-400 hover:text-emerald-300 transition-colors">None</button>
						<button onclick={() => selectTopWatchChannels(50)} class="text-emerald-400 hover:text-emerald-300 transition-colors">Top 50</button>
						<button onclick={() => selectTopWatchChannels(100)} class="text-emerald-400 hover:text-emerald-300 transition-colors">Top 100</button>
					</div>
				</div>

				<p class="text-text-secondary text-sm mb-3">
					<span class="text-emerald-400 font-medium">{watchSelectedChannels.size}</span> of {watchChannels.length} selected
				</p>

				<div class="max-h-80 overflow-y-auto border border-white/5 rounded-xl mb-5 bg-void">
					{#each watchChannels as channel, i}
						<label
							class="flex items-center gap-3 p-3 hover:bg-surface cursor-pointer border-b border-white/5 last:border-b-0 transition-colors"
						>
							<input
								type="checkbox"
								checked={watchSelectedChannels.has(channel.url)}
								onchange={() => toggleWatchChannel(channel.url)}
								class="checkbox"
							/>
							<div class="flex-1 min-w-0">
								<div class="font-medium truncate text-text-primary">{channel.name}</div>
								<div class="text-sm text-text-muted">{channel.watch_count} videos watched</div>
							</div>
							<div class="text-sm text-text-dim font-mono">#{i + 1}</div>
						</label>
					{/each}
				</div>

				<div class="flex flex-wrap gap-3">
					<button
						onclick={() => { watchStep = 'upload'; }}
						class="btn btn-secondary"
					>
						Back
					</button>
					<button
						onclick={handleWatchQuickImport}
						disabled={watchSelectedChannels.size === 0}
						class="btn bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 hover:bg-emerald-500/20 disabled:opacity-50"
					>
						Quick Import by Frequency
					</button>
					{#if config?.aiEnabled}
						<button
							onclick={handleWatchOrganize}
							disabled={watchLoading || watchSelectedChannels.size === 0}
							class="btn btn-primary"
						>
							{#if watchLoading}
								<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
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
			<div class="bg-surface rounded-xl p-5 border border-white/5">
				<h3 class="text-sm font-display font-semibold mb-2">Review Groups</h3>
				<p class="text-text-muted text-sm mb-5">
					Your channels have been organized into {watchGroups.length} groups. Remove any you don't want.
				</p>

				<div class="space-y-4 mb-6">
					{#each watchGroups as group, groupIndex}
						<div class="border border-white/5 rounded-xl p-4 bg-void">
							<div class="flex items-center justify-between mb-3">
								<h4 class="font-display font-semibold text-text-primary">{group.name}</h4>
								<button
									onclick={() => removeWatchGroup(groupIndex)}
									class="text-crimson-400 hover:text-crimson-300 text-sm transition-colors"
								>
									Remove group
								</button>
							</div>
							<div class="flex flex-wrap gap-2">
								{#each group.channels as channel}
									<span class="inline-flex items-center gap-1 bg-surface rounded-lg px-3 py-1.5 text-sm text-text-secondary border border-white/5">
										{channel.name}
										<button
											onclick={() => removeWatchChannelFromGroup(groupIndex, channel.url)}
											class="text-text-dim hover:text-crimson-400 ml-1 transition-colors"
										>
											<svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<path d="M18 6L6 18M6 6l12 12"/>
											</svg>
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
						class="btn btn-secondary"
					>
						Back
					</button>
					<button
						onclick={() => { watchStep = 'confirm'; }}
						disabled={watchGroups.length === 0}
						class="btn btn-primary"
					>
						Continue
					</button>
				</div>
			</div>
		{/if}

		{#if watchStep === 'confirm'}
			<div class="bg-surface rounded-xl p-5 border border-white/5">
				<h3 class="text-sm font-display font-semibold mb-2">Confirm Import</h3>
				<p class="text-text-muted text-sm mb-5">
					This will create <span class="text-emerald-400 font-medium">{watchGroups.length}</span> feeds with a total of <span class="text-emerald-400 font-medium">{watchGroups.reduce((acc, g) => acc + g.channels.length, 0)}</span> channels.
				</p>

				<div class="space-y-2 mb-6">
					{#each watchGroups as group}
						<div class="flex items-center justify-between p-3 bg-void rounded-xl border border-white/5">
							<span class="font-display font-medium text-text-primary">{group.name}</span>
							<span class="text-text-muted text-sm">{group.channels.length} channels</span>
						</div>
					{/each}
				</div>

				<div class="flex gap-3">
					<button
						onclick={() => { watchStep = 'organize'; }}
						class="btn btn-secondary"
					>
						Back
					</button>
					<button
						onclick={handleWatchConfirm}
						disabled={watchLoading}
						class="btn bg-emerald-500 text-void hover:bg-emerald-400 disabled:opacity-50"
					>
						{#if watchLoading}
							<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
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

	<!-- YouTube Cookies Section -->
	<section class="card p-6 animate-fade-up stagger-2" style="opacity: 0;">
		<div class="flex items-center gap-3 mb-4">
			<div class="w-10 h-10 rounded-xl bg-surface flex items-center justify-center">
				<svg class="w-5 h-5 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
				</svg>
			</div>
			<div>
				<h2 class="text-lg font-display font-semibold">YouTube Cookies</h2>
				<span class="text-xs text-text-dim">Optional</span>
			</div>
		</div>
		<p class="text-text-muted text-sm mb-4">
			Paste a Netscape-format cookies.txt export here to unlock streaming when YouTube blocks your IP.
			This stays on your server.
		</p>

		{#if cookiesError}
			<div class="bg-crimson-500/10 border border-crimson-500/30 rounded-xl p-3 mb-4">
				<p class="text-crimson-400 text-sm">{cookiesError}</p>
			</div>
		{/if}
		{#if cookiesMessage}
			<div class="bg-emerald-500/10 border border-emerald-500/30 rounded-xl p-3 mb-4">
				<p class="text-emerald-400 text-sm">{cookiesMessage}</p>
			</div>
		{/if}

		<form onsubmit={handleSaveCookies}>
			<textarea
				bind:value={cookiesText}
				rows="5"
				placeholder="# Netscape HTTP Cookie File&#10;.youtube.com&#9;TRUE&#9;/&#9;FALSE&#9;0&#9;VISITOR_INFO1_LIVE&#9;..."
				class="w-full bg-void border border-white/10 rounded-xl px-4 py-3 mb-4 text-text-primary placeholder-text-dim font-mono text-sm focus:outline-none focus:border-emerald-500/50 transition-colors"
			></textarea>
			<div class="flex items-center gap-3">
				<button
					type="submit"
					disabled={cookiesSaving}
					class="btn btn-primary"
				>
					{#if cookiesSaving}
						<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
						</svg>
					{/if}
					Save Cookies
				</button>
				<button
					type="button"
					onclick={handleClearCookies}
					disabled={cookiesSaving || !config?.ytdlpCookiesConfigured}
					class="btn btn-secondary"
				>
					Clear
				</button>
				<span class="text-xs text-text-dim flex items-center gap-2">
					<span class="w-2 h-2 rounded-full {config?.ytdlpCookiesConfigured ? 'bg-emerald-400' : 'bg-text-dim'}"></span>
					{config?.ytdlpCookiesConfigured ? 'Configured' : 'Not set'}
				</span>
			</div>
		</form>
	</section>

	<!-- Import Methods Grid -->
	<div class="grid gap-6 md:grid-cols-2 animate-fade-up stagger-3" style="opacity: 0;">
		<!-- Import from URL -->
		<section class="card p-6">
			<div class="flex items-center gap-3 mb-4">
				<div class="w-10 h-10 rounded-xl bg-surface flex items-center justify-center">
					<svg class="w-5 h-5 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
						<path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
					</svg>
				</div>
				<h2 class="text-lg font-display font-semibold">Import from URL</h2>
			</div>
			<form onsubmit={handleImportURL}>
				<input
					type="url"
					bind:value={importURL}
					placeholder="https://example.com/feed.json"
					class="w-full bg-void border border-white/10 rounded-lg px-4 py-3 mb-4 text-text-primary placeholder-text-dim focus:outline-none focus:border-emerald-500/50 transition-colors"
				/>
				<button
					type="submit"
					disabled={importLoading || !importURL.trim()}
					class="btn btn-primary"
				>
					{#if importLoading}
						<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
						</svg>
					{/if}
					Import
				</button>
			</form>
		</section>

		<!-- Import from File -->
		<section class="card p-6">
			<div class="flex items-center gap-3 mb-4">
				<div class="w-10 h-10 rounded-xl bg-surface flex items-center justify-center">
					<svg class="w-5 h-5 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
						<polyline points="14 2 14 8 20 8"/>
						<line x1="12" y1="18" x2="12" y2="12"/>
						<line x1="9" y1="15" x2="15" y2="15"/>
					</svg>
				</div>
				<h2 class="text-lg font-display font-semibold">Import from File</h2>
			</div>
			<p class="text-text-muted text-sm mb-4">
				Upload a NewPipe export (JSON) or Feeds export file.
			</p>
			<form onsubmit={handleImportFile}>
				<input
					type="file"
					accept=".json"
					onchange={handleFileChange}
					class="w-full bg-void border border-white/10 rounded-lg px-4 py-3 mb-4 text-text-primary
						file:mr-4 file:py-1.5 file:px-4 file:rounded-lg file:border-0 file:bg-emerald-500 file:text-void file:font-medium file:cursor-pointer
						file:hover:bg-emerald-400 file:transition-colors focus:outline-none focus:border-emerald-500/50"
				/>
				<button
					type="submit"
					disabled={importLoading || !importFile}
					class="btn btn-primary"
				>
					{#if importLoading}
						<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
						</svg>
					{/if}
					Upload & Import
				</button>
			</form>
		</section>
	</div>

	<!-- Subscription Packs -->
	{#if packs.length > 0}
		<section class="card p-6 animate-fade-up stagger-4" style="opacity: 0;">
			<div class="flex items-center gap-3 mb-4">
				<div class="w-10 h-10 rounded-xl bg-emerald-500/10 flex items-center justify-center">
					<svg class="w-5 h-5 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
						<polyline points="3.27 6.96 12 12.01 20.73 6.96"/>
						<line x1="12" y1="22.08" x2="12" y2="12"/>
					</svg>
				</div>
				<div>
					<h2 class="text-lg font-display font-semibold">Subscription Packs</h2>
					<p class="text-text-muted text-sm">Quick-start with curated channel collections</p>
				</div>
			</div>
			<div class="grid gap-3 sm:grid-cols-2">
				{#each packs as pack}
					<button
						onclick={() => handlePackImport(pack.name)}
						disabled={importLoading}
						class="text-left bg-surface hover:bg-surface-alt disabled:opacity-50 rounded-xl p-4 border border-white/5 hover:border-emerald-500/30 transition-all group"
					>
						<h3 class="font-display font-semibold text-text-primary group-hover:text-emerald-400 transition-colors">{pack.name}</h3>
						<p class="text-sm text-text-muted mt-1">{pack.description}</p>
						{#if pack.tags && pack.tags.length > 0}
							<div class="flex flex-wrap gap-1.5 mt-3">
								{#each pack.tags ?? [] as tag}
									<span class="text-xs bg-void text-text-dim rounded-lg px-2 py-0.5 border border-white/5">{tag}</span>
								{/each}
							</div>
						{/if}
					</button>
				{/each}
			</div>
		</section>
	{/if}
</div>
