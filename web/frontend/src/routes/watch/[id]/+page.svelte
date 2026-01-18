<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { getVideoInfo, updateProgress, getFeeds, addChannel, deleteChannel, getNearbyVideos, getSegments, getQualities, startDownload, subscribeToDownloadProgress } from '$lib/api';
	import type { Feed, Video, WatchProgress, ChannelMembership, SponsorBlockSegment } from '$lib/types';

	let videoId = $derived($page.params.id ?? '');

	let title = $state('');
	let channelName = $state('');
	let channelURL = $state('');
	let viewCount = $state(0);
	let thumbnailURL = $state('');

	let selectedQuality = $state('720');
	let actualHeight = $state(0);
	let actualWidth = $state(0);
	let channelMemberships = $state<ChannelMembership[]>([]);
	let feeds = $state<Feed[]>([]);

	let videoElement = $state<HTMLVideoElement | null>(null);

	// Nearby videos
	let nearbyVideos = $state<Video[]>([]);
	let nearbyProgressMap = $state<Record<string, WatchProgress>>({});
	let nearbyFeedId = $state(0);

	let loading = $state(true);
	let error = $state<string | null>(null);
	let subscribing = $state(false);
	let removingChannelId = $state<number | null>(null);
	let selectedFeedId = $state<string>('');

	// Download state
	let availableQualities = $state<string[]>([]);
	let cachedQualities = $state<string[]>([]);
	let downloadingQuality = $state<string | null>(null);
	let downloadProgress = $state(0);
	let downloadError = $state<string | null>(null);
	let unsubscribeProgress: (() => void) | null = null;

	let lastSavedTime = 0;
	let resumeFrom = $state(0);
	let previousVideoId = '';

	// Playback speed - persisted in localStorage
	const speeds = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2];
	let playbackSpeed = $state(1);

	// SponsorBlock
	let segments = $state<SponsorBlockSegment[]>([]);
	let sponsorBlockEnabled = $state(true);
	let showSkipNotice = $state(false);
	let skipNoticeCategory = $state('');
	let skipNoticeTimeout: ReturnType<typeof setTimeout> | null = null;
	let lastSkippedSegment: string | null = null;

	// Category colors for timeline markers
	const categoryColors: Record<string, string> = {
		sponsor: '#00d400',
		intro: '#00ffff',
		outro: '#0202ed',
		interaction: '#cc00ff',
		selfpromo: '#ffff00',
		music_offtopic: '#ff9900',
		preview: '#008fd6',
		filler: '#7300ff'
	};

	const categoryNames: Record<string, string> = {
		sponsor: 'Sponsor',
		intro: 'Intro',
		outro: 'Outro',
		interaction: 'Interaction Reminder',
		selfpromo: 'Self-Promotion',
		music_offtopic: 'Non-Music',
		preview: 'Preview',
		filler: 'Filler'
	};

	function loadSavedSpeed() {
		if (typeof localStorage !== 'undefined') {
			const saved = localStorage.getItem('playbackSpeed');
			if (saved) {
				const parsed = parseFloat(saved);
				if (speeds.includes(parsed)) {
					playbackSpeed = parsed;
				}
			}
		}
	}

	function loadSponsorBlockSetting() {
		if (typeof localStorage !== 'undefined') {
			const saved = localStorage.getItem('sponsorBlockEnabled');
			if (saved !== null) {
				sponsorBlockEnabled = saved === 'true';
			}
		}
	}

	function setSponsorBlockEnabled(enabled: boolean) {
		sponsorBlockEnabled = enabled;
		if (typeof localStorage !== 'undefined') {
			localStorage.setItem('sponsorBlockEnabled', enabled.toString());
		}
	}

	function checkForSegmentSkip(currentTime: number) {
		if (!sponsorBlockEnabled || !videoElement) return;

		for (const segment of segments) {
			if (lastSkippedSegment === segment.uuid) continue;

			if (currentTime >= segment.startTime && currentTime < segment.endTime - 0.5) {
				videoElement.currentTime = segment.endTime;
				lastSkippedSegment = segment.uuid;

				skipNoticeCategory = categoryNames[segment.category] || segment.category;
				showSkipNotice = true;
				if (skipNoticeTimeout) clearTimeout(skipNoticeTimeout);
				skipNoticeTimeout = setTimeout(() => {
					showSkipNotice = false;
				}, 3000);

				break;
			}
		}

		const maxEnd = Math.max(...segments.map(s => s.endTime), 0);
		if (currentTime > maxEnd + 1) {
			lastSkippedSegment = null;
		}
	}

	function setSpeed(speed: number) {
		playbackSpeed = speed;
		if (videoElement) {
			videoElement.playbackRate = speed;
		}
		if (typeof localStorage !== 'undefined') {
			localStorage.setItem('playbackSpeed', speed.toString());
		}
	}

	function formatDuration(seconds: number): string {
		if (seconds <= 0) return '';
		const h = Math.floor(seconds / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = seconds % 60;
		if (h > 0) {
			return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
		}
		return `${m}:${s.toString().padStart(2, '0')}`;
	}

	function formatViewCount(count: number): string {
		if (count <= 0) return '';
		if (count >= 1_000_000_000) {
			return `${(count / 1_000_000_000).toFixed(1)}B views`;
		}
		if (count >= 1_000_000) {
			return `${(count / 1_000_000).toFixed(1)}M views`;
		}
		if (count >= 1_000) {
			return `${(count / 1_000).toFixed(1)}K views`;
		}
		return `${count} views`;
	}

	function getWatchedPercent(video: Video): number {
		const progress = nearbyProgressMap[video.id];
		if (!progress || progress.duration_seconds === 0) return 0;
		return Math.min(100, (progress.progress_seconds / progress.duration_seconds) * 100);
	}

	async function loadVideo(id: string) {
		if (videoElement && !loading && previousVideoId) {
			const currentTime = Math.floor(videoElement.currentTime);
			const duration = Math.floor(videoElement.duration) || 0;
			if (duration > 0) {
				await updateProgress(previousVideoId, currentTime, duration).catch(() => {});
			}
		}
		previousVideoId = id;

		loading = true;
		error = null;
		title = '';
		channelName = '';
		channelURL = '';
		viewCount = 0;
		thumbnailURL = '';
		actualHeight = 0;
		actualWidth = 0;
		channelMemberships = [];
		resumeFrom = 0;
		lastSavedTime = 0;
		nearbyVideos = [];
		nearbyProgressMap = {};
		nearbyFeedId = 0;
		segments = [];
		lastSkippedSegment = null;
		showSkipNotice = false;
		lastLoadedURL = '';

		try {
			const data = await getVideoInfo(id);
			title = data.title;
			channelName = data.channel;
			channelURL = data.channelURL;
			viewCount = data.viewCount || 0;
			thumbnailURL = data.thumbnail || '';
			channelMemberships = data.channelMemberships || [];
			resumeFrom = data.resumeFrom || 0;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load video';
		} finally {
			loading = false;
		}

		// Load available qualities
		try {
			const qualities = await getQualities(id);
			availableQualities = qualities.available;
			cachedQualities = qualities.cached || [];
			downloadingQuality = qualities.downloading;
		} catch (e) {
			console.warn('Failed to load qualities:', e);
			availableQualities = ['360', '480', '720', '1080'];
		}

		try {
			const nearby = await getNearbyVideos(id, 20);
			nearbyVideos = nearby.videos;
			nearbyProgressMap = nearby.progressMap;
			nearbyFeedId = nearby.feedId;
		} catch (e) {
			console.warn('Failed to load nearby videos:', e);
		}

		try {
			const data = await getSegments(id);
			segments = data.segments || [];
		} catch (e) {
			console.warn('Failed to load SponsorBlock segments:', e);
		}
	}

	let currentLoadingId = '';
	$effect(() => {
		const id = videoId;
		if (id === currentLoadingId) return;
		currentLoadingId = id;
		loadVideo(id);
	});

	let lastLoadedURL = '';
	$effect(() => {
		if (loading || error || !videoElement) return;

		// For non-auto qualities, only load if cached
		const effectiveQuality = selectedQuality === 'auto' || cachedQualities.includes(selectedQuality)
			? selectedQuality
			: 'auto';

		// Build the stream URL
		const newURL = `/api/stream/${videoId}?quality=${effectiveQuality}`;

		// Skip if we already loaded this exact URL
		if (lastLoadedURL === newURL) return;
		lastLoadedURL = newURL;

		const currentTime = videoElement.currentTime;
		const wasPlaying = !videoElement.paused;
		const video = videoElement;

		video.src = newURL;
		video.load();

		// Restore position after load
		video.addEventListener('loadedmetadata', () => {
			if (currentTime > 0) {
				video.currentTime = currentTime;
			}
			if (wasPlaying) {
				video.play().catch(() => {});
			}
		}, { once: true });
	});

	onMount(async () => {
		loadSavedSpeed();
		loadSponsorBlockSetting();

		try {
			feeds = await getFeeds();
			const inbox = feeds.find(f => f.is_system);
			if (inbox) {
				selectedFeedId = inbox.id.toString();
			}
		} catch (e) {
			console.warn('Failed to load feeds:', e);
		}
	});

	onDestroy(() => {
		saveProgress();
		if (videoElement) {
			videoElement.pause();
		}
		if (unsubscribeProgress) {
			unsubscribeProgress();
		}
	});

	function handleVideoLoaded() {
		if (videoElement) {
			videoElement.playbackRate = playbackSpeed;

			if (resumeFrom > 0) {
				videoElement.currentTime = resumeFrom;
				lastSavedTime = resumeFrom;
			}
		}
	}

	function handleLoadedMetadata() {
		if (!videoElement) return;
		actualHeight = videoElement.videoHeight || 0;
		actualWidth = videoElement.videoWidth || 0;
	}

	function saveProgress() {
		if (!videoElement) return;
		const currentTime = Math.floor(videoElement.currentTime);
		const duration = Math.floor(videoElement.duration) || 0;

		if (Math.abs(currentTime - lastSavedTime) >= 5 && duration > 0) {
			lastSavedTime = currentTime;
			updateProgress(videoId, currentTime, duration).catch(() => {});
		}
	}

	function handleTimeUpdate() {
		saveProgress();
		if (videoElement) {
			checkForSegmentSkip(videoElement.currentTime);
		}
	}

	function handlePause() {
		if (!videoElement) return;
		const currentTime = Math.floor(videoElement.currentTime);
		const duration = Math.floor(videoElement.duration) || 0;
		if (duration > 0) {
			updateProgress(videoId, currentTime, duration).catch(() => {});
		}
	}

	async function handleQualitySelect(quality: string) {
		// If this quality is cached, switch to it
		if (cachedQualities.includes(quality)) {
			selectedQuality = quality;
			return;
		}

		// If auto, just switch
		if (quality === 'auto') {
			selectedQuality = 'auto';
			return;
		}

		// Otherwise, start download
		downloadError = null;
		try {
			const result = await startDownload(videoId, quality);
			if (result.status === 'complete') {
				cachedQualities = [...cachedQualities, quality];
				selectedQuality = quality;
			} else {
				downloadingQuality = quality;
				downloadProgress = 0;

				// Subscribe to progress updates
				if (unsubscribeProgress) {
					unsubscribeProgress();
				}
				unsubscribeProgress = subscribeToDownloadProgress(videoId, (data) => {
					console.log('Download progress:', data);
					if (data.status === 'complete') {
						cachedQualities = [...cachedQualities, data.quality];
						downloadingQuality = null;
						downloadProgress = 0;
						// Switch to the downloaded quality
						selectedQuality = data.quality;
						if (unsubscribeProgress) {
							unsubscribeProgress();
							unsubscribeProgress = null;
						}
					} else if (data.status === 'error') {
						downloadError = data.error || 'Download failed';
						downloadingQuality = null;
						downloadProgress = 0;
						if (unsubscribeProgress) {
							unsubscribeProgress();
							unsubscribeProgress = null;
						}
					} else {
						downloadProgress = data.percent || 0;
					}
				});
			}
		} catch (e) {
			downloadError = e instanceof Error ? e.message : 'Failed to start download';
		}
	}

	async function handleSubscribe() {
		if (!selectedFeedId || !channelURL) return;

		subscribing = true;
		try {
			const channel = await addChannel(parseInt(selectedFeedId), channelURL);
			const feed = feeds.find(f => f.id === parseInt(selectedFeedId));
			const feedName = feed?.is_system ? 'Inbox' : (feed?.name || 'Feed');
			channelMemberships = [...channelMemberships, {
				channelId: channel.id,
				feedId: parseInt(selectedFeedId),
				feedName
			}];
		} catch (e) {
			console.error('Failed to subscribe:', e);
			alert(e instanceof Error ? e.message : 'Failed to subscribe');
		} finally {
			subscribing = false;
		}
	}

	async function handleRemove(membership: ChannelMembership) {
		removingChannelId = membership.channelId;
		try {
			await deleteChannel(membership.channelId);
			channelMemberships = channelMemberships.filter(m => m.channelId !== membership.channelId);
		} catch (e) {
			console.error('Failed to remove channel:', e);
			alert(e instanceof Error ? e.message : 'Failed to remove channel');
		} finally {
			removingChannelId = null;
		}
	}
</script>

<svelte:head>
	<title>{title || 'Watch'} - Feeds</title>
	{#if thumbnailURL}
		<link rel="preload" as="image" href={thumbnailURL} />
	{/if}
</svelte:head>

<div class="max-w-7xl mx-auto">
	<div class="grid lg:grid-cols-[minmax(0,1fr)_380px] gap-8">
		<!-- Main Content -->
		<div class="min-w-0 animate-fade-up" style="opacity: 0;">
			<!-- Video Player -->
			<div class="player-container mb-4">
				{#if loading}
					<div class="absolute inset-0 flex items-center justify-center">
						<div class="text-center">
							<div class="w-12 h-12 rounded-full border-2 border-emerald-500/20 border-t-emerald-500 animate-spin mx-auto mb-3"></div>
							<p class="text-text-muted font-display text-sm">Loading video...</p>
						</div>
					</div>
				{:else if error}
					<div class="absolute inset-0 flex items-center justify-center">
						<div class="text-center px-6">
							<div class="w-16 h-16 rounded-full bg-crimson-500/10 flex items-center justify-center mx-auto mb-4">
								<svg class="w-8 h-8 text-crimson-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
									<circle cx="12" cy="12" r="10"/>
									<line x1="12" y1="8" x2="12" y2="12"/>
									<line x1="12" y1="16" x2="12.01" y2="16"/>
								</svg>
							</div>
							<p class="text-crimson-400 mb-4">{error}</p>
							<a
								href="https://www.youtube.com/watch?v={videoId}"
								target="_blank"
								rel="noopener"
								class="btn btn-secondary btn-sm"
							>
								Watch on YouTube
							</a>
						</div>
					</div>
				{:else}
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						bind:this={videoElement}
						class="w-full h-full"
						controls
						preload="auto"
						playsinline
						poster={thumbnailURL || undefined}
						onloadedmetadata={handleLoadedMetadata}
						onloadeddata={handleVideoLoaded}
						ontimeupdate={handleTimeUpdate}
						onpause={handlePause}
					>
						Your browser does not support the video tag.
					</video>

					<!-- Download Progress Overlay -->
					{#if downloadingQuality}
						<div class="download-overlay">
							<div class="download-overlay-content">
								<div class="download-spinner"></div>
								<span class="download-text">Downloading {downloadingQuality}p...</span>
								<div class="download-progress-bar">
									<div class="download-progress-fill" style="width: {downloadProgress}%"></div>
								</div>
								<span class="download-percent">{Math.round(downloadProgress)}%</span>
							</div>
						</div>
					{/if}

					<!-- Skip Notice -->
					{#if showSkipNotice}
						<div class="skip-notice">
							<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
								<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
							</svg>
							<span>Skipped {skipNoticeCategory}</span>
						</div>
					{/if}
				{/if}
			</div>

			<!-- SponsorBlock Timeline -->
			{#if segments.length > 0 && videoElement}
				<div class="sponsor-timeline mb-4" title="SponsorBlock segments">
					{#each segments as segment}
						{@const duration = videoElement.duration || 1}
						{@const left = (segment.startTime / duration) * 100}
						{@const width = ((segment.endTime - segment.startTime) / duration) * 100}
						<div
							class="sponsor-segment"
							style="left: {left}%; width: {Math.max(width, 0.5)}%; background-color: {categoryColors[segment.category] || '#888'}"
							title="{categoryNames[segment.category] || segment.category}: {Math.floor(segment.startTime)}s - {Math.floor(segment.endTime)}s"
						></div>
					{/each}
				</div>
			{/if}

			<!-- Controls Row -->
			{#if !loading && !error}
				<!-- Video Controls - Mobile Optimized -->
				<div class="video-controls mb-6">
					<!-- Primary Controls Row -->
					<div class="controls-row">
						<!-- Speed Selector -->
						<div class="control-group">
							<label class="control-label">Speed</label>
							<select
								value={playbackSpeed.toString()}
								onchange={(e) => setSpeed(parseFloat(e.currentTarget.value))}
								class="select"
							>
								{#each speeds as speed}
									<option value={speed.toString()}>{speed}x</option>
								{/each}
							</select>
						</div>

						<!-- Now Playing Badge -->
						{#if actualHeight > 0}
							<div class="now-playing-badge">
								<svg class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
									<path d="M8 5v14l11-7z"/>
								</svg>
								<span>{actualHeight}p</span>
							</div>
						{:else if !loading}
							<div class="now-playing-badge">
								<svg class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
									<path d="M8 5v14l11-7z"/>
								</svg>
								<span>Auto</span>
							</div>
						{/if}

						{#if segments.length > 0}
							<span class="badge badge-success">
								<svg class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
									<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
								</svg>
								{segments.length} skip{segments.length !== 1 ? 's' : ''}
							</span>
						{/if}
					</div>

					<!-- Secondary Controls (Settings) -->
					<details class="settings-dropdown">
						<summary class="settings-toggle">
							<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<circle cx="12" cy="12" r="3"/>
								<path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/>
							</svg>
							<span>More options</span>
							<svg class="chevron w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M6 9l6 6 6-6"/>
							</svg>
						</summary>
						<div class="settings-content">
							<label class="settings-option">
								<input
									type="checkbox"
									checked={sponsorBlockEnabled}
									onchange={(e) => setSponsorBlockEnabled(e.currentTarget.checked)}
									class="checkbox"
								/>
								<div class="settings-option-text">
									<span class="settings-option-label">SponsorBlock</span>
									<span class="settings-option-desc">Auto-skip sponsors & intros</span>
								</div>
							</label>
						</div>
					</details>

					<!-- Download HD Row -->
					{#if availableQualities.length > 0}
						<div class="download-hd-row">
							<span class="download-hd-label">Download HD</span>
							<div class="download-hd-buttons">
								{#each availableQualities as q}
									{@const isCached = cachedQualities.includes(q)}
									{@const isDownloading = downloadingQuality === q}
									{@const isActive = selectedQuality === q && isCached}
									{@const qNum = parseInt(q)}
									{#if isCached || qNum > actualHeight || actualHeight === 0}
										<button
											class="download-hd-btn"
											class:cached={isCached}
											class:active={isActive}
											class:downloading={isDownloading}
											onclick={() => handleQualitySelect(q)}
											disabled={downloadingQuality !== null && !isDownloading && !isCached}
										>
											{#if isDownloading}
												<span class="download-hd-progress">{Math.round(downloadProgress)}%</span>
											{:else if isCached}
												<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
													<path d="M20 6L9 17l-5-5"/>
												</svg>
											{:else}
												<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
													<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
													<polyline points="7,10 12,15 17,10"/>
													<line x1="12" y1="15" x2="12" y2="3"/>
												</svg>
											{/if}
											<span>{q}p</span>
										</button>
									{/if}
								{/each}
							</div>
							{#if downloadError}
								<span class="download-hd-error">{downloadError}</span>
							{/if}
						</div>
					{/if}
				</div>
			{/if}

			<!-- Title & Channel -->
			<div class="mb-6">
				<h1 class="text-xl font-display font-semibold mb-3">
					{#if loading}
						<span class="skeleton inline-block h-7 w-96"></span>
					{:else}
						{title}
					{/if}
				</h1>

				<div class="flex items-center justify-between flex-wrap gap-4">
					<div class="flex items-center gap-3 text-text-secondary">
						{#if loading}
							<span class="skeleton inline-block h-5 w-32"></span>
						{:else}
							<span class="font-medium">{channelName}</span>
							{#if viewCount > 0}
								<span class="text-text-muted">·</span>
								<span class="text-text-muted">{formatViewCount(viewCount)}</span>
							{/if}
						{/if}
					</div>

					<!-- Subscribe Section -->
					<div class="flex items-center gap-2 flex-wrap">
						{#each channelMemberships as membership}
							<span class="badge">
								<a href="/feeds/{membership.feedId}" class="hover:text-emerald-400 transition-colors">
									{membership.feedName}
								</a>
								<button
									onclick={() => handleRemove(membership)}
									disabled={removingChannelId === membership.channelId}
									class="ml-1 text-text-muted hover:text-crimson-400 transition-colors disabled:opacity-50"
								>
									{#if removingChannelId === membership.channelId}
										<svg class="animate-spin h-3 w-3" viewBox="0 0 24 24" fill="none">
											<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
											<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
										</svg>
									{:else}
										<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
											<path d="M18 6L6 18M6 6l12 12"/>
										</svg>
									{/if}
								</button>
							</span>
						{/each}

						{#if channelURL && feeds.length > 0}
							<select bind:value={selectedFeedId} class="select">
								<option value="" disabled>Add to...</option>
								{#each feeds as feed}
									<option value={feed.id.toString()}>
										{feed.is_system ? 'Inbox' : feed.name}
									</option>
								{/each}
							</select>
							<button
								onclick={handleSubscribe}
								disabled={subscribing || !selectedFeedId}
								class="btn btn-primary btn-sm"
							>
								{#if subscribing}
									<svg class="animate-spin h-3 w-3" viewBox="0 0 24 24" fill="none">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
									</svg>
								{/if}
								Add
							</button>
						{:else if feeds.length === 0 && !loading}
							<a href="/import" class="text-sm text-emerald-400 hover:text-emerald-300 transition-colors">
								Create a feed first
							</a>
						{/if}
					</div>
				</div>
			</div>

			<a
				href="https://www.youtube.com/watch?v={videoId}"
				target="_blank"
				rel="noopener"
				class="inline-flex items-center gap-2 text-sm text-text-muted hover:text-emerald-400 transition-colors"
			>
				<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
					<polyline points="15,3 21,3 21,9"/>
					<line x1="10" y1="14" x2="21" y2="3"/>
				</svg>
				Watch on YouTube
			</a>

			<!-- Mobile Up Next -->
			{#if nearbyVideos.length > 0}
				<div class="mt-8 lg:hidden">
					<div class="flex items-center justify-between mb-4">
						<h2 class="font-display font-semibold">Up Next</h2>
						{#if nearbyFeedId > 0}
							<a href="/feeds/{nearbyFeedId}" class="text-sm text-emerald-400 hover:text-emerald-300 transition-colors">
								View Feed
							</a>
						{/if}
					</div>
					<div class="space-y-3">
						{#each nearbyVideos.slice(0, 6) as video}
							<a href="/watch/{video.id}" class="up-next-item group">
								<div class="video-thumbnail w-36 flex-shrink-0">
									{#if video.thumbnail}
										<img src={video.thumbnail} alt="" />
									{/if}
									{#if video.duration > 0}
										<span class="duration-badge">{formatDuration(video.duration)}</span>
									{/if}
									{#if getWatchedPercent(video) > 0}
										<div class="watch-progress">
											<div class="watch-progress-fill" style="width: {getWatchedPercent(video)}%"></div>
										</div>
									{/if}
								</div>
								<div class="flex-1 min-w-0">
									<h3 class="text-sm font-medium line-clamp-2 group-hover:text-emerald-400 transition-colors">
										{video.title}
									</h3>
									<p class="text-xs text-text-muted mt-1">{video.channel_name}</p>
								</div>
							</a>
						{/each}
					</div>
				</div>
			{/if}
		</div>

		<!-- Desktop Sidebar - Up Next -->
		{#if nearbyVideos.length > 0}
			<aside class="hidden lg:block animate-fade-up stagger-2" style="opacity: 0;">
				<div class="sticky top-20">
					<div class="flex items-center justify-between mb-4">
						<h2 class="font-display font-semibold">Up Next</h2>
						{#if nearbyFeedId > 0}
							<a href="/feeds/{nearbyFeedId}" class="text-sm text-emerald-400 hover:text-emerald-300 transition-colors">
								View Feed
							</a>
						{/if}
					</div>
					<div class="up-next-sidebar space-y-2 pr-2">
						{#each nearbyVideos as video}
							<a href="/watch/{video.id}" class="up-next-item group">
								<div class="video-thumbnail w-36 flex-shrink-0">
									{#if video.thumbnail}
										<img src={video.thumbnail} alt="" />
									{/if}
									{#if video.duration > 0}
										<span class="duration-badge">{formatDuration(video.duration)}</span>
									{/if}
									{#if getWatchedPercent(video) > 0}
										<div class="watch-progress">
											<div class="watch-progress-fill" style="width: {getWatchedPercent(video)}%"></div>
										</div>
									{/if}
								</div>
								<div class="flex-1 min-w-0">
									<h3 class="text-sm font-medium line-clamp-2 group-hover:text-emerald-400 transition-colors">
										{video.title}
									</h3>
									<p class="text-xs text-text-muted mt-1">{video.channel_name}</p>
								</div>
							</a>
						{/each}
					</div>
				</div>
			</aside>
		{/if}
	</div>
</div>
