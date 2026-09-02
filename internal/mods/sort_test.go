package mods

import (
	"reflect"
	"testing"

	"rimforge/internal/models"
)

func m(pid, name string) models.ModInfo {
	return models.ModInfo{
		PackageID:         pid,
		Name:              name,
		Authors:           "t",
		Path:              "/mods/" + pid,
		Source:            models.SourceWorkshop,
		SupportedVersions: []string{"1.6"},
	}
}

func pos(t *testing.T, r models.SortResult, id string) int {
	t.Helper()
	for i, s := range r.Sorted {
		if s == id {
			return i
		}
	}
	t.Fatalf("%s missing from %v", id, r.Sorted)
	return -1
}

func kinds(r models.SortResult, kind string) []models.SortWarning {
	var out []models.SortWarning
	for _, w := range r.Warnings {
		if w.Kind == kind {
			out = append(out, w)
		}
	}
	return out
}

func reversed(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[len(ids)-1-i] = id
	}
	return out
}

// Core, three expansions, and a handful of mods with real constraints.
func fixture() []models.ModInfo {
	core := m("ludeon.rimworld", "Core")
	core.Source = models.SourceOfficial
	royalty := m("ludeon.rimworld.royalty", "Royalty")
	royalty.Source = models.SourceOfficial
	biotech := m("ludeon.rimworld.biotech", "Biotech")
	biotech.Source = models.SourceOfficial

	harmony := m("brrainz.harmony", "Harmony")

	// Depends on Harmony, and on an id that will not be active.
	zebra := m("zed.zebra", "Zebra Mod")
	zebra.Dependencies = []string{"brrainz.harmony", "missing.dep"}

	// Same-tier peer with an alphabetically earlier name, no edges.
	apple := m("app.apple", "Apple Mod")

	// loadBefore / forceLoadAfter pair.
	early := m("x.early", "Early Bird")
	early.LoadBefore = []string{"x.late"}
	late := m("x.late", "Late Bloomer")
	late.ForceLoadAfter = []string{"x.early"}

	// Mutual incompatibility.
	foo := m("con.foo", "Conflicting Foo")
	foo.IncompatibleWith = []string{"con.bar"}
	bar := m("con.bar", "Conflicting Bar")
	bar.IncompatibleWith = []string{"con.foo"}

	// Out-of-date mod.
	old := m("old.mod", "Ancient Mod")
	old.SupportedVersions = []string{"1.4"}

	// A two-node cycle.
	cycA := m("cyc.alpha", "Cycle Alpha")
	cycA.LoadBefore = []string{"cyc.beta"}
	cycB := m("cyc.beta", "Cycle Beta")
	cycB.LoadBefore = []string{"cyc.alpha"}

	// Gets a community loadBottom rule.
	bottom := m("krkr.rocketman", "RocketMan")

	// Legitimately loads before Core, like the real Prepatcher.
	prepatcher := m("zetrith.prepatcher", "Prepatcher")
	prepatcher.LoadBefore = []string{"ludeon.rimworld"}

	// Gets a community loadTop rule. Its display name sorts last, so any
	// correct placement near the front must come from the tier, not the name.
	top := m("imranfish.xmlextensions", "XML Extensions")

	return []models.ModInfo{core, royalty, biotech, harmony, zebra, apple, early, late, foo, bar, old, cycA, cycB, bottom, prepatcher, top}
}

func communityRules() *RulesDb {
	return &RulesDb{Rules: map[string]ModRule{
		"krkr.rocketman":          {LoadBottom: true},
		"imranfish.xmlextensions": {LoadTop: true},
		"app.apple":               {LoadAfter: []string{"zed.zebra"}},
	}}
}

func emptyRules() *RulesDb { return &RulesDb{Rules: map[string]ModRule{}} }

func TestTiersPutCoreThenExpansionsFirstAndLoadBottomLast(t *testing.T) {
	// Deliberately scrambled input order.
	active := []string{"krkr.rocketman", "ludeon.rimworld.biotech", "app.apple", "ludeon.rimworld", "zed.zebra", "brrainz.harmony", "ludeon.rimworld.royalty"}
	r := SortWith(active, fixture(), communityRules(), "1.6.4871 rev600")
	if r.Sorted[0] != "ludeon.rimworld" || r.Sorted[1] != "ludeon.rimworld.royalty" || r.Sorted[2] != "ludeon.rimworld.biotech" {
		t.Fatalf("got %v", r.Sorted)
	}
	if r.Sorted[len(r.Sorted)-1] != "krkr.rocketman" || len(r.Sorted) != len(active) {
		t.Fatalf("got %v", r.Sorted)
	}
}

