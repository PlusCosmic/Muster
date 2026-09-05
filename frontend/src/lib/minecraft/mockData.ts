// DEV-ONLY fixture backend for the Minecraft module. Gated by MOCK_ENABLED.
import type { Detected, Pack, PackCheck, Settings, SyncProgress, SyncReport } from './types';

const HOUR = 3_600_000;
let settings: Settings = { manifestUrlOverride: 'https://pack.example/abc/manifest.json', minecraftDirOverride: null };
const listeners = new Set<(p: SyncProgress) => void>();
const emit = (p: SyncProgress) => listeners.forEach((l) => l(p));
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

const state: Record<string, { installed: boolean; version: string | null; syncedAtMs: number | null; profile: boolean; loader: boolean }> = {
  cobblemon: { installed: true, version: '1.0.0', syncedAtMs: Date.now() - 20 * HOUR, profile: true, loader: true },
  skyblock: { installed: false, version: null, syncedAtMs: null, profile: false, loader: false }
};

const packs = (): Pack[] => [
  {
    id: 'cobblemon',
    name: "Cosmic's Cobblemon",
    description: 'Cobblemon 1.8 on NeoForge 1.21.1 with Create, COBBLEVERSE and friends. Joins minecraft.pluscosmic.dev.',
    icon: null,
    packUrl: 'https://pack.example/abc/pack.toml',
    server: 'minecraft.pluscosmic.dev',
    minMemoryMb: 4096,
    maxMemoryMb: 8192,
    javaArgs: ['-XX:+UseZGC'],
    installDir: '/home/you/.local/share/muster/minecraft/packs/cobblemon',
    installed: state.cobblemon.installed,
    installedVersion: state.cobblemon.version,
    syncedAtMs: state.cobblemon.syncedAtMs,
    profileWritten: state.cobblemon.profile
  },
  {
    id: 'skyblock',
    name: 'Weekend Skyblock',
    description: 'A small Fabric skyblock for lazy Sundays.',
    icon: null,
    packUrl: 'https://pack.example/abc/skyblock/pack.toml',
    server: null,
    minMemoryMb: 0,
    maxMemoryMb: 4096,
    javaArgs: [],
    installDir: '/home/you/.local/share/muster/minecraft/packs/skyblock',
    installed: state.skyblock.installed,
    installedVersion: state.skyblock.version,
    syncedAtMs: state.skyblock.syncedAtMs,
    profileWritten: state.skyblock.profile
  }
];

export const mockApi = {
  getSettings: async (): Promise<Settings> => ({ ...settings }),
  updateSettings: async (s: Settings): Promise<Settings> => {
    settings = { ...s };
    return { ...settings };
  },
  detect: async (): Promise<Detected> => ({
    manifestUrl: settings.manifestUrlOverride,
    minecraftDir: settings.minecraftDirOverride ?? '/home/you/.minecraft',
    launcherInstalled: true,
    packsDir: '/home/you/.local/share/muster/minecraft/packs'
  }),
  listPacks: async (): Promise<Pack[]> => {
    await sleep(300);
    if (!settings.manifestUrlOverride) throw new Error('no pack list configured — set a manifest URL in Settings');
    return packs();
  },
  checkPack: async (id: string): Promise<PackCheck> => {
    await sleep(500);
    const s = state[id];
    const latest = id === 'cobblemon' ? '1.1.0' : '0.3.0';
    return {
      id,
      latestVersion: latest,
      minecraft: '1.21.1',
      loader: id === 'cobblemon' ? 'neoforge' : 'fabric',
      loaderVersion: id === 'cobblemon' ? '21.1.248' : '0.16.9',
      versionId: id === 'cobblemon' ? 'neoforge-21.1.248' : 'fabric-loader-0.16.9-1.21.1',
      loaderInstalled: s.loader,
      toDownload: s.version === latest ? 0 : id === 'cobblemon' ? 14 : 63,
      toDelete: s.version === latest ? 0 : 2,
      upToDate: s.version === latest
    };
  },
  syncPack: async (id: string): Promise<SyncReport> => {
    const total = id === 'cobblemon' ? 14 : 63;
    for (let i = 1; i <= total; i++) {
      emit({ id, phase: 'files', done: i, total, current: `mods/example-${i}.jar` });
      await sleep(60);
    }
    if (!state[id].loader) {
      for (const step of ['Installing neoforge 21.1.248', 'Downloading installer', 'Running installer']) {
        emit({ id, phase: 'loader', done: 0, total: 0, current: step });
        await sleep(600);
      }
    }
    emit({ id, phase: 'profile', done: 0, total: 0, current: 'Writing launcher profile' });
    await sleep(300);
    const latest = id === 'cobblemon' ? '1.1.0' : '0.3.0';
    state[id] = { installed: true, version: latest, syncedAtMs: Date.now(), profile: true, loader: true };
    return {
      id,
      version: latest,
      downloaded: Array.from({ length: total }, (_, i) => `mods/example-${i + 1}.jar`),
      deleted: ['mods/old-thing.jar'],
      manual: id === 'cobblemon' ? [{ path: 'mods/mcw-windows.jar', name: "Macaw's Windows", url: 'https://www.curseforge.com/minecraft/mc-mods/macaws-windows', why: 'CurseForge refused the download (HTTP 403)' }] : [],
      profileWritten: true,
      loaderInstalled: true,
      versionId: id === 'cobblemon' ? 'neoforge-21.1.248' : 'fabric-loader-0.16.9-1.21.1'
    };
  },
  openLauncher: async (): Promise<void> => {},
  onSyncProgress: (cb: (p: SyncProgress) => void): (() => void) => {
    listeners.add(cb);
    return () => listeners.delete(cb);
  }
};
