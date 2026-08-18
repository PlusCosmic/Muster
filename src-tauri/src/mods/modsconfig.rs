//! Reading and writing a profile's `Config/ModsConfig.xml`.

use std::path::{Path, PathBuf};

use roxmltree::{Document, Node};

use crate::models::ActiveModList;
use crate::mods::scan;

/// Location of the active-mod list inside a profile's savedatafolder.
pub fn mods_config_path(profile_dir: &Path) -> PathBuf {
    profile_dir.join("Config").join("ModsConfig.xml")
}

fn tag_eq(node: &Node, name: &str) -> bool {
    node.is_element() && node.tag_name().name().eq_ignore_ascii_case(name)
}

fn li_values(parent: Node) -> Vec<String> {
    parent
        .children()
        .filter(|c| tag_eq(c, "li"))
        .map(|li| li.text().unwrap_or("").trim().to_ascii_lowercase())
        .filter(|s| !s.is_empty())
        .collect()
}

/// Parse a ModsConfig.xml document.
pub fn parse_mods_config(xml: &str) -> Result<ActiveModList, String> {
    let cleaned = xml.trim_start_matches('\u{feff}').trim_start();
    let doc = Document::parse(cleaned).map_err(|e| format!("ModsConfig.xml parse error: {e}"))?;
    let root = doc.root_element();

    let mut list = ActiveModList {
        active_ids: Vec::new(),
        known_expansions: Vec::new(),
        version: None,
    };

    for node in root.children().filter(|c| c.is_element()) {
        match node.tag_name().name().to_ascii_lowercase().as_str() {
            "version" => {
                let v = node.text().unwrap_or("").trim().to_string();
                if !v.is_empty() {
                    list.version = Some(v);
                }
            }
            "activemods" => list.active_ids = li_values(node),
            "knownexpansions" => list.known_expansions = li_values(node),
            _ => {}
        }
    }

    // De-duplicate while preserving order; RimWorld tolerates dupes, we don't.
    let mut seen: Vec<String> = Vec::new();
    list.active_ids.retain(|id| {
        if seen.iter().any(|s| s == id) {
            false
        } else {
            seen.push(id.clone());
            true
        }
    });

    Ok(list)
}

fn escape(text: &str) -> String {
    let mut out = String::with_capacity(text.len());
    for c in text.chars() {
        match c {
            '&' => out.push_str("&amp;"),
            '<' => out.push_str("&lt;"),
            '>' => out.push_str("&gt;"),
            '"' => out.push_str("&quot;"),
            '\'' => out.push_str("&apos;"),
            _ => out.push(c),
        }
    }
    out
}

fn push_list(out: &mut String, tag: &str, items: &[String]) {
    if items.is_empty() {
        out.push_str(&format!("  <{tag} />\n"));
        return;
    }
    out.push_str(&format!("  <{tag}>\n"));
    for item in items {
        out.push_str(&format!("    <li>{}</li>\n", escape(item)));
    }
    out.push_str(&format!("  </{tag}>\n"));
}

/// Render a ModsConfig.xml document: utf-8 declaration, 2-space indent,
/// escaped text.
pub fn render_mods_config(list: &ActiveModList) -> String {
    let mut out = String::from("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<ModsConfigData>\n");
    if let Some(v) = &list.version {
        out.push_str(&format!("  <version>{}</version>\n", escape(v)));
    }
    push_list(&mut out, "activeMods", &list.active_ids);
    push_list(&mut out, "knownExpansions", &list.known_expansions);
    out.push_str("</ModsConfigData>\n");
    out
}

/// Read the list from disk. A missing file yields the Core-only default.
pub fn read_active(path: &Path) -> Result<Option<ActiveModList>, String> {
    if !path.is_file() {
        return Ok(None);
    }
    let raw = std::fs::read(path).map_err(|e| format!("{}: {e}", path.display()))?;
    let text = String::from_utf8_lossy(&raw);
    parse_mods_config(&text).map(Some)
}

/// Write the list, creating `Config/` if needed.
pub fn write_active(path: &Path, list: &ActiveModList) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent).map_err(|e| format!("{}: {e}", parent.display()))?;
    }
    std::fs::write(path, render_mods_config(list)).map_err(|e| format!("{}: {e}", path.display()))
}

/// Best-effort context from the environment: game version and the official
/// expansions that are actually installed. Never fails the caller.
async fn environment() -> (Option<String>, Vec<String>) {
    match crate::paths::detect_paths().await {
        Ok(paths) => {
            let install = paths.game_install.as_ref().map(PathBuf::from);
            let workshop: Vec<PathBuf> = paths.workshop_dirs.iter().map(PathBuf::from).collect();
            let mods = scan::scan_all(install.as_deref(), &workshop);
            (paths.game_version, scan::installed_expansions(&mods))
        }
        Err(e) => {
            eprintln!("rimforge: path detection unavailable ({e}); writing ModsConfig without environment data");
            (None, Vec::new())
        }
    }
}

