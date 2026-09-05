package java

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFromLauncherPrefersNewestComponent(t *testing.T) {
	mc := t.TempDir()
	exe := exeName()
	touch(t, filepath.Join(mc, "runtime", "java-runtime-gamma", "linux", "java-runtime-gamma", "bin", exe))
	touch(t, filepath.Join(mc, "runtime", "java-runtime-delta", "linux", "java-runtime-delta", "bin", exe))
	touch(t, filepath.Join(mc, "runtime", "java-runtime-delta", "mac-os", "java-runtime-delta", "jre.bundle", "Contents", "Home", "bin", exe))
	rt, ok := FromLauncher(mc)
	if !ok || rt.Source != "launcher" || !filepath.IsAbs(rt.Path) {
		t.Fatalf("%+v %v", rt, ok)
	}
	if filepath.Base(filepath.Dir(filepath.Dir(rt.Path))) != "java-runtime-delta" && !bytes.Contains([]byte(rt.Path), []byte("java-runtime-delta")) {
		t.Fatalf("expected delta, got %s", rt.Path)
	}
	if _, ok := FromLauncher(t.TempDir()); ok {
		t.Fatal("empty dir should have no runtime")
	}
}

func TestFromRuntimeRootMatchesStoreLayout(t *testing.T) {
	root := t.TempDir()
	exe := exeName()
	touch(t, filepath.Join(root, "java-runtime-epsilon", "windows-x64", "java-runtime-epsilon", "bin", exe))
	rt, ok := fromRuntimeRoot(root)
	if !ok || !bytes.Contains([]byte(rt.Path), []byte("epsilon")) {
		t.Fatalf("%+v %v", rt, ok)
	}
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", root)
		if len(RuntimeRoots("")) != 1 {
			t.Fatal("store root expected")
		}
	}
}

func TestAdoptiumURL(t *testing.T) {
	u, err := AdoptiumURL(21, "windows", "amd64")
	if err != nil || u != "https://api.adoptium.net/v3/binary/latest/21/ga/windows/x64/jre/hotspot/normal/eclipse" {
		t.Fatalf("%s %v", u, err)
	}
	if u, _ := AdoptiumURL(21, "darwin", "arm64"); u != "https://api.adoptium.net/v3/binary/latest/21/ga/mac/aarch64/jre/hotspot/normal/eclipse" {
		t.Fatalf("%s", u)
	}
	if _, err := AdoptiumURL(21, "plan9", "amd64"); err == nil {
		t.Fatal("expected error")
	}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	zw.Close()
	return buf.Bytes()
}

func TestProvisionExtractsAndStripsTopDir(t *testing.T) {
	exe := exeName()
	archives := map[string][]byte{
		"/jre.tar.gz": tarGz(t, map[string]string{"jdk-21.0.1+1-jre/bin/" + exe: "java", "jdk-21.0.1+1-jre/release": "x", "jdk-21.0.1+1-jre/../evil": "no"}),
		"/jre.zip":    zipBytes(t, map[string]string{"jdk-21.0.1+1-jre/bin/" + exe: "java", "jdk-21.0.1+1-jre/lib/a": "x"}),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest" {
			http.Redirect(w, r, "/jre"+map[string]string{"windows": ".zip"}[r.URL.Query().Get("os")]+map[bool]string{true: "", false: ".tar.gz"}[r.URL.Query().Get("os") == "windows"], http.StatusTemporaryRedirect)
			return
		}
		if b, ok := archives[r.URL.Path]; ok {
			_, _ = w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	for _, os_ := range []string{"linux", "windows"} {
		root := t.TempDir()
		var msgs []string
		rt, err := provisionFrom(context.Background(), srv.Client(), "t", srv.URL+"/latest?os="+os_, root, func(s string) { msgs = append(msgs, s) })
		if err != nil {
			t.Fatalf("%s: %v", os_, err)
		}
		if rt.Source != "downloaded" || rt.Path != filepath.Join(root, "jre-21", "bin", exe) {
			t.Fatalf("%+v", rt)
		}
		if _, err := os.Stat(filepath.Join(root, "evil")); !os.IsNotExist(err) {
			t.Fatal("path escape must be dropped")
		}
		if len(msgs) != 2 {
			t.Fatalf("progress %v", msgs)
		}
		if rt2, ok := FromDownloaded(root); !ok || rt2.Path != rt.Path {
			t.Fatalf("FromDownloaded: %+v %v", rt2, ok)
		}
	}
}

func TestMajorParsesRealJavaIfPresent(t *testing.T) {
	rt, ok := FromPath(context.Background())
	if !ok {
		t.Skip("no java >= 17 on PATH")
	}
	major, err := Major(context.Background(), rt.Path)
	if err != nil || major < MinMajor {
		t.Fatalf("%d %v", major, err)
	}
	if runtime.GOOS == "" {
		t.Fatal("unreachable")
	}
}
