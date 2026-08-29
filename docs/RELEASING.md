# Releasing

RimForge has one release channel: an Arch package published to a private pacman
repository on Backblaze B2. There is no in-app updater, no local promotion
script, and no timer — a machine changes only when `pacman -Syu` runs.

## How it works

`.github/workflows/release.yml` runs the definition-of-done suite on every merge
to `main` — `npm run check`, `npm run build`, and `cargo check`, `clippy`,
`test --lib` and `fmt --check` in `src-tauri/`. Only if all of that passes does
it send a `repository_dispatch` to [PlusCosmic/packages], carrying the package
name, this repository, the merged commit SHA, and the version from
`src-tauri/tauri.conf.json`.

This repository never writes to the package bucket, and it holds no `PKGBUILD`.
Both live in the packaging repository, which is the sole publisher. That is
deliberate: the pacman database is a read-modify-write over shared object
storage, and GitHub's `concurrency` groups are repository-scoped, so two
projects publishing from their own repositories could each read the old index
and silently drop the other's package. Funnelling every update through one
repository gives one concurrency group that actually serialises them.

The packaging repository builds from `git archive` of the dispatched SHA, so a
package can only ever contain committed source — a dirty working tree is never
packaged. It installs the `--no-bundle` binary (the frontend embedded, no
deb/AppImage bundler), the hicolor icons, and
`/usr/share/applications/dev.pluscosmic.rimforge.desktop`.

`pkgrel` is the packaging repository's Actions run number rather than a
hand-maintained counter, so each build is an upgrade to pacman even when the
`tauri.conf.json` version is unchanged. Bump that version for anything users
should recognise as a release; `package.json` tracks it but is not what the
dispatch reads.

## Versioning

`src-tauri/tauri.conf.json` is the version of record. Keep `package.json` and
`src-tauri/Cargo.toml` in step with it when you bump — nothing enforces that
today.

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
