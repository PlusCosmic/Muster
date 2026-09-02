# Feasibility: porting RimForge to Go + Wails

**Status: done.** The port landed on this branch; what follows is the
assessment it was made against, kept for the reasoning, with a short record of
where reality differed.

## What actually happened

- **Wails v3 beta.16** on Linux defaults to GTK4 + WebKitGTK 6.0, a separate
  package from the webkit2gtk-4.1 the Tauri build used. The port keeps the
  same runtime dependency by passing the `gtk3` build tag everywhere; the
  root `Taskfile.yml` defaults it on Linux. The `wails3` CLI itself has
  to be installed with `-tags gtk3` too.
- **The decorations toggle survived.** v3's runtime exposes
  `Window.SetFrameless`, so the Settings switch ports unchanged.
- **The frontend moved into `frontend/`.** Wails' Taskfiles hardcode that
  layout and `frontend/dist`; keeping SvelteKit at the repository root would
  also have collided with Wails' `build/` asset directory.
- **Bindings are committed.** `wails3 generate bindings` output replaces the
  hand-mirrored `types.ts`; the workflow regenerates and diffs it so the
  frontend is always checked against the real API. A wrapper layer narrows the
  generated `T[] | null` lists to `T[]`, which the Go side guarantees.
- **Trash needed no dependency.** Linux tries `gio trash` then the freedesktop
  home trash; macOS asks Finder; Windows uses the recycle bin via PowerShell.
- **The Rust tests came across** (57 of 63; the six for shortcuts went with
  the feature) plus a handful of new ones, and the previously `#[ignore]`d
  profile-lifecycle tests now run unconditionally against a scratch data dir.
- **Build time:** the production binary compiles in about 16 s cold.

---

**Verdict: feasible, moderate effort.** The frontend carries over almost
untouched; the work is a Rust-to-Go rewrite of the backend and a change to the
packaging pipeline. Roughly one to two weeks of focused solo work.

Assumes the **native shortcuts feature is dropped** as part of the port. It is
the one piece with no good Go equivalent on Windows, and it is small and
rarely used. See "What is dropped".

## Current shape

| Layer | Size | Notes |
|---|---|---|
| Frontend | ~4,500 lines Svelte 5 / TypeScript / SvelteKit static | Two files touch Tauri |
| Backend | ~3,500 lines Rust, 18 commands, 63 unit tests | Filesystem, XML, HTTP, one pure sort |
| Packaging | Arch package via `PlusCosmic/packages`, `cargo tauri build --no-bundle` | See `RELEASING.md` |

## Frontend

Wails serves any static frontend and SvelteKit with `adapter-static` is a
supported configuration. Components, stores, CSS, the responsive shell, focus
handling and the `?mock=1` fixture backend all carry over unchanged.

Only two files reference Tauri:

- `src/lib/api.ts` wraps `invoke`. This is replaced wholesale by the
  TypeScript bindings Wails generates from the bound Go struct.
- `src/lib/stores/titlebar.svelte.ts` toggles window decorations at runtime.

Wails generating the bindings also retires the hand-mirrored
`src/lib/types.ts`: the Go structs become the single source of truth, which is
an improvement over the current "keep them in lockstep" rule in
`ARCHITECTURE.md`.

**Check early: the decorations toggle.** Wails v3's window API exposes
runtime window controls, which v2 did not. Confirm in the first day that the
frameless state can be flipped from the frontend after the window exists. If
it can, the Settings switch ports as-is; if not, make it a restart-required
setting. Either way this is the only frontend-visible behaviour change.

## Backend

Every command in the `ARCHITECTURE.md` table maps to a method on one bound Go
struct. Most of the code is directory walking, XML parsing, JSON persistence and
a pure topological sort, all of which is idiomatic Go.

