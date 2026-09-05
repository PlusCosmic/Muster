# Muster

A cross-platform (Linux / Windows / macOS) desktop app for sharing mod setups
with friends and keeping them in sync. One shell, one game module per game:

- **RimWorld**: profiles — isolated `-savedatafolder` directories, each with its
  own mod list, mod settings and saves, sharing one installed game and Workshop
  library — with a mod list editor and RimSort-style auto-sort.
- **Minecraft**: enter a pack code (or a pack list URL) to get a shared packwiz
  modpack; Muster syncs it into its own directory, installs its mod loader into
  the official Minecraft launcher, and registers it there as a profile. Memory
  and JVM args are the pack's recommendation, tuned per machine. One Install
  button per pack; the launcher then has it preselected.

On first start Muster asks which games to show; the rail shows only those,
and games can be added or removed later from the button at the end of it.

Muster grew out of RimForge, its RimWorld-only predecessor, and adopts a
RimForge data directory on first start.

Built with Go + Wails 3 and Svelte 5.

## RimWorld module

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

Steam paths, the game install, Workshop folders, and game version are
auto-detected (`libraryfolders.vdf` aware, multi-library), with manual
overrides in Settings.

## Development

Requires Go 1.25+, Node 22+, the `wails3` CLI, and on Linux the
webkit2gtk-4.1 and gtk3 development packages.

```sh
go install -tags gtk3 github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.16
(cd frontend && npm install)
wails3 dev EXTRA_TAGS=gtk3      # run the app with live reload (Linux)
wails3 dev                      # Windows / macOS
```

The checks that gate a release, in the order the workflow runs them:

```sh
cd frontend && npm run check && npm run build && cd ..
wails3 generate bindings -ts -i -clean=true -d frontend/bindings -f "-tags gtk3"
gofmt -l .                      # must print nothing
go vet -tags gtk3 ./...
GOOS=windows go vet ./internal/... && GOOS=darwin go vet ./internal/...
go test -tags gtk3 ./...
go build -tags gtk3,production -trimpath -ldflags="-w -s" -o bin/muster .
```

`frontend/bindings` is generated from the Wails services (`app.go`,
`internal/rimworld/service.go`) and their models packages and is committed, so
`npm run check` works without a Go toolchain; regenerate it whenever a service
or a model changes (the workflow fails if it is stale). On Linux, Wails 3
defaults to GTK4/WebKitGTK 6.0; every Go command here passes `gtk3` to link
against webkit2gtk-4.1 instead. Arch packages both (`webkit2gtk-4.1` and
`webkitgtk-6.0`); the tag picks the one the release build and the Arch package
depend on. Windows and macOS need no tag.

During `npm run dev`, appending `?mock=1` to the URL swaps in a fixture
backend so the UI can be worked on in a plain browser.

`MUSTER_DATA_DIR` overrides the data root (`~/.local/share/muster` by
default) — used by the tests to avoid touching real data. RimForge's
`RIMFORGE_DATA_DIR` still works when the new variable is unset; the directory
it names is migrated in place on first start.

The backend↔frontend contract lives in `docs/ARCHITECTURE.md` and is binding:
if a command surface changes, that file changes in the same commit.

## Licence

All rights reserved — see `LICENSE`. The source is public to read, not to reuse.

## Releasing

Every merge to `main` runs the checks above and, if they pass, publishes an Arch
package to a private pacman repository. See `docs/RELEASING.md`.
