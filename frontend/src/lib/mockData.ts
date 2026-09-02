// DEV-ONLY fixture backend so the UI can be developed without a Go build.
// Gated by `MOCK_ENABLED` in src/lib/backend.ts — never active in production.

import type {
  ActiveModList,
  DetectedPaths,
  ModInfo,
  Profile,
  RulesDbStatus,
  Settings,
  SortResult
} from './types';

const now = Date.now();
const HOUR = 3_600_000;
const DAY = 24 * HOUR;

function mod(
  packageId: string,
  name: string,
  source: ModInfo['source'],
  supportedVersions: string[] = ['1.6'],
  extra: Partial<ModInfo> = {}
): ModInfo {
  return {
    packageId,
    name,
    authors: 'Various',
    path: `/mock/${source}/${packageId}`,
    source,
    steamWorkshopId: source === 'workshop' ? String(1000000 + name.length * 7919) : null,
    supportedVersions,
    dependencies: [],
    loadAfter: [],
    loadBefore: [],
    forceLoadAfter: [],
    forceLoadBefore: [],
    incompatibleWith: [],
    ...extra
  };
}

const MODS: ModInfo[] = [
  mod('ludeon.rimworld', 'Core', 'official'),
  mod('ludeon.rimworld.royalty', 'Royalty', 'official'),
  mod('ludeon.rimworld.ideology', 'Ideology', 'official'),
  mod('ludeon.rimworld.biotech', 'Biotech', 'official'),
  mod('ludeon.rimworld.anomaly', 'Anomaly', 'official'),
  mod('brrainz.harmony', 'Harmony', 'workshop'),
  mod('unlimitedhugs.hugslib', 'HugsLib', 'workshop', ['1.4', '1.5']),
  mod('vanillaexpanded.vfecore', 'Vanilla Framework Expanded', 'workshop', ['1.6'], {
    dependencies: ['brrainz.harmony']
  }),
  mod('vanillaexpanded.vfef', 'Vanilla Furniture Expanded', 'workshop', ['1.6'], {
    dependencies: ['vanillaexpanded.vfecore']
  }),
  mod('vanillaexpanded.vwec', 'Vanilla Weapons Expanded', 'workshop'),
  mod('dubwise.dubsbadhygiene', 'Dubs Bad Hygiene', 'workshop', ['1.5', '1.6']),
  mod('dubwise.dubsmintmenus', 'Dubs Mint Menus', 'workshop'),
  mod('smashphil.vehicleframework', 'Vehicle Framework', 'workshop', ['1.6']),
  mod('zylle.medievaloverhaul', 'Medieval Overhaul', 'workshop', ['1.6'], {
    incompatibleWith: ['vanillaexpanded.vwec']
  }),
  mod('krkr.rocketman', 'RocketMan', 'workshop', ['1.4']),
  mod('taranchuk.performancefish', 'Performance Fish', 'workshop'),
  mod('local.mysecretpatch', 'My Secret Patch', 'local', ['1.6']),
  mod('local.tweaks', 'Colony Tweaks', 'local', ['1.5']),
  mod('fluffy.modmanager', 'Mod Manager', 'workshop'),
  mod('owlchemist.smarterconstruction', 'Smarter Construction', 'workshop', ['1.6']),
  mod('pluscosmic.homestead', 'Homestead', 'local', ['1.6'], {
    dependencies: ['brrainz.harmony']
  }),
  mod('pluscosmic.constructionpriority', 'Construction Priority', 'local', ['1.6'])
];

const PROFILES: Profile[] = [
  {
    id: 'medieval-run',
    name: 'Medieval Run',
    path: '/home/you/.local/share/rimforge/profiles/medieval-run',
    createdAtMs: now - 40 * DAY,
    lastPlayedAtMs: now - 3 * HOUR,
    saveCount: 12,
    activeModCount: 8
  },
  {
    id: 'vanilla-plus',
    name: 'Vanilla Plus',
    path: '/home/you/.local/share/rimforge/profiles/vanilla-plus',
    createdAtMs: now - 90 * DAY,
    lastPlayedAtMs: now - 9 * DAY,
    saveCount: 3,
    activeModCount: 5
  },
  {
    id: 'modding-sandbox',
    name: 'Modding Sandbox',
    path: '/home/you/.local/share/rimforge/profiles/modding-sandbox',
    createdAtMs: now - 2 * DAY,
    lastPlayedAtMs: null,
    saveCount: 0,
    activeModCount: 2
  }
];

const ACTIVE: Record<string, string[]> = {
  'medieval-run': [
    'ludeon.rimworld',
    'ludeon.rimworld.royalty',
    'ludeon.rimworld.ideology',
    'brrainz.harmony',
    'vanillaexpanded.vfecore',
    'zylle.medievaloverhaul',
    'vanillaexpanded.vwec',
    'local.tweaks'
  ],
  'vanilla-plus': [
    'ludeon.rimworld',
    'ludeon.rimworld.royalty',
    'brrainz.harmony',
    'unlimitedhugs.hugslib',
    'fluffy.modmanager'
  ],
  'modding-sandbox': ['ludeon.rimworld', 'pluscosmic.homestead']
};

let settings: Settings = {
  steamRootOverride: null,
  gameInstallOverride: null,
  defaultSavedataOverride: null
};

let rules: RulesDbStatus = { cached: true, fetchedAtMs: now - 2 * DAY, ruleCount: 3412 };

const delay = <T>(value: T, ms = 140): Promise<T> =>
  new Promise((resolve) => setTimeout(() => resolve(value), ms));

