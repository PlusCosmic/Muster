//! Detection of external paths (Steam, the RimWorld install, workshop content,
//! the default savedata folder) plus the app's own data root.
//!
//! Detection never fails on a missing piece: anything we cannot find is `None`.
//! Settings overrides are applied on top of whatever was detected.

use std::fs;
use std::path::{Path, PathBuf};

use regex::Regex;

use crate::models::DetectedPaths;
use crate::settings::load_settings_sync;

/// RimWorld's Steam app id.
pub const RIMWORLD_APP_ID: &str = "294100";

// ---------------------------------------------------------------------------
// App-owned data layout
// ---------------------------------------------------------------------------

/// `<data>/rimforge` — root of everything the app owns.
///
/// `RIMFORGE_DATA_DIR`, when set, replaces the whole root. It exists so tests
/// (and anyone relocating their profiles) can point the app at another disk.
pub fn data_root() -> PathBuf {
    if let Some(dir) = std::env::var_os("RIMFORGE_DATA_DIR") {
        if !dir.is_empty() {
            return PathBuf::from(dir);
        }
    }
    let base = dirs::data_dir()
        .or_else(dirs::home_dir)
        .unwrap_or_else(|| PathBuf::from("."));
    base.join("rimforge")
}

/// `<data>/rimforge/profiles`
pub fn profiles_root() -> PathBuf {
    data_root().join("profiles")
}

/// `<data>/rimforge/cache`
pub fn cache_root() -> PathBuf {
    data_root().join("cache")
}

/// Create `<data>/rimforge` (and the given subdirectory) if absent.
pub fn ensure_dir(path: &Path) -> Result<(), String> {
    fs::create_dir_all(path).map_err(|e| format!("could not create {}: {e}", path.display()))
}

// ---------------------------------------------------------------------------
// Steam root
// ---------------------------------------------------------------------------

#[cfg(target_os = "linux")]
fn detect_steam_root() -> Option<PathBuf> {
    let home = dirs::home_dir()?;
    let candidates = [
        home.join(".steam/steam"),
        home.join(".local/share/Steam"),
        home.join(".steam/root"),
        home.join(".var/app/com.valvesoftware.Steam/data/Steam"),
    ];
    candidates
        .into_iter()
        .find(|c| c.join("steamapps").is_dir())
}

#[cfg(target_os = "macos")]
fn detect_steam_root() -> Option<PathBuf> {
    let home = dirs::home_dir()?;
    let candidate = home.join("Library/Application Support/Steam");
    if candidate.join("steamapps").is_dir() {
        Some(candidate)
    } else {
        None
    }
}

#[cfg(target_os = "windows")]
fn detect_steam_root() -> Option<PathBuf> {
    use winreg::enums::{HKEY_CURRENT_USER, HKEY_LOCAL_MACHINE};
    use winreg::RegKey;

    // (hive, subkey, value name) in priority order.
    let probes: [(_, &str, &str); 4] = [
        (HKEY_CURRENT_USER, r"SOFTWARE\Valve\Steam", "SteamPath"),
        (HKEY_LOCAL_MACHINE, r"SOFTWARE\Valve\Steam", "InstallPath"),
        (
            HKEY_LOCAL_MACHINE,
            r"SOFTWARE\WOW6432Node\Valve\Steam",
            "InstallPath",
        ),
        (HKEY_CURRENT_USER, r"SOFTWARE\Valve\Steam", "InstallPath"),
    ];
    for (hive, subkey, value) in probes {
        if let Ok(key) = RegKey::predef(hive).open_subkey(subkey) {
            if let Ok(raw) = key.get_value::<String, _>(value) {
                let path = PathBuf::from(raw.replace('/', "\\"));
                if path.join("steamapps").is_dir() {
                    return Some(path);
                }
            }
        }
    }
    let fallback = PathBuf::from(r"C:\Program Files (x86)\Steam");
    if fallback.join("steamapps").is_dir() {
        Some(fallback)
    } else {
        None
    }
}

