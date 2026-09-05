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
//	java/jre-21/         # a Temurin JRE, only if no usable Java was found
//	work/                # loader installer downloads and logs
package minecraft

import (
	"path/filepath"

	"muster/internal/appdir"
)

// Root is `<data>/muster/minecraft`.
func Root() string { return appdir.GameRoot(appdir.Minecraft) }

// PacksRoot is `<root>/packs`.
func PacksRoot() string { return filepath.Join(Root(), "packs") }

// PackDir is `<root>/packs/<id>` — does not check that it exists.
func PackDir(id string) string { return filepath.Join(PacksRoot(), id) }

// JavaRoot is `<root>/java`, where a downloaded JRE lives.
func JavaRoot() string { return filepath.Join(Root(), "java") }

// WorkDir is `<root>/work`, scratch space for loader installers.
func WorkDir() string { return filepath.Join(Root(), "work") }

// SettingsPath is `<root>/settings.json`.
func SettingsPath() string { return filepath.Join(Root(), "settings.json") }
