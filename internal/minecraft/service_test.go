package minecraft

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"context"
	"errors"

	"muster/internal/minecraft/launcher"
	"muster/internal/minecraft/loader"
	"muster/internal/minecraft/manifest"
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
	files["/m.json"] = []byte(fmt.Sprintf(`{"packs":[{"id":"test","name":"Test Pack","pack":%q,"recommended":{"minMemoryMb":1024,"maxMemoryMb":4096,"args":["-XX:+UseZGC"]},"server":"play.test"}]}`, srv.URL+"/pack/pack.toml"))
	return srv, files
}

func TestServiceRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MUSTER_DATA_DIR", root)
	t.Setenv("RIMFORGE_DATA_DIR", "")
	srv, files := fakeServer(t)
	mcDir := filepath.Join(root, "dot-minecraft")
	// A fake NeoForge installer: the jar is served by the fake server, the
	// run creates the version like the real one does.
	oldJar := loader.NeoForgeJar
	loader.NeoForgeJar = srv.URL + "/installer/%s.jar"
	t.Cleanup(func() { loader.NeoForgeJar = oldJar })
	files["/installer/21.1.248.jar"] = []byte("jar")
	var installerRuns int
	fakeRun := func(ctx context.Context, javaPath, jar string, args []string, cwd, logPath string) error {
		installerRuns++
		if javaPath != "/fake/java" {
			return errors.New("wrong java")
		}
		vdir := filepath.Join(mcDir, "versions", "neoforge-21.1.248")
		_ = os.MkdirAll(vdir, 0o755)
		return os.WriteFile(filepath.Join(vdir, "neoforge-21.1.248.json"), []byte(`{}`), 0o644)
	}
	var events []models.SyncProgress
	svc := &Service{
		Emit: func(name string, data any) {
			if name == SyncEvent {
				events = append(events, data.(models.SyncProgress))
			}
		},
		Installer:     &loader.Installer{Run: fakeRun},
		findJava:      func(context.Context, string, func(string)) (string, error) { return "/fake/java", nil },
		TotalMemoryMb: func() int { return 16384 }, // ⇒ MaxHeapMb 12288
	}

	// Nothing configured yet.
	if _, err := svc.ListPacks(); err != ErrNoPacks {
		t.Fatalf("expected ErrNoPacks, got %v", err)
	}
	m := srv.URL + "/m.json"
	if _, err := svc.UpdateSettings(models.Settings{ManifestURL: &m, MinecraftDirOverride: &mcDir}); err != nil {
		t.Fatal(err)
	}
	det, _ := svc.Detect()
	if det.ManifestURL == nil || *det.ManifestURL != m || det.LauncherInstalled || det.PacksDir != filepath.Join(root, "minecraft", "packs") || det.MaxHeapMb != 12288 {
		t.Fatalf("%+v", det)
	}

	packs, err := svc.ListPacks()
	if err != nil || len(packs) != 1 || packs[0].Installed || packs[0].ProfileWritten || packs[0].RecommendedArgs == nil {
		t.Fatalf("%+v %v", packs, err)
	}
	// Nothing saved: the recommendation fitted to the machine, min unset.
	if l := packs[0].Launch; l.MaxMemoryMb != 4096 || l.MinMemoryMb != nil || !l.FollowRecommendedArgs || len(l.Args) != 1 || packs[0].LaunchCustomised {
		t.Fatalf("default launch: %+v", packs[0])
	}

	chk, err := svc.CheckPack("test")
	if err != nil || chk.LatestVersion != "2.0" || chk.VersionID != "neoforge-21.1.248" || chk.ToDownload != 1 || chk.UpToDate || chk.LoaderInstalled {
		t.Fatalf("%+v %v", chk, err)
	}

	rep, err := svc.SyncPack("test")
	if err != nil || len(rep.Downloaded) != 1 || !rep.ProfileWritten || !rep.LoaderInstalled || rep.VersionID != "neoforge-21.1.248" || rep.Manual == nil || rep.Deleted == nil {
		t.Fatalf("%+v %v", rep, err)
	}
	if installerRuns != 1 || !launcher.HasVersion(mcDir, "neoforge-21.1.248") {
		t.Fatalf("installer runs %d", installerRuns)
	}
	var phases []string
	for _, e := range events {
		phases = append(phases, e.Phase)
	}
	if events[0].Phase != "files" || events[0].Current != "Alpha" || events[0].Total != 1 || phases[len(phases)-1] != "profile" {
		t.Fatalf("events %+v", events)
	}
	if n := len(phases); n < 4 || phases[1] != "loader" {
		t.Fatalf("expected loader steps between files and profile: %v", phases)
	}
	if _, err := os.Stat(filepath.Join(root, "minecraft", "packs", "test", "mods", "alpha.jar")); err != nil {
		t.Fatal(err)
	}
	prof, ok, err := launcher.Get(mcDir, "test")
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	if prof.Name != "Test Pack" || prof.LastVersionID != "neoforge-21.1.248" || prof.GameDir != PackDir("test") || prof.JavaArgs != "-Xmx4096M -XX:+UseZGC" {
		t.Fatalf("%+v", prof)
	}

	// The user turns the heap up and pins their own args: the profile is
	// rewritten at once, no sync needed.
	set, err := svc.SetLaunchSettings("test", models.LaunchSettings{MaxMemoryMb: 6144, Args: []string{"-XX:+UseG1GC"}})
	if err != nil || set.MaxMemoryMb != 6144 || set.FollowRecommendedArgs || set.Args[0] != "-XX:+UseG1GC" {
		t.Fatalf("%+v %v", set, err)
	}
	if prof, _, _ = launcher.Get(mcDir, "test"); prof.JavaArgs != "-Xmx6144M -XX:+UseG1GC" || prof.LastVersionID != "neoforge-21.1.248" {
		t.Fatalf("profile after SetLaunchSettings: %+v", prof)
	}
	packs, _ = svc.ListPacks()
	if !packs[0].LaunchCustomised || packs[0].Launch.MaxMemoryMb != 6144 {
		t.Fatalf("%+v", packs[0])
	}
	// Beyond the machine: clamped to MaxHeapMb. Whitespace in an arg: refused.
	if set, _ = svc.SetLaunchSettings("test", models.LaunchSettings{MaxMemoryMb: 99999, FollowRecommendedArgs: true}); set.MaxMemoryMb != 12288 || set.Args[0] != "-XX:+UseZGC" {
		t.Fatalf("%+v", set)
	}
	if _, err := svc.SetLaunchSettings("test", models.LaunchSettings{MaxMemoryMb: 4096, Args: []string{"-Dx=a b"}}); err == nil {
		t.Fatal("whitespace arg should be refused")
	}
	// Reset goes back to the fitted recommendation and rewrites the profile.
	if back, err := svc.ResetLaunchSettings("test"); err != nil || back.MaxMemoryMb != 4096 || !back.FollowRecommendedArgs {
		t.Fatalf("%+v %v", back, err)
	}
	if prof, _, _ = launcher.Get(mcDir, "test"); prof.JavaArgs != "-Xmx4096M -XX:+UseZGC" {
		t.Fatalf("profile after reset: %+v", prof)
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
	if !chk.UpToDate || chk.ToDownload != 0 || !chk.LoaderInstalled {
		t.Fatalf("%+v", chk)
	}

	// A second sync is idle: no downloads, installer not run again.
	events = nil
	if rep, err := svc.SyncPack("test"); err != nil || len(rep.Downloaded) != 0 || installerRuns != 1 || !rep.ProfileWritten {
		t.Fatalf("%+v %v runs=%d", rep, err, installerRuns)
	}

	if _, err := svc.CheckPack("nope"); err == nil {
		t.Fatal("unknown pack should error")
	}
}

