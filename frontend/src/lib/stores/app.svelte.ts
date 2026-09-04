// Central application state. Every backend call lives here (or in a component
// that immediately toasts its own failure); nothing else talks to the backend.

import * as backend from '../backend';
import { majorMinor } from '../format';
import type {
  DetectedPaths,
  ModInfo,
  Profile,
  RulesDbStatus,
  Settings,
  SortWarning
} from '../types';
import { toastError, toastSuccess } from './toasts.svelte';

export const CORE_ID = 'ludeon.rimworld';

/** Official content ships no <name> in About.xml; the game hardcodes these. */
const OFFICIAL_NAMES: Record<string, string> = {
  [CORE_ID]: 'RimWorld',
  'ludeon.rimworld.royalty': 'Royalty',
  'ludeon.rimworld.ideology': 'Ideology',
  'ludeon.rimworld.biotech': 'Biotech',
  'ludeon.rimworld.anomaly': 'Anomaly',
  'ludeon.rimworld.odyssey': 'Odyssey'
};

/** Placeholder for an active id with no matching installed mod. */
export function ghostMod(packageId: string): ModInfo {
  return {
    packageId,
    name: OFFICIAL_NAMES[packageId] ?? packageId,
    authors: '',
    path: '',
    source: 'local',
    steamWorkshopId: null,
    supportedVersions: [],
    dependencies: [],
    loadAfter: [],
    loadBefore: [],
    forceLoadAfter: [],
    forceLoadBefore: [],
    incompatibleWith: []
  };
}

function sameList(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((v, i) => v === b[i]);
}

class AppStore {
  // --- server state -------------------------------------------------------
  profiles = $state<Profile[]>([]);
  installedMods = $state<ModInfo[]>([]);
  detected = $state<DetectedPaths | null>(null);
  settings = $state<Settings | null>(null);
  rulesStatus = $state<RulesDbStatus | null>(null);

  // --- selection & mod-list draft ----------------------------------------
  selectedId = $state<string | null>(null);
  /** Draft order — what the editor shows. */
  activeIds = $state<string[]>([]);
  /** Last known persisted order, for dirty comparison. */
  savedIds = $state<string[]>([]);
  warnings = $state<SortWarning[]>([]);

  // --- flags --------------------------------------------------------------
  booting = $state(true);
  loadingProfiles = $state(false);
  loadingMods = $state(false);
  loadingActive = $state(false);
  saving = $state(false);
  sorting = $state(false);
  busyProfileId = $state<string | null>(null);

  // --- derived ------------------------------------------------------------
  selected = $derived(this.profiles.find((p) => p.id === this.selectedId) ?? null);
  dirty = $derived(this.selectedId !== null && !sameList(this.activeIds, this.savedIds));
  modsById = $derived(new Map(this.installedMods.map((m) => [m.packageId, m])));
  gameMajorMinor = $derived(majorMinor(this.detected?.gameVersion));

  /** Active list resolved to ModInfo, in load order (ghosts for unknown ids). */
  activeMods = $derived(this.activeIds.map((id) => this.modsById.get(id) ?? ghostMod(id)));

  activeIdSet = $derived(new Set(this.activeIds));

