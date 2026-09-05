package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataRootDefaultsToMusterDir(t *testing.T) {
	t.Setenv(EnvDataDir, "")
	if filepath.Base(DataRoot()) != "muster" {
		t.Fatalf("got %s", DataRoot())
	}
	if got := GameRoot(RimWorld); got != filepath.Join(DataRoot(), "rimworld") {
		t.Fatalf("got %s", got)
	}
}

func TestEnvOverridesRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvDataDir, dir)
	t.Setenv(EnvLegacyDataDir, filepath.Join(dir, "ignored"))
	if DataRoot() != dir {
		t.Fatalf("got %s", DataRoot())
	}
}

func TestLegacyEnvIsHonouredWhenNewOneIsUnset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvDataDir, "")
	t.Setenv(EnvLegacyDataDir, dir)
	if DataRoot() != dir {
		t.Fatalf("got %s", DataRoot())
	}
}

func TestMigrateLegacyMovesRelocatedRootInPlace(t *testing.T) {
	base := t.TempDir()
	root := seedLegacy(t, base, "registry.json", "settings.json", "profiles/vanilla/Config/ModsConfig.xml", "cache/communityRules.json")
	t.Setenv(EnvDataDir, "")
	t.Setenv(EnvLegacyDataDir, root)

	moved, err := MigrateLegacy()
	if err != nil || !moved {
		t.Fatalf("MigrateLegacy: %v, %v", moved, err)
	}
	for _, f := range []string{"registry.json", "settings.json", "profiles/vanilla/Config/ModsConfig.xml", "cache/communityRules.json"} {
		if _, err := os.Stat(filepath.Join(root, "rimworld", f)); err != nil {
			t.Fatalf("%s not migrated: %v", f, err)
		}
		if _, err := os.Stat(filepath.Join(root, f)); !os.IsNotExist(err) {
			t.Fatalf("%s should have moved: %v", f, err)
		}
	}
	if GameRoot(RimWorld) != filepath.Join(root, "rimworld") {
		t.Fatalf("got %s", GameRoot(RimWorld))
	}
	if moved, err := MigrateLegacy(); err != nil || moved {
		t.Fatalf("second run: %v, %v", moved, err)
	}
}

