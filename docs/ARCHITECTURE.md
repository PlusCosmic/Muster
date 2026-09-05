# Muster — Architecture Contract

**This document is the binding contract between backend and frontend, and between
work streams. If the API surface changes, this file changes in the same commit.**

Muster is a cross-platform (Linux/Windows/macOS) desktop app for sharing mod
setups with a group of friends and keeping them in sync. It is a thin **shell**
plus one **game module** per game:

- **RimWorld**: *profiles* — self-contained `-savedatafolder` directories, each
  with its own `Config/` (active mod list + mod settings), `Saves/`, and
  `Scenarios/`. Installed mods (Steam Workshop + game `Mods/` + official
  Core/DLC) are shared between profiles; only the *active list* and settings
  differ.
- **Minecraft** (not built yet): shared modpacks pulled from a manifest and set
  up as profiles in the official Minecraft launcher.

Muster is the renamed and widened successor of RimForge, which was RimWorld
only. Nothing in the RimWorld module changed in the rename beyond where its
data lives.

Stack: Go + Wails 3 backend + Svelte 5 / TypeScript / SvelteKit static frontend
(`frontend/`, built to `frontend/dist` and embedded in the binary).

## Shell vs. game modules

The shell owns what every game needs and nothing game-specific:

- **Backend**: `internal/appdir` (data root, per-game roots, legacy migration),
  `internal/trash`, `internal/version`, `internal/xmldom` (generic XML tree),
  `internal/models` (game-neutral boundary types + the shared helpers
  `Str`/`Deref`/`Int64`/`NonNil`), and the `App` service in `app.go`.
- **Frontend**: `frontend/src/lib/shell/` — the game rail, brand mark, modal,
  dialog and toast hosts, context menu, icon set, the theme/titlebar/layout
  stores, localStorage helpers, and `api.ts` for the `App` service.

Each game module is self-contained under `internal/<game>/` (Go) and
`frontend/src/lib/<game>/` + `frontend/src/routes/<game>/` (frontend). It owns
its own boundary types (`internal/<game>/models`), its own Wails service
(`internal/<game>/service.go`), its own settings file, and its own sidebar and
main pane. There is deliberately **no shared mod/profile model** across games:
what a "mod" or a "profile" is differs too much between RimWorld and Minecraft
for a common abstraction to be more than a lowest common denominator.

The frontend routes are `/rimworld` and `/minecraft`; `/` redirects to the
last game used (localStorage `muster-game`). The rail
(`lib/shell/components/GameRail.svelte`) is the only navigation between them.

## Filesystem layout (app-owned data)

All app data lives under `<data>/muster` (`internal/appdir`): `$XDG_DATA_HOME`
or `~/.local/share` on Linux, `~/Library/Application Support` on macOS,
`%APPDATA%` on Windows. `MUSTER_DATA_DIR` replaces the whole root;
`RIMFORGE_DATA_DIR` (the predecessor's override) is honoured when the new one
is unset, and the directory it names is migrated in place.

```
<data>/muster/
  settings.json               # common settings (none yet; reserved)
  rimworld/                   # RimWorld game root (internal/rimworld/paths)
    profiles/<slug>/          # one savedatafolder per profile
      Config/ModsConfig.xml   # active mods (may not exist until first run/edit)
      Saves/  Scenarios/ ...
    registry.json             # ProfileMeta records (display name, timestamps)
    settings.json             # RimWorld Settings (path overrides)
    cache/communityRules.json # cached RimSort community rules DB
    cache/rules_meta.json     # { fetchedAtMs, etag? }
  minecraft/                  # Minecraft game root (reserved)
```

Slugs are derived from the display name: lowercase, `[a-z0-9-]`, collisions
suffixed `-2`, `-3`… A profile's identity is its slug (`id`).

### Migration from RimForge

RimForge kept exactly the RimWorld layout above directly under
`<data>/rimforge`. On startup, before anything reads the data root,
`appdir.MigrateLegacy` moves RimForge's four entries (`registry.json`,
`profiles/`, `cache/`, `settings.json`) from `<data>/rimforge` into
`<data>/muster/rimworld`, one rename each. Only those four: the RimForge
directory also holds WebKitGTK's own storage, keyed by program name, which is
left where it is. The move is resumable (the two entries that prove RimForge
data, registry and profiles, go last) and refuses rather than guesses when an
entry exists on both sides; a failure is shown in a native dialog and the app
exits so nothing is written to a half-moved root. Registry records hold slugs
only, never absolute paths, so nothing inside needs rewriting. An installation
that relocated its data with `RIMFORGE_DATA_DIR` keeps that directory as its
root: the same four entries move into a `rimworld/` subdirectory there.
Frontend preferences (theme, sidebar, title bar) live in the webview's own
storage, which also follows the program name, so they reset once.

