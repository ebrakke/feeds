<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { getVideoInfo, updateProgress, getFeeds, addChannel, deleteChannel, getNearbyVideos, getStreamURLs } from '$lib/api';
	import type { Feed, Video, WatchProgress, ChannelMembership } from '$lib/types';

	let videoId = $derived($page.params.id ?? '');

	let title = $state('');
	let channelName = $state('');
	let channelURL = $state('');
	let viewCount = $state(0);
	let thumbnailURL = $state('');

	let selectedQuality = $state('720');
	let streamLoading = $state(false);
	let streamError = $state<string | null>(null);
	let actualHeight = $state(0);
	let actualWidth = $state(0);
	let useAdaptiveStreaming = $state(false);
	let channelMemberships = $state<ChannelMembership[]>([]);
	let feeds = $state<Feed[]>([]);

	let videoElement = $state<HTMLVideoElement | null>(null);
	let hasLoadedStream = $state(false);
	let currentVideoURL = $state('');
	let currentAudioURL = $state('');
	let usingMSE = $state(false);
	let progressiveURL = $state('');
	let pendingSeekTime = $state<number | null>(null);
	let adaptiveLoadKey = '';

	let mediaSource: MediaSource | null = null;
	let mediaObjectURL = '';
	let videoAbort: AbortController | null = null;
	let audioAbort: AbortController | null = null;

	// Nearby videos
	let nearbyVideos = $state<Video[]>([]);
	let nearbyProgressMap = $state<Record<string, WatchProgress>>({});
	let nearbyFeedId = $state(0);

	let loading = $state(true);
	let error = $state<string | null>(null);
	let subscribing = $state(false);
	let removingChannelId = $state<number | null>(null);
	let selectedFeedId = $state<string>('');

	let lastSavedTime = 0;
	let resumeFrom = $state(0);
	let previousVideoId = '';

	// Playback speed - persisted in localStorage
	const speeds = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2];
	let playbackSpeed = $state(1);

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
		// Save progress from previous video before switching
		if (videoElement && !loading && previousVideoId) {
			const currentTime = Math.floor(videoElement.currentTime);
			const duration = Math.floor(videoElement.duration) || 0;
			if (duration > 0) {
				await updateProgress(previousVideoId, currentTime, duration).catch(() => {});
			}
		}
		previousVideoId = id;

		// Clean up previous video
		cleanupMediaSource();

		// Reset state for new video
		loading = true;
		error = null;
		title = '';
		channelName = '';
		channelURL = '';
		viewCount = 0;
		thumbnailURL = '';
		streamLoading = false;
		streamError = null;
		hasLoadedStream = false;
		currentVideoURL = '';
		currentAudioURL = '';
		progressiveURL = '';
		pendingSeekTime = null;
		adaptiveLoadKey = '';
		usingMSE = false;
		actualHeight = 0;
		actualWidth = 0;
		channelMemberships = [];
		resumeFrom = 0;
		lastSavedTime = 0;
		nearbyVideos = [];
		nearbyProgressMap = {};
		nearbyFeedId = 0;

		// Load video info
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

		// Load nearby videos (don't block on this)
		try {
			const nearby = await getNearbyVideos(id, 20);
			nearbyVideos = nearby.videos;
			nearbyProgressMap = nearby.progressMap;
			nearbyFeedId = nearby.feedId;
		} catch (e) {
			console.warn('Failed to load nearby videos:', e);
		}
	}

	function cleanupMediaSource() {
		if (videoAbort) {
			videoAbort.abort();
			videoAbort = null;
		}
		if (audioAbort) {
			audioAbort.abort();
			audioAbort = null;
		}
		if (mediaSource && mediaSource.readyState === 'open') {
			try {
				mediaSource.endOfStream();
			} catch {
				// ignore teardown errors
			}
		}
		mediaSource = null;
		usingMSE = false;
		if (mediaObjectURL) {
			URL.revokeObjectURL(mediaObjectURL);
			mediaObjectURL = '';
		}
		if (videoElement) {
			videoElement.removeAttribute('src');
			videoElement.load();
		}
	}

	function buildMimeType(contentType: string | null, fallback: string) {
		if (!contentType) {
			return fallback;
		}
		if (contentType.includes('codecs=')) {
			return contentType;
		}
		const base = contentType.split(';')[0].trim();
		const defaults: Record<string, string> = {
			'video/mp4': 'avc1.640028',
			'audio/mp4': 'mp4a.40.2',
			'video/webm': 'vp9',
			'audio/webm': 'opus'
		};
		const match = fallback.match(/codecs="([^"]+)"/);
		const codec = match ? match[1] : (defaults[base] || '');
		return codec ? `${base}; codecs="${codec}"` : base;
	}

	async function getContentType(url: string) {
		const res = await fetch(url, { headers: { Range: 'bytes=0-0' } });
		if (!res.ok) {
			return '';
		}
		return res.headers.get('content-type') || '';
	}

	function waitForUpdateEnd(sb: SourceBuffer) {
		return new Promise<void>((resolve, reject) => {
			const onEnd = () => {
				sb.removeEventListener('updateend', onEnd);
				resolve();
			};
			const onError = () => {
				sb.removeEventListener('error', onError);
				reject(new Error('SourceBuffer error'));
			};
			sb.addEventListener('updateend', onEnd, { once: true });
			sb.addEventListener('error', onError, { once: true });
		});
	}

	async function appendChunk(sb: SourceBuffer, chunk: ArrayBuffer) {
		if (sb.updating) {
			await waitForUpdateEnd(sb);
		}
		sb.appendBuffer(chunk);
		await waitForUpdateEnd(sb);
	}

	async function streamToBuffer(url: string, sb: SourceBuffer, signal: AbortSignal) {
		const chunkSize = 2 * 1024 * 1024;
		let position = 0;
		let total = -1;

		while (total < 0 || position < total) {
			const end = total > 0 ? Math.min(total - 1, position + chunkSize - 1) : position + chunkSize - 1;
			const res = await fetch(url, {
				headers: { Range: `bytes=${position}-${end}` },
				signal
			});

			if (res.status === 416) {
				break;
			}
			if (!res.ok) {
				throw new Error(`Stream fetch failed: ${res.status}`);
			}

			const contentRange = res.headers.get('content-range');
			if (contentRange) {
				const match = contentRange.match(/\/(\d+)$/);
				if (match) {
					total = parseInt(match[1], 10);
				}
			}

			const buf = await res.arrayBuffer();
			if (buf.byteLength === 0) {
				break;
			}

			await appendChunk(sb, buf);
			position += buf.byteLength;

			if (!contentRange && buf.byteLength < chunkSize) {
				break;
			}
		}
	}

	async function handleLoadStream() {
		if (!useAdaptiveStreaming) {
			return;
		}
		streamLoading = true;
		streamError = null;

		cleanupMediaSource();

		try {
			progressiveURL = `/api/stream/${videoId}?quality=${encodeURIComponent(selectedQuality)}&adaptive=0`;
			const urls = await getStreamURLs(videoId, selectedQuality);
			currentVideoURL = urls.videoURL;
			currentAudioURL = urls.audioURL;
			hasLoadedStream = true;

			if (!videoElement) {
				throw new Error('Video element not ready');
			}

			// If no separate audio stream, use direct progressive video URL
			if (!currentAudioURL) {
				videoElement.src = currentVideoURL;
				videoElement.load();
				usingMSE = false;
			} else {
				if (!('MediaSource' in window)) {
					videoElement.src = progressiveURL;
					videoElement.load();
					usingMSE = false;
					return;
				}

				const [videoType, audioType] = await Promise.all([
					getContentType(currentVideoURL),
					getContentType(currentAudioURL)
				]);

				const videoMime = buildMimeType(videoType, 'video/mp4; codecs="avc1.640028"');
				const audioMime = buildMimeType(audioType, 'audio/mp4; codecs="mp4a.40.2"');

				if (!MediaSource.isTypeSupported(videoMime) || !MediaSource.isTypeSupported(audioMime)) {
					videoElement.src = progressiveURL;
					videoElement.load();
					usingMSE = false;
					return;
				}

				mediaSource = new MediaSource();
				mediaObjectURL = URL.createObjectURL(mediaSource);
				videoElement.src = mediaObjectURL;
				videoElement.load();

				await new Promise<void>((resolve) => {
					mediaSource?.addEventListener('sourceopen', () => resolve(), { once: true });
				});

				if (!mediaSource) {
					throw new Error('MediaSource not available');
				}

				const videoBuffer = mediaSource.addSourceBuffer(videoMime);
				const audioBuffer = mediaSource.addSourceBuffer(audioMime);
				videoAbort = new AbortController();
				audioAbort = new AbortController();

				usingMSE = true;
				void Promise.all([
					streamToBuffer(currentVideoURL, videoBuffer, videoAbort.signal),
					streamToBuffer(currentAudioURL, audioBuffer, audioAbort.signal)
				]).then(() => {
					if (mediaSource?.readyState === 'open') {
						mediaSource.endOfStream();
					}
				}).catch((err) => {
					if (err instanceof DOMException && err.name === 'AbortError') {
						return;
					}
					streamError = err instanceof Error ? err.message : 'Failed to load stream';
				});
			}
		} catch (e) {
			if (e instanceof DOMException && e.name === 'AbortError') {
				return;
			}
			streamError = e instanceof Error ? e.message : 'Failed to load stream';
			hasLoadedStream = false;
		} finally {
			streamLoading = false;
		}
	}

	// React to videoId changes
	let currentLoadingId = '';
	$effect(() => {
		const id = videoId;
		// Prevent re-running if we're already loading this video
		if (id === currentLoadingId) return;
		currentLoadingId = id;
		loadVideo(id);
	});

	$effect(() => {
		if (loading || error) return;
		if (!videoElement) return;

		progressiveURL = `/api/stream/${videoId}?quality=${encodeURIComponent(selectedQuality)}&adaptive=0`;

		if (useAdaptiveStreaming) {
			const key = `${videoId}:${selectedQuality}`;
			if (adaptiveLoadKey === key) return;
			adaptiveLoadKey = key;
			handleLoadStream();
			return;
		}

		if (usingMSE) {
			cleanupMediaSource();
		}
		videoElement.src = progressiveURL;
		videoElement.load();
	});

	onMount(async () => {
		// Load saved playback speed
		loadSavedSpeed();

		// Load feeds (only once)
		try {
			feeds = await getFeeds();
			// Pre-select Inbox if it exists
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
		cleanupMediaSource();
		if (videoElement) {
			videoElement.pause();
		}
	});

	function handleVideoLoaded() {
		if (videoElement) {
			streamLoading = false;
			// Apply saved playback speed
			videoElement.playbackRate = playbackSpeed;

			// Resume from saved position if available
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
		if (pendingSeekTime !== null) {
			videoElement.currentTime = pendingSeekTime;
			pendingSeekTime = null;
		}
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
	}

	function handlePause() {
		if (!videoElement) return;
		const currentTime = Math.floor(videoElement.currentTime);
		const duration = Math.floor(videoElement.duration) || 0;
		if (duration > 0) {
			updateProgress(videoId, currentTime, duration).catch(() => {});
		}
	}

	function handleSeeking() {
		if (!videoElement) return;
		if (usingMSE && progressiveURL) {
			const seekTime = videoElement.currentTime;
			cleanupMediaSource();
			usingMSE = false;
			pendingSeekTime = seekTime;
			videoElement.src = progressiveURL;
			videoElement.load();
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

<div class="max-w-6xl mx-auto">
	<div class="grid lg:grid-cols-[minmax(0,1fr)_360px] gap-6">
		<div class="min-w-0">
	<!-- Video container -->
	<div class="aspect-video bg-black rounded-lg overflow-hidden mb-4 relative">
		{#if loading}
			<div class="absolute inset-0 flex items-center justify-center bg-gray-900">
				<div class="text-center">
					<svg class="animate-spin h-10 w-10 text-blue-500 mx-auto mb-3" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
					<p class="text-gray-400 text-sm">Loading video...</p>
				</div>
			</div>
		{:else if error}
			<div class="absolute inset-0 flex items-center justify-center bg-gray-900">
				<div class="text-center px-4">
					<svg class="h-10 w-10 text-red-500 mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
					</svg>
					<p class="text-red-400 mb-2">{error}</p>
					<a
						href="https://www.youtube.com/watch?v={videoId}"
						target="_blank"
						rel="noopener"
						class="text-blue-400 hover:text-blue-300 text-sm"
					>
						Watch on YouTube instead
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
				src={useAdaptiveStreaming ? undefined : progressiveURL}
				onloadedmetadata={handleLoadedMetadata}
				onloadeddata={handleVideoLoaded}
				ontimeupdate={handleTimeUpdate}
				onpause={handlePause}
				onseeking={handleSeeking}
			>
				Your browser does not support the video tag.
			</video>
			{#if streamError}
				<div class="absolute inset-0 flex items-center justify-center bg-gray-900/90">
					<div class="text-center px-4">
						<svg class="h-10 w-10 text-red-500 mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
						</svg>
						<p class="text-red-400 mb-2">{streamError}</p>
						<a
							href="https://www.youtube.com/watch?v={videoId}"
							target="_blank"
							rel="noopener"
							class="text-blue-400 hover:text-blue-300 text-sm"
						>
							Watch on YouTube instead
						</a>
					</div>
				</div>
			{/if}
		{/if}
	</div>

	{#if !loading && !error}
		<div class="flex flex-wrap items-center gap-3 mb-4">
			<details class="bg-gray-800/60 border border-gray-700 rounded px-3 py-2 text-sm">
				<summary class="cursor-pointer text-gray-300 select-none">Settings</summary>
				<div class="mt-2 flex items-center gap-2 text-gray-400">
					<label class="flex items-center gap-2">
						<input
							type="checkbox"
							bind:checked={useAdaptiveStreaming}
							class="w-4 h-4 rounded border-gray-500 bg-gray-600 text-blue-500 focus:ring-blue-500 focus:ring-offset-0"
						/>
						Adaptive streaming (experimental)
					</label>
				</div>
			</details>
			<label class="text-sm text-gray-400">Quality</label>
			<select
				bind:value={selectedQuality}
				class="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm"
				disabled={streamLoading}
			>
				<option value="best">Best available</option>
				{#if useAdaptiveStreaming}
					<option value="4320">8K (4320p)</option>
					<option value="2160">4K (2160p)</option>
					<option value="1440">1440p</option>
				{/if}
				<option value="1080">1080p</option>
				<option value="720">720p</option>
				<option value="480">480p</option>
				<option value="360">360p</option>
			</select>
			<button
				onclick={handleLoadStream}
				disabled={streamLoading || !useAdaptiveStreaming}
				class="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white px-3 py-1 rounded text-sm inline-flex items-center gap-2"
			>
				{#if streamLoading}
					<svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				{/if}
				{hasLoadedStream ? 'Reload adaptive' : 'Load adaptive'}
			</button>
			{#if actualHeight > 0}
				<span class="text-xs text-gray-500">
					Actual: {actualHeight}p
				</span>
			{/if}
		</div>
	{/if}

	<!-- Speed controls -->
	{#if !loading && !error}
		<div class="flex items-center gap-1 mb-4">
			<span class="text-gray-400 text-sm mr-2">Speed:</span>
			{#each speeds as speed}
				<button
					onclick={() => setSpeed(speed)}
					class="px-2 py-1 text-sm rounded {playbackSpeed === speed ? 'bg-blue-600 text-white' : 'bg-gray-700 text-gray-300 hover:bg-gray-600'}"
				>
					{speed}x
				</button>
			{/each}
		</div>
	{/if}

	<!-- Title -->
	<h1 class="text-lg font-bold mb-2">
		{#if loading}
			<span class="inline-block bg-gray-700 rounded h-6 w-64 animate-pulse"></span>
		{:else}
			{title}
		{/if}
	</h1>

	<div class="flex items-center justify-between mb-4">
		<!-- Channel name and view count -->
		<div class="text-gray-400">
			{#if loading}
				<span class="inline-block bg-gray-700 rounded h-4 w-32 animate-pulse"></span>
			{:else}
				<span>{channelName}</span>
				{#if viewCount > 0}
					<span class="text-gray-500 ml-2">{formatViewCount(viewCount)}</span>
				{/if}
			{/if}
		</div>

		<!-- Subscribe section -->
		<div class="flex flex-col items-end gap-2">
			{#if channelMemberships.length > 0}
				<div class="flex items-center gap-2 flex-wrap justify-end">
					<span class="text-gray-400 text-sm">In:</span>
					{#each channelMemberships as membership}
						<span class="inline-flex items-center gap-1 bg-gray-800 text-sm px-2 py-1 rounded">
							<a href="/feeds/{membership.feedId}" class="hover:text-blue-400">
								{membership.feedName}
							</a>
							<button
								onclick={() => handleRemove(membership)}
								disabled={removingChannelId === membership.channelId}
								class="text-gray-500 hover:text-red-400 disabled:opacity-50 ml-1"
								title="Remove from {membership.feedName}"
							>
								{#if removingChannelId === membership.channelId}
									<svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
									</svg>
								{:else}
									<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
									</svg>
								{/if}
							</button>
						</span>
					{/each}
				</div>
			{/if}
			{#if channelURL && feeds.length > 0}
				<div class="flex items-center gap-2">
					<select
						bind:value={selectedFeedId}
						class="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm"
					>
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
						class="bg-red-600 hover:bg-red-700 disabled:opacity-50 text-white px-3 py-1 rounded text-sm inline-flex items-center gap-1"
					>
						{#if subscribing}
							<svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
						{/if}
						Add
					</button>
				</div>
			{:else if feeds.length === 0 && !loading}
				<a href="/import" class="text-sm text-blue-400 hover:text-blue-300">
					Create a feed to subscribe
				</a>
			{/if}
		</div>
	</div>

	<a
		href="https://www.youtube.com/watch?v={videoId}"
		target="_blank"
		rel="noopener"
		class="text-sm text-gray-500 hover:text-blue-400"
	>
		Watch on YouTube
	</a>

		</div>

	<!-- Up Next / Nearby Videos -->
	{#if nearbyVideos.length > 0}
		<div class="mt-8 lg:hidden">
			<div class="flex items-center justify-between mb-3">
				<h2 class="text-lg font-semibold">Up Next</h2>
				{#if nearbyFeedId > 0}
					<a href="/feeds/{nearbyFeedId}" class="text-sm text-blue-400 hover:text-blue-300">
						View Feed
					</a>
				{/if}
			</div>
			<div class="space-y-3">
				{#each nearbyVideos as video}
					<a
						href="/watch/{video.id}"
						class="flex gap-3 group"
					>
						<div class="relative flex-shrink-0 w-40 aspect-video bg-gray-800 rounded-lg overflow-hidden">
							{#if video.thumbnail}
								<img
									src={video.thumbnail}
									alt=""
									class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
								/>
							{:else}
								<div class="w-full h-full flex items-center justify-center text-gray-600">
									<svg class="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/>
									</svg>
								</div>
							{/if}
							<!-- Duration badge -->
							{#if video.duration > 0}
								<span class="absolute bottom-1 right-1 bg-black/80 text-white text-xs px-1 rounded">
									{formatDuration(video.duration)}
								</span>
							{/if}
							<!-- Watch progress bar -->
							{#if getWatchedPercent(video) > 0}
								<div class="absolute bottom-0 left-0 right-0 h-1 bg-gray-900/50">
									<div
										class="h-full bg-red-600"
										style="width: {getWatchedPercent(video)}%"
									></div>
								</div>
							{/if}
						</div>
						<div class="flex-1 min-w-0">
							<h3 class="text-sm font-medium line-clamp-2 group-hover:text-blue-400 transition-colors">
								{video.title}
							</h3>
							<p class="text-xs text-gray-500 mt-1">{video.channel_name}</p>
						</div>
					</a>
				{/each}
			</div>
		</div>

		<aside class="hidden lg:block lg:sticky lg:top-24 lg:h-[calc(100vh-7rem)]">
			<div class="flex items-center justify-between mb-3">
				<h2 class="text-lg font-semibold">Up Next</h2>
				{#if nearbyFeedId > 0}
					<a href="/feeds/{nearbyFeedId}" class="text-sm text-blue-400 hover:text-blue-300">
						View Feed
					</a>
				{/if}
			</div>
			<div class="h-full overflow-y-auto space-y-3 pr-2 scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent">
				{#each nearbyVideos as video}
					<a
						href="/watch/{video.id}"
						class="flex gap-3 group"
					>
						<div class="relative flex-shrink-0 w-40 aspect-video bg-gray-800 rounded-lg overflow-hidden">
							{#if video.thumbnail}
								<img
									src={video.thumbnail}
									alt=""
									class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
								/>
							{:else}
								<div class="w-full h-full flex items-center justify-center text-gray-600">
									<svg class="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/>
									</svg>
								</div>
							{/if}
							<!-- Duration badge -->
							{#if video.duration > 0}
								<span class="absolute bottom-1 right-1 bg-black/80 text-white text-xs px-1 rounded">
									{formatDuration(video.duration)}
								</span>
							{/if}
							<!-- Watch progress bar -->
							{#if getWatchedPercent(video) > 0}
								<div class="absolute bottom-0 left-0 right-0 h-1 bg-gray-900/50">
									<div
										class="h-full bg-red-600"
										style="width: {getWatchedPercent(video)}%"
									></div>
								</div>
							{/if}
						</div>
						<div class="flex-1 min-w-0">
							<h3 class="text-sm font-medium line-clamp-2 group-hover:text-blue-400 transition-colors">
								{video.title}
							</h3>
							<p class="text-xs text-gray-500 mt-1">{video.channel_name}</p>
						</div>
					</a>
				{/each}
			</div>
		</aside>
	{/if}
	</div>
</div>
