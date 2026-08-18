// UI theme selection. Purely a frontend preference: persisted in the webview's
// localStorage (per machine), not in the backend settings.json, and applied by
// stamping `data-theme` on <html> so the token blocks in app.css take over.

export interface Theme {
  id: string;
  name: string;
  description: string;
}

export const THEMES: Theme[] = [
  { id: 'rust', name: 'Rust', description: 'Warm dark with the rusty orange accent (default)' },
  { id: 'slate', name: 'Slate', description: 'Cool dark blue-grey with a steel accent' },
  { id: 'moss', name: 'Moss', description: 'Dark green with a herbal accent' },
  { id: 'parchment', name: 'Parchment', description: 'Light, warm paper-and-ink look' }
];

const STORAGE_KEY = 'rimforge-theme';
const DEFAULT_ID = 'rust';

function load(): string {
  if (typeof localStorage === 'undefined') return DEFAULT_ID;
  const stored = localStorage.getItem(STORAGE_KEY);
  return THEMES.some((t) => t.id === stored) ? (stored as string) : DEFAULT_ID;
}

class ThemeStore {
  current = $state<string>(load());

  set(id: string) {
    if (!THEMES.some((t) => t.id === id)) return;
    this.current = id;
    if (typeof localStorage !== 'undefined') localStorage.setItem(STORAGE_KEY, id);
    this.apply();
  }

  /** Stamp the current theme onto <html>. Called once at boot and on change. */
  apply() {
    if (typeof document === 'undefined') return;
    document.documentElement.dataset.theme = this.current;
  }
}

export const theme = new ThemeStore();
