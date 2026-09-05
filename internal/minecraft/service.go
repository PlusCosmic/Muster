package minecraft

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"muster/internal/appdir"
	"muster/internal/minecraft/java"
	"muster/internal/minecraft/launcher"
	"muster/internal/minecraft/loader"
	"muster/internal/minecraft/machine"
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
	// TotalMemoryMb overrides machine memory detection; nil means the real value.
	TotalMemoryMb func() int
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

// Detect reports the effective manifest URL, launcher location and memory.
func (s *Service) Detect() (models.Detected, error) {
	st := loadSettings()
	dir := minecraftDir(st)
	total := s.totalMemory()
	return models.Detected{
		ManifestURL:       core.Str(manifestURL(st)),
		MinecraftDir:      core.Str(dir),
		LauncherInstalled: launcher.Installed(dir),
		PacksDir:          PacksRoot(),
		TotalMemoryMb:     total,
		MaxHeapMb:         machine.MaxHeapMb(total),
	}, nil
}

func (s *Service) totalMemory() int {
	if s.TotalMemoryMb != nil {
		return s.TotalMemoryMb()
	}
	return machine.TotalMemoryMb()
}

// launchFor is the effective launch settings for a manifest pack.
func (s *Service) launchFor(p manifest.Pack, st models.Settings) (models.LaunchSettings, bool) {
	saved, ok := st.Packs[p.ID]
	var sp *models.LaunchSettings
	if ok {
		sp = &saved
	}
	return effectiveLaunch(p.Recommended, sp, machine.MaxHeapMb(s.totalMemory())), ok
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
	st := loadSettings()
	dir := minecraftDir(st)
	out := make([]models.Pack, 0, len(m.Packs))
	for _, p := range m.Packs {
		out = append(out, s.describe(p, dir, st))
	}
	return out, nil
}

func (s *Service) describe(p manifest.Pack, minecraftDir string, st models.Settings) models.Pack {
	installDir := PackDir(p.ID)
	launch, customised := s.launchFor(p, st)
	out := models.Pack{
		ID: p.ID, Name: p.Name, Description: p.Description, Icon: core.Str(p.Icon),
		PackURL: p.PackURL, Server: core.Str(p.Server),
		RecommendedMinMemoryMb: p.Recommended.MinMemoryMb, RecommendedMaxMemoryMb: p.Recommended.MaxMemoryMb,
		RecommendedArgs: core.NonNil(p.Recommended.Args),
		Launch:          launch, LaunchCustomised: customised,
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
	// Everything that can be rejected is rejected before a byte moves: a pack
	// whose loader the launcher cannot represent must not be half-applied.
	mcDir := minecraftDir(loadSettings())
	if mcDir == "" {
		return models.SyncReport{}, errors.New("the Minecraft launcher's folder is unknown — set it in Settings")
	}
	loaderName, loaderVersion := res.Pack.Loader()
	if _, err := launcher.VersionID(res.Pack.Versions["minecraft"], loaderName, loaderVersion); err != nil {
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

	if err := appdir.EnsureDir(mcDir); err != nil {
		return out, err
	}

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
	launch, _ := s.launchFor(p, loadSettings())
	if err := s.writeProfile(mcDir, p, versionID, dir, launch); err != nil {
		return out, err
	}
	out.ProfileWritten = true
	// The launcher reads its profiles on start, so one written while it is
	// open only shows after a restart. Nothing is lost; the UI says so.
	out.LauncherOpen = launcher.Running()
	return out, nil
}

func (s *Service) writeProfile(mcDir string, p manifest.Pack, versionID, gameDir string, launch models.LaunchSettings) error {
	err := launcher.Upsert(mcDir, p.ID, launcher.Profile{
		Name:          p.Name,
		Icon:          profileIcon(p),
		LastVersionID: versionID,
		GameDir:       gameDir,
		JavaArgs:      javaArgs(launch),
	})
	if err != nil {
		return fmt.Errorf("could not write the launcher profile: %w", err)
	}
	return nil
}

// GetLaunchSettings returns what the pack launches with on this machine.
func (s *Service) GetLaunchSettings(id string) (models.LaunchSettings, error) {
	p, err := s.findPack(context.Background(), id)
	if err != nil {
		return models.LaunchSettings{}, err
	}
	launch, _ := s.launchFor(p, loadSettings())
	return launch, nil
}

// SetLaunchSettings saves the user's launch settings for a pack and, when the
// pack already has a launcher profile, rewrites that profile's javaArgs so the
// change applies on the next launch without a sync. Returns the settings as
// stored (clamped to this machine).
func (s *Service) SetLaunchSettings(id string, ls models.LaunchSettings) (models.LaunchSettings, error) {
	p, err := s.findPack(context.Background(), id)
	if err != nil {
		return models.LaunchSettings{}, err
	}
	fitted := effectiveLaunch(p.Recommended, &ls, machine.MaxHeapMb(s.totalMemory()))
	if err := validateLaunch(fitted); err != nil {
		return models.LaunchSettings{}, err
	}
	st := loadSettings()
	stored := fitted
	if stored.FollowRecommendedArgs {
		stored.Args = []string{} // derived at read time while following
	}
	st.Packs[id] = stored
	if err := saveSettings(st); err != nil {
		return models.LaunchSettings{}, err
	}
	mcDir := minecraftDir(st)
	if mcDir != "" {
		if prof, ok, _ := launcher.Get(mcDir, id); ok {
			if err := s.writeProfile(mcDir, p, prof.LastVersionID, prof.GameDir, fitted); err != nil {
				return models.LaunchSettings{}, err
			}
		}
	}
	return fitted, nil
}

// ResetLaunchSettings forgets the user's launch settings for a pack, going
// back to the recommendation fitted to this machine, and rewrites the profile.
func (s *Service) ResetLaunchSettings(id string) (models.LaunchSettings, error) {
	p, err := s.findPack(context.Background(), id)
	if err != nil {
		return models.LaunchSettings{}, err
	}
	st := loadSettings()
	delete(st.Packs, id)
	if err := saveSettings(st); err != nil {
		return models.LaunchSettings{}, err
	}
	launch, _ := s.launchFor(p, st)
	mcDir := minecraftDir(st)
	if mcDir != "" {
		if prof, ok, _ := launcher.Get(mcDir, id); ok {
			if err := s.writeProfile(mcDir, p, prof.LastVersionID, prof.GameDir, launch); err != nil {
				return models.LaunchSettings{}, err
			}
		}
	}
	return launch, nil
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

// profileIcon picks a launcher icon. The launcher accepts its built-in block
// names or a data: URI; icon fetching and embedding is a later step, so packs
// get a recognisable block for now.
func profileIcon(manifest.Pack) string { return "Furnace" }
