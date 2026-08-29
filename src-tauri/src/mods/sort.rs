//! The auto-sort algorithm. Pure: reads metadata, writes nothing.

use std::cmp::Reverse;
use std::collections::{BTreeMap, BTreeSet, BinaryHeap, HashMap};
use std::path::PathBuf;

use crate::models::{ModInfo, SortResult, SortWarning};
use crate::mods::rules::RulesDb;
use crate::mods::scan;

/// Tier for Core.
const TIER_CORE: usize = 0;
/// Tier base for official expansions (offset by release order).
const TIER_EXPANSION_BASE: usize = 1;
/// Tier for mods with a community `loadTop` rule (framework mods that want to
/// sit directly behind the official content).
const TIER_TOP: usize = 100;
/// Tier for ordinary mods.
const TIER_NORMAL: usize = 1_000;
/// Tier for mods with a community `loadBottom` rule.
const TIER_BOTTOM: usize = 2_000;

fn warn(kind: &str, package_id: Option<&str>, message: String) -> SortWarning {
    SortWarning {
        kind: kind.to_string(),
        package_id: package_id.map(str::to_string),
        message,
    }
}

/// `1.6.4871 rev600` → `1.6`
pub fn major_minor(version: &str) -> Option<String> {
    let mut it = version.trim().split('.');
    let major = it.next()?.trim();
    let minor = it.next()?.trim();
    if major.is_empty() || minor.is_empty() {
        return None;
    }
    Some(format!("{major}.{minor}"))
}

fn normalize(active_ids: &[String]) -> Vec<String> {
    let mut seen = BTreeSet::new();
    active_ids
        .iter()
        .map(|id| id.trim().to_ascii_lowercase())
        .filter(|id| !id.is_empty() && seen.insert(id.clone()))
        .collect()
}

