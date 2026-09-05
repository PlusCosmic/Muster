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
// data) can point the app at another disk.
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
	return filepath.Join(baseDir(), dirName)
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
// directly under `<data>/rimforge`; Muster keeps exactly that layout under
// `<data>/muster/rimworld`, so the migration is one directory rename. It runs
// only when the data root does not exist yet and the legacy directory looks
// like ours (it holds a registry, a settings file or a profiles directory).
// Any other state — both present, neither present, an unrecognised legacy
// directory — is left alone. Returns whether a migration happened.
func MigrateLegacy() (bool, error) {
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
