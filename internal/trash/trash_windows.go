package trash

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// move uses the shell's recycle-bin API via PowerShell, so the item shows up
// in the Recycle Bin with restore support.
func move(abs string) error {
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	method := "DeleteFile"
	if info.IsDir() {
		method = "DeleteDirectory"
	}
	quoted := strings.ReplaceAll(abs, "'", "''")
	script := fmt.Sprintf(
		"Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.FileIO.FileSystem]::%s('%s', 'OnlyErrorDialogs', 'SendToRecycleBin')",
		method, quoted)
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not move %s to the recycle bin: %s", abs, strings.TrimSpace(string(out)))
	}
	return nil
}
