// DEV-ONLY fixture backend for the Minecraft module. Gated by MOCK_ENABLED.
import type { Detected, Pack, PackCheck, Settings, SyncProgress, SyncReport } from './types';

const HOUR = 3_600_000;
let settings: Settings = { manifestUrl: 'https://packs.example.com/manifest.json', minecraftDirOverride: null };
const listeners = new Set<(p: SyncProgress) => void>();
const emit = (p: SyncProgress) => listeners.forEach((l) => l(p));
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

const state: Record<string, { installed: boolean; version: string | null; syncedAtMs: number | null; profile: boolean; loader: boolean }> = {
  frontier: { installed: true, version: '1.0.0', syncedAtMs: Date.now() - 20 * HOUR, profile: true, loader: true },
  skyblock: { installed: false, version: null, syncedAtMs: null, profile: false, loader: false }
};

const packs = (): Pack[] => [
  {
    id: 'frontier',
    name: "Frontier",
    description: 'A kitchen-sink NeoForge pack: tech, exploration and a shared server.',
    icon: null,
    packUrl: 'https://packs.example.com/pack.toml',
    server: 'play.example.com',
    minMemoryMb: 4096,
    maxMemoryMb: 8192,
    javaArgs: ['-XX:+UseZGC'],
    installDir: '/home/you/.local/share/muster/minecraft/packs/frontier',
    installed: state.frontier.installed,
    installedVersion: state.frontier.version,
    syncedAtMs: state.frontier.syncedAtMs,
    profileWritten: state.frontier.profile
  },
  {
    id: 'skyblock',
    name: 'Weekend Skyblock',
    description: 'A small Fabric skyblock for lazy Sundays.',
    icon: null,
    packUrl: 'https://packs.example.com/skyblock/pack.toml',
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
    manifestUrl: settings.manifestUrl,
    minecraftDir: settings.minecraftDirOverride ?? '/home/you/.minecraft',
    launcherInstalled: true,
    packsDir: '/home/you/.local/share/muster/minecraft/packs'
  }),
  listPacks: async (): Promise<Pack[]> => {
    await sleep(300);
    if (!settings.manifestUrl) throw new Error('no pack list configured — set a manifest URL in Settings');
    return packs();
  },
  checkPack: async (id: string): Promise<PackCheck> => {
    await sleep(500);
    const s = state[id];
    const latest = id === 'frontier' ? '1.1.0' : '0.3.0';
    return {
      id,
      latestVersion: latest,
      minecraft: '1.21.1',
      loader: id === 'frontier' ? 'neoforge' : 'fabric',
      loaderVersion: id === 'frontier' ? '21.1.248' : '0.16.9',
      versionId: id === 'frontier' ? 'neoforge-21.1.248' : 'fabric-loader-0.16.9-1.21.1',
      loaderInstalled: s.loader,
      toDownload: s.version === latest ? 0 : id === 'frontier' ? 14 : 63,
      toDelete: s.version === latest ? 0 : 2,
      upToDate: s.version === latest
    };
  },
  syncPack: async (id: string): Promise<SyncReport> => {
    const total = id === 'frontier' ? 14 : 63;
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
    const latest = id === 'frontier' ? '1.1.0' : '0.3.0';
    state[id] = { installed: true, version: latest, syncedAtMs: Date.now(), profile: true, loader: true };
    return {
      id,
      version: latest,
      downloaded: Array.from({ length: total }, (_, i) => `mods/example-${i + 1}.jar`),
      deleted: ['mods/old-thing.jar'],
      manual: id === 'frontier' ? [{ path: 'mods/mcw-windows.jar', name: "Macaw's Windows", url: 'https://www.curseforge.com/minecraft/mc-mods/macaws-windows', why: 'CurseForge refused the download (HTTP 403)' }] : [],
      profileWritten: true,
      loaderInstalled: true,
      versionId: id === 'frontier' ? 'neoforge-21.1.248' : 'fabric-loader-0.16.9-1.21.1',
      launcherOpen: id === 'frontier'
    };
  },
  openLauncher: async (): Promise<void> => {},
  launcherRunning: async (): Promise<boolean> => false,
  onSyncProgress: (cb: (p: SyncProgress) => void): (() => void) => {
    listeners.add(cb);
    return () => listeners.delete(cb);
  }
};
