// Typed wrappers over the app-level (game-neutral) Wails service. Each game
// has its own api module (e.g. $lib/rimworld/api) for its own service.
import * as App from '$bindings/muster/app';
import type { AppInfo } from '$bindings/muster/internal/models/models';

export type { AppInfo };

export const revealPath = (path: string): Promise<void> => App.RevealPath(path);
export const getAppInfo = (): Promise<AppInfo> => App.GetAppInfo();

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
