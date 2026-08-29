<script lang="ts">
  import { onMount } from 'svelte';
  import { importDefaultFlow, newProfileFlow } from '$lib/actions';
  import { app } from '$lib/stores/app.svelte';
  import { layout } from '$lib/stores/layout.svelte';
  import Icon from './Icon.svelte';
  import ProfileItem from './ProfileItem.svelte';

  interface Props {
    /** Narrow windows show the sidebar as a drawer over the mod columns. */
    floating?: boolean;
    onopensettings: () => void;
  }
  let { floating = false, onopensettings }: Props = $props();

  /** Anything that takes the user to the main pane should get out of the way. */
  function leave() {
    if (floating) layout.closeDrawer();
  }

  let filter = $state('');
  let root = $state<HTMLElement | null>(null);

  // As a drawer this is a layer over the content, so focus has to follow it in:
  // the trigger that opened it sits behind the scrim, and Tab from there walks
  // the obscured controls instead of the profile list. Docked, it is just part
  // of the page and must not steal focus.
  onMount(() => {
    if (!floating) return;

    const returnTo = document.activeElement as HTMLElement | null;
    root
      ?.querySelector<HTMLElement>('a[href], button:not(:disabled), input:not(:disabled)')
      ?.focus();

    return () => {
      // Only hand focus back if it is still in here. If the user has moved on —
      // or the window widened and the drawer became the docked column — leave
      // it where they put it.
      if (root?.contains(document.activeElement) && returnTo?.isConnected) {
        returnTo.focus();
      }
    };
  });

  const shown = $derived.by(() => {
    const q = filter.trim().toLowerCase();
    const list = q ? app.profiles.filter((p) => p.name.toLowerCase().includes(q)) : app.profiles;
    return [...list].sort(
      (a, b) => (b.lastPlayedAtMs ?? b.createdAtMs) - (a.lastPlayedAtMs ?? a.createdAtMs)
    );
  });
</script>

<aside class="sidebar" class:floating bind:this={root}>
  <div class="brand">
    <!-- Inline copy of assets/rimforge.svg, recolored per theme via the
         mark tokens so it follows live theme previews too. -->
    <svg class="mark" viewBox="0 0 1024 1024" aria-hidden="true">
      <defs>
        <linearGradient id="mark-tile" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" class="g1" />
          <stop offset="1" class="g2" />
        </linearGradient>
      </defs>
      <rect width="1024" height="1024" rx="212" fill="url(#mark-tile)" />
      <g class="fg">
        <path d="M 400 420 L 624 420 Q 646 420 656 440 L 700 540 Q 708 560 684 560 L 340 560 Q 316 560 324 540 L 368 440 Q 378 420 400 420 Z" />
        <path d="M 208 600 L 432 600 Q 454 600 464 620 L 508 720 Q 516 740 492 740 L 148 740 Q 124 740 132 720 L 176 620 Q 186 600 208 600 Z" />
        <path d="M 592 600 L 816 600 Q 838 600 848 620 L 892 720 Q 900 740 876 740 L 532 740 Q 508 740 516 720 L 560 620 Q 570 600 592 600 Z" />
        <path d="M 512 180 l 22 56 56 22 -56 22 -22 56 -22 -56 -56 -22 56 -22 z" />
      </g>
    </svg>
    <div class="brand-text">
      <span class="app-name">RimForge</span>
      <span class="app-sub">Profile manager</span>
    </div>
    <button class="btn btn-ghost btn-icon" title="Settings" onclick={onopensettings}>
      <Icon name="gear" size={16} />
    </button>
    <button
      class="btn btn-ghost btn-icon"
      title={floating ? 'Close profile list (Ctrl+B)' : 'Collapse profile list (Ctrl+B)'}
      aria-label="Hide profile list"
      onclick={() => layout.toggleSidebar()}
    >
      <Icon name={floating ? 'x' : 'panelLeft'} size={16} />
    </button>
  </div>

  <div class="actions">
    <button class="btn btn-primary" onclick={() => { leave(); newProfileFlow(); }}>
      <Icon name="plus" size={14} /> New profile
    </button>
    <button
      class="btn"
      title="Copy your existing RimWorld setup into a profile"
      onclick={() => { leave(); importDefaultFlow(); }}
    >
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

  /* Drawer mode: out of the grid, over the content, with a shadow to sell it. */
  .sidebar.floating {
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 100;
    width: min(var(--sidebar-w), 86vw);
    box-shadow: var(--shadow-lg);
    animation: rf-slide-in var(--t-med);
  }

  @keyframes rf-slide-in {
    from {
      transform: translateX(-100%);
    }
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 6px;
    height: var(--header-h);
    padding: 0 8px 0 14px;
    border-bottom: 1px solid var(--border-subtle);
  }

  .mark {
    display: block;
    width: 28px;
    height: 28px;
    flex: none;
  }
  .mark .g1 {
    stop-color: var(--mark-grad-1);
  }
  .mark .g2 {
    stop-color: var(--mark-grad-2);
  }
  .mark .fg {
    fill: var(--mark-fg);
  }

  .brand-text {
    flex: 1;
    min-width: 0;
    display: grid;
    margin-right: 4px;
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