/// Core sort. `game_version` is the full version string (e.g. `1.6.4871 rev600`).
pub fn sort_with(
    active_ids: &[String],
    installed: &[ModInfo],
    rules: Option<&RulesDb>,
    game_version: Option<&str>,
) -> SortResult {
    let active = normalize(active_ids);
    let active_set: BTreeSet<&str> = active.iter().map(String::as_str).collect();

    let by_id: HashMap<&str, &ModInfo> = installed
        .iter()
        .map(|m| (m.package_id.as_str(), m))
        .collect();

    let empty_rules = RulesDb::default();
    let rules_db = rules.unwrap_or(&empty_rules);
    let rule_of = |id: &str| rules_db.rules.get(id);

    let game_mm = game_version.and_then(major_minor);

    let mut warnings: Vec<SortWarning> = Vec::new();
    if rules.is_none() {
        warnings.push(warn(
            "rulesDbUnavailable",
            None,
            "The community rules database is not cached and could not be downloaded; \
             sorting used only the mods' own About.xml metadata."
                .into(),
        ));
    }

    // ---- diagnostics -------------------------------------------------------
    let mut incompatible_pairs: BTreeSet<(String, String)> = BTreeSet::new();
    for id in &active {
        let Some(info) = by_id.get(id.as_str()) else {
            warnings.push(warn(
                "unknownMod",
                Some(id),
                format!("`{id}` is in the active list but is not installed."),
            ));
            continue;
        };

        if let (Some(mm), false) = (game_mm.as_deref(), info.supported_versions.is_empty()) {
            if !info.supported_versions.iter().any(|v| v.trim() == mm) {
                warnings.push(warn(
                    "versionMismatch",
                    Some(id),
                    format!(
                        "{} supports {} but the game is {mm}.",
                        info.name,
                        info.supported_versions.join(", ")
                    ),
                ));
            }
        }

        for dep in &info.dependencies {
            if !active_set.contains(dep.as_str()) {
                warnings.push(warn(
                    "missingDependency",
                    Some(id),
                    format!("{} requires `{dep}`, which is not active.", info.name),
                ));
            }
        }

        let declared = info.incompatible_with.iter().chain(
            rule_of(id)
                .map(|r| r.incompatible_with.iter())
                .into_iter()
                .flatten(),
        );
        for other in declared {
            if !active_set.contains(other.as_str()) || other == id {
                continue;
            }
            let pair = if id < other {
                (id.clone(), other.clone())
            } else {
                (other.clone(), id.clone())
            };
            if incompatible_pairs.insert(pair.clone()) {
                warnings.push(warn(
                    "incompatible",
                    Some(&pair.0),
                    format!("`{}` and `{}` are declared incompatible.", pair.0, pair.1),
                ));
            }
        }
    }

    // ---- tiers -------------------------------------------------------------
    let tier = |id: &str| -> usize {
        if id == scan::CORE_PACKAGE_ID {
            return TIER_CORE;
        }
        if let Some(rank) = scan::expansion_rank(id) {
            return TIER_EXPANSION_BASE + rank;
        }
        if let Some(rule) = rule_of(id) {
            // loadBottom wins if a mod somehow carries both.
            if rule.load_bottom {
                return TIER_BOTTOM;
            }
            if rule.load_top {
                return TIER_TOP;
            }
        }
        TIER_NORMAL
    };
    let sort_name = |id: &str| -> String {
        by_id
            .get(id)
            .map(|m| m.name.to_lowercase())
            .unwrap_or_else(|| id.to_string())
    };

    // ---- edges (A -> B means A loads before B) -----------------------------
    let mut edges: BTreeSet<(String, String)> = BTreeSet::new();
    let add = |a: &str, b: &str, edges: &mut BTreeSet<(String, String)>| {
        if a != b && active_set.contains(a) && active_set.contains(b) {
            edges.insert((a.to_string(), b.to_string()));
        }
    };
    for id in &active {
        if let Some(info) = by_id.get(id.as_str()) {
            for b in info.load_before.iter().chain(info.force_load_before.iter()) {
                add(id, b, &mut edges);
            }
            for a in info.load_after.iter().chain(info.force_load_after.iter()) {
                add(a, id, &mut edges);
            }
            for dep in &info.dependencies {
                add(dep, id, &mut edges);
            }
        }
        if let Some(rule) = rule_of(id) {
            for b in &rule.load_before {
                add(id, b, &mut edges);
            }
            for a in &rule.load_after {
                add(a, id, &mut edges);
            }
        }
    }

    // ---- tier propagation --------------------------------------------------
    // A few mods (Harmony, Prepatcher) legitimately declare `loadBefore` Core,
    // which blocks Core until they are placed. Without this pass, unrelated
    // ordinary mods would slip in ahead of Core while it waits. Pull every
    // ancestor down to the lowest tier it must precede, so tier order still
    // holds for everything that is not explicitly constrained.
    let mut eff: HashMap<&str, usize> = active.iter().map(|id| (id.as_str(), tier(id))).collect();
    for _ in 0..=active.len() {
        let mut changed = false;
        for (a, b) in &edges {
            let target = eff[b.as_str()];
            let source = eff.get_mut(a.as_str()).expect("edge source is active");
            if target < *source {
                *source = target;
                changed = true;
            }
        }
        if !changed {
            break;
        }
    }

    // ---- deterministic topological sort ------------------------------------
    let mut adj: BTreeMap<String, BTreeSet<String>> = BTreeMap::new();
    let mut indeg: HashMap<String, usize> = active.iter().map(|id| (id.clone(), 0)).collect();
    for (a, b) in &edges {
        if adj.entry(a.clone()).or_default().insert(b.clone()) {
            *indeg.get_mut(b).expect("edge target is active") += 1;
        }
    }

    let mut heap: BinaryHeap<Reverse<(usize, String, String)>> = BinaryHeap::new();
    let push = |id: &str, heap: &mut BinaryHeap<Reverse<(usize, String, String)>>| {
        let t = eff.get(id).copied().unwrap_or_else(|| tier(id));
        heap.push(Reverse((t, sort_name(id), id.to_string())));
    };
    for id in &active {
        if indeg[id] == 0 {
            push(id, &mut heap);
        }
    }

    let mut sorted: Vec<String> = Vec::with_capacity(active.len());
    while sorted.len() < active.len() {
        let Some(Reverse((_, _, id))) = heap.pop() else {
            // Stall: everything left is in a cycle. Drop the lexicographically
            // smallest remaining edge and carry on.
            let Some((a, b)) = adj
                .iter()
                .find(|(_, targets)| !targets.is_empty())
                .map(|(a, targets)| (a.clone(), targets.iter().next().unwrap().clone()))
            else {
                break; // no edges left but nothing ready: impossible, bail out safely
            };
            adj.get_mut(&a).unwrap().remove(&b);
            warnings.push(warn(
                "cycle",
                Some(&a),
                format!("Dependency cycle: dropped the `{a}` → `{b}` ordering rule."),
            ));
            let deg = indeg.get_mut(&b).expect("cycle edge target is active");
            *deg -= 1;
            if *deg == 0 {
                push(&b, &mut heap);
            }
            continue;
        };

        sorted.push(id.clone());
        if let Some(targets) = adj.remove(&id) {
            for t in targets {
                let deg = indeg.get_mut(&t).expect("edge target is active");
                *deg -= 1;
                if *deg == 0 {
                    push(&t, &mut heap);
                }
            }
        }
    }

    // Safety net: never lose an active mod, whatever happened above.
    for id in &active {
        if !sorted.iter().any(|s| s == id) {
            sorted.push(id.clone());
        }
    }

    SortResult { sorted, warnings }
}

