// Window-width policy for the shell.
//
// The two mod columns are the app's reason to exist, so they get first claim on
// horizontal space: below `NARROW_PX` the profile sidebar stops being a docked
// column and becomes an overlay drawer. Wide windows keep the sidebar docked,
// but the user can still collapse it to reclaim its width — that choice is a
// per-machine preference (localStorage), like the theme.

const NARROW_PX = 1100;
const STORAGE_KEY = 'rimforge-sidebar';

function load(): boolean {
  if (typeof localStorage === 'undefined') return true;
  return localStorage.getItem(STORAGE_KEY) !== 'collapsed';
}

const mediaQuery = `(max-width: ${NARROW_PX}px)`;

function isNarrow(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false;
  return window.matchMedia(mediaQuery).matches;
}

class LayoutStore {
  /** Viewport is too tight to dock the sidebar alongside the mod columns. */
  narrow = $state(isNarrow());
  /** Wide-window preference: is the docked sidebar shown? Persisted. */
  docked = $state<boolean>(load());
  /** Narrow-window drawer: closed until asked for. Never persisted. */
  drawerOpen = $state(false);

  /** Is the sidebar on screen at all, docked or floating? */
  get sidebarVisible(): boolean {
    return this.narrow ? this.drawerOpen : this.docked;
  }

  /** Is it floating over the content (vs. taking a grid column)? */
  get sidebarFloating(): boolean {
    return this.narrow;
  }

  toggleSidebar() {
    if (this.narrow) {
      this.drawerOpen = !this.drawerOpen;
    } else {
      this.docked = !this.docked;
      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(STORAGE_KEY, this.docked ? 'docked' : 'collapsed');
      }
    }
  }

  /** Dismiss the drawer — backdrop, Escape, or navigating to a profile. */
  closeDrawer() {
    this.drawerOpen = false;
  }

  /** Track the breakpoint. Returns a teardown for onMount. */
  watch(): () => void {
    if (typeof window === 'undefined' || !window.matchMedia) return () => {};
    const mq = window.matchMedia(mediaQuery);
    const sync = () => {
      this.narrow = mq.matches;
      if (!mq.matches) this.drawerOpen = false;
    };
    sync();
    mq.addEventListener('change', sync);
    return () => mq.removeEventListener('change', sync);
  }
}

export const layout = new LayoutStore();
