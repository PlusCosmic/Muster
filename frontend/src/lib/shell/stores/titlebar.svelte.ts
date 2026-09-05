// Native window title bar on/off. Like the theme, this is a per-machine
// frontend preference (localStorage), applied by asking the Wails window to
// go frameless — useful under tiling WMs where the bar is dead space.

import { Window } from '@wailsio/runtime';
import { MOCK_ENABLED } from '$lib/shell/mock';

import { readPref, writePref } from '../storage';

const STORAGE_KEY = 'titlebar';

function load(): boolean {
  return readPref(STORAGE_KEY) !== 'hidden';
}

function stamp(shown: boolean) {
  // No-op outside the Wails window (vite in a browser, mock mode).
  if (typeof window === 'undefined' || MOCK_ENABLED) return;
  Window.SetFrameless(!shown).catch((e: unknown) => console.error('SetFrameless failed:', e));
}

class TitlebarStore {
  /** The persisted choice. Previews stamp the window without touching this. */
  current = $state<boolean>(load());

  /** Persist and apply. The settings modal's Save path. */
  set(shown: boolean) {
    this.current = shown;
    writePref(STORAGE_KEY, shown ? 'shown' : 'hidden');
    stamp(shown);
  }

  /** Live preview: apply without persisting. Cancelled by apply(). */
  preview(shown: boolean) {
    stamp(shown);
  }

  /** Apply the persisted state — boot, and reverting an unsaved preview. */
  apply() {
    stamp(this.current);
  }
}

export const titlebar = new TitlebarStore();
