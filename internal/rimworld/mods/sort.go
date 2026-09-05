package mods

import (
	"container/heap"
	"fmt"
	"sort"
	"strings"

	core "muster/internal/models"
	"muster/internal/rimworld/models"
	"muster/internal/rimworld/paths"
)

// The auto-sort algorithm. Pure: reads metadata, writes nothing.

const (
	tierCore          = 0     // Core.
	tierExpansionBase = 1     // Official expansions, offset by release order.
	tierTop           = 100   // Community `loadTop` (frameworks that sit right behind official content).
	tierNormal        = 1_000 // Ordinary mods.
	tierBottom        = 2_000 // Community `loadBottom`.
)

func warn(kind, packageID, message string) models.SortWarning {
	return models.SortWarning{Kind: kind, PackageID: core.Str(packageID), Message: message}
}

// MajorMinor turns `1.6.4871 rev600` into `1.6`.
func MajorMinor(version string) (string, bool) {
	return paths.MajorMinor(version)
}

func normalize(activeIDs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range activeIDs {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

type edge struct{ a, b string }

// readyItem orders the topological frontier: tier, then display name, then
// id, so output is deterministic and independent of input order.
type readyItem struct {
	tier int
	name string
	id   string
}

type readyHeap []readyItem

func (h readyHeap) Len() int { return len(h) }
func (h readyHeap) Less(i, j int) bool {
	if h[i].tier != h[j].tier {
		return h[i].tier < h[j].tier
	}
	if h[i].name != h[j].name {
		return h[i].name < h[j].name
	}
	return h[i].id < h[j].id
}
func (h readyHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *readyHeap) Push(x any)   { *h = append(*h, x.(readyItem)) }
func (h *readyHeap) Pop() any     { old := *h; x := old[len(old)-1]; *h = old[:len(old)-1]; return x }

// SortWith is the core sort. rules may be nil (unavailable); gameVersion is
// the full version string (e.g. `1.6.4871 rev600`) or "".
func SortWith(activeIDs []string, installed []models.ModInfo, rules *RulesDb, gameVersion string) models.SortResult {
	active := normalize(activeIDs)
	activeSet := map[string]bool{}
	for _, id := range active {
		activeSet[id] = true
	}

	byID := map[string]*models.ModInfo{}
	for i := range installed {
		byID[installed[i].PackageID] = &installed[i]
	}

	ruleOf := func(id string) (ModRule, bool) {
		if rules == nil {
			return ModRule{}, false
		}
		r, ok := rules.Rules[id]
		return r, ok
	}

	gameMM, _ := MajorMinor(gameVersion)

	warnings := []models.SortWarning{}
	if rules == nil {
		warnings = append(warnings, models.SortWarning{
			Kind: "rulesDbUnavailable",
			Message: "The community rules database is not cached and could not be downloaded; " +
				"sorting used only the mods' own About.xml metadata.",
		})
	}

	// ---- diagnostics -------------------------------------------------------
	incompatiblePairs := map[edge]bool{}
	for _, id := range active {
		info, ok := byID[id]
		if !ok {
			warnings = append(warnings, warn("unknownMod", id,
				fmt.Sprintf("`%s` is in the active list but is not installed.", id)))
			continue
		}

		if gameMM != "" && len(info.SupportedVersions) > 0 {
			supported := false
			for _, v := range info.SupportedVersions {
				if strings.TrimSpace(v) == gameMM {
					supported = true
					break
				}
			}
			if !supported {
				warnings = append(warnings, warn("versionMismatch", id,
					fmt.Sprintf("%s supports %s but the game is %s.", info.Name, strings.Join(info.SupportedVersions, ", "), gameMM)))
			}
		}

		for _, dep := range info.Dependencies {
			if !activeSet[dep] {
				warnings = append(warnings, warn("missingDependency", id,
					fmt.Sprintf("%s requires `%s`, which is not active.", info.Name, dep)))
			}
		}

		declared := append([]string{}, info.IncompatibleWith...)
		if r, ok := ruleOf(id); ok {
			declared = append(declared, r.IncompatibleWith...)
		}
		for _, other := range declared {
			if !activeSet[other] || other == id {
				continue
			}
			pair := edge{id, other}
			if other < id {
				pair = edge{other, id}
			}
			if !incompatiblePairs[pair] {
				incompatiblePairs[pair] = true
				warnings = append(warnings, warn("incompatible", pair.a,
					fmt.Sprintf("`%s` and `%s` are declared incompatible.", pair.a, pair.b)))
			}
		}
	}

	// ---- tiers -------------------------------------------------------------
	tier := func(id string) int {
		if id == CorePackageID {
			return tierCore
		}
		if rank := ExpansionRank(id); rank >= 0 {
			return tierExpansionBase + rank
		}
		if r, ok := ruleOf(id); ok {
			// loadBottom wins if a mod somehow carries both.
			if r.LoadBottom {
				return tierBottom
			}
			if r.LoadTop {
				return tierTop
			}
		}
		return tierNormal
	}
	sortName := func(id string) string {
		if m, ok := byID[id]; ok {
			return strings.ToLower(m.Name)
		}
		return id
	}

	// ---- edges (A -> B means A loads before B) -----------------------------
	edgeSet := map[edge]bool{}
	add := func(a, b string) {
		if a != b && activeSet[a] && activeSet[b] {
			edgeSet[edge{a, b}] = true
		}
	}
	for _, id := range active {
		if info, ok := byID[id]; ok {
			for _, b := range info.LoadBefore {
				add(id, b)
			}
			for _, b := range info.ForceLoadBefore {
				add(id, b)
			}
			for _, a := range info.LoadAfter {
				add(a, id)
			}
			for _, a := range info.ForceLoadAfter {
				add(a, id)
			}
			for _, dep := range info.Dependencies {
				add(dep, id)
			}
		}
		if r, ok := ruleOf(id); ok {
			for _, b := range r.LoadBefore {
				add(id, b)
			}
			for _, a := range r.LoadAfter {
				add(a, id)
			}
		}
	}
	edges := make([]edge, 0, len(edgeSet))
	for e := range edgeSet {
		edges = append(edges, e)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].a != edges[j].a {
			return edges[i].a < edges[j].a
		}
		return edges[i].b < edges[j].b
	})

	// ---- tier propagation --------------------------------------------------
	// A few mods (Harmony, Prepatcher) legitimately declare `loadBefore` Core,
	// which blocks Core until they are placed. Without this pass, unrelated
	// ordinary mods would slip in ahead of Core while it waits. Pull every
	// ancestor down to the lowest tier it must precede, so tier order still
	// holds for everything that is not explicitly constrained.
	eff := map[string]int{}
	for _, id := range active {
		eff[id] = tier(id)
	}
	for i := 0; i <= len(active); i++ {
		changed := false
		for _, e := range edges {
			if eff[e.b] < eff[e.a] {
				eff[e.a] = eff[e.b]
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	// ---- deterministic topological sort ------------------------------------
	adj := map[string]map[string]bool{}
	indeg := map[string]int{}
	for _, id := range active {
		indeg[id] = 0
	}
	for _, e := range edges {
		if adj[e.a] == nil {
			adj[e.a] = map[string]bool{}
		}
		if !adj[e.a][e.b] {
			adj[e.a][e.b] = true
			indeg[e.b]++
		}
	}

	ready := &readyHeap{}
	push := func(id string) {
		heap.Push(ready, readyItem{tier: eff[id], name: sortName(id), id: id})
	}
	for _, id := range active {
		if indeg[id] == 0 {
			push(id)
		}
	}

	sorted := make([]string, 0, len(active))
	for len(sorted) < len(active) {
		if ready.Len() == 0 {
			// Stall: everything left is in a cycle. Drop the lexicographically
			// smallest remaining edge and carry on.
			a, b, ok := smallestEdge(adj)
			if !ok {
				break // no edges left but nothing ready: impossible, bail out safely
			}
			delete(adj[a], b)
			warnings = append(warnings, warn("cycle", a,
				fmt.Sprintf("Dependency cycle: dropped the `%s` → `%s` ordering rule.", a, b)))
			indeg[b]--
			if indeg[b] == 0 {
				push(b)
			}
			continue
		}

		id := heap.Pop(ready).(readyItem).id
		sorted = append(sorted, id)
		targets := adj[id]
		delete(adj, id)
		for t := range targets {
			indeg[t]--
			if indeg[t] == 0 {
				push(t)
			}
		}
	}

	// Safety net: never lose an active mod, whatever happened above.
	placed := map[string]bool{}
	for _, id := range sorted {
		placed[id] = true
	}
	for _, id := range active {
		if !placed[id] {
			sorted = append(sorted, id)
		}
	}

	return models.SortResult{Sorted: sorted, Warnings: warnings}
}

// smallestEdge finds the lexicographically smallest (a, b) still in adj.
func smallestEdge(adj map[string]map[string]bool) (string, string, bool) {
	sources := make([]string, 0, len(adj))
	for a, targets := range adj {
		if len(targets) > 0 {
			sources = append(sources, a)
		}
	}
	if len(sources) == 0 {
		return "", "", false
	}
	sort.Strings(sources)
	a := sources[0]
	b := ""
	for t := range adj[a] {
		if b == "" || t < b {
			b = t
		}
	}
	return a, b, true
}

// Sort is the `sort_mods` command body.
func Sort(activeIDs []string) (models.SortResult, error) {
	p := paths.Detect()
	installed := ScanAll(core.Deref(p.GameInstall), p.WorkshopDirs)
	rules := RulesForSort()
	return SortWith(activeIDs, installed, rules, core.Deref(p.GameVersion)), nil
}
