package main

import (
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"muster/internal/appdir"
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
	if moved, err := appdir.MigrateLegacy(); err != nil {
		log.Fatalf("muster: could not migrate the RimForge data directory: %v\n"+
			"Close anything using it and start Muster again; nothing has been lost.", err)
	} else if moved {
		log.Printf("muster: migrated RimForge data to %s", appdir.GameRoot(appdir.RimWorld))
	}

	app := application.New(application.Options{
		Name:        "Muster",
		Description: "Shared mod setups for RimWorld and Minecraft",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(&App{}),
			application.NewService(&rimworld.Service{}),
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
