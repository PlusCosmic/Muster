// Package appdir locates the app-owned data root:
//
//	<data>/rimforge/
//	  profiles/<slug>/   registry.json   settings.json   cache/
//
// RIMFORGE_DATA_DIR, when set, replaces the whole root. It exists so tests
// (and anyone relocating their profiles) can point the app at another disk.
package appdir

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

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

// DataRoot is `<data>/rimforge` — root of everything the app owns.
func DataRoot() string {
	if dir := os.Getenv("RIMFORGE_DATA_DIR"); dir != "" {
		return dir
	}
	base, ok := dataDir()
	if !ok {
		if home, err := os.UserHomeDir(); err == nil {
			base = home
		} else {
			base = "."
		}
	}
	return filepath.Join(base, "rimforge")
}

// ProfilesRoot is `<data>/rimforge/profiles`.
func ProfilesRoot() string { return filepath.Join(DataRoot(), "profiles") }

// CacheRoot is `<data>/rimforge/cache`.
func CacheRoot() string { return filepath.Join(DataRoot(), "cache") }

// ProfileDir is `<data>/rimforge/profiles/<id>` — does not check that it exists.
func ProfileDir(id string) string { return filepath.Join(ProfilesRoot(), id) }

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
