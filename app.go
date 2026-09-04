package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"rimforge/internal/launch"
	"rimforge/internal/models"
	"rimforge/internal/mods"
	"rimforge/internal/paths"
	"rimforge/internal/profiles"
	"rimforge/internal/settings"
)

// App is the service bound to the frontend. Every method here is a command
// in docs/ARCHITECTURE.md; `wails3 generate bindings` turns them into the
// typed TypeScript in frontend/bindings. Errors reach the frontend as
// rejected promises carrying the message.
type App struct{}

// RevealPath shows a path in the system file manager.
func (a *App) RevealPath(path string) error {
	return application.Get().Env.OpenFileManager(path, true)
}

func (a *App) GetSettings() (models.Settings, error) { return settings.Get() }

func (a *App) UpdateSettings(s models.Settings) (models.Settings, error) {
	return settings.Update(s)
}

func (a *App) DetectPaths() (models.DetectedPaths, error) { return paths.Detect(), nil }

func (a *App) ListProfiles() ([]models.Profile, error) { return profiles.List() }

func (a *App) CreateProfile(name string) (models.Profile, error) { return profiles.Create(name) }

func (a *App) RenameProfile(id, newName string) (models.Profile, error) {
	return profiles.Rename(id, newName)
}

func (a *App) DeleteProfile(id string) error { return profiles.Delete(id) }

func (a *App) CloneProfile(id, newName string) (models.Profile, error) {
	return profiles.Clone(id, newName)
}

func (a *App) ImportDefault(name string) (models.Profile, error) {
	return profiles.ImportDefault(name)
}

func (a *App) LaunchProfile(id string) error { return launch.Profile(id) }

func (a *App) ListInstalledMods() ([]models.ModInfo, error) { return mods.ListInstalled() }

func (a *App) GetActiveMods(profileID string) (models.ActiveModList, error) {
	return mods.GetActive(profileID)
}

func (a *App) SetActiveMods(profileID string, activeIDs []string) error {
	return mods.SetActive(profileID, activeIDs)
}

func (a *App) SortMods(activeIDs []string) (models.SortResult, error) {
	return mods.Sort(activeIDs)
}

func (a *App) RefreshRulesDb() (models.RulesDbStatus, error) { return mods.RefreshRulesDb() }

func (a *App) GetRulesDbStatus() (models.RulesDbStatus, error) { return mods.RulesDbStatus() }
