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
	if DataRoot() != dir {
		t.Fatalf("got %s", DataRoot())
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
