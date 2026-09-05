// Package java finds, or fetches, a Java runtime good enough to run a mod
// loader's installer. The game itself is launched by the official launcher
// with its own runtime; this is only for the install step.
//
// Order of preference: the launcher's bundled runtimes under
// `.minecraft/runtime/` (already on disk, the right vendor), then a `java` on
// PATH that is new enough, then a Temurin JRE downloaded from Adoptium into a
// directory we own.
package java

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// MinMajor is the oldest Java that runs current loader installers.
const MinMajor = 17

// ProvisionMajor is the Java we download when nothing usable is present.
const ProvisionMajor = 21

// Runtime is a usable java executable.
type Runtime struct {
	Path   string
	Source string // "launcher", "path", "downloaded"
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return "java.exe"
	}
	return "java"
}

// greek orders the launcher's runtime components, which are named
// java-runtime-alpha, -beta, -gamma, -delta… in release order. Unknown names
// sort last.
var greek = []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa", "lambda", "mu"}

func componentRank(name string) int {
	for i, g := range greek {
		if strings.HasSuffix(name, "-"+g) {
			return i
		}
	}
	return -1
}

// RuntimeRoots are the directories the launcher keeps its bundled runtimes
// in: `<minecraft>/runtime` for the standalone launcher, and on Windows the
// Store launcher's package cache (verified on the Store build 2.6.2:
// `%LOCALAPPDATA%\Packages\Microsoft.4297127D64EC6_8wekyb3d8bbwe\LocalCache\Local\runtime`).
func RuntimeRoots(minecraftDir string) []string {
	var roots []string
	if minecraftDir != "" {
		roots = append(roots, filepath.Join(minecraftDir, "runtime"))
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			roots = append(roots, filepath.Join(local, "Packages", "Microsoft.4297127D64EC6_8wekyb3d8bbwe", "LocalCache", "Local", "runtime"))
		}
	}
	return roots
}

// FromLauncher looks for the launcher's bundled runtimes in RuntimeRoots:
// `<root>/<component>/<platform>/<component>/bin/java`, plus macOS's
// `…/jre.bundle/Contents/Home/bin/java`. Newest component first, and only one
// that is actually new enough: the launcher also keeps jre-legacy (Java 8)
// around for old versions of the game.
func FromLauncher(ctx context.Context, minecraftDir string) (Runtime, bool) {
	for _, root := range RuntimeRoots(minecraftDir) {
		for _, candidate := range candidatesIn(root) {
			if major, err := Major(ctx, candidate); err == nil && major >= MinMajor {
				return Runtime{Path: candidate, Source: "launcher"}, true
			}
		}
	}
	return Runtime{}, false
}

// candidatesIn lists every java executable under a runtime root, newest
// component first.
func candidatesIn(root string) []string {
	var out []string
	components, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(components))
	for _, c := range components {
		if c.IsDir() {
			names = append(names, c.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool {
		ri, rj := componentRank(names[i]), componentRank(names[j])
		if ri != rj {
			return ri > rj
		}
		return names[i] > names[j]
	})
	for _, comp := range names {
		platforms, err := os.ReadDir(filepath.Join(root, comp))
		if err != nil {
			continue
		}
		for _, p := range platforms {
			if !p.IsDir() {
				continue
			}
			for _, rel := range []string{
				filepath.Join(comp, "bin", exeName()),
				filepath.Join(comp, "jre.bundle", "Contents", "Home", "bin", exeName()),
			} {
				candidate := filepath.Join(root, comp, p.Name(), rel)
				if isExecutable(candidate) {
					out = append(out, candidate)
				}
			}
		}
	}
	return out
}

func isExecutable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

var versionRe = regexp.MustCompile(`version "(\d+)(?:\.(\d+))?`)

// Major runs `java -version` and returns the major version (8 for "1.8").
func Major(ctx context.Context, javaPath string) (int, error) {
	cmd := exec.CommandContext(ctx, javaPath, "-version")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("%s -version: %w", javaPath, err)
	}
	m := versionRe.FindStringSubmatch(out.String())
	if m == nil {
		return 0, fmt.Errorf("%s -version: unrecognised output %q", javaPath, strings.TrimSpace(out.String()))
	}
	major, _ := strconv.Atoi(m[1])
	if major == 1 && m[2] != "" {
		major, _ = strconv.Atoi(m[2])
	}
	return major, nil
}

// FromPath finds a `java` on PATH that is at least MinMajor.
func FromPath(ctx context.Context) (Runtime, bool) {
	p, err := exec.LookPath(exeName())
	if err != nil {
		return Runtime{}, false
	}
	if major, err := Major(ctx, p); err != nil || major < MinMajor {
		return Runtime{}, false
	}
	return Runtime{Path: p, Source: "path"}, true
}

