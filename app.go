package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"muster/internal/appdir"
	"muster/internal/models"
	"muster/internal/version"
)

// App is the game-neutral service bound to the frontend: things about the
// app itself. Each game module binds its own service (internal/rimworld).
// Every method here is a command in docs/ARCHITECTURE.md; `wails3 generate
// bindings` turns them into the typed TypeScript in frontend/bindings.
type App struct {
	// checkForUpdates opens the update window and installs any newer release;
	// nil when this build has no update source (dev builds, the Arch package).
	checkForUpdates func()
}

// RevealPath shows a path in the system file manager.
func (a *App) RevealPath(path string) error {
	return application.Get().Env.OpenFileManager(path, true)
}

// GetAppInfo reports the running version, where the app keeps its data, and
// whether it can update itself.
func (a *App) GetAppInfo() (models.AppInfo, error) {
	return models.AppInfo{Version: version.Version, DataRoot: appdir.DataRoot(), SelfUpdates: a.checkForUpdates != nil}, nil
}

// CheckForUpdates opens the update window, checks for a newer release and
// installs it if there is one. No-op when SelfUpdates is false.
func (a *App) CheckForUpdates() error {
	if a.checkForUpdates == nil {
		return nil
	}
	a.checkForUpdates()
	return nil
}
