package launcher

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultDir is where the official launcher keeps `.minecraft` on this OS, or
// "" if the home directory is unknown. It may not exist yet. On Linux the
// Flatpak launcher (com.mojang.Minecraft) keeps its own under
// ~/.var/app/…/.minecraft; that wins when it is the one that has actually run.
func DefaultDir() string {
	candidates := candidateDirs()
	if len(candidates) == 0 {
		return ""
	}
	for _, c := range candidates {
		if Installed(c) {
			return c
		}
	}
	return candidates[0]
}

func candidateDirs() []string {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("APPDATA"); v != "" {
			return []string{filepath.Join(v, ".minecraft")}
		}
		return nil
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return []string{filepath.Join(home, "Library", "Application Support", "minecraft")}
		}
		return nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []string{
			filepath.Join(home, ".minecraft"),
			filepath.Join(home, ".var", "app", "com.mojang.Minecraft", ".minecraft"),
		}
	}
}

// Installed reports whether dir looks like a launcher that has run at least
// once: it has a profiles file or a versions directory.
func Installed(dir string) bool {
	if dir == "" {
		return false
	}
	for _, name := range []string{ProfilesFile, StoreProfilesFile, "versions"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}