// ---------------------------------------------------------------------------
// libraryfolders.vdf
// ---------------------------------------------------------------------------

/// Unescape a VDF string literal (`\\` ⇒ `\`, `\"` ⇒ `"`, `\n`/`\t` literals).
fn unescape_vdf(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    let mut chars = s.chars();
    while let Some(c) = chars.next() {
        if c != '\\' {
            out.push(c);
            continue;
        }
        match chars.next() {
            Some('\\') => out.push('\\'),
            Some('"') => out.push('"'),
            Some('n') => out.push('\n'),
            Some('t') => out.push('\t'),
            Some(other) => out.push(other),
            None => out.push('\\'),
        }
    }
    out
}

/// Extract every library `"path"` value from a `libraryfolders.vdf` body.
///
/// Deliberately regex-based (no VDF crate, per the architecture contract).
pub fn parse_library_paths(vdf: &str) -> Vec<PathBuf> {
    let re = match Regex::new(r#""path"\s+"((?:[^"\\]|\\.)*)""#) {
        Ok(re) => re,
        Err(e) => {
            eprintln!("rimforge: bad libraryfolders regex: {e}");
            return Vec::new();
        }
    };
    let mut out: Vec<PathBuf> = Vec::new();
    for caps in re.captures_iter(vdf) {
        let raw = unescape_vdf(&caps[1]);
        let path = PathBuf::from(raw);
        if !out.contains(&path) {
            out.push(path);
        }
    }
    out
}

/// All Steam libraries: the root itself plus everything in `libraryfolders.vdf`.
pub fn steam_libraries(steam_root: &Path) -> Vec<PathBuf> {
    let mut libs: Vec<PathBuf> = vec![steam_root.to_path_buf()];
    let vdf = steam_root.join("steamapps").join("libraryfolders.vdf");
    if let Ok(body) = fs::read_to_string(&vdf) {
        for p in parse_library_paths(&body) {
            if !libs.contains(&p) {
                libs.push(p);
            }
        }
    }
    libs.retain(|l| l.join("steamapps").is_dir());
    libs
}

// ---------------------------------------------------------------------------
// Game install / workshop / version
// ---------------------------------------------------------------------------

/// Platform-specific main binary inside the RimWorld install directory.
pub fn game_binary_name() -> &'static str {
    #[cfg(target_os = "linux")]
    {
        "RimWorldLinux"
    }
    #[cfg(target_os = "macos")]
    {
        "RimWorldMac.app"
    }
    #[cfg(target_os = "windows")]
    {
        "RimWorldWin64.exe"
    }
    #[cfg(not(any(target_os = "linux", target_os = "macos", target_os = "windows")))]
    {
        "RimWorldLinux"
    }
}

/// First library whose `steamapps/common/RimWorld` looks like a real install.
pub fn find_game_install(libraries: &[PathBuf]) -> Option<PathBuf> {
    let candidates: Vec<PathBuf> = libraries
        .iter()
        .map(|lib| lib.join("steamapps").join("common").join("RimWorld"))
        .filter(|dir| dir.is_dir())
        .collect();
    // Prefer one that actually contains this platform's executable.
    candidates
        .iter()
        .find(|dir| dir.join(game_binary_name()).exists())
        .cloned()
        .or_else(|| candidates.into_iter().next())
}

/// `<library>/steamapps/workshop/content/294100` for every library that has it.
pub fn find_workshop_dirs(libraries: &[PathBuf]) -> Vec<PathBuf> {
    libraries
        .iter()
        .map(|lib| {
            lib.join("steamapps")
                .join("workshop")
                .join("content")
                .join(RIMWORLD_APP_ID)
        })
        .filter(|dir| dir.is_dir())
        .collect()
}

