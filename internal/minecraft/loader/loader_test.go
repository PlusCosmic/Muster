package loader

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"muster/internal/minecraft/launcher"
)

func TestFabricWritesMetaProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fabric/1.21.1/0.16.9/profile/json" {
			_, _ = w.Write([]byte(`{"id":"fabric-loader-0.16.9-1.21.1","inheritsFrom":"1.21.1","mainClass":"net.fabricmc.loader.impl.launch.knot.KnotClient"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	old := FabricMeta
	FabricMeta = srv.URL + "/fabric/%s/%s/profile/json"
	defer func() { FabricMeta = old }()

	mc := t.TempDir()
	var steps []string
	id, err := (&Installer{}).Ensure(context.Background(), mc, "1.21.1", "fabric", "0.16.9", t.TempDir(),
		func() (string, error) { t.Fatal("fabric must not need java"); return "", nil },
		func(s string) { steps = append(steps, s) })
	if err != nil || id != "fabric-loader-0.16.9-1.21.1" {
		t.Fatalf("%s %v", id, err)
	}
	if !launcher.HasVersion(mc, id) {
		t.Fatal("version json missing")
	}
	if len(steps) != 1 {
		t.Fatalf("%v", steps)
	}
	// Already installed: no network, no steps.
	FabricMeta = "http://127.0.0.1:1/%s/%s"
	steps = nil
	if id2, err := (&Installer{}).Ensure(context.Background(), mc, "1.21.1", "fabric", "0.16.9", t.TempDir(), nil, func(s string) { steps = append(steps, s) }); err != nil || id2 != id || len(steps) != 0 {
		t.Fatalf("%s %v %v", id2, err, steps)
	}
	// Wrong id from the meta server is refused.
	if _, err := (&Installer{}).Ensure(context.Background(), mc, "1.21.1", "fabric", "0.99.0", t.TempDir(), nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestNeoForgeRunsInstallerAndCleansInjectedProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "neoforge-21.1.248-installer.jar") {
			_, _ = w.Write([]byte("PK-fake-jar"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	old := NeoForgeJar
	NeoForgeJar = srv.URL + "/neoforge/%[1]s/neoforge-%[1]s-installer.jar"
	defer func() { NeoForgeJar = old }()

	mc := t.TempDir()
	// The user already has a profile; the installer must not disturb it.
	if err := launcher.Upsert(mc, "frontier", launcher.Profile{Name: "Pack", LastVersionID: "neoforge-21.1.248"}); err != nil {
		t.Fatal(err)
	}
	var ran []string
	fake := func(ctx context.Context, java, jar string, args []string, cwd, logPath string) error {
		ran = append(ran, java, filepath.Base(jar), strings.Join(args, " "))
		if _, err := os.Stat(jar); err != nil {
			return err
		}
		// Behave like the real installer: create the version and inject a profile.
		vdir := filepath.Join(mc, "versions", "neoforge-21.1.248")
		_ = os.MkdirAll(vdir, 0o755)
		_ = os.WriteFile(filepath.Join(vdir, "neoforge-21.1.248.json"), []byte(`{"id":"neoforge-21.1.248"}`), 0o644)
		p := filepath.Join(mc, launcher.ProfilesFile)
		raw, _ := os.ReadFile(p)
		var doc map[string]json.RawMessage
		_ = json.Unmarshal(raw, &doc)
		var profiles map[string]json.RawMessage
		_ = json.Unmarshal(doc["profiles"], &profiles)
		profiles["NeoForge"] = json.RawMessage(`{"name":"NeoForge","type":"custom","lastVersionId":"neoforge-21.1.248","icon":"data:…"}`)
		doc["profiles"], _ = json.Marshal(profiles)
		out, _ := json.Marshal(doc)
		return os.WriteFile(p, out, 0o644)
	}
	work := t.TempDir()
	var steps []string
	id, err := (&Installer{Run: fake}).Ensure(context.Background(), mc, "1.21.1", "neoforge", "21.1.248", work,
		func() (string, error) { return "/fake/java", nil }, func(s string) { steps = append(steps, s) })
	if err != nil || id != "neoforge-21.1.248" {
		t.Fatalf("%s %v", id, err)
	}
	if len(ran) != 3 || ran[0] != "/fake/java" || ran[1] != "neoforge-21.1.248-installer.jar" || ran[2] != "--install-client "+mc {
		t.Fatalf("ran %v", ran)
	}
	if len(steps) != 3 {
		t.Fatalf("steps %v", steps)
	}
	raw, _ := os.ReadFile(filepath.Join(mc, launcher.ProfilesFile))
	if strings.Contains(string(raw), `"NeoForge"`) {
		t.Fatalf("injected profile should be removed: %s", raw)
	}
	if _, ok, _ := launcher.Get(mc, "frontier"); !ok {
		t.Fatal("our profile must survive")
	}
	if entries, _ := os.ReadDir(work); len(entries) != 0 {
		t.Fatalf("installer jar should be cleaned up, left %v", entries)
	}
}

func TestNeoForgeCreatesProfilesFileForFreshDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("jar")) }))
	defer srv.Close()
	old := NeoForgeJar
	NeoForgeJar = srv.URL + "/%s.jar"
	defer func() { NeoForgeJar = old }()
	mc := filepath.Join(t.TempDir(), "fresh")
	fake := func(ctx context.Context, java, jar string, args []string, cwd, logPath string) error {
		if _, err := os.Stat(filepath.Join(mc, launcher.ProfilesFile)); err != nil {
			return errors.New("installer needs launcher_profiles.json")
		}
		vdir := filepath.Join(mc, "versions", "neoforge-1")
		_ = os.MkdirAll(vdir, 0o755)
		return os.WriteFile(filepath.Join(vdir, "neoforge-1.json"), []byte(`{}`), 0o644)
	}
	if _, err := (&Installer{Run: fake}).Ensure(context.Background(), mc, "1.21.1", "neoforge", "1", t.TempDir(), func() (string, error) { return "j", nil }, nil); err != nil {
		t.Fatal(err)
	}
}

func TestInstallerFailureSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("jar")) }))
	defer srv.Close()
	old := NeoForgeJar
	NeoForgeJar = srv.URL + "/%s.jar"
	defer func() { NeoForgeJar = old }()
	fake := func(context.Context, string, string, []string, string, string) error {
		return errors.New("exit status 1")
	}
	_, err := (&Installer{Run: fake}).Ensure(context.Background(), t.TempDir(), "1.21.1", "neoforge", "1", t.TempDir(), func() (string, error) { return "j", nil }, nil)
	if err == nil || !strings.Contains(err.Error(), "installer failed") {
		t.Fatalf("%v", err)
	}
	// Java lookup failure is reported before anything is downloaded.
	_, err = (&Installer{}).Ensure(context.Background(), t.TempDir(), "1.21.1", "neoforge", "1", t.TempDir(), func() (string, error) { return "", errors.New("no java") }, nil)
	if err == nil || !strings.Contains(err.Error(), "no java") {
		t.Fatalf("%v", err)
	}
}

func TestUnsupportedLoader(t *testing.T) {
	if _, err := (&Installer{}).Ensure(context.Background(), t.TempDir(), "1.21.1", "rift", "1", t.TempDir(), nil, nil); err == nil {
		t.Fatal("expected error")
	}
	if id, err := (&Installer{}).Ensure(context.Background(), t.TempDir(), "1.21.1", "", "", t.TempDir(), nil, nil); err != nil || id != "1.21.1" {
		t.Fatalf("%s %v", id, err)
	}
}
