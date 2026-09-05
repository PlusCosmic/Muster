package minecraft

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"muster/internal/appdir"
	"muster/internal/minecraft/java"
	"muster/internal/minecraft/launcher"
	"muster/internal/minecraft/loader"
	"muster/internal/minecraft/manifest"
	"muster/internal/minecraft/models"
	"muster/internal/minecraft/packwiz"
	core "muster/internal/models"
	"muster/internal/version"
)

// SyncEvent is the Wails event name SyncPack emits models.SyncProgress on.
const SyncEvent = "minecraft:sync"

// Service is the Minecraft service bound to the frontend. Every method here is
// a command in docs/ARCHITECTURE.md.
type Service struct {
	// HTTP is used for the manifest and pack downloads; nil means a default.
	HTTP *http.Client
	// Emit publishes a frontend event (main.go wires the Wails emitter in).
	// nil drops events, which keeps this package free of Wails for tests and
	// for cross-platform vetting.
	Emit func(name string, data any)
	// Installer installs loaders; nil means a default. Tests inject a fake Run.
	Installer *loader.Installer
	// findJava locates a Java for loader installers; nil means java.Ensure.
	findJava func(ctx context.Context, minecraftDir string, progress func(string)) (string, error)
}

func (s *Service) installer() *loader.Installer {
	if s.Installer != nil {
		if s.Installer.HTTP == nil {
			s.Installer.HTTP = s.client()
		}
		if s.Installer.UserAgent == "" {
			s.Installer.UserAgent = userAgent()
		}
		return s.Installer
	}
	return &loader.Installer{HTTP: s.client(), UserAgent: userAgent()}
}

func (s *Service) javaFor(ctx context.Context, minecraftDir string, progress func(string)) (string, error) {
	if s.findJava != nil {
		return s.findJava(ctx, minecraftDir, progress)
	}
	rt, err := java.Ensure(ctx, s.client(), userAgent(), minecraftDir, JavaRoot(), progress)
	if err != nil {
		return "", fmt.Errorf("no Java to run the loader installer with: %w", err)
	}
	return rt.Path, nil
}

func (s *Service) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (s *Service) packwiz() *packwiz.Client {
	return &packwiz.Client{HTTP: s.client(), UserAgent: userAgent()}
}

func userAgent() string { return "Muster/" + version.Version }

func (s *Service) publish(name string, data any) {
	if s.Emit != nil {
		s.Emit(name, data)
	}
}

func (s *Service) GetSettings() (models.Settings, error) { return loadSettings(), nil }

func (s *Service) UpdateSettings(v models.Settings) (models.Settings, error) {
	v = normalizeSettings(v)
	if err := saveSettings(v); err != nil {
		return models.Settings{}, err
	}
	return v, nil
}

// Detect reports the effective manifest URL and launcher location.
func (s *Service) Detect() (models.Detected, error) {
	st := loadSettings()
	dir := minecraftDir(st)
	return models.Detected{
		ManifestURL:       core.Str(manifestURL(st)),
		MinecraftDir:      core.Str(dir),
		LauncherInstalled: launcher.Installed(dir),
		PacksDir:          PacksRoot(),
	}, nil
}

// ErrNoManifest is returned when no manifest URL is configured.
var ErrNoManifest = errors.New("no pack list configured — set a manifest URL in Settings")

func (s *Service) fetchManifest(ctx context.Context) (manifest.Manifest, error) {
	url := manifestURL(loadSettings())
	if url == "" {
		return manifest.Manifest{}, ErrNoManifest
	}
	return manifest.Fetch(ctx, s.client(), userAgent(), url)
}

// ListPacks fetches the manifest and pairs each entry with its local state.
func (s *Service) ListPacks() ([]models.Pack, error) {
	m, err := s.fetchManifest(context.Background())
	if err != nil {
		return nil, err
	}
	dir := minecraftDir(loadSettings())
	out := make([]models.Pack, 0, len(m.Packs))
	for _, p := range m.Packs {
		out = append(out, s.describe(p, dir))
	}
	return out, nil
}

func (s *Service) describe(p manifest.Pack, minecraftDir string) models.Pack {
	installDir := PackDir(p.ID)
	out := models.Pack{
		ID: p.ID, Name: p.Name, Description: p.Description, Icon: core.Str(p.Icon),
		PackURL: p.PackURL, Server: core.Str(p.Server),
		MinMemoryMb: p.Java.MinMemoryMb, MaxMemoryMb: p.Java.MaxMemoryMb, JavaArgs: core.NonNil(p.Java.Args),
		InstallDir: installDir,
	}
	if st, err := packwiz.LoadState(installDir); err == nil && st.PackVersion != "" {
		out.Installed = true
		out.InstalledVersion = core.Str(st.PackVersion)
		if st.SyncedAtMs != 0 {
			out.SyncedAtMs = core.Int64(st.SyncedAtMs)
		}
	}
	if minecraftDir != "" {
		_, ok, _ := launcher.Get(minecraftDir, p.ID)
		out.ProfileWritten = ok
	}
	return out
}

func (s *Service) findPack(ctx context.Context, id string) (manifest.Pack, error) {
	m, err := s.fetchManifest(ctx)
	if err != nil {
		return manifest.Pack{}, err
	}
	for _, p := range m.Packs {
		if p.ID == id {
			return p, nil
		}
	}
	return manifest.Pack{}, fmt.Errorf("pack %q is not in the manifest", id)
}

