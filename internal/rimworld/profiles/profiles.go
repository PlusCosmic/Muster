// Package profiles owns the registry (`registry.json`) and the profile
// directories themselves.
//
// A profile is a self-contained `-savedatafolder`: `<data>/muster/rimworld/profiles/<id>`
// with its own `Config/`, `Saves/` and `Scenarios/`. `registry.json` holds only
// the metadata that cannot be derived from the filesystem (display name and
// timestamps); save/mod counts are recomputed on every listing.
//
// Concurrency: registry writes are plain read-modify-write. Commands are not
// expected to race meaningfully in v1.
package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"muster/internal/appdir"
	core "muster/internal/models"
	"muster/internal/rimworld/models"
	"muster/internal/rimworld/paths"
	"muster/internal/trash"
	"muster/internal/xmldom"
)

// CorePackageID is the single mod every profile starts with.
const CorePackageID = "ludeon.rimworld"

// trashFn is swapped out by tests so a scratch profile never reaches the
// real system trash.
var trashFn = trash.Move

// ---------------------------------------------------------------------------
// Registry types
// ---------------------------------------------------------------------------

// Meta is one record in registry.json. Not part of the frontend contract; the
// derived models.Profile is what crosses the boundary.
type Meta struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CreatedAtMs    int64  `json:"createdAtMs"`
	LastPlayedAtMs *int64 `json:"lastPlayedAtMs,omitempty"`
}

type Registry struct {
	Profiles []Meta `json:"profiles"`
}

func (r *Registry) indexOf(id string) int {
	for i, p := range r.Profiles {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

// NowMs is milliseconds since the Unix epoch.
func NowMs() int64 { return time.Now().UnixMilli() }

// RegistryPath is `<data>/muster/rimworld/registry.json`.
func RegistryPath() string { return filepath.Join(paths.Root(), "registry.json") }

// ---------------------------------------------------------------------------
// Slugs
// ---------------------------------------------------------------------------

// Slugify lowercases to `[a-z0-9-]`, collapsing runs of separators and
// trimming them. An empty result falls back to `profile`.
func Slugify(name string) string {
	var out strings.Builder
	pendingDash := false
	for _, ch := range name {
		lower := ch
		if ch >= 'A' && ch <= 'Z' {
			lower = ch + ('a' - 'A')
		}
		if (lower >= 'a' && lower <= 'z') || (lower >= '0' && lower <= '9') {
			if pendingDash && out.Len() > 0 {
				out.WriteByte('-')
			}
			pendingDash = false
			out.WriteRune(lower)
		} else {
			// Any non [a-z0-9] character (including non-ASCII) is a separator.
			pendingDash = true
		}
	}
	if out.Len() == 0 {
		return "profile"
	}
	return out.String()
}

// UniqueSlug appends `-2`, `-3`, … until the slug is free.
func UniqueSlug(name string, taken map[string]bool) string {
	base := Slugify(name)
	if !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !taken[candidate] {
			return candidate
		}
	}
}

// takenIDs are ids already in use: registry entries and any stray directory.
func takenIDs(reg *Registry) map[string]bool {
	taken := map[string]bool{}
	for _, p := range reg.Profiles {
		taken[p.ID] = true
	}
	if entries, err := os.ReadDir(paths.ProfilesRoot()); err == nil {
		for _, e := range entries {
			taken[e.Name()] = true
		}
	}
	return taken
}

// ---------------------------------------------------------------------------
// Registry IO
// ---------------------------------------------------------------------------

// LoadRegistry reads registry.json. Missing ⇒ empty. Malformed ⇒ error (we do
// not want to silently drop a user's profile list).
func LoadRegistry() (*Registry, error) {
	path := RegistryPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Registry{}, nil
		}
		return nil, fmt.Errorf("could not read %s: %w", path, err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return &Registry{}, nil
	}
	var reg Registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil, fmt.Errorf("malformed %s: %w", path, err)
	}
	return &reg, nil
}

// SaveRegistry writes registry.json atomically.
func SaveRegistry(reg *Registry) error {
	if err := appdir.EnsureDir(paths.Root()); err != nil {
		return err
	}
	if reg.Profiles == nil {
		reg.Profiles = []Meta{}
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("could not serialise registry: %w", err)
	}
	return appdir.WriteFileAtomic(RegistryPath(), data)
}

// ---------------------------------------------------------------------------
// Derived counts
// ---------------------------------------------------------------------------

// CountSaves counts `*.rws` files directly inside `<profile>/Saves`.
func CountSaves(dir string) int {
	entries, err := os.ReadDir(filepath.Join(dir, "Saves"))
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if strings.EqualFold(filepath.Ext(e.Name()), ".rws") {
			n++
		}
	}
	return n
}

