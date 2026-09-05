//go:build !windows

package launcher

// ps -eo comm lists the executable name of every process; the launcher's is
// minecraft-launcher on Linux and "Minecraft" for the macOS app bundle.
var processListCommand = []string{"ps", "-eo", "comm="}

var launcherProcessNames = []string{"minecraft-launcher\n", "\nMinecraft\n"}
