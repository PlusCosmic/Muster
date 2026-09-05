// Registry of "do I have unsaved work?" checks, so the window-close guard can
// live in the persistent shell layout rather than in a game route. A route
// unmounts when the user switches games, but its store (and any draft in it)
// does not, so the guard must outlive the route. Stores register once, at
// module load.

const checks: Array<() => boolean> = [];

export function registerUnsavedCheck(check: () => boolean): void {
  checks.push(check);
}

/** True when any registered module has unsaved work. */
export function anyUnsaved(): boolean {
  return checks.some((c) => c());
}
