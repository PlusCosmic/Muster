<script lang="ts">
  import { onMount } from 'svelte';
  import EmptyState from '$lib/rimworld/components/EmptyState.svelte';
  import Icon from '$lib/shell/components/Icon.svelte';
  import { anyModalOpen } from '$lib/shell/components/Modal.svelte';
  import ProfilePane from '$lib/rimworld/components/ProfilePane.svelte';
  import SettingsModal from '$lib/rimworld/components/SettingsModal.svelte';
  import Sidebar from '$lib/rimworld/components/Sidebar.svelte';
  import { app } from '$lib/rimworld/stores/app.svelte';
  import { layout } from '$lib/shell/stores/layout.svelte';
  import { newProfileFlow } from '$lib/rimworld/actions';

  let settingsOpen = $state(false);

  onMount(() => {
    app.init();
    return layout.watch();
  });

  function onkeydown(e: KeyboardEvent) {
    const mod = e.ctrlKey || e.metaKey;
    // A modal is above the drawer, so it gets the keystroke to itself.
    if (e.key === 'Escape' && !anyModalOpen() && layout.narrow && layout.drawerOpen) {
      layout.closeDrawer();
    } else if (mod && e.key.toLowerCase() === 'b') {
      e.preventDefault();
      layout.toggleSidebar();
    } else if (mod && e.key.toLowerCase() === 's') {
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
  <title>Muster · RimWorld</title>
</svelte:head>

<svelte:window {onkeydown} {onbeforeunload} />

<div class="shell" class:solo={!layout.sidebarVisible || layout.sidebarFloating}>
  {#if layout.sidebarVisible}
    <Sidebar floating={layout.sidebarFloating} onopensettings={() => (settingsOpen = true)} />
  {/if}
  {#if layout.sidebarFloating && layout.drawerOpen}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="scrim" onclick={() => layout.closeDrawer()}></div>
  {/if}

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
      <span>
        {layout.sidebarVisible
          ? 'Select a profile from the left, or create a new one.'
          : 'Open the profile list, or create a new one.'}
      </span>
      <div class="boot-actions">
        {#if !layout.sidebarVisible}
          <button class="btn" onclick={() => layout.toggleSidebar()}>
            <Icon name="panelLeft" size={14} /> Show profiles
          </button>
        {/if}
        <button class="btn btn-primary" onclick={newProfileFlow}>
          <Icon name="plus" size={14} /> New profile
        </button>
      </div>
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
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background: var(--bg-app);
  }

  /* Sidebar collapsed, or floating over the content as a drawer: the main
     pane gets the whole width. */
  .shell.solo {
    grid-template-areas: 'main';
    grid-template-columns: minmax(0, 1fr);
  }

  .scrim {
    position: fixed;
    inset: 0 0 0 var(--rail-w);
    z-index: 90;
    background: var(--bg-overlay);
    animation: rf-fade var(--t-med);
  }

  @keyframes rf-fade {
    from {
      opacity: 0;
    }
  }

  .boot-actions {
    display: flex;
    gap: 8px;
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
