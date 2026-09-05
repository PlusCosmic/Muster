<script lang="ts">
  // The narrow strip down the left edge that switches between game modules.
  // Always docked, whatever the window width: each game's own sidebar handles
  // its narrow-window behaviour to the right of this.
  import { page } from '$app/state';
  import { GAMES, rememberGame } from '../games';
  import BrandMark from './BrandMark.svelte';
  import Icon from './Icon.svelte';

  const activeId = $derived(GAMES.find((g) => page.url.pathname.startsWith(g.path))?.id);
</script>

<nav class="rail" aria-label="Games">
  <div class="brand" title="Muster">
    <BrandMark size={30} />
  </div>
  <ul>
    {#each GAMES as game (game.id)}
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
  </ul>
</nav>

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

  a {
    position: relative;
    display: grid;
    place-items: center;
    width: 38px;
    height: 38px;
    color: var(--text-faint);
    border-radius: var(--r-md);
    transition:
      background var(--t-fast),
      color var(--t-fast);
  }
  a:hover {
    color: var(--text);
    background: var(--bg-hover);
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
  a:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
