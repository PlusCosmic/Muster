// Single choke point for the Minecraft module's backend calls; `?mock=1` in
// `vite dev` swaps in ./mockData (see $lib/shell/mock and $lib/rimworld/backend
// for the reasoning, which is identical).
import { MOCK_ENABLED } from '$lib/shell/mock';
import * as api from './api';

const impl: Promise<typeof api> = MOCK_ENABLED
  ? import('./mockData').then((m) => m.mockApi as unknown as typeof api)
  : Promise.resolve(api);

const wrap = <K extends keyof typeof api>(name: K): (typeof api)[K] =>
  ((...args: unknown[]) =>
    impl.then((b) => (b[name] as (...a: unknown[]) => unknown)(...args))) as (typeof api)[K];

export const getSettings = wrap('getSettings');
export const updateSettings = wrap('updateSettings');
export const detect = wrap('detect');
export const listPacks = wrap('listPacks');
export const checkPack = wrap('checkPack');
export const syncPack = wrap('syncPack');
export const openLauncher = wrap('openLauncher');

/** Subscription: the unsubscribe resolves once the implementation is loaded. */
export function onSyncProgress(cb: (p: import('./types').SyncProgress) => void): () => void {
  let off: (() => void) | null = null;
  let cancelled = false;
  impl.then((b) => {
    if (cancelled) return;
    off = b.onSyncProgress(cb);
  });
  return () => {
    cancelled = true;
    off?.();
  };
}
