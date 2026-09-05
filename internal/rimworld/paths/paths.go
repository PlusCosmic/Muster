// Package paths knows where RimWorld's things are: the app-owned layout under
// the RimWorld game root, and the external locations it detects (Steam, the
// game install, workshop content, the default savedata folder).
//
// App-owned layout (`<data>/muster/rimworld`):
//
//	profiles/<slug>/          # one savedatafolder per profile
//	registry.json             # profile metadata
//	settings.json             # path overrides
//	cache/                    # community rules DB
//
// Detection never fails on a missing piece: anything we cannot find is nil.
// Settings overrides are applied on top of whatever was detected.
package paths

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"muster/internal/appdir"
	core "muster/internal/models"
	"muster/internal/rimworld/models"
	"muster/internal/rimworld/settings"
)

// RimWorldAppID is RimWorld's Steam app id.
const RimWorldAppID = "294100"

// Root is `<data>/muster/rimworld` — everything the RimWorld module owns.
func Root() string { return appdir.GameRoot(appdir.RimWorld) }

// ProfilesRoot is `<root>/profiles`.
func ProfilesRoot() string { return filepath.Join(Root(), "profiles") }

// ProfileDir is `<root>/profiles/<id>` — does not check that it exists.
func ProfileDir(id string) string { return filepath.Join(ProfilesRoot(), id) }

// CacheRoot is `<root>/cache`.
func CacheRoot() string { return filepath.Join(Root(), "cache") }

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ---------------------------------------------------------------------------
// libraryfolders.vdf
// ---------------------------------------------------------------------------

// unescapeVDF unescapes a VDF string literal (`\\` ⇒ `\`, `\"` ⇒ `"`, `\n`/`\t`).
func unescapeVDF(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c != '\\' {
			out.WriteRune(c)
			continue
		}
		if i+1 >= len(runes) {
			out.WriteRune('\\')
			break
		}
		i++
		switch runes[i] {
		case '\\':
			out.WriteRune('\\')
		case '"':
			out.WriteRune('"')
		case 'n':
			out.WriteRune('\n')
		case 't':
			out.WriteRune('\t')
		default:
			out.WriteRune(runes[i])
		}
	}
	return out.String()
}

var libraryPathRe = regexp.MustCompile(`"path"\s+"((?:[^"\\]|\\.)*)"`)

// ParseLibraryPaths extracts every library `"path"` value from a
// libraryfolders.vdf body. Deliberately regex-based (no VDF dependency, per
// the architecture contract).
func ParseLibraryPaths(vdf string) []string {
	var out []string
	for _, m := range libraryPathRe.FindAllStringSubmatch(vdf, -1) {
		p := unescapeVDF(m[1])
		if !contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// SteamLibraries returns every Steam library: the root itself plus everything
// in libraryfolders.vdf, keeping only those with a steamapps/ directory.
func SteamLibraries(steamRoot string) []string {
	libs := []string{steamRoot}
	vdf := filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf")
	if body, err := os.ReadFile(vdf); err == nil {
		for _, p := range ParseLibraryPaths(string(body)) {
			if !contains(libs, p) {
				libs = append(libs, p)
			}
		}
	}
	var out []string
	for _, l := range libs {
		if isDir(filepath.Join(l, "steamapps")) {
			out = append(out, l)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Game install / workshop / version
// ---------------------------------------------------------------------------

// FindGameInstall returns the first library whose steamapps/common/RimWorld
// looks like a real install, preferring one with this platform's executable.
func FindGameInstall(libraries []string) (string, bool) {
	var candidates []string
	for _, lib := range libraries {
		dir := filepath.Join(lib, "steamapps", "common", "RimWorld")
		if isDir(dir) {
			candidates = append(candidates, dir)
		}
	}
	for _, dir := range candidates {
		if exists(filepath.Join(dir, GameBinaryName())) {
			return dir, true
		}
	}
	if len(candidates) > 0 {
		return candidates[0], true
	}
	return "", false
}

// FindWorkshopDirs returns `<library>/steamapps/workshop/content/294100` for
// every library that has it.
func FindWorkshopDirs(libraries []string) []string {
	out := []string{}
	for _, lib := range libraries {
		dir := filepath.Join(lib, "steamapps", "workshop", "content", RimWorldAppID)
		if isDir(dir) {
			out = append(out, dir)
		}
	}
	return out
}

// ParseVersion parses Version.txt: the first non-blank line, BOM stripped.
func ParseVersion(contents string) (string, bool) {
	for _, line := range strings.Split(contents, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "\ufeff"))
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	}
	return "", false
}

// MajorMinor turns `1.6.4535 rev991` into `1.6`.
func MajorMinor(version string) (string, bool) {
	fields := strings.Fields(version)
	if len(fields) == 0 {
		return "", false
	}
	parts := strings.Split(fields[0], ".")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[0] + "." + parts[1], true
}

// ReadGameVersion reads `<install>/Version.txt`.
func ReadGameVersion(install string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(install, "Version.txt"))
	if err != nil {
		return "", false
	}
	return ParseVersion(string(raw))
}

// ---------------------------------------------------------------------------
// Detection entry point
// ---------------------------------------------------------------------------

// Detect runs blocking detection with settings overrides applied on top.
func Detect() models.DetectedPaths {
	s := settings.Load()

	steamRoot := core.Deref(s.SteamRootOverride)
	if steamRoot == "" {
		steamRoot = detectSteamRoot()
	}

	var libraries []string
	if steamRoot != "" {
		libraries = SteamLibraries(steamRoot)
	}

	gameInstall := core.Deref(s.GameInstallOverride)
	if gameInstall == "" {
		gameInstall, _ = FindGameInstall(libraries)
	}

	workshopDirs := FindWorkshopDirs(libraries)

	gameVersion := ""
	if gameInstall != "" {
		gameVersion, _ = ReadGameVersion(gameInstall)
	}

	defaultSavedata := core.Deref(s.DefaultSavedataOverride)
	if defaultSavedata == "" {
		defaultSavedata = defaultSavedataDir()
	}
	if defaultSavedata != "" && !exists(defaultSavedata) {
		defaultSavedata = ""
	}

	return models.DetectedPaths{
		SteamRoot:       core.Str(steamRoot),
		GameInstall:     core.Str(gameInstall),
		DefaultSavedata: core.Str(defaultSavedata),
		WorkshopDirs:    workshopDirs,
		GameVersion:     core.Str(gameVersion),
		ProfilesDir:     ProfilesRoot(),
	}
}

// Home is the user's home directory, or "" if unknown.
func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		log.Printf("muster: no home directory: %v", err)
		return ""
	}
	return h
}
