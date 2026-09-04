package mods

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimforge/internal/models"
)

const sampleConfig = "\uFEFF<?xml version=\"1.0\" encoding=\"utf-8\"?>\n" + `<ModsConfigData>
  <version>1.6.4871 rev600</version>
  <activeMods>
    <li>Ludeon.RimWorld</li>
    <li>brrainz.harmony</li>
    <li>brrainz.harmony</li>
  </activeMods>
  <knownExpansions>
    <li>Ludeon.RimWorld.Biotech</li>
  </knownExpansions>
</ModsConfigData>`

func TestParsesLowercasingAndDeduping(t *testing.T) {
	list, err := ParseModsConfig(sampleConfig)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "version", models.Deref(list.Version), "1.6.4871 rev600")
	eq(t, "activeIds", list.ActiveIDs, []string{"ludeon.rimworld", "brrainz.harmony"})
	eq(t, "knownExpansions", list.KnownExpansions, []string{"ludeon.rimworld.biotech"})
}

func TestRoundTripsThroughRenderAndParse(t *testing.T) {
	original, err := ParseModsConfig(sampleConfig)
	if err != nil {
		t.Fatal(err)
	}
	rendered := RenderModsConfig(original)
	if !strings.HasPrefix(rendered, "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n") {
		t.Fatalf("got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "\n  <activeMods>\n    <li>ludeon.rimworld</li>\n") {
		t.Fatalf("got:\n%s", rendered)
	}
	again, err := ParseModsConfig(rendered)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "activeIds", again.ActiveIDs, original.ActiveIDs)
	eq(t, "knownExpansions", again.KnownExpansions, original.KnownExpansions)
	eq(t, "version", again.Version, original.Version)
}

func TestEscapesTextAndRendersEmptyListsSelfClosing(t *testing.T) {
	list := models.ActiveModList{
		ActiveIDs:       []string{"a&b<c>"},
		KnownExpansions: []string{},
		Version:         models.Str(`1.6 "rev"`),
	}
	xml := RenderModsConfig(list)
	for _, want := range []string{"<li>a&amp;b&lt;c&gt;</li>", "<version>1.6 &quot;rev&quot;</version>", "<knownExpansions />"} {
		if !strings.Contains(xml, want) {
			t.Fatalf("missing %q in:\n%s", want, xml)
		}
	}
	// Still well-formed after escaping.
	back, err := ParseModsConfig(xml)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "activeIds", back.ActiveIDs, []string{"a&b<c>"})
}

func TestOmitsVersionWhenUnknown(t *testing.T) {
	xml := RenderModsConfig(models.ActiveModList{ActiveIDs: []string{"ludeon.rimworld"}})
	if strings.Contains(xml, "<version") {
		t.Fatalf("got:\n%s", xml)
	}
	back, err := ParseModsConfig(xml)
	if err != nil {
		t.Fatal(err)
	}
	if back.Version != nil {
		t.Fatal("version should be nil")
	}
	if back.KnownExpansions == nil {
		t.Fatal("knownExpansions must never be nil")
	}
}

func TestWritesAndReadsBackFromDisk(t *testing.T) {
	path := ModsConfigPath(t.TempDir())
	if got, err := ReadActive(path); err != nil || got != nil {
		t.Fatalf("missing file should read as nil, got %v %v", got, err)
	}
	list := models.ActiveModList{
		ActiveIDs:       []string{"ludeon.rimworld", "brrainz.harmony"},
		KnownExpansions: []string{"ludeon.rimworld.odyssey"},
		Version:         models.Str("1.6.4871 rev600"),
	}
	if err := WriteActive(path, list); err != nil {
		t.Fatal(err)
	}
	back, err := ReadActive(path)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "activeIds", back.ActiveIDs, list.ActiveIDs)
	eq(t, "knownExpansions", back.KnownExpansions, list.KnownExpansions)
	eq(t, "version", back.Version, list.Version)
}

func TestMalformedConfigIsAnError(t *testing.T) {
	if _, err := ParseModsConfig("<ModsConfigData><activeMods>"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSetActivePreservesVersionAndNormalises(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RIMFORGE_DATA_DIR", root)
	// Point detection nowhere so environment() finds no expansions.
	t.Setenv("HOME", filepath.Join(root, "nohome"))
	path := ModsConfigPath(filepath.Join(root, "profiles", "p"))
	if err := WriteActive(path, models.ActiveModList{
		ActiveIDs:       []string{"ludeon.rimworld"},
		KnownExpansions: []string{"ludeon.rimworld.biotech"},
		Version:         models.Str("1.5.0 rev1"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetActive("p", []string{" Ludeon.RimWorld ", "brrainz.harmony", "BRRAINZ.HARMONY", ""}); err != nil {
		t.Fatal(err)
	}
	back, err := ReadActive(path)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "activeIds", back.ActiveIDs, []string{"ludeon.rimworld", "brrainz.harmony"})
	eq(t, "version kept", models.Deref(back.Version), "1.5.0 rev1")
	eq(t, "known kept when detection fails", back.KnownExpansions, []string{"ludeon.rimworld.biotech"})

	got, err := GetActive("missing-profile")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "default active", got.ActiveIDs, []string{CorePackageID})
	if _, err := os.Stat(ModsConfigPath(filepath.Join(root, "profiles", "missing-profile"))); !os.IsNotExist(err) {
		t.Fatal("GetActive must not create files")
	}
}
