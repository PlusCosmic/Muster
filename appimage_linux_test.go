//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageBesideAppImage(t *testing.T) {
	dir := t.TempDir()
	appImage := filepath.Join(dir, "Muster-linux-x86_64.AppImage")
	if err := os.WriteFile(appImage, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The updater's download is created without the executable bit.
	staged := filepath.Join(t.TempDir(), "Muster-linux-x86_64.AppImage")
	if err := os.WriteFile(staged, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := stageBesideAppImage(staged, appImage)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(filepath.Dir(got)) != dir {
		t.Errorf("staged at %s, want a directory directly under %s", got, dir)
	}
	if !strings.HasPrefix(filepath.Base(filepath.Dir(got)), "wails-update-") {
		t.Errorf("staging directory %s must be named wails-update-* so the helper removes it", filepath.Dir(got))
	}
	if filepath.Base(got) != filepath.Base(appImage) {
		t.Errorf("staged file is %s, want the AppImage's name", filepath.Base(got))
	}
	b, err := os.ReadFile(got)
	if err != nil || string(b) != "new" {
		t.Errorf("staged content = %q, %v", b, err)
	}
	if fi, _ := os.Stat(got); fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("staged copy is not executable: %v", fi.Mode())
	}
}

func TestStageBesideAppImageUnwritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	ro := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(t.TempDir(), "x.AppImage")
	if err := os.WriteFile(staged, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := stageBesideAppImage(staged, filepath.Join(ro, "x.AppImage")); err == nil {
		t.Error("expected an error for an unwritable directory")
	}
}

func TestAppImagePath(t *testing.T) {
	t.Setenv("APPIMAGE", "")
	if got := appImagePath(); got != "" {
		t.Errorf("unset: got %q", got)
	}
	t.Setenv("APPIMAGE", "relative.AppImage")
	if got := appImagePath(); got != "" {
		t.Errorf("relative: got %q", got)
	}
	t.Setenv("APPIMAGE", filepath.Join(t.TempDir(), "missing.AppImage"))
	if got := appImagePath(); got != "" {
		t.Errorf("missing: got %q", got)
	}
	p := filepath.Join(t.TempDir(), "Muster.AppImage")
	if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPIMAGE", p)
	if got := appImagePath(); got != p {
		t.Errorf("got %q, want %q", got, p)
	}
}

func TestLaunchAppImageHelperRefusesForeignStaging(t *testing.T) {
	if err := launchAppImageHelper("/nonexistent/Muster.AppImage", "/tmp/elsewhere/Muster.AppImage"); err == nil {
		t.Error("expected a refusal when the staged file is not beside the AppImage")
	}
}
