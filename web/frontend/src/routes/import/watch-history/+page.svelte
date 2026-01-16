<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	onMount(() => {
		goto('/import');
	});
</script>

<svelte:head>
	<title>Import Watch History - Feeds</title>
</svelte:head>

<div class="max-w-3xl mx-auto">
	<h1 class="text-2xl font-bold mb-2">Import Watch History</h1>
	<p class="text-gray-400 mb-6">This flow moved to the main Import page.</p>
	<a href="/import" class="text-blue-400 hover:underline">Go to Import</a>
</div>
