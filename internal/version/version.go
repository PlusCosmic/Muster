// Package version holds the application version and the build-time knobs
// that tell a build where it came from.
//
// build/config.yml is the version of record; keep Version in step with it
// when bumping (nothing enforces that today). Release builds must use the
// same string the update manifest is published under.
package version

// Version is the running version, without a "v" prefix.
const Version = "0.2.0"

// UpdateManifestURL is where the self-updater looks for newer releases: the
// signed Wails update manifest attached to the latest GitHub release. Public
// infrastructure, so it lives in source. Whether a build uses it is decided
// at runtime (see main.go): platforms with a package manager do not.
const UpdateManifestURL = "https://github.com/PlusCosmic/Muster/releases/latest/download/stable.json"
