//! Discovery of installed mods: official content, local `Mods/`, Steam Workshop.

use std::path::{Path, PathBuf};

use crate::models::{ModInfo, ModSource};
use crate::mods::about::{parse_about, AboutData};

/// Core, then the expansions in release order. Used for the fixed sort tiers
/// and for `knownExpansions` in ModsConfig.xml.
pub const CORE_PACKAGE_ID: &str = "ludeon.rimworld";

/// Official expansion package ids, in release order.
pub const OFFICIAL_EXPANSIONS: &[&str] = &[
    "ludeon.rimworld.royalty",
    "ludeon.rimworld.ideology",
    "ludeon.rimworld.biotech",
    "ludeon.rimworld.anomaly",
    "ludeon.rimworld.odyssey",
];

/// Is this an official expansion (not Core)?
pub fn is_official_expansion(package_id: &str) -> bool {
    OFFICIAL_EXPANSIONS.contains(&package_id)
}

/// Release-order rank of an official expansion, if it is one.
pub fn expansion_rank(package_id: &str) -> Option<usize> {
    OFFICIAL_EXPANSIONS.iter().position(|e| *e == package_id)
}

fn about_path(mod_dir: &Path) -> PathBuf {
    mod_dir.join("About").join("About.xml")
}

fn to_mod_info(
    about: AboutData,
    dir: &Path,
    source: ModSource,
    steam_workshop_id: Option<String>,
) -> ModInfo {
    ModInfo {
        package_id: about.package_id,
        name: about.name,
        authors: about.authors,
        path: dir.to_string_lossy().to_string(),
        source,
        steam_workshop_id,
        supported_versions: about.supported_versions,
        dependencies: about.dependencies,
        load_after: about.load_after,
        load_before: about.load_before,
        force_load_after: about.force_load_after,
        force_load_before: about.force_load_before,
        incompatible_with: about.incompatible_with,
    }
}

/// Read one mod directory. `Ok(None)` = not a mod dir (no About.xml) — silent.
/// `Err` = present but unreadable/malformed — caller logs and skips.
pub fn read_mod_dir(dir: &Path, source: ModSource) -> Result<Option<ModInfo>, String> {
    let about_file = about_path(dir);
    if !about_file.is_file() {
        return Ok(None);
    }
    let raw = std::fs::read(&about_file).map_err(|e| format!("{}: {e}", about_file.display()))?;
    // About.xml is nominally utf-8; be lenient about stray bytes.
    let text = String::from_utf8_lossy(&raw);
    let about = parse_about(&text).map_err(|e| format!("{}: {e}", about_file.display()))?;

    let steam_workshop_id = if source == ModSource::Workshop {
        dir.file_name().map(|n| n.to_string_lossy().to_string())
    } else {
        None
    };
    Ok(Some(to_mod_info(about, dir, source, steam_workshop_id)))
}

/// Scan every immediate subdirectory of `parent` as a candidate mod folder.
/// Directory entries are visited in sorted order so results are deterministic.
pub fn scan_mod_container(parent: &Path, source: ModSource) -> Vec<ModInfo> {
    let entries = match std::fs::read_dir(parent) {
        Ok(e) => e,
        Err(e) => {
            eprintln!("rimforge: cannot read {}: {e}", parent.display());
            return Vec::new();
        }
    };

    let mut dirs: Vec<PathBuf> = entries
        .filter_map(|e| e.ok())
        .map(|e| e.path())
        .filter(|p| p.is_dir())
        .collect();
    dirs.sort();

    let mut out = Vec::new();
    for dir in dirs {
        match read_mod_dir(&dir, source) {
            Ok(Some(m)) => out.push(m),
            Ok(None) => {}
            Err(e) => eprintln!("rimforge: skipping malformed mod: {e}"),
        }
    }
    out
}

/// Full scan. Later sources never override earlier ones, so precedence is
/// official > local > workshop.
pub fn scan_all(game_install: Option<&Path>, workshop_dirs: &[PathBuf]) -> Vec<ModInfo> {
    let mut found: Vec<ModInfo> = Vec::new();

    if let Some(install) = game_install {
        found.extend(scan_mod_container(&install.join("Data"), ModSource::Official));
        found.extend(scan_mod_container(&install.join("Mods"), ModSource::Local));
    }
    for wd in workshop_dirs {
        found.extend(scan_mod_container(wd, ModSource::Workshop));
    }

    let mut seen: Vec<String> = Vec::new();
    let mut out: Vec<ModInfo> = Vec::new();
    for m in found {
        if seen.iter().any(|s| s == &m.package_id) {
            continue;
        }
        seen.push(m.package_id.clone());
        out.push(m);
    }
    out
}

