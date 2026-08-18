<script lang="ts" module>
  import type { IconName } from './Icon.svelte';

  export interface MenuItem {
    label: string;
    icon?: IconName;
    danger?: boolean;
    separatorBefore?: boolean;
    disabled?: boolean;
    onselect: () => void;
  }
</script>

<script lang="ts">
  import Icon from './Icon.svelte';

  interface Props {
    x: number;
    y: number;
    items: MenuItem[];
    onclose: () => void;
  }
  let { x, y, items, onclose }: Props = $props();

  let el = $state<HTMLDivElement | null>(null);
  // Set once the menu has been measured, so it can be flipped back on-screen.
  let pos = $state<{ left: number; top: number } | null>(null);

  $effect(() => {
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const left = Math.min(x, window.innerWidth - rect.width - 8);
    const top = Math.min(y, window.innerHeight - rect.height - 8);
    pos = { left: Math.max(8, left), top: Math.max(8, top) };
    el.querySelector<HTMLElement>('button:not(:disabled)')?.focus();
  });

  function choose(item: MenuItem) {
    onclose();
    item.onselect();
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation();
      onclose();
    }
  }
</script>

<svelte:window onpointerdown={onclose} onresize={onclose} {onkeydown} />

<div
  class="menu"
  role="menu"
  tabindex="-1"
  bind:this={el}
  style:left="{pos ? pos.left : x}px"
  style:top="{pos ? pos.top : y}px"
  style:visibility={pos ? 'visible' : 'hidden'}
  onpointerdown={(e) => e.stopPropagation()}
>
  {#each items as item, i (item.label + i)}
    {#if item.separatorBefore}<hr />{/if}
    <button
      class="item"
      class:danger={item.danger}
      role="menuitem"
      disabled={item.disabled}
      onclick={() => choose(item)}
    >
      {#if item.icon}<Icon name={item.icon} size={14} />{:else}<span class="gap"></span>{/if}
      {item.label}
    </button>
  {/each}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 300;
    min-width: 186px;
    padding: 4px;
    background: var(--bg-raised);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
    box-shadow: var(--shadow-lg);
    animation: pop 100ms ease;
  }

  .item {
    display: flex;
    align-items: center;
    gap: 9px;
    width: 100%;
    padding: 6px 10px;
    font: inherit;
    font-size: 12.5px;
    text-align: left;
    color: var(--text);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    cursor: pointer;
    transition: background var(--t-fast), color var(--t-fast);
  }
  .item:hover:not(:disabled),
  .item:focus-visible {
    background: var(--bg-hover);
  }
  .item:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .item.danger {
    color: #eb8b89;
  }
  .item.danger:hover:not(:disabled) {
    background: var(--danger-soft);
    color: #f5b6b4;
  }

  .gap {
    width: 14px;
  }

  hr {
    margin: 4px 6px;
    border: none;
    border-top: 1px solid var(--border);
  }

  @keyframes pop {
    from {
      opacity: 0;
      transform: scale(0.97);
    }
  }
</style>
