<script lang="ts">
  import type { ModInfo } from '$lib/types';
  import Icon from './Icon.svelte';
  import SourceBadge from './SourceBadge.svelte';

  interface Props {
    mod: ModInfo;
    /** Which column this row lives in. */
    column: 'active' | 'inactive';
    /** 1-based load position, shown in the active column. */
    position?: number;
    mismatch?: boolean;
    /** Core can't be removed from the load order. */
    locked?: boolean;
    /** Active id with no installed mod behind it. */
    missing?: boolean;
    /** Flagged by the last auto-sort run. */
    warned?: boolean;
    dragging?: boolean;
    dropEdge?: 'above' | 'below' | null;
    gameVersion?: string | null;
    onmove: () => void;
    onmoveup?: () => void;
    onmovedown?: () => void;
    ondragstart?: (e: DragEvent) => void;
    ondragover?: (e: DragEvent) => void;
    ondragleave?: (e: DragEvent) => void;
    ondrop?: (e: DragEvent) => void;
    ondragend?: (e: DragEvent) => void;
  }

  let {
    mod,
    column,
    position,
    mismatch = false,
    locked = false,
    missing = false,
    warned = false,
    dragging = false,
    dropEdge = null,
    gameVersion = null,
    onmove,
    onmoveup,
    onmovedown,
    ondragstart,
    ondragover,
    ondragleave,
    ondrop,
    ondragend
  }: Props = $props();

  const canMove = $derived(!(column === 'active' && locked));

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      if (canMove) onmove();
    } else if (e.altKey && e.key === 'ArrowUp') {
      e.preventDefault();
      onmoveup?.();
    } else if (e.altKey && e.key === 'ArrowDown') {
      e.preventDefault();
      onmovedown?.();
    }
  }

  const versionTitle = $derived(
    mod.supportedVersions.length
      ? `Supports ${mod.supportedVersions.join(', ')} — game is ${gameVersion ?? 'unknown'}`
      : 'No supportedVersions listed in About.xml'
  );
</script>

<li
  class="row"
  class:dragging
  class:missing
  class:warned
  class:drop-above={dropEdge === 'above'}
  class:drop-below={dropEdge === 'below'}
  draggable="true"
  role="option"
  aria-selected="false"
  tabindex="0"
  title={mod.packageId}
  {onkeydown}
  ondblclick={() => canMove && onmove()}
  {ondragstart}
  {ondragover}
  {ondragleave}
  {ondrop}
  {ondragend}
>
  {#if column === 'active'}
    <span class="grip" aria-hidden="true"><Icon name="grip" size={14} /></span>
    <span class="pos">{position}</span>
  {/if}

  {#if mod.source === 'official' && !missing}
    <span class="gem" aria-hidden="true">◆</span>
  {/if}
  <span class="name truncate">{mod.name}</span>

  <span class="meta">
    {#if mismatch}
      <span class="flag mismatch" title={versionTitle}>
        <Icon name="alert" size={12} />
        <span class="flag-text">{mod.supportedVersions[0] ?? '?'}</span>
      </span>
    {/if}
    <SourceBadge source={mod.source} unknown={missing} />
    {#if !canMove}
      <span class="lock" title="Core is required and can’t be removed">req</span>
    {/if}
  </span>

  <span class="actions">
    {#if column === 'active' && !locked}
      <button
        class="mini"
        title="Move up (Alt+↑)"
        onclick={(e) => {
          e.stopPropagation();
          onmoveup?.();
        }}><Icon name="chevronUp" size={13} /></button
      >
      <button
        class="mini"
        title="Move down (Alt+↓)"
        onclick={(e) => {
          e.stopPropagation();
          onmovedown?.();
        }}><Icon name="chevronDown" size={13} /></button
      >
    {/if}
    {#if canMove}
      <button
        class="mini move"
        title={column === 'active' ? 'Deactivate' : 'Activate'}
        onclick={(e) => {
          e.stopPropagation();
          onmove();
        }}
      >
        <Icon name={column === 'active' ? 'chevronLeft' : 'chevronRight'} size={14} />
      </button>
    {/if}
  </span>
</li>

<style>
  .row {
    position: relative;
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 5px 6px 5px 8px;
    min-height: 30px;
    border-radius: var(--r-sm);
    border: 1px solid transparent;
    cursor: grab;
    transition: background var(--t-fast), border-color var(--t-fast);
  }
  .row:hover {
    background: var(--bg-hover);
  }
  .row:focus-visible {
    background: var(--bg-hover);
  }
  .row:active {
    cursor: grabbing;
  }
  .row.dragging {
    opacity: 0.4;
  }
  .row.missing .name {
    color: var(--danger);
    font-family: var(--font-mono);
    font-size: 12px;
  }
  .row.warned {
    background: var(--warn-soft);
  }

  .row.drop-above::before,
  .row.drop-below::after {
    content: '';
    position: absolute;
    left: 4px;
    right: 4px;
    height: 2px;
    background: var(--accent);
    border-radius: 2px;
  }
  .row.drop-above::before {
    top: -2px;
  }
  .row.drop-below::after {
    bottom: -2px;
  }

  .grip {
    color: var(--text-faint);
    opacity: 0;
    transition: opacity var(--t-fast);
  }
  .row:hover .grip {
    opacity: 1;
  }

  .pos {
    flex: none;
    min-width: 20px;
    font-family: var(--font-mono);
    font-size: 10.5px;
    color: var(--text-faint);
    text-align: right;
  }

  .name {
    flex: 1;
    min-width: 0;
    font-size: 12.5px;
  }

  .gem {
    flex: none;
    margin-right: -2px;
    font-size: 8px;
    color: var(--src-official);
  }

  .meta {
    flex: none;
    display: flex;
    align-items: center;
    gap: 5px;
  }

  .flag {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 1px 5px 1px 4px;
    font-size: 9.5px;
    font-weight: 700;
    letter-spacing: 0.03em;
    border-radius: 3px;
    color: var(--warn);
    background: var(--warn-soft);
    border: 1px solid rgba(224, 163, 60, 0.35);
  }
  .flag-text {
    font-family: var(--font-mono);
  }

  .actions {
    flex: none;
    display: flex;
    align-items: center;
    gap: 2px;
    opacity: 0;
    transition: opacity var(--t-fast);
  }
  .row:hover .actions,
  .row:focus-within .actions,
  .row:focus-visible .actions {
    opacity: 1;
  }

  .mini {
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    padding: 0;
    color: var(--text-muted);
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--r-sm);
    cursor: pointer;
    transition: background var(--t-fast), color var(--t-fast);
  }
  .mini:hover {
    background: var(--bg-active);
    color: var(--text);
  }
  .mini.move:hover {
    color: var(--accent);
  }

  .lock {
    padding: 0 6px;
    font-size: 9.5px;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--text-faint);
    opacity: 1;
  }

  /* Single-column mode (see ModListEditor): the two lists aren't both on
     screen, so dragging between them is awkward — keep the move buttons
     permanently visible instead of revealing them on hover. */
  @container editor (max-width: 780px) {
    .actions {
      opacity: 1;
    }
    .grip {
      opacity: 1;
    }
  }
</style>
