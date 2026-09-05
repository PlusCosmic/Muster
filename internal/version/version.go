// Package version holds the application version and the build-time knobs
// that tell a build where it came from.
//
// build/config.yml is the version of record; keep Version in step with it
// when bumping (nothing enforces that today). Release builds must use the
// same string the update manifest is published under.
package version

// Version is the running version, without a "v" prefix.
const Version = "0.2.0"

// UpdateManifestURL is where the self-updater looks for newer releases: a
// Wails update manifest (`wails3 updater manifest`). Injected at build time
// (`-ldflags "-X muster/internal/version.UpdateManifestURL=…"`) by the
// release pipeline for platforms that update themselves; empty means the
// updater is off, which is right for dev builds and for the Arch package,
// where pacman does the updating.
var UpdateManifestURL string
