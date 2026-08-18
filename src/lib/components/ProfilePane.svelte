<script lang="ts">
  import { launchProfileFlow, reloadCurrentFlow, renameProfileFlow } from '$lib/actions';
  import { app } from '$lib/stores/app.svelte';
  import { absoluteTime, relativeTime } from '$lib/format';
  import Icon from './Icon.svelte';
  import ModListEditor from './ModListEditor.svelte';

  const profile = $derived(app.selected);
  const busy = $derived(!!profile && app.busyProfileId === profile.id);
</script>

{#if profile}
  <section class="pane">
    <header class="head">
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
              Unsaved changes
            </span>
          {/if}
        </div>
        <div class="path-row">
          <Icon name="folder" size={12} />
          <span class="path mono truncate" data-selectable title={profile.path}>{profile.path}</span>
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
          Reload
        </button>
        <button class="btn" disabled={!app.dirty} onclick={() => app.resetDraft()}>
          <Icon name="undo" size={14} /> Revert
        </button>
        <button
          class="btn save"
          class:btn-primary={app.dirty}
          disabled={!app.dirty || app.saving}
          onclick={() => app.save()}
        >
          <Icon name="save" size={14} />
          {app.saving ? 'Saving…' : 'Save'}
        </button>
        <div class="divider"></div>
        <button
          class="btn btn-primary launch"
          disabled={busy}
          onclick={() => launchProfileFlow(profile.id)}
        >
          <Icon name="play" size={14} /> Launch
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
  }

  .head {
    flex: none;
    display: flex;
    align-items: center;
    gap: 16px;
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
</style>
