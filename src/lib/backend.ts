// Single choke point for backend calls.
//
// Production always uses the supervisor-owned wrappers in `$lib/api`. During
// `vite dev` only, appending `?mock=1` to the URL swaps in the fixture backend
// from `$lib/mockData` so the UI can be worked on before the Rust commands
// land. `import.meta.env.DEV` is statically false in a production build, so the
// mock is never imported (and never bundled) there.
//
// No top-level await here: TLA makes this whole module graph "async", and
// WebKitGTK's JSC resolves `await import()` of async graphs before evaluation
// finishes, which blanks the app in dev (SvelteKit reads a half-initialized
// route module). Every api function returns a Promise already, so awaiting the
// implementation inside each wrapper is behavior-identical.

import * as api from './api';

export const MOCK_ENABLED: boolean =
  import.meta.env.DEV &&
  typeof location !== 'undefined' &&
  new URLSearchParams(location.search).get('mock') === '1';

const impl: Promise<typeof api> = MOCK_ENABLED
  ? import('./mockData').then((m) => m.mockApi as unknown as typeof api)
  : Promise.resolve(api);

const wrap = <K extends keyof typeof api>(name: K): (typeof api)[K] =>
  ((...args: unknown[]) =>
    impl.then((b) => (b[name] as (...a: unknown[]) => unknown)(...args))) as (typeof api)[K];

export const getSettings = wrap('getSettings');
export const updateSettings = wrap('updateSettings');
export const detectPaths = wrap('detectPaths');
export const listProfiles = wrap('listProfiles');
export const createProfile = wrap('createProfile');
export const renameProfile = wrap('renameProfile');
export const deleteProfile = wrap('deleteProfile');
export const cloneProfile = wrap('cloneProfile');
export const importDefault = wrap('importDefault');
export const launchProfile = wrap('launchProfile');
export const createShortcut = wrap('createShortcut');
export const listInstalledMods = wrap('listInstalledMods');
export const getActiveMods = wrap('getActiveMods');
export const setActiveMods = wrap('setActiveMods');
export const sortMods = wrap('sortMods');
export const refreshRulesDb = wrap('refreshRulesDb');
export const getRulesDbStatus = wrap('getRulesDbStatus');
