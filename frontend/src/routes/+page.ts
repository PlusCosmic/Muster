// The root route only decides which game to show: the one used last.
import { redirect } from '@sveltejs/kit';
import { lastGame } from '$lib/shell/games';

export function load() {
  redirect(307, lastGame().path);
}