## Command API (Wails services)

Services are bound with `application.NewService` in `main.go`. Each method
returns `(T, error)` or `error`; a non-nil error reaches the frontend as a
rejected promise carrying its message. **Every struct crossing the boundary
lives in a `models` package (`internal/models` for the app, `internal/<game>/models`
for a game) with camelCase `json` tags; optional values are pointers; list
fields are never nil (`models.NonNil`) — no exceptions.**
`wails3 generate bindings` writes the TypeScript for every service and models
package to `frontend/bindings` (committed; regenerate when either changes).
Each game's `frontend/src/lib/<game>/types.ts` narrows the generated
`T[] | null` lists back to `T[]`, and its `api.ts` holds the typed wrappers.

### App service (`app.go`, game-neutral)

| Method | Args | Returns | Notes |
|---|---|---|---|
| `RevealPath` | `path` | — | show in system file manager (`Env.OpenFileManager`) |
| `GetAppInfo` | — | `AppInfo` | `{ version, dataRoot }` |

### RimWorld service (`internal/rimworld/service.go`)

| Method | Args | Returns | Notes |
|---|---|---|---|
| `GetSettings` | — | `Settings` | |
| `UpdateSettings` | `settings: Settings` | `Settings` | persists overrides |
| `DetectPaths` | — | `DetectedPaths` | applies overrides on top of detection |
| `ListProfiles` | — | `Profile[]` | |
| `CreateProfile` | `name` | `Profile` | empty savedatafolder + Config/ |
| `RenameProfile` | `id, newName` | `Profile` | display name only; slug/dir unchanged |
| `DeleteProfile` | `id` | — | move dir to OS trash (`internal/trash`) |
| `CloneProfile` | `id, newName` | `Profile` | deep copy dir |
| `ImportDefault` | `name` | `Profile` | copy default savedata's Config+Saves+Scenarios; never mutate source (it may be symlinked) — copy file *contents*, following symlinks |
| `LaunchProfile` | `id` | — | updates `lastPlayedAtMs` |
| `ListInstalledMods` | — | `ModInfo[]` | official + local + workshop |
| `GetActiveMods` | `profileId` | `ActiveModList` | missing ModsConfig.xml ⇒ `["ludeon.rimworld"]` |
| `SetActiveMods` | `profileId, activeIds: string[]` | — | writes ModsConfig.xml |
| `SortMods` | `activeIds: string[]` | `SortResult` | pure |
| `RefreshRulesDb` | — | `RulesDbStatus` | force re-fetch |
| `GetRulesDbStatus` | — | `RulesDbStatus` | cache state only, no network |

## Shared types

App (`internal/models/models.go`):

```
AppInfo         { version, dataRoot }
```

RimWorld (`internal/rimworld/models/models.go`):

