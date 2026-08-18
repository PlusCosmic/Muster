//! Launching a profile through Steam.
//!
//! Steam-mediated so the game keeps its Steam context (Workshop, achievements).
//! We spawn and return immediately — the app never waits on RimWorld.

use std::path::PathBuf;
use std::process::{Command, Stdio};

use crate::paths::RIMWORLD_APP_ID;
use crate::profiles::{profile_dir, touch_last_played};

/// The `-savedatafolder=<abs path>` argument for a profile.
pub fn savedata_arg(profile_path: &std::path::Path) -> String {
    format!("-savedatafolder={}", profile_path.display())
}

/// Build the command used to launch a profile on this platform.
/// Returns `(program, args)`.
fn launch_command(profile_path: &std::path::Path) -> Result<(PathBuf, Vec<String>), String> {
    let arg = savedata_arg(profile_path);

    #[cfg(target_os = "linux")]
    {
        Ok((
            PathBuf::from("steam"),
            vec![
                "-applaunch".into(),
                RIMWORLD_APP_ID.into(),
                arg,
            ],
        ))
    }

    #[cfg(target_os = "windows")]
    {
        let steam_root = crate::paths::detect_paths_sync()
            .steam_root
            .ok_or_else(|| "Steam installation not found".to_string())?;
        let exe = PathBuf::from(steam_root).join("steam.exe");
        if !exe.exists() {
            return Err(format!("Steam executable not found at {}", exe.display()));
        }
        Ok((
            exe,
            vec!["-applaunch".into(), RIMWORLD_APP_ID.into(), arg],
        ))
    }

    #[cfg(target_os = "macos")]
    {
        Ok((
            PathBuf::from("open"),
            vec![
                "-a".into(),
                "Steam".into(),
                "--args".into(),
                "-applaunch".into(),
                RIMWORLD_APP_ID.into(),
                arg,
            ],
        ))
    }

    #[cfg(not(any(target_os = "linux", target_os = "macos", target_os = "windows")))]
    {
        let _ = arg;
        Err("launching is not supported on this platform".to_string())
    }
}

pub async fn launch_profile(id: String) -> Result<(), String> {
    // Errors if the profile is unknown.
    crate::profiles::find_profile(&id).await?;
    let dir = profile_dir(&id);
    if !dir.is_dir() {
        return Err(format!("profile directory missing: {}", dir.display()));
    }

    let (program, args) = launch_command(&dir)?;

    Command::new(&program)
        .args(&args)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .map_err(|e| {
            if e.kind() == std::io::ErrorKind::NotFound {
                format!(
                    "Steam not found (could not run `{}`) — is Steam installed and on PATH?",
                    program.display()
                )
            } else {
                format!("could not launch Steam: {e}")
            }
        })?;
    // Detached: we deliberately drop the child without waiting.

    touch_last_played(&id)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;

    #[test]
    fn savedata_arg_is_absolute_and_prefixed() {
        let arg = savedata_arg(Path::new("/home/x/.local/share/rimforge/profiles/vanilla"));
        assert_eq!(
            arg,
            "-savedatafolder=/home/x/.local/share/rimforge/profiles/vanilla"
        );
    }

    #[test]
    #[cfg(target_os = "linux")]
    fn linux_command_is_steam_applaunch() {
        let (program, args) = launch_command(Path::new("/p/vanilla")).unwrap();
        assert_eq!(program, PathBuf::from("steam"));
        assert_eq!(
            args,
            vec![
                "-applaunch".to_string(),
                "294100".to_string(),
                "-savedatafolder=/p/vanilla".to_string()
            ]
        );
    }

    #[test]
    #[cfg(target_os = "macos")]
    fn macos_command_uses_open() {
        let (program, args) = launch_command(Path::new("/p/vanilla")).unwrap();
        assert_eq!(program, PathBuf::from("open"));
        assert_eq!(args[0], "-a");
        assert_eq!(args[1], "Steam");
        assert_eq!(args[2], "--args");
    }
}
