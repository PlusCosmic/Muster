package mods

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rimforge/internal/models"
	"rimforge/internal/paths"
)

// CorePackageID is Core's package id.
const CorePackageID = "ludeon.rimworld"

// OfficialExpansions lists the official expansion package ids in release
// order. Used for the fixed sort tiers and for `knownExpansions`.
var OfficialExpansions = []string{
	"ludeon.rimworld.royalty",
	"ludeon.rimworld.ideology",
	"ludeon.rimworld.biotech",
	"ludeon.rimworld.anomaly",
	"ludeon.rimworld.odyssey",
}

// Official content ships About.xml with no `<name>` — the game hardcodes the
// display names, so we do too instead of falling back to the packageId.
var officialNames = map[string]string{
	CorePackageID:              "RimWorld",
	"ludeon.rimworld.royalty":  "Royalty",
	"ludeon.rimworld.ideology": "Ideology",
	"ludeon.rimworld.biotech":  "Biotech",
	"ludeon.rimworld.anomaly":  "Anomaly",
	"ludeon.rimworld.odyssey":  "Odyssey",
}

// ExpansionRank is the release-order rank of an official expansion, or -1.
func ExpansionRank(packageID string) int {
	for i, e := range OfficialExpansions {
		if e == packageID {
			return i
		}
	}
	return -1
}

// IsOfficialExpansion reports whether packageID is an expansion (not Core).
func IsOfficialExpansion(packageID string) bool { return ExpansionRank(packageID) >= 0 }

// findCaseInsensitive finds an entry of parent named name, ignoring case.
func findCaseInsensitive(parent, name string) (string, bool) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), name) {
			return filepath.Join(parent, e.Name()), true
		}
	}
	return "", false
}

// aboutPath finds `About/About.xml` case-insensitively. RimWorld tolerates
// any casing (e.g. Medieval Go-juice ships `About.XML`), which matters on
// Linux where the exact-case path simply doesn't exist.
func aboutPath(modDir string) (string, bool) {
	exact := filepath.Join(modDir, "About", "About.xml")
	if info, err := os.Stat(exact); err == nil && info.Mode().IsRegular() {
		return exact, true
	}
	aboutDir, ok := findCaseInsensitive(modDir, "About")
	if !ok {
		return "", false
	}
	file, ok := findCaseInsensitive(aboutDir, "About.xml")
	if !ok {
		return "", false
	}
	if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return file, true
}

func toModInfo(about AboutData, dir string, source models.ModSource, workshopID string) models.ModInfo {
	// ParseAbout falls back to name = packageId when <name> is absent, which
	// is how all official content ships.
	name := about.Name
	if name == about.PackageID {
		if official, ok := officialNames[about.PackageID]; ok {
			name = official
		}
	}
	return models.ModInfo{
		PackageID:         about.PackageID,
		Name:              name,
		Authors:           about.Authors,
		Path:              dir,
		Source:            source,
		SteamWorkshopID:   models.Str(workshopID),
		SupportedVersions: models.NonNil(about.SupportedVersions),
		Dependencies:      models.NonNil(about.Dependencies),
		LoadAfter:         models.NonNil(about.LoadAfter),
		LoadBefore:        models.NonNil(about.LoadBefore),
		ForceLoadAfter:    models.NonNil(about.ForceLoadAfter),
		ForceLoadBefore:   models.NonNil(about.ForceLoadBefore),
		IncompatibleWith:  models.NonNil(about.IncompatibleWith),
	}
}

// ReadModDir reads one mod directory. (nil, nil) = not a mod dir (no
// About.xml) — silent. An error = present but unreadable/malformed — the
// caller logs and skips.
func ReadModDir(dir string, source models.ModSource) (*models.ModInfo, error) {
	aboutFile, ok := aboutPath(dir)
	if !ok {
		return nil, nil
	}
	raw, err := os.ReadFile(aboutFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", aboutFile, err)
	}
	// About.xml is nominally utf-8; be lenient about stray bytes.
	about, err := ParseAbout(strings.ToValidUTF8(string(raw), "�"))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", aboutFile, err)
	}
	workshopID := ""
	if source == models.SourceWorkshop {
		workshopID = filepath.Base(dir)
	}
	info := toModInfo(about, dir, source, workshopID)
	return &info, nil
}

// ScanModContainer scans every immediate subdirectory of parent as a
// candidate mod folder, in sorted order so results are deterministic.
func ScanModContainer(parent string, source models.ModSource) []models.ModInfo {
	entries, err := os.ReadDir(parent)
	if err != nil {
		log.Printf("rimforge: cannot read %s: %v", parent, err)
		return nil
	}
	var dirs []string
	for _, e := range entries {
		p := filepath.Join(parent, e.Name())
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			dirs = append(dirs, p)
		}
	}
	sort.Strings(dirs)

	var out []models.ModInfo
	for _, dir := range dirs {
		m, err := ReadModDir(dir, source)
		switch {
		case err != nil:
			log.Printf("rimforge: skipping malformed mod: %v", err)
		case m != nil:
			out = append(out, *m)
		}
	}
	return out
}

// ScanAll runs the full scan. Later sources never override earlier ones, so
// precedence is official > local > workshop.
func ScanAll(gameInstall string, workshopDirs []string) []models.ModInfo {
	var found []models.ModInfo
	if gameInstall != "" {
		found = append(found, ScanModContainer(filepath.Join(gameInstall, "Data"), models.SourceOfficial)...)
		found = append(found, ScanModContainer(filepath.Join(gameInstall, "Mods"), models.SourceLocal)...)
	}
	for _, wd := range workshopDirs {
		found = append(found, ScanModContainer(wd, models.SourceWorkshop)...)
	}

	seen := map[string]bool{}
	out := make([]models.ModInfo, 0, len(found))
	for _, m := range found {
		if seen[m.PackageID] {
			continue
		}
		seen[m.PackageID] = true
		out = append(out, m)
	}
	return out
}

// InstalledExpansions returns the installed official expansion ids in release
// order — what goes into `<knownExpansions>`.
func InstalledExpansions(mods []models.ModInfo) []string {
	ids := []string{}
	for _, m := range mods {
		if m.Source == models.SourceOfficial && IsOfficialExpansion(m.PackageID) {
			ids = append(ids, m.PackageID)
		}
	}
	sort.SliceStable(ids, func(i, j int) bool { return ExpansionRank(ids[i]) < ExpansionRank(ids[j]) })
	return ids
}

// ListInstalled is the `list_installed_mods` command body.
func ListInstalled() ([]models.ModInfo, error) {
	p := paths.Detect()
	return ScanAll(models.Deref(p.GameInstall), p.WorkshopDirs), nil
}