func TestJavaArgs(t *testing.T) {
	min := 2048
	if got := javaArgs(models.LaunchSettings{MaxMemoryMb: 8192}); got != "-Xmx8192M" {
		t.Fatalf("%q", got)
	}
	if got := javaArgs(models.LaunchSettings{MaxMemoryMb: 8192, MinMemoryMb: &min, Args: []string{"-XX:+UseZGC", "-XX:+AlwaysPreTouch"}}); got != "-Xms2048M -Xmx8192M -XX:+UseZGC -XX:+AlwaysPreTouch" {
		t.Fatalf("%q", got)
	}
}

func TestEffectiveLaunch(t *testing.T) {
	rec := manifest.Recommended{MinMemoryMb: 4096, MaxMemoryMb: 8192, Args: []string{"-XX:+UseZGC"}}
	// Small machine: recommendation clamped to what it can give, min left unset.
	l := effectiveLaunch(rec, nil, 6144)
	if l.MaxMemoryMb != 6144 || l.MinMemoryMb != nil || !l.FollowRecommendedArgs || len(l.Args) != 1 {
		t.Fatalf("%+v", l)
	}
	// Unknown machine memory: no upper clamp.
	if l = effectiveLaunch(rec, nil, 0); l.MaxMemoryMb != 8192 {
		t.Fatalf("%+v", l)
	}
	// No recommendation at all: a sensible default.
	if l = effectiveLaunch(manifest.Recommended{}, nil, 0); l.MaxMemoryMb != defaultHeapMb || len(l.Args) != 0 {
		t.Fatalf("%+v", l)
	}
	// Saved, following args: args come from the (possibly newer) recommendation.
	saved := models.LaunchSettings{MaxMemoryMb: 5000, FollowRecommendedArgs: true, Args: []string{"stale"}}
	if l = effectiveLaunch(rec, &saved, 0); l.MaxMemoryMb != 4608 || l.Args[0] != "-XX:+UseZGC" {
		t.Fatalf("%+v", l)
	}
	// Saved, pinned args, min above max gets pulled down.
	min := 9000
	saved = models.LaunchSettings{MaxMemoryMb: 8192, MinMemoryMb: &min, Args: []string{"-Xss1M"}}
	if l = effectiveLaunch(rec, &saved, 0); *l.MinMemoryMb != 8192 || l.Args[0] != "-Xss1M" {
		t.Fatalf("%+v", l)
	}
}

