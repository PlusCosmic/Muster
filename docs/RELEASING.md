# Releasing

RimForge has one release channel: an Arch package published to a private pacman
repository on Backblaze B2. There is no in-app updater, no local promotion
script, and no timer — a machine changes only when `pacman -Syu` runs.

## How it works

`.github/workflows/release.yml` runs the definition-of-done suite on every merge
to `main` — `npm run check` and `npm run build` in `frontend/`, a check that the
committed `frontend/bindings` match what `wails3 generate bindings` produces,
then `gofmt`, `go vet`, `go test` and a production `go build` with the `gtk3`
tag. Only if all of that passes does it send a `repository_dispatch` to
[PlusCosmic/packages], carrying the package name, this repository, the merged
commit SHA, and the version from `build/config.yml`.

The dispatch also refuses to run for anything but the current tip of `main`.
`pkgrel` is the packaging repository's run number, so *any* dispatch outranks
every package already published — a manual run launched from a feature branch,
or a re-run of an older `main` run, would hand unmerged or stale code a higher
`pkgrel` and offer it to clients as an upgrade. Nothing prunes a bad package
from the bucket, so recovering from one means bumping the version to supersede
it.

This repository never writes to the package bucket, and it holds no `PKGBUILD`.
Both live in the packaging repository, which is the sole publisher. That is
deliberate: the pacman database is a read-modify-write over shared object
storage, and GitHub's `concurrency` groups are repository-scoped, so two
projects publishing from their own repositories could each read the old index
and silently drop the other's package. Funnelling every update through one
repository gives one concurrency group that actually serialises them.

The packaging repository builds from `git archive` of the dispatched SHA, so a
package can only ever contain committed source — a dirty working tree is never
packaged. It installs the bare binary (the frontend embedded, no AppImage or
nfpm bundler), the hicolor icons, and
`/usr/share/applications/dev.pluscosmic.rimforge.desktop`. The build it runs is
the same one the workflow verifies:

```sh
cd frontend && npm ci && npm run build && cd ..
go build -tags gtk3,production -trimpath -buildvcs=false -ldflags="-w -s" -o rimforge .
```

with `go`, `nodejs`, `npm`, `webkit2gtk-4.1` and `gtk3` as build dependencies
(`webkit2gtk-4.1` and `gtk3` at run time). No `wails3` CLI is needed to build:
the bindings it generates are committed. The `%u` in the Wails-generated
`build/linux/desktop` is for URL handling this app does not do; the packaging
repository's own desktop file is the one that ships.

`pkgrel` is the packaging repository's Actions run number rather than a
hand-maintained counter, so each build is an upgrade to pacman even when the
`build/config.yml` version is unchanged. Bump that version for anything users
should recognise as a release; `frontend/package.json` tracks it but is not
what the dispatch reads.

## Versioning

`build/config.yml` (`info.version`) is the version of record. Keep
`frontend/package.json` and `internal/version/version.go` (the User-Agent sent
to the community rules database) in step with it when you bump — nothing
enforces that today.

## Installing

Client machines need the `[cosmic]` repository in `/etc/pacman.conf`; the server
line is recorded in [PlusCosmic/packages], which is private precisely because
that URL is the only thing keeping the package repository unlisted. This
repository is public, but the source is all rights reserved — see `LICENSE`.

```sh
sudo pacman -Sy rimforge
```

RimForge shells out to `steam` to launch the game and to detect the install and
Workshop paths, so Steam is an `optdepends` in name only — the app is not much
use without it. `RIMFORGE_DATA_DIR` still overrides the data root for a packaged
build, the same as for a development one.

## Setup

One secret, `PACKAGES_DISPATCH_TOKEN`: a fine-grained PAT with `contents: write`
on `PlusCosmic/packages`. `GITHUB_TOKEN` cannot dispatch across repositories.

Nothing is needed on the packaging side. This repository is public, so the
publish workflow checks it out with its own token; a private application
repository would need an `APP_CHECKOUT_TOKEN` there as well.

Everything else — the B2 credentials, the bucket path, the client `pacman.conf`
line and the `PKGBUILD` — lives in [PlusCosmic/packages].

Packages are unsigned, and clients set `SigLevel = Optional TrustAll` scoped to
that repository so it does not weaken verification for `core` or `extra`. The
tradeoff is explicit: obscurity hides the URL, but anyone who obtains write
access to the bucket can ship a package that runs as root on every client.

## Rehearsing a packaging change

`scripts/build-local.sh` in the packaging repository runs the same staging and
`makepkg` invocation the workflow does, against a local checkout of this one and
without publishing:

```sh
scripts/build-local.sh rimforge ~/Projects/RimForge
```

[PlusCosmic/packages]: https://github.com/PlusCosmic/packages
