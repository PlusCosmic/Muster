//! Profile registry (`registry.json`) and the profile directories themselves.
//!
//! A profile is a self-contained `-savedatafolder`: `<data>/rimforge/profiles/<id>`
//! with its own `Config/`, `Saves/` and `Scenarios/`. `registry.json` holds only
//! the metadata that cannot be derived from the filesystem (display name and
//! timestamps); save/mod counts are recomputed on every listing.
//!
//! Concurrency: registry writes are plain read-modify-write. Commands are not
//! expected to race meaningfully in v1.

use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

use crate::models::Profile;
use crate::paths::{data_root, detect_paths_sync, ensure_dir, profiles_root};

/// The single mod every profile starts with.
const CORE_PACKAGE_ID: &str = "ludeon.rimworld";

// ---------------------------------------------------------------------------
// Registry types
// ---------------------------------------------------------------------------

/// One record in `registry.json`. Not part of the frontend contract; the
/// derived [`Profile`] in `models.rs` is what crosses the boundary.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileMeta {
    pub id: String,
    pub name: String,
    pub created_at_ms: i64,
    #[serde(default)]
    pub last_played_at_ms: Option<i64>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Registry {
    #[serde(default)]
    pub profiles: Vec<ProfileMeta>,
}

impl Registry {
    fn index_of(&self, id: &str) -> Option<usize> {
        self.profiles.iter().position(|p| p.id == id)
    }
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

/// Milliseconds since the Unix epoch.
pub fn now_ms() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis() as i64)
        .unwrap_or(0)
}

/// `<data>/rimforge/registry.json`
pub fn registry_path() -> PathBuf {
    data_root().join("registry.json")
}

/// `<data>/rimforge/profiles/<id>` — does **not** check that it exists.
pub fn profile_dir(id: &str) -> PathBuf {
    profiles_root().join(id)
}

fn to_string(path: &Path) -> String {
    path.to_string_lossy().to_string()
}

// ---------------------------------------------------------------------------
// Slugs
// ---------------------------------------------------------------------------

/// Lowercase, `[a-z0-9-]` only, runs of separators collapsed, trimmed.
/// An empty result falls back to `profile`.
pub fn slugify(name: &str) -> String {
    let mut out = String::with_capacity(name.len());
    let mut pending_dash = false;
    for ch in name.chars() {
        let lower = ch.to_ascii_lowercase();
        if lower.is_ascii_alphanumeric() {
            if pending_dash && !out.is_empty() {
                out.push('-');
            }
            pending_dash = false;
            out.push(lower);
        } else {
            // Any non [a-z0-9] character (including non-ASCII) is a separator.
            pending_dash = true;
        }
    }
    if out.is_empty() {
        "profile".to_string()
    } else {
        out
    }
}

/// Append `-2`, `-3`, … until the slug is free.
pub fn unique_slug(name: &str, taken: &HashSet<String>) -> String {
    let base = slugify(name);
    if !taken.contains(&base) {
        return base;
    }
    let mut n = 2u32;
    loop {
        let candidate = format!("{base}-{n}");
        if !taken.contains(&candidate) {
            return candidate;
        }
        n += 1;
    }
}

/// Ids already in use: registry entries *and* any stray directory on disk.
fn taken_ids(reg: &Registry) -> HashSet<String> {
    let mut taken: HashSet<String> = reg.profiles.iter().map(|p| p.id.clone()).collect();
    if let Ok(entries) = fs::read_dir(profiles_root()) {
        for entry in entries.flatten() {
            if let Some(name) = entry.file_name().to_str() {
                taken.insert(name.to_string());
            }
        }
    }
    taken
}

// ---------------------------------------------------------------------------
// Registry IO
// ---------------------------------------------------------------------------

