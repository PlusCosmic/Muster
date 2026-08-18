//! Persistence for `settings.json` (path overrides).
//!
//! Lives at `<data>/rimforge/settings.json`. A missing file is not an error —
//! it simply means `Settings::default()`.

use std::fs;
use std::path::PathBuf;

use crate::models::Settings;
use crate::paths::data_root;

/// Absolute path to `settings.json`.
pub fn settings_path() -> PathBuf {
    data_root().join("settings.json")
}

/// Normalise `Some("")` / whitespace-only overrides down to `None` so the
/// frontend can clear an override by submitting an empty text field.
fn blank_to_none(v: Option<String>) -> Option<String> {
    v.and_then(|s| {
        let t = s.trim();
        if t.is_empty() {
            None
        } else {
            Some(t.to_string())
        }
    })
}

fn normalize(mut s: Settings) -> Settings {
    s.steam_root_override = blank_to_none(s.steam_root_override);
    s.game_install_override = blank_to_none(s.game_install_override);
    s.default_savedata_override = blank_to_none(s.default_savedata_override);
    s
}

/// Blocking load. Missing file ⇒ `Default`. Malformed file ⇒ `Default` plus a
/// warning on stderr (we never want a corrupt settings file to brick the app).
pub fn load_settings_sync() -> Settings {
    let path = settings_path();
    let raw = match fs::read_to_string(&path) {
        Ok(raw) => raw,
        Err(e) => {
            if e.kind() != std::io::ErrorKind::NotFound {
                eprintln!("rimforge: could not read {}: {e}", path.display());
            }
            return Settings::default();
        }
    };
    match serde_json::from_str::<Settings>(&raw) {
        Ok(s) => normalize(s),
        Err(e) => {
            eprintln!("rimforge: malformed {}: {e}", path.display());
            Settings::default()
        }
    }
}

/// Blocking save, creating `<data>/rimforge/` if needed. Written to a temp file
/// and renamed so a crash mid-write cannot truncate the settings.
pub fn save_settings_sync(settings: &Settings) -> Result<(), String> {
    let path = settings_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("could not create {}: {e}", parent.display()))?;
    }
    let json = serde_json::to_string_pretty(settings)
        .map_err(|e| format!("could not serialise settings: {e}"))?;
    let tmp = path.with_extension("json.tmp");
    fs::write(&tmp, json).map_err(|e| format!("could not write {}: {e}", tmp.display()))?;
    fs::rename(&tmp, &path).map_err(|e| format!("could not write {}: {e}", path.display()))?;
    Ok(())
}

pub async fn get_settings() -> Result<Settings, String> {
    Ok(load_settings_sync())
}

pub async fn update_settings(settings: Settings) -> Result<Settings, String> {
    let settings = normalize(settings);
    save_settings_sync(&settings)?;
    Ok(settings)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn blank_overrides_become_none() {
        let s = normalize(Settings {
            steam_root_override: Some("   ".into()),
            game_install_override: Some("".into()),
            default_savedata_override: Some("  /tmp/x  ".into()),
        });
        assert_eq!(s.steam_root_override, None);
        assert_eq!(s.game_install_override, None);
        assert_eq!(s.default_savedata_override.as_deref(), Some("/tmp/x"));
    }

    #[test]
    fn settings_roundtrip_json_is_camel_case() {
        let s = Settings {
            steam_root_override: Some("/steam".into()),
            ..Default::default()
        };
        let json = serde_json::to_string(&s).unwrap();
        assert!(json.contains("steamRootOverride"), "got {json}");
        let back: Settings = serde_json::from_str(&json).unwrap();
        assert_eq!(back.steam_root_override.as_deref(), Some("/steam"));
    }

    #[test]
    fn missing_file_deserialises_as_default() {
        // An empty object must be accepted (all fields optional).
        let s: Settings = serde_json::from_str("{}").unwrap();
        assert!(s.steam_root_override.is_none());
        assert!(s.game_install_override.is_none());
        assert!(s.default_savedata_override.is_none());
    }
}
