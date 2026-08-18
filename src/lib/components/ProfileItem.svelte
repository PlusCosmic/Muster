<script lang="ts">
  import {
    cloneProfileFlow,
    createShortcutFlow,
    deleteProfileFlow,
    launchProfileFlow,
    renameProfileFlow,
    selectProfileGuarded
  } from '$lib/actions';
  import { app } from '$lib/stores/app.svelte';
  import { absoluteTime, plural, relativeTime } from '$lib/format';
  import type { Profile } from '$lib/types';
  import ContextMenu, { type MenuItem } from './ContextMenu.svelte';
  import Icon from './Icon.svelte';

  interface Props {
    profile: Profile;
    selected: boolean;
  }
  let { profile, selected }: Props = $props();

  let menu = $state<{ x: number; y: number } | null>(null);

  const busy = $derived(app.busyProfileId === profile.id);
  const unsaved = $derived(selected && app.dirty);

  const items: MenuItem[] = $derived([
    { label: 'Launch RimWorld', icon: 'play', onselect: () => launchProfileFlow(profile.id) },
    {
      label: 'Create desktop shortcut',
      icon: 'link',
      separatorBefore: true,
      onselect: () => createShortcutFlow(profile.id)
    },
    { label: 'Clone…', icon: 'copy', onselect: () => cloneProfileFlow(profile.id) },
    { label: 'Rename…', icon: 'pencil', onselect: () => renameProfileFlow(profile.id) },
    {
      label: 'Delete…',
      icon: 'trash',
      danger: true,
      separatorBefore: true,
      onselect: () => deleteProfileFlow(profile.id)
    }
  ]);

  function openMenu(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    menu = { x: e.clientX, y: e.clientY };
  }

  function openMenuFromButton(e: MouseEvent) {
    e.stopPropagation();
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    menu = { x: rect.right - 4, y: rect.bottom + 4 };
  }
</script>

<li class="wrap">
  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <div
    class="item"
    class:selected
    class:busy
    role="button"
    tabindex="0"
    aria-current={selected ? 'true' : undefined}
    oncontextmenu={openMenu}
    onclick={() => selectProfileGuarded(profile.id)}
    onkeydown={(e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        selectProfileGuarded(profile.id);
      }
    }}
  >
    <span class="bar" aria-hidden="true"></span>

    <span class="main">
      <span class="name-row">
        <span class="name truncate">{profile.name}</span>
        {#if unsaved}
          <span class="dot" title="Unsaved mod-list changes"></span>
        {/if}
      </span>
      <span class="stats">
        <span title="Active mods">{profile.activeModCount} mods</span>
        <span class="sep">·</span>
        <span title="Saved games">{plural(profile.saveCount, 'save')}</span>
      </span>
      <span class="played" title={absoluteTime(profile.lastPlayedAtMs)}>
        {relativeTime(profile.lastPlayedAtMs)}
      </span>
    </span>

    <span class="hover-actions">
      <button
        class="act"
        title="Launch RimWorld with this profile"
        onclick={(e) => {
          e.stopPropagation();
          launchProfileFlow(profile.id);
        }}
      >
        <Icon name="play" size={13} />
      </button>
      <button class="act" title="More actions" onclick={openMenuFromButton}>
        <Icon name="dot" size={14} strokeWidth={3} />
      </button>
    </span>
  </div>
</li>

{#if menu}
  <ContextMenu x={menu.x} y={menu.y} {items} onclose={() => (menu = null)} />
{/if}

<style>
  .wrap {
    list-style: none;
  }

  .item {
    position: relative;
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 8px 8px 12px;
    font: inherit;
    text-align: left;
    color: var(--text);
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--r-md);
    cursor: pointer;
    transition: background var(--t-fast), border-color var(--t-fast);
  }
  .item:hover {
    background: var(--bg-hover);
  }
  .item.selected {
    background: var(--accent-soft);
    border-color: var(--accent-line);
  }
  .item.busy {
    opacity: 0.6;
    pointer-events: none;
  }

  .bar {
    position: absolute;
    left: 0;
    top: 8px;
    bottom: 8px;
    width: 3px;
    border-radius: 0 3px 3px 0;
    background: var(--accent);
    opacity: 0;
    transition: opacity var(--t-fast);
  }
  .item.selected .bar {
    opacity: 1;
  }

  .main {
    flex: 1;
    min-width: 0;
    display: grid;
    gap: 1px;
  }

  .name-row {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }

  .name {
    font-size: 13px;
    font-weight: 600;
  }
  .item.selected .name {
    color: #f4d3bd;
  }

  .dot {
    flex: none;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-soft);
  }

  .stats,
  .played {
    font-size: 11px;
    color: var(--text-faint);
    display: flex;
    gap: 5px;
  }
  .sep {
    opacity: 0.5;
  }

  .hover-actions {
    flex: none;
    display: flex;
    gap: 1px;
    opacity: 0;
    transition: opacity var(--t-fast);
  }
  .item:hover .hover-actions,
  .item:focus-within .hover-actions,
  .item.selected .hover-actions {
    opacity: 1;
  }

  .act {
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    color: var(--text-muted);
    background: transparent;
    border: none;
    border-radius: var(--r-sm);
    cursor: pointer;
    transition: background var(--t-fast), color var(--t-fast);
  }
  .act:hover {
    background: var(--bg-active);
    color: var(--accent);
  }
</style>
