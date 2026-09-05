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
//	      "java": { "minMemoryMb": 4096, "maxMemoryMb": 8192, "args": ["-XX:+UseZGC"] },
//	      "server": "play.example.com"              // optional, offered as a quick-join
//	    }
//	  ]
//	}
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	PackURL     string `json:"pack"`
	Java        Java   `json:"java"`
	Server      string `json:"server"`
}

// Java is the launch configuration the author recommends.
type Java struct {
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
	for i, p := range m.Packs {
		if !idRe.MatchString(p.ID) {
			return Manifest{}, fmt.Errorf("manifest: pack %d has bad id %q", i, p.ID)
		}
		if seen[p.ID] {
			return Manifest{}, fmt.Errorf("manifest: duplicate pack id %q", p.ID)
		}
		seen[p.ID] = true
		if p.Name == "" {
			return Manifest{}, fmt.Errorf("manifest: pack %q has no name", p.ID)
		}
		if u, err := url.Parse(p.PackURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return Manifest{}, fmt.Errorf("manifest: pack %q has bad pack url %q", p.ID, p.PackURL)
		}
		if p.Java.MaxMemoryMb != 0 && p.Java.MinMemoryMb > p.Java.MaxMemoryMb {
			return Manifest{}, fmt.Errorf("manifest: pack %q has minMemoryMb > maxMemoryMb", p.ID)
		}
		for _, a := range p.Java.Args {
			// The launcher stores JVM options as one space-separated string and
			// splits it on whitespace with no quoting, so an argument containing
			// whitespace cannot be represented; refuse rather than mangle it.
			if a == "" || strings.ContainsAny(a, " \t\n") {
				return Manifest{}, fmt.Errorf("manifest: pack %q has a java arg with whitespace: %q", p.ID, a)
			}
		}
		if p.Java.Args == nil {
			m.Packs[i].Java.Args = []string{}
		}
	}
	if m.Packs == nil {
		m.Packs = []Pack{}
	}
	return m, nil
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
