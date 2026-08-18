use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Settings {
    pub steam_root_override: Option<String>,
    pub game_install_override: Option<String>,
    pub default_savedata_override: Option<String>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DetectedPaths {
    pub steam_root: Option<String>,
    pub game_install: Option<String>,
    pub default_savedata: Option<String>,
    pub workshop_dirs: Vec<String>,
    pub game_version: Option<String>,
    pub profiles_dir: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Profile {
    pub id: String,
    pub name: String,
    pub path: String,
    pub created_at_ms: i64,
    pub last_played_at_ms: Option<i64>,
    pub save_count: usize,
    pub active_mod_count: usize,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ModSource {
    Official,
    Local,
    Workshop,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ModInfo {
    pub package_id: String,
    pub name: String,
    pub authors: String,
    pub path: String,
    pub source: ModSource,
    pub steam_workshop_id: Option<String>,
    pub supported_versions: Vec<String>,
    pub dependencies: Vec<String>,
    pub load_after: Vec<String>,
    pub load_before: Vec<String>,
    pub force_load_after: Vec<String>,
    pub force_load_before: Vec<String>,
    pub incompatible_with: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ActiveModList {
    pub active_ids: Vec<String>,
    pub known_expansions: Vec<String>,
    pub version: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SortWarning {
    pub kind: String,
    pub package_id: Option<String>,
    pub message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SortResult {
    pub sorted: Vec<String>,
    pub warnings: Vec<SortWarning>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RulesDbStatus {
    pub cached: bool,
    pub fetched_at_ms: Option<i64>,
    pub rule_count: usize,
}
