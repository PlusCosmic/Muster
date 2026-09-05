// Package appdir locates the app-owned data root and the per-game roots
// beneath it:
//
//	<data>/muster/
//	  settings.json        # the app's own settings (internal/settings)
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
	"encoding/json"
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

// Games is every game module, in rail order.
var Games = []Game{RimWorld, Minecraft}

// GamesInUse reports the game modules whose root holds anything at all: a
// module that has been opened before, whatever it wrote there. The shell
// uses it to preselect games on the welcome screen.
func GamesInUse() []Game {
	used := []Game{}
	for _, g := range Games {
		entries, err := os.ReadDir(GameRoot(g))
		if err == nil && len(entries) > 0 {
			used = append(used, g)
		}
	}
	return used
}

// legacyRoot is the RimForge data directory that MigrateLegacy adopts: the
// sibling of the data root named `rimforge`. Deriving it from DataRoot rather
// than the platform data dir keeps the migration testable under MUSTER_DATA_DIR.
func legacyRoot() string { return filepath.Join(filepath.Dir(DataRoot()), legacyDirName) }

// MigrateLegacy adopts a RimForge data directory as the RimWorld game root.
//
// RimForge kept `profiles/`, `registry.json`, `settings.json` and `cache/`
// directly under its root; Muster keeps exactly that layout under
// `<root>/rimworld`. Only those four entries move — RimForge's root also holds
// WebKitGTK's own storage (`storage/`, `hsts-storage.sqlite`, …), which is
// keyed by program name and is not ours to relocate, and a user's custom root
// may hold anything. Two sources, same procedure:
//
//   - Default locations: `<data>/rimforge` ⇒ `<data>/muster/rimworld`.
//   - RIMFORGE_DATA_DIR set (and MUSTER_DATA_DIR not): that directory *is*
//     the data root, so its entries move into `<root>/rimworld` in place.
//     Data the user deliberately relocated stays where they put it.
//
// Each entry is one same-filesystem rename, and the move is resumable: the two
// entries that count as evidence of RimForge data (registry, profiles) go
// last, so a run that dies part-way always leaves one behind for the next run
// to pick up. Once Muster has run (the destination exists) a lone
// settings.json or cache/ is no longer taken as RimForge's, because Muster
// keeps its own settings.json at the root; and a root settings.json that
// holds Muster's settings (a `games` key, which RimForge never wrote) is
// never RimForge's, however fresh the root — the user may have picked only
// Minecraft, so the RimWorld root need not exist yet. An entry present on
// both sides is an error rather than a guess. Returns whether anything was
// moved.
func MigrateLegacy() (bool, error) {
	src := legacyRoot()
	if rootFromLegacyEnv() {
		src = DataRoot()
	}
	return migrateEntries(src, GameRoot(RimWorld))
}

// legacyEntries is everything RimForge ever wrote to its root, in the order
// they are moved; see MigrateLegacy for why the markers come last.
var legacyEntries = []string{"settings.json", "cache", "registry.json", "profiles"}

// legacyMarkers are the entries that always mean RimForge data.
var legacyMarkers = []string{"registry.json", "profiles"}

func anyExists(dir string, names []string) bool {
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// isAppSettings reports whether the file at path is Muster's own root
// settings.json rather than a RimForge one: Muster's has a top-level `games`
// key, RimForge's only ever had path overrides. Unreadable or malformed ⇒
// false, and the usual rules decide.
func isAppSettings(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return false
	}
	_, ok := keys["games"]
	return ok
}

func migrateEntries(src, dst string) (bool, error) {
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return false, nil
	}
	_, dstErr := os.Stat(dst)
	neverRan := os.IsNotExist(dstErr)
	entries := legacyEntries
	if isAppSettings(filepath.Join(src, "settings.json")) {
		entries = entries[1:] // Muster's, not RimForge's: stays put
	}
	if !anyExists(src, legacyMarkers) && !(neverRan && anyExists(src, entries)) {
		return false, nil
	}
	if err := EnsureDir(dst); err != nil {
		return false, err
	}
	moved := false
	for _, name := range entries {
		from := filepath.Join(src, name)
		if _, err := os.Lstat(from); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return moved, fmt.Errorf("could not stat %s: %w", from, err)
		}
		to := filepath.Join(dst, name)
		if _, err := os.Lstat(to); err == nil {
			return moved, fmt.Errorf("both %s and %s exist; resolve by hand", from, to)
		}
		if err := os.Rename(from, to); err != nil {
			return moved, fmt.Errorf("could not move %s into %s: %w", from, dst, err)
		}
		moved = true
	}
	return moved, nil
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
