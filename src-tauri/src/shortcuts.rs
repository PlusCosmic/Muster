//! Per-profile native shortcuts.
//!
//! Shortcuts exec Steam directly, so RimForge does not need to be running (or
//! even installed) for a shortcut to work.

use std::fs;
use std::path::{Path, PathBuf};

use crate::models::Profile;
use crate::paths::RIMWORLD_APP_ID;
use crate::profiles::{find_profile, profile_dir};

/// Display title used for every platform's shortcut: `RimWorld — <name>`.
pub fn shortcut_title(name: &str) -> String {
    format!("RimWorld \u{2014} {name}")
}

/// Strip characters that are illegal or awkward in a filename.
pub fn sanitize_filename(name: &str) -> String {
    let cleaned: String = name
        .chars()
        .map(|c| match c {
            '/' | '\\' | ':' | '*' | '?' | '"' | '<' | '>' | '|' | '\0' => '-',
            c if c.is_control() => '-',
            c => c,
        })
        .collect();
    let trimmed = cleaned.trim().trim_matches('.').trim();
    if trimmed.is_empty() {
        "RimWorld Profile".to_string()
    } else {
        trimmed.to_string()
    }
}

/// Escape a value for a `.desktop` `Exec=` line (reserved chars per the
/// freedesktop spec) and wrap it so paths with spaces survive.
fn desktop_exec_quote(arg: &str) -> String {
    let escaped = arg.replace('\\', r"\\").replace('"', r#"\""#);
    format!("\"{escaped}\"")
}

/// Escape a value for a `.desktop` key that is not `Exec` (Name, Comment…).
fn desktop_value(value: &str) -> String {
    value
        .replace('\\', r"\\")
        .replace('\n', r"\n")
        .replace('\t', r"\t")
        .replace('\r', r"\r")
}

/// Body of the generated `.desktop` file.
pub fn desktop_entry(name: &str, profile_path: &Path) -> String {
    let exec = format!(
        "steam -applaunch {RIMWORLD_APP_ID} {}",
        desktop_exec_quote(&format!("-savedatafolder={}", profile_path.display()))
    );
    format!(
        "[Desktop Entry]\n\
         Type=Application\n\
         Version=1.0\n\
         Name={}\n\
         Comment=Launch RimWorld with the \"{}\" RimForge profile\n\
         Exec={exec}\n\
         Icon=steam_icon_{RIMWORLD_APP_ID}\n\
         Terminal=false\n\
         Categories=Game;\n\
         StartupNotify=true\n",
        desktop_value(&shortcut_title(name)),
        desktop_value(name),
    )
}

/// Contents of the macOS `.app` stub's `Contents/MacOS/launch` script.
pub fn macos_launch_script(profile_path: &Path) -> String {
    // Single-quote the path and escape any embedded single quotes.
    let quoted = format!("'{}'", profile_path.display().to_string().replace('\'', r"'\''"));
    format!(
        "#!/bin/sh\nexec open -a Steam --args -applaunch {RIMWORLD_APP_ID} -savedatafolder={quoted}\n"
    )
}

/// Contents of the macOS `.app` stub's `Contents/Info.plist`.
pub fn macos_info_plist(name: &str, bundle_id_suffix: &str) -> String {
    let title = shortcut_title(name);
    format!(
        "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n\
         <!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n\
         <plist version=\"1.0\">\n\
         <dict>\n\
         \t<key>CFBundleName</key>\n\t<string>{title}</string>\n\
         \t<key>CFBundleDisplayName</key>\n\t<string>{title}</string>\n\
         \t<key>CFBundleIdentifier</key>\n\t<string>dev.rimforge.shortcut.{bundle_id_suffix}</string>\n\
         \t<key>CFBundleExecutable</key>\n\t<string>launch</string>\n\
         \t<key>CFBundlePackageType</key>\n\t<string>APPL</string>\n\
         \t<key>CFBundleInfoDictionaryVersion</key>\n\t<string>6.0</string>\n\
         \t<key>CFBundleShortVersionString</key>\n\t<string>1.0</string>\n\
         \t<key>LSUIElement</key>\n\t<true/>\n\
         </dict>\n\
         </plist>\n"
    )
}

// ---------------------------------------------------------------------------
// Per-platform creation
// ---------------------------------------------------------------------------

#[cfg(target_os = "linux")]
fn create_shortcut_impl(profile: &Profile, dir: &Path) -> Result<PathBuf, String> {
    let apps = dirs::home_dir()
        .ok_or_else(|| "could not determine home directory".to_string())?
        .join(".local")
        .join("share")
        .join("applications");
    fs::create_dir_all(&apps).map_err(|e| format!("could not create {}: {e}", apps.display()))?;
    let path = apps.join(format!("rimforge-{}.desktop", profile.id));
    fs::write(&path, desktop_entry(&profile.name, dir))
        .map_err(|e| format!("could not write {}: {e}", path.display()))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let _ = fs::set_permissions(&path, fs::Permissions::from_mode(0o755));
    }
    Ok(path)
}

