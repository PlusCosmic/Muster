// The game modules the shell can switch between. Each has a route, and each
// route owns its own sidebar, main pane and settings; the shell only provides
// the rail that moves between them. This is the catalogue of every module the
// build ships; which of them the user has switched on is the `modules` store.
import type { IconName } from './components/Icon.svelte';
import { readPref, writePref } from './storage';

export type GameId = 'rimworld' | 'minecraft';

export interface Game {
  id: GameId;
  name: string;
  path: string;
  icon: IconName;
  /** One line for the welcome screen and the games dialog. */
  blurb: string;
}

export const GAMES: Game[] = [
  {
    id: 'rimworld',
    name: 'RimWorld',
    path: '/rimworld',
    icon: 'rimworld',
    blurb:
      'Profiles: separate mod lists, mod settings and saves that share one install and Workshop library, with RimSort-style auto-sort.'
  },
  {
    id: 'minecraft',
    name: 'Minecraft',
    path: '/minecraft',
    icon: 'minecraft',
    blurb:
      'Shared packs: enter the code you were given and Muster syncs the pack and adds it to the official launcher.'
  }
];

export function isGameId(id: string): id is GameId {
  return GAMES.some((g) => g.id === id);
}

const KEY = 'game';

/**
 * The game to open on launch, out of `enabled`: the last one used when it is
 * still on, else the first. `enabled` must not be empty.
 */
export function lastGame(enabled: Game[]): Game {
  const id = readPref(KEY);
  return enabled.find((g) => g.id === id) ?? enabled[0];
}

export function rememberGame(id: GameId): void {
  writePref(KEY, id);
}
