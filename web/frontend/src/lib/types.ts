export interface Feed {
	id: number;
	name: string;
	description?: string;
	author?: string;
	tags?: string;
	created_at: string;
	updated_at: string;
}

export interface Channel {
	id: number;
	feed_id: number;
	url: string;
	name: string;
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
}

export interface WatchProgress {
	video_id: string;
	progress_seconds: number;
	duration_seconds: number;
	completed: boolean;
	last_watched: string;
}

export interface Config {
	aiEnabled: boolean;
}
