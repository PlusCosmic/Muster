package modrinth

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"muster/internal/minecraft/packwiz"
)

func TestParseRef(t *testing.T) {
	cases := []struct {
		in      string
		ok      bool
		project string
		version string
	}{
		{"https://modrinth.com/modpack/fabulously-optimized", true, "fabulously-optimized", ""},
		{"  modrinth.com/modpack/fabulously-optimized/  ", true, "fabulously-optimized", ""},
		{"https://modrinth.com/modpack/fabulously-optimized/version/13.4.0", true, "fabulously-optimized", "13.4.0"},
		{"https://modrinth.com/modpack/fabulously-optimized/versions?l=fabric", true, "fabulously-optimized", ""},
		{"https://www.modrinth.com/project/1KVo5zza/version/HiDsl7yh", true, "1KVo5zza", "HiDsl7yh"},
		{"https://modrinth.com/", true, "", ""},
		{"amber-otter-42", false, "", ""},
		{"https://example.com/modrinth.com/x", false, "", ""},
		{"https://notmodrinth.com/modpack/x", false, "", ""},
	}
	for _, c := range cases {
		ref, ok := ParseRef(c.in)
		if ok != c.ok || ref.Project != c.project || ref.Version != c.version {
			t.Errorf("%q: got %+v %v, want %q %q %v", c.in, ref, ok, c.project, c.version, c.ok)
		}
	}
	if (Ref{}).Validate() == nil || (Ref{Project: "a/b"}).Validate() == nil || (Ref{Project: "ok-slug"}).Validate() != nil {
		t.Fatal("Validate")
	}
}

func TestPick(t *testing.T) {
	vs := []Version{
		{ID: "b", Number: "2.0-beta", Type: "beta", DatePublished: "2026-03-01"},
		{ID: "r1", Number: "1.0", Type: "release", DatePublished: "2026-01-01"},
		{ID: "r2", Number: "1.1", Type: "release", DatePublished: "2026-02-01"},
	}
	if v, err := Pick(vs, ""); err != nil || v.ID != "r2" {
		t.Fatalf("newest release: %+v %v", v, err)
	}
	if v, err := Pick(vs, "1.0"); err != nil || v.ID != "r1" {
		t.Fatalf("pin by number: %+v %v", v, err)
	}
	if v, err := Pick(vs, "b"); err != nil || v.Number != "2.0-beta" {
		t.Fatalf("pin by id: %+v %v", v, err)
	}
	if _, err := Pick(vs, "9.9"); err == nil {
		t.Fatal("missing pin should error")
	}
	if v, err := Pick(vs[:1], ""); err != nil || v.ID != "b" {
		t.Fatalf("no release ⇒ newest of anything: %+v %v", v, err)
	}
	if _, err := Pick(nil, ""); err == nil {
		t.Fatal("no versions should error")
	}
}

