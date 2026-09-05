<script lang="ts">
  import { launchProfileFlow, reloadCurrentFlow, renameProfileFlow } from '$lib/rimworld/actions';
  import { revealPath } from '$lib/shell/api';
  import { app } from '$lib/rimworld/stores/app.svelte';
  import { layout } from '$lib/shell/stores/layout.svelte';
  import { toastError } from '$lib/shell/stores/toasts.svelte';
  import { absoluteTime, relativeTime } from '$lib/shell/format';
  import Icon from '$lib/shell/components/Icon.svelte';
  import ModListEditor from '$lib/rimworld/components/ModListEditor.svelte';

  const profile = $derived(app.selected);
  const busy = $derived(!!profile && app.busyProfileId === profile.id);
  /** The sidebar is off to the side only while it's docked. */
  const showSidebarToggle = $derived(!layout.sidebarVisible || layout.sidebarFloating);
</script>

{#if profile}
  <section class="pane">
    <header class="head">
      {#if showSidebarToggle}
        <button
          class="btn btn-ghost btn-icon reveal"
          title="Show profiles (Ctrl+B)"
          aria-label="Show profiles"
          onclick={() => layout.toggleSidebar()}
        >
          <Icon name="menu" size={16} />
        </button>
      {/if}

      <div class="titles">
        <div class="title-row">
          <h1 class="truncate">{profile.name}</h1>
          <button
            class="btn btn-ghost btn-icon rename"
            title="Rename profile"
            onclick={() => renameProfileFlow(profile.id)}
          >
            <Icon name="pencil" size={14} />
          </button>
          {#if app.dirty}
            <span class="dirty" title="These changes are not written to ModsConfig.xml yet">
              Unsaved<span class="lbl">&nbsp;changes</span>
            </span>
          {/if}
        </div>
        <div class="path-row">
          <Icon name="folder" size={12} />
          <button
            class="path mono truncate"
            title="Open {profile.path} in file manager"
            onclick={() => revealPath(profile.path).catch((e) => toastError('Open folder', e))}
          >
            {profile.path}
          </button>
          <span class="dot-sep">·</span>
          <span class="played" title={absoluteTime(profile.lastPlayedAtMs)}>
            {relativeTime(profile.lastPlayedAtMs)}
          </span>
        </div>
      </div>

      <div class="head-actions">
        <button
          class="btn"
          title="Discard local changes and re-read from disk"
          disabled={app.loadingActive || app.loadingMods}
          onclick={reloadCurrentFlow}
        >
          <Icon name="refresh" size={14} class={app.loadingActive || app.loadingMods ? 'spin' : ''} />
          <span class="lbl">Reload</span>
        </button>
        <button
          class="btn"
          title="Discard unsaved changes"
          disabled={!app.dirty}
          onclick={() => app.resetDraft()}
        >
          <Icon name="undo" size={14} /> <span class="lbl">Revert</span>
        </button>
        <button
          class="btn save"
          class:btn-primary={app.dirty}
          disabled={!app.dirty || app.saving}
          onclick={() => app.save()}
        >
          <Icon name="save" size={14} />
          <span class="lbl">{app.saving ? 'Saving…' : 'Save'}</span>
        </button>
        <div class="divider"></div>
        <button
          class="btn btn-primary launch"
          disabled={busy}
          onclick={() => launchProfileFlow(profile.id)}
        >
          <Icon name="play" size={14} /> <span class="launch-lbl">Launch</span>
        </button>
      </div>
    </header>

    <ModListEditor />
  </section>
{/if}

<style>
  .pane {
    grid-area: main;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    /* Everything below sizes itself against the pane, not the window, so the
       layout reacts to the sidebar collapsing as well as to window resizes. */
    container: pane / inline-size;
  }

  .reveal {
    flex: none;
    margin-left: -4px;
  }

  .head {
    flex: none;
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: var(--header-h);
    padding: 10px 16px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-sunken);
  }

  .titles {
    flex: 1;
    min-width: 0;
  }

  .title-row {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }

  h1 {
    font-size: 17px;
    font-weight: 650;
  }

  .rename {
    opacity: 0;
    transition: opacity var(--t-fast);
  }
  .title-row:hover .rename {
    opacity: 1;
  }

  .dirty {
    flex: none;
    margin-left: 4px;
    padding: 2px 8px;
    font-size: 10.5px;
    font-weight: 700;
    letter-spacing: 0.03em;
    color: var(--accent);
    background: var(--accent-soft);
    border: 1px solid var(--accent-line);
    border-radius: 99px;
  }

  .path-row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 2px;
    color: var(--text-faint);
    min-width: 0;
  }
  .path {
    min-width: 0;
    background: none;
    border: none;
    padding: 0;
    font-size: inherit;
    color: inherit;
    cursor: pointer;
    text-align: left;
  }
  .path:hover {
    color: var(--text);
    text-decoration: underline;
  }
  .dot-sep {
    opacity: 0.5;
  }
  .played {
    flex: none;
    font-size: 11px;
  }

  .head-actions {
    flex: none;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .divider {
    width: 1px;
    height: 22px;
    margin: 0 2px;
    background: var(--border);
  }

  .launch {
    padding-inline: 16px;
  }

  /* ---------- Tight panes ----------
     Header controls shed their labels before the profile name has to start
     losing characters; Launch keeps its label longest since it's the verb
     people came for. */

  @container pane (max-width: 720px) {
    .head-actions .lbl {
      display: none;
    }
    .head-actions .btn {
      padding-inline: 8px;
    }
    .divider {
      display: none;
    }
  }

  @container pane (max-width: 520px) {
    .head {
      gap: 8px;
      padding-inline: 10px;
    }
    h1 {
      font-size: 15.5px;
    }
    .launch {
      padding-inline: 10px;
    }
    .launch-lbl {
      display: none;
    }
    .rename {
      display: none;
    }
    .dirty .lbl {
      display: none;
    }
  }
</style>

