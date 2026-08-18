<script lang="ts">
  import { onMount } from 'svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import ProfilePane from '$lib/components/ProfilePane.svelte';
  import SettingsModal from '$lib/components/SettingsModal.svelte';
  import Sidebar from '$lib/components/Sidebar.svelte';
  import { app } from '$lib/stores/app.svelte';
  import { newProfileFlow } from '$lib/actions';

  let settingsOpen = $state(false);

  onMount(() => {
    app.init();
  });

  function onkeydown(e: KeyboardEvent) {
    const mod = e.ctrlKey || e.metaKey;
    if (mod && e.key.toLowerCase() === 's') {
      e.preventDefault();
      if (app.dirty && !app.saving) app.save();
    } else if (mod && e.key === ',') {
      e.preventDefault();
      settingsOpen = true;
    }
  }

  // Warn on close-with-unsaved-changes where the webview supports it.
  function onbeforeunload(e: BeforeUnloadEvent) {
    if (app.dirty) e.preventDefault();
  }
</script>

<svelte:head>
  <title>RimForge</title>
</svelte:head>

<svelte:window {onkeydown} {onbeforeunload} />

<div class="shell">
  <Sidebar onopensettings={() => (settingsOpen = true)} />

  {#if app.booting}
    <div class="boot">
      <span class="spinner" aria-hidden="true"></span>
      <span>Looking for RimWorld…</span>
    </div>
  {:else if app.profiles.length === 0}
    <EmptyState onopensettings={() => (settingsOpen = true)} />
  {:else if app.selected}
    <ProfilePane />
  {:else}
    <div class="boot">
      <Icon name="folder" size={24} />
      <span>Select a profile from the left, or create a new one.</span>
      <button class="btn btn-primary" onclick={newProfileFlow}>
        <Icon name="plus" size={14} /> New profile
      </button>
    </div>
  {/if}
</div>

{#if settingsOpen}
  <SettingsModal onclose={() => (settingsOpen = false)} />
{/if}

<style>
  .shell {
    display: grid;
    grid-template-areas: 'sidebar main';
    grid-template-columns: var(--sidebar-w) minmax(0, 1fr);
    height: 100vh;
    min-height: 0;
    overflow: hidden;
    background: var(--bg-app);
  }

  .boot {
    grid-area: main;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 14px;
    color: var(--text-faint);
    font-size: 13px;
  }

  .spinner {
    width: 22px;
    height: 22px;
    border: 2px solid var(--border-strong);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: rf-spin 800ms linear infinite;
  }

  @keyframes rf-spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
