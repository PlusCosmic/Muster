package settings

import (
	"encoding/json"
	"strings"
	"testing"

	core "muster/internal/models"
	"muster/internal/rimworld/models"
)

func str(s string) *string { return &s }

func TestBlankOverridesBecomeNil(t *testing.T) {
	s := Normalize(models.Settings{
		SteamRootOverride:       str("   "),
		GameInstallOverride:     str(""),
		DefaultSavedataOverride: str("  /tmp/x  "),
	})
	if s.SteamRootOverride != nil || s.GameInstallOverride != nil {
		t.Fatalf("blank overrides should be nil: %+v", s)
	}
	if got := core.Deref(s.DefaultSavedataOverride); got != "/tmp/x" {
		t.Fatalf("expected trimmed override, got %q", got)
	}
}

func TestRoundtripJSONIsCamelCase(t *testing.T) {
	s := models.Settings{SteamRootOverride: str("/steam")}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "steamRootOverride") {
		t.Fatalf("got %s", raw)
	}
	var back models.Settings
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if core.Deref(back.SteamRootOverride) != "/steam" {
		t.Fatalf("got %+v", back)
	}
}

func TestEmptyObjectDeserialisesAsDefault(t *testing.T) {
	var s models.Settings
	if err := json.Unmarshal([]byte("{}"), &s); err != nil {
		t.Fatal(err)
	}
	if s.SteamRootOverride != nil || s.GameInstallOverride != nil || s.DefaultSavedataOverride != nil {
		t.Fatalf("expected all nil, got %+v", s)
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	t.Setenv("MUSTER_DATA_DIR", t.TempDir())
	if got := Load(); got.SteamRootOverride != nil {
		t.Fatalf("missing file should load as default, got %+v", got)
	}
	want, err := Update(models.Settings{GameInstallOverride: str(" /games/RimWorld ")})
	if err != nil {
		t.Fatal(err)
	}
	if core.Deref(want.GameInstallOverride) != "/games/RimWorld" {
		t.Fatalf("update should normalise, got %+v", want)
	}
	if got := Load(); core.Deref(got.GameInstallOverride) != "/games/RimWorld" {
		t.Fatalf("reload mismatch: %+v", got)
	}
}
