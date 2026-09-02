// The boundary types, derived from the bindings that `wails3 generate
// bindings` writes from internal/models/models.go. Nothing here is hand-
// maintained beyond the mapping: regenerate rather than edit. Shapes are
// bound by docs/ARCHITECTURE.md.
//
// The generator types every Go slice as `T[] | null` because a nil slice
// would serialise as null. The backend never sends one (see models.NonNil),
// so the list fields are narrowed back to plain arrays here.

import type * as m from '$bindings/rimforge/internal/models/models';

type Lists<T> = { [K in keyof T]: T[K] extends (infer U)[] | null ? U[] : T[K] };

export type Settings = m.Settings;
export type DetectedPaths = Lists<m.DetectedPaths>;
export type Profile = m.Profile;

/** The generated enum's values, minus the Go zero value it also declares. */
export type ModSource = Exclude<`${m.ModSource}`, ''>;

export interface ModInfo extends Omit<Lists<m.ModInfo>, 'source'> {
  source: ModSource;
}

export type ActiveModList = Lists<m.ActiveModList>;
export type SortWarning = m.SortWarning;
export type SortResult = Lists<m.SortResult>;
export type RulesDbStatus = m.RulesDbStatus;
