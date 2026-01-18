import type { Feed, Channel, Video, WatchProgress, Config, WatchHistoryChannel, GroupSuggestion, ChannelMembership, SponsorBlockSegment } from './types';

const API_BASE = '/api';

async function fetchJSON<T>(url: string, options?: RequestInit): Promise<T> {
	const res = await fetch(API_BASE + url, {
		headers: { 'Content-Type': 'application/json' },
		...options
	});
	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || res.statusText);
	}
	if (res.status === 204) return undefined as T;
	return res.json();
}

// Config
export async function getConfig(): Promise<Config> {
	return fetchJSON('/config');
}

export async function setYtdlpCookies(cookies: string): Promise<void> {
	return fetchJSON('/config/ytdlp-cookies', {
		method: 'POST',
		body: JSON.stringify({ cookies })
	});
}

export async function clearYtdlpCookies(): Promise<void> {
	return fetchJSON('/config/ytdlp-cookies', {
		method: 'POST',
		body: JSON.stringify({ clear: true })
	});
}

// Feeds
export async function getFeeds(): Promise<Feed[]> {
	return fetchJSON('/feeds');
}

export async function getFeed(id: number, limit = 100, offset = 0): Promise<{
	feed: Feed;
	channels: Channel[];
	videos: Video[];
	progressMap: Record<string, WatchProgress>;
	allFeeds: Feed[];
	total: number;
	offset: number;
	limit: number;
}> {
	return fetchJSON(`/feeds/${id}?limit=${limit}&offset=${offset}`);
}

export async function createFeed(name: string): Promise<Feed> {
	return fetchJSON('/feeds', {
		method: 'POST',
		body: JSON.stringify({ name })
	});
}

export async function deleteFeed(id: number): Promise<void> {
	return fetchJSON(`/feeds/${id}`, { method: 'DELETE' });
}

export async function refreshFeed(id: number): Promise<{
	videosFound: number;
	channels: number;
	errors: string[];
}> {
	return fetchJSON(`/feeds/${id}/refresh`, { method: 'POST' });
}

// Channels
export async function getChannel(id: number): Promise<{
	channel: Channel;
	videos: Video[];
	progressMap: Record<string, WatchProgress>;
}> {
	return fetchJSON(`/channels/${id}`);
}

export async function addChannel(feedId: number, url: string): Promise<Channel> {
	return fetchJSON(`/feeds/${feedId}/channels`, {
		method: 'POST',
		body: JSON.stringify({ url })
	});
}

export async function deleteChannel(id: number): Promise<void> {
	return fetchJSON(`/channels/${id}`, { method: 'DELETE' });
}

export async function moveChannel(id: number, feedId: number): Promise<void> {
	return fetchJSON(`/channels/${id}/move`, {
		method: 'POST',
		body: JSON.stringify({ feedId })
	});
}

export async function refreshChannel(id: number): Promise<{
	videosFound: number;
	channel: string;
}> {
	return fetchJSON(`/channels/${id}/refresh`, { method: 'POST' });
}

// Videos
export async function getRecentVideos(limit = 100, offset = 0): Promise<{
	videos: Video[];
	progressMap: Record<string, WatchProgress>;
	total: number;
	offset: number;
	limit: number;
}> {
	return fetchJSON(`/videos/recent?limit=${limit}&offset=${offset}`);
}

export async function getHistory(limit = 100): Promise<{
	videos: Video[];
	progressMap: Record<string, WatchProgress>;
}> {
	return fetchJSON(`/videos/history?limit=${limit}`);
}

export async function getVideoInfo(id: string): Promise<{
	title: string;
	channel: string;
	streamURL: string;
	channelURL: string;
	channelMemberships: ChannelMembership[];
	viewCount: number;
	resumeFrom: number;
	thumbnail?: string;
}> {
	return fetchJSON(`/videos/${id}/info`);
}

export async function getNearbyVideos(id: string, limit = 20, offset = 0): Promise<{
	videos: Video[];
	feedId: number;
	progressMap: Record<string, WatchProgress>;
}> {
	return fetchJSON(`/videos/${id}/nearby?limit=${limit}&offset=${offset}`);
}

export async function updateProgress(id: string, progress: number, duration: number): Promise<void> {
	return fetchJSON(`/videos/${id}/progress`, {
		method: 'POST',
		body: JSON.stringify({ progress, duration })
	});
}

export async function markWatched(id: string): Promise<void> {
	return fetchJSON(`/videos/${id}/watched`, { method: 'POST' });
}

