// Typed wrappers over the app-level (game-neutral) Wails service. Each game
// has its own api module (e.g. $lib/rimworld/api) for its own service.
import * as App from '$bindings/muster/app';
import type { AppInfo } from '$bindings/muster/internal/models/models';

export type { AppInfo };

export const revealPath = (path: string): Promise<void> => App.RevealPath(path);
export const getAppInfo = (): Promise<AppInfo> => App.GetAppInfo();
