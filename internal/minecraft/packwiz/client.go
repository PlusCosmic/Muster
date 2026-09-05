package packwiz

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Client fetches packs. The zero value uses http.DefaultClient.
type Client struct {
	HTTP      *http.Client
	UserAgent string
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

// get fetches a URL fully. Non-2xx is an error carrying the status.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, &StatusError{URL: url, Status: resp.StatusCode}
	}
	return io.ReadAll(resp.Body)
}

// StatusError is a non-2xx response.
type StatusError struct {
	URL    string
	Status int
}

func (e *StatusError) Error() string { return fmt.Sprintf("%s: HTTP %d", e.URL, e.Status) }

// Load reads a pack from its pack.toml URL down to every client-side file,
// verifying the index against pack.toml and every metafile against the index.
func (c *Client) Load(ctx context.Context, packURL string) (*Resolved, error) {
	base, err := baseOf(packURL)
	if err != nil {
		return nil, err
	}
	raw, err := c.get(ctx, packURL)
	if err != nil {
		return nil, fmt.Errorf("fetch pack.toml: %w", err)
	}
	pack, err := ParsePack(raw)
	if err != nil {
		return nil, err
	}

	indexURL := join(base, pack.Index.File)
	raw, err = c.get(ctx, indexURL)
	if err != nil {
		return nil, fmt.Errorf("fetch index: %w", err)
	}
	if err := Verify(pack.Index.HashFormat, pack.Index.Hash, raw); err != nil {
		return nil, fmt.Errorf("index.toml: %w", err)
	}
	index, err := ParseIndex(raw)
	if err != nil {
		return nil, err
	}
	indexDir := path.Dir(pack.Index.File)
	if indexDir == "." {
		indexDir = ""
	} else {
		indexDir += "/"
	}

	res := &Resolved{Pack: pack, BaseURL: base}
	seen := map[string]string{}
	add := func(e Entry, from string) error {
		if e.Path == StateFile || e.Path == StateFile+".tmp" {
			return fmt.Errorf("%s: %q is reserved for Muster's own state", from, e.Path)
		}
		if other, dup := seen[e.Path]; dup {
			return fmt.Errorf("%s and %s both install %q", other, from, e.Path)
		}
		seen[e.Path] = from
		res.Entries = append(res.Entries, e)
		return nil
	}
	for _, f := range index.Files {
		format := f.HashFormat
		if format == "" {
			format = index.HashFormat
		}
		rel := indexDir + f.File
		if !f.Metafile {
			if err := add(Entry{
				Path:       rel,
				Name:       rel,
				URL:        join(base, rel),
				HashFormat: format,
				Hash:       f.Hash,
				Preserve:   f.Preserve,
			}, rel); err != nil {
				return nil, err
			}
			continue
		}
		raw, err := c.get(ctx, join(base, rel))
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", rel, err)
		}
		if err := Verify(format, f.Hash, raw); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		m, err := ParseMetafile(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if !m.ForClient() {
			continue
		}
		e, err := resolveMetafile(rel, m)
		if err != nil {
			return nil, err
		}
		e.Preserve = f.Preserve
		if err := add(e, rel); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// download streams a URL into a temp file beside dest while hashing it, and
// renames it into place only if the hash matches. Nothing is held in memory,
// so a multi-gigabyte resource pack costs no more than a small jar. Returns
// the byte count.
func (c *Client) download(ctx context.Context, e Entry, dest string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.URL, nil)
	if err != nil {
		return 0, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return 0, &StatusError{URL: e.URL, Status: resp.StatusCode}
	}
	h, err := NewHash(e.HashFormat)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, e.Hash) {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("%s mismatch: want %s, got %s", e.HashFormat, e.Hash, got)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return n, nil
}
