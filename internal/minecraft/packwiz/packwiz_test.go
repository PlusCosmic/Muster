package packwiz

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakePack builds a consistent pack: files are served under /p/, and the
// index and pack.toml hashes are computed from the served bytes.
type fakePack struct {
	files  map[string][]byte // path -> content (pack files and metafiles)
	mods   map[string][]byte // download URL path -> jar bytes (served under /dl/)
	cfDeny map[string]int    // /dl/ path -> status to return instead
	srv    *httptest.Server
	hits   map[string]int
}

func newFakePack(t *testing.T) *fakePack {
	t.Helper()
	fp := &fakePack{files: map[string][]byte{}, mods: map[string][]byte{}, cfDeny: map[string]int{}, hits: map[string]int{}}
	fp.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fp.hits[r.URL.Path]++
		if strings.HasPrefix(r.URL.Path, "/p/") {
			if b, ok := fp.files[strings.TrimPrefix(r.URL.Path, "/p/")]; ok {
				_, _ = w.Write(b)
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/dl/") {
			if code, ok := fp.cfDeny[r.URL.Path]; ok {
				w.WriteHeader(code)
				return
			}
			if b, ok := fp.mods[r.URL.Path]; ok {
				_, _ = w.Write(b)
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(fp.srv.Close)
	return fp
}

func (fp *fakePack) packURL() string { return fp.srv.URL + "/p/pack.toml" }

// metafile adds a mod under mods/<name>.pw.toml with a direct URL.
func (fp *fakePack) metafile(name, side string, jar []byte, extra string) {
	h, _ := HashBytes("sha512", jar)
	fp.mods["/dl/"+name+".jar"] = jar
	fp.files["mods/"+name+".pw.toml"] = []byte(fmt.Sprintf(`name = %q
filename = %q
side = %q

[download]
url = %q
hash-format = "sha512"
hash = %q
%s
`, name, name+".jar", side, fp.srv.URL+"/dl/"+name+".jar", h, extra))
}

// finish writes index.toml and pack.toml over whatever files exist.
func (fp *fakePack) finish(version string, extraIndex string) {
	var b strings.Builder
	b.WriteString("hash-format = \"sha256\"\n")
	for p, content := range fp.files {
		if p == "index.toml" || p == "pack.toml" {
			continue
		}
		h, _ := HashBytes("sha256", content)
		fmt.Fprintf(&b, "\n[[files]]\nfile = %q\nhash = %q\n", p, h)
		if strings.HasSuffix(p, ".pw.toml") {
			b.WriteString("metafile = true\n")
		}
		if strings.Contains(extraIndex, p) {
			b.WriteString("preserve = true\n")
		}
	}
	index := []byte(b.String())
	fp.files["index.toml"] = index
	ih, _ := HashBytes("sha256", index)
	fp.files["pack.toml"] = []byte(fmt.Sprintf(`name = "Test Pack"
version = %q
pack-format = "packwiz:1.1.0"

[index]
file = "index.toml"
hash-format = "sha256"
hash = %q

[versions]
minecraft = "1.21.1"
neoforge = "21.1.248"
`, version, ih))
}

func TestLoadResolvesEveryClientFileAndSkipsServerOnes(t *testing.T) {
	fp := newFakePack(t)
	fp.files["config/a.toml"] = []byte("x = 1\n")
	fp.metafile("alpha", "both", []byte("ALPHA"), "")
	fp.metafile("beta", "client", []byte("BETA"), "")
	fp.metafile("gamma", "server", []byte("GAMMA"), "")
	fp.finish("1.0.0", "")

	res, err := (&Client{}).Load(context.Background(), fp.packURL())
	if err != nil {
		t.Fatal(err)
	}
	if res.Pack.Name != "Test Pack" {
		t.Fatalf("pack: %+v", res.Pack)
	}
	if l, v := res.Pack.Loader(); l != "neoforge" || v != "21.1.248" {
		t.Fatalf("loader %s %s", l, v)
	}
	paths := map[string]Entry{}
	for _, e := range res.Entries {
		paths[e.Path] = e
	}
	for _, want := range []string{"config/a.toml", "mods/alpha.jar", "mods/beta.jar"} {
		if _, ok := paths[want]; !ok {
			t.Fatalf("missing %s in %v", want, paths)
		}
	}
	if _, ok := paths["mods/gamma.jar"]; ok {
		t.Fatal("server-side mod should be skipped")
	}
	if paths["config/a.toml"].URL != fp.srv.URL+"/p/config/a.toml" {
		t.Fatalf("plain file url %s", paths["config/a.toml"].URL)
	}
}

func TestLoadRejectsTamperedIndex(t *testing.T) {
	fp := newFakePack(t)
	fp.metafile("alpha", "both", []byte("ALPHA"), "")
	fp.finish("1.0.0", "")
	fp.files["index.toml"] = append(fp.files["index.toml"], '\n', '#', 'x')
	if _, err := (&Client{}).Load(context.Background(), fp.packURL()); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected hash error, got %v", err)
	}
}

func TestCurseForgeURLPattern(t *testing.T) {
	got := curseForgeURL(7317672, "mcw-mcwwindows-2.4.2-mc1.21.1neoforge.jar")
	want := "https://edge.forgecdn.net/files/7317/672/mcw-mcwwindows-2.4.2-mc1.21.1neoforge.jar"
	if got != want {
		t.Fatalf("got %s", got)
	}
	if got := curseForgeURL(5045, "a b+c.jar"); got != "https://edge.forgecdn.net/files/5/45/a%20b+c.jar" {
		t.Fatalf("got %s", got)
	}
}

func TestResolveMetafileCurseForgeNeedsFileID(t *testing.T) {
	m := Metafile{Name: "x", Filename: "x.jar", Download: Download{Mode: "metadata:curseforge", HashFormat: "sha1", Hash: "aa"}}
	if _, err := resolveMetafile("mods/x.pw.toml", m); err == nil {
		t.Fatal("expected error without file-id")
	}
	m.Update.CurseForge = &CurseForgeUpdate{FileID: 1234567, ProjectID: 1}
	e, err := resolveMetafile("mods/x.pw.toml", m)
	if err != nil || !e.CurseForge || e.Path != "mods/x.jar" || !strings.Contains(e.URL, "/1234/567/") {
		t.Fatalf("%+v %v", e, err)
	}
}

func TestPlanAndApplyRoundTrip(t *testing.T) {
	fp := newFakePack(t)
	fp.files["config/a.toml"] = []byte("x = 1\n")
	fp.metafile("alpha", "both", []byte("ALPHA"), "")
	fp.metafile("beta", "client", []byte("BETA"), "")
	fp.finish("1.0.0", "")
	c := &Client{}
	ctx := context.Background()
	dir := t.TempDir()

	res, err := c.Load(ctx, fp.packURL())
	if err != nil {
		t.Fatal(err)
	}
	state, _ := LoadState(dir)
	plan := MakePlan(res, dir, state, nil)
	if len(plan.Download) != 3 || len(plan.Delete) != 0 {
		t.Fatalf("fresh plan: %+v", plan)
	}
	var seen []string
	rep, err := c.Apply(ctx, res, dir, plan, fp.packURL(), func(done, total int, e Entry) { seen = append(seen, fmt.Sprintf("%d/%d %s", done, total, e.Path)) })
	if err != nil || len(rep.Downloaded) != 3 || len(rep.Manual) != 0 {
		t.Fatalf("apply: %+v %v", rep, err)
	}
	if len(seen) != 3 || seen[0] != "1/3 config/a.toml" {
		t.Fatalf("progress %v", seen)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "mods", "alpha.jar")); string(b) != "ALPHA" {
		t.Fatalf("alpha: %q", b)
	}

	// Second sync: nothing to do.
	state, _ = LoadState(dir)
	if state.PackVersion != "1.0.0" || len(state.Files) != 3 {
		t.Fatalf("state %+v", state)
	}
	if plan := MakePlan(res, dir, state, nil); !plan.Empty() || plan.Keep != 3 {
		t.Fatalf("second plan: %+v", plan)
	}

	// Pack update: alpha changes, beta leaves, delta arrives; user deleted a.toml.
	fp.metafile("alpha", "both", []byte("ALPHA2"), "")
	delete(fp.files, "mods/beta.pw.toml")
	fp.metafile("delta", "both", []byte("DELTA"), "")
	fp.finish("1.1.0", "")
	_ = os.Remove(filepath.Join(dir, "config", "a.toml"))
	res, err = c.Load(ctx, fp.packURL())
	if err != nil {
		t.Fatal(err)
	}
	plan = MakePlan(res, dir, state, nil)
	if len(plan.Download) != 3 || len(plan.Delete) != 1 || plan.Delete[0] != "mods/beta.jar" {
		t.Fatalf("update plan: %+v", plan)
	}
	rep, err = c.Apply(ctx, res, dir, plan, fp.packURL(), nil)
	if err != nil || len(rep.Deleted) != 1 || len(rep.Downloaded) != 3 {
		t.Fatalf("apply update: %+v %v", rep, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mods", "beta.jar")); !os.IsNotExist(err) {
		t.Fatal("beta should be gone")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "mods", "alpha.jar")); string(b) != "ALPHA2" {
		t.Fatalf("alpha: %q", b)
	}
}

func TestApplyLeavesUserFilesAloneAndRefusesBadHash(t *testing.T) {
	fp := newFakePack(t)
	fp.metafile("alpha", "both", []byte("ALPHA"), "")
	fp.finish("1.0.0", "")
	dir := t.TempDir()
	// A mod the user dropped in themselves is not in the state, so never deleted.
	if err := os.MkdirAll(filepath.Join(dir, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mods", "mine.jar"), []byte("MINE"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Client{}
	res, _ := c.Load(context.Background(), fp.packURL())
	state, _ := LoadState(dir)
	if _, err := c.Apply(context.Background(), res, dir, MakePlan(res, dir, state, nil), fp.packURL(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mods", "mine.jar")); err != nil {
		t.Fatal("user's jar should survive")
	}

	// CDN serves a different jar than the metafile promises: refuse it.
	fp.mods["/dl/alpha.jar"] = []byte("EVIL")
	_ = os.Remove(filepath.Join(dir, "mods", "alpha.jar"))
	state, _ = LoadState(dir)
	plan := MakePlan(res, dir, state, nil)
	if len(plan.Download) != 1 {
		t.Fatalf("plan %+v", plan)
	}
	if _, err := c.Apply(context.Background(), res, dir, plan, fp.packURL(), nil); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected hash error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mods", "alpha.jar")); !os.IsNotExist(err) {
		t.Fatal("bad download must not land on disk")
	}
}

func TestApplyReportsRefusedCurseForgeFilesInsteadOfFailing(t *testing.T) {
	fp := newFakePack(t)
	fp.metafile("alpha", "both", []byte("ALPHA"), "")
	// A CurseForge-mode metafile whose derived URL we point at our server.
	jar := []byte("CF")
	h, _ := HashBytes("sha1", jar)
	fp.files["mods/cf.pw.toml"] = []byte(fmt.Sprintf(`name = "CF Mod"
filename = "cf.jar"
side = "both"

[download]
hash-format = "sha1"
hash = %q
mode = "metadata:curseforge"

[update]
[update.curseforge]
file-id = 7317672
project-id = 1
`, h))
	fp.finish("1.0.0", "")
	dir := t.TempDir()
	c := &Client{}
	res, err := c.Load(context.Background(), fp.packURL())
	if err != nil {
		t.Fatal(err)
	}
	// Rewrite the derived URL onto the test server and have it refuse.
	for i := range res.Entries {
		if res.Entries[i].CurseForge {
			res.Entries[i].URL = fp.srv.URL + "/dl/cf.jar"
			fp.cfDeny["/dl/cf.jar"] = 403
		}
	}
	state, _ := LoadState(dir)
	rep, err := c.Apply(context.Background(), res, dir, MakePlan(res, dir, state, nil), fp.packURL(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Downloaded) != 1 || len(rep.Manual) != 1 || rep.Manual[0].Path != "mods/cf.jar" {
		t.Fatalf("%+v", rep)
	}
	// Not recorded as installed, so the next plan asks for it again.
	state, _ = LoadState(dir)
	if plan := MakePlan(res, dir, state, nil); len(plan.Download) != 1 || plan.Download[0].Path != "mods/cf.jar" {
		t.Fatalf("%+v", plan)
	}
}

func TestOptionalFilesFollowDefaultUnlessExcluded(t *testing.T) {
	fp := newFakePack(t)
	fp.metafile("shader", "client", []byte("S"), "[option]\noptional = true\ndefault = false\n")
	fp.metafile("hud", "client", []byte("H"), "[option]\noptional = true\ndefault = true\n")
	fp.finish("1.0.0", "")
	res, err := (&Client{}).Load(context.Background(), fp.packURL())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	state, _ := LoadState(dir)
	plan := MakePlan(res, dir, state, nil)
	if len(plan.Download) != 1 || plan.Download[0].Path != "mods/hud.jar" {
		t.Fatalf("%+v", plan)
	}
	plan = MakePlan(res, dir, state, map[string]bool{"mods/hud.jar": true})
	if len(plan.Download) != 0 {
		t.Fatalf("%+v", plan)
	}
}

func TestParseIndexRefusesEscapingPaths(t *testing.T) {
	for _, bad := range []string{"../x", "/etc/passwd", `a\b`} {
		if _, err := ParseIndex([]byte(fmt.Sprintf("[[files]]\nfile = %q\nhash = \"00\"\n", bad))); err == nil {
			t.Fatalf("%q should be refused", bad)
		}
	}
}
