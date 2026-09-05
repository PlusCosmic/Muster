// Package launcher talks to the official Minecraft launcher: where its
// `.minecraft` directory is, how to add a profile to it, and how to open it.
//
// The launcher keeps its installations in `.minecraft/launcher_profiles.json`:
//
//	{
//	  "profiles": {
//	    "<key>": { "name", "type": "custom", "created", "lastUsed", "icon",
//	               "lastVersionId", "gameDir", "javaArgs", "javaDir" }
//	  },
//	  "settings": { … }, "version": 3
//	}
//
// The launcher rewrites this file itself, so we never own it: every write is a
// read-merge-write that touches only our own profile entries and preserves
// every other byte of structure as raw JSON. The Microsoft Store build has
// historically kept a sibling `launcher_profiles_microsoft_store.json`; when
// it exists, it gets the same edit.
package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ProfilesFile and StoreProfilesFile are the launcher's installation lists.
const (
	ProfilesFile      = "launcher_profiles.json"
	StoreProfilesFile = "launcher_profiles_microsoft_store.json"
)

// Profile is one installation as the launcher stores it. Zero-valued optional
// fields are omitted so the launcher's defaults apply.
type Profile struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Created       string `json:"created,omitempty"`
	LastUsed      string `json:"lastUsed,omitempty"`
	Icon          string `json:"icon,omitempty"`
	LastVersionID string `json:"lastVersionId"`
	GameDir       string `json:"gameDir,omitempty"`
	JavaArgs      string `json:"javaArgs,omitempty"`
	JavaDir       string `json:"javaDir,omitempty"`
}

// ProfileKey is the launcher_profiles.json key for one of our packs. Stable,
// so re-syncing updates the same entry.
func ProfileKey(packID string) string { return "muster-" + packID }

// VersionID is the launcher's installation id for a loader on a Minecraft
// version — the directory name the loader's installer creates under
// `.minecraft/versions/`.
func VersionID(minecraft, loader, loaderVersion string) (string, error) {
	switch loader {
	case "":
		return minecraft, nil
	case "fabric":
		return fmt.Sprintf("fabric-loader-%s-%s", loaderVersion, minecraft), nil
	case "quilt":
		return fmt.Sprintf("quilt-loader-%s-%s", loaderVersion, minecraft), nil
	case "neoforge":
		return "neoforge-" + loaderVersion, nil
	case "forge":
		return fmt.Sprintf("%s-forge-%s", minecraft, loaderVersion), nil
	}
	return "", fmt.Errorf("unsupported loader %q", loader)
}

// Now formats a time the way the launcher does.
func Now() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }

// document is launcher_profiles.json with only the parts we edit decoded.
type document struct {
	top      map[string]json.RawMessage
	profiles map[string]json.RawMessage
}

func readDocument(path string) (*document, error) {
	d := &document{top: map[string]json.RawMessage{}, profiles: map[string]json.RawMessage{}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &d.top); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if p, ok := d.top["profiles"]; ok && len(p) > 0 && string(p) != "null" {
		if err := json.Unmarshal(p, &d.profiles); err != nil {
			return nil, fmt.Errorf("%s: profiles: %w", filepath.Base(path), err)
		}
	}
	return d, nil
}

func (d *document) write(path string) error {
	p, err := json.Marshal(d.profiles)
	if err != nil {
		return err
	}
	d.top["profiles"] = p
	if _, ok := d.top["version"]; !ok {
		d.top["version"] = json.RawMessage("3")
	}
	out, err := json.MarshalIndent(d.top, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Upsert writes profile under ProfileKey(packID) in every profiles file the
// launcher keeps in dir, preserving everything else in those files. Only the
// fields Muster owns are written; anything else the launcher or the user has
// put on the profile (a resolution, a custom javaDir, …) is kept. An existing
// entry keeps its `created` timestamp; `lastUsed` is set to now so the
// launcher preselects it. The launcher itself only reads this file on start,
// so it must be restarted to notice a new entry.
func Upsert(dir, packID string, profile Profile) error {
	key := ProfileKey(packID)
	if profile.Type == "" {
		profile.Type = "custom"
	}
	profile.LastUsed = Now()
	for _, name := range profilesFiles(dir) {
		path := filepath.Join(dir, name)
		d, err := readDocument(path)
		if err != nil {
			return err
		}
		merged := map[string]json.RawMessage{}
		if old, ok := d.profiles[key]; ok {
			if err := json.Unmarshal(old, &merged); err != nil {
				merged = map[string]json.RawMessage{}
			}
		}
		p := profile
		if c, ok := merged["created"]; ok && len(c) > 2 {
			p.Created = "" // keep the recorded one
		} else {
			p.Created = p.LastUsed
		}
		ours, err := json.Marshal(p)
		if err != nil {
			return err
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(ours, &fields); err != nil {
			return err
		}
		for k, v := range fields {
			merged[k] = v
		}
		raw, err := json.Marshal(merged)
		if err != nil {
			return err
		}
		d.profiles[key] = raw
		if err := d.write(path); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// Remove deletes our profile for packID from every profiles file in dir.
func Remove(dir, packID string) error { return RemoveKeys(dir, []string{ProfileKey(packID)}) }

// RemoveKeys deletes the given profile keys (any owner) from every profiles
// file in dir.
func RemoveKeys(dir string, keys []string) error {
	for _, name := range profilesFiles(dir) {
		if err := RemoveKeysFrom(dir, name, keys); err != nil {
			return err
		}
	}
	return nil
}

// RemoveKeysFrom deletes the given profile keys from one profiles file in dir.
// Used to undo the entries loader installers inject, file by file.
func RemoveKeysFrom(dir, name string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	d, err := readDocument(path)
	if err != nil {
		return err
	}
	changed := false
	for _, key := range keys {
		if _, ok := d.profiles[key]; ok {
			delete(d.profiles, key)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := d.write(path); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// ProfilesFiles is the main profiles file plus the Store variant when it exists.
func ProfilesFiles(dir string) []string { return profilesFiles(dir) }

// Get reads our profile for packID from the main profiles file, if present.
func Get(dir, packID string) (Profile, bool, error) {
	d, err := readDocument(filepath.Join(dir, ProfilesFile))
	if err != nil {
		return Profile{}, false, err
	}
	raw, ok := d.profiles[ProfileKey(packID)]
	if !ok {
		return Profile{}, false, nil
	}
	var p Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		return Profile{}, false, err
	}
	return p, true, nil
}

// profilesFiles is the main file plus the Store variant when it exists.
func profilesFiles(dir string) []string {
	files := []string{ProfilesFile}
	if _, err := os.Stat(filepath.Join(dir, StoreProfilesFile)); err == nil {
		files = append(files, StoreProfilesFile)
	}
	return files
}

// HasVersion reports whether the launcher has an installation directory for
// versionID, i.e. `.minecraft/versions/<id>/<id>.json` exists.
func HasVersion(dir, versionID string) bool {
	_, err := os.Stat(filepath.Join(dir, "versions", versionID, versionID+".json"))
	return err == nil
}
