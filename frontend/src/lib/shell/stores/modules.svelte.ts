// Which game modules are switched on. Persisted in the backend's own
// settings.json (not localStorage: it is app data, and it must survive the
// webview's storage being reset). An empty list means the user has not been
// through the welcome screen yet; the root route sends them there.
import { GAMES, isGameId, type Game, type GameId } from '../games';
import { getSettings, updateSettings } from '../api';
import { MOCK_ENABLED } from '../mock';
import { readPref, writePref } from '../storage';
import { toastError } from './toasts.svelte';

// In mock mode the choice lives in localStorage so the welcome flow can be
// exercised in a plain browser (clear `muster-mock-games` to see it again).
const MOCK_KEY = 'mock-games';

function onlyKnown(ids: string[]): GameId[] {
  // Rail order is the catalogue's, whatever order the ids arrived in.
  return GAMES.map((g) => g.id).filter((id) => ids.includes(id));
}

class ModulesStore {
  /** Ids of the games that are on, in rail order. */
  enabled = $state<GameId[]>([]);
  /** Has the backend been asked? The layout waits on this before routing. */
  loaded = $state(false);

  /** The enabled games, in rail order. */
  get games(): Game[] {
    return GAMES.filter((g) => this.enabled.includes(g.id));
  }

  /** Has the user picked at least one game (been through the welcome screen)? */
  get onboarded(): boolean {
    return this.enabled.length > 0;
  }

  async load(): Promise<void> {
    if (this.loaded) return;
    try {
      if (MOCK_ENABLED) {
        const raw = readPref(MOCK_KEY);
        this.enabled = onlyKnown(raw ? raw.split(',') : []);
      } else {
        this.enabled = onlyKnown((await getSettings()).games);
      }
    } catch (e) {
      // Without the setting the app is unusable, so fall back to every game
      // rather than to the welcome screen (which could not save either).
      toastError('Could not read which games are on', e);
      this.enabled = GAMES.map((g) => g.id);
    }
    this.loaded = true;
  }

  /** Persist a new selection. Resolves to whether it stuck. */
  async set(ids: string[]): Promise<boolean> {
    const wanted = onlyKnown(ids.filter(isGameId));
    if (wanted.length === 0) return false;
    try {
      if (MOCK_ENABLED) {
        writePref(MOCK_KEY, wanted.join(','));
        this.enabled = wanted;
      } else {
        this.enabled = onlyKnown((await updateSettings({ games: wanted })).games);
      }
      return true;
    } catch (e) {
      toastError('Could not save your games', e);
      return false;
    }
  }
}

export const modules = new ModulesStore();
