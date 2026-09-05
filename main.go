package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"muster/internal/appdir"
	"muster/internal/minecraft"
	"muster/internal/rimworld"
)

// The SvelteKit static build (frontend/dist) is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

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
	app := application.New(application.Options{
		Name:        "Muster",
		Description: "Shared mod setups for RimWorld and Minecraft",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(&App{}),
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
