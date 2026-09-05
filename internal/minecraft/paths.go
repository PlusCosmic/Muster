// Package minecraft is the Minecraft game module: shared packwiz modpacks,
// pulled from a manifest, installed into their own directories and offered to
// the official Minecraft launcher as profiles. Service is its face to the
// frontend.
//
// App-owned layout (`<data>/muster/minecraft`):
//
//	settings.json        # manifest and .minecraft overrides
//	packs/<id>/          # one install (the profile's gameDir) per pack
//	  muster-pack.json   # what the last sync put there (packwiz.StateFile)
package minecraft

import (
	"path/filepath"

	"muster/internal/appdir"
)

// DefaultManifestURL is the manifest offered when the user has not set one.
// It is injected at build time (`-ldflags "-X muster/internal/minecraft.DefaultManifestURL=…"`)
// because a private pack's manifest URL is its privacy layer and this
// repository is public. Empty means "ask the user".
var DefaultManifestURL string

// Root is `<data>/muster/minecraft`.
func Root() string { return appdir.GameRoot(appdir.Minecraft) }

// PacksRoot is `<root>/packs`.
func PacksRoot() string { return filepath.Join(Root(), "packs") }

// PackDir is `<root>/packs/<id>` — does not check that it exists.
func PackDir(id string) string { return filepath.Join(PacksRoot(), id) }

// SettingsPath is `<root>/settings.json`.
func SettingsPath() string { return filepath.Join(Root(), "settings.json") }
