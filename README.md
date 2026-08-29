# RimForge

A cross-platform (Linux / Windows / macOS) desktop manager for RimWorld
**profiles** — isolated `-savedatafolder` directories, each with its own mod
list, mod settings, and saves, sharing one installed game and Workshop library.

Built with Tauri 2 (Rust) + Svelte 5.

## What it does

- **Profiles**: create, rename, clone, delete (to system trash), and import
  your current vanilla RimWorld setup as a starting profile.
- **Launch through Steam**: each profile launches via
  `steam -applaunch 294100 -savedatafolder=<profile>` so overlay and playtime
  keep working.
- **Mod list editor**: two-column active/inactive editor with drag-and-drop
  ordering, search, and per-profile `ModsConfig.xml` persistence.
- **Auto-sort**: RimSort-style tiered topological sort using each mod's
  About.xml constraints plus the
  [RimSort Community Rules Database](https://github.com/RimSort/Community-Rules-Database)
  (cached locally, works offline). Warnings for missing dependencies,
  incompatibilities, version mismatches, and ordering cycles.
- **Native shortcuts**: generate a per-profile launcher (`.desktop` / Start
  Menu `.lnk` / macOS `.app` stub) that starts the game directly — RimForge
  doesn't need to be running.

Steam paths, the game install, Workshop folders, and game version are
auto-detected (`libraryfolders.vdf` aware, multi-library), with manual
overrides in Settings.

## Development

```sh
npm install
npm run tauri dev      # run the app
npm run check          # svelte-check
cargo test             # backend tests (run in src-tauri/)
npm run tauri build    # release bundles
```

`RIMFORGE_DATA_DIR` overrides the data root (`~/.local/share/rimforge` by
default) — used by the profile-lifecycle tests to avoid touching real data.

The backend↔frontend contract lives in `docs/ARCHITECTURE.md` and is binding:
if the command surface changes, that file changes in the same commit.

## Licence

All rights reserved — see `LICENSE`. The source is public to read, not to reuse.