```
Settings        { steamRootOverride?, gameInstallOverride?, defaultSavedataOverride? : string|null }
DetectedPaths   { steamRoot?, gameInstall?, defaultSavedata?, workshopDirs: string[], gameVersion?, profilesDir }
Profile         { id, name, path, createdAtMs, lastPlayedAtMs?, saveCount, activeModCount }
ModSource       "official" | "local" | "workshop"           (string enum)
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

## RimWorld module

Go packages under `internal/rimworld/`: `paths` (app-owned layout + external
detection), `settings`, `profiles`, `mods` (`scan.go`, `about.go`,
`modsconfig.go`, `rules.go`, `sort.go`), `launch`, `models`, and `service.go`.
`paths` must not import `settings`' dependants: `settings` reads its file path
through `appdir` directly to keep `paths → settings` acyclic.

### External paths (detected, overridable in settings)

- **Steam root**: Linux `~/.steam/steam` or `~/.local/share/Steam`; Windows via
  `HKLM/HKCU SOFTWARE\Valve\Steam InstallPath` falling back to
  `C:\Program Files (x86)\Steam`; macOS `~/Library/Application Support/Steam`.
- **Library folders**: parse `<steam>/steamapps/libraryfolders.vdf`. Do not add
  a VDF dependency: extract with a regex for `"path"\s+"(...)"` (unescape `\\`).
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

### Launch

Steam-mediated, per OS (game args pass through after `-applaunch`):

- Linux: `steam -applaunch 294100 -savedatafolder=<abs path>` (spawn, don't wait)
- Windows: `<steamRoot>\steam.exe -applaunch 294100 -savedatafolder=<abs path>`
- macOS: `open -a Steam --args -applaunch 294100 -savedatafolder=<abs path>`

Only one instance can run at a time (Steam constraint); we don't police it.

### XML formats

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
Parse with `internal/xmldom` (a case-insensitive element tree over
`encoding/xml`). Malformed About.xml ⇒ skip mod, log to stderr.

### Community rules DB

RimSort's community rules database on GitHub (raw JSON). **Verify the exact
raw URL against the RimSort project before wiring it** — expected shape is a
top-level `rules` object keyed by lowercase packageId with `loadAfter`,
`loadBefore`, `loadBottom` entries (keys of the nested objects are the target
packageIds). Fetch with `net/http` (ETag-aware), cache to `cache/communityRules.json`;
refresh only when the frontend calls `refresh_rules_db` or cache is missing.
No cache and no network ⇒ sort proceeds with About.xml data only and returns a
warning (`kind: "rulesDbUnavailable"`).

### Auto-sort algorithm (`sort_mods`)

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

## Module ownership (work streams — disjoint, do not edit outside your scope)

- **Supervisor-owned** (agents must NOT edit): `docs/ARCHITECTURE.md`,
  `app.go`, `main.go`, every `models` package, each game's `service.go`,
  `go.mod`, `build/config.yml`, `Taskfile.yml`, `frontend/package.json`,
  each game's `frontend/src/lib/<game>/types.ts` and `api.ts`,
  `frontend/src/lib/shell/api.ts`, and the generated `frontend/bindings/`.
  Missing dep or contract gap ⇒ report back, don't self-serve.
- **Shell**: `internal/appdir`, `internal/trash`, `internal/version`,
  `internal/xmldom`, and `frontend/src/lib/shell/`, `frontend/src/routes/+layout.*`,
  `frontend/src/routes/+page.ts`, `frontend/src/app.css`.
- **RimWorld backend**: `internal/rimworld/{paths,settings,profiles,launch,mods}`.
- **RimWorld frontend**: `frontend/src/lib/rimworld/` (except `types.ts`,
  `api.ts`) and `frontend/src/routes/rimworld/`.
- **Minecraft**: `internal/minecraft/`, `frontend/src/lib/minecraft/`,
  `frontend/src/routes/minecraft/` (reserved).

Definition of done: backend ⇒ `go vet -tags gtk3 ./...` clean, `gofmt -l .`
empty, and unit tests for pure logic (`go test -tags gtk3 ./...`); frontend ⇒
`npm run check` clean and `npm run build` succeeds in `frontend/`.

## Platform notes

- `main.go` sets `WEBKIT_DISABLE_DMABUF_RENDERER=1` on Linux before the app
  starts (NVIDIA + Hyprland WebKitGTK crash workaround). Wails only does this
  itself when it detects an NVIDIA GPU. Already done — leave it.
- Linux builds use the `gtk3` build tag (webkit2gtk-4.1); Wails 3 otherwise
  targets GTK4 + WebKitGTK 6.0, a separate package (`webkitgtk-6.0` on Arch)
  that the release build does not depend on.
- Windows-only code (registry, recycle bin) lives in `_windows.go` files.
- All fs paths crossing the boundary are absolute strings.
