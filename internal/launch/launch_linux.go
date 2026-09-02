package launch

import "rimforge/internal/paths"

func command(profilePath string) (string, []string, error) {
	return "steam", []string{"-applaunch", paths.RimWorldAppID, SavedataArg(profilePath)}, nil
}
