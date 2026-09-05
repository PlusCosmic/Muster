// The root route only decides where to start: the welcome screen until the
// user has picked their games, then the game used last.
import { redirect } from '@sveltejs/kit';
import { lastGame } from '$lib/shell/games';
import { modules } from '$lib/shell/stores/modules.svelte';

export async function load({ parent }: { parent: () => Promise<unknown> }) {
  await parent();
  redirect(307, modules.onboarded ? lastGame(modules.games).path : '/welcome');
}