/// `get_active_mods` command body.
pub async fn get_active_mods(profile_id: String) -> Result<ActiveModList, String> {
    let path = mods_config_path(&crate::profiles::profile_dir(&profile_id));
    match read_active(&path)? {
        Some(list) => Ok(list),
        None => {
            let (version, known_expansions) = environment().await;
            Ok(ActiveModList {
                active_ids: vec![scan::CORE_PACKAGE_ID.to_string()],
                known_expansions,
                version,
            })
        }
    }
}

/// `set_active_mods` command body. Preserves the existing `<version>` if the
/// file already exists; `knownExpansions` is always the installed official set.
pub async fn set_active_mods(profile_id: String, active_ids: Vec<String>) -> Result<(), String> {
    let path = mods_config_path(&crate::profiles::profile_dir(&profile_id));
    let existing = read_active(&path)?;
    let (detected_version, mut known_expansions) = environment().await;

    if known_expansions.is_empty() {
        // Detection failed — don't clobber what the profile already knew.
        if let Some(prev) = &existing {
            known_expansions = prev.known_expansions.clone();
        }
    }

    let version = existing
        .as_ref()
        .and_then(|p| p.version.clone())
        .or(detected_version);

    let mut seen: Vec<String> = Vec::new();
    let normalized: Vec<String> = active_ids
        .into_iter()
        .map(|id| id.trim().to_ascii_lowercase())
        .filter(|id| {
            if id.is_empty() || seen.iter().any(|s| s == id) {
                false
            } else {
                seen.push(id.clone());
                true
            }
        })
        .collect();

    write_active(
        &path,
        &ActiveModList {
            active_ids: normalized,
            known_expansions,
            version,
        },
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    const SAMPLE: &str = "\u{feff}<?xml version=\"1.0\" encoding=\"utf-8\"?>
<ModsConfigData>
  <version>1.6.4871 rev600</version>
  <activeMods>
    <li>Ludeon.RimWorld</li>
    <li>brrainz.harmony</li>
    <li>brrainz.harmony</li>
  </activeMods>
  <knownExpansions>
    <li>Ludeon.RimWorld.Biotech</li>
  </knownExpansions>
</ModsConfigData>";

    #[test]
    fn parses_lowercasing_and_deduping() {
        let list = parse_mods_config(SAMPLE).unwrap();
        assert_eq!(list.version.as_deref(), Some("1.6.4871 rev600"));
        assert_eq!(list.active_ids, vec!["ludeon.rimworld", "brrainz.harmony"]);
        assert_eq!(list.known_expansions, vec!["ludeon.rimworld.biotech"]);
    }

    #[test]
    fn round_trips_through_render_and_parse() {
        let original = parse_mods_config(SAMPLE).unwrap();
        let rendered = render_mods_config(&original);
        assert!(rendered.starts_with("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n"));
        assert!(rendered.contains("\n  <activeMods>\n    <li>ludeon.rimworld</li>\n"));
        let again = parse_mods_config(&rendered).unwrap();
        assert_eq!(again.active_ids, original.active_ids);
        assert_eq!(again.known_expansions, original.known_expansions);
        assert_eq!(again.version, original.version);
    }

    #[test]
    fn escapes_text_and_renders_empty_lists_self_closing() {
        let list = ActiveModList {
            active_ids: vec!["a&b<c>".into()],
            known_expansions: vec![],
            version: Some("1.6 \"rev\"".into()),
        };
        let xml = render_mods_config(&list);
        assert!(xml.contains("<li>a&amp;b&lt;c&gt;</li>"));
        assert!(xml.contains("<version>1.6 &quot;rev&quot;</version>"));
        assert!(xml.contains("<knownExpansions />"));
        // Still well-formed after escaping.
        let back = parse_mods_config(&xml).unwrap();
        assert_eq!(back.active_ids, vec!["a&b<c>"]);
    }

    #[test]
    fn omits_version_when_unknown() {
        let list = ActiveModList {
            active_ids: vec!["ludeon.rimworld".into()],
            known_expansions: vec![],
            version: None,
        };
        let xml = render_mods_config(&list);
        assert!(!xml.contains("<version"));
        assert!(parse_mods_config(&xml).unwrap().version.is_none());
    }

    #[test]
    fn writes_and_reads_back_from_disk() {
        let dir = std::env::temp_dir()
            .join(format!("rimforge-modsconfig-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        let path = mods_config_path(&dir);
        assert!(read_active(&path).unwrap().is_none());

        let list = ActiveModList {
            active_ids: vec!["ludeon.rimworld".into(), "brrainz.harmony".into()],
            known_expansions: vec!["ludeon.rimworld.odyssey".into()],
            version: Some("1.6.4871 rev600".into()),
        };
        write_active(&path, &list).unwrap();
        let back = read_active(&path).unwrap().unwrap();
        assert_eq!(back.active_ids, list.active_ids);
        assert_eq!(back.known_expansions, list.known_expansions);
        assert_eq!(back.version, list.version);
        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn malformed_config_is_an_error() {
        assert!(parse_mods_config("<ModsConfigData><activeMods>").is_err());
    }
}
