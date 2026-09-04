// Package trash moves files and directories to the OS trash / recycle bin
// rather than deleting them, so a user can recover a profile they removed.
package trash

import (
	"fmt"
	"os"
	"path/filepath"
)

// Move sends path to the system trash. The path must exist.
func Move(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("could not resolve %s: %w", path, err)
	}
	if _, err := os.Lstat(abs); err != nil {
		return fmt.Errorf("could not move %s to trash: %w", abs, err)
	}
	return move(abs)
}
