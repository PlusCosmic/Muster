<script lang="ts">
  // A list of every game module with a tick each: the welcome screen and the
  // games dialog both pick from it. Toggling never persists; the parent does.
  import { GAMES, type GameId } from '../games';
  import Icon from './Icon.svelte';

  interface Props {
    selected: GameId[];
    onchange: (selected: GameId[]) => void;
  }
  let { selected, onchange }: Props = $props();

  function toggle(id: GameId) {
    onchange(
      selected.includes(id)
        ? selected.filter((s) => s !== id)
        : GAMES.map((g) => g.id).filter((g) => g === id || selected.includes(g))
    );
  }
</script>

<div class="picker" role="group" aria-label="Games">
  {#each GAMES as game (game.id)}
    {@const on = selected.includes(game.id)}
    <button
      type="button"
      class="choice"
      class:on
      role="checkbox"
      aria-checked={on}
      onclick={() => toggle(game.id)}
    >
      <span class="choice-icon"><Icon name={game.icon} size={20} strokeWidth={1.6} /></span>
      <span class="choice-text">
        <strong>{game.name}</strong>
        <span>{game.blurb}</span>
      </span>
      <span class="tick" aria-hidden="true">
        {#if on}<Icon name="check" size={14} strokeWidth={2.4} />{/if}
      </span>
    </button>
  {/each}
</div>

<style>
  .picker {
    display: grid;
    gap: 10px;
    text-align: left;
  }

  .choice {
    display: flex;
    align-items: flex-start;
    gap: 13px;
    width: 100%;
    padding: 14px 16px;
    font: inherit;
    text-align: left;
    color: var(--text);
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    cursor: pointer;
    transition:
      border-color var(--t-med),
      background var(--t-med),
      transform var(--t-fast);
  }
  .choice:hover {
    background: var(--bg-raised);
    border-color: var(--border-strong);
  }
  .choice:active {
    transform: translateY(1px);
  }
  .choice:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .choice.on {
    border-color: var(--accent-line);
    background: var(--accent-soft);
  }
  .choice.on:hover {
    border-color: var(--accent);
  }

  .choice-icon {
    display: grid;
    place-items: center;
    width: 36px;
    height: 36px;
    flex: none;
    color: var(--text-muted);
    background: var(--bg-sunken);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    transition: color var(--t-med);
  }
  .choice.on .choice-icon {
    color: var(--accent);
  }

  .choice-text {
    display: grid;
    gap: 3px;
    min-width: 0;
    flex: 1;
  }
  .choice-text strong {
    font-size: 13.5px;
    font-weight: 650;
  }
  .choice-text span {
    font-size: 12px;
    line-height: 1.5;
    color: var(--text-muted);
  }

  .tick {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    flex: none;
    margin-top: 8px;
    color: var(--text-on-accent);
    background: var(--bg-sunken);
    border: 1px solid var(--border-strong);
    border-radius: 50%;
    transition:
      background var(--t-med),
      border-color var(--t-med);
  }
  .choice.on .tick {
    background: var(--accent);
    border-color: var(--accent);
  }
</style>
