package minecraft

import "muster/internal/minecraft/manifest"

func manifestJava(min, max int, args []string) manifest.Java {
	return manifest.Java{MinMemoryMb: min, MaxMemoryMb: max, Args: args}
}
