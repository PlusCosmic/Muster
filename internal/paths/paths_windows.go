package paths

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func detectSteamRoot() string {
	probes := []struct {
		hive   registry.Key
		subkey string
		value  string
	}{
		{registry.CURRENT_USER, `SOFTWARE\Valve\Steam`, "SteamPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath"},
		{registry.CURRENT_USER, `SOFTWARE\Valve\Steam`, "InstallPath"},
	}
	for _, p := range probes {
		key, err := registry.OpenKey(p.hive, p.subkey, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		raw, _, err := key.GetStringValue(p.value)
		key.Close()
		if err != nil {
			continue
		}
		path := strings.ReplaceAll(raw, "/", `\`)
		if isDir(filepath.Join(path, "steamapps")) {
			return path
		}
	}
	fallback := `C:\Program Files (x86)\Steam`
	if isDir(filepath.Join(fallback, "steamapps")) {
		return fallback
	}
	return ""
}

// GameBinaryName is the main binary inside the RimWorld install directory.
func GameBinaryName() string { return "RimWorldWin64.exe" }

// defaultSavedataDir is the vanilla `-savedatafolder` the game uses unmanaged.
func defaultSavedataDir() string {
	h := home()
	if h == "" {
		return ""
	}
	return filepath.Join(h, `AppData\LocalLow\Ludeon Studios\RimWorld by Ludeon Studios`)
}
