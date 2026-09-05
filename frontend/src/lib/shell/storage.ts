// localStorage helpers for per-machine UI preferences. Keys are prefixed
// `muster-`; a `rimforge-` key from the app's RimWorld-only predecessor is
// read as a fallback so an upgrade keeps the user's theme and layout.
const PREFIX = 'muster-';
const LEGACY_PREFIX = 'rimforge-';

export function readPref(key: string): string | null {
  if (typeof localStorage === 'undefined') return null;
  try {
    return localStorage.getItem(PREFIX + key) ?? localStorage.getItem(LEGACY_PREFIX + key);
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