func TestMigrateLegacyInPlaceMovesOnlyRimForgeEntries(t *testing.T) {
	base := t.TempDir()
	root := seedLegacy(t, base, "registry.json", "profiles/a/Config/ModsConfig.xml", "notes.txt", "backups/old.zip")
	t.Setenv(EnvDataDir, "")
	t.Setenv(EnvLegacyDataDir, root)
	if moved, err := MigrateLegacy(); err != nil || !moved {
		t.Fatalf("%v, %v", moved, err)
	}
	for _, f := range []string{"notes.txt", "backups/old.zip"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Fatalf("%s should have stayed put: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "rimworld", "profiles", "a", "Config", "ModsConfig.xml")); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyInPlaceResumesAfterPartialMove(t *testing.T) {
	base := t.TempDir()
	root := seedLegacy(t, base, "registry.json", "profiles/a/x", "cache/communityRules.json", "settings.json")
	t.Setenv(EnvDataDir, "")
	t.Setenv(EnvLegacyDataDir, root)
	// Simulate a run that moved the registry and then died.
	if err := os.MkdirAll(filepath.Join(root, "rimworld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "registry.json"), filepath.Join(root, "rimworld", "registry.json")); err != nil {
		t.Fatal(err)
	}
	if moved, err := MigrateLegacy(); err != nil || !moved {
		t.Fatalf("%v, %v", moved, err)
	}
	for _, f := range []string{"registry.json", "profiles/a/x", "cache/communityRules.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(root, "rimworld", f)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
}

func TestMigrateLegacyInPlaceResumesWhenOnlyMarkersRemain(t *testing.T) {
	base := t.TempDir()
	// A run that moved settings.json and cache/ and then died: the markers
	// (registry, profiles) are still at the root, so the next run finishes.
	root := seedLegacy(t, base, "registry.json", "profiles/a/x", "rimworld/settings.json", "rimworld/cache/communityRules.json")
	t.Setenv(EnvDataDir, "")
	t.Setenv(EnvLegacyDataDir, root)
	if moved, err := MigrateLegacy(); err != nil || !moved {
		t.Fatalf("%v, %v", moved, err)
	}
	for _, f := range []string{"registry.json", "profiles/a/x", "settings.json", "cache/communityRules.json"} {
		if _, err := os.Stat(filepath.Join(root, "rimworld", f)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "registry.json")); !os.IsNotExist(err) {
		t.Fatalf("registry should have moved: %v", err)
	}
}

func TestMigrateLegacyInPlaceRefusesConflicts(t *testing.T) {
	base := t.TempDir()
	root := seedLegacy(t, base, "registry.json", "rimworld/registry.json")
	t.Setenv(EnvDataDir, "")
	t.Setenv(EnvLegacyDataDir, root)
	if _, err := MigrateLegacy(); err == nil {
		t.Fatal("expected an error for an entry present on both sides")
	}
	if _, err := os.Stat(filepath.Join(root, "registry.json")); err != nil {
		t.Fatalf("source should be untouched: %v", err)
	}
}

func TestMigrateLegacyInPlaceLeavesNewLayoutAlone(t *testing.T) {
	root := t.TempDir()
	t.Setenv(EnvDataDir, "")
	t.Setenv(EnvLegacyDataDir, root)
	// Already in Muster's layout, plus Muster's own common settings.json at the
	// root: nothing here is RimForge data, so nothing moves.
	for _, f := range []string{"rimworld/registry.json", "settings.json"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if moved, err := MigrateLegacy(); err != nil || moved {
		t.Fatalf("%v, %v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(root, "settings.json")); err != nil {
		t.Fatal(err)
	}
}

func seedLegacy(t *testing.T, base string, files ...string) string {
	t.Helper()
	legacy := filepath.Join(base, "rimforge")
	for _, f := range files {
		p := filepath.Join(legacy, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return legacy
}

func TestMigrateLegacyAdoptsRimForgeDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv(EnvDataDir, filepath.Join(base, "muster"))
	legacy := seedLegacy(t, base, "registry.json", "settings.json", "profiles/vanilla/Config/ModsConfig.xml", "cache/communityRules.json")

	moved, err := MigrateLegacy()
	if err != nil || !moved {
		t.Fatalf("MigrateLegacy: %v, %v", moved, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy dir should be gone: %v", err)
	}
	for _, f := range []string{"registry.json", "settings.json", "profiles/vanilla/Config/ModsConfig.xml", "cache/communityRules.json"} {
		if _, err := os.Stat(filepath.Join(GameRoot(RimWorld), f)); err != nil {
			t.Fatalf("%s not migrated: %v", f, err)
		}
	}
	// Idempotent: a second run is a no-op.
	if moved, err := MigrateLegacy(); err != nil || moved {
		t.Fatalf("second run: %v, %v", moved, err)
	}
}

func TestMigrateLegacyLeavesExistingRootAlone(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "muster")
	t.Setenv(EnvDataDir, root)
	legacy := seedLegacy(t, base, "registry.json")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if moved, err := MigrateLegacy(); err != nil || moved {
		t.Fatalf("should not touch anything: %v, %v", moved, err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy dir should still exist: %v", err)
	}
}

func TestMigrateLegacyIgnoresUnrecognisedDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv(EnvDataDir, filepath.Join(base, "muster"))
	legacy := seedLegacy(t, base, "unrelated.txt")
	if moved, err := MigrateLegacy(); err != nil || moved {
		t.Fatalf("should not adopt: %v, %v", moved, err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(DataRoot()); !os.IsNotExist(err) {
		t.Fatalf("root should not have been created: %v", err)
	}
}

func TestMigrateLegacyNoopWhenNothingExists(t *testing.T) {
	t.Setenv(EnvDataDir, filepath.Join(t.TempDir(), "muster"))
	if moved, err := MigrateLegacy(); err != nil || moved {
		t.Fatalf("%v, %v", moved, err)
	}
}
