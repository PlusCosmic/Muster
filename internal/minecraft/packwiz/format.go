// Package packwiz reads packwiz packs (https://packwiz.infra.link) and syncs
// them into a directory: the client-side half of packwiz-installer, in Go.
//
// A pack is three TOML layers, all fetched relative to the pack.toml URL:
//
//	pack.toml           name, version, [versions] minecraft + loader,
//	                    [index] file + hash of index.toml
//	index.toml          [[files]] file + hash for every pack file; entries
//	                    with metafile = true are .pw.toml metafiles
//	<name>.pw.toml      one downloadable file: filename, side, [download]
//	                    url or mode, hash-format + hash
//
// Hashes are verified at every layer, so a pack served through a CDN that
// caches stale content fails loudly rather than installing the wrong jar.
package packwiz

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"

	"github.com/BurntSushi/toml"
)

// Pack is pack.toml.
type Pack struct {
	Name        string            `toml:"name"`
	Author      string            `toml:"author"`
	Version     string            `toml:"version"`
	Description string            `toml:"description"`
	PackFormat  string            `toml:"pack-format"`
	Index       IndexRef          `toml:"index"`
	Versions    map[string]string `toml:"versions"`
}

// IndexRef is pack.toml's [index] table.
type IndexRef struct {
	File       string `toml:"file"`
	HashFormat string `toml:"hash-format"`
	Hash       string `toml:"hash"`
}

// Loader returns the mod loader named in [versions] and its version, or "".
// packwiz's key set: fabric, forge, neoforge, quilt, liteloader.
func (p Pack) Loader() (name, version string) {
	for _, l := range []string{"neoforge", "fabric", "forge", "quilt", "liteloader"} {
		if v, ok := p.Versions[l]; ok && v != "" {
			return l, v
		}
	}
	return "", ""
}

// Index is index.toml.
type Index struct {
	HashFormat string      `toml:"hash-format"`
	Files      []IndexFile `toml:"files"`
}

// IndexFile is one [[files]] entry. File is a slash-separated path relative
// to the pack root; Hash is in the index's hash-format unless the entry
// carries its own.
type IndexFile struct {
	File       string `toml:"file"`
	Hash       string `toml:"hash"`
	HashFormat string `toml:"hash-format"`
	Alias      string `toml:"alias"`
	Metafile   bool   `toml:"metafile"`
	Preserve   bool   `toml:"preserve"`
}

// Metafile is a .pw.toml.
type Metafile struct {
	Name     string   `toml:"name"`
	Filename string   `toml:"filename"`
	Side     string   `toml:"side"`
	Download Download `toml:"download"`
	Option   Option   `toml:"option"`
	Update   Update   `toml:"update"`
}

// Download is a metafile's [download] table. Exactly one of URL or Mode is
// meaningful: a URL is fetched directly; Mode "metadata:curseforge" means the
// URL has to be derived from [update.curseforge] (see Resolve).
type Download struct {
	URL        string `toml:"url"`
	Mode       string `toml:"mode"`
	HashFormat string `toml:"hash-format"`
	Hash       string `toml:"hash"`
}

// Option marks a file the user may opt out of.
type Option struct {
	Optional    bool   `toml:"optional"`
	Default     bool   `toml:"default"`
	Description string `toml:"description"`
}

// Update carries the per-source identifiers packwiz uses to check for updates.
type Update struct {
	Modrinth   *ModrinthUpdate   `toml:"modrinth"`
	CurseForge *CurseForgeUpdate `toml:"curseforge"`
}

type ModrinthUpdate struct {
	ModID   string `toml:"mod-id"`
	Version string `toml:"version"`
}

type CurseForgeUpdate struct {
	FileID    int `toml:"file-id"`
	ProjectID int `toml:"project-id"`
}

// ForClient reports whether the file belongs on a player's machine.
// packwiz sides: "both" (default), "client", "server".
func (m Metafile) ForClient() bool {
	return m.Side == "" || m.Side == "both" || m.Side == "client"
}

// ParsePack, ParseIndex and ParseMetafile decode the three layers.
func ParsePack(data []byte) (Pack, error) {
	var p Pack
	if err := toml.Unmarshal(data, &p); err != nil {
		return Pack{}, fmt.Errorf("pack.toml: %w", err)
	}
	if p.Index.File == "" {
		return Pack{}, fmt.Errorf("pack.toml: no [index] file")
	}
	return p, nil
}

func ParseIndex(data []byte) (Index, error) {
	var i Index
	if err := toml.Unmarshal(data, &i); err != nil {
		return Index{}, fmt.Errorf("index.toml: %w", err)
	}
	for _, f := range i.Files {
		if strings.HasPrefix(f.File, "/") || strings.Contains(f.File, "..") || strings.Contains(f.File, "\\") {
			return Index{}, fmt.Errorf("index.toml: refusing path %q", f.File)
		}
	}
	return i, nil
}

func ParseMetafile(data []byte) (Metafile, error) {
	var m Metafile
	if err := toml.Unmarshal(data, &m); err != nil {
		return Metafile{}, fmt.Errorf("metafile: %w", err)
	}
	if m.Filename == "" || strings.ContainsAny(m.Filename, `/\`) {
		return Metafile{}, fmt.Errorf("metafile %q: bad filename %q", m.Name, m.Filename)
	}
	return m, nil
}

// NewHash returns a hasher for a packwiz hash-format name.
func NewHash(format string) (hash.Hash, error) {
	switch strings.ToLower(format) {
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	case "sha1":
		return sha1.New(), nil
	case "md5":
		return md5.New(), nil
	}
	return nil, fmt.Errorf("unsupported hash format %q", format)
}

// HashBytes hashes data with the named format, as lowercase hex.
func HashBytes(format string, data []byte) (string, error) {
	h, err := NewHash(format)
	if err != nil {
		return "", err
	}
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashReader hashes everything readable from r.
func HashReader(format string, r io.Reader) (string, error) {
	h, err := NewHash(format)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Verify checks data against an expected hash; the comparison ignores case.
func Verify(format, want string, data []byte) error {
	got, err := HashBytes(format, data)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%s mismatch: want %s, got %s", format, want, got)
	}
	return nil
}
