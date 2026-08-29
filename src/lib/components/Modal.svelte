<script module lang="ts">
  // How many modals are mounted, so the shell can tell whether anything is
  // layered above it. Escape has to dismiss one layer at a time, and the
  // instance handler below cannot enforce that alone: it and the shell's
  // handler are both listeners on `window`, where stopPropagation has no
  // effect between siblings and the shell's is registered first anyway.
  const mounted = $state({ count: 0 });

  export function anyModalOpen(): boolean {
    return mounted.count > 0;
  }
</script>

<script lang="ts">
  import { onMount } from 'svelte';
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';

  interface Props {
    title: string;
    subtitle?: string;
    width?: number;
    onclose: () => void;
    /** Clicking the backdrop / pressing Escape closes. Off for blocking flows. */
    dismissable?: boolean;
    children: Snippet;
    footer?: Snippet;
  }

  let {
    title,
    subtitle,
    width = 460,
    onclose,
    dismissable = true,
    children,
    footer
  }: Props = $props();

  let panel = $state<HTMLDivElement | null>(null);

  // onMount rather than $effect: incrementing reads the count, which inside an
  // effect would make it its own dependency.
  onMount(() => {
    mounted.count += 1;
    return () => {
      mounted.count -= 1;
    };
  });

  $effect(() => {
    if (!panel) return;
    const target =
      panel.querySelector<HTMLElement>('[data-autofocus]') ??
      panel.querySelector<HTMLElement>('input, select, textarea, button');
    target?.focus();
  });

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && dismissable) {
      e.stopPropagation();
      onclose();
      return;
    }
    if (e.key !== 'Tab' || !panel) return;
    const focusables = [
      ...panel.querySelectorAll<HTMLElement>(
        'a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'
      )
    ];
    if (focusables.length === 0) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    const active = document.activeElement as HTMLElement | null;
    if (e.shiftKey && (active === first || !panel.contains(active))) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && active === last) {
      e.preventDefault();
      first.focus();
    }
  }
</script>

<svelte:window {onkeydown} />

<div
  class="backdrop"
  role="presentation"
  onclick={(e) => {
    if (dismissable && e.target === e.currentTarget) onclose();
  }}
>
  <div
    class="panel"
    style:max-width="{width}px"
    role="dialog"
    aria-modal="true"
    aria-label={title}
    bind:this={panel}
  >
    <header>
      <div class="titles">
        <h2>{title}</h2>
        {#if subtitle}<p class="sub">{subtitle}</p>{/if}
      </div>
      {#if dismissable}
        <button class="btn btn-ghost btn-icon" onclick={onclose} aria-label="Close" tabindex="-1">
          <Icon name="x" size={16} />
        </button>
      {/if}
    </header>

    <div class="body">
      {@render children()}
    </div>

    {#if footer}
      <footer>{@render footer()}</footer>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 200;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 32px;
    background: var(--bg-overlay);
    backdrop-filter: blur(2px);
    animation: fade 120ms ease;
  }

  .panel {
    width: 100%;
    max-height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--bg-panel);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-lg);
    animation: pop 140ms cubic-bezier(0.2, 0.9, 0.3, 1);
    overflow: hidden;
  }

  /* Narrow windows: the 32px gutter is most of the dialog's width. */
  @media (max-width: 600px) {
    .backdrop {
      padding: 14px;
    }
  }

  header {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 16px 16px 12px 18px;
    border-bottom: 1px solid var(--border-subtle);
  }

  .titles {
    flex: 1;
    min-width: 0;
  }

  h2 {
    font-size: 15px;
  }

  .sub {
    margin-top: 3px;
    font-size: 12px;
    color: var(--text-muted);
  }

  .body {
    padding: 16px 18px;
    overflow-y: auto;
    font-size: 13px;
    line-height: 1.55;
  }

  footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 16px;
    background: var(--bg-sunken);
    border-top: 1px solid var(--border-subtle);
  }

  @keyframes fade {
    from {
      opacity: 0;
    }
  }
  @keyframes pop {
    from {
      opacity: 0;
      transform: translateY(8px) scale(0.985);
    }
  }
</style>
