<script lang="ts">
  // The narrow strip down the left edge that switches between game modules.
  // Always docked, whatever the window width: each game's own sidebar handles
  // its narrow-window behaviour to the right of this. Shows only the games
  // the user has switched on; the button at the end of the list edits that.
  import { page } from '$app/state';
  import { rememberGame } from '../games';
  import { modules } from '../stores/modules.svelte';
  import BrandMark from './BrandMark.svelte';
  import GamesModal from './GamesModal.svelte';
  import Icon from './Icon.svelte';

  const activeId = $derived(modules.games.find((g) => page.url.pathname.startsWith(g.path))?.id);
  let gamesOpen = $state(false);
</script>

<nav class="rail" aria-label="Games">
  <div class="brand" title="Muster">
    <BrandMark size={30} />
  </div>
  <ul>
    {#each modules.games as game (game.id)}
      <li>
        <a
          href={game.path}
          class:active={game.id === activeId}
          title={game.name}
          aria-label={game.name}
          aria-current={game.id === activeId ? 'page' : undefined}
          onclick={() => rememberGame(game.id)}
        >
          <Icon name={game.icon} size={20} strokeWidth={1.6} />
        </a>
      </li>
    {/each}
    <li class="manage">
      <button type="button" title="Add or remove games" aria-label="Add or remove games" onclick={() => (gamesOpen = true)}>
        <Icon name="plus" size={18} strokeWidth={1.8} />
      </button>
    </li>
  </ul>
</nav>

{#if gamesOpen}
  <GamesModal onclose={() => (gamesOpen = false)} />
{/if}

<style>
  .rail {
    grid-area: rail;
    display: flex;
    flex-direction: column;
    align-items: center;
    width: var(--rail-w);
    padding: 12px 0;
    gap: 14px;
    background: var(--bg-sunken);
    border-right: 1px solid var(--border);
    z-index: 110;
  }

  .brand {
    display: grid;
    place-items: center;
    height: var(--header-h);
    margin-top: -12px;
  }

  ul {
    display: grid;
    gap: 6px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .manage {
    margin-top: 6px;
    padding-top: 12px;
    border-top: 1px solid var(--border);
  }

  a,
  button {
    position: relative;
    display: grid;
    place-items: center;
    width: 38px;
    height: 38px;
    padding: 0;
    color: var(--text-faint);
    background: none;
    border: none;
    border-radius: var(--r-md);
    cursor: pointer;
    transition:
      background var(--t-fast),
      color var(--t-fast);
  }
  a:hover,
  button:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
  button {
    border: 1px dashed var(--border-strong);
  }
  a.active {
    color: var(--accent);
    background: var(--bg-active);
  }
  a.active::before {
    content: '';
    position: absolute;
    left: calc(-1 * (var(--rail-w) - 38px) / 2);
    top: 9px;
    bottom: 9px;
    width: 3px;
    border-radius: 0 3px 3px 0;
    background: var(--accent);
  }
  a:focus-visible,
  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
