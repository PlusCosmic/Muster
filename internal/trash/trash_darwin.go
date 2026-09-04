package trash

import (
	"fmt"
	"os/exec"
	"strings"
)

// move asks Finder to delete the item, which is how files reach the Trash
// with Put Back support.
func move(abs string) error {
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(abs)
	script := fmt.Sprintf(`tell application "Finder" to delete POSIX file "%s"`, quoted)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not move %s to trash: %s", abs, strings.TrimSpace(string(out)))
	}
	return nil
}
