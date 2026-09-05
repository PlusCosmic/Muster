// Package machine reports facts about the computer Muster runs on that a pack
// cannot know: today, how much memory it has. A pack only recommends a Java
// heap; the machine decides what it can actually give.
package machine

// TotalMemoryMb is the installed physical memory in MiB, or 0 if unknown.
func TotalMemoryMb() int { return totalMemoryMb() }

// MaxHeapMb is the largest Java heap Muster will offer for a machine with
// totalMb of memory: three quarters of it, leaving room for the OS, the
// launcher and the game's own off-heap use, floored at MinHeapMb. 0 in ⇒ 0 out.
func MaxHeapMb(totalMb int) int {
	if totalMb <= 0 {
		return 0
	}
	m := totalMb * 3 / 4
	m -= m % HeapStepMb
	if m < MinHeapMb {
		return MinHeapMb
	}
	return m
}

// MinHeapMb and HeapStepMb bound and quantise the memory slider.
const (
	MinHeapMb  = 1024
	HeapStepMb = 512
)
