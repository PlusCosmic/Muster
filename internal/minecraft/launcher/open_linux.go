package launcher

// openCommands lists ways to start the launcher, tried in order. The
// minecraft.net .deb/.rpm/AUR builds install `minecraft-launcher`; Flatpak
// users have com.mojang.Minecraft.
func openCommands() [][]string {
	return [][]string{
		{"minecraft-launcher"},
		{"flatpak", "run", "com.mojang.Minecraft"},
	}
}
