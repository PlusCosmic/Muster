use crate::models::{ActiveModList, ModInfo, RulesDbStatus, SortResult};

pub async fn list_installed_mods() -> Result<Vec<ModInfo>, String> {
    Err("not implemented: stream B".into())
}

pub async fn get_active_mods(_profile_id: String) -> Result<ActiveModList, String> {
    Err("not implemented: stream B".into())
}

pub async fn set_active_mods(_profile_id: String, _active_ids: Vec<String>) -> Result<(), String> {
    Err("not implemented: stream B".into())
}

pub async fn sort_mods(_active_ids: Vec<String>) -> Result<SortResult, String> {
    Err("not implemented: stream B".into())
}

pub async fn refresh_rules_db() -> Result<RulesDbStatus, String> {
    Err("not implemented: stream B".into())
}

pub async fn get_rules_db_status() -> Result<RulesDbStatus, String> {
    Err("not implemented: stream B".into())
}
