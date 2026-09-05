package paths

import "path/filepath"

func detectSteamRoot() string {
	h := home()
	if h == "" {
		return ""
	}
	c := filepath.Join(h, "Library/Application Support/Steam")
	if isDir(filepath.Join(c, "steamapps")) {
		return c
	}
	return ""
}

// GameBinaryName is the main binary inside the RimWorld install directory.
func GameBinaryName() string { return "RimWorldMac.app" }

// defaultSavedataDir is the vanilla `-savedatafolder` the game uses unmanaged.
func defaultSavedataDir() string {
	h := home()
	if h == "" {
		return ""
	}
	return filepath.Join(h, "Library/Application Support/RimWorld")
}
