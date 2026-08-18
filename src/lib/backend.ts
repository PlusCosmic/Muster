// Single choke point for backend calls.
//
// Production always uses the supervisor-owned wrappers in `$lib/api`. During
// `vite dev` only, appending `?mock=1` to the URL swaps in the fixture backend
// from `$lib/mockData` so the UI can be worked on before the Rust commands
// land. `import.meta.env.DEV` is statically false in a production build, so the
// mock is never imported (and never bundled) there.

import * as api from './api';

export const MOCK_ENABLED: boolean =
  import.meta.env.DEV &&
  typeof location !== 'undefined' &&
  new URLSearchParams(location.search).get('mock') === '1';

const backend: typeof api = MOCK_ENABLED
  ? ((await import('./mockData')).mockApi as unknown as typeof api)
  : api;

export const {
  getSettings,
  updateSettings,
  detectPaths,
  listProfiles,
  createProfile,
  renameProfile,
  deleteProfile,
  cloneProfile,
  importDefault,
  launchProfile,
  createShortcut,
  listInstalledMods,
  getActiveMods,
  setActiveMods,
  sortMods,
  refreshRulesDb,
  getRulesDbStatus
} = backend;
