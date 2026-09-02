// Package launch starts a profile through Steam.
//
// Steam-mediated so the game keeps its Steam context (Workshop, achievements).
// We spawn and return immediately — the app never waits on RimWorld.
package launch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"rimforge/internal/appdir"
	"rimforge/internal/profiles"
)

// SavedataArg is the `-savedatafolder=<abs path>` argument for a profile.
func SavedataArg(profilePath string) string {
	return "-savedatafolder=" + profilePath
}

// Profile is the `launch_profile` command body.
func Profile(id string) error {
	// Errors if the profile is unknown.
	if _, err := profiles.Find(id); err != nil {
		return err
	}
	dir := appdir.ProfileDir(id)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("profile directory missing: %s", dir)
	}

	program, args, err := command(dir)
	if err != nil {
		return err
	}

	cmd := exec.Command(program, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("Steam not found (could not run `%s`) — is Steam installed and on PATH?", program)
		}
		return fmt.Errorf("could not launch Steam: %w", err)
	}
	// Detached: reap in the background so the child never lingers as a
	// zombie, but never block on it.
	go func() { _ = cmd.Wait() }()

	return profiles.TouchLastPlayed(id)
}
