<script lang="ts">
  import logo from '$lib/assets/rimforge.svg';
  import { importDefaultFlow, newProfileFlow } from '$lib/actions';
  import { app } from '$lib/stores/app.svelte';
  import Icon from './Icon.svelte';
  import ProfileItem from './ProfileItem.svelte';

  interface Props {
    onopensettings: () => void;
  }
  let { onopensettings }: Props = $props();

  let filter = $state('');

  const shown = $derived.by(() => {
    const q = filter.trim().toLowerCase();
    const list = q ? app.profiles.filter((p) => p.name.toLowerCase().includes(q)) : app.profiles;
    return [...list].sort(
      (a, b) => (b.lastPlayedAtMs ?? b.createdAtMs) - (a.lastPlayedAtMs ?? a.createdAtMs)
    );
  });
</script>

<aside class="sidebar">
  <div class="brand">
    <img class="mark" src={logo} alt="" />
    <div class="brand-text">
      <span class="app-name">RimForge</span>
      <span class="app-sub">Profile manager</span>
    </div>
    <button class="btn btn-ghost btn-icon" title="Settings" onclick={onopensettings}>
      <Icon name="gear" size={16} />
    </button>
  </div>

  <div class="actions">
    <button class="btn btn-primary" onclick={newProfileFlow}>
      <Icon name="plus" size={14} /> New profile
    </button>
    <button class="btn" title="Copy your existing RimWorld setup into a profile" onclick={importDefaultFlow}>
      <Icon name="import" size={14} /> Import current setup
    </button>
  </div>

  {#if app.profiles.length > 6}
    <div class="filter">
      <Icon name="search" size={13} />
      <input placeholder="Filter profiles" bind:value={filter} spellcheck="false" />
    </div>
  {/if}

  <div class="list-head">
    <span>Profiles</span>
    <span class="n">{app.profiles.length}</span>
  </div>

  <ul class="list">
    {#if app.loadingProfiles && app.profiles.length === 0}
      <li class="empty">Loading…</li>
    {:else if shown.length === 0}
      <li class="empty">
        {app.profiles.length === 0 ? 'No profiles yet.' : 'No profiles match that filter.'}
      </li>
    {:else}
      {#each shown as profile (profile.id)}
        <ProfileItem {profile} selected={profile.id === app.selectedId} />
      {/each}
    {/if}
  </ul>

  <footer>
    {#if app.detected?.gameVersion}
      <span class="ver" title="Detected from Version.txt">RimWorld {app.detected.gameVersion}</span>
    {:else}
      <span class="ver missing" title="Check the game install path in Settings">
        <Icon name="alert" size={12} /> Game not detected
      </span>
    {/if}
    <span class="mods">{app.installedMods.length} mods installed</span>
  </footer>
</aside>

<style>
  .sidebar {
    grid-area: sidebar;
    display: flex;
    flex-direction: column;
    min-height: 0;
    width: var(--sidebar-w);
    background: var(--bg-sunken);
    border-right: 1px solid var(--border);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    height: var(--header-h);
    padding: 0 10px 0 14px;
    border-bottom: 1px solid var(--border-subtle);
  }

  .mark {
    display: block;
    width: 28px;
    height: 28px;
  }

  .brand-text {
    flex: 1;
    min-width: 0;
    display: grid;
  }
  .app-name {
    font-size: 13.5px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }
  .app-sub {
    font-size: 10.5px;
    color: var(--text-faint);
  }

  .actions {
    display: grid;
    gap: 6px;
    padding: 12px;
  }
  .actions .btn {
    justify-content: flex-start;
  }

  .filter {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 12px 8px;
    padding: 0 8px;
    color: var(--text-faint);
    background: var(--bg-app);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .filter:focus-within {
    border-color: var(--accent);
  }
  .filter input {
    flex: 1;
    min-width: 0;
    padding: 5px 0;
    font: inherit;
    font-size: 12px;
    color: var(--text);
    background: none;
    border: none;
    outline: none;
  }

  .list-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 14px 6px;
    font-size: 10.5px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-faint);
  }
  .n {
    font-family: var(--font-mono);
    letter-spacing: 0;
  }

  .list {
    flex: 1;
    min-height: 0;
    margin: 0;
    padding: 0 8px 8px;
    list-style: none;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .empty {
    list-style: none;
    padding: 18px 12px;
    text-align: center;
    font-size: 12px;
    color: var(--text-faint);
  }

  footer {
    flex: none;
    display: grid;
    gap: 1px;
    padding: 8px 14px;
    border-top: 1px solid var(--border-subtle);
    font-size: 10.5px;
    color: var(--text-faint);
  }
  .ver {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .ver.missing {
    color: var(--warn);
  }
  .mods {
    opacity: 0.75;
  }
</style>
