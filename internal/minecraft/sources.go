package minecraft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"muster/internal/minecraft/manifest"
	"muster/internal/minecraft/models"
	"muster/internal/minecraft/registry"
)

// ErrNoPacks is returned when the user has neither entered a pack code nor set
// a pack list URL.
var ErrNoPacks = errors.New("no packs yet — enter a pack code, or set a pack list URL in Settings")

// source says where a pack came from.
type source struct {
	pack manifest.Pack
	kind string // "code" | "manifest"
	code string
}

func (s *Service) registry(st models.Settings) *registry.Client {
	return &registry.Client{BaseURL: registryURL(st), HTTP: s.client(), UserAgent: userAgent()}
}

// refreshTimeout bounds how long ListPacks waits on the registry before
// falling back to what each code resolved to last time.
const refreshTimeout = 8 * time.Second

// sources is every pack the user can see: codes first (each re-resolved
// against the registry, falling back to the cached copy), then the manifest's
// packs. A manifest pack whose id collides with a code's is dropped. With no
// codes and no manifest URL it returns ErrNoPacks. Settings are updated with
// fresh registrations as a side effect.
func (s *Service) sources(ctx context.Context) ([]source, error) {
	st := loadSettings()
	if len(st.Codes) == 0 && manifestURL(st) == "" {
		return nil, ErrNoPacks
	}
	var out []source
	seen := map[string]bool{}

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
		changed := false
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
		if changed {
			_ = saveSettings(st)
		}
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
