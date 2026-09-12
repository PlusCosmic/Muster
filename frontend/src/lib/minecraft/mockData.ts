// DEV-ONLY fixture backend for the Minecraft module. Gated by MOCK_ENABLED.
import type { Detected, LaunchSettings, ModrinthLookup, ModrinthVersion, Pack, PackCheck, Settings, SyncProgress, SyncReport } from './types';

const HOUR = 3_600_000;
let settings: Settings = {
  codes: [{ code: 'plum-weasel-23', addedAtMs: Date.now() - 3 * HOUR, pack: null }],
  modrinth: [{ projectId: '1KVo5zza', slug: 'fabulously-optimized', packId: 'modrinth-fabulously-optimized', version: '13.4.0', addedAtMs: Date.now() - HOUR, pack: null }],
  manifestUrl: 'https://packs.example.com/manifest.json',
  registryUrlOverride: null,
  minecraftDirOverride: null,
  packs: {}
};
const TOTAL_MB = 16384;
const MAX_HEAP = 12288;
const recommended: Record<string, { min: number; max: number; args: string[] }> = {
  frontier: { min: 4096, max: 8192, args: ['-XX:+UseZGC', '-XX:+ZGenerational'] },
  skyblock: { min: 0, max: 4096, args: [] },
  'modrinth-fabulously-optimized': { min: 0, max: 0, args: [] }
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
  skyblock: { installed: false, version: null, syncedAtMs: null, profile: false, loader: false },
  'modrinth-fabulously-optimized': { installed: true, version: '13.4.0', syncedAtMs: Date.now() - 2 * HOUR, profile: true, loader: true }
};

