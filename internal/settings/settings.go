// Package settings persists the app's own `settings.json`: what the shell
// needs before any game module is involved. Today that is the list of game
// modules the user has switched on.
//
// Lives at `<data>/muster/settings.json`. A missing file is not an error — it
// is the zero Settings, which the frontend reads as "not set up yet" and
// answers with the welcome screen.
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
	"muster/internal/models"
)

// Path is the absolute path to settings.json.
func Path() string { return filepath.Join(appdir.DataRoot(), "settings.json") }

// Normalize keeps only known game ids, once each, in the rail's fixed order.
// Anything an older or newer build wrote that this one does not know is
// dropped rather than failing the load.
func Normalize(s models.AppSettings) models.AppSettings {
	wanted := map[string]bool{}
	for _, id := range s.Games {
		wanted[strings.ToLower(strings.TrimSpace(id))] = true
	}
	games := []string{}
	for _, g := range appdir.Games {
		if wanted[string(g)] {
			games = append(games, string(g))
		}
	}
	s.Games = games
	return s
}

// Load reads settings.json. Missing ⇒ zero value: first run. A file that
// exists but cannot be read is an error, not a first run — the frontend
// would otherwise send the user back through the welcome screen and
// overwrite (or fail to overwrite) a choice that is still there. Malformed ⇒
// zero value plus a warning on stderr: a corrupt settings file must never
// brick the app, and the welcome screen rewrites it.
func Load() (models.AppSettings, error) {
	path := Path()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Normalize(models.AppSettings{}), nil
		}
		return models.AppSettings{}, fmt.Errorf("could not read %s: %w", path, err)
	}
	var s models.AppSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		log.Printf("muster: malformed %s: %v", path, err)
		return Normalize(models.AppSettings{}), nil
	}
	return Normalize(s), nil
}

// Save writes settings.json atomically, creating the data root if needed.
func Save(s models.AppSettings) error {
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

// Get is the `GetSettings` command body.
func Get() (models.AppSettings, error) { return Load() }

// Update is the `UpdateSettings` command body: normalise, persist, echo back.
func Update(s models.AppSettings) (models.AppSettings, error) {
	s = Normalize(s)
	if err := Save(s); err != nil {
		return models.AppSettings{}, err
	}
	return s, nil
}
