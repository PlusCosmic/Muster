// Package models holds every struct that crosses the Go/frontend boundary
// for the Minecraft game module. Same conventions as internal/models.
package models

// Settings is the module's settings.json.
type Settings struct {
	// Codes are the pack codes the user has entered, with what each resolved
	// to last time, so the pack list works when the registry is unreachable.
	Codes []PackCode `json:"codes"`
	// Modrinth is every Modrinth modpack the user added by link, with what
	// each resolved to last time.
	Modrinth []ModrinthPack `json:"modrinth"`
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

// ModrinthPack is one Modrinth modpack the user added.
type ModrinthPack struct {
	// ProjectID is Modrinth's stable id for the project; lookups use it.
	ProjectID string `json:"projectId"`
	// Slug is the project's slug when it was added, for display.
	Slug string `json:"slug"`
	// PackID is the pack's id (and so its install directory and launcher
	// profile): `modrinth-<slug>` with the slug reduced to [a-z0-9-], plus
	// the project id when another added pack's slug reduces the same.
	// Fixed at add time, so a rename on Modrinth does not orphan the
	// install.
	PackID string `json:"packId"`
	// Version is the version number the pack is held at. A sync installs
	// exactly this; it only changes when the user picks another (or takes
	// an update), never because Modrinth published one.
	Version   string `json:"version"`
	AddedAtMs int64  `json:"addedAtMs"`
	// Pack is the project as last seen, in the manifest entry shape (like
	// PackCode.Pack), so the pack stays listed when Modrinth is unreachable.
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
	// Source is "code" (entered pack code), "modrinth" (a Modrinth modpack
	// added by link) or "manifest" (from the pack list).
	Source string `json:"source"`
	// Code is the pack code this came from, when Source is "code".
	Code *string `json:"code"`
	// Project is the Modrinth project slug, when Source is "modrinth".
	Project *string `json:"project"`
	// HeldVersion is the Modrinth version a sync installs, when Source is
	// "modrinth". Updating is an explicit choice (SetModrinthVersion).
	HeldVersion *string `json:"heldVersion"`
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
	ID string `json:"id"`
	// LatestVersion is the newest the source offers: pack.toml's version for
	// a packwiz pack, the newest release for a Modrinth pack.
	LatestVersion string `json:"latestVersion"`
	// TargetVersion is what a sync installs now. For a packwiz pack it is
	// LatestVersion; for a Modrinth pack it is the held version.
	TargetVersion string `json:"targetVersion"`
	// UpdateAvailable: a newer version exists than the one a sync would
	// install (Modrinth packs only; the user chooses whether to take it).
	UpdateAvailable bool   `json:"updateAvailable"`
	Minecraft       string `json:"minecraft"`
	Loader          string `json:"loader"`
	LoaderVersion   string `json:"loaderVersion"`
	// VersionID is the launcher installation id the profile needs.
	VersionID string `json:"versionId"`
	// LoaderInstalled: the launcher already has that installation.
	LoaderInstalled bool `json:"loaderInstalled"`
	ToDownload      int  `json:"toDownload"`
	ToDelete        int  `json:"toDelete"`
	// UpToDate: the install matches TargetVersion and no file needs work.
	UpToDate bool `json:"upToDate"`
}

// ModrinthVersion is one published version of a Modrinth modpack, for the
// version picker.
type ModrinthVersion struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	// Type is "release", "beta" or "alpha".
	Type          string   `json:"type"`
	PublishedAtMs int64    `json:"publishedAtMs"`
	GameVersions  []string `json:"gameVersions"`
	Loaders       []string `json:"loaders"`
}

// ModrinthLookup is what a pasted Modrinth link points at, before it is
// added: enough to show the pack and pick a version.
type ModrinthLookup struct {
	// Project is the slug; Input is the link as pasted, to hand back to
	// AddModrinthPack.
	Project     string  `json:"project"`
	Input       string  `json:"input"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Icon        *string `json:"icon"`
	PageURL     string  `json:"pageUrl"`
	// Versions, newest first. Suggested is the one preselected: the link's
	// version when it named one, else the newest release.
	Versions  []ModrinthVersion `json:"versions"`
	Suggested string            `json:"suggested"`
	// AlreadyAdded: this project is in the list already (at HeldVersion).
	AlreadyAdded bool    `json:"alreadyAdded"`
	HeldVersion  *string `json:"heldVersion"`
}

// SyncProgress is emitted as the `minecraft:sync` event during SyncPack.
// Phase is "files" (Done/Total count downloads), "loader" (Current is a
// step description; Done/Total are 0), "profile", or "waiting" (Current
// says what the network is being waited on for; the previous phase resumes
// after).
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
