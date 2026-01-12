<script lang="ts">
	import '../app.css';

	let { children } = $props();
</script>

<svelte:head>
	<title>Feeds</title>
</svelte:head>

<div class="bg-gray-900 text-gray-100 min-h-screen">
	<!-- Header -->
	<header class="sticky top-0 z-40 bg-gray-800 border-b border-gray-700">
		<div class="max-w-6xl mx-auto px-4 py-3 flex items-center justify-between">
			<a href="/" class="text-xl font-bold hover:text-blue-400">Feeds</a>
			<div class="flex items-center gap-3">
				<a href="/history" class="text-gray-400 hover:text-white text-sm">
					History
				</a>
				<a href="/import" class="bg-blue-600 hover:bg-blue-700 text-white py-2 px-4 rounded text-sm">
					+ Import
				</a>
			</div>
		</div>
	</header>

	<main class="max-w-6xl mx-auto p-4">
		{@render children()}
	</main>
</div>
