package profiles

import (
	"encoding/json"
	"muster/internal/rimworld/paths"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"muster/internal/appdir"
	"muster/internal/rimworld/models"
	"muster/internal/rimworld/settings"
)

func taken(items ...string) map[string]bool {
	m := map[string]bool{}
	for _, s := range items {
		m[s] = true
	}
	return m
}

func TestSlugifyBasics(t *testing.T) {
	cases := map[string]string{
		"Vanilla":                  "vanilla",
		"Medieval Overhaul":        "medieval-overhaul",
		"  Trimmed  ":              "trimmed",
		"RimWorld 1.6!":            "rimworld-1-6",
		"multi   spaces":           "multi-spaces",
		"--leading-and-trailing--": "leading-and-trailing",
		"Mix_of/Chars\\Here":       "mix-of-chars-here",
		"Café Run":                 "caf-run",
		"✨✨✨":                      "profile",
		"":                         "profile",
		"   ":                      "profile",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUniqueSlugHandlesCollisions(t *testing.T) {
	if got := UniqueSlug("Vanilla", taken()); got != "vanilla" {
		t.Fatal(got)
	}
	if got := UniqueSlug("Vanilla", taken("vanilla")); got != "vanilla-2" {
		t.Fatal(got)
	}
	if got := UniqueSlug("vanilla", taken("vanilla", "vanilla-2")); got != "vanilla-3" {
		t.Fatal(got)
	}
	// Gaps are filled: -2 is free even though -3 is taken.
	if got := UniqueSlug("Vanilla!", taken("vanilla", "vanilla-3")); got != "vanilla-2" {
		t.Fatal(got)
	}
	// Different display names that slug to the same base still collide.
	if got := UniqueSlug("Medieval  Overhaul", taken("medieval-overhaul")); got != "medieval-overhaul-2" {
		t.Fatal(got)
	}
	// Empty-ish names collide on the fallback slug too.
	if got := UniqueSlug("!!!", taken("profile", "profile-2")); got != "profile-3" {
		t.Fatal(got)
	}
}

func TestUniqueSlugIsStableOverManyCollisions(t *testing.T) {
	existing := taken()
	var produced []string
	for i := 0; i < 5; i++ {
		s := UniqueSlug("Test Run", existing)
		existing[s] = true
		produced = append(produced, s)
	}
	want := []string{"test-run", "test-run-2", "test-run-3", "test-run-4", "test-run-5"}
	if !reflect.DeepEqual(produced, want) {
		t.Fatalf("got %v", produced)
	}
}

func TestCountsActiveModsFromXML(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<ModsConfigData>
  <version>1.6.4535 rev991</version>
  <activeMods>
    <li>ludeon.rimworld</li>
    <li>ludeon.rimworld.royalty</li>
    <li>brrainz.harmony</li>
  </activeMods>
  <knownExpansions><li>ludeon.rimworld.royalty</li></knownExpansions>
</ModsConfigData>`
	if got := CountActiveModsInXML(xml); got != 3 {
		t.Fatalf("got %d", got)
	}
}

func TestMalformedOrEmptyXMLCountsZero(t *testing.T) {
	for _, body := range []string{"<ModsConfigData>", "<ModsConfigData><activeMods /></ModsConfigData>", ""} {
		if got := CountActiveModsInXML(body); got != 0 {
			t.Errorf("%q: got %d", body, got)
		}
	}
}

func TestMinimalModsConfigShape(t *testing.T) {
	with := MinimalModsConfig("1.6.4535 rev991")
	if !strings.Contains(with, "<version>1.6.4535 rev991</version>") || !strings.Contains(with, "<li>ludeon.rimworld</li>") {
		t.Fatalf("got:\n%s", with)
	}
	if CountActiveModsInXML(with) != 1 {
		t.Fatal("expected one active mod")
	}
	without := MinimalModsConfig("")
	if strings.Contains(without, "<version>") || CountActiveModsInXML(without) != 1 {
		t.Fatalf("got:\n%s", without)
	}
}

func TestProfileDirIsUnderProfilesRoot(t *testing.T) {
	dir := paths.ProfileDir("vanilla")
	if !strings.HasSuffix(dir, filepath.Join("profiles", "vanilla")) {
		t.Fatal(dir)
	}
}

func TestRegistryJSONRoundtrip(t *testing.T) {
	reg := Registry{Profiles: []Meta{{ID: "vanilla", Name: "Vanilla", CreatedAtMs: 1_700_000_000_000}}}
	raw, err := json.Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "createdAtMs") {
		t.Fatalf("got %s", raw)
	}
	var back Registry
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Profiles) != 1 || back.Profiles[0].ID != "vanilla" {
		t.Fatalf("got %+v", back)
	}

	// Tolerates an older/partial record without lastPlayedAtMs.
	var partial Registry
	if err := json.Unmarshal([]byte(`{"profiles":[{"id":"a","name":"A","createdAtMs":1}]}`), &partial); err != nil {
		t.Fatal(err)
	}
	if partial.Profiles[0].LastPlayedAtMs != nil {
		t.Fatal("expected nil lastPlayedAtMs")
	}
	// And an empty file body.
	var empty Registry
	if err := json.Unmarshal([]byte("{}"), &empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Profiles) != 0 {
		t.Fatal("expected no profiles")
	}
}

// scratch points the data root at a temp dir and replaces the trash with a
// plain delete so nothing leaves the sandbox.
func scratch(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("MUSTER_DATA_DIR", root)
	prev := trashFn
	trashFn = os.RemoveAll
	t.Cleanup(func() { trashFn = prev })
	return root
}

func TestCrudRoundtripAgainstScratchDataDir(t *testing.T) {
	root := scratch(t)
	if appdir.DataRoot() != root {
		t.Fatal("data root override not applied")
	}

	a, err := Create("Smoke Test")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "smoke-test" || a.ActiveModCount != 1 || a.SaveCount != 0 {
		t.Fatalf("got %+v", a)
	}
	if _, err := os.Stat(filepath.Join(paths.ProfileDir(a.ID), "Config", "ModsConfig.xml")); err != nil {
		t.Fatal(err)
	}

	b, err := Create("Smoke Test")
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != "smoke-test-2" {
		t.Fatal(b.ID)
	}

	renamed, err := Rename(a.ID, "Renamed")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != "smoke-test" || renamed.Name != "Renamed" {
		t.Fatalf("got %+v", renamed)
	}

	// A save file so the clone has something to deep-copy.
	saves := filepath.Join(paths.ProfileDir(a.ID), "Saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "Colony.rws"), []byte("<savegame/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	cloned, err := Clone(a.ID, "Cloned Run")
	if err != nil {
		t.Fatal(err)
	}
	if cloned.ID != "cloned-run" || cloned.SaveCount != 1 || cloned.ActiveModCount != 1 {
		t.Fatalf("got %+v", cloned)
	}
	if _, err := os.Stat(filepath.Join(paths.ProfileDir(cloned.ID), "Saves", "Colony.rws")); err != nil {
		t.Fatal(err)
	}

	list, err := List()
	if err != nil || len(list) != 3 {
		t.Fatalf("got %d profiles, err %v", len(list), err)
	}
	if p, err := Find("smoke-test"); err != nil || p.Name != "Renamed" {
		t.Fatalf("got %+v, %v", p, err)
	}
	if _, err := Find("nope"); err == nil {
		t.Fatal("unknown id should error")
	}

	if err := TouchLastPlayed(a.ID); err != nil {
		t.Fatal(err)
	}
	if p, _ := Find(a.ID); p.LastPlayedAtMs == nil {
		t.Fatal("lastPlayedAtMs should be set")
	}

	for _, id := range []string{a.ID, b.ID, cloned.ID} {
		if err := Delete(id); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(paths.ProfileDir(id)); !os.IsNotExist(err) {
			t.Fatalf("%s dir should be gone", id)
		}
	}
	if list, _ := List(); len(list) != 0 {
		t.Fatalf("expected no profiles, got %d", len(list))
	}
}

func TestListWithNoRegistryIsEmptyNotNil(t *testing.T) {
	scratch(t)
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("got %#v", list)
	}
}

func TestEmptyNamesAreRejected(t *testing.T) {
	scratch(t)
	for _, name := range []string{"", "   "} {
		if _, err := Create(name); err == nil {
			t.Errorf("Create(%q) should fail", name)
		}
	}
}

func TestImportDefaultCopiesSymlinkedSourceWithoutMutatingIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	root := scratch(t)

	// A "real" config store that the savedata folder only links to.
	store := filepath.Join(root, "store")
	source := filepath.Join(root, "fake-savedata")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(store, "Config"), 0o755))
	must(os.WriteFile(filepath.Join(store, "Config", "ModsConfig.xml"), []byte(MinimalModsConfig("1.6.4871 rev598")), 0o644))
	must(os.MkdirAll(filepath.Join(source, "Saves"), 0o755))
	must(os.MkdirAll(filepath.Join(source, "Scenarios"), 0o755))
	must(os.WriteFile(filepath.Join(source, "Saves", "Colony.rws"), []byte("<savegame/>"), 0o644))
	must(os.Symlink(filepath.Join(store, "Config"), filepath.Join(source, "Config")))

	must(settings.Save(models.Settings{DefaultSavedataOverride: &source}))

	profile, err := ImportDefault("Imported")
	must(err)
	if profile.ID != "imported" || profile.SaveCount != 1 || profile.ActiveModCount != 1 {
		t.Fatalf("got %+v", profile)
	}

	dir := paths.ProfileDir(profile.ID)
	for _, p := range []string{filepath.Join(dir, "Config", "ModsConfig.xml"), filepath.Join(dir, "Saves", "Colony.rws"), filepath.Join(dir, "Scenarios")} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal(err)
		}
	}
	// Config was copied by content, not re-linked.
	if info, _ := os.Lstat(filepath.Join(dir, "Config")); info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("copied Config should not be a symlink")
	}
	// Source is still the symlink it was.
	if info, _ := os.Lstat(filepath.Join(source, "Config")); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("source Config should still be a symlink")
	}
}

func TestCopyTreeFollowsSymlinksAndCopiesContents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	base := t.TempDir()
	src := filepath.Join(base, "src")
	real := filepath.Join(base, "real")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(src, "nested"), 0o755))
	must(os.MkdirAll(real, 0o755))
	must(os.WriteFile(filepath.Join(src, "nested", "a.txt"), []byte("hello"), 0o644))
	must(os.WriteFile(filepath.Join(real, "linked.txt"), []byte("linked contents"), 0o644))
	must(os.Symlink(filepath.Join(real, "linked.txt"), filepath.Join(src, "link.txt")))
	must(os.Symlink(real, filepath.Join(src, "linkdir")))

	dst := filepath.Join(base, "dst")
	must(copyTree(src, dst, 0))

	read := func(p string) string {
		t.Helper()
		b, err := os.ReadFile(p)
		must(err)
		return string(b)
	}
	if read(filepath.Join(dst, "nested", "a.txt")) != "hello" {
		t.Fatal("nested file not copied")
	}
	if read(filepath.Join(dst, "link.txt")) != "linked contents" {
		t.Fatal("symlinked file not copied by content")
	}
	for _, p := range []string{filepath.Join(dst, "link.txt"), filepath.Join(dst, "linkdir")} {
		if info, _ := os.Lstat(p); info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("%s should be a real copy", p)
		}
	}
	if read(filepath.Join(dst, "linkdir", "linked.txt")) != "linked contents" {
		t.Fatal("symlinked dir not copied by content")
	}
	if info, _ := os.Lstat(filepath.Join(src, "link.txt")); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("source symlink should be untouched")
	}
}
