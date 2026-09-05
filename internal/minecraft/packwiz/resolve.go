package packwiz

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Entry is one file the pack wants on disk, with everything needed to fetch
// and verify it. Path is slash-separated, relative to the install directory.
type Entry struct {
	Path       string
	Name       string // display name (metafile name, or the path)
	URL        string
	HashFormat string
	Hash       string
	Optional   bool
	Default    bool
	Preserve   bool
	// CurseForge is set when the URL was derived from CurseForge ids rather
	// than given by the pack; those downloads can legitimately be refused.
	CurseForge bool
	// PageURL is where a person can get the file by hand when the download is
	// refused: the CurseForge project page. Empty for direct downloads.
	PageURL string
}

// Resolved is a pack read to the bottom: every client-side file as an Entry.
type Resolved struct {
	Pack    Pack
	BaseURL string // directory of pack.toml, with trailing slash
	Entries []Entry
}

// baseOf returns the directory part of a URL, with a trailing slash.
func baseOf(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("bad pack URL %q", rawURL)
	}
	u.Path = path.Dir(u.Path) + "/"
	u.RawQuery, u.Fragment = "", ""
	return u.String(), nil
}

// join resolves a slash-separated pack-relative path against a base URL,
// escaping each segment (jar names carry '+' and spaces).
func join(base, rel string) string {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return base + strings.Join(parts, "/")
}

// curseForgeURL is the CDN location CurseForge serves a file from, derived
// from its numeric file id: files/<id/1000>/<id%1000>/<filename>. It is
// documented nowhere and has been stable for a decade; when it fails the
// pack author has to supply the file another way (see Report.Manual).
func curseForgeURL(fileID int, filename string) string {
	return fmt.Sprintf("https://edge.forgecdn.net/files/%d/%d/%s", fileID/1000, fileID%1000, url.PathEscape(filename))
}

// resolveMetafile turns a metafile at pack-relative metaPath into an Entry.
func resolveMetafile(metaPath string, m Metafile) (Entry, error) {
	dir := path.Dir(metaPath)
	if dir == "." {
		dir = ""
	} else {
		dir += "/"
	}
	e := Entry{
		Path:       dir + m.Filename,
		Name:       m.Name,
		HashFormat: m.Download.HashFormat,
		Hash:       m.Download.Hash,
		Optional:   m.Option.Optional,
		Default:    m.Option.Default,
	}
	if e.Name == "" {
		e.Name = m.Filename
	}
	switch {
	case m.Download.URL != "":
		e.URL = m.Download.URL
	case m.Download.Mode == "metadata:curseforge":
		if m.Update.CurseForge == nil || m.Update.CurseForge.FileID == 0 {
			return Entry{}, fmt.Errorf("%s: curseforge download without [update.curseforge] file-id", metaPath)
		}
		e.URL = curseForgeURL(m.Update.CurseForge.FileID, m.Filename)
		e.CurseForge = true
		if m.Update.CurseForge.ProjectID != 0 {
			e.PageURL = fmt.Sprintf("https://www.curseforge.com/projects/%d", m.Update.CurseForge.ProjectID)
		}
	default:
		return Entry{}, fmt.Errorf("%s: no download url and unknown mode %q", metaPath, m.Download.Mode)
	}
	if e.Hash == "" || e.HashFormat == "" {
		return Entry{}, fmt.Errorf("%s: download has no hash", metaPath)
	}
	return e, nil
}
