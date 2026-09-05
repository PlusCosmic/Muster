// localStorage helpers for per-machine UI preferences (theme, sidebar,
// title bar). Keys are prefixed `muster-`. The webview keys its storage by
// program name, so RimForge's `rimforge-*` values are in a store this app
// cannot see; those three cosmetic preferences reset once on upgrade.
const PREFIX = 'muster-';

export function readPref(key: string): string | null {
  if (typeof localStorage === 'undefined') return null;
  try {
    return localStorage.getItem(PREFIX + key);
  } catch {
    return null;
  }
}

export function writePref(key: string, value: string): void {
  if (typeof localStorage === 'undefined') return;
  try {
    localStorage.setItem(PREFIX + key, value);
  } catch {
    // Storage unavailable (private mode, quota): the preference just won't stick.
  }
}
