package main

import (
	"os"
	"path/filepath"
	"testing"

	"rimforge/internal/models"
)

// Drives the bound service end to end against a scratch data directory,
// the way the frontend does — everything except RevealPath, which needs a
// running window, and LaunchProfile, which would start Steam.
func TestServiceRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RIMFORGE_DATA_DIR", root)
	// A home with no Steam, so detection finds nothing and never touches
	// the machine running the tests.
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "home", ".local", "share"))

	// Pre-seed an empty rules cache so SortMods never reaches the network.
	if err := os.MkdirAll(filepath.Join(root, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cache", "communityRules.json"), []byte(`{"rules":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var app App

	s, err := app.UpdateSettings(models.Settings{SteamRootOverride: models.Str("  ")})
	if err != nil || s.SteamRootOverride != nil {
		t.Fatalf("UpdateSettings: %+v, %v", s, err)
	}
	if got, _ := app.GetSettings(); got != s {
		t.Fatalf("GetSettings: %+v", got)
	}

	detected, err := app.DetectPaths()
	if err != nil || detected.ProfilesDir != filepath.Join(root, "profiles") || detected.WorkshopDirs == nil {
		t.Fatalf("DetectPaths: %+v, %v", detected, err)
	}

	if list, err := app.ListProfiles(); err != nil || list == nil || len(list) != 0 {
		t.Fatalf("ListProfiles on empty data dir: %#v, %v", list, err)
	}

	p, err := app.CreateProfile("Smoke")
	if err != nil || p.ID != "smoke" || p.ActiveModCount != 1 {
		t.Fatalf("CreateProfile: %+v, %v", p, err)
	}

	active, err := app.GetActiveMods(p.ID)
	if err != nil || len(active.ActiveIDs) != 1 || active.ActiveIDs[0] != "ludeon.rimworld" || active.KnownExpansions == nil {
		t.Fatalf("GetActiveMods: %+v, %v", active, err)
	}
	if err := app.SetActiveMods(p.ID, []string{"Ludeon.RimWorld", "brrainz.harmony"}); err != nil {
		t.Fatal(err)
	}
	active, _ = app.GetActiveMods(p.ID)
	if len(active.ActiveIDs) != 2 || active.ActiveIDs[1] != "brrainz.harmony" {
		t.Fatalf("SetActiveMods did not persist: %+v", active)
	}

	// Nothing is installed here, so both ids are unknown; the seeded cache
	// stands in for the community rules database.
	sorted, err := app.SortMods(active.ActiveIDs)
	if err != nil || len(sorted.Sorted) != 2 || len(sorted.Warnings) != 2 {
		t.Fatalf("SortMods: %+v, %v", sorted, err)
	}
	if st, _ := app.GetRulesDbStatus(); !st.Cached || st.RuleCount != 0 {
		t.Fatalf("GetRulesDbStatus: %+v", st)
	}

	mods, err := app.ListInstalledMods()
	if err != nil || mods == nil || len(mods) != 0 {
		t.Fatalf("ListInstalledMods: %#v, %v", mods, err)
	}

	cloned, err := app.CloneProfile(p.ID, "Smoke")
	if err != nil || cloned.ID != "smoke-2" || cloned.ActiveModCount != 2 {
		t.Fatalf("CloneProfile: %+v, %v", cloned, err)
	}
	if _, err := app.RenameProfile(cloned.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ImportDefault("Imported"); err == nil {
		t.Fatal("ImportDefault with no default savedata should fail")
	}

	// Delete goes through the real trash implementation; with XDG_DATA_HOME
	// pointed inside the scratch root (and gio absent or refusing a path on
	// another filesystem) nothing leaves the sandbox.
	if err := app.DeleteProfile(cloned.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "profiles", cloned.ID)); !os.IsNotExist(err) {
		t.Fatal("deleted profile directory should be gone")
	}
	if list, _ := app.ListProfiles(); len(list) != 1 || list[0].ID != "smoke" {
		t.Fatalf("ListProfiles after delete: %+v", list)
	}
}
