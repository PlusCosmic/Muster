// Package mods is the mods backend: About.xml parsing, installed-mod
// discovery, ModsConfig.xml read/write, the community rules database, and
// auto-sort. Everything is synchronous so it can be unit-tested directly.
package mods

import (
	"errors"
	"strings"

	"rimforge/internal/xmldom"
)

// AboutData is everything we care about from an About.xml. Tag matching is
// case-insensitive; every package id is lowercased and de-duplicated while
// preserving document order.
type AboutData struct {
	PackageID         string
	Name              string
	Authors           string
	SupportedVersions []string
	Dependencies      []string
	LoadAfter         []string
	LoadBefore        []string
	ForceLoadAfter    []string
	ForceLoadBefore   []string
	IncompatibleWith  []string
}

func pushID(out *[]string, raw string) {
	id := strings.ToLower(strings.TrimSpace(raw))
	if id == "" {
		return
	}
	for _, e := range *out {
		if e == id {
			return
		}
	}
	*out = append(*out, id)
}

// collectLiIDs collects `<parent><li>id</li>…</parent>`.
func collectLiIDs(parent *xmldom.Node, out *[]string) {
	for _, li := range parent.ChildrenNamed("li") {
		pushID(out, li.Text())
	}
}

// collectDepIDs collects `<li><packageId>id</packageId>…</li>` entries
// (the modDependencies shape).
func collectDepIDs(parent *xmldom.Node, out *[]string) {
	for _, li := range parent.ChildrenNamed("li") {
		if pid := li.Child("packageId"); pid != nil {
			pushID(out, pid.Text())
			continue
		}
		// Some mods write a bare `<li>id</li>` even under modDependencies.
		if t := li.Text(); !strings.Contains(t, "\n") {
			pushID(out, t)
		}
	}
}

// collectByVersion merges every `<xByVersion><v1.6>…</v1.6></xByVersion>`
// block: which one applies depends on the running game, and being
// over-inclusive only adds ordering constraints, never removes them.
func collectByVersion(parent *xmldom.Node, out *[]string, f func(*xmldom.Node, *[]string)) {
	for _, v := range parent.Children {
		f(v, out)
	}
}

// ParseAbout parses an About.xml document. An error means the file is
// malformed or has no usable identity; callers skip the mod and log.
func ParseAbout(body string) (AboutData, error) {
	root, err := xmldom.Parse(body)
	if err != nil {
		return AboutData{}, err
	}
	var about AboutData
	for _, node := range root.Children {
		switch strings.ToLower(node.Name) {
		case "packageid":
			about.PackageID = strings.ToLower(node.Text())
		case "name":
			about.Name = node.Text()
		case "author":
			if about.Authors == "" {
				about.Authors = node.Text()
			}
		case "authors":
			var list []string
			for _, li := range node.ChildrenNamed("li") {
				if t := li.Text(); t != "" {
					list = append(list, t)
				}
			}
			if len(list) > 0 {
				about.Authors = strings.Join(list, ", ")
			}
		case "supportedversions":
			for _, li := range node.ChildrenNamed("li") {
				v := li.Text()
				if v != "" && !containsStr(about.SupportedVersions, v) {
					about.SupportedVersions = append(about.SupportedVersions, v)
				}
			}
		case "moddependencies":
			collectDepIDs(node, &about.Dependencies)
		case "moddependenciesbyversion":
			collectByVersion(node, &about.Dependencies, collectDepIDs)
		case "loadafter":
			collectLiIDs(node, &about.LoadAfter)
		case "loadafterbyversion":
			collectByVersion(node, &about.LoadAfter, collectLiIDs)
		case "loadbefore":
			collectLiIDs(node, &about.LoadBefore)
		case "loadbeforebyversion":
			collectByVersion(node, &about.LoadBefore, collectLiIDs)
		case "forceloadafter":
			collectLiIDs(node, &about.ForceLoadAfter)
		case "forceloadbefore":
			collectLiIDs(node, &about.ForceLoadBefore)
		case "incompatiblewith":
			collectLiIDs(node, &about.IncompatibleWith)
		case "incompatiblewithbyversion":
			collectByVersion(node, &about.IncompatibleWith, collectLiIDs)
		}
	}
	if about.PackageID == "" {
		return AboutData{}, errors.New("missing <packageId>")
	}
	if about.Name == "" {
		about.Name = about.PackageID
	}
	return about, nil
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
