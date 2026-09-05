<script lang="ts">
  // Version and self-update controls, shared by every game's settings modal.
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { checkForUpdates, getAppInfo, type AppInfo } from '$lib/shell/api';
  import { MOCK_ENABLED } from '$lib/shell/mock';
  import { toastError } from '$lib/shell/stores/toasts.svelte';

  let info = $state<AppInfo | null>(null);
  let checking = $state(false);

  onMount(async () => {
    if (MOCK_ENABLED) {
      info = { version: 'dev', dataRoot: '/home/you/.local/share/muster', selfUpdates: true };
      return;
    }
    try {
      info = await getAppInfo();
    } catch (e) {
      toastError('Could not read the app version', e);
    }
  });

  async function check() {
    checking = true;
    try {
      if (!MOCK_ENABLED) await checkForUpdates();
    } catch (e) {
      toastError('Could not check for updates', e);
    } finally {
      // The update window takes over from here; re-enable after a moment.
      setTimeout(() => (checking = false), 1500);
    }
  }
</script>

<div class="about">
  <dl class="facts">
    <dt>Version</dt>
    <dd class="mono">{info?.version ?? '—'}</dd>
    <dt>Data folder</dt>
    <dd class="mono" data-selectable>{info?.dataRoot ?? '—'}</dd>
  </dl>
  {#if info?.selfUpdates}
    <div class="row">
      <span class="hint">Muster checks for new versions on its own; you can also check now.</span>
      <button class="btn" disabled={checking} onclick={check}>
        <Icon name="refresh" size={14} class={checking ? 'spin' : ''} /> Check for updates
      </button>
    </div>
  {:else if info}
    <p class="hint">This build is updated by your package manager.</p>
  {/if}
</div>

<style>
  .about {
    margin-top: 12px;
  }
  .facts {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 6px 16px;
    margin: 0;
    font-size: 12px;
  }
  .facts dt {
    color: var(--text-faint);
  }
  .facts dd {
    margin: 0;
    word-break: break-all;
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    margin-top: 12px;
  }
  .hint {
    margin-top: 8px;
  }
</style>
