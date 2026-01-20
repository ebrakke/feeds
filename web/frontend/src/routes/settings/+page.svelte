<script lang="ts">
  import { onMount } from 'svelte';

  let cookiesValue = $state('');
  let saving = $state(false);
  let message = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  onMount(async () => {
    try {
      const res = await fetch('/api/config');
      const data = await res.json();
      if (data.hasCookies) {
        cookiesValue = '(cookies configured)';
      }
    } catch (e) {
      console.error('Failed to load config:', e);
    }
  });

  async function saveCookies() {
    if (!cookiesValue || cookiesValue === '(cookies configured)') return;

    saving = true;
    message = null;

    try {
      const res = await fetch('/api/config/ytdlp-cookies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ cookies: cookiesValue })
      });

      if (res.ok) {
        message = { type: 'success', text: 'Cookies saved successfully' };
        cookiesValue = '(cookies configured)';
      } else {
        const data = await res.json();
        message = { type: 'error', text: data.error || 'Failed to save cookies' };
      }
    } catch (e) {
      message = { type: 'error', text: 'Failed to save cookies' };
    } finally {
      saving = false;
    }
  }
</script>

<svelte:head>
  <title>Settings - Feeds</title>
</svelte:head>

<div class="space-y-8">
  <div>
    <h1 class="text-2xl font-display font-bold text-text-primary">Settings</h1>
    <p class="text-text-secondary mt-1">Configure your Feeds app</p>
  </div>

  <!-- YouTube Cookies Section -->
  <div class="card p-6 space-y-4">
    <div>
      <h2 class="text-lg font-semibold text-text-primary">YouTube Cookies</h2>
      <p class="text-sm text-text-secondary mt-1">
        Provide cookies for age-restricted or member-only content
      </p>
    </div>

    <div class="space-y-3">
      <textarea
        bind:value={cookiesValue}
        placeholder="Paste your cookies.txt content here..."
        class="input w-full h-32 font-mono text-sm resize-none"
        disabled={saving}
      ></textarea>

      {#if message}
        <p class="text-sm {message.type === 'success' ? 'text-emerald-500' : 'text-crimson-500'}">
          {message.text}
        </p>
      {/if}

      <button
        onclick={saveCookies}
        disabled={saving || !cookiesValue || cookiesValue === '(cookies configured)'}
        class="btn btn-primary"
      >
        {saving ? 'Saving...' : 'Save Cookies'}
      </button>
    </div>
  </div>

  <!-- About Section -->
  <div class="card p-6 space-y-4">
    <div>
      <h2 class="text-lg font-semibold text-text-primary">About</h2>
      <p class="text-sm text-text-secondary mt-1">
        Feeds is a personal YouTube feed aggregator
      </p>
    </div>
  </div>
</div>
