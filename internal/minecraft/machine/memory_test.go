package machine

import "testing"

func TestMaxHeapMb(t *testing.T) {
	cases := map[int]int{0: 0, 2048: 1536, 1200: MinHeapMb, 8192: 6144, 16384: 12288, 32768: 24576, 12000: 8704}
	for in, want := range cases {
		if got := MaxHeapMb(in); got != want {
			t.Errorf("MaxHeapMb(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestTotalMemoryLooksSane(t *testing.T) {
	mb := TotalMemoryMb()
	if mb == 0 {
		t.Skip("memory not detectable here")
	}
	if mb < 256 || mb > 64*1024*1024 {
		t.Fatalf("implausible total memory: %d MiB", mb)
	}
}
