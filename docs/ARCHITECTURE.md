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
- **Minecraft**: shared packwiz modpacks pulled from a *manifest*, installed
  into their own directories, and offered to the official Minecraft launcher
  as profiles. The launcher does auth, Java, assets and the actual launch; we
  do the pack.

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
  minecraft/                  # Minecraft game root (internal/minecraft)
    settings.json             # manifest + .minecraft overrides
    packs/<id>/               # one install per pack = its launcher profile's gameDir
      muster-pack.json        # what the last sync put there (packwiz.StateFile)
    java/jre-21/              # Temurin JRE, only when no usable Java was found
    work/                     # loader installer downloads and logs
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
| `GetAppInfo` | — | `AppInfo` | `{ version, dataRoot, selfUpdates }` |
| `CheckForUpdates` | — | `bool` | is a newer release available; when it is, the update window opens and installs it; a failed check is an error; always false unless `selfUpdates` |

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

### Minecraft service (`internal/minecraft/service.go`)

| Method | Args | Returns | Notes |
|---|---|---|---|
| `GetSettings` | — | `Settings` | |
| `UpdateSettings` | `settings: Settings` | `Settings` | persists overrides |
| `Detect` | — | `Detected` | effective manifest URL, `.minecraft`, launcher presence |
| `ListPacks` | — | `Pack[]` | manifest entries + local install state; network: manifest only |
| `CheckPack` | `id` | `PackCheck` | loads the pack, plans a sync, reports counts; writes nothing |
| `SyncPack` | `id` | `SyncReport` | download/delete to match the pack, install the loader if the launcher lacks it, then write the launcher profile; emits `minecraft:sync` (`SyncProgress`) per file and per loader step |
| `GetLaunchSettings` | `id` | `LaunchSettings` | what the pack launches with on this machine |
| `SetLaunchSettings` | `id, settings: LaunchSettings` | `LaunchSettings` | save (clamped to this machine) and rewrite the launcher profile if it exists |
| `ResetLaunchSettings` | `id` | `LaunchSettings` | forget the user's settings; back to the recommendation fitted to this machine |
| `OpenLauncher` | — | — | start the official launcher |
| `LauncherRunning` | — | `bool` | is it already open (a profile written since only shows after a restart) |

## Shared types

App (`internal/models/models.go`):

```
AppInfo         { version, dataRoot, selfUpdates }
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

Minecraft (`internal/minecraft/models/models.go`):

```
Settings        { manifestUrl?, minecraftDirOverride?, packs: { [id]: LaunchSettings } }
LaunchSettings  { maxMemoryMb, minMemoryMb?, args: string[], followRecommendedArgs }
Detected        { manifestUrl?, minecraftDir?, launcherInstalled, packsDir, totalMemoryMb, maxHeapMb }
Pack            { id, name, description, icon?, packUrl, server?,
                  recommendedMinMemoryMb, recommendedMaxMemoryMb, recommendedArgs: string[],
                  launch: LaunchSettings, launchCustomised,
                  installDir, installed, installedVersion?, syncedAtMs?, profileWritten }
PackCheck       { id, latestVersion, minecraft, loader, loaderVersion, versionId,
                  loaderInstalled, toDownload, toDelete, upToDate }
SyncProgress    { id, phase: "files"|"loader"|"profile", done, total, current }   (event payload)
Manual          { path, name, url, why }
SyncReport      { id, version, downloaded: string[], deleted: string[], manual: Manual[],
                  profileWritten, loaderInstalled, versionId, launcherOpen }
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

## Minecraft module

Go packages under `internal/minecraft/`: `manifest` (the pack list),
`packwiz` (pack format, hashing, download resolution, sync planner/applier),
`launcher` (`.minecraft` location, `launcher_profiles.json` merge-writes,
opening the launcher), `loader` (installing a loader into the launcher),
`java` (a runtime to run loader installers with), `models`, and the service,
settings and layout in the package root.

### Manifest

One JSON document at a URL the pack author controls:

```jsonc
{ "packs": [ {
    "id": "frontier",                         // [a-z0-9-], names packs/<id>
    "name": "Frontier",
    "description": "…", "icon": "https://…",  // optional
    "pack": "https://…/<slug>/pack.toml",     // the packwiz pack
    "recommended": { "minMemoryMb": 4096, "maxMemoryMb": 8192, "args": ["-XX:+UseZGC"] },
    "server": "play.example.com"              // optional
} ] }
```

