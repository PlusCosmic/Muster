package launcher

import (
	"errors"
	"fmt"
	"os/exec"
)

// Open starts the official launcher and returns without waiting. The launcher
// shows its profile dropdown with the most recently used entry selected, which
// Upsert has just made ours.
func Open() error {
	candidates := openCommands()
	var lastErr error
	for _, c := range candidates {
		cmd := exec.Command(c[0], c[1:]...)
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		go func() { _ = cmd.Wait() }()
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("no launcher command for this platform")
	}
	if errors.Is(lastErr, exec.ErrNotFound) {
		return fmt.Errorf("the Minecraft launcher was not found — install it from minecraft.net/download")
	}
	return fmt.Errorf("could not start the Minecraft launcher: %w", lastErr)
}
