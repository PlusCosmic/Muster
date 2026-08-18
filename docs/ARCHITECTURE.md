# RimForge — Architecture Contract

**This document is the binding contract between backend and frontend, and between
work streams. If the API surface changes, this file changes in the same commit.**

RimForge is a cross-platform (Linux/Windows/macOS) manager for RimWorld
*profiles*: self-contained `-savedatafolder` directories, each with its own
`Config/` (active mod list + mod settings), `Saves/`, and `Scenarios/`.
Installed mods (Steam Workshop + game `Mods/` + official Core/DLC) are shared
between profiles; only the *active list* and settings differ.

Stack: Tauri 2 (Rust backend) + Svelte 5 / TypeScript / SvelteKit static frontend.

## Filesystem layout (app-owned data)

All app data lives under `dirs::data_dir().join("rimforge")`:

```
<data>/rimforge/
  profiles/<slug>/          # one savedatafolder per profile
    Config/ModsConfig.xml   # active mods (may not exist until first run/edit)
    Saves/  Scenarios/ ...
  registry.json             # ProfileMeta records (display name, timestamps)
  settings.json             # Settings (path overrides)
  cache/communityRules.json # cached RimSort community rules DB
  cache/rules_meta.json     # { fetchedAtMs, etag? }
```

Slugs are derived from the display name: lowercase, `[a-z0-9-]`, collisions
suffixed `-2`, `-3`… A profile's identity is its slug (`id`).

## External paths (detected, overridable in settings)

- **Steam root**: Linux `~/.steam/steam` or `~/.local/share/Steam`; Windows via
  `HKLM/HKCU SOFTWARE\Valve\Steam InstallPath` falling back to
  `C:\Program Files (x86)\Steam`; macOS `~/Library/Application Support/Steam`.
- **Library folders**: parse `<steam>/steamapps/libraryfolders.vdf`. Do not add
  a VDF crate: extract with a line regex for `"path"\s+"(...)"` (unescape `\\`).
- **Game install**: first library containing `steamapps/common/RimWorld`
  (Linux binary `RimWorldLinux`, mac `RimWorldMac.app`, win `RimWorldWin64.exe`).
- **Official content**: `<install>/Data/<Name>/About/About.xml` (Core + DLC).
- **Local mods**: `<install>/Mods/*/About/About.xml`.
- **Workshop mods**: `<library>/steamapps/workshop/content/294100/<id>/About/About.xml`
  across **all** libraries.
- **Game version**: `<install>/Version.txt`, e.g. `1.6.4535 rev991` → major.minor = `1.6`.
- **Default savedata** (for import): Linux
  `~/.config/unity3d/Ludeon Studios/RimWorld by Ludeon Studios`; Windows
  `%USERPROFILE%\AppData\LocalLow\Ludeon Studios\RimWorld by Ludeon Studios`;
  macOS `~/Library/Application Support/RimWorld`.

## Launch

Steam-mediated, per OS (game args pass through after `-applaunch`):

