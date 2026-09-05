package minecraft

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"muster/internal/appdir"
	"muster/internal/minecraft/launcher"
	"muster/internal/minecraft/models"
)

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

// normalizeSettings trims every override and drops the blank ones.
func normalizeSettings(s models.Settings) models.Settings {
	s.ManifestURLOverride = blankToNil(s.ManifestURLOverride)
	s.MinecraftDirOverride = blankToNil(s.MinecraftDirOverride)
	return s
}

// loadSettings reads settings.json. Missing or malformed ⇒ zero value (a
// corrupt settings file must never brick the app).
func loadSettings() models.Settings {
	raw, err := os.ReadFile(SettingsPath())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("muster: could not read %s: %v", SettingsPath(), err)
		}
		return models.Settings{}
	}
	var s models.Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		log.Printf("muster: malformed %s: %v", SettingsPath(), err)
		return models.Settings{}
	}
	return normalizeSettings(s)
}

func saveSettings(s models.Settings) error {
	if err := appdir.EnsureDir(filepath.Dir(SettingsPath())); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("could not serialise settings: %w", err)
	}
	return appdir.WriteFileAtomic(SettingsPath(), data)
}

// manifestURL is the effective manifest URL, or "".
func manifestURL(s models.Settings) string {
	if s.ManifestURLOverride != nil {
		return *s.ManifestURLOverride
	}
	return DefaultManifestURL
}

// minecraftDir is the effective `.minecraft` directory, or "".
func minecraftDir(s models.Settings) string {
	if s.MinecraftDirOverride != nil {
		return *s.MinecraftDirOverride
	}
	return launcher.DefaultDir()
}
