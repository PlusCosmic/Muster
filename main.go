package main

import (
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
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

	app := application.New(application.Options{
		Name:        "RimForge",
		Description: "Profile and mod list manager for RimWorld",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(&App{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			ProgramName: "rimforge",
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "RimForge",
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
