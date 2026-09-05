// The game modules the shell can switch between. Each has a route, and each
// route owns its own sidebar, main pane and settings; the shell only provides
// the rail that moves between them.
import type { IconName } from './components/Icon.svelte';
import { readPref, writePref } from './storage';

export interface Game {
  id: 'rimworld' | 'minecraft';
  name: string;
  path: string;
  icon: IconName;
}

export const GAMES: Game[] = [
  { id: 'rimworld', name: 'RimWorld', path: '/rimworld', icon: 'rimworld' },
  { id: 'minecraft', name: 'Minecraft', path: '/minecraft', icon: 'minecraft' }
];

const KEY = 'game';

/** The game to open on launch: the last one used, else the first. */
export function lastGame(): Game {
  const id = readPref(KEY);
  return GAMES.find((g) => g.id === id) ?? GAMES[0];
}

export function rememberGame(id: Game['id']): void {
  writePref(KEY, id);
}
