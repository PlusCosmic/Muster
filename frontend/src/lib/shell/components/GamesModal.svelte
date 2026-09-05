<script lang="ts">
  // Add or remove games from the rail: the same choice as the welcome screen,
  // editable later.
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { GAMES, lastGame, type GameId } from '../games';
  import { modules } from '../stores/modules.svelte';
  import GamePicker from './GamePicker.svelte';
  import Modal from './Modal.svelte';

  let { onclose }: { onclose: () => void } = $props();

  let draft = $state<GameId[]>(modules.enabled);
  let saving = $state(false);

  const changed = $derived(draft.join(',') !== modules.enabled.join(','));

  async function save() {
    saving = true;
    const ok = await modules.set(draft);
    saving = false;
    if (!ok) return;
    onclose();
    // The game on screen may just have been switched off.
    const here = GAMES.find((g) => page.url.pathname.startsWith(g.path));
    if (here && !modules.enabled.includes(here.id)) await goto(lastGame(modules.games).path);
  }
</script>

<Modal title="Games" subtitle="Pick the games Muster shows in the rail." width={520} {onclose}>
  <GamePicker selected={draft} onchange={(ids) => (draft = ids)} />
  {#if draft.length === 0}
    <p class="hint">Keep at least one game on.</p>
  {/if}

  {#snippet footer()}
    <button class="btn" onclick={onclose}>Cancel</button>
    <button class="btn btn-primary" disabled={!changed || draft.length === 0 || saving} onclick={save}>
      {saving ? 'Saving…' : 'Save'}
    </button>
  {/snippet}
</Modal>

<style>
  .hint {
    margin: 12px 0 0;
    font-size: 12px;
    color: var(--text-faint);
  }
</style>
