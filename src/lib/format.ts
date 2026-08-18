/** Small formatting helpers shared across the UI. */

const MIN = 60_000;
const HOUR = 60 * MIN;
const DAY = 24 * HOUR;

/** "3 hours ago", "just now", "Never". */
export function relativeTime(ms: number | null | undefined, never = 'Never played'): string {
  if (ms === null || ms === undefined) return never;
  const diff = Date.now() - ms;
  if (diff < 0) return 'just now';
  if (diff < MIN) return 'just now';
  if (diff < HOUR) return plural(Math.floor(diff / MIN), 'minute') + ' ago';
  if (diff < DAY) return plural(Math.floor(diff / HOUR), 'hour') + ' ago';
  if (diff < 7 * DAY) return plural(Math.floor(diff / DAY), 'day') + ' ago';
  if (diff < 31 * DAY) return plural(Math.floor(diff / (7 * DAY)), 'week') + ' ago';
  if (diff < 365 * DAY) return plural(Math.floor(diff / (30 * DAY)), 'month') + ' ago';
  return plural(Math.floor(diff / (365 * DAY)), 'year') + ' ago';
}

export function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? '' : 's'}`;
}

/** Absolute timestamp for tooltips / rules-db status. */
export function absoluteTime(ms: number | null | undefined): string {
  if (ms === null || ms === undefined) return '—';
  return new Date(ms).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
}

/** "1.6.4535 rev991" -> "1.6"; null-safe. */
export function majorMinor(version: string | null | undefined): string | null {
  if (!version) return null;
  const m = /(\d+)\.(\d+)/.exec(version);
  return m ? `${m[1]}.${m[2]}` : null;
}

/** Shorten a long absolute path for display, keeping the tail readable. */
export function shortenPath(path: string, max = 56): string {
  if (path.length <= max) return path;
  return '…' + path.slice(path.length - (max - 1));
}
