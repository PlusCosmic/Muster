package minecraft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"muster/internal/minecraft/manifest"
	"muster/internal/minecraft/models"
	"muster/internal/minecraft/modrinth"
	"muster/internal/minecraft/registry"
	core "muster/internal/models"
)

// ErrNoPacks is returned when the user has neither entered a pack code nor set
// a pack list URL.
var ErrNoPacks = errors.New("no packs yet — enter a pack code or paste a Modrinth link, or set a pack list URL in Settings")

// source says where a pack came from.
type source struct {
	pack manifest.Pack
	kind string // "code" | "modrinth" | "manifest"
	code string
	// modrinth is set when kind is "modrinth".
	modrinth *models.ModrinthPack
}

func (s *Service) registry(st models.Settings) *registry.Client {
	return &registry.Client{BaseURL: registryURL(st), HTTP: s.client(), UserAgent: userAgent()}
}

func (s *Service) modrinth() *modrinth.Client {
	return &modrinth.Client{BaseURL: s.ModrinthURL, HTTP: s.client(), UserAgent: userAgent()}
}

// modrinthPackID names a Modrinth pack: "modrinth-" plus the slug reduced to
// what a pack id may contain.
func modrinthPackID(slug string) string {
	var b strings.Builder
	b.WriteString("modrinth-")
	lastDash := true
	for _, r := range strings.ToLower(slug) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	id := strings.TrimRight(b.String(), "-")
	if len(id) > 64 {
		id = strings.TrimRight(id[:64], "-")
	}
	return id
}

// modrinthEntry is the manifest-shaped description of a Modrinth project:
// what the pack list shows, with the page as the pack URL. Nothing to
// recommend: Modrinth versions carry no memory advice.
func modrinthEntry(slug string, p modrinth.Project) manifest.Pack {
	return manifest.Pack{
		ID: modrinthPackID(slug), Name: p.Title, Description: p.Description, Icon: p.IconURL,
		PackURL: p.PageURL(), Recommended: manifest.Recommended{Args: []string{}},
	}
}

// refreshTimeout bounds how long ListPacks waits on the registry before
// falling back to what each code resolved to last time.
const refreshTimeout = 8 * time.Second

// sources is every pack the user can see: codes first (each re-resolved
// against the registry, falling back to the cached copy), then Modrinth packs
// (likewise re-resolved against Modrinth), then the manifest's packs. A pack
// whose id collides with an earlier one's is dropped. With no codes, no
// Modrinth packs and no manifest URL it returns ErrNoPacks. Settings are
// updated with fresh registrations as a side effect.
func (s *Service) sources(ctx context.Context) ([]source, error) {
	st := loadSettings()
	if len(st.Codes) == 0 && len(st.Modrinth) == 0 && manifestURL(st) == "" {
		return nil, ErrNoPacks
	}
	var out []source
	seen := map[string]bool{}
	changed := false

	if len(st.Codes) > 0 {
		reg := s.registry(st)
		rctx, cancel := context.WithTimeout(ctx, refreshTimeout)
		defer cancel()
		fresh := make([]*registry.Registration, len(st.Codes))
		var wg sync.WaitGroup
		for i, c := range st.Codes {
			wg.Add(1)
			go func(i int, code string) {
				defer wg.Done()
				if r, err := reg.Resolve(rctx, code); err == nil {
					fresh[i] = &r
				}
			}(i, c.Code)
		}
		wg.Wait()
		for i, c := range st.Codes {
			var p manifest.Pack
			if fresh[i] != nil {
				p = fresh[i].Pack
				if raw, err := json.Marshal(p); err == nil && string(raw) != string(c.Pack) {
					st.Codes[i].Pack = raw
					changed = true
				}
			} else if err := json.Unmarshal(c.Pack, &p); err != nil || manifest.ValidatePack(&p) != nil {
				continue // never resolved and unreachable now: nothing to show
			}
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			out = append(out, source{pack: p, kind: "code", code: c.Code})
		}
	}

	if len(st.Modrinth) > 0 {
		mr := s.modrinth()
		rctx, cancel := context.WithTimeout(ctx, refreshTimeout)
		defer cancel()
		fresh := make([]*modrinth.Project, len(st.Modrinth))
		var wg sync.WaitGroup
		for i, m := range st.Modrinth {
			wg.Add(1)
			go func(i int, id string) {
				defer wg.Done()
				if p, err := mr.Project(rctx, id); err == nil {
					fresh[i] = &p
				}
			}(i, m.ProjectID)
		}
		wg.Wait()
		for i := range st.Modrinth {
			m := &st.Modrinth[i]
			var p manifest.Pack
			if fresh[i] != nil {
				p = modrinthEntry(m.Slug, *fresh[i])
				if raw, err := json.Marshal(p); err == nil && string(raw) != string(m.Pack) {
					m.Pack = raw
					changed = true
				}
			} else if err := json.Unmarshal(m.Pack, &p); err != nil || manifest.ValidatePack(&p) != nil {
				continue
			}
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			out = append(out, source{pack: p, kind: "modrinth", modrinth: m})
		}
	}
	if changed {
		_ = saveSettings(st)
	}

	if url := manifestURL(st); url != "" {
		m, err := manifest.Fetch(ctx, s.client(), userAgent(), url)
		if err != nil {
			if len(out) == 0 {
				return nil, err
			}
			// Codes still work; the manifest's failure is not fatal.
		} else {
			for _, p := range m.Packs {
				if seen[p.ID] {
					continue
				}
				seen[p.ID] = true
				out = append(out, source{pack: p, kind: "manifest"})
			}
		}
	}
	return out, nil
}

