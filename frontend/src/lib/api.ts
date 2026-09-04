// Typed wrappers over the generated Wails bindings — the only place the
// frontend calls the backend. Command names and shapes are bound by
// docs/ARCHITECTURE.md; the bindings themselves come from
// `wails3 generate bindings` and live in frontend/bindings.
//
// The generated signatures type every list as `T[] | null` (a nil Go slice).
// The backend guarantees non-null lists (models.NonNil), so results are
// narrowed to the app's types here rather than null-checked everywhere.
import * as App from '$bindings/rimforge/app';
import type {
  ActiveModList,
  DetectedPaths,
  ModInfo,
  Profile,
  RulesDbStatus,
  Settings,
  SortResult
} from './types';

export const revealPath = (path: string): Promise<void> => App.RevealPath(path);

export const getSettings = (): Promise<Settings> => App.GetSettings();
export const updateSettings = (settings: Settings): Promise<Settings> =>
  App.UpdateSettings(settings);

export const detectPaths = (): Promise<DetectedPaths> =>
  App.DetectPaths() as Promise<DetectedPaths>;

export const listProfiles = (): Promise<Profile[]> => App.ListProfiles().then((p) => p ?? []);
export const createProfile = (name: string): Promise<Profile> => App.CreateProfile(name);
export const renameProfile = (id: string, newName: string): Promise<Profile> =>
  App.RenameProfile(id, newName);
export const deleteProfile = (id: string): Promise<void> => App.DeleteProfile(id);
export const cloneProfile = (id: string, newName: string): Promise<Profile> =>
  App.CloneProfile(id, newName);
export const importDefault = (name: string): Promise<Profile> => App.ImportDefault(name);

export const launchProfile = (id: string): Promise<void> => App.LaunchProfile(id);

export const listInstalledMods = (): Promise<ModInfo[]> =>
  App.ListInstalledMods().then((m) => (m ?? []) as ModInfo[]);
export const getActiveMods = (profileId: string): Promise<ActiveModList> =>
  App.GetActiveMods(profileId) as Promise<ActiveModList>;
export const setActiveMods = (profileId: string, activeIds: string[]): Promise<void> =>
  App.SetActiveMods(profileId, activeIds);
export const sortMods = (activeIds: string[]): Promise<SortResult> =>
  App.SortMods(activeIds) as Promise<SortResult>;

export const refreshRulesDb = (): Promise<RulesDbStatus> => App.RefreshRulesDb();
export const getRulesDbStatus = (): Promise<RulesDbStatus> => App.GetRulesDbStatus();