- Linux: `steam -applaunch 294100 -savedatafolder=<abs path>` (spawn, don't wait)
- Windows: `<steamRoot>\steam.exe -applaunch 294100 -savedatafolder=<abs path>`
- macOS: `open -a Steam --args -applaunch 294100 -savedatafolder=<abs path>`

Only one instance can run at a time (Steam constraint); we don't police it.

## Shortcuts (per profile, exec Steam directly — RimForge not needed at run time)

- Linux: `~/.local/share/applications/rimforge-<slug>.desktop`
  (`Exec=steam -applaunch 294100 -savedatafolder=<path>`, `Name=RimWorld — <name>`)
- Windows: Start Menu `.lnk` via the `mslnk` crate targeting `steam.exe` with args.
- macOS: `~/Applications/RimWorld — <name>.app` stub: `Contents/Info.plist` +
  `Contents/MacOS/launch` shell script (chmod +x) running the `open` command above.

## XML formats

`ModsConfig.xml` (profile `Config/`):

```xml
<?xml version="1.0" encoding="utf-8"?>
<ModsConfigData>
  <version>1.6.4535 rev991</version>
  <activeMods><li>ludeon.rimworld</li><li>brrainz.harmony</li>…</activeMods>
  <knownExpansions><li>ludeon.rimworld.royalty</li>…</knownExpansions>
</ModsConfigData>
```

Package ids in `activeMods` are **lowercased**. When writing, preserve
`version` from the game (use detected game version string if creating fresh),
and set `knownExpansions` to the official expansion ids that are installed.

`About/About.xml` fields we read (all optional except packageId/name;
matching is case-insensitive, store ids lowercased):
`name`, `author`/`authors`, `packageId`, `supportedVersions/li`,
`modDependencies/li/packageId`, `loadAfter/li`, `loadBefore/li`,
`forceLoadAfter/li`, `forceLoadBefore/li`, `incompatibleWith/li`.
Parse with `roxmltree`. Malformed About.xml ⇒ skip mod, log via `eprintln!`.

## Community rules DB

RimSort's community rules database on GitHub (raw JSON). **Verify the exact
raw URL against the RimSort project before wiring it** — expected shape is a
top-level `rules` object keyed by lowercase packageId with `loadAfter`,
`loadBefore`, `loadBottom` entries (keys of the nested objects are the target
packageIds). Fetch with reqwest (rustls), cache to `cache/communityRules.json`;
refresh only when the frontend calls `refresh_rules_db` or cache is missing.
No cache and no network ⇒ sort proceeds with About.xml data only and returns a
warning (`kind: "rulesDbUnavailable"`).

## Auto-sort algorithm (`sort_mods`)

Input: the active id list. Output: sorted list + warnings. Pure — does not write.

1. Fixed tier ordering: `ludeon.rimworld` (Core) first, then installed official
   expansions in release order (royalty, ideology, biotech, anomaly, odyssey),
   then everything else, then any mod with a community `loadBottom` rule last.
2. Build a directed graph over active mods (edge A→B = A loads before B) from:
   About.xml `loadBefore`/`loadAfter`/`forceLoadBefore`/`forceLoadAfter`,
   `modDependencies` (dependency → dependent), community rules. Ignore edges
   referencing inactive/unknown ids.
3. Topological sort, deterministic: ties broken by mod display name
   (case-insensitive alphabetical). On a cycle, drop the lexicographically
   smallest edge, emit warning `kind: "cycle"`, continue.
4. Warnings additionally include: `missingDependency` (active mod depends on an
   id that is not active), `incompatible` (two active mods declare each other),
   `versionMismatch` (mod's supportedVersions lacks the game's major.minor),
   `unknownMod` (active id not found among installed mods).

## Command API (Tauri commands)

Registered in `src-tauri/src/lib.rs`. All are `async fn` returning
`Result<T, String>`. **Every struct crossing the boundary lives in
`src-tauri/src/models.rs` and derives
`Serialize/Deserialize` with `#[serde(rename_all = "camelCase")]` — no
exceptions.** TypeScript mirrors live in `src/lib/types.ts`; typed wrappers in
`src/lib/api.ts`. Tauri also camelCases command *arguments* on the JS side
(e.g. Rust `new_name` ⇒ JS `newName`).

| Command | Args | Returns | Notes |
|---|---|---|---|
| `reveal_path` | `path: String` | `()` | show in system file manager (opener plugin) |
| `get_settings` | — | `Settings` | |
| `update_settings` | `settings: Settings` | `Settings` | persists overrides |
| `detect_paths` | — | `DetectedPaths` | applies overrides on top of detection |
| `list_profiles` | — | `Profile[]` | |
| `create_profile` | `name: String` | `Profile` | empty savedatafolder + Config/ |
| `rename_profile` | `id, new_name: String` | `Profile` | display name only; slug/dir unchanged |
| `delete_profile` | `id: String` | `()` | move dir to OS trash (`trash` crate) |
| `clone_profile` | `id, new_name: String` | `Profile` | deep copy dir |
| `import_default` | `name: String` | `Profile` | copy default savedata's Config+Saves+Scenarios; never mutate source (it may be symlinked) — copy file *contents*, following symlinks |
| `launch_profile` | `id: String` | `()` | updates `lastPlayedAtMs` |
| `create_shortcut` | `id: String` | `String` | returns created path |
| `list_installed_mods` | — | `ModInfo[]` | official + local + workshop |
| `get_active_mods` | `profile_id: String` | `ActiveModList` | missing ModsConfig.xml ⇒ `["ludeon.rimworld"]` |
| `set_active_mods` | `profile_id: String, active_ids: Vec<String>` | `()` | writes ModsConfig.xml |
| `sort_mods` | `active_ids: Vec<String>` | `SortResult` | pure |
| `refresh_rules_db` | — | `RulesDbStatus` | force re-fetch |
| `get_rules_db_status` | — | `RulesDbStatus` | cache state only, no network |

## Shared types (authoritative Rust definitions in `models.rs`)

```
Settings        { steamRootOverride?, gameInstallOverride?, defaultSavedataOverride? : string|null }
DetectedPaths   { steamRoot?, gameInstall?, defaultSavedata?, workshopDirs: string[], gameVersion?, profilesDir }
Profile         { id, name, path, createdAtMs, lastPlayedAtMs?, saveCount, activeModCount }
ModSource       "official" | "local" | "workshop"           (serde rename_all = "lowercase")
ModInfo         { packageId, name, authors, path, source, steamWorkshopId?,
                  supportedVersions: string[], dependencies: string[],
                  loadAfter: string[], loadBefore: string[],
                  forceLoadAfter: string[], forceLoadBefore: string[],
                  incompatibleWith: string[] }
ActiveModList   { activeIds: string[], knownExpansions: string[], version? }
SortWarning     { kind, packageId?, message }                (kind is a plain string)
SortResult      { sorted: string[], warnings: SortWarning[] }
RulesDbStatus   { cached: bool, fetchedAtMs?, ruleCount }
```

## Module ownership (work streams — disjoint, do not edit outside your scope)

- **Supervisor-owned** (agents must NOT edit): `docs/ARCHITECTURE.md`,
  `src-tauri/src/lib.rs`, `src-tauri/src/models.rs`, `src-tauri/src/main.rs`,
  `src-tauri/Cargo.toml`, `src-tauri/tauri.conf.json`, `package.json`,
  `src/lib/types.ts`, `src/lib/api.ts`. Missing dep or contract gap ⇒ report
  back, don't self-serve.
- **Stream A (core backend)**: `src-tauri/src/paths.rs`, `profiles.rs`,
  `launch.rs`, `shortcuts.rs`, `settings.rs`.
- **Stream B (mods backend)**: `src-tauri/src/mods/` (`scan.rs`, `about.rs`,
  `modsconfig.rs`, `rules.rs`, `sort.rs`, plus `mod.rs` re-exports; `mod.rs` is
  pre-stubbed — replace stub bodies, keep signatures).
- **Stream C (frontend)**: everything under `src/` except `lib/types.ts` and
  `lib/api.ts`, plus `static/`.

Definition of done: Stream A/B ⇒ `cargo check` clean (warnings ok) in
`src-tauri/` and unit tests for pure logic (`cargo test`); Stream C ⇒
`npm run check` clean and `npm run build` succeeds.

## Platform notes

- `main.rs` sets `WEBKIT_DISABLE_DMABUF_RENDERER=1` on Linux before Tauri init
  (NVIDIA + Hyprland WebKitGTK crash workaround). Already done — leave it.
- Windows-only deps are target-gated in Cargo.toml.
- All fs paths crossing the boundary are absolute strings.
