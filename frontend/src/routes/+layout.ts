// Wails serves a static bundle with no Node.js server, so the app runs as an
// SPA (adapter-static with an index.html fallback) and never server-renders.
// See: https://svelte.dev/docs/kit/single-page-apps
export const ssr = false;

import { modules } from '$lib/shell/stores/modules.svelte';

// Which games are on decides both the rail and where `/` goes, so it is
// known before anything renders. One local IPC call; nothing to wait on
// after the first navigation.
export async function load() {
  await modules.load();
}
