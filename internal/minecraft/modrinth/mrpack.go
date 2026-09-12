package modrinth

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"muster/internal/minecraft/packwiz"
)

// IndexFile is modrinth.index.json's name inside the archive.
const IndexFile = "modrinth.index.json"

// MaxPackSize bounds how much of an .mrpack is read: the archive holds the
// index and overrides, never the mods themselves.
const MaxPackSize = 256 << 20

// Index is modrinth.index.json.
type Index struct {
	FormatVersion int               `json:"formatVersion"`
	Game          string            `json:"game"`
	VersionID     string            `json:"versionId"`
	Name          string            `json:"name"`
	Summary       string            `json:"summary"`
	Files         []IndexEntry      `json:"files"`
	Dependencies  map[string]string `json:"dependencies"`
}

// IndexEntry is one downloadable file.
type IndexEntry struct {
	Path      string            `json:"path"`
	Hashes    map[string]string `json:"hashes"`
	Env       *Env              `json:"env"`
	Downloads []string          `json:"downloads"`
	FileSize  int64             `json:"fileSize"`
}

// Env says which side a file belongs to: required | optional | unsupported.
type Env struct {
	Client string `json:"client"`
	Server string `json:"server"`
}

// ForClient reports whether the file belongs on a player's machine; no env
// means both sides.
func (e IndexEntry) ForClient() bool { return e.Env == nil || e.Env.Client != "unsupported" }

// Optional reports whether the player may leave the file out.
func (e IndexEntry) Optional() bool { return e.Env != nil && e.Env.Client == "optional" }

// loaders maps the index's dependency keys to packwiz's loader names, in the
// order they are tried.
var loaders = [][2]string{{"neoforge", "neoforge"}, {"fabric-loader", "fabric"}, {"forge", "forge"}, {"quilt-loader", "quilt"}}

// Loader returns the loader the pack depends on, as packwiz names it.
func (i Index) Loader() (name, version string) {
	for _, l := range loaders {
		if v := i.Dependencies[l[0]]; v != "" {
			return l[1], v
		}
	}
	return "", ""
}

// downloadHosts are where the format allows a file to be fetched from; a
// pack pointing anywhere else is refused, as the specification requires.
var downloadHosts = map[string]bool{
	"cdn.modrinth.com":          true,
	"github.com":                true,
	"raw.githubusercontent.com": true,
	"gitlab.com":                true,
}

// ParseIndex decodes and validates an index.
func ParseIndex(data []byte) (Index, error) {
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return Index{}, fmt.Errorf("%s: %w", IndexFile, err)
	}
	if idx.FormatVersion != 1 {
		return Index{}, fmt.Errorf("%s: unsupported formatVersion %d", IndexFile, idx.FormatVersion)
	}
	if idx.Game != "minecraft" {
		return Index{}, fmt.Errorf("%s: not a Minecraft pack (game %q)", IndexFile, idx.Game)
	}
	if idx.Dependencies["minecraft"] == "" {
		return Index{}, fmt.Errorf("%s: no minecraft version in dependencies", IndexFile)
	}
	if idx.VersionID == "" {
		return Index{}, fmt.Errorf("%s: no versionId", IndexFile)
	}
	for _, f := range idx.Files {
		if !packwiz.SafeRelPath(f.Path) {
			return Index{}, fmt.Errorf("%s: refusing path %q", IndexFile, f.Path)
		}
	}
	return idx, nil
}

// hashOf picks the strongest hash the entry carries.
func hashOf(h map[string]string) (format, hash string) {
	for _, f := range []string{"sha512", "sha1"} {
		if v := h[f]; v != "" {
			return f, v
		}
	}
	return "", ""
}

