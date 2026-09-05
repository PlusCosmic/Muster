// Typed wrappers over the generated Wails bindings for the Minecraft service —
// the only place the Minecraft module calls its backend. Command names and
// shapes are bound by docs/ARCHITECTURE.md.
import { Events } from '@wailsio/runtime';
import * as Svc from '$bindings/muster/internal/minecraft/service';
import type { Detected, Pack, PackCheck, Settings, SyncProgress, SyncReport } from './types';

export const SYNC_EVENT = 'minecraft:sync';

export const getSettings = (): Promise<Settings> => Svc.GetSettings();
export const updateSettings = (settings: Settings): Promise<Settings> => Svc.UpdateSettings(settings);
export const detect = (): Promise<Detected> => Svc.Detect();
export const listPacks = (): Promise<Pack[]> => Svc.ListPacks().then((p) => (p ?? []) as Pack[]);
export const checkPack = (id: string): Promise<PackCheck> => Svc.CheckPack(id);
export const syncPack = (id: string): Promise<SyncReport> => Svc.SyncPack(id) as Promise<SyncReport>;
export const openLauncher = (): Promise<void> => Svc.OpenLauncher();
export const launcherRunning = (): Promise<boolean> => Svc.LauncherRunning();

/** Subscribe to sync progress. Returns an unsubscribe function. */
export const onSyncProgress = (cb: (p: SyncProgress) => void): (() => void) =>
  Events.On(SYNC_EVENT, (e: { data: unknown }) => cb(e.data as SyncProgress));
