// Package models holds every struct that crosses the Go/frontend boundary.
//
// Field names are camelCased in JSON, matching the TypeScript mirrors that
// `wails3 generate bindings` writes to frontend/bindings. Optional values are
// pointers (JSON null); list fields must never be nil, because the frontend
// calls array methods on them without checking — use NonNil when building.
package models

type Settings struct {
	SteamRootOverride       *string `json:"steamRootOverride"`
	GameInstallOverride     *string `json:"gameInstallOverride"`
	DefaultSavedataOverride *string `json:"defaultSavedataOverride"`
}

type DetectedPaths struct {
	SteamRoot       *string  `json:"steamRoot"`
	GameInstall     *string  `json:"gameInstall"`
	DefaultSavedata *string  `json:"defaultSavedata"`
	WorkshopDirs    []string `json:"workshopDirs"`
	GameVersion     *string  `json:"gameVersion"`
	ProfilesDir     string   `json:"profilesDir"`
}

type Profile struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Path           string `json:"path"`
	CreatedAtMs    int64  `json:"createdAtMs"`
	LastPlayedAtMs *int64 `json:"lastPlayedAtMs"`
	SaveCount      int    `json:"saveCount"`
	ActiveModCount int    `json:"activeModCount"`
}

// ModSource is where an installed mod was found.
type ModSource string

const (
	SourceOfficial ModSource = "official"
	SourceLocal    ModSource = "local"
	SourceWorkshop ModSource = "workshop"
)

type ModInfo struct {
	PackageID         string    `json:"packageId"`
	Name              string    `json:"name"`
	Authors           string    `json:"authors"`
	Path              string    `json:"path"`
	Source            ModSource `json:"source"`
	SteamWorkshopID   *string   `json:"steamWorkshopId"`
	SupportedVersions []string  `json:"supportedVersions"`
	Dependencies      []string  `json:"dependencies"`
	LoadAfter         []string  `json:"loadAfter"`
	LoadBefore        []string  `json:"loadBefore"`
	ForceLoadAfter    []string  `json:"forceLoadAfter"`
	ForceLoadBefore   []string  `json:"forceLoadBefore"`
	IncompatibleWith  []string  `json:"incompatibleWith"`
}

type ActiveModList struct {
	ActiveIDs       []string `json:"activeIds"`
	KnownExpansions []string `json:"knownExpansions"`
	Version         *string  `json:"version"`
}

type SortWarning struct {
	Kind      string  `json:"kind"`
	PackageID *string `json:"packageId"`
	Message   string  `json:"message"`
}

type SortResult struct {
	Sorted   []string      `json:"sorted"`
	Warnings []SortWarning `json:"warnings"`
}

type RulesDbStatus struct {
	Cached      bool   `json:"cached"`
	FetchedAtMs *int64 `json:"fetchedAtMs"`
	RuleCount   int    `json:"ruleCount"`
}

// Str returns a pointer to s, or nil when s is empty.
func Str(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Deref returns *p, or "" when p is nil.
func Deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Int64 returns a pointer to v.
func Int64(v int64) *int64 { return &v }

// NonNil turns a nil slice into an empty one so it serialises as [] not null.
func NonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
