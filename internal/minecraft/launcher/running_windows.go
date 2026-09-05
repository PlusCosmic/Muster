package launcher

// tasklist lists every process with its image name; the Store build runs as
// Minecraft.exe, the legacy MSI install as MinecraftLauncher.exe.
var processListCommand = []string{"tasklist", "/NH", "/FO", "CSV"}

var launcherProcessNames = []string{`"Minecraft.exe"`, `"MinecraftLauncher.exe"`}
