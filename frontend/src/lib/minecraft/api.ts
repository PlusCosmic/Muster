// Typed wrappers over the generated Wails bindings for the Minecraft service —
// the only place the Minecraft module calls its backend. Command names and
// shapes are bound by docs/ARCHITECTURE.md.
import { Events } from '@wailsio/runtime';
import * as Svc from '$bindings/muster/internal/minecraft/service';
import type {
  Detected,
  LaunchSettings,
  ModrinthLookup,
  ModrinthVersion,
  Pack,
  PackCheck,
  Settings,
  SyncProgress,
  SyncReport
} from './types';

export const SYNC_EVENT = 'minecraft:sync';

export const getSettings = (): Promise<Settings> => Svc.GetSettings() as Promise<Settings>;
export const updateSettings = (settings: Settings): Promise<Settings> =>
  Svc.UpdateSettings(settings) as Promise<Settings>;
export const detect = (): Promise<Detected> => Svc.Detect();
export const listPacks = (): Promise<Pack[]> => Svc.ListPacks().then((p) => (p ?? []) as Pack[]);
export const checkPack = (id: string): Promise<PackCheck> => Svc.CheckPack(id);
export const syncPack = (id: string): Promise<SyncReport> => Svc.SyncPack(id) as Promise<SyncReport>;
export const openLauncher = (): Promise<void> => Svc.OpenLauncher();
export const addPackCode = (input: string): Promise<Pack> => Svc.AddPackCode(input) as Promise<Pack>;
export const removePackCode = (code: string): Promise<void> => Svc.RemovePackCode(code);
export const lookupModrinth = (input: string): Promise<ModrinthLookup> => Svc.LookupModrinth(input) as Promise<ModrinthLookup>;
export const addModrinthPack = (input: string, version: string): Promise<Pack> =>
  Svc.AddModrinthPack(input, version) as Promise<Pack>;
export const removeModrinthPack = (id: string): Promise<void> => Svc.RemoveModrinthPack(id);
export const listModrinthVersions = (id: string): Promise<ModrinthVersion[]> =>
  Svc.ListModrinthVersions(id).then((v) => (v ?? []) as ModrinthVersion[]);
export const setModrinthVersion = (id: string, version: string): Promise<Pack> =>
  Svc.SetModrinthVersion(id, version) as Promise<Pack>;
export const getLaunchSettings = (id: string): Promise<LaunchSettings> =>
  Svc.GetLaunchSettings(id) as Promise<LaunchSettings>;
export const setLaunchSettings = (id: string, ls: LaunchSettings): Promise<LaunchSettings> =>
  Svc.SetLaunchSettings(id, ls) as Promise<LaunchSettings>;
export const resetLaunchSettings = (id: string): Promise<LaunchSettings> =>
  Svc.ResetLaunchSettings(id) as Promise<LaunchSettings>;
export const launcherRunning = (): Promise<boolean> => Svc.LauncherRunning();

/** Subscribe to sync progress. Returns an unsubscribe function. */
export const onSyncProgress = (cb: (p: SyncProgress) => void): (() => void) =>
  Events.On(SYNC_EVENT, (e: { data: unknown }) => cb(e.data as SyncProgress));
