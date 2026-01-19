import { writable } from 'svelte/store';

export interface Toast {
	id: number;
	message: string;
	type: 'success' | 'error';
}

function createToastStore() {
	const { subscribe, update } = writable<Toast[]>([]);
	let nextId = 0;

	return {
		subscribe,
		show(message: string, type: 'success' | 'error' = 'success') {
			const id = nextId++;
			update((toasts) => [...toasts, { id, message, type }]);
			setTimeout(() => {
				update((toasts) => toasts.filter((t) => t.id !== id));
			}, 3000);
		},
		success(message: string) {
			this.show(message, 'success');
		},
		error(message: string) {
			this.show(message, 'error');
		}
	};
}

export const toast = createToastStore();
