package mods

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	core "muster/internal/models"
	"muster/internal/rimworld/models"
	"muster/internal/rimworld/paths"
	"muster/internal/xmldom"
)

// ModsConfigPath is the active-mod list inside a profile's savedatafolder.
func ModsConfigPath(profileDir string) string {
	return filepath.Join(profileDir, "Config", "ModsConfig.xml")
}

func liValues(parent *xmldom.Node) []string {
	out := []string{}
	for _, li := range parent.ChildrenNamed("li") {
		if v := strings.ToLower(strings.TrimSpace(li.Text())); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ParseModsConfig parses a ModsConfig.xml document.
func ParseModsConfig(body string) (models.ActiveModList, error) {
	root, err := xmldom.Parse(body)
	if err != nil {
		return models.ActiveModList{}, fmt.Errorf("ModsConfig.xml %w", err)
	}
	list := models.ActiveModList{ActiveIDs: []string{}, KnownExpansions: []string{}}
	for _, node := range root.Children {
		switch strings.ToLower(node.Name) {
		case "version":
			list.Version = core.Str(strings.TrimSpace(node.Text()))
		case "activemods":
			list.ActiveIDs = liValues(node)
		case "knownexpansions":
			list.KnownExpansions = liValues(node)
		}
	}
	// De-duplicate while preserving order; RimWorld tolerates dupes, we don't.
	list.ActiveIDs = dedupe(list.ActiveIDs)
	return list, nil
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

func pushList(b *strings.Builder, tag string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(b, "  <%s />\n", tag)
		return
	}
	fmt.Fprintf(b, "  <%s>\n", tag)
	for _, item := range items {
		fmt.Fprintf(b, "    <li>%s</li>\n", xmlEscaper.Replace(item))
	}
	fmt.Fprintf(b, "  </%s>\n", tag)
}

// RenderModsConfig renders a ModsConfig.xml document: utf-8 declaration,
// 2-space indent, escaped text.
func RenderModsConfig(list models.ActiveModList) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<ModsConfigData>\n")
	if v := core.Deref(list.Version); v != "" {
		fmt.Fprintf(&b, "  <version>%s</version>\n", xmlEscaper.Replace(v))
	}
	pushList(&b, "activeMods", list.ActiveIDs)
	pushList(&b, "knownExpansions", list.KnownExpansions)
	b.WriteString("</ModsConfigData>\n")
	return b.String()
}

// ReadActive reads the list from disk. A missing file yields (nil, nil).
func ReadActive(path string) (*models.ActiveModList, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	list, err := ParseModsConfig(strings.ToValidUTF8(string(raw), "�"))
	if err != nil {
		return nil, err
	}
	return &list, nil
}

// WriteActive writes the list, creating `Config/` if needed.
func WriteActive(path string, list models.ActiveModList) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(RenderModsConfig(list)), 0o644); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// environment is best-effort context: the game version and the official
// expansions that are actually installed. Never fails the caller.
func environment() (version string, knownExpansions []string) {
	p := paths.Detect()
	mods := ScanAll(core.Deref(p.GameInstall), p.WorkshopDirs)
	return core.Deref(p.GameVersion), InstalledExpansions(mods)
}

// GetActive is the `get_active_mods` command body. A missing ModsConfig.xml
// yields the Core-only default.
func GetActive(profileID string) (models.ActiveModList, error) {
	list, err := ReadActive(ModsConfigPath(paths.ProfileDir(profileID)))
	if err != nil {
		return models.ActiveModList{}, err
	}
	if list != nil {
		return *list, nil
	}
	version, known := environment()
	return models.ActiveModList{
		ActiveIDs:       []string{CorePackageID},
		KnownExpansions: known,
		Version:         core.Str(version),
	}, nil
}

// SetActive is the `set_active_mods` command body. Preserves the existing
// `<version>` if the file already exists; `knownExpansions` is always the
// installed official set.
func SetActive(profileID string, activeIDs []string) error {
	path := ModsConfigPath(paths.ProfileDir(profileID))
	existing, err := ReadActive(path)
	if err != nil {
		return err
	}
	detectedVersion, known := environment()

	if len(known) == 0 && existing != nil {
		// Detection failed — don't clobber what the profile already knew.
		known = existing.KnownExpansions
		log.Printf("muster: no official expansions detected; keeping %v", known)
	}

	version := detectedVersion
	if existing != nil && existing.Version != nil {
		version = *existing.Version
	}

	normalized := []string{}
	for _, id := range activeIDs {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			normalized = append(normalized, id)
		}
	}
	return WriteActive(path, models.ActiveModList{
		ActiveIDs:       dedupe(normalized),
		KnownExpansions: core.NonNil(known),
		Version:         core.Str(version),
	})
}
