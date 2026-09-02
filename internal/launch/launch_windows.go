package launch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"rimforge/internal/models"
	"rimforge/internal/paths"
)

func command(profilePath string) (string, []string, error) {
	steamRoot := models.Deref(paths.Detect().SteamRoot)
	if steamRoot == "" {
		return "", nil, errors.New("Steam installation not found")
	}
	exe := filepath.Join(steamRoot, "steam.exe")
	if _, err := os.Stat(exe); err != nil {
		return "", nil, fmt.Errorf("Steam executable not found at %s", exe)
	}
	return exe, []string{"-applaunch", paths.RimWorldAppID, SavedataArg(profilePath)}, nil
}
