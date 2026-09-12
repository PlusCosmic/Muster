// Boundary types for the Minecraft module, derived from the bindings that
// `wails3 generate bindings` writes from internal/minecraft/models/models.go.
// Regenerate rather than edit. Shapes are bound by docs/ARCHITECTURE.md.
import type * as m from '$bindings/muster/internal/minecraft/models/models';

type Lists<T> = { [K in keyof T]: T[K] extends (infer U)[] | null ? U[] : T[K] };

export type PackCode = m.PackCode;
export type ModrinthPack = m.ModrinthPack;
export type Settings = Omit<Lists<m.Settings>, 'packs' | 'codes' | 'modrinth'> & {
  packs: Record<string, LaunchSettings>;
  codes: PackCode[];
  modrinth: ModrinthPack[];
};
export type Detected = m.Detected;
export type LaunchSettings = Lists<m.LaunchSettings>;
export type Pack = Omit<Lists<m.Pack>, 'launch'> & { launch: LaunchSettings };
export type PackCheck = m.PackCheck;
export type ModrinthVersion = Lists<m.ModrinthVersion>;
export type ModrinthLookup = Omit<Lists<m.ModrinthLookup>, 'versions'> & { versions: ModrinthVersion[] };
export type Manual = m.Manual;
export type SyncReport = Lists<m.SyncReport>;

export type SyncPhase = 'files' | 'loader' | 'profile' | 'waiting';

/**
 * Payload of the `minecraft:sync` event (models.SyncProgress). The bindings
 * generator only emits models that appear in a method signature, and this one
 * travels by event, so it is mirrored here by hand — keep in step with
 * internal/minecraft/models/models.go.
 */
export interface SyncProgress {
  id: string;
  phase: SyncPhase | string;
  done: number;
  total: number;
  current: string;
}
