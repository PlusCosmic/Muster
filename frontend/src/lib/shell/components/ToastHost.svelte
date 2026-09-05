<script lang="ts">
  import { fly } from 'svelte/transition';
  import { toasts } from '$lib/shell/stores/toasts.svelte';
  import Icon, { type IconName } from '$lib/shell/components/Icon.svelte';

  const ICONS: Record<string, IconName> = {
    error: 'alert',
    success: 'check',
    info: 'info'
  };
</script>

<div class="stack" aria-live="polite">
  {#each toasts.items as toast (toast.id)}
    <div class="toast {toast.kind}" transition:fly={{ y: 12, duration: 160 }}>
      <span class="glyph"><Icon name={ICONS[toast.kind] ?? 'info'} size={15} /></span>
      <div class="text">
        <div class="msg">{toast.message}</div>
        {#if toast.detail}<div class="detail" data-selectable>{toast.detail}</div>{/if}
      </div>
      <button class="close" onclick={() => toasts.dismiss(toast.id)} aria-label="Dismiss">
        <Icon name="x" size={13} />
      </button>
    </div>
  {/each}
</div>

<style>
  .stack {
    position: fixed;
    right: 16px;
    bottom: 16px;
    z-index: 400;
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: min(400px, calc(100vw - 32px));
    pointer-events: none;
  }

  .toast {
    pointer-events: auto;
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 10px 10px 10px 12px;
    background: var(--bg-raised);
    border: 1px solid var(--border-strong);
    border-left-width: 3px;
    border-radius: var(--r-md);
    box-shadow: var(--shadow-md);
  }

  .toast.error {
    border-left-color: var(--danger);
  }
  .toast.success {
    border-left-color: var(--ok);
  }
  .toast.info {
    border-left-color: var(--info);
  }

  .glyph {
    margin-top: 1px;
  }
  .error .glyph {
    color: var(--danger);
  }
  .success .glyph {
    color: var(--ok);
  }
  .info .glyph {
    color: var(--info);
  }

  .text {
    flex: 1;
    min-width: 0;
  }

  .msg {
    font-size: 13px;
    font-weight: 600;
  }

  .detail {
    margin-top: 2px;
    font-size: 12px;
    color: var(--text-muted);
    word-break: break-word;
    max-height: 6.6em;
    overflow-y: auto;
  }

  .close {
    flex: none;
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    padding: 0;
    color: var(--text-faint);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    cursor: pointer;
    transition: color var(--t-fast), background var(--t-fast);
  }
  .close:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
</style>
