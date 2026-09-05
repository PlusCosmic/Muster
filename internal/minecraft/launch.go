package minecraft

import (
	"fmt"
	"strings"

	"muster/internal/minecraft/machine"
	"muster/internal/minecraft/manifest"
	"muster/internal/minecraft/models"
)

// defaultHeapMb is the heap offered when a pack recommends none.
const defaultHeapMb = 4096

// clampHeap fits a heap request to the slider's range for a machine with
// maxHeapMb available (0 = unknown, no upper bound), on HeapStepMb steps.
func clampHeap(mb, maxHeapMb int) int {
	if mb <= 0 {
		mb = defaultHeapMb
	}
	mb -= mb % machine.HeapStepMb
	if mb < machine.MinHeapMb {
		mb = machine.MinHeapMb
	}
	if maxHeapMb > 0 && mb > maxHeapMb {
		mb = maxHeapMb
	}
	return mb
}

// effectiveLaunch is what a pack launches with on this machine: the saved
// settings when the user has set any (args still following the recommendation
// unless pinned), otherwise the recommendation fitted to the machine.
func effectiveLaunch(rec manifest.Recommended, saved *models.LaunchSettings, maxHeapMb int) models.LaunchSettings {
	recArgs := append([]string{}, rec.Args...)
	if saved == nil {
		return models.LaunchSettings{
			MaxMemoryMb:           clampHeap(rec.MaxMemoryMb, maxHeapMb),
			MinMemoryMb:           nil,
			Args:                  recArgs,
			FollowRecommendedArgs: true,
		}
	}
	out := *saved
	out.MaxMemoryMb = clampHeap(out.MaxMemoryMb, maxHeapMb)
	if out.MinMemoryMb != nil {
		min := clampHeap(*out.MinMemoryMb, out.MaxMemoryMb)
		out.MinMemoryMb = &min
	}
	if out.FollowRecommendedArgs || out.Args == nil {
		out.Args = recArgs
	}
	return out
}

// validateLaunch rejects settings the launcher cannot carry: it splits
// javaArgs on whitespace with no quoting.
func validateLaunch(ls models.LaunchSettings) error {
	for _, a := range ls.Args {
		if a == "" || strings.ContainsAny(a, " \t\n") {
			return fmt.Errorf("JVM argument %q contains whitespace, which the Minecraft launcher cannot pass on", a)
		}
	}
	if ls.MaxMemoryMb < machine.MinHeapMb {
		return fmt.Errorf("memory must be at least %d MB", machine.MinHeapMb)
	}
	if ls.MinMemoryMb != nil && *ls.MinMemoryMb > ls.MaxMemoryMb {
		return fmt.Errorf("minimum memory cannot exceed the maximum")
	}
	return nil
}

// javaArgs renders launch settings as the launcher's single javaArgs string.
func javaArgs(ls models.LaunchSettings) string {
	var parts []string
	if ls.MinMemoryMb != nil && *ls.MinMemoryMb > 0 {
		parts = append(parts, fmt.Sprintf("-Xms%dM", *ls.MinMemoryMb))
	}
	parts = append(parts, fmt.Sprintf("-Xmx%dM", ls.MaxMemoryMb))
	parts = append(parts, ls.Args...)
	return strings.Join(parts, " ")
}
