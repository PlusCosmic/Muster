// Dev-only fixture switch. During `vite dev`, opening the app with `?mock=1`
// makes each game's backend module swap in its fixture data so the UI can be
// worked on in a plain browser. The choice is remembered for the tab
// (sessionStorage) because the root route redirects and the game rail
// navigates without the query string; a reload would otherwise silently fall
// back to the real bindings. `import.meta.env.DEV` is statically false in a
// production build, so nothing mock-related is ever bundled there.

const KEY = 'muster-mock';

function detect(): boolean {
  if (!import.meta.env.DEV || typeof location === 'undefined') return false;
  const param = new URLSearchParams(location.search).get('mock');
  try {
    if (param === '1') sessionStorage.setItem(KEY, '1');
    else if (param === '0') sessionStorage.removeItem(KEY);
    return sessionStorage.getItem(KEY) === '1';
  } catch {
    return param === '1';
  }
}

export const MOCK_ENABLED: boolean = detect();
