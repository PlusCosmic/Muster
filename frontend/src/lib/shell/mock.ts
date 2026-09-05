// Dev-only fixture switch. During `vite dev`, appending `?mock=1` to the URL
// makes each game's backend module swap in its fixture data so the UI can be
// worked on in a plain browser. `import.meta.env.DEV` is statically false in
// a production build, so nothing mock-related is ever bundled there.
export const MOCK_ENABLED: boolean =
  import.meta.env.DEV &&
  typeof location !== 'undefined' &&
  new URLSearchParams(location.search).get('mock') === '1';
