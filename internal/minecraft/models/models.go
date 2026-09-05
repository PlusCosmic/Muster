// Package models holds every struct that crosses the Go/frontend boundary
// for the Minecraft game module. Same conventions as internal/models.
package models

// Settings is the module's settings.json.
type Settings struct {
	// ManifestURLOverride replaces the built-in manifest URL (which is baked in
	// at build time and may be empty).
	ManifestURLOverride *string `json:"manifestUrlOverride"`
	// MinecraftDirOverride replaces the detected `.minecraft` directory.
	MinecraftDirOverride *string `json:"minecraftDirOverride"`
}

// Detected is what the module found on this machine.
type Detected struct {
	// ManifestURL is the effective manifest URL, or nil when none is configured.
	ManifestURL *string `json:"manifestUrl"`
	// MinecraftDir is the effective `.minecraft` directory, or nil if unknown.
	MinecraftDir *string `json:"minecraftDir"`
	// LauncherInstalled: the launcher has run at least once in MinecraftDir.
	LauncherInstalled bool `json:"launcherInstalled"`
	// PacksDir is where packs are installed.
	PacksDir string `json:"packsDir"`
}

// Pack is a manifest entry plus what is installed locally. Everything from
// the manifest is present even when nothing is installed.
type Pack struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        *string  `json:"icon"`
	PackURL     string   `json:"packUrl"`
	Server      *string  `json:"server"`
	MinMemoryMb int      `json:"minMemoryMb"`
	MaxMemoryMb int      `json:"maxMemoryMb"`
	JavaArgs    []string `json:"javaArgs"`

	InstallDir       string  `json:"installDir"`
	Installed        bool    `json:"installed"`
	InstalledVersion *string `json:"installedVersion"`
	SyncedAtMs       *int64  `json:"syncedAtMs"`
	// ProfileWritten: the launcher has our profile for this pack.
	ProfileWritten bool `json:"profileWritten"`
}

// PackCheck is the result of looking at the pack's current upstream state
// without changing anything.
type PackCheck struct {
	ID            string `json:"id"`
	LatestVersion string `json:"latestVersion"`
	Minecraft     string `json:"minecraft"`
	Loader        string `json:"loader"`
	LoaderVersion string `json:"loaderVersion"`
	// VersionID is the launcher installation id the profile needs.
	VersionID string `json:"versionId"`
	// LoaderInstalled: the launcher already has that installation.
	LoaderInstalled bool `json:"loaderInstalled"`
	ToDownload      int  `json:"toDownload"`
	ToDelete        int  `json:"toDelete"`
	UpToDate        bool `json:"upToDate"`
}

// SyncProgress is emitted as the `minecraft:sync` event during SyncPack.
// Phase is "files" (Done/Total count downloads), "loader" (Current is a
// step description; Done/Total are 0), or "profile".
type SyncProgress struct {
	ID      string `json:"id"`
	Phase   string `json:"phase"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Current string `json:"current"`
}

// Manual is a file the user has to download themselves.
type Manual struct {
	Path string `json:"path"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Why  string `json:"why"`
}

// SyncReport is what SyncPack did.
type SyncReport struct {
	ID             string   `json:"id"`
	Version        string   `json:"version"`
	Downloaded     []string `json:"downloaded"`
	Deleted        []string `json:"deleted"`
	Manual         []Manual `json:"manual"`
	ProfileWritten bool     `json:"profileWritten"`
	// LoaderInstalled: the launcher has the loader installation the profile
	// points at (installed during this sync if it was missing).
	LoaderInstalled bool `json:"loaderInstalled"`
	// VersionID is that installation's id.
	VersionID string `json:"versionId"`
}