Minecraft version and loader come from `pack.toml`, never from the manifest.
Muster ships with no manifest and knows about no particular pack: the user
pastes the URL of the list they were given (first-run card or Settings), and
it is stored in the module's `settings.json`. Nothing in this repository or
in any build names a pack; that keeps the app usable by anyone with a
packwiz pack to share, and keeps a private pack's URL out of a public repo.

`recommended` is advice, not configuration (the older key `java` is still
read as the same thing): see "Launch settings" below.

### Sync

`packwiz.Client.Load` reads `pack.toml` → `index.toml` (hash-verified against
pack.toml) → every `.pw.toml` (hash-verified against the index), keeping only
client-side files (`side` = both/client). Download URLs are the metafile's
`[download] url`, or for `mode = "metadata:curseforge"` the CurseForge CDN
pattern `edge.forgecdn.net/files/<id/1000>/<id%1000>/<filename>`; a
CurseForge file the CDN refuses (403/404) is reported in `SyncReport.manual`
rather than failing the sync. Every download is hash-verified before it lands.

`Load` also refuses a pack whose entries would collide (two metafiles
installing the same filename), name `muster-pack.json`, or escape the install
directory (index path and entries alike), and one with no
`[versions] minecraft`.

`MakePlan` diffs the resolved pack against `packs/<id>/muster-pack.json`, which
records the path, hash and size of every file the last sync wrote: missing,
changed, or no longer matching (a size mismatch, or a hash mismatch when the
recorded stamp predates sizes) ⇒ download; recorded but no longer in the pack
⇒ delete; anything the user added themselves is never touched. The size
check is the compromise that keeps the on-open check fast; a corruption that
preserves the size is not caught. `preserve = true` files are not overwritten
once installed. Optional files follow their pack default. `Apply` streams
each download through the hash into a temp file beside its target and writes
the state after every file, so an interrupted sync resumes exactly. A
CurseForge refusal is reported with the project page URL, not the failed CDN
link.

### Loader install

Between files and profile, `SyncPack` makes sure `.minecraft/versions/<id>/`
exists for the pack's loader (`loader.Installer.Ensure`), and does nothing
when it already does:

- **Fabric / Quilt**: fetch the launcher profile JSON from
  `meta.fabricmc.net` / `meta.quiltmc.org`
  (`/versions/loader/<mc>/<loader>/profile/json`) and write it as
  `versions/<id>/<id>.json`. The launcher fetches vanilla and the libraries
  on first play. No Java needed.
- **NeoForge / Forge**: download the installer jar from the loader's Maven
  and run `java -jar <installer> --install-client <.minecraft>` (verified on
  neoforge-21.1.248: headless, downloads vanilla itself, ~9 s). The installer
  needs `launcher_profiles.json` to exist (created if not) and injects a
  "NeoForge" profile, which is removed again — per profiles file, comparing
  each file's keys before and after — so friends' dropdowns show only packs.
  Logs land in `work/`. A Fabric/Quilt profile JSON must carry the expected
  `id`; an id-less document is refused rather than installed.