/// Load `registry.json`. Missing ⇒ empty registry. Malformed ⇒ error (we do not
/// want to silently drop a user's profile list).
pub fn load_registry() -> Result<Registry, String> {
    let path = registry_path();
    match fs::read_to_string(&path) {
        Ok(raw) => {
            if raw.trim().is_empty() {
                return Ok(Registry::default());
            }
            serde_json::from_str(&raw).map_err(|e| format!("malformed {}: {e}", path.display()))
        }
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(Registry::default()),
        Err(e) => Err(format!("could not read {}: {e}", path.display())),
    }
}

pub fn save_registry(reg: &Registry) -> Result<(), String> {
    let path = registry_path();
    ensure_dir(&data_root())?;
    let json =
        serde_json::to_string_pretty(reg).map_err(|e| format!("could not serialise registry: {e}"))?;
    let tmp = path.with_extension("json.tmp");
    fs::write(&tmp, json).map_err(|e| format!("could not write {}: {e}", tmp.display()))?;
    fs::rename(&tmp, &path).map_err(|e| format!("could not write {}: {e}", path.display()))?;
    Ok(())
}

// ---------------------------------------------------------------------------
// Derived counts
// ---------------------------------------------------------------------------

/// Number of `*.rws` files directly inside `<profile>/Saves`.
pub fn count_saves(dir: &Path) -> usize {
    let saves = dir.join("Saves");
    let entries = match fs::read_dir(&saves) {
        Ok(e) => e,
        Err(_) => return 0,
    };
    entries
        .flatten()
        .filter(|entry| {
            entry
                .path()
                .extension()
                .and_then(|e| e.to_str())
                .is_some_and(|e| e.eq_ignore_ascii_case("rws"))
        })
        .count()
}

/// Count `<activeMods>/<li>` entries in a ModsConfig.xml body.
pub fn count_active_mods_in_xml(xml: &str) -> usize {
    let doc = match roxmltree::Document::parse(xml) {
        Ok(doc) => doc,
        Err(e) => {
            eprintln!("rimforge: malformed ModsConfig.xml: {e}");
            return 0;
        }
    };
    doc.descendants()
        .find(|n| n.is_element() && n.tag_name().name().eq_ignore_ascii_case("activeMods"))
        .map(|node| {
            node.children()
                .filter(|c| c.is_element() && c.tag_name().name().eq_ignore_ascii_case("li"))
                .count()
        })
        .unwrap_or(0)
}

/// Active mod count from `<profile>/Config/ModsConfig.xml`; 0 if absent.
pub fn count_active_mods(dir: &Path) -> usize {
    let path = dir.join("Config").join("ModsConfig.xml");
    match fs::read_to_string(&path) {
        Ok(xml) => count_active_mods_in_xml(&xml),
        Err(_) => 0,
    }
}

fn to_profile(meta: &ProfileMeta) -> Profile {
    let dir = profile_dir(&meta.id);
    Profile {
        id: meta.id.clone(),
        name: meta.name.clone(),
        path: to_string(&dir),
        created_at_ms: meta.created_at_ms,
        last_played_at_ms: meta.last_played_at_ms,
        save_count: count_saves(&dir),
        active_mod_count: count_active_mods(&dir),
    }
}

// ---------------------------------------------------------------------------
// ModsConfig.xml bootstrap
// ---------------------------------------------------------------------------

fn xml_escape(s: &str) -> String {
    s.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
}

/// Minimal ModsConfig.xml: Core only, plus the detected game version if known.
pub fn minimal_mods_config(version: Option<&str>) -> String {
    let version_line = match version {
        Some(v) => format!("  <version>{}</version>\n", xml_escape(v)),
        None => String::new(),
    };
    format!(
        "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<ModsConfigData>\n{version_line}  <activeMods>\n    <li>{CORE_PACKAGE_ID}</li>\n  </activeMods>\n</ModsConfigData>\n"
    )
}

// ---------------------------------------------------------------------------
// Copying
// ---------------------------------------------------------------------------