  /** Installed-but-not-active, sorted for browsing. */
  inactiveMods = $derived(
    this.installedMods
      .filter((m) => !this.activeIdSet.has(m.packageId))
      .sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()))
  );

  mod(packageId: string): ModInfo {
    return this.modsById.get(packageId) ?? ghostMod(packageId);
  }

  /** True when the mod doesn't list the running game's major.minor. */
  isVersionMismatch(m: ModInfo): boolean {
    const gv = this.gameMajorMinor;
    if (!gv || m.supportedVersions.length === 0) return false;
    return !m.supportedVersions.some((v) => majorMinor(v) === gv);
  }

  // --- loading ------------------------------------------------------------

  async init() {
    this.booting = true;
    await Promise.all([
      this.loadSettings(),
      this.loadDetected(),
      this.loadProfiles(),
      this.loadInstalledMods(),
      this.loadRulesStatus()
    ]);
    if (!this.selectedId && this.profiles.length > 0) {
      await this.selectProfile(this.mostRecentProfileId());
    }
    this.booting = false;
  }

  mostRecentProfileId(): string {
    const sorted = [...this.profiles].sort(
      (a, b) => (b.lastPlayedAtMs ?? b.createdAtMs) - (a.lastPlayedAtMs ?? a.createdAtMs)
    );
    return sorted[0].id;
  }

  async loadSettings() {
    try {
      this.settings = await backend.getSettings();
    } catch (e) {
      toastError('Could not load settings', e);
    }
  }

  async loadDetected() {
    try {
      this.detected = await backend.detectPaths();
    } catch (e) {
      toastError('Could not detect RimWorld paths', e);
    }
  }

  async loadProfiles() {
    this.loadingProfiles = true;
    try {
      this.profiles = await backend.listProfiles();
      if (this.selectedId && !this.profiles.some((p) => p.id === this.selectedId)) {
        this.clearSelection();
      }
    } catch (e) {
      toastError('Could not list profiles', e);
    } finally {
      this.loadingProfiles = false;
    }
  }

  async loadInstalledMods() {
    this.loadingMods = true;
    try {
      this.installedMods = await backend.listInstalledMods();
    } catch (e) {
      toastError('Could not scan installed mods', e);
    } finally {
      this.loadingMods = false;
    }
  }

  async loadRulesStatus() {
    try {
      this.rulesStatus = await backend.getRulesDbStatus();
    } catch (e) {
      toastError('Could not read rules database status', e);
    }
  }

  clearSelection() {
    this.selectedId = null;
    this.activeIds = [];
    this.savedIds = [];
    this.warnings = [];
  }

  /** Select a profile and load its active mod list. Discards any draft. */
  async selectProfile(id: string) {
    this.selectedId = id;
    this.warnings = [];
    await this.loadActiveMods();
  }

  async loadActiveMods() {
    const id = this.selectedId;
    if (!id) return;
    this.loadingActive = true;
    try {
      const list = await backend.getActiveMods(id);
      this.savedIds = [...list.activeIds];
      this.activeIds = [...list.activeIds];
    } catch (e) {
      this.savedIds = [];
      this.activeIds = [];
      toastError('Could not read the active mod list', e);
    } finally {
      this.loadingActive = false;
    }
  }

  /** Re-read everything for the current profile from disk, dropping the draft. */
  async reloadCurrent() {
    this.warnings = [];
    await Promise.all([this.loadInstalledMods(), this.loadActiveMods()]);
  }

  // --- mod list draft edits ----------------------------------------------

  activate(packageId: string, index?: number) {
    if (this.activeIds.includes(packageId)) return;
    const next = [...this.activeIds];
    next.splice(index ?? next.length, 0, packageId);
    this.activeIds = next;
  }

  activateMany(packageIds: string[]) {
    const next = [...this.activeIds];
    for (const id of packageIds) if (!next.includes(id)) next.push(id);
    this.activeIds = next;
  }

  deactivate(packageId: string) {
    if (packageId === CORE_ID) return;
    this.activeIds = this.activeIds.filter((id) => id !== packageId);
  }

  deactivateAll() {
    this.activeIds = this.activeIds.filter((id) => id === CORE_ID);
  }

  /** Move the item at `from` so it lands at index `to` in the new list. */
  reorder(from: number, to: number) {
    if (from === to || from < 0 || from >= this.activeIds.length) return;
    const next = [...this.activeIds];
    const [item] = next.splice(from, 1);
    next.splice(Math.max(0, Math.min(to, next.length)), 0, item);
    this.activeIds = next;
  }

  moveBy(packageId: string, delta: number) {
    const from = this.activeIds.indexOf(packageId);
    if (from < 0) return;
    this.reorder(from, Math.max(0, Math.min(this.activeIds.length - 1, from + delta)));
  }

  resetDraft() {
    this.activeIds = [...this.savedIds];
    this.warnings = [];
  }

  // --- actions ------------------------------------------------------------

  async save(): Promise<boolean> {
    const id = this.selectedId;
    if (!id) return false;
    this.saving = true;
    try {
      await backend.setActiveMods(id, this.activeIds);
      this.savedIds = [...this.activeIds];
      const p = this.profiles.find((x) => x.id === id);
      if (p) p.activeModCount = this.activeIds.length;
      toastSuccess('Mod list saved', `${this.activeIds.length} active mods written to ModsConfig.xml`);
      return true;
    } catch (e) {
      toastError('Could not save the mod list', e);
      return false;
    } finally {
      this.saving = false;
    }
  }

  async autoSort(): Promise<void> {
    if (this.activeIds.length === 0) return;
    this.sorting = true;
    try {
      const result = await backend.sortMods(this.activeIds);
      this.activeIds = [...result.sorted];
      this.warnings = result.warnings;
      if (result.warnings.length === 0) {
        toastSuccess('Sorted', 'No problems found in the load order.');
      }
    } catch (e) {
      toastError('Auto-sort failed', e);
    } finally {
      this.sorting = false;
    }
  }

  async launch(id: string): Promise<void> {
    this.busyProfileId = id;
    try {
      await backend.launchProfile(id);
      const p = this.profiles.find((x) => x.id === id);
      if (p) p.lastPlayedAtMs = Date.now();
      toastSuccess('Launching RimWorld', 'Steam is starting the game with this profile.');
    } catch (e) {
      toastError('Could not launch RimWorld', e);
    } finally {
      this.busyProfileId = null;
    }
  }

  async createProfile(name: string): Promise<Profile | null> {
    try {
      const p = await backend.createProfile(name);
      await this.loadProfiles();
      await this.selectProfile(p.id);
      toastSuccess('Profile created', p.name);
      return p;
    } catch (e) {
      toastError('Could not create the profile', e);
      return null;
    }
  }

  async importDefault(name: string): Promise<Profile | null> {
    try {
      const p = await backend.importDefault(name);
      await this.loadProfiles();
      await this.selectProfile(p.id);
      toastSuccess('Setup imported', `Copied your current RimWorld config and saves into “${p.name}”.`);
      return p;
    } catch (e) {
      toastError('Could not import your current setup', e);
      return null;
    }
  }

  async cloneProfile(id: string, newName: string): Promise<Profile | null> {
    this.busyProfileId = id;
    try {
      const p = await backend.cloneProfile(id, newName);
      await this.loadProfiles();
      await this.selectProfile(p.id);
      toastSuccess('Profile cloned', p.name);
      return p;
    } catch (e) {
      toastError('Could not clone the profile', e);
      return null;
    } finally {
      this.busyProfileId = null;
    }
  }

  async renameProfile(id: string, newName: string): Promise<boolean> {
    this.busyProfileId = id;
    try {
      await backend.renameProfile(id, newName);
      await this.loadProfiles();
      toastSuccess('Profile renamed', newName);
      return true;
    } catch (e) {
      toastError('Could not rename the profile', e);
      return false;
    } finally {
      this.busyProfileId = null;
    }
  }

  async deleteProfile(id: string): Promise<boolean> {
    this.busyProfileId = id;
    const name = this.profiles.find((p) => p.id === id)?.name ?? id;
    try {
      await backend.deleteProfile(id);
      const wasSelected = this.selectedId === id;
      await this.loadProfiles();
      if (wasSelected) {
        this.clearSelection();
        if (this.profiles.length > 0) await this.selectProfile(this.mostRecentProfileId());
      }
      toastSuccess('Profile moved to trash', name);
      return true;
    } catch (e) {
      toastError('Could not delete the profile', e);
      return false;
    } finally {
      this.busyProfileId = null;
    }
  }

  async updateSettings(next: Settings): Promise<boolean> {
    try {
      this.settings = await backend.updateSettings(next);
      await this.loadDetected();
      await this.loadInstalledMods();
      toastSuccess('Settings saved');
      return true;
    } catch (e) {
      toastError('Could not save settings', e);
      return false;
    }
  }

  async refreshRulesDb(): Promise<RulesDbStatus | null> {
    try {
      this.rulesStatus = await backend.refreshRulesDb();
      toastSuccess('Rules database refreshed', `${this.rulesStatus.ruleCount} rules cached.`);
      return this.rulesStatus;
    } catch (e) {
      toastError('Could not refresh the rules database', e);
      return null;
    }
  }
}

export const app = new AppStore();
