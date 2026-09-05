// Package manifest reads the list of packs Muster offers: one JSON document
// at a URL the pack author controls, next to the packs it points at.
//
//	{
//	  "packs": [
//	    {
//	      "id": "frontier",                        // stable, [a-z0-9-]; names the install dir
//	      "name": "Frontier",
//	      "description": "…",                      // optional
//	      "icon": "https://…/icon.png",             // optional
//	      "pack": "https://…/<slug>/pack.toml",     // the packwiz pack
//	      "recommended": { "minMemoryMb": 4096, "maxMemoryMb": 8192, "args": ["-XX:+UseZGC"] },
//	      "server": "play.example.com"              // optional, offered as a quick-join
//	    }
//	  ]
//	}
//
// "recommended" is advice, not configuration: every machine differs, so Muster
// shows it and lets the user set the heap and JVM args they actually want
// (the older key "java" is still read as the same thing).
//
// Minecraft version and loader are not repeated here: pack.toml already
// carries them. Muster ships with no manifest of its own; users paste the
// URL of the list they were given.
package manifest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Manifest is the document.
type Manifest struct {
	Packs []Pack `json:"packs"`
}

// Pack is one offered pack.
type Pack struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Icon        string      `json:"icon"`
	PackURL     string      `json:"pack"`
	Recommended Recommended `json:"recommended"`
	Server      string      `json:"server"`
	// LegacyJava is the pre-rename spelling of Recommended.
	LegacyJava *Recommended `json:"java,omitempty"`
}

// Recommended is the launch configuration the author suggests: a heap range
// and JVM args. Advisory; see LaunchSettings on the Muster side.
type Recommended struct {
	MinMemoryMb int      `json:"minMemoryMb"`
	MaxMemoryMb int      `json:"maxMemoryMb"`
	Args        []string `json:"args"`
}

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Parse decodes and validates a manifest.
func Parse(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: %w", err)
	}
	seen := map[string]bool{}
	for i := range m.Packs {
		if err := ValidatePack(&m.Packs[i]); err != nil {
			return Manifest{}, fmt.Errorf("manifest: pack %d: %w", i, err)
		}
		if seen[m.Packs[i].ID] {
			return Manifest{}, fmt.Errorf("manifest: duplicate pack id %q", m.Packs[i].ID)
		}
		seen[m.Packs[i].ID] = true
	}
	if m.Packs == nil {
		m.Packs = []Pack{}
	}
	return m, nil
}

// ValidatePack checks one pack entry and normalises it in place: the legacy
// `java` key folds into Recommended, and a nil args list becomes empty.
func ValidatePack(p *Pack) error {
	if p.LegacyJava != nil {
		if p.Recommended.MaxMemoryMb == 0 && p.Recommended.MinMemoryMb == 0 && len(p.Recommended.Args) == 0 {
			p.Recommended = *p.LegacyJava
		}
		p.LegacyJava = nil
	}
	if !idRe.MatchString(p.ID) {
		return fmt.Errorf("bad id %q", p.ID)
	}
	if p.Name == "" {
		return fmt.Errorf("pack %q has no name", p.ID)
	}
	if u, err := url.Parse(p.PackURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("pack %q has bad pack url %q", p.ID, p.PackURL)
	}
	if p.Recommended.MaxMemoryMb != 0 && p.Recommended.MinMemoryMb > p.Recommended.MaxMemoryMb {
		return fmt.Errorf("pack %q has minMemoryMb > maxMemoryMb", p.ID)
	}
	for _, a := range p.Recommended.Args {
		// The launcher stores JVM options as one space-separated string and
		// splits it on whitespace with no quoting, so an argument containing
		// whitespace cannot be represented; refuse rather than mangle it.
		if a == "" || strings.ContainsAny(a, " \t\n") {
			return fmt.Errorf("pack %q has a java arg with whitespace: %q", p.ID, a)
		}
	}
	if p.Recommended.Args == nil {
		p.Recommended.Args = []string{}
	}
	return nil
}

// Fetch downloads and parses the manifest at url.
func Fetch(ctx context.Context, client *http.Client, userAgent, url string) (Manifest, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Manifest{}, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return Manifest{}, fmt.Errorf("fetch manifest: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Manifest{}, err
	}
	return Parse(data)
}
