// Typed wrappers over Tauri invoke — the only place the frontend calls the
// backend. Command names and shapes are bound by docs/ARCHITECTURE.md.
import { invoke } from '@tauri-apps/api/core';
import type {
  ActiveModList,
  DetectedPaths,
  ModInfo,
  Profile,
  RulesDbStatus,
  Settings,
  SortResult
} from './types';

export const getSettings = () => invoke<Settings>('get_settings');
export const updateSettings = (settings: Settings) =>
  invoke<Settings>('update_settings', { settings });

export const detectPaths = () => invoke<DetectedPaths>('detect_paths');

export const listProfiles = () => invoke<Profile[]>('list_profiles');
export const createProfile = (name: string) => invoke<Profile>('create_profile', { name });
export const renameProfile = (id: string, newName: string) =>
  invoke<Profile>('rename_profile', { id, newName });
export const deleteProfile = (id: string) => invoke<void>('delete_profile', { id });
export const cloneProfile = (id: string, newName: string) =>
  invoke<Profile>('clone_profile', { id, newName });
export const importDefault = (name: string) => invoke<Profile>('import_default', { name });

export const launchProfile = (id: string) => invoke<void>('launch_profile', { id });
export const createShortcut = (id: string) => invoke<string>('create_shortcut', { id });

export const listInstalledMods = () => invoke<ModInfo[]>('list_installed_mods');
export const getActiveMods = (profileId: string) =>
  invoke<ActiveModList>('get_active_mods', { profileId });
export const setActiveMods = (profileId: string, activeIds: string[]) =>
  invoke<void>('set_active_mods', { profileId, activeIds });
export const sortMods = (activeIds: string[]) => invoke<SortResult>('sort_mods', { activeIds });

export const refreshRulesDb = () => invoke<RulesDbStatus>('refresh_rules_db');
export const getRulesDbStatus = () => invoke<RulesDbStatus>('get_rules_db_status');