/// Parse `Version.txt`. Returns the full version string, e.g. `1.6.4535 rev991`.
pub fn parse_version(contents: &str) -> Option<String> {
    let line = contents.lines().find(|l| !l.trim().is_empty())?;
    let trimmed = line.trim().trim_start_matches('\u{feff}').trim();
    if trimmed.is_empty() {
        None
    } else {
        Some(trimmed.to_string())
    }
}

/// `1.6.4535 rev991` ⇒ `1.6`. Used for `supportedVersions` matching.
pub fn major_minor(version: &str) -> Option<String> {
    let numeric = version.split_whitespace().next()?;
    let mut parts = numeric.split('.');
    let major = parts.next()?;
    let minor = parts.next()?;
    if major.is_empty() || minor.is_empty() {
        return None;
    }
    Some(format!("{major}.{minor}"))
}

/// Read `<install>/Version.txt`.
pub fn read_game_version(install: &Path) -> Option<String> {
    let raw = fs::read_to_string(install.join("Version.txt")).ok()?;
    parse_version(&raw)
}

// ---------------------------------------------------------------------------
// Default savedata folder
// ---------------------------------------------------------------------------

/// The vanilla `-savedatafolder` location the game uses when unmanaged.
pub fn default_savedata_dir() -> Option<PathBuf> {
    let home = dirs::home_dir()?;
    #[cfg(target_os = "linux")]
    {
        Some(
            home.join(".config")
                .join("unity3d")
                .join("Ludeon Studios")
                .join("RimWorld by Ludeon Studios"),
        )
    }
    #[cfg(target_os = "windows")]
    {
        Some(home.join("AppData\\LocalLow\\Ludeon Studios\\RimWorld by Ludeon Studios"))
    }
    #[cfg(target_os = "macos")]
    {
        Some(home.join("Library/Application Support/RimWorld"))
    }
    #[cfg(not(any(target_os = "linux", target_os = "macos", target_os = "windows")))]
    {
        let _ = home;
        None
    }
}

// ---------------------------------------------------------------------------
// Detection entry points
// ---------------------------------------------------------------------------

fn existing(path: PathBuf) -> Option<PathBuf> {
    if path.exists() {
        Some(path)
    } else {
        None
    }
}

fn to_string(path: &Path) -> String {
    path.to_string_lossy().to_string()
}

/// Blocking detection with settings overrides applied on top.
pub fn detect_paths_sync() -> DetectedPaths {
    let settings = load_settings_sync();

    let steam_root = settings
        .steam_root_override
        .as_ref()
        .map(PathBuf::from)
        .or_else(detect_steam_root);

    let libraries = steam_root
        .as_deref()
        .map(steam_libraries)
        .unwrap_or_default();

    let game_install = settings
        .game_install_override
        .as_ref()
        .map(PathBuf::from)
        .or_else(|| find_game_install(&libraries));

    let workshop_dirs = find_workshop_dirs(&libraries);

    let game_version = game_install.as_deref().and_then(read_game_version);

    let default_savedata = settings
        .default_savedata_override
        .as_ref()
        .map(PathBuf::from)
        .or_else(default_savedata_dir)
        .and_then(existing);

    DetectedPaths {
        steam_root: steam_root.as_deref().map(to_string),
        game_install: game_install.as_deref().map(to_string),
        default_savedata: default_savedata.as_deref().map(to_string),
        workshop_dirs: workshop_dirs.iter().map(|p| to_string(p)).collect(),
        game_version,
        profiles_dir: to_string(&profiles_root()),
    }
}

pub async fn detect_paths() -> Result<DetectedPaths, String> {
    Ok(detect_paths_sync())
}

/// Same as [`detect_paths`], for use by other backend modules.
pub async fn resolved_paths() -> Result<DetectedPaths, String> {
    Ok(detect_paths_sync())
}

#[cfg(test)]
mod tests {
    use super::*;

