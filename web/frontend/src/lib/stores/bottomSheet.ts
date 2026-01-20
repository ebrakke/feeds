import { writable } from 'svelte/store';
import type { Feed } from '$lib/types';

export interface BottomSheetState {
	open: boolean;
	title: string;
	channelId: number | null;
	channelName: string;
	feeds: Feed[];
	memberFeedIds: number[]; // feeds this channel is already in
}

const initialState: BottomSheetState = {
	open: false,
	title: '',
	channelId: null,
	channelName: '',
	feeds: [],
	memberFeedIds: []
};

function createBottomSheetStore() {
	const { subscribe, set } = writable<BottomSheetState>(initialState);

	return {
		subscribe,
		open(options: {
			title: string;
			channelId: number;
			channelName: string;
			feeds: Feed[];
			memberFeedIds: number[];
		}) {
			set({
				open: true,
				title: options.title,
				channelId: options.channelId,
				channelName: options.channelName,
				feeds: options.feeds,
				memberFeedIds: options.memberFeedIds
			});
		},
		close() {
			set(initialState);
		}
	};
}

export const bottomSheet = createBottomSheetStore();