const FO_VERSIONS: ModrinthVersion[] = [
  { id: 'HiDsl7yh', number: '14.0.0', type: 'release', publishedAtMs: Date.now() - 2 * 24 * HOUR, gameVersions: ['26.2'], loaders: ['fabric'] },
  { id: 'dW11Fn4e', number: '14.0.0-beta.7', type: 'beta', publishedAtMs: Date.now() - 13 * 24 * HOUR, gameVersions: ['26.2'], loaders: ['fabric'] },
  { id: 'ueJkd2dE', number: '13.4.0', type: 'release', publishedAtMs: Date.now() - 13 * 24 * HOUR, gameVersions: ['26.1.2'], loaders: ['fabric'] },
  { id: 'K0lc692U', number: '13.3.0', type: 'release', publishedAtMs: Date.now() - 40 * 24 * HOUR, gameVersions: ['1.21.11'], loaders: ['fabric'] },
  { id: 'a1', number: '13.2.1', type: 'release', publishedAtMs: Date.now() - 70 * 24 * HOUR, gameVersions: ['1.21.10'], loaders: ['fabric'] },
  { id: 'a2', number: '13.2.0-alpha.1', type: 'alpha', publishedAtMs: Date.now() - 90 * 24 * HOUR, gameVersions: ['1.21.10'], loaders: ['fabric'] }
];
const newestRelease = () => FO_VERSIONS.find((v) => v.type === 'release')!.number;
const parseModrinth = (input: string) => /modrinth\.com\/(?:modpack|project)\/([^/?#]+)(?:\/version\/([^/?#]+))?/i.exec(input.trim());

const packs = (): Pack[] => [
  {
    id: 'frontier',
    name: 'Frontier',
    source: 'code',
    code: 'plum-weasel-23',
    project: null,
    heldVersion: null,
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
    project: null,
    heldVersion: null,
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
  },
  {
    id: 'modrinth-fabulously-optimized',
    name: 'Fabulously Optimized',
    source: 'modrinth',
    code: null,
    project: 'fabulously-optimized',
    heldVersion: settings.modrinth[0]?.version ?? null,
    description: 'Beautiful graphics, speedy performance and familiar features in a simple package.',
    icon: 'https://cdn.modrinth.com/data/1KVo5zza/icon.webp',
    packUrl: 'https://modrinth.com/modpack/fabulously-optimized',
    server: null,
    recommendedMinMemoryMb: 0,
    recommendedMaxMemoryMb: 0,
    recommendedArgs: [],
    launch: launchFor('modrinth-fabulously-optimized').launch,
    launchCustomised: launchFor('modrinth-fabulously-optimized').customised,
    installDir: '/home/you/.local/share/muster/minecraft/packs/modrinth-fabulously-optimized',
    installed: state['modrinth-fabulously-optimized'].installed,
    installedVersion: state['modrinth-fabulously-optimized'].version,
    syncedAtMs: state['modrinth-fabulously-optimized'].syncedAtMs,
    profileWritten: state['modrinth-fabulously-optimized'].profile
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
    if (!settings.manifestUrl && settings.codes.length === 0 && settings.modrinth.length === 0) throw new Error('no packs yet — enter a pack code or paste a Modrinth link, or set a pack list URL in Settings');
    return packs().filter((p) =>
      p.source === 'code' ? settings.codes.some((c) => c.code === p.code)
      : p.source === 'modrinth' ? settings.modrinth.some((m) => m.slug === p.project)
      : !!settings.manifestUrl
    );
  },
  checkPack: async (id: string): Promise<PackCheck> => {
    await sleep(500);
    const s = state[id];
    const target = id === 'frontier' ? '1.1.0' : id === 'skyblock' ? '0.3.0' : settings.modrinth[0]?.version ?? newestRelease();
    const latest = id === 'modrinth-fabulously-optimized' ? newestRelease() : target;
    return {
      id,
      latestVersion: latest,
      targetVersion: target,
      updateAvailable: latest !== target,
      minecraft: '1.21.1',
      loader: id === 'frontier' ? 'neoforge' : 'fabric',
      loaderVersion: id === 'frontier' ? '21.1.248' : '0.16.9',
      versionId: id === 'frontier' ? 'neoforge-21.1.248' : 'fabric-loader-0.16.9-1.21.1',
      loaderInstalled: s.loader,
      toDownload: s.version === target ? 0 : id === 'frontier' ? 14 : id === 'skyblock' ? 63 : 101,
      toDelete: s.version === target ? 0 : 2,
      upToDate: s.version === target
    };
  },
  syncPack: async (id: string): Promise<SyncReport> => {
    const total = id === 'frontier' ? 14 : 63;
    for (let i = 1; i <= total; i++) {
      emit({ id, phase: 'files', done: i, total, current: `mods/example-${i}.jar` });
      await sleep(60);
    }
    if (id === 'modrinth-fabulously-optimized') {
      emit({ id, phase: 'waiting', done: 0, total: 0, current: "Waiting 4 s for api.modrinth.com's rate limit…" });
      await sleep(1500);
    }
    if (!state[id].loader) {
      for (const step of ['Installing neoforge 21.1.248', 'Downloading installer', 'Running installer']) {
        emit({ id, phase: 'loader', done: 0, total: 0, current: step });
        await sleep(600);
      }
    }
    emit({ id, phase: 'profile', done: 0, total: 0, current: 'Writing launcher profile' });
    await sleep(300);
    const latest = id === 'frontier' ? '1.1.0' : id === 'skyblock' ? '0.3.0' : settings.modrinth[0]?.version ?? newestRelease();
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
  lookupModrinth: async (input: string): Promise<ModrinthLookup> => {
    await sleep(400);
    const m = parseModrinth(input);
    if (!m) throw new Error('that is not a Modrinth link — it should look like https://modrinth.com/modpack/<name>');
    if (m[1] !== 'fabulously-optimized') throw new Error('Modrinth has no project by that name');
    const linked = m[2] ? decodeURIComponent(m[2]) : '';
    if (linked && !FO_VERSIONS.some((v) => v.number === linked || v.id === linked)) throw new Error(`the project has no version "${linked}"`);
    const held = settings.modrinth[0]?.version ?? null;
    return {
      project: 'fabulously-optimized',
      input: input.trim(),
      name: 'Fabulously Optimized',
      description: 'Beautiful graphics, speedy performance and familiar features in a simple package.',
      icon: null,
      pageUrl: 'https://modrinth.com/modpack/fabulously-optimized',
      versions: FO_VERSIONS,
      suggested: FO_VERSIONS.find((v) => v.number === linked || v.id === linked)?.number ?? newestRelease(),
      alreadyAdded: settings.modrinth.length > 0,
      heldVersion: held
    };
  },
  addModrinthPack: async (input: string, version: string): Promise<Pack> => {
    await sleep(400);
    const m = parseModrinth(input);
    if (!m) throw new Error('that is not a Modrinth link — it should look like https://modrinth.com/modpack/<name>');
    if (m[1] !== 'fabulously-optimized') throw new Error('Modrinth has no project by that name');
    const want = version || (m[2] ? decodeURIComponent(m[2]) : '');
    const v = want ? FO_VERSIONS.find((x) => x.number === want || x.id === want) : FO_VERSIONS.find((x) => x.type === 'release');
    if (!v) throw new Error(`the project has no version "${want}"`);
    settings.modrinth = [{ projectId: '1KVo5zza', slug: 'fabulously-optimized', packId: 'modrinth-fabulously-optimized', version: v.number, addedAtMs: settings.modrinth[0]?.addedAtMs ?? Date.now(), pack: null }];
    return packs()[2];
  },
  listModrinthVersions: async (id: string): Promise<ModrinthVersion[]> => {
    await sleep(300);
    if (id !== 'modrinth-fabulously-optimized' || settings.modrinth.length === 0) throw new Error(`pack "${id}" is not in your packs`);
    return FO_VERSIONS;
  },
  setModrinthVersion: async (id: string, version: string): Promise<Pack> => {
    await sleep(200);
    if (id !== 'modrinth-fabulously-optimized' || settings.modrinth.length === 0) throw new Error(`pack "${id}" is not in your packs`);
    const v = FO_VERSIONS.find((x) => x.number === version || x.id === version);
    if (!v) throw new Error(`the project has no version "${version}"`);
    settings.modrinth = settings.modrinth.map((m) => ({ ...m, version: v.number }));
    return packs()[2];
  },
  removeModrinthPack: async (id: string): Promise<void> => {
    settings.modrinth = settings.modrinth.filter((m) => m.packId !== id);
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
