// DEV-ONLY fixture backend for the Minecraft module. Gated by MOCK_ENABLED.
import type { Detected, LaunchSettings, Pack, PackCheck, Settings, SyncProgress, SyncReport } from './types';

const HOUR = 3_600_000;
let settings: Settings = { codes: [{ code: 'plum-weasel-23', addedAtMs: Date.now() - 3 * HOUR, pack: null }], manifestUrl: 'https://packs.example.com/manifest.json', registryUrlOverride: null, minecraftDirOverride: null, packs: {} };
const TOTAL_MB = 16384;
const MAX_HEAP = 12288;
const recommended: Record<string, { min: number; max: number; args: string[] }> = {
  frontier: { min: 4096, max: 8192, args: ['-XX:+UseZGC', '-XX:+ZGenerational'] },
  skyblock: { min: 0, max: 4096, args: [] }
};
function launchFor(id: string): { launch: LaunchSettings; customised: boolean } {
  const rec = recommended[id];
  const saved = settings.packs[id];
  if (!saved) {
    return { launch: { maxMemoryMb: Math.min(rec.max || 4096, MAX_HEAP), minMemoryMb: null, args: [...rec.args], followRecommendedArgs: true }, customised: false };
  }
  return { launch: { ...saved, args: saved.followRecommendedArgs ? [...rec.args] : saved.args }, customised: true };
}
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
    name: 'Frontier',
    source: 'code',
    code: 'plum-weasel-23',
    description: 'A kitchen-sink NeoForge pack: tech, exploration and a shared server.',
    icon: null,
    packUrl: 'https://packs.example.com/pack.toml',
    server: 'play.example.com',
    recommendedMinMemoryMb: 4096,
    recommendedMaxMemoryMb: 8192,
    recommendedArgs: recommended.frontier.args,
    launch: launchFor('frontier').launch,
    launchCustomised: launchFor('frontier').customised,
    installDir: '/home/you/.local/share/muster/minecraft/packs/frontier',
    installed: state.frontier.installed,
    installedVersion: state.frontier.version,
    syncedAtMs: state.frontier.syncedAtMs,
    profileWritten: state.frontier.profile
  },
  {
    id: 'skyblock',
    name: 'Weekend Skyblock',
    source: 'manifest',
    code: null,
    description: 'A small Fabric skyblock for lazy Sundays.',
    icon: null,
    packUrl: 'https://packs.example.com/skyblock/pack.toml',
    server: null,
    recommendedMinMemoryMb: 0,
    recommendedMaxMemoryMb: 4096,
    recommendedArgs: [],
    launch: launchFor('skyblock').launch,
    launchCustomised: launchFor('skyblock').customised,
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
    registryUrl: settings.registryUrlOverride ?? 'https://api.musterlauncher.com',
    minecraftDir: settings.minecraftDirOverride ?? '/home/you/.minecraft',
    launcherInstalled: true,
    packsDir: '/home/you/.local/share/muster/minecraft/packs',
    totalMemoryMb: TOTAL_MB,
    maxHeapMb: MAX_HEAP
  }),
  listPacks: async (): Promise<Pack[]> => {
    await sleep(300);
    if (!settings.manifestUrl && settings.codes.length === 0) throw new Error('no packs yet — enter a pack code, or set a pack list URL in Settings');
    return packs().filter((p) => (p.source === 'code' ? settings.codes.some((c) => c.code === p.code) : !!settings.manifestUrl));
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
  addPackCode: async (input: string): Promise<Pack> => {
    await sleep(400);
    const code = input.trim().toLowerCase().replace(/^.*\//, '').replace(/\s+/g, '-');
    if (!/^[a-z0-9]+(-[a-z0-9]+)+$/.test(code)) throw new Error(`"${input}" does not look like a pack code (e.g. amber-otter-42)`);
    if (code !== 'plum-weasel-23') throw new Error('no pack is registered with that code');
    if (!settings.codes.some((c) => c.code === code)) settings.codes = [...settings.codes, { code, addedAtMs: Date.now(), pack: null }];
    return packs()[0];
  },
  removePackCode: async (code: string): Promise<void> => {
    settings.codes = settings.codes.filter((c) => c.code !== code);
  },
  getLaunchSettings: async (id: string): Promise<LaunchSettings> => launchFor(id).launch,
  setLaunchSettings: async (id: string, ls: LaunchSettings): Promise<LaunchSettings> => {
    if (ls.args.some((a) => /\s/.test(a) || a === '')) throw new Error(`JVM argument contains whitespace, which the Minecraft launcher cannot pass on`);
    const max = Math.max(1024, Math.min(MAX_HEAP, ls.maxMemoryMb - (ls.maxMemoryMb % 512)));
    settings.packs = { ...settings.packs, [id]: { ...ls, maxMemoryMb: max } };
    return launchFor(id).launch;
  },
  resetLaunchSettings: async (id: string): Promise<LaunchSettings> => {
    const { [id]: _drop, ...rest } = settings.packs;
    settings.packs = rest;
    return launchFor(id).launch;
  },
  onSyncProgress: (cb: (p: SyncProgress) => void): (() => void) => {
    listeners.add(cb);
    return () => listeners.delete(cb);
  }
};
