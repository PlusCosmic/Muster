package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"muster/internal/models"
)

func TestNormalizeKeepsKnownGamesInRailOrder(t *testing.T) {
	s := Normalize(models.AppSettings{Games: []string{" Minecraft ", "factorio", "rimworld", "minecraft"}})
	if want := []string{"rimworld", "minecraft"}; !reflect.DeepEqual(s.Games, want) {
		t.Fatalf("got %v, want %v", s.Games, want)
	}
}

func TestNormalizeNeverYieldsNil(t *testing.T) {
	s := Normalize(models.AppSettings{})
	if s.Games == nil || len(s.Games) != 0 {
		t.Fatalf("expected an empty list, got %#v", s.Games)
	}
}

func TestMissingFileIsFirstRun(t *testing.T) {
	t.Setenv("MUSTER_DATA_DIR", t.TempDir())
	if got, err := Load(); err != nil || len(got.Games) != 0 {
		t.Fatalf("expected no games, got %v, %v", got.Games, err)
	}
}

func TestUnreadableFileIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything")
	}
	root := t.TempDir()
	t.Setenv("MUSTER_DATA_DIR", root)
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"games":["rimworld"]}`), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a file that exists but cannot be read")
	}
}

func TestMalformedFileIsFirstRun(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MUSTER_DATA_DIR", root)
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(); err != nil || len(got.Games) != 0 {
		t.Fatalf("expected no games, got %v, %v", got.Games, err)
	}
}

func TestUpdateRoundTrips(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MUSTER_DATA_DIR", filepath.Join(root, "fresh"))

	saved, err := Update(models.AppSettings{Games: []string{"minecraft"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"minecraft"}; !reflect.DeepEqual(saved.Games, want) {
		t.Fatalf("Update returned %v, want %v", saved.Games, want)
	}
	raw, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"games"`) {
		t.Fatalf("expected camelCase json, got %s", raw)
	}
	if got, _ := Get(); !reflect.DeepEqual(got, saved) {
		t.Fatalf("Get: %+v, want %+v", got, saved)
	}
}
