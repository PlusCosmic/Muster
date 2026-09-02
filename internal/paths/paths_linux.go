package paths

import "path/filepath"

func detectSteamRoot() string {
	h := home()
	if h == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(h, ".steam/steam"),
		filepath.Join(h, ".local/share/Steam"),
		filepath.Join(h, ".steam/root"),
		filepath.Join(h, ".var/app/com.valvesoftware.Steam/data/Steam"),
	}
	for _, c := range candidates {
		if isDir(filepath.Join(c, "steamapps")) {
			return c
		}
	}
	return ""
}

// GameBinaryName is the main binary inside the RimWorld install directory.
func GameBinaryName() string { return "RimWorldLinux" }

// defaultSavedataDir is the vanilla `-savedatafolder` the game uses unmanaged.
func defaultSavedataDir() string {
	h := home()
	if h == "" {
		return ""
	}
	return filepath.Join(h, ".config", "unity3d", "Ludeon Studios", "RimWorld by Ludeon Studios")
}
