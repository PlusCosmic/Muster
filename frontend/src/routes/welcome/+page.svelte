<script lang="ts">
  // First run: say what Muster is and ask which games to show. Also reachable
  // later, when it simply edits the same choice.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { getAppInfo } from '$lib/shell/api';
  import BrandMark from '$lib/shell/components/BrandMark.svelte';
  import GamePicker from '$lib/shell/components/GamePicker.svelte';
  import Icon from '$lib/shell/components/Icon.svelte';
  import { isGameId, lastGame, type GameId } from '$lib/shell/games';
  import { MOCK_ENABLED } from '$lib/shell/mock';
  import { modules } from '$lib/shell/stores/modules.svelte';

  let selected = $state<GameId[]>(modules.enabled);
  let saving = $state(false);

  onMount(async () => {
    // Coming back to this screen keeps the current choice. A first run
    // starts from whatever already has data on disk: an upgrade from a build
    // that showed every game must not hide one that was in use.
    if (modules.onboarded || MOCK_ENABLED) return;
    try {
      const info = await getAppInfo();
      if (selected.length === 0) selected = info.gamesWithData.filter(isGameId);
    } catch {
      // Nothing preselected, then; the user picks.
    }
  });

  async function finish() {
    if (selected.length === 0 || saving) return;
    saving = true;
    const ok = await modules.set(selected);
    saving = false;
    if (ok) await goto(lastGame(modules.games).path);
  }
</script>

<svelte:head>
  <title>Muster · Welcome</title>
</svelte:head>

<div class="welcome">
  <div class="card">
    <div class="brand"><BrandMark size={56} /></div>
    <h1>Welcome to Muster</h1>
    <p class="lede">
      Muster keeps a group's mod setups in step: one shell, one module per game. Pick the games
      you play. You can add or remove games any time from the rail on the left.
    </p>

    <GamePicker {selected} onchange={(ids) => (selected = ids)} />

    <div class="actions">
      <span class="hint" aria-live="polite">
        {selected.length === 0 ? 'Pick at least one game to continue.' : ''}
      </span>
      <button class="btn btn-primary" disabled={selected.length === 0 || saving} onclick={finish}>
        {saving ? 'Saving…' : 'Continue'}
        {#if !saving}<Icon name="chevronRight" size={14} />{/if}
      </button>
    </div>
  </div>
</div>

<style>
  .welcome {
    display: grid;
    place-items: center;
    height: 100%;
    padding: 32px;
    overflow-y: auto;
    background:
      radial-gradient(70% 50% at 50% 0%, var(--accent-soft), transparent 70%),
      var(--bg-app);
  }

  .card {
    width: 100%;
    max-width: 540px;
    text-align: center;
  }

  .brand {
    display: grid;
    place-items: center;
    margin: 0 auto 18px;
    filter: drop-shadow(0 8px 26px var(--accent-soft));
  }

  h1 {
    margin: 0;
    font-size: 24px;
    letter-spacing: -0.02em;
  }

  .lede {
    margin: 10px auto 26px;
    max-width: 440px;
    font-size: 13px;
    line-height: 1.6;
    color: var(--text-muted);
  }

  .actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-top: 22px;
  }
  .hint {
    font-size: 12px;
    color: var(--text-faint);
  }
</style>
