package machine

import (
	"os/exec"
	"strconv"
	"strings"
)

func totalMemoryMb() int {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	b, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return int(b / (1024 * 1024))
}
