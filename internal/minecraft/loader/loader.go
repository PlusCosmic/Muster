// Package loader installs a mod loader into the official launcher, i.e.
// creates `.minecraft/versions/<id>/<id>.json` so a profile can point at it.
//
// Fabric and Quilt publish a ready-made launcher profile JSON from their meta
// servers; writing it is the whole install (the launcher fetches the vanilla
// parent and the libraries it lists on first launch). NeoForge and Forge ship
// an installer jar that patches the client and must be run with Java:
// `java -jar <installer> --install-client <.minecraft>`. Verified against
// neoforge-21.1.248: it downloads vanilla itself, needs launcher_profiles.json
// to exist, takes about nine seconds, and injects a "NeoForge" profile that we
// remove again so the launcher's dropdown only shows the pack.
package loader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"muster/internal/minecraft/launcher"
)

// Runner runs a Java program: `java -jar jar args...` with cwd, capturing
// output into logPath. Injectable for tests.
type Runner func(ctx context.Context, java, jar string, args []string, cwd, logPath string) error

// Installer knows how to install loaders. The zero value is usable.
type Installer struct {
	HTTP      *http.Client
	UserAgent string
	Run       Runner
}

func (i *Installer) http() *http.Client {
	if i.HTTP != nil {
		return i.HTTP
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (i *Installer) run() Runner {
	if i.Run != nil {
		return i.Run
	}
	return execRunner
}

// execRunner is the real Runner.
func execRunner(ctx context.Context, java, jar string, args []string, cwd, logPath string) error {
	cmd := exec.CommandContext(ctx, java, append([]string{"-jar", jar}, args...)...)
	cmd.Dir = cwd
	logf, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd.Stdout, cmd.Stderr = logf, logf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w (see %s)", err, logPath)
	}
	return nil
}

func (i *Installer) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if i.UserAgent != "" {
		req.Header.Set("User-Agent", i.UserAgent)
	}
	resp, err := i.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// Meta endpoints and installer Maven coordinates. Vars so tests can point
// them at a local server.
var (
	FabricMeta  = "https://meta.fabricmc.net/v2/versions/loader/%s/%s/profile/json"
	QuiltMeta   = "https://meta.quiltmc.org/v3/versions/loader/%s/%s/profile/json"
	NeoForgeJar = "https://maven.neoforged.net/releases/net/neoforged/neoforge/%[1]s/neoforge-%[1]s-installer.jar"
	ForgeJar    = "https://maven.minecraftforge.net/net/minecraftforge/forge/%[1]s-%[2]s/forge-%[1]s-%[2]s-installer.jar"
)

// NeedsJava reports whether installing this loader runs an installer jar.
func NeedsJava(loader string) bool { return loader == "neoforge" || loader == "forge" }

// Ensure makes sure the launcher in minecraftDir has the installation for the
// loader, returning its version id. `java` is only called for loaders that
// need it. workDir receives installer downloads and logs. progress gets a
// short line per step.
func (i *Installer) Ensure(ctx context.Context, minecraftDir, minecraft, loader, loaderVersion, workDir string, java func() (string, error), progress func(string)) (string, error) {
	id, err := launcher.VersionID(minecraft, loader, loaderVersion)
	if err != nil {
		return "", err
	}
	if launcher.HasVersion(minecraftDir, id) {
		return id, nil
	}
	say := func(s string) {
		if progress != nil {
			progress(s)
		}
	}
	switch loader {
	case "":
		// Vanilla: the launcher installs versions itself when asked to play one.
		return id, nil
	case "fabric", "quilt":
		say("Installing " + loader + " " + loaderVersion)
		meta := FabricMeta
		if loader == "quilt" {
			meta = QuiltMeta
		}
		raw, err := i.get(ctx, fmt.Sprintf(meta, minecraft, loaderVersion))
		if err != nil {
			return "", fmt.Errorf("fetch %s profile: %w", loader, err)
		}
		if err := writeVersion(minecraftDir, id, raw); err != nil {
			return "", err
		}
	case "neoforge", "forge":
		say("Installing " + loader + " " + loaderVersion)
		url := fmt.Sprintf(NeoForgeJar, loaderVersion)
		if loader == "forge" {
			url = fmt.Sprintf(ForgeJar, minecraft, loaderVersion)
		}
		if err := i.runInstaller(ctx, minecraftDir, url, workDir, java, say); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported loader %q", loader)
	}
	if !launcher.HasVersion(minecraftDir, id) {
		return "", fmt.Errorf("%s installed but the launcher has no versions/%s — the id scheme may have changed", loader, id)
	}
	return id, nil
}

// writeVersion installs a launcher version JSON, after checking it is JSON
// whose id matches the directory it goes in.
func writeVersion(minecraftDir, id string, raw []byte) error {
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("version json for %s: %w", id, err)
	}
	if v.ID != id {
		return fmt.Errorf("meta server returned version %q, expected %q", v.ID, id)
	}
	dir := filepath.Join(minecraftDir, "versions", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, id+".json.tmp")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, id+".json"))
}

func (i *Installer) runInstaller(ctx context.Context, minecraftDir, url, workDir string, java func() (string, error), say func(string)) error {
	javaPath, err := java()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	say("Downloading installer")
	raw, err := i.get(ctx, url)
	if err != nil {
		return fmt.Errorf("download installer: %w", err)
	}
	jar := filepath.Join(workDir, filepath.Base(url))
	if err := os.WriteFile(jar, raw, 0o644); err != nil {
		return err
	}
	defer os.Remove(jar)

	// The installer refuses a directory without launcher_profiles.json, and
	// writes a profile of its own into it that we do not want.
	profiles := filepath.Join(minecraftDir, launcher.ProfilesFile)
	if _, err := os.Stat(profiles); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(minecraftDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(profiles, []byte("{\n  \"profiles\": {},\n  \"version\": 3\n}\n"), 0o644); err != nil {
			return err
		}
	}
	// Snapshot every profiles file separately: the installer may touch either,
	// and an entry that already existed in one must not be removed from it
	// because it was new in the other.
	files := launcher.ProfilesFiles(minecraftDir)
	before := map[string]map[string]bool{}
	for _, name := range files {
		keys, err := profileKeys(filepath.Join(minecraftDir, name))
		if err != nil {
			return err
		}
		before[name] = keys
	}

	say("Running installer")
	logPath := filepath.Join(workDir, strings.TrimSuffix(filepath.Base(url), ".jar")+".log")
	if err := i.run()(ctx, javaPath, jar, []string{"--install-client", minecraftDir}, workDir, logPath); err != nil {
		return fmt.Errorf("installer failed: %w", err)
	}
	// Also drop the installer's own log if it left one beside the jar.
	_ = os.Remove(jar + ".log")

	for _, name := range files {
		after, err := profileKeys(filepath.Join(minecraftDir, name))
		if err != nil {
			return err
		}
		var injected []string
		for k := range after {
			if !before[name][k] {
				injected = append(injected, k)
			}
		}
		if err := launcher.RemoveKeysFrom(minecraftDir, name, injected); err != nil {
			return err
		}
	}
	return nil
}

// profileKeys lists the profile keys in a profiles file; a missing file has none.
func profileKeys(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Profiles map[string]json.RawMessage `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	keys := map[string]bool{}
	for k := range doc.Profiles {
		keys[k] = true
	}
	return keys, nil
}
