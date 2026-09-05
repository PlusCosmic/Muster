// Central state for the Minecraft module. Every backend call lives here;
// components render and dispatch.
import * as backend from '../backend';
import type { Detected, LaunchSettings, Pack, PackCheck, Settings, SyncProgress, SyncReport } from '../types';
import { toastError, toastInfo, toastSuccess } from '$lib/shell/stores/toasts.svelte';

export const NO_MANIFEST = 'no pack list configured';

class PacksStore {
  packs = $state<Pack[]>([]);
  detected = $state<Detected | null>(null);
  settings = $state<Settings | null>(null);
  /** Upstream state per pack id, filled lazily by check(). */
  checks = $state<Record<string, PackCheck>>({});
  checking = $state<Record<string, boolean>>({});
  /** The one sync that may run at a time. */
  syncingId = $state<string | null>(null);
  progress = $state<SyncProgress | null>(null);
  /** Last sync's outcome per pack id, for the manual-download list. */
  reports = $state<Record<string, SyncReport>>({});

  booting = $state(true);
  loadingPacks = $state(false);
  /** Set when listing failed because no manifest URL is configured. */
  needsManifest = $state(false);
  /** Set when listing failed for another reason (network, bad manifest). */
  listError = $state<string | null>(null);

  initialised = false;
  private unsubscribe: (() => void) | null = null;

  async init() {
    if (this.initialised) return;
    this.initialised = true;
    this.unsubscribe = backend.onSyncProgress((p) => {
      if (p.id === this.syncingId) this.progress = p;
    });
    this.booting = true;
    await Promise.all([this.loadSettings(), this.loadDetected()]);
    await this.loadPacks();
    this.booting = false;
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
      this.detected = await backend.detect();
    } catch (e) {
      toastError('Could not detect the Minecraft launcher', e);
    }
  }

  async loadPacks() {
    this.loadingPacks = true;
    try {
      this.packs = await backend.listPacks();
      this.needsManifest = false;
      this.listError = null;
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes(NO_MANIFEST)) {
        this.needsManifest = true;
        this.listError = null;
      } else {
        this.listError = msg;
      }
      this.packs = [];
    } finally {
      this.loadingPacks = false;
    }
  }

  /** Ask upstream what a sync would do. Quiet on failure: the card shows it. */
  async check(id: string): Promise<PackCheck | null> {
    this.checking = { ...this.checking, [id]: true };
    try {
      const c = await backend.checkPack(id);
      this.checks = { ...this.checks, [id]: c };
      return c;
    } catch (e) {
      toastError('Could not check the pack', e);
      return null;
    } finally {
      this.checking = { ...this.checking, [id]: false };
    }
  }

  async sync(id: string): Promise<SyncReport | null> {
    if (this.syncingId) return null;
    this.syncingId = id;
    this.progress = null;
    try {
      const report = await backend.syncPack(id);
      this.reports = { ...this.reports, [id]: report };
      await Promise.all([this.loadPacks(), this.check(id)]);
      const n = report.downloaded.length;
      const what = n === 0 ? 'Already up to date' : `${n} file${n === 1 ? '' : 's'} updated`;
      const next = report.launcherOpen
        ? 'The Minecraft launcher is open: close it and open it again, and the pack will be selected.'
        : 'Open the Minecraft launcher and press Play.';
      toastSuccess(`${this.nameOf(id)} is ready`, `${what}. ${next}`);
      return report;
    } catch (e) {
      toastError(`Could not install ${this.nameOf(id)}`, e);
      await this.loadPacks();
      return null;
    } finally {
      this.syncingId = null;
      this.progress = null;
    }
  }

  async openLauncher(): Promise<void> {
    try {
      if (await backend.launcherRunning()) {
        toastInfo(
          'The Minecraft launcher is already open',
          'Close it and click Open launcher again; it only picks up new packs when it starts.'
        );
        return;
      }
      await backend.openLauncher();
    } catch (e) {
      toastError('Could not open the Minecraft launcher', e);
    }
  }

  async updateSettings(next: Settings): Promise<boolean> {
    try {
      this.settings = await backend.updateSettings(next);
      await this.loadDetected();
      await this.loadPacks();
      this.checks = {};
      toastSuccess('Settings saved');
      return true;
    } catch (e) {
      toastError('Could not save settings', e);
      return false;
    }
  }

  /** Save a pack's launch settings; the launcher profile is rewritten by the backend. */
  async setLaunch(id: string, ls: LaunchSettings): Promise<boolean> {
    try {
      const stored = await backend.setLaunchSettings(id, ls);
      this.patchLaunch(id, stored, true);
      toastSuccess('Launch settings saved', `${this.nameOf(id)} will use ${(stored.maxMemoryMb / 1024).toFixed(1).replace(/\.0$/, '')} GB.`);
      return true;
    } catch (e) {
      toastError('Could not save launch settings', e);
      return false;
    }
  }

  async resetLaunch(id: string): Promise<boolean> {
    try {
      const stored = await backend.resetLaunchSettings(id);
      this.patchLaunch(id, stored, false);
      toastSuccess('Launch settings reset', 'Back to what the pack recommends for this machine.');
      return true;
    } catch (e) {
      toastError('Could not reset launch settings', e);
      return false;
    }
  }

  private patchLaunch(id: string, launch: LaunchSettings, customised: boolean) {
    this.packs = this.packs.map((p) => (p.id === id ? { ...p, launch, launchCustomised: customised } : p));
  }

  /** First-run: save just the manifest URL and load. */
  async setManifestUrl(url: string): Promise<boolean> {
    return this.updateSettings({ ...(this.settings ?? { manifestUrl: null, minecraftDirOverride: null, packs: {} }), manifestUrl: url });
  }

  nameOf(id: string): string {
    return this.packs.find((p) => p.id === id)?.name ?? id;
  }
}

export const packs = new PacksStore();
