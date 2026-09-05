// Package models holds every struct that crosses the Go/frontend boundary
// for the Minecraft game module. Same conventions as internal/models.
package models

// Settings is the module's settings.json.
type Settings struct {
	// Codes are the pack codes the user has entered, with what each resolved
	// to last time, so the pack list works when the registry is unreachable.
	Codes []PackCode `json:"codes"`
	// ManifestURL is an optional pack list (a manifest) the user was given.
	// Muster ships with none: the app knows nothing about any particular pack.
	ManifestURL *string `json:"manifestUrl"`
	// RegistryURLOverride replaces the public pack registry (self-hosters).
	RegistryURLOverride *string `json:"registryUrlOverride"`
	// MinecraftDirOverride replaces the detected `.minecraft` directory.
	MinecraftDirOverride *string `json:"minecraftDirOverride"`
	// Packs holds each pack's launch settings, by pack id, once the user has
	// touched them. Absent ⇒ derived from the pack's recommendation and this
	// machine's memory (see LaunchSettings).
	Packs map[string]LaunchSettings `json:"packs"`
}

// PackCode is one entered code and the registration it resolved to.
type PackCode struct {
	Code      string `json:"code"`
	AddedAtMs int64  `json:"addedAtMs"`
	// Pack is the registration's pack entry as last seen, as JSON of the
	// manifest entry shape. Kept opaque here so models stays free of the
	// manifest package; the service decodes it.
	Pack []byte `json:"pack"`
}

// LaunchSettings is how a pack is launched on this machine. A pack only
// recommends; these are what the launcher profile actually gets.
type LaunchSettings struct {
	// MaxMemoryMb is the Java heap (-Xmx). Clamped to Detected.maxHeapMb.
	MaxMemoryMb int `json:"maxMemoryMb"`
	// MinMemoryMb is -Xms when set; nil lets the JVM start small and grow.
	MinMemoryMb *int `json:"minMemoryMb"`
	// Args are the extra JVM options.
	Args []string `json:"args"`
	// FollowRecommendedArgs: Args track the pack's recommendation as it
	// changes. Editing them pins them (false).
	FollowRecommendedArgs bool `json:"followRecommendedArgs"`
}

// Detected is what the module found on this machine.
type Detected struct {
	// ManifestURL is the configured manifest URL, or nil when none is.
	ManifestURL *string `json:"manifestUrl"`
	// RegistryURL is the pack registry in use.
	RegistryURL string `json:"registryUrl"`
	// MinecraftDir is the effective `.minecraft` directory, or nil if unknown.
	MinecraftDir *string `json:"minecraftDir"`
	// LauncherInstalled: the launcher has run at least once in MinecraftDir.
	LauncherInstalled bool `json:"launcherInstalled"`
	// PacksDir is where packs are installed.
	PacksDir string `json:"packsDir"`
	// TotalMemoryMb is this machine's physical memory; 0 if unknown.
	TotalMemoryMb int `json:"totalMemoryMb"`
	// MaxHeapMb is the largest heap the memory slider offers (about three
	// quarters of TotalMemoryMb); 0 if unknown.
	MaxHeapMb int `json:"maxHeapMb"`
}

// Pack is a manifest entry plus what is installed locally. Everything from
// the manifest is present even when nothing is installed.
type Pack struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Source is "code" (entered pack code) or "manifest" (from the pack list).
	Source string `json:"source"`
	// Code is the pack code this came from, when Source is "code".
	Code        *string `json:"code"`
	Description string  `json:"description"`
	Icon        *string `json:"icon"`
	PackURL     string  `json:"packUrl"`
	Server      *string `json:"server"`
	// What the pack's author recommends; advisory.
	RecommendedMinMemoryMb int      `json:"recommendedMinMemoryMb"`
	RecommendedMaxMemoryMb int      `json:"recommendedMaxMemoryMb"`
	RecommendedArgs        []string `json:"recommendedArgs"`
	// Launch is what this machine will actually use: the saved settings, or
	// the recommendation fitted to this machine when nothing is saved yet.
	Launch LaunchSettings `json:"launch"`
	// LaunchCustomised: the user has saved launch settings for this pack.
	LaunchCustomised bool `json:"launchCustomised"`

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
	// LauncherOpen: the launcher was running when the profile was written, so
	// it has to be closed and reopened before the pack shows up in it.
	LauncherOpen bool `json:"launcherOpen"`
}