// AddPackCode resolves a code against the registry, remembers it, and returns
// the pack. Entering a code already present just refreshes it.
func (s *Service) AddPackCode(input string) (models.Pack, error) {
	code, err := registry.NormalizeCode(input)
	if err != nil {
		return models.Pack{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	st := loadSettings()
	reg, err := s.registry(st).Resolve(ctx, code)
	if err != nil {
		return models.Pack{}, err
	}
	raw, err := json.Marshal(reg.Pack)
	if err != nil {
		return models.Pack{}, err
	}
	entry := models.PackCode{Code: code, AddedAtMs: time.Now().UnixMilli(), Pack: raw}
	replaced := false
	for i, c := range st.Codes {
		if c.Code == code {
			entry.AddedAtMs = c.AddedAtMs
			st.Codes[i] = entry
			replaced = true
		}
	}
	if !replaced {
		st.Codes = append(st.Codes, entry)
	}
	if err := saveSettings(st); err != nil {
		return models.Pack{}, err
	}
	p := s.describe(reg.Pack, minecraftDir(st), st)
	p.Source, p.Code = "code", &code
	return p, nil
}

// RemovePackCode forgets a code. Installed files and the launcher profile are
// left alone; the pack simply stops being listed.
func (s *Service) RemovePackCode(code string) error {
	st := loadSettings()
	kept := st.Codes[:0]
	found := false
	for _, c := range st.Codes {
		if c.Code == code {
			found = true
			continue
		}
		kept = append(kept, c)
	}
	if !found {
		return fmt.Errorf("no pack with code %q", code)
	}
	st.Codes = kept
	return saveSettings(st)
}

// modrinthVersions converts API versions for the frontend.
func modrinthVersions(vs []modrinth.Version) []models.ModrinthVersion {
	out := make([]models.ModrinthVersion, 0, len(vs))
	for _, v := range vs {
		if _, err := v.Pack(); err != nil {
			continue // nothing installable attached
		}
		out = append(out, models.ModrinthVersion{
			ID: v.ID, Number: v.Number, Type: v.Type, PublishedAtMs: v.PublishedAtMs(),
			GameVersions: core.NonNil(v.GameVersions), Loaders: core.NonNil(v.Loaders),
		})
	}
	return out
}

func parseModrinthLink(input string) (modrinth.Ref, error) {
	ref, ok := modrinth.ParseRef(input)
	if !ok {
		return modrinth.Ref{}, errors.New("that is not a Modrinth link — it should look like https://modrinth.com/modpack/<name>")
	}
	return ref, ref.Validate()
}

// LookupModrinth reads a pasted modrinth.com link far enough to show the
// pack and its versions, so the user can choose one before adding. Nothing
// is saved.
func (s *Service) LookupModrinth(input string) (models.ModrinthLookup, error) {
	ref, err := parseModrinthLink(input)
	if err != nil {
		return models.ModrinthLookup{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	mr := s.modrinth()
	proj, err := mr.Project(ctx, ref.Project)
	if err != nil {
		return models.ModrinthLookup{}, err
	}
	vs, err := mr.Versions(ctx, proj.ID)
	if err != nil {
		return models.ModrinthLookup{}, err
	}
	suggested, err := modrinth.Pick(vs, ref.Version)
	if err != nil {
		return models.ModrinthLookup{}, err
	}
	out := models.ModrinthLookup{
		Project: proj.Slug, Input: strings.TrimSpace(input), Name: proj.Title, Description: proj.Description,
		Icon: core.Str(proj.IconURL), PageURL: proj.PageURL(),
		Versions: modrinthVersions(vs), Suggested: suggested.Number,
	}
	for _, m := range loadSettings().Modrinth {
		if m.ProjectID == proj.ID {
			out.AlreadyAdded, out.HeldVersion = true, core.Str(m.Version)
		}
	}
	return out, nil
}

// AddModrinthPack adds a Modrinth modpack from a pasted modrinth.com link,
// held at the given version number (or, when blank, the version the link
// names, else the newest release). The project is looked up, the version
// checked to exist, and both remembered. Adding a project already present
// moves it to the chosen version.
func (s *Service) AddModrinthPack(input, version string) (models.Pack, error) {
	ref, err := parseModrinthLink(input)
	if err != nil {
		return models.Pack{}, err
	}
	if version == "" {
		version = ref.Version
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	mr := s.modrinth()
	proj, err := mr.Project(ctx, ref.Project)
	if err != nil {
		return models.Pack{}, err
	}
	vs, err := mr.Versions(ctx, proj.ID)
	if err != nil {
		return models.Pack{}, err
	}
	v, err := modrinth.Pick(vs, version)
	if err != nil {
		return models.Pack{}, err
	}
	st := loadSettings()
	entry := models.ModrinthPack{ProjectID: proj.ID, Slug: proj.Slug, Version: v.Number, AddedAtMs: time.Now().UnixMilli()}
	for _, m := range st.Modrinth {
		if m.ProjectID == proj.ID {
			entry.Slug, entry.AddedAtMs = m.Slug, m.AddedAtMs
		}
	}
	p := modrinthEntry(entry.Slug, proj)
	if entry.Pack, err = json.Marshal(p); err != nil {
		return models.Pack{}, err
	}
	replaced := false
	for i, m := range st.Modrinth {
		if m.ProjectID == proj.ID {
			st.Modrinth[i] = entry
			replaced = true
		}
	}
	if !replaced {
		st.Modrinth = append(st.Modrinth, entry)
	}
	if err := saveSettings(st); err != nil {
		return models.Pack{}, err
	}
	out := s.describe(p, minecraftDir(st), st)
	out.Source, out.Project, out.HeldVersion = "modrinth", &entry.Slug, &entry.Version
	return out, nil
}

// ListModrinthVersions lists the versions of an added Modrinth pack, newest
// first, for the version picker.
func (s *Service) ListModrinthVersions(id string) ([]models.ModrinthVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	src, err := s.findSource(ctx, id)
	if err != nil {
		return nil, err
	}
	if src.modrinth == nil {
		return nil, fmt.Errorf("%s is not a Modrinth pack", id)
	}
	vs, err := s.modrinth().Versions(ctx, src.modrinth.ProjectID)
	if err != nil {
		return nil, err
	}
	return modrinthVersions(vs), nil
}

// SetModrinthVersion holds an added Modrinth pack at another version (by
// number or id). Nothing is installed until the next sync; CheckPack then
// reports the move. Returns the pack.
func (s *Service) SetModrinthVersion(id, version string) (models.Pack, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	src, err := s.findSource(ctx, id)
	if err != nil {
		return models.Pack{}, err
	}
	if src.modrinth == nil {
		return models.Pack{}, fmt.Errorf("%s is not a Modrinth pack", id)
	}
	vs, err := s.modrinth().Versions(ctx, src.modrinth.ProjectID)
	if err != nil {
		return models.Pack{}, err
	}
	v, err := modrinth.Pick(vs, strings.TrimSpace(version))
	if err != nil {
		return models.Pack{}, err
	}
	st := loadSettings()
	var held *models.ModrinthPack
	for i := range st.Modrinth {
		if st.Modrinth[i].ProjectID == src.modrinth.ProjectID {
			st.Modrinth[i].Version = v.Number
			held = &st.Modrinth[i]
		}
	}
	if held == nil {
		return models.Pack{}, fmt.Errorf("no Modrinth pack %q", id)
	}
	if err := saveSettings(st); err != nil {
		return models.Pack{}, err
	}
	out := s.describe(src.pack, minecraftDir(st), st)
	out.Source, out.Project, out.HeldVersion = "modrinth", &held.Slug, &held.Version
	return out, nil
}

// RemoveModrinthPack forgets a Modrinth pack by its pack id. Installed files
// and the launcher profile are left alone.
func (s *Service) RemoveModrinthPack(id string) error {
	st := loadSettings()
	kept := st.Modrinth[:0]
	found := false
	for _, m := range st.Modrinth {
		if modrinthPackID(m.Slug) == id {
			found = true
			continue
		}
		kept = append(kept, m)
	}
	if !found {
		return fmt.Errorf("no Modrinth pack %q", id)
	}
	st.Modrinth = kept
	return saveSettings(st)
}
