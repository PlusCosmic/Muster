// Package settings persists `settings.json` (path overrides).
//
// Lives at `<data>/muster/rimworld/settings.json`. A missing file is not an error —
// it simply means the zero Settings.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"muster/internal/appdir"
	"muster/internal/rimworld/models"
)

// Path is the absolute path to settings.json.
func Path() string { return filepath.Join(appdir.GameRoot(appdir.RimWorld), "settings.json") }

// blankToNil normalises "" / whitespace-only overrides down to nil so the
// frontend can clear an override by submitting an empty text field.
func blankToNil(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	if t == "" {
		return nil
	}
	return &t
}

// Normalize trims every override and drops the blank ones.
func Normalize(s models.Settings) models.Settings {
	s.SteamRootOverride = blankToNil(s.SteamRootOverride)
	s.GameInstallOverride = blankToNil(s.GameInstallOverride)
	s.DefaultSavedataOverride = blankToNil(s.DefaultSavedataOverride)
	return s
}

// Load reads settings.json. Missing ⇒ zero value. Malformed ⇒ zero value plus a
// warning on stderr (a corrupt settings file must never brick the app).
func Load() models.Settings {
	path := Path()
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("muster: could not read %s: %v", path, err)
		}
		return models.Settings{}
	}
	var s models.Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		log.Printf("muster: malformed %s: %v", path, err)
		return models.Settings{}
	}
	return Normalize(s)
}

// Save writes settings.json atomically, creating the RimWorld root if needed.
func Save(s models.Settings) error {
	path := Path()
	if err := appdir.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("could not serialise settings: %w", err)
	}
	return appdir.WriteFileAtomic(path, data)
}

// Get is the `get_settings` command body.
func Get() (models.Settings, error) { return Load(), nil }

// Update is the `update_settings` command body: normalise, persist, echo back.
func Update(s models.Settings) (models.Settings, error) {
	s = Normalize(s)
	if err := Save(s); err != nil {
		return models.Settings{}, err
	}
	return s, nil
}
