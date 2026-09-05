package packwiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	Files       map[string]string `json:"files"` // path -> "<format>:<hash>:<size>" (older states omit the size)
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

func stamp(e Entry, size int64) string {
	return e.HashFormat + ":" + e.Hash + ":" + strconv.FormatInt(size, 10)
}

// stampMatches reports whether a recorded stamp is for entry e, and the size
// it recorded (-1 when the stamp predates sizes).
func stampMatches(recorded string, e Entry) (bool, int64) {
	parts := strings.SplitN(recorded, ":", 3)
	if len(parts) < 2 || parts[0] != e.HashFormat || !strings.EqualFold(parts[1], e.Hash) {
		return false, -1
	}
	if len(parts) == 3 {
		if n, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
			return true, n
		}
	}
	return true, -1
}

// current reports whether the file on disk is the one the state says it is.
// A recorded size that matches is taken as proof (hashing 200 jars on every
// check would make the app slow to open); a stamp without a size, or a
// mismatched size, falls back to hashing the file. Returns whether the file is
// current and, when it is, its size so the state can be refreshed.
func current(local string, recorded string, e Entry) (bool, int64) {
	info, err := os.Stat(local)
	if err != nil || info.IsDir() {
		return false, 0
	}
	ok, size := stampMatches(recorded, e)
	if !ok {
		return false, 0
	}
	if size >= 0 && size == info.Size() {
		return true, size
	}
	f, err := os.Open(local)
	if err != nil {
		return false, 0
	}
	defer f.Close()
	got, err := HashReader(e.HashFormat, f)
	if err != nil || !strings.EqualFold(got, e.Hash) {
		return false, 0
	}
	return true, info.Size()
}

// Plan is what a sync would do. Pure: computed from the resolved pack, the
// recorded state, and which files exist.
type Plan struct {
	Download []Entry  // missing, changed, or no longer matching their hash
	Delete   []string // ours last time, gone from the pack now
	Keep     int
	// Restamp: files verified current whose state entry lacked a size; Apply
	// records the size so the next check is cheap.
	Restamp map[string]int64
}

// Empty reports whether the plan changes anything on disk.
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
		if statErr == nil && e.Preserve && state.Files[e.Path] != "" {
			// The pack says: never overwrite the user's edits once installed.
			plan.Keep++
			continue
		}
		if ok, size := current(local, state.Files[e.Path], e); ok {
			plan.Keep++
			if _, hadSize := stampMatches(state.Files[e.Path], e); hadSize < 0 {
				if plan.Restamp == nil {
					plan.Restamp = map[string]int64{}
				}
				plan.Restamp[e.Path] = size
			}
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
// themselves — today, a CurseForge file the CDN refused. URL is a page a
// person can download from (the project page), not the failed CDN link.
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

// Apply executes a plan against dir: streams each download to a temp file
// beside the target while hashing it, renames into place only on a match,
// deletes what left the pack, and records the new state. A CurseForge file
// the CDN refuses is reported in Report.Manual instead of failing the sync;
// any other failure aborts and leaves the state file describing exactly what
// is on disk.
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
	for p, size := range plan.Restamp {
		if rec, ok := state.Files[p]; ok {
			parts := strings.SplitN(rec, ":", 3)
			state.Files[p] = parts[0] + ":" + parts[1] + ":" + strconv.FormatInt(size, 10)
		}
	}
	if err := save(); err != nil {
		return rep, err
	}

	for i, e := range plan.Download {
		if progress != nil {
			progress(i+1, len(plan.Download), e)
		}
		local := filepath.Join(dir, filepath.FromSlash(e.Path))
		n, err := c.download(ctx, e, local)
		if err != nil {
			var se *StatusError
			if e.CurseForge && errors.As(err, &se) && (se.Status == 403 || se.Status == 404) {
				page := e.PageURL
				if page == "" {
					page = e.URL
				}
				rep.Manual = append(rep.Manual, Manual{Path: e.Path, Name: e.Name, URL: page, Why: fmt.Sprintf("CurseForge refused the download (HTTP %d)", se.Status)})
				continue
			}
			return rep, fmt.Errorf("download %s: %w", e.Name, err)
		}
		state.Files[e.Path] = stamp(e, n)
		rep.Downloaded = append(rep.Downloaded, e.Path)
		if err := save(); err != nil {
			return rep, err
		}
	}
	state.SyncedAtMs = nowMs()
	return rep, save()
}