func TestLoadTopSitsBehindOfficialContentAndAheadOfOrdinaryMods(t *testing.T) {
	active := []string{"krkr.rocketman", "app.apple", "imranfish.xmlextensions", "old.mod", "ludeon.rimworld.biotech", "ludeon.rimworld", "ludeon.rimworld.royalty"}
	expected := []string{
		"ludeon.rimworld",
		"ludeon.rimworld.royalty",
		"ludeon.rimworld.biotech",
		"imranfish.xmlextensions",
		"old.mod",   // Ancient Mod
		"app.apple", // Apple Mod
		"krkr.rocketman",
	}
	r := SortWith(active, fixture(), communityRules(), "")
	if !reflect.DeepEqual(r.Sorted, expected) {
		t.Fatalf("got %v", r.Sorted)
	}
	// "XML Extensions" sorts last by name, so this ordering can only come
	// from the tier — and it must not depend on input order.
	if got := SortWith(reversed(active), fixture(), communityRules(), "").Sorted; !reflect.DeepEqual(got, expected) {
		t.Fatalf("reversed input: got %v", got)
	}
}

func TestLoadTopIsIgnoredWithoutTheCommunityRulesDb(t *testing.T) {
	r := SortWith([]string{"imranfish.xmlextensions", "app.apple", "ludeon.rimworld"}, fixture(), nil, "")
	// Falls back to name order among ordinary mods: Apple Mod < XML Extensions.
	if want := []string{"ludeon.rimworld", "app.apple", "imranfish.xmlextensions"}; !reflect.DeepEqual(r.Sorted, want) {
		t.Fatalf("got %v", r.Sorted)
	}
}

func TestOnlyExplicitlyConstrainedModsMayPrecedeCore(t *testing.T) {
	r := SortWith([]string{"app.apple", "old.mod", "zetrith.prepatcher", "ludeon.rimworld"}, fixture(), nil, "")
	// Prepatcher declares loadBefore Core, so it (and only it) goes first;
	// the unconstrained mods stay behind Core rather than filling the gap.
	if want := []string{"zetrith.prepatcher", "ludeon.rimworld", "old.mod", "app.apple"}; !reflect.DeepEqual(r.Sorted, want) {
		t.Fatalf("got %v", r.Sorted)
	}
}

func TestHonoursDependenciesLoadBeforeAndLoadAfter(t *testing.T) {
	r := SortWith([]string{"x.late", "zed.zebra", "x.early", "brrainz.harmony", "ludeon.rimworld"}, fixture(), nil, "1.6.0")
	if !(pos(t, r, "brrainz.harmony") < pos(t, r, "zed.zebra")) {
		t.Fatalf("dependency order wrong: %v", r.Sorted)
	}
	if !(pos(t, r, "x.early") < pos(t, r, "x.late")) {
		t.Fatalf("loadBefore order wrong: %v", r.Sorted)
	}
	if pos(t, r, "ludeon.rimworld") != 0 {
		t.Fatalf("core should be first: %v", r.Sorted)
	}
}

func TestCommunityRulesAddEdges(t *testing.T) {
	active := []string{"app.apple", "zed.zebra", "brrainz.harmony"}
	// Without rules "Apple Mod" sorts before "Zebra Mod" by name…
	plain := SortWith(active, fixture(), emptyRules(), "")
	if !(pos(t, plain, "app.apple") < pos(t, plain, "zed.zebra")) {
		t.Fatalf("got %v", plain.Sorted)
	}
	// …with the community loadAfter rule it must come after.
	ruled := SortWith(active, fixture(), communityRules(), "")
	if !(pos(t, ruled, "zed.zebra") < pos(t, ruled, "app.apple")) {
		t.Fatalf("got %v", ruled.Sorted)
	}
}

