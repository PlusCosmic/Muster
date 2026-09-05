package packwiz

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestRealPack exercises Load and MakePlan against a live pack, and checks
// that every CurseForge-derived URL is actually served, downloading the first
// few files fully to verify hashes end to end. Opt-in:
//
//	MUSTER_TEST_PACK_URL=https://…/pack.toml go test -v ./internal/minecraft/packwiz -run RealPack
func TestRealPack(t *testing.T) {
	url := os.Getenv("MUSTER_TEST_PACK_URL")
	if url == "" {
		t.Skip("set MUSTER_TEST_PACK_URL to test against a live pack")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c := &Client{HTTP: &http.Client{Timeout: time.Minute}, UserAgent: "Muster-test"}
	res, err := c.Load(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	loader, lv := res.Pack.Loader()
	t.Logf("pack %q v%s mc=%s %s=%s: %d client entries", res.Pack.Name, res.Pack.Version, res.Pack.Versions["minecraft"], loader, lv, len(res.Entries))

	cf := 0
	for _, e := range res.Entries {
		if !e.CurseForge {
			continue
		}
		cf++
		req, _ := http.NewRequestWithContext(ctx, http.MethodHead, e.URL, nil)
		req.Header.Set("User-Agent", "Muster-test")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			t.Errorf("%s: %v", e.Name, err)
			continue
		}
		resp.Body.Close()
		t.Logf("curseforge %-40s HTTP %d  %s", e.Name, resp.StatusCode, e.URL)
		if resp.StatusCode != 200 {
			t.Errorf("%s: CDN pattern refused (%d)", e.Name, resp.StatusCode)
		}
	}
	t.Logf("%d curseforge entries", cf)

	dir := t.TempDir()
	plan := MakePlan(res, dir, State{Files: map[string]string{}}, nil)
	if len(plan.Download) != len(res.Entries) {
		t.Fatalf("fresh plan should download everything: %d vs %d", len(plan.Download), len(res.Entries))
	}
	// Download a handful end to end, preferring one CurseForge file.
	var sample []Entry
	for _, e := range plan.Download {
		if e.CurseForge && len(sample) == 0 {
			sample = append(sample, e)
		}
	}
	for _, e := range plan.Download {
		if len(sample) >= 4 {
			break
		}
		if !e.CurseForge {
			sample = append(sample, e)
		}
	}
	rep, err := c.Apply(ctx, res, dir, Plan{Download: sample}, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("downloaded %v, manual %v", rep.Downloaded, rep.Manual)
	if len(rep.Downloaded) != len(sample) {
		t.Fatalf("expected %d downloads", len(sample))
	}
}