// buildPack zips an index and overrides into an .mrpack.
func buildPack(t *testing.T, idx any, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	raw, _ := json.Marshal(idx)
	w, _ := zw.Create(IndexFile)
	_, _ = w.Write(raw)
	for name, content := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func index(files []IndexEntry) map[string]any {
	return map[string]any{
		"formatVersion": 1, "game": "minecraft", "versionId": "1.2.3", "name": "Test Pack", "summary": "A test",
		"files":        files,
		"dependencies": map[string]string{"minecraft": "1.21.1", "fabric-loader": "0.16.9"},
	}
}

func TestResolveTurnsIndexAndOverridesIntoEntries(t *testing.T) {
	raw := buildPack(t, index([]IndexEntry{
		{Path: "mods/alpha.jar", Hashes: map[string]string{"sha1": "aa", "sha512": "ff"}, Env: &Env{Client: "required", Server: "required"}, Downloads: []string{"https://cdn.modrinth.com/data/x/alpha.jar"}},
		{Path: "mods/server-only.jar", Hashes: map[string]string{"sha512": "ff"}, Env: &Env{Client: "unsupported", Server: "required"}, Downloads: []string{"https://cdn.modrinth.com/data/x/s.jar"}},
		{Path: "mods/maybe.jar", Hashes: map[string]string{"sha1": "aa"}, Env: &Env{Client: "optional", Server: "optional"}, Downloads: []string{"https://github.com/x/y/releases/maybe.jar"}},
	}), map[string]string{
		"overrides/":                         "",
		"overrides/config/a.toml":            "base",
		"overrides/config/b.toml":            "b",
		"client-overrides/config/a.toml":     "client wins",
		"client-overrides/options.txt":       "fov:1",
		"server-overrides/server.properties": "ignored",
	})
	res, err := Resolve(raw, Version{Number: "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Pack.Name != "Test Pack" || res.Pack.Version != "1.2.3" || res.Pack.Versions["minecraft"] != "1.21.1" || res.Pack.Versions["fabric"] != "0.16.9" {
		t.Fatalf("%+v", res.Pack)
	}
	if l, v := res.Pack.Loader(); l != "fabric" || v != "0.16.9" {
		t.Fatalf("%s %s", l, v)
	}
	byPath := map[string]packwiz.Entry{}
	for _, e := range res.Entries {
		byPath[e.Path] = e
	}
	if len(byPath) != 5 {
		t.Fatalf("entries: %+v", res.Entries)
	}
	if e := byPath["mods/alpha.jar"]; e.HashFormat != "sha512" || e.Hash != "ff" || e.URL == "" || e.Optional || e.Open != nil {
		t.Fatalf("alpha: %+v", e)
	}
	if e := byPath["mods/maybe.jar"]; e.HashFormat != "sha1" || !e.Optional || !e.Default {
		t.Fatalf("maybe: %+v", e)
	}
	if _, ok := byPath["mods/server-only.jar"]; ok {
		t.Fatal("server-only file resolved")
	}
	if _, ok := byPath["server.properties"]; ok {
		t.Fatal("server override resolved")
	}
	a := byPath["config/a.toml"]
	if a.Open == nil || a.HashFormat != "sha512" {
		t.Fatalf("a: %+v", a)
	}
	rc, err := a.Open()
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "client wins" {
		t.Fatalf("client-overrides should win: %q", got)
	}
	want, _ := packwiz.HashBytes("sha512", []byte("client wins"))
	if a.Hash != want {
		t.Fatalf("hash of override: %s", a.Hash)
	}
	if _, ok := byPath["options.txt"]; !ok {
		t.Fatal("client-overrides/options.txt missing")
	}
}

func TestResolveRefusesBadPacks(t *testing.T) {
	good := []IndexEntry{{Path: "mods/a.jar", Hashes: map[string]string{"sha512": "ff"}, Downloads: []string{"https://cdn.modrinth.com/a.jar"}}}
	cases := map[string][]byte{
		"not a zip":            []byte("nope"),
		"no index":             buildPack(t, nil, map[string]string{"overrides/x": "y"})[:0],
		"escaping index path":  buildPack(t, index([]IndexEntry{{Path: "../a.jar", Hashes: map[string]string{"sha512": "ff"}, Downloads: []string{"https://cdn.modrinth.com/a.jar"}}}), nil),
		"escaping override":    buildPack(t, index(good), map[string]string{"overrides/../evil": "x"}),
		"disallowed host":      buildPack(t, index([]IndexEntry{{Path: "mods/a.jar", Hashes: map[string]string{"sha512": "ff"}, Downloads: []string{"https://evil.example/a.jar"}}}), nil),
		"no hash":              buildPack(t, index([]IndexEntry{{Path: "mods/a.jar", Downloads: []string{"https://cdn.modrinth.com/a.jar"}}}), nil),
		"collision with state": buildPack(t, index(good), map[string]string{"overrides/" + packwiz.StateFile: "x"}),
		"duplicate path":       buildPack(t, index(append(good, good[0])), nil),
	}
	// "no index" needs a real zip without the index; build it by hand.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("overrides/x")
	_, _ = w.Write([]byte("y"))
	_ = zw.Close()
	cases["no index"] = buf.Bytes()
	for name, raw := range cases {
		if _, err := Resolve(raw, Version{}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	bad := index(good)
	bad["dependencies"] = map[string]string{"fabric-loader": "1"}
	if _, err := Resolve(buildPack(t, bad, nil), Version{}); err == nil {
		t.Error("no minecraft version: accepted")
	}
}

func TestClientLoadsProjectVersionsAndPack(t *testing.T) {
	raw := buildPack(t, index(nil), map[string]string{"overrides/config/a.toml": "a"})
	sha, _ := packwiz.HashBytes("sha512", raw)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/project/test-pack":
			_ = json.NewEncoder(w).Encode(Project{ID: "abc12345", Slug: "test-pack", Title: "Test Pack", ProjectType: "modpack"})
		case "/v2/project/a-mod":
			_ = json.NewEncoder(w).Encode(Project{ID: "m", Slug: "a-mod", Title: "A Mod", ProjectType: "mod"})
		case "/v2/project/test-pack/version":
			_ = json.NewEncoder(w).Encode([]Version{{ID: "v1", Number: "1.2.3", Type: "release", DatePublished: "2026-01-01", Files: []File{
				{URL: srv.URL + "/dl/other.zip", Filename: "other.zip", Hashes: map[string]string{"sha512": "00"}},
				{URL: srv.URL + "/dl/test.mrpack", Filename: "test.mrpack", Primary: true, Hashes: map[string]string{"sha512": sha}},
			}}})
		case "/dl/test.mrpack":
			_, _ = w.Write(raw)
		case "/dl/tampered.mrpack":
			_, _ = w.Write(append(raw, 0))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"not_found","description":"no"}`))
		}
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL + "/v2", HTTP: srv.Client()}
	ctx := context.Background()
	p, err := c.Project(ctx, "test-pack")
	if err != nil || p.ID != "abc12345" || p.PageURL() != "https://modrinth.com/modpack/test-pack" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := c.Project(ctx, "a-mod"); err == nil {
		t.Fatal("a mod is not a modpack")
	}
	if _, err := c.Project(ctx, "nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	vs, err := c.Versions(ctx, "test-pack")
	if err != nil || len(vs) != 1 {
		t.Fatal(err)
	}
	v, _ := Pick(vs, "")
	res, err := c.Load(ctx, v)
	if err != nil || res.Pack.Version != "1.2.3" || len(res.Entries) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	v.Files[1].URL = srv.URL + "/dl/tampered.mrpack"
	if _, err := c.Load(ctx, v); err == nil {
		t.Fatal("tampered pack should fail its hash")
	}
}

// TestRealPack resolves a local .mrpack. Opt-in:
//
//	MUSTER_TEST_MRPACK=/path/to/pack.mrpack go test ./internal/minecraft/modrinth -run RealPack -v
func TestRealPack(t *testing.T) {
	path := os.Getenv("MUSTER_TEST_MRPACK")
	if path == "" {
		t.Skip("set MUSTER_TEST_MRPACK to resolve a real pack")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Resolve(raw, Version{})
	if err != nil {
		t.Fatal(err)
	}
	l, lv := res.Pack.Loader()
	t.Logf("%s %s: minecraft %s, %s %s, %d entries", res.Pack.Name, res.Pack.Version, res.Pack.Versions["minecraft"], l, lv, len(res.Entries))
	for _, e := range res.Entries[:min(8, len(res.Entries))] {
		t.Logf("  %s  %s  local=%v", e.Path, e.URL, e.Open != nil)
	}
}
