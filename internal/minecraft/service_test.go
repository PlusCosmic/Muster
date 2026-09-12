package minecraft

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"muster/internal/minecraft/launcher"
	"muster/internal/minecraft/loader"
	"muster/internal/minecraft/manifest"
	"muster/internal/minecraft/models"
	"muster/internal/minecraft/packwiz"
	"muster/internal/retry"
)

// fastClient is the service's real retrying client with the sleeps taken
// out, so a test that serves 503s or closes its server does not wait.
func fastClient() *http.Client {
	return &http.Client{Transport: &retry.Transport{Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}}
}

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
	svc := &Service{HTTP: fastClient(), TotalMemoryMb: func() int { return 16384 }}
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

// A fake Modrinth: one modpack project with two versions, the newest a beta,
// and the .mrpack for each served with the pack's own overrides.
func fakeModrinth(t *testing.T, srvURL func() string) (http.Handler, map[string][]byte) {
	t.Helper()
	files := map[string][]byte{}
	jar := []byte("JAR")
	jh, _ := packwiz.HashBytes("sha512", jar)
	files["/cdn/alpha.jar"] = jar
	mrpack := func(version string, overrides map[string]string) []byte {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		idx := map[string]any{
			"formatVersion": 1, "game": "minecraft", "versionId": version, "name": "Fancy Pack",
			"files": []map[string]any{{
				"path": "mods/alpha.jar", "hashes": map[string]string{"sha512": jh},
				"env":       map[string]string{"client": "required", "server": "required"},
				"downloads": []string{"https://cdn.modrinth.com/alpha.jar"}, "fileSize": 3,
			}},
			"dependencies": map[string]string{"minecraft": "1.21.1", "neoforge": "21.1.248"},
		}
		raw, _ := json.Marshal(idx)
		w, _ := zw.Create("modrinth.index.json")
		_, _ = w.Write(raw)
		for name, body := range overrides {
			w, _ := zw.Create(name)
			_, _ = w.Write([]byte(body))
		}
		_ = zw.Close()
		return buf.Bytes()
	}
	files["/dl/1.0.mrpack"] = mrpack("1.0", map[string]string{"overrides/config/a.toml": "one"})
	files["/dl/1.1.mrpack"] = mrpack("1.1", map[string]string{"overrides/config/a.toml": "two", "client-overrides/options.txt": "fov:1"})
	files["/dl/2.0-beta.mrpack"] = mrpack("2.0-beta", nil)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b, ok := files[r.URL.Path]; ok {
			_, _ = w.Write(b)
			return
		}
		version := func(id, number, typ, date, file string) map[string]any {
			sha, _ := packwiz.HashBytes("sha512", files["/dl/"+file])
			return map[string]any{
				"id": id, "version_number": number, "version_type": typ, "date_published": date,
				"files": []map[string]any{{"url": srvURL() + "/dl/" + file, "filename": file, "primary": true, "hashes": map[string]string{"sha512": sha}}},
			}
		}
		switch r.URL.Path {
		case "/v2/project/fancy-pack", "/v2/project/AbCdEf12":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "AbCdEf12", "slug": "fancy-pack", "title": "Fancy Pack", "description": "Very fancy.", "icon_url": "https://cdn.modrinth.com/icon.webp", "project_type": "modpack"})
		case "/v2/project/AbCdEf12/version":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				version("vB", "2.0-beta", "beta", "2026-03-01T00:00:00Z", "2.0-beta.mrpack"),
				version("v11", "1.1", "release", "2026-02-01T00:00:00Z", "1.1.mrpack"),
				version("v10", "1.0", "release", "2026-01-01T00:00:00Z", "1.0.mrpack"),
			})
		case "/v2/project/some-mod":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "m", "slug": "some-mod", "title": "Some Mod", "project_type": "mod"})
		case "/v2/project/a-b-c", "/v2/project/P1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "P1", "slug": "a-b-c", "title": "ABC One", "project_type": "modpack"})
		case "/v2/project/a__b..c", "/v2/project/P2":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "P2", "slug": "a__b..c", "title": "ABC Two", "project_type": "modpack"})
		case "/v2/project/P1/version", "/v2/project/P2/version":
			_ = json.NewEncoder(w).Encode([]map[string]any{version("x1", "1.0", "release", "2026-01-01T00:00:00Z", "1.0.mrpack")})
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"not_found","description":"no"}`))
		}
	})
	return h, files
}

func TestModrinthPacks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MUSTER_DATA_DIR", root)
	t.Setenv("RIMFORGE_DATA_DIR", "")
	var srv *httptest.Server
	h, _ := fakeModrinth(t, func() string { return srv.URL })
	srv = httptest.NewServer(h)
	t.Cleanup(srv.Close)
	mcDir := filepath.Join(root, "dot-minecraft")
	vdir := filepath.Join(mcDir, "versions", "neoforge-21.1.248")
	_ = os.MkdirAll(vdir, 0o755)
	_ = os.WriteFile(filepath.Join(vdir, "neoforge-21.1.248.json"), []byte(`{}`), 0o644)
	svc := &Service{
		ModrinthURL:   srv.URL + "/v2",
		HTTP:          rewriteHost(srv),
		Installer:     &loader.Installer{Run: func(context.Context, string, string, []string, string, string) error { return nil }},
		findJava:      func(context.Context, string, func(string)) (string, error) { return "/fake/java", nil },
		TotalMemoryMb: func() int { return 16384 },
	}
	if _, err := svc.UpdateSettings(models.Settings{MinecraftDirOverride: &mcDir}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.AddModrinthPack("amber-otter-42", ""); err == nil {
		t.Fatal("a pack code is not a Modrinth link")
	}
	if _, err := svc.LookupModrinth("amber-otter-42"); err == nil {
		t.Fatal("a pack code is not a Modrinth link")
	}
	if _, err := svc.AddModrinthPack("https://modrinth.com/modpack/nope", ""); err == nil {
		t.Fatal("unknown project should error")
	}
	if _, err := svc.AddModrinthPack("https://modrinth.com/mod/some-mod", ""); err == nil {
		t.Fatal("a mod is not a modpack")
	}
	if _, err := svc.ListModrinthVersions("modrinth-fancy-pack"); err == nil {
		t.Fatal("not added yet")
	}
	if _, err := svc.AddModrinthPack("https://modrinth.com/modpack/fancy-pack", "9.9"); err == nil {
		t.Fatal("unknown version should error")
	}
	look, err := svc.LookupModrinth("https://modrinth.com/modpack/fancy-pack")
	if err != nil || look.Name != "Fancy Pack" || look.Suggested != "1.1" || len(look.Versions) != 3 || look.Versions[0].Number != "2.0-beta" || look.Versions[0].Type != "beta" || look.AlreadyAdded || look.Versions[0].PublishedAtMs == 0 {
		t.Fatalf("%+v %v", look, err)
	}
	if look, _ = svc.LookupModrinth("https://modrinth.com/modpack/fancy-pack/version/1.0"); look.Suggested != "1.0" {
		t.Fatalf("link version should be suggested: %+v", look)
	}
	// Blank version ⇒ held at the newest release from the start.
	p, err := svc.AddModrinthPack("https://modrinth.com/modpack/fancy-pack", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "modrinth-fancy-pack" || p.Source != "modrinth" || p.Project == nil || *p.Project != "fancy-pack" || p.HeldVersion == nil || *p.HeldVersion != "1.1" || p.Name != "Fancy Pack" || *p.Icon != "https://cdn.modrinth.com/icon.webp" || p.PackURL != "https://modrinth.com/modpack/fancy-pack" {
		t.Fatalf("%+v", p)
	}
	if look, _ = svc.LookupModrinth("https://modrinth.com/modpack/fancy-pack"); !look.AlreadyAdded || *look.HeldVersion != "1.1" {
		t.Fatalf("%+v", look)
	}
	if p.Launch.MaxMemoryMb != defaultHeapMb || len(p.RecommendedArgs) != 0 {
		t.Fatalf("launch: %+v", p.Launch)
	}
	st := loadSettings()
	if len(st.Modrinth) != 1 || st.Modrinth[0].ProjectID != "AbCdEf12" || st.Modrinth[0].Slug != "fancy-pack" || st.Modrinth[0].Version != "1.1" || len(st.Modrinth[0].Pack) == 0 {
		t.Fatalf("%+v", st.Modrinth)
	}

	packs, err := svc.ListPacks()
	if err != nil || len(packs) != 1 || packs[0].ID != "modrinth-fancy-pack" || packs[0].Source != "modrinth" || packs[0].Installed {
		t.Fatalf("%+v %v", packs, err)
	}

	// Held at the newest release, not the newer beta; nothing newer to offer.
	chk, err := svc.CheckPack("modrinth-fancy-pack")
	if err != nil || chk.LatestVersion != "1.1" || chk.TargetVersion != "1.1" || chk.UpdateAvailable || chk.VersionID != "neoforge-21.1.248" || chk.ToDownload != 3 || !chk.LoaderInstalled || chk.UpToDate {
		t.Fatalf("%+v %v", chk, err)
	}
	rep, err := svc.SyncPack("modrinth-fancy-pack")
	if err != nil || len(rep.Downloaded) != 3 || rep.Version != "1.1" || !rep.ProfileWritten {
		t.Fatalf("%+v %v", rep, err)
	}
	dir := PackDir("modrinth-fancy-pack")
	if b, _ := os.ReadFile(filepath.Join(dir, "config", "a.toml")); string(b) != "two" {
		t.Fatalf("override: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "options.txt")); string(b) != "fov:1" {
		t.Fatalf("client override: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "mods", "alpha.jar")); string(b) != "JAR" {
		t.Fatalf("jar: %q", b)
	}
	state, _ := packwiz.LoadState(dir)
	if state.PackVersion != "1.1" || state.PackURL != srv.URL+"/dl/1.1.mrpack" || len(state.Files) != 3 {
		t.Fatalf("%+v", state)
	}
	if prof, ok, _ := launcher.Get(mcDir, "modrinth-fancy-pack"); !ok || prof.Name != "Fancy Pack" || prof.GameDir != dir {
		t.Fatalf("%+v", prof)
	}
	if chk, _ = svc.CheckPack("modrinth-fancy-pack"); !chk.UpToDate {
		t.Fatalf("%+v", chk)
	}

	// Choosing an older release holds the pack there: the check reports the
	// move and that something newer exists, and a sync moves the pack back,
	// dropping the file 1.0 does not have.
	vs, err := svc.ListModrinthVersions("modrinth-fancy-pack")
	if err != nil || len(vs) != 3 || vs[2].Number != "1.0" {
		t.Fatalf("%+v %v", vs, err)
	}
	if _, err := svc.SetModrinthVersion("modrinth-fancy-pack", "9.9"); err == nil {
		t.Fatal("unknown version should error")
	}
	p, err = svc.SetModrinthVersion("modrinth-fancy-pack", "1.0")
	if err != nil || p.HeldVersion == nil || *p.HeldVersion != "1.0" || !p.Installed || *p.InstalledVersion != "1.1" {
		t.Fatalf("%+v %v", p, err)
	}
	if st = loadSettings(); len(st.Modrinth) != 1 || st.Modrinth[0].Version != "1.0" {
		t.Fatalf("%+v", st.Modrinth)
	}
	chk, err = svc.CheckPack("modrinth-fancy-pack")
	if err != nil || chk.LatestVersion != "1.1" || chk.TargetVersion != "1.0" || !chk.UpdateAvailable || chk.UpToDate || chk.ToDownload != 1 || chk.ToDelete != 1 {
		t.Fatalf("%+v %v", chk, err)
	}
	if rep, err = svc.SyncPack("modrinth-fancy-pack"); err != nil || rep.Version != "1.0" || len(rep.Deleted) != 1 {
		t.Fatalf("%+v %v", rep, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "options.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("options.txt should be gone at 1.0")
	}
	// A version link re-adds at that version; the id (by version id here) works too.
	if p, err = svc.AddModrinthPack("https://modrinth.com/modpack/fancy-pack/version/v11", ""); err != nil || *p.HeldVersion != "1.1" {
		t.Fatalf("%+v %v", p, err)
	}
	if p, err = svc.SetModrinthVersion("modrinth-fancy-pack", "vB"); err != nil || *p.HeldVersion != "2.0-beta" {
		t.Fatalf("%+v %v", p, err)
	}
	if p, err = svc.SetModrinthVersion("modrinth-fancy-pack", "1.0"); err != nil || *p.HeldVersion != "1.0" {
		t.Fatalf("%+v %v", p, err)
	}

	// Two projects whose slugs reduce to the same id both get listed, the
	// second under an id carrying its project id, and removing one leaves
	// the other.
	one, err := svc.AddModrinthPack("https://modrinth.com/modpack/a-b-c", "")
	if err != nil || one.ID != "modrinth-a-b-c" {
		t.Fatalf("%+v %v", one, err)
	}
	two, err := svc.AddModrinthPack("https://modrinth.com/modpack/a__b..c", "")
	if err != nil || two.ID != "modrinth-a-b-c-p2" || *two.Project != "a__b..c" {
		t.Fatalf("%+v %v", two, err)
	}
	if packs, _ = svc.ListPacks(); len(packs) != 3 {
		t.Fatalf("%+v", packs)
	}
	if err := svc.RemoveModrinthPack("modrinth-a-b-c"); err != nil {
		t.Fatal(err)
	}
	if packs, _ = svc.ListPacks(); len(packs) != 2 || packs[1].ID != "modrinth-a-b-c-p2" {
		t.Fatalf("%+v", packs)
	}
	if err := svc.RemoveModrinthPack("modrinth-a-b-c-p2"); err != nil {
		t.Fatal(err)
	}

	// Modrinth down: the pack stays listed from its cached copy.
	srv.Close()
	packs, err = svc.ListPacks()
	if err != nil || len(packs) != 1 || !packs[0].Installed || *packs[0].InstalledVersion != "1.0" {
		t.Fatalf("offline: %+v %v", packs, err)
	}

	if err := svc.RemoveModrinthPack("modrinth-nope"); err == nil {
		t.Fatal("unknown id should error")
	}
	if err := svc.RemoveModrinthPack("modrinth-fancy-pack"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ListPacks(); err != ErrNoPacks {
		t.Fatalf("expected ErrNoPacks, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mods", "alpha.jar")); err != nil {
		t.Fatal("removing a pack must leave its files")
	}
}

func TestModrinthPackID(t *testing.T) {
	for in, want := range map[string]string{
		"fabulously-optimized": "modrinth-fabulously-optimized",
		"Cobblemon (Fabric)":   "modrinth-cobblemon-fabric",
		"a__b..c":              "modrinth-a-b-c",
	} {
		if got := modrinthPackID(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
	if id := modrinthPackID(strings.Repeat("x", 80)); len(id) != 64 {
		t.Errorf("long: %d", len(id))
	}
}

// rewriteHost sends every request for the mrpack format's allowed download
// hosts to the fake server instead, so a pack can be synced end to end.
func rewriteHost(srv *httptest.Server) *http.Client {
	target, _ := url.Parse(srv.URL)
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "cdn.modrinth.com" {
			r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
			r.URL.Path = "/cdn" + r.URL.Path
		}
		return http.DefaultTransport.RoundTrip(r)
	})}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