/// `sort_mods` command body.
pub async fn sort_mods(active_ids: Vec<String>) -> Result<SortResult, String> {
    let paths = crate::paths::detect_paths().await?;
    let install = paths.game_install.as_ref().map(PathBuf::from);
    let workshop: Vec<PathBuf> = paths.workshop_dirs.iter().map(PathBuf::from).collect();
    let installed = scan::scan_all(install.as_deref(), &workshop);
    let rules = crate::mods::rules::rules_for_sort().await;
    Ok(sort_with(
        &active_ids,
        &installed,
        rules.as_ref(),
        paths.game_version.as_deref(),
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::models::ModSource;
    use crate::mods::rules::ModRule;

    fn m(pid: &str, name: &str) -> ModInfo {
        ModInfo {
            package_id: pid.into(),
            name: name.into(),
            authors: "t".into(),
            path: format!("/mods/{pid}"),
            source: ModSource::Workshop,
            steam_workshop_id: None,
            supported_versions: vec!["1.6".into()],
            dependencies: vec![],
            load_after: vec![],
            load_before: vec![],
            force_load_after: vec![],
            force_load_before: vec![],
            incompatible_with: vec![],
        }
    }

    fn ids(v: &[&str]) -> Vec<String> {
        v.iter().map(|s| s.to_string()).collect()
    }

    fn pos(result: &SortResult, id: &str) -> usize {
        result
            .sorted
            .iter()
            .position(|s| s == id)
            .unwrap_or_else(|| panic!("{id} missing from {:?}", result.sorted))
    }

    fn kinds<'a>(result: &'a SortResult, kind: &str) -> Vec<&'a SortWarning> {
        result.warnings.iter().filter(|w| w.kind == kind).collect()
    }

    /// Core, three expansions, and a handful of mods with real constraints.
    fn fixture() -> Vec<ModInfo> {
        let mut core = m("ludeon.rimworld", "Core");
        core.source = ModSource::Official;
        let mut royalty = m("ludeon.rimworld.royalty", "Royalty");
        royalty.source = ModSource::Official;
        let mut biotech = m("ludeon.rimworld.biotech", "Biotech");
        biotech.source = ModSource::Official;

        let harmony = m("brrainz.harmony", "Harmony");

        // Depends on Harmony, and on an id that will not be active.
        let mut zebra = m("zed.zebra", "Zebra Mod");
        zebra.dependencies = ids(&["brrainz.harmony", "missing.dep"]);

        // Same-tier peer with an alphabetically earlier name, no edges.
        let apple = m("app.apple", "Apple Mod");

        // loadBefore / forceLoadAfter pair.
        let mut early = m("x.early", "Early Bird");
        early.load_before = ids(&["x.late"]);
        let mut late = m("x.late", "Late Bloomer");
        late.force_load_after = ids(&["x.early"]);

        // Mutual incompatibility.
        let mut foo = m("con.foo", "Conflicting Foo");
        foo.incompatible_with = ids(&["con.bar"]);
        let mut bar = m("con.bar", "Conflicting Bar");
        bar.incompatible_with = ids(&["con.foo"]);

        // Out-of-date mod.
        let mut old = m("old.mod", "Ancient Mod");
        old.supported_versions = ids(&["1.4"]);

        // A two-node cycle.
        let mut cyc_a = m("cyc.alpha", "Cycle Alpha");
        cyc_a.load_before = ids(&["cyc.beta"]);
        let mut cyc_b = m("cyc.beta", "Cycle Beta");
        cyc_b.load_before = ids(&["cyc.alpha"]);

        // Gets a community loadBottom rule.
        let bottom = m("krkr.rocketman", "RocketMan");

        // Legitimately loads before Core, like the real Prepatcher.
        let mut prepatcher = m("zetrith.prepatcher", "Prepatcher");
        prepatcher.load_before = ids(&["ludeon.rimworld"]);

        // Gets a community loadTop rule. Its display name sorts last, so any
        // correct placement near the front must come from the tier, not the name.
        let top = m("imranfish.xmlextensions", "XML Extensions");

        vec![
            core, royalty, biotech, harmony, zebra, apple, early, late, foo, bar, old, cyc_a,
            cyc_b, bottom, prepatcher, top,
        ]
    }

    fn community_rules() -> RulesDb {
        let mut db = RulesDb::default();
        db.rules.insert(
            "krkr.rocketman".into(),
            ModRule {
                load_bottom: true,
                ..Default::default()
            },
        );
        db.rules.insert(
            "imranfish.xmlextensions".into(),
            ModRule {
                load_top: true,
                ..Default::default()
            },
        );
        db.rules.insert(
            "app.apple".into(),
            ModRule {
                load_after: ids(&["zed.zebra"]),
                ..Default::default()
            },
        );
        db
    }

    #[test]
    fn tiers_put_core_then_expansions_first_and_load_bottom_last() {
        let installed = fixture();
        let rules = community_rules();
        // Deliberately scrambled input order.
        let active = ids(&[
            "krkr.rocketman",
            "ludeon.rimworld.biotech",
            "app.apple",
            "ludeon.rimworld",
            "zed.zebra",
            "brrainz.harmony",
            "ludeon.rimworld.royalty",
        ]);
        let r = sort_with(&active, &installed, Some(&rules), Some("1.6.4871 rev600"));

        assert_eq!(r.sorted[0], "ludeon.rimworld");
        assert_eq!(r.sorted[1], "ludeon.rimworld.royalty");
        assert_eq!(r.sorted[2], "ludeon.rimworld.biotech");
        assert_eq!(r.sorted.last().unwrap(), "krkr.rocketman");
        assert_eq!(r.sorted.len(), active.len());
    }

    #[test]
    fn load_top_sits_behind_official_content_and_ahead_of_ordinary_mods() {
        let installed = fixture();
        let rules = community_rules();
        let active = ids(&[
            "krkr.rocketman",
            "app.apple",
            "imranfish.xmlextensions",
            "old.mod",
            "ludeon.rimworld.biotech",
            "ludeon.rimworld",
            "ludeon.rimworld.royalty",
        ]);
        let expected = ids(&[
            "ludeon.rimworld",
            "ludeon.rimworld.royalty",
            "ludeon.rimworld.biotech",
            "imranfish.xmlextensions",
            "old.mod",   // Ancient Mod
            "app.apple", // Apple Mod
            "krkr.rocketman",
        ]);
        let r = sort_with(&active, &installed, Some(&rules), None);
        assert_eq!(r.sorted, expected);

        // "XML Extensions" sorts last by name, so this ordering can only come
        // from the tier — and it must not depend on input order.
        let mut reversed = active.clone();
        reversed.reverse();
        assert_eq!(
            sort_with(&reversed, &installed, Some(&rules), None).sorted,
            expected
        );
    }

    #[test]
    fn load_top_is_ignored_without_the_community_rules_db() {
        let installed = fixture();
        let active = ids(&["imranfish.xmlextensions", "app.apple", "ludeon.rimworld"]);
        let r = sort_with(&active, &installed, None, None);
        // Falls back to name order among ordinary mods: Apple Mod < XML Extensions.
        assert_eq!(
            r.sorted,
            ids(&["ludeon.rimworld", "app.apple", "imranfish.xmlextensions"])
        );
    }

    #[test]
    fn only_explicitly_constrained_mods_may_precede_core() {
        let installed = fixture();
        let active = ids(&[
            "app.apple",
            "old.mod",
            "zetrith.prepatcher",
            "ludeon.rimworld",
        ]);
        let r = sort_with(&active, &installed, None, None);
        // Prepatcher declares loadBefore Core, so it (and only it) goes first;
        // the unconstrained mods stay behind Core rather than filling the gap.
        assert_eq!(
            r.sorted,
            ids(&[
                "zetrith.prepatcher",
                "ludeon.rimworld",
                "old.mod",
                "app.apple"
            ])
        );
    }

    #[test]
    fn honours_dependencies_load_before_and_load_after() {
        let installed = fixture();
        let active = ids(&[
            "x.late",
            "zed.zebra",
            "x.early",
            "brrainz.harmony",
            "ludeon.rimworld",
        ]);
        let r = sort_with(&active, &installed, None, Some("1.6.0"));
        assert!(pos(&r, "brrainz.harmony") < pos(&r, "zed.zebra"));
        assert!(pos(&r, "x.early") < pos(&r, "x.late"));
        assert_eq!(pos(&r, "ludeon.rimworld"), 0);
    }

    #[test]
    fn community_rules_add_edges() {
        let installed = fixture();
        let rules = community_rules();
        let active = ids(&["app.apple", "zed.zebra", "brrainz.harmony"]);
        // Without rules "Apple Mod" sorts before "Zebra Mod" by name…
        let plain = sort_with(&active, &installed, Some(&RulesDb::default()), None);
        assert!(pos(&plain, "app.apple") < pos(&plain, "zed.zebra"));
        // …with the community loadAfter rule it must come after.
        let ruled = sort_with(&active, &installed, Some(&rules), None);
        assert!(pos(&ruled, "zed.zebra") < pos(&ruled, "app.apple"));
    }

    #[test]
    fn ties_break_on_display_name_deterministically() {
        let installed = fixture();
        let active_a = ids(&["zed.zebra", "app.apple", "old.mod", "brrainz.harmony"]);
        let mut active_b = active_a.clone();
        active_b.reverse();
        let ra = sort_with(&active_a, &installed, None, None);
        let rb = sort_with(&active_b, &installed, None, None);
        assert_eq!(ra.sorted, rb.sorted, "input order must not affect output");
        // "Ancient Mod" < "Apple Mod" < "Harmony" < "Zebra Mod"
        assert_eq!(
            ra.sorted,
            ids(&["old.mod", "app.apple", "brrainz.harmony", "zed.zebra"])
        );
    }

    #[test]
    fn breaks_cycles_with_a_warning_and_keeps_every_mod() {
        let installed = fixture();
        let active = ids(&["cyc.beta", "cyc.alpha", "ludeon.rimworld"]);
        let r = sort_with(&active, &installed, None, None);
        assert_eq!(r.sorted.len(), 3);
        let cycles = kinds(&r, "cycle");
        assert_eq!(cycles.len(), 1, "warnings: {:?}", r.warnings);
        // Smallest edge is ("cyc.alpha", "cyc.beta"), so alpha's rule is dropped
        // and beta ends up first.
        assert_eq!(cycles[0].package_id.as_deref(), Some("cyc.alpha"));
        assert!(pos(&r, "cyc.beta") < pos(&r, "cyc.alpha"));
        // Deterministic across input orders.
        let mut reversed = active.clone();
        reversed.reverse();
        assert_eq!(
            sort_with(&reversed, &installed, None, None).sorted,
            r.sorted
        );
    }

    #[test]
    fn emits_the_other_four_warning_kinds() {
        let installed = fixture();
        let active = ids(&[
            "ludeon.rimworld",
            "zed.zebra",
            "brrainz.harmony",
            "con.foo",
            "con.bar",
            "old.mod",
            "not.installed",
        ]);
        let r = sort_with(&active, &installed, None, Some("1.6.4871 rev600"));

        let missing = kinds(&r, "missingDependency");
        assert_eq!(missing.len(), 1);
        assert_eq!(missing[0].package_id.as_deref(), Some("zed.zebra"));
        assert!(missing[0].message.contains("missing.dep"));

        let incompatible = kinds(&r, "incompatible");
        assert_eq!(incompatible.len(), 1, "mutual declarations report once");
        assert_eq!(incompatible[0].package_id.as_deref(), Some("con.bar"));

        let version = kinds(&r, "versionMismatch");
        assert_eq!(version.len(), 1);
        assert_eq!(version[0].package_id.as_deref(), Some("old.mod"));

        let unknown = kinds(&r, "unknownMod");
        assert_eq!(unknown.len(), 1);
        assert_eq!(unknown[0].package_id.as_deref(), Some("not.installed"));

        assert_eq!(kinds(&r, "rulesDbUnavailable").len(), 1);
        assert_eq!(r.sorted.len(), active.len());
    }

    #[test]
    fn cached_rules_suppress_the_unavailable_warning() {
        let r = sort_with(
            &ids(&["ludeon.rimworld"]),
            &fixture(),
            Some(&RulesDb::default()),
            None,
        );
        assert!(kinds(&r, "rulesDbUnavailable").is_empty());
    }

    #[test]
    fn normalizes_case_and_duplicates_in_the_input() {
        let r = sort_with(
            &ids(&["Ludeon.RimWorld", "ludeon.rimworld", " Brrainz.Harmony "]),
            &fixture(),
            None,
            None,
        );
        assert_eq!(r.sorted, ids(&["ludeon.rimworld", "brrainz.harmony"]));
    }

    #[test]
    fn extracts_major_minor() {
        assert_eq!(major_minor("1.6.4871 rev600").as_deref(), Some("1.6"));
        assert_eq!(major_minor("1.5").as_deref(), Some("1.5"));
        assert_eq!(major_minor("garbage"), None);
    }
}
