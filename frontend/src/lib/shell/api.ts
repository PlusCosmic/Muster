// Typed wrappers over the app-level (game-neutral) Wails service. Each game
// has its own api module (e.g. $lib/rimworld/api) for its own service.
import * as App from '$bindings/muster/app';
import type {
  AppInfo as RawAppInfo,
  AppSettings as RawAppSettings
} from '$bindings/muster/internal/models/models';

// The generated types allow null for every list (Go's nil slice); the backend
// never sends one, and these narrow the lists back to arrays.
export interface AppInfo extends Omit<RawAppInfo, 'gamesWithData'> {
  gamesWithData: string[];
}
export interface AppSettings extends Omit<RawAppSettings, 'games'> {
  games: string[];
}

export const revealPath = (path: string): Promise<void> => App.RevealPath(path);
export const checkForUpdates = (): Promise<boolean> => App.CheckForUpdates();

export async function getAppInfo(): Promise<AppInfo> {
  const info = await App.GetAppInfo();
  return { ...info, gamesWithData: info.gamesWithData ?? [] };
}

export async function getSettings(): Promise<AppSettings> {
  const s = await App.GetSettings();
  return { games: s.games ?? [] };
}

export async function updateSettings(settings: AppSettings): Promise<AppSettings> {
  const s = await App.UpdateSettings(settings);
  return { games: s.games ?? [] };
}

/** Open a URL in the system browser (a new tab when running in a plain browser). */
export async function openExternal(url: string): Promise<void> {
  const { Browser } = await import('@wailsio/runtime');
  const { MOCK_ENABLED } = await import('./mock');
  if (MOCK_ENABLED || typeof (window as unknown as { _wails?: unknown })._wails === 'undefined') {
    window.open(url, '_blank', 'noopener');
    return;
  }
  await Browser.OpenURL(url);
}
