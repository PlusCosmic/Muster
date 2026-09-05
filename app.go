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
type App struct{}

// RevealPath shows a path in the system file manager.
func (a *App) RevealPath(path string) error {
	return application.Get().Env.OpenFileManager(path, true)
}

// GetAppInfo reports the running version and where the app keeps its data.
func (a *App) GetAppInfo() (models.AppInfo, error) {
	return models.AppInfo{Version: version.Version, DataRoot: appdir.DataRoot()}, nil
}
