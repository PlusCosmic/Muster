package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const existing = `{
  "profiles": {
    "abc123": {"name": "Latest release", "type": "latest-release", "lastVersionId": "latest-release", "created": "2025-01-01T00:00:00.000Z", "lastUsed": "2025-06-01T00:00:00.000Z", "icon": "Grass"},
    "muster-frontier": {"name": "Old", "type": "custom", "lastVersionId": "neoforge-21.1.100", "created": "2025-03-03T00:00:00.000Z", "resolution": {"width": 1920, "height": 1080}, "javaDir": "C:\\\\my\\\\java.exe"}
  },
  "settings": {"crashAssistance": true, "enableSnapshots": false},
  "version": 3,
  "somethingNew": {"the launcher": "added this"}
}`

func TestUpsertMergesWithoutClobbering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ProfilesFile)
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Upsert(dir, "frontier", Profile{
		Name: "Frontier", LastVersionID: "neoforge-21.1.248", GameDir: `/data/packs/frontier`, JavaArgs: "-Xmx8G",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"settings", "version", "somethingNew"} {
		if _, ok := top[k]; !ok {
			t.Fatalf("lost top-level %q", k)
		}
	}
	var profiles map[string]map[string]any
	_ = json.Unmarshal(top["profiles"], &profiles)
	if profiles["abc123"]["name"] != "Latest release" || profiles["abc123"]["icon"] != "Grass" {
		t.Fatalf("other profile damaged: %v", profiles["abc123"])
	}
	ours := profiles["muster-frontier"]
	if ours["name"] != "Frontier" || ours["lastVersionId"] != "neoforge-21.1.248" || ours["type"] != "custom" || ours["javaArgs"] != "-Xmx8G" {
		t.Fatalf("ours: %v", ours)
	}
	if ours["created"] != "2025-03-03T00:00:00.000Z" {
		t.Fatalf("created should be preserved: %v", ours["created"])
	}
	if ours["lastUsed"] == nil || !strings.HasSuffix(ours["lastUsed"].(string), "Z") {
		t.Fatalf("lastUsed should be set: %v", ours["lastUsed"])
	}
	if ours["javaDir"] != `C:\\my\\java.exe` {
		t.Fatalf("user's javaDir should survive: %v", ours["javaDir"])
	}
	if res, ok := ours["resolution"].(map[string]any); !ok || res["width"] != float64(1920) {
		t.Fatalf("launcher-managed resolution should survive: %v", ours["resolution"])
	}

	got, ok, err := Get(dir, "frontier")
	if err != nil || !ok || got.GameDir != "/data/packs/frontier" {
		t.Fatalf("%+v %v %v", got, ok, err)
	}
}

func TestUpsertCreatesFileAndStoreVariantWhenPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, StoreProfilesFile), []byte(`{"profiles":{},"version":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Upsert(dir, "p", Profile{Name: "P", LastVersionID: "1.21.1"}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{ProfilesFile, StoreProfilesFile} {
		raw, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil || !strings.Contains(string(raw), `"muster-p"`) {
			t.Fatalf("%s: %v %s", f, err, raw)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(dir, ProfilesFile))
	if !strings.Contains(string(raw), `"version": 3`) {
		t.Fatalf("fresh file should carry version 3: %s", raw)
	}
	if err := Remove(dir, "p"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := Get(dir, "p"); ok {
		t.Fatal("should be removed")
	}
}

func TestVersionID(t *testing.T) {
	cases := []struct{ mc, loader, v, want string }{
		{"1.21.1", "neoforge", "21.1.248", "neoforge-21.1.248"},
		{"1.21.1", "fabric", "0.16.9", "fabric-loader-0.16.9-1.21.1"},
		{"1.20.1", "forge", "47.3.0", "1.20.1-forge-47.3.0"},
		{"1.21.1", "", "", "1.21.1"},
	}
	for _, c := range cases {
		got, err := VersionID(c.mc, c.loader, c.v)
		if err != nil || got != c.want {
			t.Errorf("%v: got %q %v", c, got, err)
		}
	}
	if _, err := VersionID("1.21.1", "rift", "1"); err == nil {
		t.Fatal("unknown loader should error")
	}
}

func TestHasVersionAndInstalled(t *testing.T) {
	dir := t.TempDir()
	if Installed(dir) || HasVersion(dir, "neoforge-1") {
		t.Fatal("empty dir")
	}
	if err := os.MkdirAll(filepath.Join(dir, "versions", "neoforge-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !Installed(dir) || HasVersion(dir, "neoforge-1") {
		t.Fatal("versions dir without json")
	}
	if err := os.WriteFile(filepath.Join(dir, "versions", "neoforge-1", "neoforge-1.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasVersion(dir, "neoforge-1") {
		t.Fatal("should be present")
	}
}
