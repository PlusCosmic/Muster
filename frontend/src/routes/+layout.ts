// Wails serves a static bundle with no Node.js server, so the app runs as an
// SPA (adapter-static with an index.html fallback) and never server-renders.
// See: https://svelte.dev/docs/kit/single-page-apps
export const ssr = false;

import { redirect } from '@sveltejs/kit';
import { GAMES, lastGame } from '$lib/shell/games';
import { modules } from '$lib/shell/stores/modules.svelte';

// Which games are on decides both the rail and where `/` goes, so it is
// known before anything renders. One local IPC call; nothing to wait on
// after the first navigation. Reading `url` makes this rerun on every
// navigation, which is what the guard below needs: a game route is only
// reachable while that game is on (Back after switching it off, a stale
// bookmark in the dev browser), and none before the welcome screen.
export async function load({ url }: { url: URL }) {
  await modules.load();
  const game = GAMES.find((g) => url.pathname.startsWith(g.path));
  if (!game) return;
  if (!modules.onboarded) redirect(307, '/welcome');
  if (!modules.enabled.includes(game.id)) redirect(307, lastGame(modules.games).path);
}
