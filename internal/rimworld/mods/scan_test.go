package mods

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	core "muster/internal/models"
	"muster/internal/rimworld/models"
)

func writeMod(t *testing.T, root, name, xml string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "About"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "About", "About.xml"), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func aboutXML(pid, name string) string {
	return fmt.Sprintf("<ModMetaData><packageId>%s</packageId><name>%s</name></ModMetaData>", pid, name)
}

func TestFindsAboutXMLCaseInsensitively(t *testing.T) {
	// Medieval Go-juice ships About/About.XML; RimWorld accepts any casing.
	dir := filepath.Join(t.TempDir(), "GoJuice")
	if err := os.MkdirAll(filepath.Join(dir, "about"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "about", "About.XML"), []byte(aboutXML("Rince.gaulishgojuice", "Medieval Go-juice")), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := ReadModDir(dir, models.SourceLocal)
	if err != nil || info == nil {
		t.Fatalf("got %v, %v", info, err)
	}
	if info.PackageID != "rince.gaulishgojuice" {
		t.Fatal(info.PackageID)
	}
}

func TestOfficialContentWithoutNameGetsDisplayName(t *testing.T) {
	root := t.TempDir()
	// Real Data/*/About.xml files ship no <name> element at all.
	dir := writeMod(t, root, "Royalty", "<ModMetaData><packageId>Ludeon.RimWorld.Royalty</packageId></ModMetaData>")
	info, err := ReadModDir(dir, models.SourceOfficial)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Royalty" {
		t.Fatal(info.Name)
	}
	// Unofficial mods without a <name> still fall back to the packageId.
	dir = writeMod(t, root, "NoName", "<ModMetaData><packageId>A.B</packageId></ModMetaData>")
	info, err = ReadModDir(dir, models.SourceLocal)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "a.b" {
		t.Fatal(info.Name)
	}
	if info.SupportedVersions == nil || info.Dependencies == nil {
		t.Fatal("list fields must never be nil")
	}
}

func TestScansAllThreeSourcesWithPrecedenceAndWorkshopIDs(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "RimWorld")
	writeMod(t, filepath.Join(install, "Data"), "Core", aboutXML("Ludeon.RimWorld", "Core"))
	writeMod(t, filepath.Join(install, "Data"), "Biotech", aboutXML("Ludeon.RimWorld.Biotech", "Biotech"))
	writeMod(t, filepath.Join(install, "Mods"), "MyMod", aboutXML("Me.Mine", "Mine"))
	// Same id as the local mod: local wins, workshop copy is dropped.
	ws := filepath.Join(root, "294100")
	writeMod(t, ws, "123456", aboutXML("Me.Mine", "Mine (Workshop)"))
	writeMod(t, ws, "987654", aboutXML("Other.Mod", "Other"))
	// Not a mod: no About.xml. Must be skipped silently.
	if err := os.MkdirAll(filepath.Join(ws, "nonsense"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Malformed: must be skipped, not fatal.
	writeMod(t, ws, "555", "<ModMetaData><name>broken</ModMetaData>")

	mods := ScanAll(install, []string{ws})
	var ids []string
	for _, m := range mods {
		ids = append(ids, m.PackageID)
	}
	want := []string{"ludeon.rimworld.biotech", "ludeon.rimworld", "me.mine", "other.mod"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("got %v", ids)
	}

	byID := map[string]models.ModInfo{}
	for _, m := range mods {
		byID[m.PackageID] = m
	}
	mine := byID["me.mine"]
	if mine.Source != models.SourceLocal || mine.Name != "Mine" || mine.SteamWorkshopID != nil {
		t.Fatalf("got %+v", mine)
	}
	other := byID["other.mod"]
	if other.Source != models.SourceWorkshop || core.Deref(other.SteamWorkshopID) != "987654" {
		t.Fatalf("got %+v", other)
	}
	if got := InstalledExpansions(mods); !reflect.DeepEqual(got, []string{"ludeon.rimworld.biotech"}) {
		t.Fatalf("got %v", got)
	}
}

func TestMissingDirectoriesAreNotFatal(t *testing.T) {
	if mods := ScanAll("/definitely/not/here", nil); len(mods) != 0 {
		t.Fatalf("got %v", mods)
	}
}

func TestInstalledExpansionsIsNeverNil(t *testing.T) {
	if got := InstalledExpansions(nil); got == nil {
		t.Fatal("expected an empty slice")
	}
}

// Real-machine scan. Run with:
// RIMFORGE_TEST_GAME_INSTALL=... RIMFORGE_TEST_WORKSHOP=a:b go test -v ./internal/mods -run RealInstall
func TestRealInstallScan(t *testing.T) {
	install := os.Getenv("RIMFORGE_TEST_GAME_INSTALL")
	if install == "" {
		t.Skip("set RIMFORGE_TEST_GAME_INSTALL to scan a real install")
	}
	workshop := filepath.SplitList(os.Getenv("RIMFORGE_TEST_WORKSHOP"))
	mods := ScanAll(install, workshop)
	t.Logf("scanned %d mods; expansions %v", len(mods), InstalledExpansions(mods))
	if len(mods) == 0 {
		t.Fatal("expected to find mods")
	}
}
