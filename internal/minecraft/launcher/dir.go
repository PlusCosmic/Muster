package launcher

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultDir is where the official launcher keeps `.minecraft` on this OS, or
// "" if the home directory is unknown. It may not exist yet.
func DefaultDir() string {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("APPDATA"); v != "" {
			return filepath.Join(v, ".minecraft")
		}
		return ""
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "minecraft")
		}
		return ""
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".minecraft")
		}
		return ""
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