/// Recursively copy `src` into `dst`, **following symlinks** and copying file
/// contents. The source is never modified — important because the default
/// savedata folder is frequently symlink-managed (e.g. a dotfiles repo).
fn copy_tree_following_symlinks(src: &Path, dst: &Path, depth: u32) -> Result<(), String> {
    if depth > 64 {
        return Err(format!(
            "refusing to recurse further at {} (symlink loop?)",
            src.display()
        ));
    }
    ensure_dir(dst)?;
    let entries =
        fs::read_dir(src).map_err(|e| format!("could not read {}: {e}", src.display()))?;
    for entry in entries {
        let entry = match entry {
            Ok(e) => e,
            Err(e) => {
                eprintln!("rimforge: skipping entry in {}: {e}", src.display());
                continue;
            }
        };
        let from = entry.path();
        let to = dst.join(entry.file_name());
        // `metadata` (not `symlink_metadata`) follows symlinks by design.
        let meta = match fs::metadata(&from) {
            Ok(m) => m,
            Err(e) => {
                eprintln!("rimforge: skipping {} ({e})", from.display());
                continue;
            }
        };
        if meta.is_dir() {
            copy_tree_following_symlinks(&from, &to, depth + 1)?;
        } else if meta.is_file() {
            fs::copy(&from, &to).map_err(|e| {
                format!("could not copy {} to {}: {e}", from.display(), to.display())
            })?;
        }
    }
    Ok(())
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

pub async fn list_profiles() -> Result<Vec<Profile>, String> {
    let reg = load_registry()?;
    Ok(reg.profiles.iter().map(to_profile).collect())
}

/// Look a profile up by id. Used by other backend modules (and Stream B).
pub async fn find_profile(id: &str) -> Result<Profile, String> {
    let reg = load_registry()?;
    reg.profiles
        .iter()
        .find(|p| p.id == id)
        .map(to_profile)
        .ok_or_else(|| format!("unknown profile: {id}"))
}

pub async fn create_profile(name: String) -> Result<Profile, String> {
    let name = name.trim().to_string();
    if name.is_empty() {
        return Err("profile name cannot be empty".into());
    }
    let mut reg = load_registry()?;
    ensure_dir(&profiles_root())?;
    let id = unique_slug(&name, &taken_ids(&reg));
    let dir = profile_dir(&id);

    ensure_dir(&dir)?;
    ensure_dir(&dir.join("Config"))?;

    let version = detect_paths_sync().game_version;
    let xml = minimal_mods_config(version.as_deref());
    let config_path = dir.join("Config").join("ModsConfig.xml");
    fs::write(&config_path, xml)
        .map_err(|e| format!("could not write {}: {e}", config_path.display()))?;

    let meta = ProfileMeta {
        id,
        name,
        created_at_ms: now_ms(),
        last_played_at_ms: None,
    };
    let profile = to_profile(&meta);
    reg.profiles.push(meta);
    save_registry(&reg)?;
    Ok(profile)
}

pub async fn rename_profile(id: String, new_name: String) -> Result<Profile, String> {
    let new_name = new_name.trim().to_string();
    if new_name.is_empty() {
        return Err("profile name cannot be empty".into());
    }
    let mut reg = load_registry()?;
    let idx = reg
        .index_of(&id)
        .ok_or_else(|| format!("unknown profile: {id}"))?;
    reg.profiles[idx].name = new_name;
    let profile = to_profile(&reg.profiles[idx]);
    save_registry(&reg)?;
    Ok(profile)
}

pub async fn delete_profile(id: String) -> Result<(), String> {
    let mut reg = load_registry()?;
    let idx = reg
        .index_of(&id)
        .ok_or_else(|| format!("unknown profile: {id}"))?;
    let dir = profile_dir(&id);
    if dir.exists() {
        trash::delete(&dir)
            .map_err(|e| format!("could not move {} to trash: {e}", dir.display()))?;
    }
    reg.profiles.remove(idx);
    save_registry(&reg)?;
    Ok(())
}

pub async fn clone_profile(id: String, new_name: String) -> Result<Profile, String> {
    let new_name = new_name.trim().to_string();
    if new_name.is_empty() {
        return Err("profile name cannot be empty".into());
    }
    let mut reg = load_registry()?;
    if reg.index_of(&id).is_none() {
        return Err(format!("unknown profile: {id}"));
    }
    let src = profile_dir(&id);
    if !src.is_dir() {
        return Err(format!("profile directory missing: {}", src.display()));
    }
    ensure_dir(&profiles_root())?;
    let new_id = unique_slug(&new_name, &taken_ids(&reg));
    let dst = profile_dir(&new_id);
    ensure_dir(&dst)?;

    let mut opts = fs_extra::dir::CopyOptions::new();
    opts.content_only = true;
    opts.overwrite = true;
    fs_extra::dir::copy(&src, &dst, &opts)
        .map_err(|e| format!("could not copy {} to {}: {e}", src.display(), dst.display()))?;

    let meta = ProfileMeta {
        id: new_id,
        name: new_name,
        created_at_ms: now_ms(),
        last_played_at_ms: None,
    };
    let profile = to_profile(&meta);
    reg.profiles.push(meta);
    save_registry(&reg)?;
    Ok(profile)
}

pub async fn import_default(name: String) -> Result<Profile, String> {
    let name = name.trim().to_string();
    if name.is_empty() {
        return Err("profile name cannot be empty".into());
    }
    let detected = detect_paths_sync();
    let source = detected
        .default_savedata
        .as_ref()
        .map(PathBuf::from)
        .ok_or_else(|| "no default RimWorld savedata folder found to import".to_string())?;
    if !source.is_dir() {
        return Err(format!(
            "default savedata folder not found: {}",
            source.display()
        ));
    }

    let mut reg = load_registry()?;
    ensure_dir(&profiles_root())?;
    let id = unique_slug(&name, &taken_ids(&reg));
    let dir = profile_dir(&id);
    ensure_dir(&dir)?;

    let mut copied_any = false;
    for sub in ["Config", "Saves", "Scenarios"] {
        let from = source.join(sub);
        // `metadata` follows symlinks: a symlinked Config/ is copied by content.
        if fs::metadata(&from).map(|m| m.is_dir()).unwrap_or(false) {
            copy_tree_following_symlinks(&from, &dir.join(sub), 0)?;
            copied_any = true;
        }
    }
    if !copied_any {
        eprintln!(
            "rimforge: {} had no Config/Saves/Scenarios to import",
            source.display()
        );
    }
    // Always guarantee a Config/ directory exists.
    ensure_dir(&dir.join("Config"))?;

    let meta = ProfileMeta {
        id,
        name,
        created_at_ms: now_ms(),
        last_played_at_ms: None,
    };
    let profile = to_profile(&meta);
    reg.profiles.push(meta);
    save_registry(&reg)?;
    Ok(profile)
}

/// Stamp `lastPlayedAtMs` on a profile. Used by `launch`.
pub fn touch_last_played(id: &str) -> Result<(), String> {
    let mut reg = load_registry()?;
    let idx = reg
        .index_of(id)
        .ok_or_else(|| format!("unknown profile: {id}"))?;
    reg.profiles[idx].last_played_at_ms = Some(now_ms());
    save_registry(&reg)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn taken(items: &[&str]) -> HashSet<String> {
        items.iter().map(|s| s.to_string()).collect()
    }

    #[test]
    fn slugify_basics() {
        assert_eq!(slugify("Vanilla"), "vanilla");
        assert_eq!(slugify("Medieval Overhaul"), "medieval-overhaul");
        assert_eq!(slugify("  Trimmed  "), "trimmed");
        assert_eq!(slugify("RimWorld 1.6!"), "rimworld-1-6");
        assert_eq!(slugify("multi   spaces"), "multi-spaces");
        assert_eq!(slugify("--leading-and-trailing--"), "leading-and-trailing");
        assert_eq!(slugify("Mix_of/Chars\\Here"), "mix-of-chars-here");
    }

    #[test]
    fn slugify_non_ascii_and_empty() {
        // Non-ASCII characters act as separators; an all-separator name falls
        // back to a usable slug rather than an empty directory name.
        assert_eq!(slugify("Café Run"), "caf-run");
        assert_eq!(slugify("✨✨✨"), "profile");
        assert_eq!(slugify(""), "profile");
        assert_eq!(slugify("   "), "profile");
    }

    #[test]
    fn unique_slug_handles_collisions() {
        let existing = taken(&[]);
        assert_eq!(unique_slug("Vanilla", &existing), "vanilla");

        let existing = taken(&["vanilla"]);
        assert_eq!(unique_slug("Vanilla", &existing), "vanilla-2");

        let existing = taken(&["vanilla", "vanilla-2"]);
        assert_eq!(unique_slug("vanilla", &existing), "vanilla-3");

        // Gaps are filled: -2 is free even though -3 is taken.
        let existing = taken(&["vanilla", "vanilla-3"]);
        assert_eq!(unique_slug("Vanilla!", &existing), "vanilla-2");

        // Different display names that slug to the same base still collide.
        let existing = taken(&["medieval-overhaul"]);
        assert_eq!(
            unique_slug("Medieval  Overhaul", &existing),
            "medieval-overhaul-2"
        );

        // Empty-ish names collide on the fallback slug too.
        let existing = taken(&["profile", "profile-2"]);
        assert_eq!(unique_slug("!!!", &existing), "profile-3");
    }

    #[test]
    fn unique_slug_is_stable_over_many_collisions() {
        let mut existing: HashSet<String> = HashSet::new();
        let mut produced = Vec::new();
        for _ in 0..5 {
            let s = unique_slug("Test Run", &existing);
            existing.insert(s.clone());
            produced.push(s);
        }
        assert_eq!(
            produced,
            vec!["test-run", "test-run-2", "test-run-3", "test-run-4", "test-run-5"]
        );
    }

    #[test]
    fn counts_active_mods_from_xml() {
        let xml = r#"<?xml version="1.0" encoding="utf-8"?>
<ModsConfigData>
  <version>1.6.4535 rev991</version>
  <activeMods>
    <li>ludeon.rimworld</li>
    <li>ludeon.rimworld.royalty</li>
    <li>brrainz.harmony</li>
  </activeMods>
  <knownExpansions><li>ludeon.rimworld.royalty</li></knownExpansions>
</ModsConfigData>"#;
        assert_eq!(count_active_mods_in_xml(xml), 3);
    }

    #[test]
    fn malformed_or_empty_xml_counts_zero() {
        assert_eq!(count_active_mods_in_xml("<ModsConfigData>"), 0);
        assert_eq!(
            count_active_mods_in_xml("<ModsConfigData><activeMods /></ModsConfigData>"),
            0
        );
        assert_eq!(count_active_mods_in_xml(""), 0);
    }

    #[test]
    fn minimal_mods_config_shape() {
        let with_version = minimal_mods_config(Some("1.6.4535 rev991"));
        assert!(with_version.contains("<version>1.6.4535 rev991</version>"));
        assert!(with_version.contains("<li>ludeon.rimworld</li>"));
        assert_eq!(count_active_mods_in_xml(&with_version), 1);

        let without = minimal_mods_config(None);
        assert!(!without.contains("<version>"));
        assert_eq!(count_active_mods_in_xml(&without), 1);
    }

    #[test]
    fn profile_dir_is_under_profiles_root() {
        let dir = profile_dir("vanilla");
        assert!(dir.ends_with("profiles/vanilla") || dir.ends_with("profiles\\vanilla"));
    }

    #[test]
    fn registry_json_roundtrip() {
        let reg = Registry {
            profiles: vec![ProfileMeta {
                id: "vanilla".into(),
                name: "Vanilla".into(),
                created_at_ms: 1_700_000_000_000,
                last_played_at_ms: None,
            }],
        };
        let json = serde_json::to_string(&reg).unwrap();
        assert!(json.contains("createdAtMs"), "got {json}");
        let back: Registry = serde_json::from_str(&json).unwrap();
        assert_eq!(back.profiles.len(), 1);
        assert_eq!(back.profiles[0].id, "vanilla");

        // Tolerates an older/partial record without lastPlayedAtMs.
        let partial: Registry = serde_json::from_str(
            r#"{"profiles":[{"id":"a","name":"A","createdAtMs":1}]}"#,
        )
        .unwrap();
        assert_eq!(partial.profiles[0].last_played_at_ms, None);
        // And an empty file body.
        let empty: Registry = serde_json::from_str("{}").unwrap();
        assert!(empty.profiles.is_empty());
    }

    /// End-to-end exercise of the registry + directory lifecycle. Ignored by
    /// default because it writes real files (and uses the OS trash); run with
    /// `RIMFORGE_DATA_DIR=<scratch> cargo test -- --ignored --test-threads=1`.
    #[test]
    #[ignore]
    fn crud_roundtrip_against_scratch_data_dir() {
        let root = match std::env::var_os("RIMFORGE_DATA_DIR") {
            Some(r) => PathBuf::from(r),
            None => panic!("set RIMFORGE_DATA_DIR to a scratch directory"),
        };
        assert_eq!(data_root(), root);

        futures_lite_block(async {
            let a = create_profile("Smoke Test".into()).await.unwrap();
            assert_eq!(a.id, "smoke-test");
            assert_eq!(a.active_mod_count, 1);
            assert_eq!(a.save_count, 0);
            assert!(profile_dir(&a.id).join("Config/ModsConfig.xml").is_file());

            let b = create_profile("Smoke Test".into()).await.unwrap();
            assert_eq!(b.id, "smoke-test-2");

            let renamed = rename_profile(a.id.clone(), "Renamed".into()).await.unwrap();
            assert_eq!(renamed.id, "smoke-test");
            assert_eq!(renamed.name, "Renamed");

            // A save file so the clone has something to deep-copy.
            fs::create_dir_all(profile_dir(&a.id).join("Saves")).unwrap();
            fs::write(profile_dir(&a.id).join("Saves/Colony.rws"), "<savegame/>").unwrap();

            let cloned = clone_profile(a.id.clone(), "Cloned Run".into()).await.unwrap();
            assert_eq!(cloned.id, "cloned-run");
            assert_eq!(cloned.save_count, 1);
            assert_eq!(cloned.active_mod_count, 1);
            assert!(profile_dir(&cloned.id).join("Saves/Colony.rws").is_file());

            assert_eq!(list_profiles().await.unwrap().len(), 3);
            assert_eq!(find_profile("smoke-test").await.unwrap().name, "Renamed");
            assert!(find_profile("nope").await.is_err());

            touch_last_played(&a.id).unwrap();
            assert!(find_profile(&a.id).await.unwrap().last_played_at_ms.is_some());

            for id in [a.id.clone(), b.id.clone(), cloned.id.clone()] {
                delete_profile(id.clone()).await.unwrap();
                assert!(!profile_dir(&id).exists(), "{id} dir should be gone");
            }
            assert!(list_profiles().await.unwrap().is_empty());
        });
    }

    /// Exercises `import_default` against a synthetic, symlink-managed
    /// savedata folder wired in through the `defaultSavedataOverride` setting.
    /// Ignored for the same reason as the CRUD test.
    #[test]
    #[ignore]
    fn import_default_copies_symlinked_source_without_mutating_it() {
        let root = PathBuf::from(
            std::env::var_os("RIMFORGE_DATA_DIR")
                .expect("set RIMFORGE_DATA_DIR to a scratch directory"),
        );

        // A "real" config store that the savedata folder only links to.
        let store = root.join("store");
        let source = root.join("fake-savedata");
        fs::create_dir_all(store.join("Config")).unwrap();
        fs::write(
            store.join("Config").join("ModsConfig.xml"),
            minimal_mods_config(Some("1.6.4871 rev598")),
        )
        .unwrap();
        fs::create_dir_all(source.join("Saves")).unwrap();
        fs::create_dir_all(source.join("Scenarios")).unwrap();
        fs::write(source.join("Saves").join("Colony.rws"), "<savegame/>").unwrap();
        #[cfg(unix)]
        std::os::unix::fs::symlink(store.join("Config"), source.join("Config")).unwrap();
        #[cfg(not(unix))]
        fs::create_dir_all(source.join("Config")).unwrap();

        crate::settings::save_settings_sync(&crate::models::Settings {
            default_savedata_override: Some(source.display().to_string()),
            ..Default::default()
        })
        .unwrap();

        let fut = import_default("Imported".into());
        let profile = futures_lite_block(fut).unwrap();
        assert_eq!(profile.id, "imported");
        assert_eq!(profile.save_count, 1);
        assert_eq!(profile.active_mod_count, 1);

        let dir = profile_dir(&profile.id);
        assert!(dir.join("Config").join("ModsConfig.xml").is_file());
        assert!(dir.join("Saves").join("Colony.rws").is_file());
        assert!(dir.join("Scenarios").is_dir());
        #[cfg(unix)]
        {
            // Config was copied by content, not re-linked.
            assert!(!fs::symlink_metadata(dir.join("Config"))
                .unwrap()
                .file_type()
                .is_symlink());
            // Source is still the symlink it was.
            assert!(fs::symlink_metadata(source.join("Config"))
                .unwrap()
                .file_type()
                .is_symlink());
        }

        futures_lite_block(delete_profile(profile.id.clone())).unwrap();
        crate::settings::save_settings_sync(&crate::models::Settings::default()).unwrap();
    }

    /// Minimal executor: these futures contain no real await points.
    #[cfg(test)]
    fn futures_lite_block<T>(fut: impl std::future::Future<Output = T>) -> T {
        use std::task::{Context, Poll, RawWaker, RawWakerVTable, Waker};
        fn noop(_: *const ()) {}
        fn clone_waker(p: *const ()) -> RawWaker {
            RawWaker::new(p, &VTABLE)
        }
        static VTABLE: RawWakerVTable = RawWakerVTable::new(clone_waker, noop, noop, noop);
        let waker = unsafe { Waker::from_raw(RawWaker::new(std::ptr::null(), &VTABLE)) };
        let mut cx = Context::from_waker(&waker);
        let mut fut = Box::pin(fut);
        match fut.as_mut().poll(&mut cx) {
            Poll::Ready(v) => v,
            Poll::Pending => panic!("future unexpectedly pending"),
        }
    }

    #[test]
    fn copy_tree_follows_symlinks_and_copies_contents() {
        let base = std::env::temp_dir().join(format!("rimforge-test-{}", now_ms()));
        let src = base.join("src");
        let real = base.join("real");
        fs::create_dir_all(src.join("nested")).unwrap();
        fs::create_dir_all(&real).unwrap();
        fs::write(src.join("nested").join("a.txt"), "hello").unwrap();
        fs::write(real.join("linked.txt"), "linked contents").unwrap();

        #[cfg(unix)]
        {
            std::os::unix::fs::symlink(real.join("linked.txt"), src.join("link.txt")).unwrap();
            std::os::unix::fs::symlink(&real, src.join("linkdir")).unwrap();
        }

        let dst = base.join("dst");
        copy_tree_following_symlinks(&src, &dst, 0).unwrap();

        assert_eq!(fs::read_to_string(dst.join("nested/a.txt")).unwrap(), "hello");
        #[cfg(unix)]
        {
            // Copied as a real file, not as a symlink.
            let copied = dst.join("link.txt");
            assert_eq!(fs::read_to_string(&copied).unwrap(), "linked contents");
            assert!(!fs::symlink_metadata(&copied).unwrap().file_type().is_symlink());
            let copied_dir = dst.join("linkdir");
            assert!(!fs::symlink_metadata(&copied_dir).unwrap().file_type().is_symlink());
            assert_eq!(
                fs::read_to_string(copied_dir.join("linked.txt")).unwrap(),
                "linked contents"
            );
            // Source untouched.
            assert!(fs::symlink_metadata(src.join("link.txt"))
                .unwrap()
                .file_type()
                .is_symlink());
        }

        fs::remove_dir_all(&base).ok();
    }
}
