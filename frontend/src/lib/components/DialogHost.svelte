<script lang="ts">
  import { dialogs } from '$lib/stores/dialogs.svelte';
  import Modal from './Modal.svelte';

  let promptValue = $state('');

  // Reset the field whenever a new prompt opens.
  let promptToken = $derived(dialogs.promptSpec);
  $effect(() => {
    promptValue = promptToken?.initial ?? '';
  });

  const promptValid = $derived(promptValue.trim().length > 0);

  function submitPrompt(e: Event) {
    e.preventDefault();
    if (!promptValid) return;
    dialogs.settlePrompt(promptValue.trim());
  }
</script>

{#if dialogs.confirmSpec}
  {@const spec = dialogs.confirmSpec}
  <Modal title={spec.title} width={430} onclose={() => dialogs.settleConfirm(false)}>
    <p>{spec.body}</p>
    {#if spec.note}
      <p class="note" class:danger={spec.danger}>{spec.note}</p>
    {/if}

    {#snippet footer()}
      <button class="btn" onclick={() => dialogs.settleConfirm(false)}>
        {spec.cancelLabel ?? 'Cancel'}
      </button>
      <button
        class="btn {spec.danger ? 'btn-danger' : 'btn-primary'}"
        data-autofocus
        onclick={() => dialogs.settleConfirm(true)}
      >
        {spec.confirmLabel ?? 'Confirm'}
      </button>
    {/snippet}
  </Modal>
{/if}

{#if dialogs.promptSpec}
  {@const spec = dialogs.promptSpec}
  <Modal title={spec.title} width={450} onclose={() => dialogs.settlePrompt(null)}>
    {#if spec.body}<p class="body-text">{spec.body}</p>{/if}
    <form onsubmit={submitPrompt}>
      <label class="label" for="rf-prompt-input">{spec.label}</label>
      <input
        id="rf-prompt-input"
        class="input"
        data-autofocus
        placeholder={spec.placeholder ?? ''}
        bind:value={promptValue}
        autocomplete="off"
        spellcheck="false"
      />
      {#if spec.note}<p class="hint">{spec.note}</p>{/if}
    </form>

    {#snippet footer()}
      <button class="btn" onclick={() => dialogs.settlePrompt(null)}>Cancel</button>
      <button class="btn btn-primary" disabled={!promptValid} onclick={submitPrompt}>
        {spec.confirmLabel ?? 'Create'}
      </button>
    {/snippet}
  </Modal>
{/if}

<style>
  .note {
    margin-top: 10px;
    padding: 9px 11px;
    font-size: 12.5px;
    color: var(--text-muted);
    background: var(--bg-sunken);
    border: 1px solid var(--border);
    border-left: 3px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .note.danger {
    color: #f2b3b1;
    background: var(--danger-soft);
    border-color: rgba(224, 82, 79, 0.35);
    border-left-color: var(--danger);
  }
  .body-text {
    margin-bottom: 14px;
    color: var(--text-muted);
  }
</style>
