package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"

	"muster/internal/appdir"
	"muster/internal/minecraft"
	"muster/internal/rimworld"
	"muster/internal/version"
)

// The SvelteKit static build (frontend/dist) is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// The trust root for self-updates: releases must be signed by the matching
// private key, which lives in the release pipeline, never here.
//
//go:embed build/updater.pub
var updaterPublicKey []byte

// updateCheckInterval is how often a release build looks for updates while
// running. The first check happens shortly after start.
const updateCheckInterval = 6 * time.Hour

func main() {
	if runtime.GOOS == "linux" && os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		// WebKitGTK's DMA-BUF renderer crashes Wayland on NVIDIA (Gdk Error 71).
		// Wails only sets this when it detects an NVIDIA GPU; set it
		// unconditionally like the Tauri build did, so hybrid setups are
		// covered too.
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}

	// Adopt a RimForge (the RimWorld-only predecessor) data directory before
	// anything reads from the data root. A failed migration stops the app:
	// running on with a half-moved root would let the user write new data
	// there, and the next start would then refuse (or skip) the retry. The
	// legacy directory is left intact and the migration resumes next start.
	// The error is shown in a native dialog once the app is up, because the
	// production Windows build has no console for stderr.
	migrated, migrateErr := appdir.MigrateLegacy()
	if migrateErr != nil {
		log.Printf("muster: could not migrate the RimForge data directory: %v", migrateErr)
	} else if migrated {
		log.Printf("muster: migrated RimForge data to %s", appdir.GameRoot(appdir.RimWorld))
	}

	mc := &minecraft.Service{}
	appSvc := &App{}
	app := application.New(application.Options{
		Name:        "Muster",
		Description: "Shared mod setups for RimWorld and Minecraft",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(appSvc),
			application.NewService(&rimworld.Service{}),
			application.NewService(mc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			ProgramName: "muster",
		},
	})

	mc.Emit = func(name string, data any) { app.Event.Emit(name, data) }
	appSvc.checkForUpdates = setupUpdater(app)

	if migrateErr != nil {
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
			app.Dialog.Error().
				SetTitle("Muster could not move your RimForge data").
				SetMessage(fmt.Sprintf("%v\n\nClose anything that is using that folder and start Muster again. Nothing has been lost.", migrateErr)).
				Show()
			app.Quit()
		})
		if err := app.Run(); err != nil {
			log.Fatal(err)
		}
		os.Exit(1)
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Muster",
		Width:            1200,
		Height:           800,
		MinWidth:         560,
		MinHeight:        520,
		BackgroundColour: application.NewRGB(24, 22, 20),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// setupUpdater wires the self-updater on platforms where nothing else
// updates the app (Linux builds come from a package manager), and returns
// the "Check for updates" action for the App service (nil when updates are
// off). The Updater's own periodic loop opens the update window even when up
// to date, so the timer here runs a silent Check and only opens the window
// when there is something to install.
func setupUpdater(app *application.App) func() {
	if runtime.GOOS == "linux" || os.Getenv("MUSTER_NO_SELF_UPDATE") != "" {
		return nil
	}
	provider, err := endpoint.New(endpoint.Config{URL: version.UpdateManifestURL, Channel: "stable"})
	if err != nil {
		log.Printf("muster: updater disabled: %v", err)
		return nil
	}
	err = app.Updater.Init(updater.Config{
		CurrentVersion: version.Version,
		Providers:      []updater.Provider{provider},
		PublicKey:      updaterPublicKey,
		Window:         &updater.BuiltinWindow{},
	})
	if err != nil {
		log.Printf("muster: updater disabled: %v", err)
		return nil
	}
	install := func() {
		if err := app.Updater.CheckAndInstall(context.Background()); err != nil {
			log.Printf("muster: update: %v", err)
		}
	}
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			// A short delay so the main window is up before an update window.
			for delay := 10 * time.Second; ; delay = updateCheckInterval {
				time.Sleep(delay)
				rel, err := app.Updater.Check(context.Background())
				if err != nil {
					log.Printf("muster: update check: %v", err)
					continue
				}
				if rel != nil {
					install()
				}
			}
		}()
	})
	return func() { go install() }
}
