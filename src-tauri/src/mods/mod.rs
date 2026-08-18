//! Mods backend: About.xml parsing, installed-mod discovery, ModsConfig.xml
//! read/write, the community rules database, and auto-sort.
//!
//! This module only re-exports the command bodies; the logic lives in the
//! submodules and is deliberately synchronous so it can be unit-tested without
//! an async runtime.

pub mod about;
pub mod modsconfig;
pub mod rules;
pub mod scan;
pub mod sort;

use crate::models::{ActiveModList, ModInfo, RulesDbStatus, SortResult};

pub async fn list_installed_mods() -> Result<Vec<ModInfo>, String> {
    scan::list_installed_mods().await
}

pub async fn get_active_mods(profile_id: String) -> Result<ActiveModList, String> {
    modsconfig::get_active_mods(profile_id).await
}

pub async fn set_active_mods(profile_id: String, active_ids: Vec<String>) -> Result<(), String> {
    modsconfig::set_active_mods(profile_id, active_ids).await
}

pub async fn sort_mods(active_ids: Vec<String>) -> Result<SortResult, String> {
    sort::sort_mods(active_ids).await
}

pub async fn refresh_rules_db() -> Result<RulesDbStatus, String> {
    rules::refresh_rules_db().await
}

pub async fn get_rules_db_status() -> Result<RulesDbStatus, String> {
    rules::get_rules_db_status().await
}
