// Package registry is the client for the Muster pack registry: a small
// service where a pack author registers a pack and gets a shareable code, and
// where a code resolves back to the pack. The registry stores pointers, not
// packs; everything Muster needs to install is still fetched from the pack's
// own host.
//
//	GET /v1/packs/{code}  →  Registration | 404 {error:{code,message}}
//
// The registry URL is app infrastructure, like the update URL, so the default
// is a constant; self-hosters can override it in settings.
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"muster/internal/minecraft/manifest"
)

// DefaultURL is the public registry.
const DefaultURL = "https://api.musterlauncher.com"

// Registration is what a code resolves to.
type Registration struct {
	Code      string        `json:"code"`
	Pack      manifest.Pack `json:"pack"`
	CreatedAt string        `json:"createdAt"`
	UpdatedAt string        `json:"updatedAt"`
	Resolved  *Resolved     `json:"resolved"`
}

// Resolved is what the registry last read from the pack's pack.toml.
type Resolved struct {
	Version       string `json:"version"`
	Minecraft     string `json:"minecraft"`
	Loader        string `json:"loader"`
	LoaderVersion string `json:"loaderVersion"`
	CheckedAt     string `json:"checkedAt"`
}

// Error is the registry's error document.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("registry: HTTP %d", e.Status)
}

// ErrNotFound is returned for a code nobody has registered (or has deleted).
var ErrNotFound = errors.New("no pack is registered with that code")

// Client talks to one registry.
type Client struct {
	BaseURL   string
	HTTP      *http.Client
	UserAgent string
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return DefaultURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

var codeRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)+$`)

// NormalizeCode turns whatever the user pasted into a code: the code itself in
// any case or with spaces, a `muster://add/<code>` deep link, or a registry
// page URL ending in `/p/<code>`.
func NormalizeCode(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", errors.New("enter a pack code")
	}
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil || u.Path == "" {
			return "", errors.New("that link does not contain a pack code")
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		s = parts[len(parts)-1]
		if u.Scheme == "muster" && u.Host != "" && len(parts) == 0 {
			s = u.Host
		}
	}
	s = strings.ToLower(strings.Join(strings.Fields(s), "-"))
	if !codeRe.MatchString(s) {
		return "", fmt.Errorf("%q does not look like a pack code (e.g. amber-otter-42)", strings.TrimSpace(input))
	}
	return s, nil
}

// Resolve looks a code up.
func (c *Client) Resolve(ctx context.Context, code string) (Registration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/v1/packs/"+url.PathEscape(code), nil)
	if err != nil {
		return Registration{}, err
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return Registration{}, fmt.Errorf("could not reach the pack registry: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Registration{}, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return Registration{}, ErrNotFound
	}
	if resp.StatusCode/100 != 2 {
		var doc struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &doc)
		return Registration{}, &Error{Status: resp.StatusCode, Code: doc.Error.Code, Message: doc.Error.Message}
	}
	var reg Registration
	if err := json.Unmarshal(body, &reg); err != nil {
		return Registration{}, fmt.Errorf("registry returned something that is not a registration: %w", err)
	}
	if reg.Code == "" {
		reg.Code = code
	}
	if err := manifest.ValidatePack(&reg.Pack); err != nil {
		return Registration{}, fmt.Errorf("the registry's entry for %s is not usable: %w", code, err)
	}
	return reg, nil
}
