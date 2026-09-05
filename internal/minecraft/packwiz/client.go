package packwiz

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
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
	for _, f := range index.Files {
		format := f.HashFormat
		if format == "" {
			format = index.HashFormat
		}
		rel := indexDir + f.File
		if !f.Metafile {
			res.Entries = append(res.Entries, Entry{
				Path:       rel,
				Name:       rel,
				URL:        join(base, rel),
				HashFormat: format,
				Hash:       f.Hash,
				Preserve:   f.Preserve,
			})
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
		res.Entries = append(res.Entries, e)
	}
	return res, nil
}
