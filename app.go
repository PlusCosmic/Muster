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
	// checkForUpdates looks for a newer release, reporting whether one exists
	// (and starting its install when it does); nil when this build does not
	// update itself (dev builds, the Arch package).
	checkForUpdates func() (bool, error)
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

// CheckForUpdates looks for a newer release and reports whether one exists;
// when it does, the update window opens and installs it. A failed check (no
// network, bad manifest) is returned so the UI can say so. Always false when
// SelfUpdates is false.
func (a *App) CheckForUpdates() (bool, error) {
	if a.checkForUpdates == nil {
		return false, nil
	}
	return a.checkForUpdates()
}
