import type { Feed, Channel, Video, WatchProgress, Config, WatchHistoryChannel, GroupSuggestion } from './types';

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

// Feeds
export async function getFeeds(): Promise<Feed[]> {
	return fetchJSON('/feeds');
}

export async function getFeed(id: number): Promise<{
	feed: Feed;
	channels: Channel[];
	videos: Video[];
	progressMap: Record<string, WatchProgress>;
	allFeeds: Feed[];
}> {
	return fetchJSON(`/feeds/${id}`);
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
export async function getRecentVideos(limit = 100): Promise<{
	videos: Video[];
	progressMap: Record<string, WatchProgress>;
}> {
	return fetchJSON(`/videos/recent?limit=${limit}`);
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
	existingChannelID: number;
	resumeFrom: number;
}> {
	return fetchJSON(`/videos/${id}/info`);
}

export async function getNearbyVideos(id: string, limit = 20): Promise<{
	videos: Video[];
	feedId: number;
	progressMap: Record<string, WatchProgress>;
}> {
	return fetchJSON(`/videos/${id}/nearby?limit=${limit}`);
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
