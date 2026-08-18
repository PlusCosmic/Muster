// TypeScript mirrors of src-tauri/src/models.rs — keep in lockstep with
// docs/ARCHITECTURE.md. All Rust snake_case fields arrive camelCased.

export interface Settings {
  steamRootOverride: string | null;
  gameInstallOverride: string | null;
  defaultSavedataOverride: string | null;
}

export interface DetectedPaths {
  steamRoot: string | null;
  gameInstall: string | null;
  defaultSavedata: string | null;
  workshopDirs: string[];
  gameVersion: string | null;
  profilesDir: string;
}

export interface Profile {
  id: string;
  name: string;
  path: string;
  createdAtMs: number;
  lastPlayedAtMs: number | null;
  saveCount: number;
  activeModCount: number;
}

export type ModSource = 'official' | 'local' | 'workshop';

export interface ModInfo {
  packageId: string;
  name: string;
  authors: string;
  path: string;
  source: ModSource;
  steamWorkshopId: string | null;
  supportedVersions: string[];
  dependencies: string[];
  loadAfter: string[];
  loadBefore: string[];
  forceLoadAfter: string[];
  forceLoadBefore: string[];
  incompatibleWith: string[];
}

export interface ActiveModList {
  activeIds: string[];
  knownExpansions: string[];
  version: string | null;
}

export interface SortWarning {
  kind: string;
  packageId: string | null;
  message: string;
}

export interface SortResult {
  sorted: string[];
  warnings: SortWarning[];
}

export interface RulesDbStatus {
  cached: boolean;
  fetchedAtMs: number | null;
  ruleCount: number;
}
