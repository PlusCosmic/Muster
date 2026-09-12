// Package modrinth adds Modrinth modpacks as a pack source: the user pastes a
// modrinth.com link, the project is looked up on the Modrinth API, and the
// chosen version's .mrpack is turned into the same resolved-file model a
// packwiz pack becomes, so sync, state, loader install and the launcher
// profile need no second implementation.
//
//	GET https://api.modrinth.com/v2/project/{id|slug}          → Project
//	GET https://api.modrinth.com/v2/project/{id|slug}/version  → []Version
//
// An .mrpack is a zip: modrinth.index.json lists every downloadable file with
// its path, hashes, environment and download URLs; overrides/ and
// client-overrides/ carry files that ship inside the pack (configs, resource
// packs). See https://support.modrinth.com/en/articles/8802351-modrinth-modpack-format-mrpack
package modrinth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// PublishedAtMs is the version's publication time as Unix milliseconds, or
// 0 when unparseable.
func (v Version) PublishedAtMs() int64 {
	t, err := time.Parse(time.RFC3339Nano, v.DatePublished)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

// Newest returns the newest release, or the newest of anything when the
// project has never published a release. Same choice as Pick with no pin.
func Newest(vs []Version) (Version, error) { return Pick(vs, "") }

// DefaultURL is the public Modrinth API.
const DefaultURL = "https://api.modrinth.com/v2"

// SiteURL is the website, for project pages.
const SiteURL = "https://modrinth.com"

// Ref is what the user asked for: a project, and optionally one version of
// it (pinned, so a sync never moves it).
type Ref struct {
	Project string // slug or id, as pasted
	Version string // version number or id; "" ⇒ newest release
}

// Project is the part of a Modrinth project Muster shows.
type Project struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	IconURL     string `json:"icon_url"`
	ProjectType string `json:"project_type"`
}

// PageURL is the project's page on modrinth.com.
func (p Project) PageURL() string { return SiteURL + "/modpack/" + url.PathEscape(p.Slug) }

// Version is one published version of a project.
type Version struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"project_id"`
	Number        string   `json:"version_number"`
	Type          string   `json:"version_type"` // release | beta | alpha
	DatePublished string   `json:"date_published"`
	GameVersions  []string `json:"game_versions"`
	Loaders       []string `json:"loaders"`
	Files         []File   `json:"files"`
}

// File is one downloadable attached to a version; the primary one is the
// .mrpack.
type File struct {
	URL      string            `json:"url"`
	Filename string            `json:"filename"`
	Primary  bool              `json:"primary"`
	Size     int64             `json:"size"`
	Hashes   map[string]string `json:"hashes"`
}

// Pack returns the version's .mrpack: the primary file, or failing that the
// only file with the extension.
func (v Version) Pack() (File, error) {
	var candidates []File
	for _, f := range v.Files {
		if f.Primary && strings.HasSuffix(strings.ToLower(f.Filename), ".mrpack") {
			return f, nil
		}
		if strings.HasSuffix(strings.ToLower(f.Filename), ".mrpack") {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	return File{}, fmt.Errorf("version %s has no .mrpack file", v.Number)
}

// Client talks to one Modrinth API.
type Client struct {
	BaseURL   string
	HTTP      *http.Client
	UserAgent string
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return DefaultURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

// ErrNotFound is returned for a project Modrinth does not know.
var ErrNotFound = errors.New("Modrinth has no project by that name")

func (c *Client) get(ctx context.Context, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return fmt.Errorf("could not reach Modrinth: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode/100 != 2 {
		var doc struct {
			Error       string `json:"error"`
			Description string `json:"description"`
		}
		_ = json.Unmarshal(body, &doc)
		if doc.Description != "" {
			return fmt.Errorf("Modrinth: %s", doc.Description)
		}
		return fmt.Errorf("Modrinth: HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("Modrinth returned something unexpected: %w", err)
	}
	return nil
}

// Project looks a project up by slug or id and checks that it is a modpack.
func (c *Client) Project(ctx context.Context, idOrSlug string) (Project, error) {
	var p Project
	if err := c.get(ctx, "/project/"+url.PathEscape(idOrSlug), &p); err != nil {
		return Project{}, err
	}
	if p.ID == "" || p.Slug == "" {
		return Project{}, errors.New("Modrinth returned a project without an id")
	}
	if p.ProjectType != "modpack" {
		return Project{}, fmt.Errorf("%s is a Modrinth %s, not a modpack", p.Title, p.ProjectType)
	}
	return p, nil
}

// Versions lists a project's versions, newest first.
func (c *Client) Versions(ctx context.Context, idOrSlug string) ([]Version, error) {
	var vs []Version
	if err := c.get(ctx, "/project/"+url.PathEscape(idOrSlug)+"/version", &vs); err != nil {
		return nil, err
	}
	sort.SliceStable(vs, func(i, j int) bool { return vs[i].DatePublished > vs[j].DatePublished })
	return vs, nil
}

// Pick chooses the version a sync installs: the pinned one when the Ref
// names it (by number or id), otherwise the newest release, or the newest of
// anything when the project has never published a release.
func Pick(vs []Version, pin string) (Version, error) {
	if len(vs) == 0 {
		return Version{}, errors.New("the project has no versions yet")
	}
	if pin != "" {
		for _, v := range vs {
			if v.Number == pin || v.ID == pin {
				return v, nil
			}
		}
		return Version{}, fmt.Errorf("the project has no version %q", pin)
	}
	newest := vs[0]
	for _, v := range vs {
		if v.DatePublished > newest.DatePublished {
			newest = v
		}
	}
	var release *Version
	for i := range vs {
		if vs[i].Type == "release" && (release == nil || vs[i].DatePublished > release.DatePublished) {
			release = &vs[i]
		}
	}
	if release != nil {
		return *release, nil
	}
	return newest, nil
}

var slugRe = regexp.MustCompile(`^[\w!@$()` + "`" + `.+,"'-]{1,64}$`)

// ParseRef reads a pasted Modrinth link. Accepted:
//
//	https://modrinth.com/modpack/<slug>
//	https://modrinth.com/modpack/<slug>/version/<number-or-id>
//	https://modrinth.com/project/<id-or-slug>[/version/…]
//	modrinth.com/… (scheme optional)
//
// Anything else is not a Modrinth link, so the caller can try it as a pack
// code instead.
func ParseRef(input string) (Ref, bool) {
	s := strings.TrimSpace(input)
	if !strings.Contains(strings.ToLower(s), "modrinth.com/") {
		return Ref{}, false
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return Ref{}, false
	}
	host := strings.ToLower(u.Hostname())
	if host != "modrinth.com" && !strings.HasSuffix(host, ".modrinth.com") {
		return Ref{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// /<type>/<slug>[/version/<v>] — the type segment is whatever the site
	// uses (modpack, project, mod…); the API decides what it really is.
	if len(parts) < 2 || parts[1] == "" {
		return Ref{}, true
	}
	ref := Ref{Project: parts[1]}
	if len(parts) >= 4 && parts[2] == "version" {
		if v, err := url.PathUnescape(parts[3]); err == nil {
			ref.Version = v
		} else {
			ref.Version = parts[3]
		}
	}
	return ref, true
}

// IsLink reports whether input looks like a Modrinth link at all.
func IsLink(input string) bool { _, ok := ParseRef(input); return ok }

// Validate checks a Ref is usable.
func (r Ref) Validate() error {
	if r.Project == "" || !slugRe.MatchString(r.Project) {
		return errors.New("that Modrinth link does not name a project — it should look like https://modrinth.com/modpack/<name>")
	}
	return nil
}
