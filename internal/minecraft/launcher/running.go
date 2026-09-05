package launcher

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Running reports whether the official launcher is open. It reads its
// installation list on start, so a profile added while it is open only shows
// once it has been closed and reopened (seen on the Windows Store build;
// nothing is lost, the file on disk is right). Errors from the process
// listing count as "not running": the check is advisory.
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