| Rust crate | Go replacement | Effort |
|---|---|---|
| `roxmltree` | `encoding/xml` (or `etree`) | Easy. Case-insensitive element matching is done by hand, as it is now. |
| `reqwest` + ETag cache | `net/http` | Easy. |
| `winreg` | `golang.org/x/sys/windows/registry` | Easy. |
| `dirs` | `os.UserConfigDir` plus a small XDG data-dir helper | Easy. Go has no direct `data_dir()`; keep the `RIMFORGE_DATA_DIR` override. |
| `fs_extra` deep copy | `filepath.WalkDir` copy that follows symlinks | Easy. Preserve the "copy contents, never touch the source" rule for `import_default`. |
| `serde` camelCase | struct tags | Mechanical. |
| `regex` (libraryfolders.vdf) | `regexp` | Trivial. |
| `trash` | small third-party module, or per-OS shell-out (`gio trash`, PowerShell, `osascript`) | Medium. No stdlib support; pick one and test on each OS. |
| `mslnk` | none needed | Gone with the shortcuts feature. |

The sort in `src-tauri/src/mods/sort.rs` is pure and has 12 tests; port the
tests first and the algorithm against them. The remaining tests cover paths,
profiles, About.xml parsing and ModsConfig.xml round-trips and should all
translate directly to `go test`.

About 29 `cfg(...)` sites become build tags or `runtime.GOOS` switches.

## Platform and packaging

- **Linux still renders with WebKitGTK**, so the NVIDIA DMA-BUF workaround in
  `src-tauri/src/main.rs` is still required. Set
  `WEBKIT_DISABLE_DMABUF_RENDERER=1` from Go before `wails.Run`. Same engine
  means the UI behaves identically to today.
- **Arch packaging changes.** `.github/workflows/release.yml` and the
  `PKGBUILD` in `PlusCosmic/packages` swap the cargo checks and build for
  `go vet`, `go test`, and `wails3 build`. v3 drives builds through a
  Taskfile, so the PKGBUILD either calls `wails3` or the underlying `task`
  targets directly. v3 links webkit2gtk-4.1 on Linux, which is what current
  Arch ships. Toolchain becomes Go + npm + webkit2gtk instead of Rust + npm +
  webkit2gtk.
- **Version of record** moves from `tauri.conf.json` to the v3 build config;
  update the dispatch step in `release.yml` accordingly.
- **macOS and Windows** builds still need their native hosts. cgo means Wails
  does not cross-compile any better than Tauri; this is a wash.
- **Build speed** improves substantially. Go's compile times are the strongest
  practical argument for the port.

## What is dropped

- **Native shortcuts** (`create_shortcut`, `src-tauri/src/shortcuts.rs`, its
  six tests, the frontend action and Settings copy). The Linux `.desktop` and
  macOS `.app` stub variants were trivial; only the Windows `.lnk` writer had
  no clean Go answer. Dropping the whole feature keeps the port uniform across
  platforms. Remove the command from the `ARCHITECTURE.md` table in the same
  commit.
- **Tauri's capability/permission model.** RimForge only grants
  `core:default`, `set-decorations` and `opener`, so nothing is lost in
  practice.
- **Rust's type system** over the filesystem and parsing code. The ported test
  suite is the mitigation.

## Wails version

Target **v3**. It is in beta with a stable desktop API and is already used in
production; the remaining work upstream is polish, not API change. Its
binding model is a good fit here: the backend becomes one service struct
registered with the application, and `wails3 generate bindings` emits the
TypeScript that replaces `api.ts` and `types.ts`.

Test the packaged binary on each OS before shipping, per the upstream advice,
and pin the v3 version in `go.mod` so a beta bump cannot land through a plain
`go get -u`.

## Suggested order

1. Scaffold `wails3 init` with the existing `src/` as the frontend; get the
   mock backend rendering. Verify the decorations toggle here.
2. Port `models` and the pure sort with its tests.
3. Port paths, settings, profiles, launch; run `wails3 generate bindings`
   and wire the output into `backend.ts`.
4. Port mod scanning, About.xml, ModsConfig.xml, rules DB.
5. Remove shortcuts from frontend and `ARCHITECTURE.md`.
6. Rewrite `release.yml` and the packaging `PKGBUILD`.

## Bottom line

Nothing in RimForge is hard to express in Go, and the frontend is almost
entirely portable. Do it for Go's build speed and ecosystem familiarity. Do not
do it expecting a smaller binary, better performance or wider platform reach,
because those do not change.