#[cfg(target_os = "windows")]
fn create_shortcut_impl(profile: &Profile, dir: &Path) -> Result<PathBuf, String> {
    use mslnk::ShellLink;

    let steam_root = crate::paths::detect_paths_sync()
        .steam_root
        .ok_or_else(|| "Steam installation not found".to_string())?;
    let exe = PathBuf::from(steam_root).join("steam.exe");
    if !exe.exists() {
        return Err(format!("Steam executable not found at {}", exe.display()));
    }

    let programs = dirs::data_dir()
        .ok_or_else(|| "could not determine AppData directory".to_string())?
        .join("Microsoft")
        .join("Windows")
        .join("Start Menu")
        .join("Programs")
        .join("RimForge");
    fs::create_dir_all(&programs)
        .map_err(|e| format!("could not create {}: {e}", programs.display()))?;

    let path = programs.join(format!("{}.lnk", sanitize_filename(&shortcut_title(&profile.name))));
    let mut link =
        ShellLink::new(&exe).map_err(|e| format!("could not build shortcut for {}: {e}", exe.display()))?;
    link.set_arguments(Some(format!(
        "-applaunch {RIMWORLD_APP_ID} \"-savedatafolder={}\"",
        dir.display()
    )));
    link.set_name(Some(shortcut_title(&profile.name)));
    link.set_working_dir(Some(
        exe.parent()
            .map(|p| p.display().to_string())
            .unwrap_or_default(),
    ));
    link.create_lnk(&path)
        .map_err(|e| format!("could not write {}: {e}", path.display()))?;
    Ok(path)
}

#[cfg(target_os = "macos")]
fn create_shortcut_impl(profile: &Profile, dir: &Path) -> Result<PathBuf, String> {
    let apps = dirs::home_dir()
        .ok_or_else(|| "could not determine home directory".to_string())?
        .join("Applications");
    let bundle = apps.join(format!(
        "{}.app",
        sanitize_filename(&shortcut_title(&profile.name))
    ));
    let macos_dir = bundle.join("Contents").join("MacOS");
    fs::create_dir_all(&macos_dir)
        .map_err(|e| format!("could not create {}: {e}", macos_dir.display()))?;

    let plist = bundle.join("Contents").join("Info.plist");
    fs::write(&plist, macos_info_plist(&profile.name, &profile.id))
        .map_err(|e| format!("could not write {}: {e}", plist.display()))?;

    let script = macos_dir.join("launch");
    fs::write(&script, macos_launch_script(dir))
        .map_err(|e| format!("could not write {}: {e}", script.display()))?;
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(&script, fs::Permissions::from_mode(0o755))
            .map_err(|e| format!("could not chmod {}: {e}", script.display()))?;
    }
    Ok(bundle)
}

#[cfg(not(any(target_os = "linux", target_os = "macos", target_os = "windows")))]
fn create_shortcut_impl(_profile: &Profile, _dir: &Path) -> Result<PathBuf, String> {
    Err("shortcuts are not supported on this platform".to_string())
}

pub async fn create_shortcut(id: String) -> Result<String, String> {
    let profile = find_profile(&id).await?;
    let dir = profile_dir(&id);
    if !dir.is_dir() {
        return Err(format!("profile directory missing: {}", dir.display()));
    }
    let path = create_shortcut_impl(&profile, &dir)?;
    Ok(path.to_string_lossy().to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn title_uses_em_dash() {
        assert_eq!(shortcut_title("Vanilla"), "RimWorld — Vanilla");
    }

    #[test]
    fn sanitizes_filenames() {
        assert_eq!(sanitize_filename("A/B:C"), "A-B-C");
        assert_eq!(sanitize_filename("  spaced  "), "spaced");
        assert_eq!(sanitize_filename("///"), "---");
        assert_eq!(sanitize_filename(""), "RimWorld Profile");
    }

    #[test]
    fn desktop_entry_has_required_keys() {
        let body = desktop_entry("Medieval Overhaul", Path::new("/p/medieval overhaul"));
        assert!(body.starts_with("[Desktop Entry]\n"));
        assert!(body.contains("Name=RimWorld — Medieval Overhaul\n"));
        assert!(body.contains(
            "Exec=steam -applaunch 294100 \"-savedatafolder=/p/medieval overhaul\"\n"
        ));
        assert!(body.contains("Type=Application\n"));
        assert!(body.contains("Terminal=false\n"));
    }

    #[test]
    fn macos_script_quotes_the_path() {
        let script = macos_launch_script(Path::new("/Users/x/My Profiles/vanilla"));
        assert!(script.starts_with("#!/bin/sh\n"));
        assert!(script.contains(
            "open -a Steam --args -applaunch 294100 -savedatafolder='/Users/x/My Profiles/vanilla'"
        ));
    }

    /// Writes a real shortcut. Ignored by default; run with
    /// `HOME=<scratch> RIMFORGE_DATA_DIR=<scratch>/data cargo test -- --ignored`.
    #[test]
    #[ignore]
    #[cfg(target_os = "linux")]
    fn writes_a_real_desktop_file() {
        let profile = Profile {
            id: "smoke-shortcut".into(),
            name: "Smoke Shortcut".into(),
            path: "/p/smoke-shortcut".into(),
            created_at_ms: 0,
            last_played_at_ms: None,
            save_count: 0,
            active_mod_count: 1,
        };
        let path = create_shortcut_impl(&profile, Path::new("/p/smoke-shortcut")).unwrap();
        assert!(path.ends_with("applications/rimforge-smoke-shortcut.desktop"));
        let body = fs::read_to_string(&path).unwrap();
        assert!(body.contains("Exec=steam -applaunch 294100 \"-savedatafolder=/p/smoke-shortcut\""));
        println!("wrote {}", path.display());
        fs::remove_file(&path).ok();
    }

    #[test]
    fn macos_plist_is_well_formed_xml() {
        let plist = macos_info_plist("Vanilla", "vanilla");
        assert!(plist.contains("<!DOCTYPE plist PUBLIC"));
        // roxmltree rejects the external DTD reference, so validate the body.
        let body: String = plist
            .lines()
            .filter(|l| !l.starts_with("<!DOCTYPE"))
            .collect::<Vec<_>>()
            .join("\n");
        roxmltree::Document::parse(&body).expect("plist body should be well-formed XML");
        assert!(plist.contains("<string>launch</string>"));
        assert!(plist.contains("<string>RimWorld — Vanilla</string>"));
    }
}