Java for the installer (`java.Ensure`), in order: the launcher's own bundled
runtime — `<.minecraft>/runtime/<component>/<platform>/<component>/bin/java`
for the standalone launcher, and on Windows the Store launcher's package cache
`%LOCALAPPDATA%\Packages\Microsoft.4297127D64EC6_8wekyb3d8bbwe\LocalCache\Local\runtime`
(verified on Store build 2.6.2); components are ranked alpha < beta < gamma <
delta < epsilon…, newest first, and each candidate must answer `-version`
with 17+ (the launcher also keeps jre-legacy, Java 8) — then a JRE we
downloaded before (`jre-21/bin/java`, or `jre-21/Contents/Home/bin/java` for
Temurin's macOS bundles), then a `java` on PATH that is 17+, and last a
Temurin 21 JRE from the Adoptium API into `java/jre-21/`.

### Launcher profile

### Launch settings

Every machine differs, so a pack's `recommended` heap and JVM args are shown
and offered, never imposed. `internal/minecraft/machine` reads the machine's
physical memory (`/proc/meminfo`, `GlobalMemoryStatusEx`, `sysctl
hw.memsize`) and offers at most three quarters of it (`Detected.maxHeapMb`,
on 512 MB steps, floor 1 GB). With nothing saved, a pack launches with its
recommended maximum clamped to that, no `-Xms`, and the recommended args
(`Pack.launch`, `launchCustomised = false`). The user can move the heap
slider, reserve the heap up front (`minMemoryMb = maxMemoryMb`), and edit
the args; saved settings live in `settings.json` under `packs.<id>` and
`followRecommendedArgs` keeps args tracking the pack until first edited.
`SetLaunchSettings` also rewrites the launcher profile's `javaArgs` when a
profile exists, so a change applies on the next launch without a sync. Args
containing whitespace are refused everywhere, since the launcher splits its
string without quoting.

`SyncPack` then upserts `profiles["muster-<id>"]` in
`.minecraft/launcher_profiles.json` (and the Store variant when present) with
`gameDir = packs/<id>`, `lastVersionId` = the loader's installation id
(`neoforge-<v>`, `fabric-loader-<v>-<mc>`, `<mc>-forge-<v>`; validated before
any file moves), `javaArgs` rendered from the effective launch settings
(`-Xms` only when reserved, `-Xmx`, then the args), and `lastUsed = now` so
the launcher preselects it. Writes are read-merge-write over raw JSON: every other profile and
top-level key is preserved byte for byte, and on our own profile only the
fields Muster owns are written, so a resolution or javaDir the user set in
the launcher survives a sync. On Linux the Flatpak launcher's
`~/.var/app/com.mojang.Minecraft/.minecraft` is used when it, rather than
`~/.minecraft`, is the one that has run. The launcher only reads this
file on start (verified on the Store build: a profile added while it was open
showed after a relaunch, nothing lost), so `SyncReport.launcherOpen` and
`LauncherRunning` report when `launcher.Running()` sees a launcher process
(`Minecraft.exe` / `MinecraftLauncher.exe` on Windows, `minecraft-launcher`
/ `Minecraft` elsewhere) and the UI tells the user to close and reopen it.

`PackCheck.loaderInstalled` reports whether `.minecraft/versions/<versionId>/`
already exists, i.e. whether a sync would need to install it.

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
- **Minecraft backend**: `internal/minecraft/{manifest,packwiz,launcher,loader,java,machine}`
  and the non-service files in `internal/minecraft/`.
- **Minecraft frontend**: `frontend/src/lib/minecraft/` (except `types.ts`,
  `api.ts`) and `frontend/src/routes/minecraft/`.

Definition of done: backend ⇒ `go vet -tags gtk3 ./...` clean, `gofmt -l .`
empty, and unit tests for pure logic (`go test -tags gtk3 ./...`); frontend ⇒
`npm run check` clean and `npm run build` succeeds in `frontend/`.

## Self-update

Builds for platforms without a package manager (Windows today, macOS when it
ships) update themselves through Wails' updater (`pkg/updater`) with the
`endpoint` provider: the signed Wails update manifest `stable.json` attached
to the latest GitHub release, at `version.UpdateManifestURL`
(`github.com/PlusCosmic/Muster/releases/latest/download/stable.json`, a
stable URL GitHub redirects to the newest release's asset). `main.go` leaves
it off on Linux, where the package manager updates the app, and when
`MUSTER_NO_SELF_UPDATE` is set (dev); then `AppInfo.selfUpdates` is false.

Trust: every artifact is signed (ed25519ph over its sha512) with a key held
only by the release pipeline; the matching public key is `build/updater.pub`,
embedded in the binary and pinned as the updater's only trust root. The
manifest URL cannot substitute a key.

Behaviour: the Updater's own periodic loop opens the built-in window even
when up to date, so `setupUpdater` runs a silent `Check` ten seconds after
start and every six hours, and calls `CheckAndInstall` (which opens the
window, downloads, verifies, and offers Restart & Apply) only when a release
is found. The settings modals' About section offers a manual check.
`version.Version` is the version of record for the comparison and must match
the version the manifest is published under.

## Platform notes

- `main.go` sets `WEBKIT_DISABLE_DMABUF_RENDERER=1` on Linux before the app
  starts (NVIDIA + Hyprland WebKitGTK crash workaround). Wails only does this
  itself when it detects an NVIDIA GPU. Already done — leave it.
- Linux builds use the `gtk3` build tag (webkit2gtk-4.1); Wails 3 otherwise
  targets GTK4 + WebKitGTK 6.0, a separate package (`webkitgtk-6.0` on Arch)
  that the release build does not depend on.
- Windows-only code (registry, recycle bin) lives in `_windows.go` files.
- All fs paths crossing the boundary are absolute strings.