// CheckPack loads the pack and reports what a sync would do, without doing it.
func (s *Service) CheckPack(id string) (models.PackCheck, error) {
	ctx := context.Background()
	p, err := s.findPack(ctx, id)
	if err != nil {
		return models.PackCheck{}, err
	}
	res, err := s.packwiz().Load(ctx, p.PackURL)
	if err != nil {
		return models.PackCheck{}, err
	}
	dir := PackDir(id)
	state, err := packwiz.LoadState(dir)
	if err != nil {
		return models.PackCheck{}, err
	}
	plan := packwiz.MakePlan(res, dir, state, nil)
	loader, loaderVersion := res.Pack.Loader()
	versionID, err := launcher.VersionID(res.Pack.Versions["minecraft"], loader, loaderVersion)
	if err != nil {
		return models.PackCheck{}, err
	}
	return models.PackCheck{
		ID: id, LatestVersion: res.Pack.Version,
		Minecraft: res.Pack.Versions["minecraft"], Loader: loader, LoaderVersion: loaderVersion,
		VersionID:       versionID,
		LoaderInstalled: launcher.HasVersion(minecraftDir(loadSettings()), versionID),
		ToDownload:      len(plan.Download), ToDelete: len(plan.Delete),
		UpToDate: plan.Empty() && state.PackVersion == res.Pack.Version,
	}, nil
}

// SyncPack brings the pack's install directory up to date, makes sure the
// launcher has the pack's loader, and writes (or refreshes) its launcher
// profile. Progress goes out on SyncEvent in three phases.
func (s *Service) SyncPack(id string) (models.SyncReport, error) {
	ctx := context.Background()
	p, err := s.findPack(ctx, id)
	if err != nil {
		return models.SyncReport{}, err
	}
	pw := s.packwiz()
	res, err := pw.Load(ctx, p.PackURL)
	if err != nil {
		return models.SyncReport{}, err
	}
	dir := PackDir(id)
	if err := appdir.EnsureDir(dir); err != nil {
		return models.SyncReport{}, err
	}
	state, err := packwiz.LoadState(dir)
	if err != nil {
		return models.SyncReport{}, err
	}
	plan := packwiz.MakePlan(res, dir, state, nil)
	rep, err := pw.Apply(ctx, res, dir, plan, p.PackURL, func(done, total int, e packwiz.Entry) {
		s.publish(SyncEvent, models.SyncProgress{ID: id, Phase: "files", Done: done, Total: total, Current: e.Name})
	})
	out := models.SyncReport{
		ID: id, Version: res.Pack.Version,
		Downloaded: core.NonNil(rep.Downloaded), Deleted: core.NonNil(rep.Deleted), Manual: manuals(rep.Manual),
	}
	if err != nil {
		return out, err
	}

	mcDir := minecraftDir(loadSettings())
	if mcDir == "" {
		return out, errors.New("the Minecraft launcher's folder is unknown — set it in Settings")
	}
	if err := appdir.EnsureDir(mcDir); err != nil {
		return out, err
	}

	loaderName, loaderVersion := res.Pack.Loader()
	step := func(msg string) {
		s.publish(SyncEvent, models.SyncProgress{ID: id, Phase: "loader", Current: msg})
	}
	versionID, err := s.installer().Ensure(ctx, mcDir, res.Pack.Versions["minecraft"], loaderName, loaderVersion, WorkDir(),
		func() (string, error) { return s.javaFor(ctx, mcDir, step) }, step)
	if err != nil {
		return out, fmt.Errorf("could not install %s: %w", loaderName, err)
	}
	out.VersionID = versionID
	out.LoaderInstalled = true

	s.publish(SyncEvent, models.SyncProgress{ID: id, Phase: "profile", Current: p.Name})
	err = launcher.Upsert(mcDir, id, launcher.Profile{
		Name:          p.Name,
		Icon:          profileIcon(p),
		LastVersionID: versionID,
		GameDir:       dir,
		JavaArgs:      javaArgs(p.Java),
	})
	if err != nil {
		return out, fmt.Errorf("could not write the launcher profile: %w", err)
	}
	out.ProfileWritten = true
	// The launcher reads its profiles on start, so one written while it is
	// open only shows after a restart. Nothing is lost; the UI says so.
	out.LauncherOpen = launcher.Running()
	return out, nil
}

// OpenLauncher starts the official Minecraft launcher.
func (s *Service) OpenLauncher() error { return launcher.Open() }

// LauncherRunning reports whether the official launcher is already open.
// Opening it again then does nothing, and a profile written since it started
// is only picked up after it is closed and reopened.
func (s *Service) LauncherRunning() (bool, error) { return launcher.Running(), nil }

func manuals(in []packwiz.Manual) []models.Manual {
	out := make([]models.Manual, 0, len(in))
	for _, m := range in {
		out = append(out, models.Manual{Path: m.Path, Name: m.Name, URL: m.URL, Why: m.Why})
	}
	return out
}

// javaArgs renders the manifest's Java configuration as the launcher's single
// javaArgs string. The launcher's own default is `-Xmx2G` plus GC tuning, so
// a pack without memory settings gets nothing here and keeps that default.
func javaArgs(j manifest.Java) string {
	var parts []string
	if j.MinMemoryMb > 0 {
		parts = append(parts, fmt.Sprintf("-Xms%dM", j.MinMemoryMb))
	}
	if j.MaxMemoryMb > 0 {
		parts = append(parts, fmt.Sprintf("-Xmx%dM", j.MaxMemoryMb))
	}
	parts = append(parts, j.Args...)
	return strings.Join(parts, " ")
}

// profileIcon picks a launcher icon. The launcher accepts its built-in block
// names or a data: URI; icon fetching and embedding is a later step, so packs
// get a recognisable block for now.
func profileIcon(manifest.Pack) string { return "Furnace" }
