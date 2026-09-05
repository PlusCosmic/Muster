package launch

import "muster/internal/rimworld/paths"

func command(profilePath string) (string, []string, error) {
	return "steam", []string{"-applaunch", paths.RimWorldAppID, SavedataArg(profilePath)}, nil
}
