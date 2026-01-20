import { writable, get } from 'svelte/store';

export interface NavigationOrigin {
	feedId: number;
	feedName: string;
	path: string;
}

function createNavigationStore() {
	const { subscribe, set, update } = writable<NavigationOrigin | null>(null);

	return {
		subscribe,
		setOrigin: (origin: NavigationOrigin) => set(origin),
		clear: () => set(null),
		get: () => get({ subscribe })
	};
}

export const navigationOrigin = createNavigationStore();
