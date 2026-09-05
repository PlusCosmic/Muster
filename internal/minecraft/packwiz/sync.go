package packwiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// StateFile records, inside the install directory, what the last sync put
// there, so the next one can tell its own files from the user's and skip
// what is already current.
const StateFile = "muster-pack.json"

// State is StateFile's content.
type State struct {
	PackURL     string            `json:"packUrl"`
	PackName    string            `json:"packName"`
	PackVersion string            `json:"packVersion"`
	SyncedAtMs  int64             `json:"syncedAtMs"`
	Files       map[string]string `json:"files"` // path -> "<format>:<hash>"
}

// LoadState reads the state file; a missing file is an empty State.
func LoadState(dir string) (State, error) {
	raw, err := os.ReadFile(filepath.Join(dir, StateFile))
	if errors.Is(err, os.ErrNotExist) {
		return State{Files: map[string]string{}}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return State{}, fmt.Errorf("%s: %w", StateFile, err)
	}
	if s.Files == nil {
		s.Files = map[string]string{}
	}
	return s, nil
}

func saveState(dir string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, StateFile), data)
}

func writeAtomic(p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func stamp(e Entry) string { return e.HashFormat + ":" + e.Hash }

// Plan is what a sync would do. Pure: computed from the resolved pack, the
// recorded state, and which files exist.
type Plan struct {
	Download []Entry  // missing or changed
	Delete   []string // ours last time, gone from the pack now
	Keep     int
}

// Empty reports whether the plan changes nothing.
func (p Plan) Empty() bool { return len(p.Download) == 0 && len(p.Delete) == 0 }

// MakePlan diffs a resolved pack against an install directory. Optional
// entries default to their pack default; excluded ones are treated as absent.
func MakePlan(res *Resolved, dir string, state State, excluded map[string]bool) Plan {
	var plan Plan
	wanted := map[string]bool{}
	for _, e := range res.Entries {
		if e.Optional && (excluded[e.Path] || (!e.Default && !state.hasFile(e.Path))) {
			continue
		}
		wanted[e.Path] = true
		local := filepath.Join(dir, filepath.FromSlash(e.Path))
		_, statErr := os.Stat(local)
		if statErr == nil && state.Files[e.Path] == stamp(e) {
			plan.Keep++
			continue
		}
		if statErr == nil && e.Preserve && state.Files[e.Path] != "" {
			// The pack says: never overwrite the user's edits once installed.
			plan.Keep++
			continue
		}
		plan.Download = append(plan.Download, e)
	}
	for p := range state.Files {
		if !wanted[p] {
			plan.Delete = append(plan.Delete, p)
		}
	}
	sort.Strings(plan.Delete)
	sort.Slice(plan.Download, func(i, j int) bool { return plan.Download[i].Path < plan.Download[j].Path })
	return plan
}

func (s State) hasFile(p string) bool { _, ok := s.Files[p]; return ok }

// Manual is a file the sync could not fetch but which the user can obtain
// themselves — today, a CurseForge file the CDN refused.
type Manual struct {
	Path string `json:"path"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Why  string `json:"why"`
}

// Report is the outcome of Apply.
type Report struct {
	Downloaded []string
	Deleted    []string
	Manual     []Manual
}

// Progress is called before each download with the 1-based index and total.
type Progress func(done, total int, current Entry)

// Apply executes a plan against dir: downloads to a temp file beside the
// target, verifies the hash, renames into place, deletes what left the pack,
// and records the new state. A CurseForge file the CDN refuses is reported
// in Report.Manual instead of failing the sync; any other failure aborts and
// leaves the state file describing exactly what is on disk.
func (c *Client) Apply(ctx context.Context, res *Resolved, dir string, plan Plan, packURL string, progress Progress) (Report, error) {
	state, err := LoadState(dir)
	if err != nil {
		return Report{}, err
	}
	state.PackURL, state.PackName, state.PackVersion = packURL, res.Pack.Name, res.Pack.Version
	var rep Report
	// Persist after every file so an interrupted sync resumes precisely.
	save := func() error { return saveState(dir, state) }

	for _, p := range plan.Delete {
		local := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.Remove(local); err != nil && !errors.Is(err, os.ErrNotExist) {
			return rep, fmt.Errorf("remove %s: %w", p, err)
		}
		delete(state.Files, p)
		rep.Deleted = append(rep.Deleted, p)
	}
	if err := save(); err != nil {
		return rep, err
	}

	for i, e := range plan.Download {
		if progress != nil {
			progress(i+1, len(plan.Download), e)
		}
		data, err := c.get(ctx, e.URL)
		if err != nil {
			var se *StatusError
			if e.CurseForge && errors.As(err, &se) && (se.Status == 403 || se.Status == 404) {
				rep.Manual = append(rep.Manual, Manual{Path: e.Path, Name: e.Name, URL: e.URL, Why: fmt.Sprintf("CurseForge refused the download (HTTP %d)", se.Status)})
				continue
			}
			return rep, fmt.Errorf("download %s: %w", e.Name, err)
		}
		if err := Verify(e.HashFormat, e.Hash, data); err != nil {
			return rep, fmt.Errorf("%s: %w", e.Name, err)
		}
		local := filepath.Join(dir, filepath.FromSlash(e.Path))
		if err := writeAtomic(local, data); err != nil {
			return rep, fmt.Errorf("write %s: %w", e.Path, err)
		}
		state.Files[e.Path] = stamp(e)
		rep.Downloaded = append(rep.Downloaded, e.Path)
		if err := save(); err != nil {
			return rep, err
		}
	}
	state.SyncedAtMs = nowMs()
	return rep, save()
}