export async function markUnwatched(id: string): Promise<void> {
	return fetchJSON(`/videos/${id}/watched`, { method: 'DELETE' });
}

// Import
export async function importFromURL(url: string): Promise<Feed> {
	return fetchJSON('/import/url', {
		method: 'POST',
		body: JSON.stringify({ url })
	});
}

export async function importFromFile(file: File): Promise<Feed> {
	const formData = new FormData();
	formData.append('file', file);
	const res = await fetch(API_BASE + '/import/file', {
		method: 'POST',
		body: formData
	});
	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || res.statusText);
	}
	return res.json();
}

// Packs
export async function getPacks(): Promise<{
	name: string;
	description: string;
	author: string;
	tags: string[];
}[]> {
	return fetchJSON('/packs');
}

export async function getPack(name: string): Promise<{
	name: string;
	description: string;
	author: string;
	tags: string[];
	channels: { url: string; name: string }[];
}> {
	return fetchJSON(`/packs/${name}`);
}

// Watch History Import
export async function importWatchHistory(file: File): Promise<{
	channels: WatchHistoryChannel[];
	totalVideos: number;
}> {
	const formData = new FormData();
	formData.append('file', file);
	const res = await fetch(API_BASE + '/import/watch-history', {
		method: 'POST',
		body: formData
	});
	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || res.statusText);
	}
	return res.json();
}

export async function organizeWatchHistory(channels: WatchHistoryChannel[]): Promise<{
	groups: GroupSuggestion[];
}> {
	return fetchJSON('/import/watch-history/organize', {
		method: 'POST',
		body: JSON.stringify({ channels })
	});
}

export async function confirmOrganize(
	groups: { name: string; channels: string[] }[],
	channelNames: Record<string, string>
): Promise<{ feeds: Feed[] }> {
	return fetchJSON('/import/confirm', {
		method: 'POST',
		body: JSON.stringify({ groups, channelNames })
	});
}

// SponsorBlock
export async function getSegments(videoId: string): Promise<{
	segments: SponsorBlockSegment[];
	cached: boolean;
	error?: string;
}> {
	return fetchJSON(`/videos/${videoId}/segments`);
}

// Video Downloads
export async function startDownload(videoId: string, quality: string): Promise<{
	status: string;
	quality: string;
}> {
	return fetchJSON(`/videos/${videoId}/download`, {
		method: 'POST',
		body: JSON.stringify({ quality })
	});
}

export async function getQualities(videoId: string): Promise<{
	available: string[];
	cached: string[];
	downloading: string | null;
}> {
	return fetchJSON(`/videos/${videoId}/qualities`);
}

export function subscribeToDownloadProgress(
	videoId: string,
	onProgress: (data: { quality: string; percent: number; status: string; error?: string }) => void
): () => void {
	let closed = false;
	let es: EventSource | null = new EventSource(`/api/videos/${videoId}/download/status`);

	const handleMessage = (e: MessageEvent) => {
		const data = JSON.parse(e.data);
		onProgress(data);

		// Close on terminal states
		if (data.status === 'complete' || data.status === 'error') {
			closed = true;
			es?.close();
			es = null;
		}
	};

	es.addEventListener('progress', handleMessage);
	es.addEventListener('status', handleMessage);

	es.onerror = () => {
		if (closed || !es) return;

		// On error, poll for status instead of giving up
		es.close();
		es = null;

		// Poll to check if download completed while disconnected
		const poll = async () => {
			if (closed) return;
			try {
				const qualities = await getQualities(videoId);
				// Check if the quality we were downloading is now cached
				// We don't know which quality, so just report if anything new is cached
				if (qualities.cached.length > 0 && !qualities.downloading) {
					// Download completed while we were disconnected
					onProgress({ quality: qualities.cached[0], percent: 100, status: 'complete' });
					closed = true;
				} else if (qualities.downloading) {
					// Still downloading, try to reconnect
					setTimeout(() => {
						if (closed) return;
						es = new EventSource(`/api/videos/${videoId}/download/status`);
						es.addEventListener('progress', handleMessage);
						es.addEventListener('status', handleMessage);
						es.onerror = () => {
							es?.close();
							es = null;
							if (!closed) setTimeout(poll, 2000);
						};
					}, 1000);
				}
			} catch {
				// Retry poll after delay
				if (!closed) setTimeout(poll, 2000);
			}
		};
		poll();
	};

	return () => {
		closed = true;
		es?.close();
		es = null;
	};
}