const clone = <T>(v: T): T => JSON.parse(JSON.stringify(v)) as T;

function slugify(name: string): string {
  const base = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'profile';
  let slug = base;
  let n = 2;
  while (PROFILES.some((p) => p.id === slug)) slug = `${base}-${n++}`;
  return slug;
}

function add(name: string, saveCount: number, activeIds: string[]): Profile {
  const id = slugify(name);
  const p: Profile = {
    id,
    name,
    path: `/home/you/.local/share/rimforge/profiles/${id}`,
    createdAtMs: Date.now(),
    lastPlayedAtMs: null,
    saveCount,
    activeModCount: activeIds.length
  };
  PROFILES.push(p);
  ACTIVE[id] = [...activeIds];
  return p;
}

function find(id: string): Profile {
  const p = PROFILES.find((x) => x.id === id);
  if (!p) throw `No such profile: ${id}`;
  return p;
}

export const mockApi = {
  getSettings: () => delay(clone(settings)),
  updateSettings: (next: Settings) => {
    settings = clone(next);
    return delay(clone(settings));
  },
  detectPaths: () =>
    delay<DetectedPaths>({
      steamRoot: settings.steamRootOverride ?? '/home/you/.steam/steam',
      gameInstall:
        settings.gameInstallOverride ?? '/home/you/.steam/steam/steamapps/common/RimWorld',
      defaultSavedata:
        settings.defaultSavedataOverride ??
        '/home/you/.config/unity3d/Ludeon Studios/RimWorld by Ludeon Studios',
      workshopDirs: ['/home/you/.steam/steam/steamapps/workshop/content/294100'],
      gameVersion: '1.6.4535 rev991',
      profilesDir: '/home/you/.local/share/rimforge/profiles'
    }),

  listProfiles: () => delay(clone(PROFILES)),
  createProfile: (name: string) => delay(clone(add(name, 0, ['ludeon.rimworld']))),
  renameProfile: (id: string, newName: string) => {
    const p = find(id);
    p.name = newName;
    return delay(clone(p));
  },
  deleteProfile: (id: string) => {
    const i = PROFILES.findIndex((p) => p.id === id);
    if (i >= 0) PROFILES.splice(i, 1);
    delete ACTIVE[id];
    return delay(undefined as void);
  },
  cloneProfile: (id: string, newName: string) => {
    const src = find(id);
    return delay(clone(add(newName, src.saveCount, ACTIVE[id] ?? [])));
  },
  importDefault: (name: string) =>
    delay(clone(add(name, 7, ['ludeon.rimworld', 'brrainz.harmony', 'unlimitedhugs.hugslib']))),

  launchProfile: (id: string) => {
    find(id).lastPlayedAtMs = Date.now();
    return delay(undefined as void);
  },

  listInstalledMods: () => delay(clone(MODS), 260),
  getActiveMods: (profileId: string) =>
    delay<ActiveModList>({
      activeIds: [...(ACTIVE[profileId] ?? ['ludeon.rimworld'])],
      knownExpansions: MODS.filter(
        (m) => m.source === 'official' && m.packageId !== 'ludeon.rimworld'
      ).map((m) => m.packageId),
      version: '1.6.4535 rev991'
    }),
  setActiveMods: (profileId: string, activeIds: string[]) => {
    ACTIVE[profileId] = [...activeIds];
    find(profileId).activeModCount = activeIds.length;
    return delay(undefined as void);
  },

  sortMods: (activeIds: string[]): Promise<SortResult> => {
    const tier = (id: string) => {
      if (id === 'ludeon.rimworld') return 0;
      if (id.startsWith('ludeon.rimworld.')) return 1;
      return 2;
    };
    const byId = new Map(MODS.map((m) => [m.packageId, m]));
    const sorted = [...activeIds].sort((a, b) => {
      const t = tier(a) - tier(b);
      if (t !== 0) return t;
      const na = byId.get(a)?.name ?? a;
      const nb = byId.get(b)?.name ?? b;
      return na.toLowerCase().localeCompare(nb.toLowerCase());
    });
    const warnings: SortResult['warnings'] = [];
    const set = new Set(activeIds);
    for (const id of activeIds) {
      const m = byId.get(id);
      if (!m) {
        warnings.push({ kind: 'unknownMod', packageId: id, message: `${id} is not installed.` });
        continue;
      }
      for (const dep of m.dependencies) {
        if (!set.has(dep)) {
          warnings.push({
            kind: 'missingDependency',
            packageId: id,
            message: `${m.name} requires ${dep}, which is not active.`
          });
        }
      }
      for (const inc of m.incompatibleWith) {
        if (set.has(inc)) {
          warnings.push({
            kind: 'incompatible',
            packageId: id,
            message: `${m.name} declares it is incompatible with ${byId.get(inc)?.name ?? inc}.`
          });
        }
      }
      if (m.supportedVersions.length && !m.supportedVersions.includes('1.6')) {
        warnings.push({
          kind: 'versionMismatch',
          packageId: id,
          message: `${m.name} does not list support for 1.6 (has ${m.supportedVersions.join(', ')}).`
        });
      }
    }
    if (!rules.cached) {
      warnings.push({
        kind: 'rulesDbUnavailable',
        packageId: null,
        message: 'Community rules database unavailable — sorted using About.xml data only.'
      });
    }
    return delay({ sorted, warnings }, 320);
  },

  refreshRulesDb: () => {
    rules = { cached: true, fetchedAtMs: Date.now(), ruleCount: 3419 };
    return delay(clone(rules), 500);
  },
  getRulesDbStatus: () => delay(clone(rules))
};