// CountActiveModsInXML counts `<activeMods>/<li>` entries in a ModsConfig.xml body.
func CountActiveModsInXML(body string) int {
	root, err := xmldom.Parse(body)
	if err != nil {
		log.Printf("muster: malformed ModsConfig.xml: %v", err)
		return 0
	}
	if node := findDescendant(root, "activeMods"); node != nil {
		return len(node.ChildrenNamed("li"))
	}
	return 0
}

func findDescendant(n *xmldom.Node, name string) *xmldom.Node {
	if n.Is(name) {
		return n
	}
	for _, c := range n.Children {
		if found := findDescendant(c, name); found != nil {
			return found
		}
	}
	return nil
}

// CountActiveMods reads `<profile>/Config/ModsConfig.xml`; 0 if absent.
func CountActiveMods(dir string) int {
	raw, err := os.ReadFile(filepath.Join(dir, "Config", "ModsConfig.xml"))
	if err != nil {
		return 0
	}
	return CountActiveModsInXML(string(raw))
}

func toProfile(meta Meta) models.Profile {
	dir := paths.ProfileDir(meta.ID)
	return models.Profile{
		ID:             meta.ID,
		Name:           meta.Name,
		Path:           dir,
		CreatedAtMs:    meta.CreatedAtMs,
		LastPlayedAtMs: meta.LastPlayedAtMs,
		SaveCount:      CountSaves(dir),
		ActiveModCount: CountActiveMods(dir),
	}
}

// ---------------------------------------------------------------------------
// ModsConfig.xml bootstrap
// ---------------------------------------------------------------------------

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// MinimalModsConfig renders a Core-only ModsConfig.xml, plus the detected game
// version if known.
func MinimalModsConfig(version string) string {
	versionLine := ""
	if version != "" {
		versionLine = fmt.Sprintf("  <version>%s</version>\n", xmlEscape(version))
	}
	return "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<ModsConfigData>\n" +
		versionLine +
		"  <activeMods>\n    <li>" + CorePackageID + "</li>\n  </activeMods>\n</ModsConfigData>\n"
}

// ---------------------------------------------------------------------------
// Copying
// ---------------------------------------------------------------------------

// copyTree recursively copies src into dst, following symlinks and copying
// file contents. The source is never modified — important because the default
// savedata folder is frequently symlink-managed (e.g. a dotfiles repo).
func copyTree(src, dst string, depth int) error {
	if depth > 64 {
		return fmt.Errorf("refusing to recurse further at %s (symlink loop?)", src)
	}
	if err := appdir.EnsureDir(dst); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", src, err)
	}
	for _, e := range entries {
		from := filepath.Join(src, e.Name())
		to := filepath.Join(dst, e.Name())
		// os.Stat (not Lstat) follows symlinks by design.
		info, err := os.Stat(from)
		if err != nil {
			log.Printf("muster: skipping %s (%v)", from, err)
			continue
		}
		switch {
		case info.IsDir():
			if err := copyTree(from, to, depth+1); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := copyFile(from, to); err != nil {
				return fmt.Errorf("could not copy %s to %s: %w", from, to, err)
			}
		}
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// List is the `list_profiles` command body.
func List() ([]models.Profile, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	out := make([]models.Profile, 0, len(reg.Profiles))
	for _, m := range reg.Profiles {
		out = append(out, toProfile(m))
	}
	return out, nil
}

// Find looks a profile up by id.
func Find(id string) (models.Profile, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return models.Profile{}, err
	}
	if i := reg.indexOf(id); i >= 0 {
		return toProfile(reg.Profiles[i]), nil
	}
	return models.Profile{}, fmt.Errorf("unknown profile: %s", id)
}

func requireName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("profile name cannot be empty")
	}
	return name, nil
}

func register(reg *Registry, id, name string) (models.Profile, error) {
	meta := Meta{ID: id, Name: name, CreatedAtMs: NowMs()}
	profile := toProfile(meta)
	reg.Profiles = append(reg.Profiles, meta)
	if err := SaveRegistry(reg); err != nil {
		return models.Profile{}, err
	}
	return profile, nil
}

// Create is the `create_profile` command body: an empty savedatafolder with a
// Core-only ModsConfig.xml.
func Create(name string) (models.Profile, error) {
	name, err := requireName(name)
	if err != nil {
		return models.Profile{}, err
	}
	reg, err := LoadRegistry()
	if err != nil {
		return models.Profile{}, err
	}
	if err := appdir.EnsureDir(paths.ProfilesRoot()); err != nil {
		return models.Profile{}, err
	}
	id := UniqueSlug(name, takenIDs(reg))
	dir := paths.ProfileDir(id)
	if err := appdir.EnsureDir(filepath.Join(dir, "Config")); err != nil {
		return models.Profile{}, err
	}

	version := core.Deref(paths.Detect().GameVersion)
	configPath := filepath.Join(dir, "Config", "ModsConfig.xml")
	if err := os.WriteFile(configPath, []byte(MinimalModsConfig(version)), 0o644); err != nil {
		return models.Profile{}, fmt.Errorf("could not write %s: %w", configPath, err)
	}
	return register(reg, id, name)
}