// entryOf turns an index entry into a download.
func entryOf(f IndexEntry) (packwiz.Entry, error) {
	format, hash := hashOf(f.Hashes)
	if hash == "" {
		return packwiz.Entry{}, fmt.Errorf("%s: no sha512 or sha1 hash", f.Path)
	}
	if len(f.Downloads) == 0 {
		return packwiz.Entry{}, fmt.Errorf("%s: no download url", f.Path)
	}
	// Every URL on an allowed host is kept, in the index's order, so a
	// mirror that is down or missing does not fail the file.
	var urls []string
	for _, dl := range f.Downloads {
		if u, err := url.Parse(dl); err == nil && u.Scheme == "https" && downloadHosts[strings.ToLower(u.Hostname())] {
			urls = append(urls, dl)
		}
	}
	if len(urls) == 0 {
		return packwiz.Entry{}, fmt.Errorf("%s: no download from a host the mrpack format allows (%s)", f.Path, strings.Join(f.Downloads, ", "))
	}
	return packwiz.Entry{
		Path: f.Path, Name: path.Base(f.Path), URL: urls[0], Mirrors: urls[1:],
		HashFormat: format, Hash: hash,
		Optional: f.Optional(), Default: true,
	}, nil
}

// Load fetches a version's .mrpack, verifies it against the hash Modrinth
// published for it, and resolves it: one Entry per client-side download and
// one per file under overrides/ and client-overrides/ (the latter winning
// when both carry the same path). The pack's own files stay attached to the
// archive in memory through Entry.Open.
func (c *Client) Load(ctx context.Context, v Version) (*packwiz.Resolved, error) {
	f, err := v.Pack()
	if err != nil {
		return nil, err
	}
	raw, err := c.fetch(ctx, f)
	if err != nil {
		return nil, err
	}
	return Resolve(raw, v)
}

func (c *Client) fetch(ctx context.Context, f File) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", f.Filename, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("fetch %s: HTTP %d", f.Filename, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxPackSize+1))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", f.Filename, err)
	}
	if len(raw) > MaxPackSize {
		return nil, fmt.Errorf("%s is larger than %d MB", f.Filename, MaxPackSize>>20)
	}
	if format, hash := hashOf(f.Hashes); hash != "" {
		if err := packwiz.Verify(format, hash, raw); err != nil {
			return nil, fmt.Errorf("%s: %w", f.Filename, err)
		}
	}
	return raw, nil
}

// Resolve reads an .mrpack held in memory. v supplies the version number the
// state records (the index's versionId is used when v is zero).
func Resolve(raw []byte, v Version) (*packwiz.Resolved, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("not an .mrpack (zip): %w", err)
	}
	var idx *Index
	overrides := map[string]*zip.File{}
	for _, zf := range zr.File {
		name := zf.Name
		if name == IndexFile {
			rc, err := zf.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			parsed, err := ParseIndex(data)
			if err != nil {
				return nil, err
			}
			idx = &parsed
			continue
		}
		if zf.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
			continue
		}
		for _, prefix := range []string{"overrides/", "client-overrides/"} {
			rel, ok := strings.CutPrefix(name, prefix)
			if !ok {
				continue
			}
			if !packwiz.SafeRelPath(rel) {
				return nil, fmt.Errorf("%s: refusing path %q", prefix, rel)
			}
			if prev, dup := overrides[rel]; !dup || strings.HasPrefix(name, "client-overrides/") || !strings.HasPrefix(prev.Name, "client-overrides/") {
				overrides[rel] = zf
			}
		}
	}
	if idx == nil {
		return nil, errors.New("not an .mrpack: no " + IndexFile)
	}
	loader, loaderVersion := idx.Loader()
	version := v.Number
	if version == "" {
		version = idx.VersionID
	}
	res := &packwiz.Resolved{Pack: packwiz.Pack{
		Name: idx.Name, Version: version, Description: idx.Summary,
		Versions: map[string]string{"minecraft": idx.Dependencies["minecraft"]},
	}}
	if loader != "" {
		res.Pack.Versions[loader] = loaderVersion
	}
	for _, f := range idx.Files {
		if !f.ForClient() {
			continue
		}
		e, err := entryOf(f)
		if err != nil {
			return nil, err
		}
		if err := res.Add(e, IndexFile); err != nil {
			return nil, err
		}
	}
	rels := make([]string, 0, len(overrides))
	for rel := range overrides {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		zf := overrides[rel]
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		hash, err := packwiz.HashReader("sha512", rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", zf.Name, err)
		}
		e := packwiz.Entry{Path: rel, Name: rel, HashFormat: "sha512", Hash: hash, Open: zf.Open}
		if err := res.Add(e, zf.Name); err != nil {
			return nil, err
		}
	}
	return res, nil
}
