<script lang="ts">
  import { fly } from 'svelte/transition';

  let { open = $bindable(false) } = $props();

  function handleClose() {
    open = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      handleClose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 bg-black/60 backdrop-blur-sm z-[9998]"
    onclick={handleClose}
    onkeydown={(e) => e.key === 'Enter' && handleClose()}
    role="button"
    tabindex="-1"
    aria-label="Close menu"
    transition:fly={{ duration: 200, opacity: 0 }}
  ></div>

  <!-- Menu Panel -->
  <div
    class="fixed top-0 left-0 bottom-0 w-72 max-w-[80vw] bg-surface border-r border-border-subtle z-[9999] flex flex-col"
    transition:fly={{ x: -288, duration: 300, opacity: 1 }}
  >
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-border-subtle">
      <span class="text-lg font-display font-semibold text-text-primary">Menu</span>
      <button
        onclick={handleClose}
        class="p-2 -m-2 text-text-muted hover:text-text-primary transition-colors"
        aria-label="Close menu"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Menu Items -->
    <nav class="flex-1 overflow-y-auto py-2">
      <!-- Import/Export Section -->
      <div class="px-3 py-2">
        <div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Import & Export</div>
        <a
          href="/import"
          onclick={handleClose}
          class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
          </svg>
          <span>Import watch history</span>
        </a>
        <a
          href="/import#packs"
          onclick={handleClose}
          class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
          </svg>
          <span>Subscription packs</span>
        </a>
      </div>

      <div class="my-2 border-t border-border-subtle"></div>

      <!-- Views Section -->
      <div class="px-3 py-2">
        <div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Views</div>
        <a
          href="/all"
          onclick={handleClose}
          class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
          </svg>
          <span>All videos</span>
        </a>
        <a
          href="/history"
          onclick={handleClose}
          class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span>Watch history</span>
        </a>
      </div>
    </nav>

    <!-- Footer with safe area -->
    <div class="p-4 border-t border-border-subtle" style="padding-bottom: max(1rem, env(safe-area-inset-bottom));">
      <a
        href="/settings"
        onclick={handleClose}
        class="flex items-center gap-3 px-3 py-3 rounded-lg text-text-secondary hover:text-text-primary hover:bg-elevated transition-colors"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <span>Settings</span>
      </a>
    </div>
  </div>
{/if}
