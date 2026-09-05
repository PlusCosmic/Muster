// Package appdir locates the app-owned data root and the per-game roots
// beneath it:
//
//	<data>/muster/
//	  settings.json        # common settings (none yet)
//	  rimworld/            # everything the RimWorld game module owns
//	  minecraft/           # everything the Minecraft game module owns
//
// What lives inside a game root is that module's business (see
// internal/rimworld/paths for RimWorld's layout). MUSTER_DATA_DIR, when set,
// replaces the whole root. It exists so tests (and anyone relocating their
// data) can point the app at another disk. RIMFORGE_DATA_DIR, the
// predecessor's equivalent, is honoured as a fallback so an installation that
// relocated its data keeps finding it; see MigrateLegacy.
package appdir

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Game identifies a game module's subtree under the data root.
type Game string

const (
	RimWorld  Game = "rimworld"
	Minecraft Game = "minecraft"
)

// dirName is the app's directory under the platform data directory.
const dirName = "muster"

// legacyDirName is where RimForge, the RimWorld-only predecessor, kept its
// data. See MigrateLegacy.
const legacyDirName = "rimforge"

// EnvDataDir is the environment variable that replaces the data root.
const EnvDataDir = "MUSTER_DATA_DIR"

// EnvLegacyDataDir is RimForge's data-root override. When set and EnvDataDir
// is not, it is the data root: the directory it names is migrated in place.
const EnvLegacyDataDir = "RIMFORGE_DATA_DIR"

// dataDir mirrors the Rust `dirs::data_dir()` this app was built on:
// $XDG_DATA_HOME (~/.local/share) on Linux, ~/Library/Application Support on
// macOS and %APPDATA% on Windows.
func dataDir() (string, bool) {
	switch runtime.GOOS {
	case "linux":
		if v := os.Getenv("XDG_DATA_HOME"); v != "" {
			return v, true
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share"), true
		}
		return "", false
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support"), true
		}
		return "", false
	default:
		if dir, err := os.UserConfigDir(); err == nil {
			return dir, true
		}
		return "", false
	}
}

func baseDir() string {
	base, ok := dataDir()
	if !ok {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return "."
	}
	return base
}

// DataRoot is `<data>/muster` — root of everything the app owns.
func DataRoot() string {
	if dir := os.Getenv(EnvDataDir); dir != "" {
		return dir
	}
	if dir := os.Getenv(EnvLegacyDataDir); dir != "" {
		return dir
	}
	return filepath.Join(baseDir(), dirName)
}

// rootFromLegacyEnv reports whether the data root comes from RIMFORGE_DATA_DIR.
func rootFromLegacyEnv() bool {
	return os.Getenv(EnvDataDir) == "" && os.Getenv(EnvLegacyDataDir) != ""
}

// GameRoot is `<data>/muster/<game>` — root of everything one game module owns.
func GameRoot(game Game) string { return filepath.Join(DataRoot(), string(game)) }

// legacyRoot is the RimForge data directory that MigrateLegacy adopts: the
// sibling of the data root named `rimforge`. Deriving it from DataRoot rather
// than the platform data dir keeps the migration testable under MUSTER_DATA_DIR.
func legacyRoot() string { return filepath.Join(filepath.Dir(DataRoot()), legacyDirName) }

// MigrateLegacy adopts a RimForge data directory as the RimWorld game root.
//
// RimForge kept `profiles/`, `registry.json`, `settings.json` and `cache/`
// directly under its root; Muster keeps exactly that layout under
// `<root>/rimworld`. Two cases, both plain same-filesystem renames:
//
//   - Default locations: `<data>/rimforge` exists, `<data>/muster` does not,
//     and the legacy directory looks like ours (it holds a registry, a
//     settings file or a profiles directory) ⇒ rename it to
//     `<data>/muster/rimworld`.
//   - RIMFORGE_DATA_DIR set (and MUSTER_DATA_DIR not): that directory *is*
//     the data root, so it is migrated in place — its RimForge entries move
//     into a new `rimworld/` subdirectory. Data the user deliberately
//     relocated stays where they put it.
//
// Any other state — both present, neither present, an unrecognised legacy
// directory, an in-place root that already has `rimworld/` — is left alone.
// Returns whether a migration happened.
func MigrateLegacy() (bool, error) {
	if rootFromLegacyEnv() {
		return migrateInPlace(DataRoot())
	}
	root := DataRoot()
	if _, err := os.Stat(root); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("could not stat %s: %w", root, err)
	}
	legacy := legacyRoot()
	if info, err := os.Stat(legacy); err != nil || !info.IsDir() || !looksLikeRimForge(legacy) {
		return false, nil
	}
	if err := EnsureDir(root); err != nil {
		return false, err
	}
	dst := GameRoot(RimWorld)
	if err := os.Rename(legacy, dst); err != nil {
		// Leave no half-made root behind: a later run should retry cleanly.
		_ = os.Remove(root)
		return false, fmt.Errorf("could not move %s to %s: %w", legacy, dst, err)
	}
	return true, nil
}

// migrateInPlace moves a RimForge layout at root into root/rimworld.
func migrateInPlace(root string) (bool, error) {
	if !looksLikeRimForge(root) {
		return false, nil
	}
	dst := filepath.Join(root, string(RimWorld))
	if _, err := os.Stat(dst); err == nil {
		// Already migrated (or a mixed state we must not guess about).
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("could not stat %s: %w", dst, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return false, fmt.Errorf("could not read %s: %w", root, err)
	}
	if err := EnsureDir(dst); err != nil {
		return false, err
	}
	for _, e := range entries {
		from := filepath.Join(root, e.Name())
		if err := os.Rename(from, filepath.Join(dst, e.Name())); err != nil {
			return false, fmt.Errorf("could not move %s into %s: %w", from, dst, err)
		}
	}
	return true, nil
}

func looksLikeRimForge(dir string) bool {
	for _, name := range []string{"registry.json", "settings.json", "profiles"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// EnsureDir creates path (and parents) if absent.
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", path, err)
	}
	return nil
}

// WriteFileAtomic writes data to a temp file beside path and renames it into
// place, so a crash mid-write cannot truncate the target.
func WriteFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("could not write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	return nil
}
