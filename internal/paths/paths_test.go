package paths

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"rimforge/internal/appdir"
)

const vdf = `
"libraryfolders"
{
	"0"
	{
		"path"		"/home/cosmic/.local/share/Steam"
		"label"		""
		"contentid"		"6781820241171433978"
		"apps"
		{
			"228980"		"918280233"
		}
	}
	"1"
	{
		"path"		"/mnt/big-disk/SteamLibrary"
		"label"		""
		"apps"
		{
			"294100"		"859843592"
		}
	}
}
`

const vdfWindows = `
"libraryfolders"
{
	"0"
	{
		"path"		"C:\\Program Files (x86)\\Steam"
	}
	"1"
	{
		"path"		"D:\\Games\\SteamLibrary"
	}
}
`

func TestParsesUnixLibraryPaths(t *testing.T) {
	got := ParseLibraryPaths(vdf)
	want := []string{"/home/cosmic/.local/share/Steam", "/mnt/big-disk/SteamLibrary"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestUnescapesWindowsBackslashes(t *testing.T) {
	got := ParseLibraryPaths(vdfWindows)
	want := []string{`C:\Program Files (x86)\Steam`, `D:\Games\SteamLibrary`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestIgnoresNonPathKeysAndDedups(t *testing.T) {
	body := `
			"path"		"/a"
			"mounted"	"1"
			"path"		"/a"
			"contentpath"	"/should/not/match"
		`
	// "contentpath" must not match because the regex anchors on the exact
	// quoted key `"path"`.
	if got := ParseLibraryPaths(body); !reflect.DeepEqual(got, []string{"/a"}) {
		t.Fatalf("got %v", got)
	}
}

func TestEmptyOrGarbageVdfYieldsNothing(t *testing.T) {
	if got := ParseLibraryPaths(""); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if got := ParseLibraryPaths("not a vdf at all"); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestParsesFullVersionString(t *testing.T) {
	cases := map[string]string{
		"1.6.4535 rev991\n":         "1.6.4535 rev991",
		"\ufeff1.5.4104 rev435\r\n": "1.5.4104 rev435",
		"\n\n1.4.3901 rev1\n":       "1.4.3901 rev1",
	}
	for in, want := range cases {
		got, ok := ParseVersion(in)
		if !ok || got != want {
			t.Errorf("ParseVersion(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "   \n  \n"} {
		if _, ok := ParseVersion(in); ok {
			t.Errorf("ParseVersion(%q) should fail", in)
		}
	}
}

func TestDerivesMajorMinor(t *testing.T) {
	if v, ok := MajorMinor("1.6.4535 rev991"); !ok || v != "1.6" {
		t.Fatalf("got %q %v", v, ok)
	}
	if v, ok := MajorMinor("1.6"); !ok || v != "1.6" {
		t.Fatalf("got %q %v", v, ok)
	}
	if _, ok := MajorMinor("bogus"); ok {
		t.Fatal("bogus should fail")
	}
	if _, ok := MajorMinor(""); ok {
		t.Fatal("empty should fail")
	}
}

func TestDataRootEndsInRimforge(t *testing.T) {
	t.Setenv("RIMFORGE_DATA_DIR", "")
	if filepath.Base(appdir.DataRoot()) != "rimforge" {
		t.Fatalf("got %s", appdir.DataRoot())
	}
	if filepath.Base(appdir.ProfilesRoot()) != "profiles" {
		t.Fatalf("got %s", appdir.ProfilesRoot())
	}
	if !strings.HasPrefix(appdir.ProfilesRoot(), appdir.DataRoot()) {
		t.Fatalf("%s should be under %s", appdir.ProfilesRoot(), appdir.DataRoot())
	}
}

func TestDataRootOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RIMFORGE_DATA_DIR", dir)
	if appdir.DataRoot() != dir {
		t.Fatalf("got %s want %s", appdir.DataRoot(), dir)
	}
}

// Smoke test: prints what detection finds on the machine running the tests.
// Never launches anything. Run with `go test -v ./internal/paths`.
func TestSmokeDetectOnThisMachine(t *testing.T) {
	p := Detect()
	t.Logf("steamRoot:       %v", ptr(p.SteamRoot))
	t.Logf("gameInstall:     %v", ptr(p.GameInstall))
	t.Logf("gameVersion:     %v", ptr(p.GameVersion))
	t.Logf("defaultSavedata: %v", ptr(p.DefaultSavedata))
	t.Logf("workshopDirs:    %v", p.WorkshopDirs)
	t.Logf("profilesDir:     %s", p.ProfilesDir)
	if p.ProfilesDir == "" {
		t.Fatal("profilesDir must never be empty")
	}
	if p.WorkshopDirs == nil {
		t.Fatal("workshopDirs must serialise as [], never null")
	}
}

func ptr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}
