package launcher

import (
	"os"
	"path/filepath"
)

// openCommands: the Store/unified launcher registers an AppsFolder entry; the
// legacy MSI installer puts MinecraftLauncher.exe under Program Files (x86).
// The AppsFolder id is the launcher's package family name and app id.
func openCommands() [][]string {
	cmds := [][]string{
		{"explorer.exe", `shell:AppsFolder\Microsoft.4297127D64EC6_8wekyb3d8bbwe!Minecraft`},
	}
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		if base := os.Getenv(env); base != "" {
			exe := filepath.Join(base, "Minecraft Launcher", "MinecraftLauncher.exe")
			if _, err := os.Stat(exe); err == nil {
				cmds = append([][]string{{exe}}, cmds...)
			}
		}
	}
	return cmds
}
