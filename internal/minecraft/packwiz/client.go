package packwiz

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
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
	add := res.Add
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

// sizedBody is a response body that knows its announced length.
type sizedBody struct {
	io.ReadCloser
	length int64
}

func (b sizedBody) ContentLength() int64 { return b.length }

// open returns the entry's bytes: from inside the pack when it carries them,
// otherwise by fetching its URL.
func (c *Client) open(ctx context.Context, e Entry) (io.ReadCloser, error) {
	if e.Open != nil {
		return e.Open()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.URL, nil)
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
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return nil, &StatusError{URL: e.URL, Status: resp.StatusCode}
	}
	return sizedBody{resp.Body, resp.ContentLength}, nil
}

// downloadAttempts is how many times a download cut off mid-body (a short
// read, or a hash mismatch on fewer bytes than announced) is tried again.
// Failures before the body starts are retried underneath by retry.Transport.
const downloadAttempts = 3

// errTruncated marks a body that ended before its announced length.
var errTruncated = errors.New("connection closed before the file was complete")

// download fetches an entry from its URL, then from each mirror in turn
// when that fails for any reason but a cancelled context: a mirror that is
// down, refuses, or serves the wrong bytes is not the last word while
// another is listed. Returns the byte count, or the last mirror's error.
func (c *Client) download(ctx context.Context, e Entry, dest string) (int64, error) {
	n, err := c.downloadFrom(ctx, e, dest)
	for _, m := range e.Mirrors {
		if err == nil || ctx.Err() != nil {
			break
		}
		alt := e
		alt.URL = m
		n, err = c.downloadFrom(ctx, alt, dest)
	}
	return n, err
}

// downloadFrom streams an entry into a temp file beside dest while hashing
// it, and renames it into place only if the hash matches. Nothing is held in
// memory, so a multi-gigabyte resource pack costs no more than a small jar.
// Returns the byte count.
func (c *Client) downloadFrom(ctx context.Context, e Entry, dest string) (int64, error) {
	var n int64
	var err error
	for attempt := 0; attempt < downloadAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		n, err = c.downloadOnce(ctx, e, dest)
		if err == nil || ctx.Err() != nil || e.Open != nil {
			return n, err
		}
		var se *StatusError
		if errors.As(err, &se) {
			return n, err // the server answered; repeating changes nothing
		}
		// Network errors mid-body and truncation are worth another go; a
		// complete body with the wrong hash is not.
		if !errors.Is(err, errTruncated) && !isNetErr(err) {
			return n, err
		}
	}
	return n, err
}

func isNetErr(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)
}

func (c *Client) downloadOnce(ctx context.Context, e Entry, dest string) (int64, error) {
	body, err := c.open(ctx, e)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	var want int64 = -1
	if lr, ok := body.(interface{ ContentLength() int64 }); ok {
		want = lr.ContentLength()
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
	n, err := io.Copy(io.MultiWriter(f, h), body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if want >= 0 && n < want {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("%w (%d of %d bytes)", errTruncated, n, want)
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