// FromDownloaded finds a runtime Provision put under javaRoot. Temurin's
// macOS archives are app bundles, so the executable sits under
// Contents/Home there.
func FromDownloaded(javaRoot string) (Runtime, bool) {
	base := filepath.Join(javaRoot, fmt.Sprintf("jre-%d", ProvisionMajor))
	for _, p := range []string{
		filepath.Join(base, "bin", exeName()),
		filepath.Join(base, "Contents", "Home", "bin", exeName()),
	} {
		if isExecutable(p) {
			return Runtime{Path: p, Source: "downloaded"}, true
		}
	}
	return Runtime{}, false
}

// AdoptiumURL is the Adoptium API endpoint for the latest GA Temurin JRE of
// major for this OS and architecture. It redirects to the archive.
func AdoptiumURL(major int, goos, goarch string) (string, error) {
	osName := map[string]string{"linux": "linux", "windows": "windows", "darwin": "mac"}[goos]
	arch := map[string]string{"amd64": "x64", "arm64": "aarch64"}[goarch]
	if osName == "" || arch == "" {
		return "", fmt.Errorf("no Temurin build for %s/%s", goos, goarch)
	}
	return fmt.Sprintf("https://api.adoptium.net/v3/binary/latest/%d/ga/%s/%s/jre/hotspot/normal/eclipse", major, osName, arch), nil
}

// Provision downloads a Temurin JRE into javaRoot/jre-<major> and returns it.
// The archive's single top-level directory is stripped.
func Provision(ctx context.Context, client *http.Client, userAgent, javaRoot string, progress func(string)) (Runtime, error) {
	url, err := AdoptiumURL(ProvisionMajor, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Runtime{}, err
	}
	return provisionFrom(ctx, client, userAgent, url, javaRoot, progress)
}

func provisionFrom(ctx context.Context, client *http.Client, userAgent, url, javaRoot string, progress func(string)) (Runtime, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if progress != nil {
		progress(fmt.Sprintf("Downloading Java %d", ProvisionMajor))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Runtime{}, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Runtime{}, fmt.Errorf("download Java: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return Runtime{}, fmt.Errorf("download Java: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Runtime{}, fmt.Errorf("download Java: %w", err)
	}
	dest := filepath.Join(javaRoot, fmt.Sprintf("jre-%d", ProvisionMajor))
	tmp := dest + ".tmp"
	_ = os.RemoveAll(tmp)
	if progress != nil {
		progress("Unpacking Java")
	}
	isZip := strings.HasSuffix(strings.ToLower(resp.Request.URL.Path), ".zip") || bytes.HasPrefix(data, []byte("PK"))
	if isZip {
		err = extractZip(data, tmp)
	} else {
		err = extractTarGz(data, tmp)
	}
	if err != nil {
		_ = os.RemoveAll(tmp)
		return Runtime{}, fmt.Errorf("unpack Java: %w", err)
	}
	_ = os.RemoveAll(dest)
	if err := os.Rename(tmp, dest); err != nil {
		return Runtime{}, err
	}
	rt, ok := FromDownloaded(javaRoot)
	if !ok {
		return Runtime{}, errors.New("unpacked Java has no bin/java")
	}
	return rt, nil
}

// stripTop drops the archive's single top-level directory from a path and
// refuses anything that would escape dest.
func stripTop(name string) (string, bool) {
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	parts := strings.SplitN(name, "/", 2)
	if len(parts) < 2 || parts[1] == "" {
		return "", false
	}
	rel := parts[1]
	if strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
		return "", false
	}
	return filepath.FromSlash(rel), true
}

func extractTarGz(data []byte, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel, ok := stripTop(h.Name)
		if !ok {
			continue
		}
		target := filepath.Join(dest, rel)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode)|0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			if strings.Contains(h.Linkname, "..") {
				continue
			}
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
			_ = os.Symlink(h.Linkname, target)
		}
	}
}

func extractZip(data []byte, dest string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		rel, ok := stripTop(f.Name)
		if !ok {
			continue
		}
		target := filepath.Join(dest, rel)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0o600)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// Ensure returns a usable runtime, downloading one only as a last resort.
func Ensure(ctx context.Context, client *http.Client, userAgent, minecraftDir, javaRoot string, progress func(string)) (Runtime, error) {
	if rt, ok := FromLauncher(ctx, minecraftDir); ok {
		return rt, nil
	}
	if rt, ok := FromDownloaded(javaRoot); ok {
		return rt, nil
	}
	if rt, ok := FromPath(ctx); ok {
		return rt, nil
	}
	return Provision(ctx, client, userAgent, javaRoot, progress)
}
