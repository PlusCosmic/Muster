pub mod launch;
pub mod models;
pub mod mods;
pub mod paths;
pub mod profiles;
pub mod settings;
pub mod shortcuts;

use models::*;

#[tauri::command]
async fn reveal_path(path: String) -> Result<(), String> {
    tauri_plugin_opener::reveal_item_in_dir(&path).map_err(|e| e.to_string())
}

#[tauri::command]
async fn get_settings() -> Result<Settings, String> {
    settings::get_settings().await
}

#[tauri::command]
async fn update_settings(settings: Settings) -> Result<Settings, String> {
    settings::update_settings(settings).await
}

#[tauri::command]
async fn detect_paths() -> Result<DetectedPaths, String> {
    paths::detect_paths().await
}

#[tauri::command]
async fn list_profiles() -> Result<Vec<Profile>, String> {
    profiles::list_profiles().await
}

#[tauri::command]
async fn create_profile(name: String) -> Result<Profile, String> {
    profiles::create_profile(name).await
}

#[tauri::command]
async fn rename_profile(id: String, new_name: String) -> Result<Profile, String> {
    profiles::rename_profile(id, new_name).await
}

#[tauri::command]
async fn delete_profile(id: String) -> Result<(), String> {
    profiles::delete_profile(id).await
}

#[tauri::command]
async fn clone_profile(id: String, new_name: String) -> Result<Profile, String> {
    profiles::clone_profile(id, new_name).await
}

#[tauri::command]
async fn import_default(name: String) -> Result<Profile, String> {
    profiles::import_default(name).await
}

#[tauri::command]
async fn launch_profile(id: String) -> Result<(), String> {
    launch::launch_profile(id).await
}

#[tauri::command]
async fn create_shortcut(id: String) -> Result<String, String> {
    shortcuts::create_shortcut(id).await
}

#[tauri::command]
async fn list_installed_mods() -> Result<Vec<ModInfo>, String> {
    mods::list_installed_mods().await
}

#[tauri::command]
async fn get_active_mods(profile_id: String) -> Result<ActiveModList, String> {
    mods::get_active_mods(profile_id).await
}

#[tauri::command]
async fn set_active_mods(profile_id: String, active_ids: Vec<String>) -> Result<(), String> {
    mods::set_active_mods(profile_id, active_ids).await
}

#[tauri::command]
async fn sort_mods(active_ids: Vec<String>) -> Result<SortResult, String> {
    mods::sort_mods(active_ids).await
}

#[tauri::command]
async fn refresh_rules_db() -> Result<RulesDbStatus, String> {
    mods::refresh_rules_db().await
}

#[tauri::command]
async fn get_rules_db_status() -> Result<RulesDbStatus, String> {
    mods::get_rules_db_status().await
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![
            reveal_path,
            get_settings,
            update_settings,
            detect_paths,
            list_profiles,
            create_profile,
            rename_profile,
            delete_profile,
            clone_profile,
            import_default,
            launch_profile,
            create_shortcut,
            list_installed_mods,
            get_active_mods,
            set_active_mods,
            sort_mods,
            refresh_rules_db,
            get_rules_db_status,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
