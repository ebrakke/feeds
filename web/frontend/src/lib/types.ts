export interface Feed {
	id: number;
	name: string;
	description?: string;
	author?: string;
	tags?: string;
	is_system?: boolean;
	created_at: string;
	updated_at: string;
}

export interface Channel {
	id: number;
	feed_id: number;
	url: string;
	name: string;
}

export interface ChannelMembership {
	channelId: number;
	feedId: number;
	feedName: string;
}

export interface Video {
	id: string;
	channel_id: number;
	title: string;
	channel_name: string;
	thumbnail: string;
	duration: number;
	published: string;
	url: string;
	is_short: boolean | null;
}

export interface WatchProgress {
	video_id: string;
	progress_seconds: number;
	duration_seconds: number;
	watched_at: string;
}

export interface Config {
	ytdlpCookiesConfigured: boolean;
}

export interface WatchHistoryChannel {
	url: string;
	name: string;
	watch_count: number;
}

export interface GroupSuggestion {
	name: string;
	channels: { url: string; name: string }[];
}

export interface SponsorBlockSegment {
	uuid: string;
	startTime: number;
	endTime: number;
	category: string;
	actionType: string;
	votes: number;
}
