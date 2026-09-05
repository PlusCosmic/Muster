// Package rimworld is the RimWorld game module: profiles (isolated
// -savedatafolder directories), the installed-mod scan, the mod list editor's
// persistence and the auto-sort. Service is its face to the frontend.
package rimworld

import (
	"muster/internal/rimworld/launch"
	"muster/internal/rimworld/models"
	"muster/internal/rimworld/mods"
	"muster/internal/rimworld/paths"
	"muster/internal/rimworld/profiles"
	"muster/internal/rimworld/settings"
)

// Service is the RimWorld service bound to the frontend. Every method here is
// a command in docs/ARCHITECTURE.md; `wails3 generate bindings` turns them
// into the typed TypeScript in frontend/bindings. Errors reach the frontend as
// rejected promises carrying the message.
type Service struct{}

func (s *Service) GetSettings() (models.Settings, error) { return settings.Get() }

func (s *Service) UpdateSettings(v models.Settings) (models.Settings, error) {
	return settings.Update(v)
}

func (s *Service) DetectPaths() (models.DetectedPaths, error) { return paths.Detect(), nil }

func (s *Service) ListProfiles() ([]models.Profile, error) { return profiles.List() }

func (s *Service) CreateProfile(name string) (models.Profile, error) { return profiles.Create(name) }

func (s *Service) RenameProfile(id, newName string) (models.Profile, error) {
	return profiles.Rename(id, newName)
}

func (s *Service) DeleteProfile(id string) error { return profiles.Delete(id) }

func (s *Service) CloneProfile(id, newName string) (models.Profile, error) {
	return profiles.Clone(id, newName)
}

func (s *Service) ImportDefault(name string) (models.Profile, error) {
	return profiles.ImportDefault(name)
}

func (s *Service) LaunchProfile(id string) error { return launch.Profile(id) }

func (s *Service) ListInstalledMods() ([]models.ModInfo, error) { return mods.ListInstalled() }

func (s *Service) GetActiveMods(profileID string) (models.ActiveModList, error) {
	return mods.GetActive(profileID)
}

func (s *Service) SetActiveMods(profileID string, activeIDs []string) error {
	return mods.SetActive(profileID, activeIDs)
}

func (s *Service) SortMods(activeIDs []string) (models.SortResult, error) {
	return mods.Sort(activeIDs)
}

func (s *Service) RefreshRulesDb() (models.RulesDbStatus, error) { return mods.RefreshRulesDb() }

func (s *Service) GetRulesDbStatus() (models.RulesDbStatus, error) { return mods.RulesDbStatus() }