func TestPackCodes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MUSTER_DATA_DIR", root)
	t.Setenv("RIMFORGE_DATA_DIR", "")
	packSrv, _ := fakeServer(t)
	var regDown bool
	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if regDown {
			w.WriteHeader(503)
			return
		}
		switch r.URL.Path {
		case "/v1/packs/plum-weasel-23":
			fmt.Fprintf(w, `{"code":"plum-weasel-23","pack":{"id":"frontier","name":"Frontier","pack":%q,"recommended":{"maxMemoryMb":6144,"args":["-XX:+UseZGC"]},"server":"play.example.com"}}`, packSrv.URL+"/pack/pack.toml")
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no pack registered with that code"}}`))
		}
	}))
	defer reg.Close()
	mcDir := filepath.Join(root, "dot-minecraft")
	svc := &Service{TotalMemoryMb: func() int { return 16384 }}
	regURL := reg.URL
	if _, err := svc.UpdateSettings(models.Settings{RegistryURLOverride: &regURL, MinecraftDirOverride: &mcDir}); err != nil {
		t.Fatal(err)
	}
	if det, _ := svc.Detect(); det.RegistryURL != reg.URL {
		t.Fatalf("%+v", det)
	}

	if _, err := svc.AddPackCode("nobody-home-1"); err == nil {
		t.Fatal("unknown code should fail")
	}
	if _, err := svc.AddPackCode("not a code!"); err == nil {
		t.Fatal("garbage should fail before any network")
	}
	// A pasted deep link works, and the recommendation comes through to launch defaults.
	p, err := svc.AddPackCode("muster://add/Plum-Weasel-23")
	if err != nil || p.ID != "frontier" || p.Source != "code" || p.Code == nil || *p.Code != "plum-weasel-23" || p.Launch.MaxMemoryMb != 6144 {
		t.Fatalf("%+v %v", p, err)
	}
	packs, err := svc.ListPacks()
	if err != nil || len(packs) != 1 || packs[0].Source != "code" {
		t.Fatalf("%+v %v", packs, err)
	}
	// Entering it again refreshes rather than duplicates.
	if _, err := svc.AddPackCode("plum-weasel-23"); err != nil {
		t.Fatal(err)
	}
	if st := loadSettings(); len(st.Codes) != 1 {
		t.Fatalf("codes: %+v", st.Codes)
	}
	// Registry down: the cached registration keeps the pack listed and checkable.
	regDown = true
	packs, err = svc.ListPacks()
	if err != nil || len(packs) != 1 || packs[0].Name != "Frontier" {
		t.Fatalf("offline: %+v %v", packs, err)
	}
	if chk, err := svc.CheckPack("frontier"); err != nil || chk.LatestVersion != "2.0" {
		t.Fatalf("offline check: %+v %v", chk, err)
	}
	regDown = false

	// A manifest pack with the same id as a code's is hidden; a different one shows.
	m := packSrv.URL + "/m.json"
	st := loadSettings()
	st.ManifestURL = &m
	if _, err := svc.UpdateSettings(st); err != nil {
		t.Fatal(err)
	}
	packs, _ = svc.ListPacks()
	if len(packs) != 2 || packs[0].Source != "code" || packs[1].Source != "manifest" || packs[1].ID != "test" {
		t.Fatalf("%+v", packs)
	}

	if err := svc.RemovePackCode("plum-weasel-23"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemovePackCode("plum-weasel-23"); err == nil {
		t.Fatal("second removal should fail")
	}
	packs, _ = svc.ListPacks()
	if len(packs) != 1 || packs[0].Source != "manifest" {
		t.Fatalf("%+v", packs)
	}
}
