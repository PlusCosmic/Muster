package minecraft

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"muster/internal/minecraft/launcher"
	"muster/internal/minecraft/models"
	"muster/internal/minecraft/packwiz"
)

// A manifest with one pack, served with the pack itself.
func fakeServer(t *testing.T) (*httptest.Server, map[string][]byte) {
	t.Helper()
	files := map[string][]byte{}
	jar := []byte("JAR")
	jh, _ := packwiz.HashBytes("sha512", jar)
	files["/dl/alpha.jar"] = jar
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b, ok := files[r.URL.Path]; ok {
			_, _ = w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	meta := []byte(fmt.Sprintf("name = \"Alpha\"\nfilename = \"alpha.jar\"\nside = \"both\"\n\n[download]\nurl = %q\nhash-format = \"sha512\"\nhash = %q\n", srv.URL+"/dl/alpha.jar", jh))
	files["/pack/mods/alpha.pw.toml"] = meta
	mh, _ := packwiz.HashBytes("sha256", meta)
	index := []byte(fmt.Sprintf("hash-format = \"sha256\"\n\n[[files]]\nfile = \"mods/alpha.pw.toml\"\nhash = %q\nmetafile = true\n", mh))
	files["/pack/index.toml"] = index
	ih, _ := packwiz.HashBytes("sha256", index)
	files["/pack/pack.toml"] = []byte(fmt.Sprintf("name = \"Test\"\nversion = \"2.0\"\n\n[index]\nfile = \"index.toml\"\nhash-format = \"sha256\"\nhash = %q\n\n[versions]\nminecraft = \"1.21.1\"\nneoforge = \"21.1.248\"\n", ih))
	files["/m.json"] = []byte(fmt.Sprintf(`{"packs":[{"id":"test","name":"Test Pack","pack":%q,"java":{"minMemoryMb":1024,"maxMemoryMb":4096,"args":["-XX:+UseZGC"]},"server":"play.test"}]}`, srv.URL+"/pack/pack.toml"))
	return srv, files
}

func TestServiceRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MUSTER_DATA_DIR", root)
	t.Setenv("RIMFORGE_DATA_DIR", "")
	srv, _ := fakeServer(t)
	mcDir := filepath.Join(root, "dot-minecraft")
	var events []models.SyncProgress
	svc := &Service{Emit: func(name string, data any) {
		if name == SyncEvent {
			events = append(events, data.(models.SyncProgress))
		}
	}}

	// No manifest configured yet.
	if _, err := svc.ListPacks(); err != ErrNoManifest {
		t.Fatalf("expected ErrNoManifest, got %v", err)
	}
	m := srv.URL + "/m.json"
	if _, err := svc.UpdateSettings(models.Settings{ManifestURLOverride: &m, MinecraftDirOverride: &mcDir}); err != nil {
		t.Fatal(err)
	}
	det, _ := svc.Detect()
	if det.ManifestURL == nil || *det.ManifestURL != m || det.LauncherInstalled || det.PacksDir != filepath.Join(root, "minecraft", "packs") {
		t.Fatalf("%+v", det)
	}

	packs, err := svc.ListPacks()
	if err != nil || len(packs) != 1 || packs[0].Installed || packs[0].ProfileWritten || packs[0].JavaArgs == nil {
		t.Fatalf("%+v %v", packs, err)
	}

	chk, err := svc.CheckPack("test")
	if err != nil || chk.LatestVersion != "2.0" || chk.VersionID != "neoforge-21.1.248" || chk.ToDownload != 1 || chk.UpToDate || chk.LoaderInstalled {
		t.Fatalf("%+v %v", chk, err)
	}

	rep, err := svc.SyncPack("test")
	if err != nil || len(rep.Downloaded) != 1 || !rep.ProfileWritten || rep.LoaderInstalled || rep.Manual == nil || rep.Deleted == nil {
		t.Fatalf("%+v %v", rep, err)
	}
	if len(events) != 1 || events[0].Current != "Alpha" || events[0].Total != 1 {
		t.Fatalf("events %+v", events)
	}
	if _, err := os.Stat(filepath.Join(root, "minecraft", "packs", "test", "mods", "alpha.jar")); err != nil {
		t.Fatal(err)
	}
	prof, ok, err := launcher.Get(mcDir, "test")
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	if prof.Name != "Test Pack" || prof.LastVersionID != "neoforge-21.1.248" || prof.GameDir != PackDir("test") || prof.JavaArgs != "-Xms1024M -Xmx4096M -XX:+UseZGC" {
		t.Fatalf("%+v", prof)
	}
	raw, _ := os.ReadFile(filepath.Join(mcDir, launcher.ProfilesFile))
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil || string(doc["version"]) != "3" {
		t.Fatalf("profiles file: %s", raw)
	}

	packs, _ = svc.ListPacks()
	if !packs[0].Installed || *packs[0].InstalledVersion != "2.0" || !packs[0].ProfileWritten || packs[0].SyncedAtMs == nil {
		t.Fatalf("%+v", packs[0])
	}
	chk, _ = svc.CheckPack("test")
	if !chk.UpToDate || chk.ToDownload != 0 {
		t.Fatalf("%+v", chk)
	}

	// The loader installation appearing flips the flag.
	vdir := filepath.Join(mcDir, "versions", "neoforge-21.1.248")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vdir, "neoforge-21.1.248.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if chk, _ = svc.CheckPack("test"); !chk.LoaderInstalled {
		t.Fatalf("%+v", chk)
	}

	if _, err := svc.CheckPack("nope"); err == nil {
		t.Fatal("unknown pack should error")
	}
}

func TestJavaArgs(t *testing.T) {
	if got := javaArgs(manifestJava(0, 0, nil)); got != "" {
		t.Fatalf("%q", got)
	}
	if got := javaArgs(manifestJava(0, 8192, []string{"-XX:+UseZGC", "-XX:+AlwaysPreTouch"})); got != "-Xmx8192M -XX:+UseZGC -XX:+AlwaysPreTouch" {
		t.Fatalf("%q", got)
	}
}