/// Installed official expansion ids, in release order — what goes into
/// `<knownExpansions>`.
pub fn installed_expansions(mods: &[ModInfo]) -> Vec<String> {
    let mut ids: Vec<&str> = mods
        .iter()
        .filter(|m| m.source == ModSource::Official && is_official_expansion(&m.package_id))
        .map(|m| m.package_id.as_str())
        .collect();
    ids.sort_by_key(|id| expansion_rank(id).unwrap_or(usize::MAX));
    ids.into_iter().map(|s| s.to_string()).collect()
}

/// `list_installed_mods` command body.
pub async fn list_installed_mods() -> Result<Vec<ModInfo>, String> {
    let paths = crate::paths::detect_paths().await?;
    let install = paths.game_install.as_ref().map(PathBuf::from);
    let workshop: Vec<PathBuf> = paths.workshop_dirs.iter().map(PathBuf::from).collect();
    Ok(scan_all(install.as_deref(), &workshop))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn write_mod(root: &Path, name: &str, xml: &str) -> PathBuf {
        let dir = root.join(name);
        std::fs::create_dir_all(dir.join("About")).unwrap();
        std::fs::write(dir.join("About").join("About.xml"), xml).unwrap();
        dir
    }

    fn about_xml(pid: &str, name: &str) -> String {
        format!("<ModMetaData><packageId>{pid}</packageId><name>{name}</name></ModMetaData>")
    }

    fn temp_root(tag: &str) -> PathBuf {
        let dir = std::env::temp_dir().join(format!("rimforge-scan-{tag}-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(&dir).unwrap();
        dir
    }

    #[test]
    fn scans_all_three_sources_with_precedence_and_workshop_ids() {
        let root = temp_root("sources");
        let install = root.join("RimWorld");
        write_mod(&install.join("Data"), "Core", &about_xml("Ludeon.RimWorld", "Core"));
        write_mod(
            &install.join("Data"),
            "Biotech",
            &about_xml("Ludeon.RimWorld.Biotech", "Biotech"),
        );
        write_mod(&install.join("Mods"), "MyMod", &about_xml("Me.Mine", "Mine"));
        // Same id as the local mod: local wins, workshop copy is dropped.
        let ws = root.join("294100");
        write_mod(&ws, "123456", &about_xml("Me.Mine", "Mine (Workshop)"));
        write_mod(&ws, "987654", &about_xml("Other.Mod", "Other"));
        // Not a mod: no About.xml. Must be skipped silently.
        std::fs::create_dir_all(ws.join("nonsense")).unwrap();
        // Malformed: must be skipped, not fatal.
        write_mod(&ws, "555", "<ModMetaData><name>broken</ModMetaData>");

        let mods = scan_all(Some(&install), &[ws]);
        let ids: Vec<&str> = mods.iter().map(|m| m.package_id.as_str()).collect();
        assert_eq!(
            ids,
            vec![
                "ludeon.rimworld.biotech",
                "ludeon.rimworld",
                "me.mine",
                "other.mod"
            ]
        );

        let mine = mods.iter().find(|m| m.package_id == "me.mine").unwrap();
        assert_eq!(mine.source, ModSource::Local);
        assert_eq!(mine.name, "Mine");
        assert_eq!(mine.steam_workshop_id, None);

        let other = mods.iter().find(|m| m.package_id == "other.mod").unwrap();
        assert_eq!(other.source, ModSource::Workshop);
        assert_eq!(other.steam_workshop_id.as_deref(), Some("987654"));

        assert_eq!(installed_expansions(&mods), vec!["ludeon.rimworld.biotech"]);
        std::fs::remove_dir_all(&root).unwrap();
    }

    #[test]
    fn missing_directories_are_not_fatal() {
        let mods = scan_all(Some(Path::new("/definitely/not/here")), &[]);
        assert!(mods.is_empty());
    }

    /// Real-machine scan. Run with:
    /// `RIMFORGE_TEST_GAME_INSTALL=... RIMFORGE_TEST_WORKSHOP=... cargo test -- --ignored --nocapture`
    #[test]
    #[ignore = "requires a real RimWorld install; set RIMFORGE_TEST_* env vars"]
    fn real_install_scan() {
        let install = std::env::var("RIMFORGE_TEST_GAME_INSTALL").ok().map(PathBuf::from);
        let workshop: Vec<PathBuf> = std::env::var("RIMFORGE_TEST_WORKSHOP")
            .unwrap_or_default()
            .split(':')
            .filter(|s| !s.is_empty())
            .map(PathBuf::from)
            .collect();

        let start = std::time::Instant::now();
        let mods = scan_all(install.as_deref(), &workshop);
        let elapsed = start.elapsed();

        let official = mods.iter().filter(|m| m.source == ModSource::Official).count();
        let local = mods.iter().filter(|m| m.source == ModSource::Local).count();
        let ws = mods.iter().filter(|m| m.source == ModSource::Workshop).count();
        println!(
            "scanned {} mods in {:?} (official {official}, local {local}, workshop {ws})",
            mods.len(),
            elapsed
        );
        println!("expansions: {:?}", installed_expansions(&mods));
        assert!(!mods.is_empty(), "expected to find mods");
    }
}