    const VDF: &str = r#"
"libraryfolders"
{
	"0"
	{
		"path"		"/home/cosmic/.local/share/Steam"
		"label"		""
		"contentid"		"6781820241171433978"
		"apps"
		{
			"228980"		"918280233"
		}
	}
	"1"
	{
		"path"		"/mnt/big-disk/SteamLibrary"
		"label"		""
		"apps"
		{
			"294100"		"859843592"
		}
	}
}
"#;

    const VDF_WINDOWS: &str = r#"
"libraryfolders"
{
	"0"
	{
		"path"		"C:\\Program Files (x86)\\Steam"
	}
	"1"
	{
		"path"		"D:\\Games\\SteamLibrary"
	}
}
"#;

    #[test]
    fn parses_unix_library_paths() {
        let paths = parse_library_paths(VDF);
        assert_eq!(
            paths,
            vec![
                PathBuf::from("/home/cosmic/.local/share/Steam"),
                PathBuf::from("/mnt/big-disk/SteamLibrary"),
            ]
        );
    }

    #[test]
    fn unescapes_windows_backslashes() {
        let paths = parse_library_paths(VDF_WINDOWS);
        assert_eq!(
            paths,
            vec![
                PathBuf::from(r"C:\Program Files (x86)\Steam"),
                PathBuf::from(r"D:\Games\SteamLibrary"),
            ]
        );
    }

    #[test]
    fn ignores_non_path_keys_and_dedups() {
        let vdf = r#"
			"path"		"/a"
			"mounted"	"1"
			"path"		"/a"
			"contentpath"	"/should/not/match"
		"#;
        // "contentpath" must not match because the regex anchors on the exact
        // quoted key `"path"`.
        assert_eq!(parse_library_paths(vdf), vec![PathBuf::from("/a")]);
    }

    #[test]
    fn empty_or_garbage_vdf_yields_nothing() {
        assert!(parse_library_paths("").is_empty());
        assert!(parse_library_paths("not a vdf at all").is_empty());
    }

    #[test]
    fn parses_full_version_string() {
        assert_eq!(
            parse_version("1.6.4535 rev991\n").as_deref(),
            Some("1.6.4535 rev991")
        );
        assert_eq!(
            parse_version("\u{feff}1.5.4104 rev435\r\n").as_deref(),
            Some("1.5.4104 rev435")
        );
        assert_eq!(parse_version("\n\n1.4.3901 rev1\n").as_deref(), Some("1.4.3901 rev1"));
        assert_eq!(parse_version(""), None);
        assert_eq!(parse_version("   \n  \n"), None);
    }

    #[test]
    fn derives_major_minor() {
        assert_eq!(major_minor("1.6.4535 rev991").as_deref(), Some("1.6"));
        assert_eq!(major_minor("1.6").as_deref(), Some("1.6"));
        assert_eq!(major_minor("bogus"), None);
        assert_eq!(major_minor(""), None);
    }

    #[test]
    fn data_root_ends_in_rimforge() {
        if std::env::var_os("RIMFORGE_DATA_DIR").is_none() {
            assert_eq!(data_root().file_name().unwrap(), "rimforge");
        }
        assert_eq!(profiles_root().file_name().unwrap(), "profiles");
        assert!(profiles_root().starts_with(data_root()));
    }

    /// Smoke test: prints what detection finds on the machine running the
    /// tests. Never launches anything. Run with `cargo test -- --nocapture`.
    #[test]
    fn smoke_detect_paths_on_this_machine() {
        let p = detect_paths_sync();
        println!("--- detect_paths() ---");
        println!("steamRoot:       {:?}", p.steam_root);
        println!("gameInstall:     {:?}", p.game_install);
        println!("gameVersion:     {:?}", p.game_version);
        println!("defaultSavedata: {:?}", p.default_savedata);
        println!("workshopDirs:    {:?}", p.workshop_dirs);
        println!("profilesDir:     {}", p.profiles_dir);
        assert!(!p.profiles_dir.is_empty());
    }
}
