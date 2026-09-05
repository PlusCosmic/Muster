<script lang="ts">
  import type { Snippet } from 'svelte';
  import '../app.css';
  import DialogHost from '$lib/shell/components/DialogHost.svelte';
  import GameRail from '$lib/shell/components/GameRail.svelte';
  import ToastHost from '$lib/shell/components/ToastHost.svelte';
  import { theme } from '$lib/shell/stores/theme.svelte';
  import { titlebar } from '$lib/shell/stores/titlebar.svelte';

  let { children }: { children: Snippet } = $props();

  theme.apply();
  titlebar.apply();
</script>

<div class="app">
  <GameRail />
  <div class="game">
    {@render children()}
  </div>
</div>

<DialogHost />
<ToastHost />

<style>
  .app {
    display: grid;
    grid-template-areas: 'rail game';
    grid-template-columns: var(--rail-w) minmax(0, 1fr);
    height: 100vh;
    overflow: hidden;
    background: var(--bg-app);
  }
  .game {
    grid-area: game;
    min-width: 0;
    min-height: 0;
    display: grid;
  }
</style>