// Rename is the `rename_profile` command body: display name only.
func Rename(id, newName string) (models.Profile, error) {
	newName, err := requireName(newName)
	if err != nil {
		return models.Profile{}, err
	}
	reg, err := LoadRegistry()
	if err != nil {
		return models.Profile{}, err
	}
	i := reg.indexOf(id)
	if i < 0 {
		return models.Profile{}, fmt.Errorf("unknown profile: %s", id)
	}
	reg.Profiles[i].Name = newName
	profile := toProfile(reg.Profiles[i])
	if err := SaveRegistry(reg); err != nil {
		return models.Profile{}, err
	}
	return profile, nil
}

// Delete is the `delete_profile` command body: the directory goes to the OS
// trash and the registry entry is removed.
func Delete(id string) error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}
	i := reg.indexOf(id)
	if i < 0 {
		return fmt.Errorf("unknown profile: %s", id)
	}
	dir := paths.ProfileDir(id)
	if _, err := os.Lstat(dir); err == nil {
		if err := trashFn(dir); err != nil {
			return fmt.Errorf("could not move %s to trash: %w", dir, err)
		}
	}
	reg.Profiles = append(reg.Profiles[:i], reg.Profiles[i+1:]...)
	return SaveRegistry(reg)
}

// Clone is the `clone_profile` command body: a deep copy of the directory.
func Clone(id, newName string) (models.Profile, error) {
	newName, err := requireName(newName)
	if err != nil {
		return models.Profile{}, err
	}
	reg, err := LoadRegistry()
	if err != nil {
		return models.Profile{}, err
	}
	if reg.indexOf(id) < 0 {
		return models.Profile{}, fmt.Errorf("unknown profile: %s", id)
	}
	src := paths.ProfileDir(id)
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return models.Profile{}, fmt.Errorf("profile directory missing: %s", src)
	}
	if err := appdir.EnsureDir(paths.ProfilesRoot()); err != nil {
		return models.Profile{}, err
	}
	newID := UniqueSlug(newName, takenIDs(reg))
	dst := paths.ProfileDir(newID)
	if err := copyTree(src, dst, 0); err != nil {
		return models.Profile{}, fmt.Errorf("could not copy %s to %s: %w", src, dst, err)
	}
	return register(reg, newID, newName)
}

// ImportDefault is the `import_default` command body: copy the vanilla
// savedata's Config/Saves/Scenarios into a new profile without touching the
// source.
func ImportDefault(name string) (models.Profile, error) {
	name, err := requireName(name)
	if err != nil {
		return models.Profile{}, err
	}
	source := core.Deref(paths.Detect().DefaultSavedata)
	if source == "" {
		return models.Profile{}, errors.New("no default RimWorld savedata folder found to import")
	}
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		return models.Profile{}, fmt.Errorf("default savedata folder not found: %s", source)
	}

	reg, err := LoadRegistry()
	if err != nil {
		return models.Profile{}, err
	}
	if err := appdir.EnsureDir(paths.ProfilesRoot()); err != nil {
		return models.Profile{}, err
	}
	id := UniqueSlug(name, takenIDs(reg))
	dir := paths.ProfileDir(id)
	if err := appdir.EnsureDir(dir); err != nil {
		return models.Profile{}, err
	}

	copiedAny := false
	for _, sub := range []string{"Config", "Saves", "Scenarios"} {
		from := filepath.Join(source, sub)
		// os.Stat follows symlinks: a symlinked Config/ is copied by content.
		if info, err := os.Stat(from); err == nil && info.IsDir() {
			if err := copyTree(from, filepath.Join(dir, sub), 0); err != nil {
				return models.Profile{}, err
			}
			copiedAny = true
		}
	}
	if !copiedAny {
		log.Printf("muster: %s had no Config/Saves/Scenarios to import", source)
	}
	// Always guarantee a Config/ directory exists.
	if err := appdir.EnsureDir(filepath.Join(dir, "Config")); err != nil {
		return models.Profile{}, err
	}
	return register(reg, id, name)
}

// TouchLastPlayed stamps lastPlayedAtMs on a profile. Used by launch.
func TouchLastPlayed(id string) error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}
	i := reg.indexOf(id)
	if i < 0 {
		return fmt.Errorf("unknown profile: %s", id)
	}
	reg.Profiles[i].LastPlayedAtMs = core.Int64(NowMs())
	return SaveRegistry(reg)
}
