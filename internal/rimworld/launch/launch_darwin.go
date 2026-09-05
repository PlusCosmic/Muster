package launch

import "muster/internal/rimworld/paths"

func command(profilePath string) (string, []string, error) {
	return "open", []string{"-a", "Steam", "--args", "-applaunch", paths.RimWorldAppID, SavedataArg(profilePath)}, nil
}
