package launcher

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Running reports whether the official launcher is open. It keeps its
// installation list in memory while open and writes it back on exit, so a
// profile added underneath it is ignored or overwritten (seen on the Windows
// Store build: the new profile only appeared once the launcher had been
// closed and reopened). Errors from the process listing count as "not
// running": the check is advisory.
func Running() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, processListCommand[0], processListCommand[1:]...).Output()
	if err != nil {
		return false
	}
	list := strings.ToLower(string(out))
	for _, name := range launcherProcessNames {
		if strings.Contains(list, strings.ToLower(name)) {
			return true
		}
	}
	return false
}