func TestTiesBreakOnDisplayNameDeterministically(t *testing.T) {
	a := []string{"zed.zebra", "app.apple", "old.mod", "brrainz.harmony"}
	ra := SortWith(a, fixture(), nil, "")
	rb := SortWith(reversed(a), fixture(), nil, "")
	if !reflect.DeepEqual(ra.Sorted, rb.Sorted) {
		t.Fatalf("input order must not affect output: %v vs %v", ra.Sorted, rb.Sorted)
	}
	// "Ancient Mod" < "Apple Mod" < "Harmony" < "Zebra Mod"
	if want := []string{"old.mod", "app.apple", "brrainz.harmony", "zed.zebra"}; !reflect.DeepEqual(ra.Sorted, want) {
		t.Fatalf("got %v", ra.Sorted)
	}
}

func TestBreaksCyclesWithAWarningAndKeepsEveryMod(t *testing.T) {
	active := []string{"cyc.beta", "cyc.alpha", "ludeon.rimworld"}
	r := SortWith(active, fixture(), nil, "")
	if len(r.Sorted) != 3 {
		t.Fatalf("got %v", r.Sorted)
	}
	cycles := kinds(r, "cycle")
	if len(cycles) != 1 {
		t.Fatalf("warnings: %+v", r.Warnings)
	}
	// Smallest edge is ("cyc.alpha", "cyc.beta"), so alpha's rule is dropped
	// and beta ends up first.
	if models.Deref(cycles[0].PackageID) != "cyc.alpha" {
		t.Fatalf("got %+v", cycles[0])
	}
	if !(pos(t, r, "cyc.beta") < pos(t, r, "cyc.alpha")) {
		t.Fatalf("got %v", r.Sorted)
	}
	// Deterministic across input orders.
	if got := SortWith(reversed(active), fixture(), nil, "").Sorted; !reflect.DeepEqual(got, r.Sorted) {
		t.Fatalf("reversed input: got %v", got)
	}
}

func TestEmitsTheOtherFourWarningKinds(t *testing.T) {
	active := []string{"ludeon.rimworld", "zed.zebra", "brrainz.harmony", "con.foo", "con.bar", "old.mod", "not.installed"}
	r := SortWith(active, fixture(), nil, "1.6.4871 rev600")

	missing := kinds(r, "missingDependency")
	if len(missing) != 1 || models.Deref(missing[0].PackageID) != "zed.zebra" || !contains(missing[0].Message, "missing.dep") {
		t.Fatalf("got %+v", missing)
	}
	incompatible := kinds(r, "incompatible")
	if len(incompatible) != 1 || models.Deref(incompatible[0].PackageID) != "con.bar" {
		t.Fatalf("mutual declarations report once: %+v", incompatible)
	}
	version := kinds(r, "versionMismatch")
	if len(version) != 1 || models.Deref(version[0].PackageID) != "old.mod" {
		t.Fatalf("got %+v", version)
	}
	unknown := kinds(r, "unknownMod")
	if len(unknown) != 1 || models.Deref(unknown[0].PackageID) != "not.installed" {
		t.Fatalf("got %+v", unknown)
	}
	if len(kinds(r, "rulesDbUnavailable")) != 1 {
		t.Fatalf("got %+v", r.Warnings)
	}
	if len(r.Sorted) != len(active) {
		t.Fatalf("got %v", r.Sorted)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestCachedRulesSuppressTheUnavailableWarning(t *testing.T) {
	r := SortWith([]string{"ludeon.rimworld"}, fixture(), emptyRules(), "")
	if len(kinds(r, "rulesDbUnavailable")) != 0 {
		t.Fatalf("got %+v", r.Warnings)
	}
	if r.Warnings == nil {
		t.Fatal("warnings must serialise as [], never null")
	}
}

func TestNormalizesCaseAndDuplicatesInTheInput(t *testing.T) {
	r := SortWith([]string{"Ludeon.RimWorld", "ludeon.rimworld", " Brrainz.Harmony "}, fixture(), nil, "")
	if want := []string{"ludeon.rimworld", "brrainz.harmony"}; !reflect.DeepEqual(r.Sorted, want) {
		t.Fatalf("got %v", r.Sorted)
	}
}

func TestExtractsMajorMinor(t *testing.T) {
	if v, ok := MajorMinor("1.6.4871 rev600"); !ok || v != "1.6" {
		t.Fatal(v, ok)
	}
	if v, ok := MajorMinor("1.5"); !ok || v != "1.5" {
		t.Fatal(v, ok)
	}
	if _, ok := MajorMinor("garbage"); ok {
		t.Fatal("garbage should fail")
	}
}
