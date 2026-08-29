//! The RimSort community rules database: fetch, cache, parse.
//!
//! Source of truth (verified 2026-08-18): the `communityRules.json` file at the
//! root of <https://github.com/RimSort/Community-Rules-Database>, served raw
//! from the `main` branch. Actual schema:
//!
//! ```json
//! {
//!   "timestamp": 1777950016,
//!   "rules": {
//!     "<packageid>": {
//!       "loadAfter":  { "<other packageid>": { "name": ["Display Name"] } },
//!       "loadBefore": { "<other packageid>": { "name": ["Display Name"] } },
//!       "incompatibleWith": { "<other packageid>": { "name": "…", "comment": "…" } },
//!       "loadTop":    { "comment": "…", "value": true },
//!       "loadBottom": { "comment": "…", "value": true }
//!     }
//!   }
//! }
//! ```
//!
//! Note the deviation from the architecture sketch: `loadBottom` (and the
//! undocumented `loadTop`) are `{comment, value: bool}` objects, *not* maps of
//! target package ids. Keys are not reliably lowercase in the upstream file, so
//! we lowercase everything on load.

use std::collections::HashMap;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::models::RulesDbStatus;

/// Verified raw URL of the community rules database.
pub const COMMUNITY_RULES_URL: &str =
    "https://raw.githubusercontent.com/RimSort/Community-Rules-Database/main/communityRules.json";

const RULES_FILE: &str = "communityRules.json";
const META_FILE: &str = "rules_meta.json";

/// Community-sourced ordering hints for one mod.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ModRule {
    pub load_after: Vec<String>,
    pub load_before: Vec<String>,
    pub incompatible_with: Vec<String>,
    pub load_top: bool,
    pub load_bottom: bool,
}

/// The parsed database, keyed by lowercase package id.
#[derive(Debug, Clone, Default)]
pub struct RulesDb {
    pub rules: HashMap<String, ModRule>,
    pub timestamp: Option<i64>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RulesMeta {
    fetched_at_ms: Option<i64>,
    etag: Option<String>,
}

fn now_ms() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis() as i64)
        .unwrap_or(0)
}

/// `<data>/rimforge/cache`
pub fn cache_dir() -> Result<PathBuf, String> {
    let base = dirs::data_dir().ok_or_else(|| "cannot determine data directory".to_string())?;
    Ok(base.join("rimforge").join("cache"))
}

fn rules_path() -> Result<PathBuf, String> {
    Ok(cache_dir()?.join(RULES_FILE))
}

fn meta_path() -> Result<PathBuf, String> {
    Ok(cache_dir()?.join(META_FILE))
}

fn ids_of(value: Option<&Value>) -> Vec<String> {
    let mut out = Vec::new();
    if let Some(Value::Object(map)) = value {
        for key in map.keys() {
            let id = key.trim().to_ascii_lowercase();
            if !id.is_empty() && !out.contains(&id) {
                out.push(id);
            }
        }
        out.sort();
    }
    out
}

/// `{"comment": "...", "value": true}` — also tolerates a bare boolean.
fn flag_of(value: Option<&Value>) -> bool {
    match value {
        Some(Value::Bool(b)) => *b,
        Some(Value::Object(map)) => map.get("value").and_then(Value::as_bool).unwrap_or(true),
        _ => false,
    }
}

/// Parse the raw JSON into a rules database.
pub fn parse_rules(json: &str) -> Result<RulesDb, String> {
    let root: Value =
        serde_json::from_str(json).map_err(|e| format!("communityRules.json parse error: {e}"))?;

    let timestamp = root.get("timestamp").and_then(Value::as_i64);
    // Tolerate both `{ "rules": {…} }` and a bare top-level rules object.
    let rules_obj = root
        .get("rules")
        .filter(|v| v.is_object())
        .unwrap_or(&root)
        .as_object()
        .ok_or_else(|| "communityRules.json: expected an object".to_string())?;

    let mut rules = HashMap::new();
    for (pid, entry) in rules_obj {
        let pid = pid.trim().to_ascii_lowercase();
        if pid.is_empty() || pid == "timestamp" || !entry.is_object() {
            continue;
        }
        let rule = ModRule {
            load_after: ids_of(entry.get("loadAfter")),
            load_before: ids_of(entry.get("loadBefore")),
            incompatible_with: ids_of(entry.get("incompatibleWith")),
            load_top: flag_of(entry.get("loadTop")),
            load_bottom: flag_of(entry.get("loadBottom")),
        };
        rules.insert(pid, rule);
    }

    Ok(RulesDb { rules, timestamp })
}

/// Load the cached database, if there is one. Corrupt cache = no cache.
pub fn load_cached() -> Option<RulesDb> {
    let path = rules_path().ok()?;
    let text = std::fs::read_to_string(&path).ok()?;
    match parse_rules(&text) {
        Ok(db) => Some(db),
        Err(e) => {
            eprintln!("rimforge: discarding corrupt rules cache ({e})");
            None
        }
    }
}

fn load_meta() -> RulesMeta {
    meta_path()
        .ok()
        .and_then(|p| std::fs::read_to_string(p).ok())
        .and_then(|t| serde_json::from_str(&t).ok())
        .unwrap_or_default()
}

fn store(json: &str, etag: Option<String>) -> Result<(), String> {
    let dir = cache_dir()?;
    std::fs::create_dir_all(&dir).map_err(|e| format!("{}: {e}", dir.display()))?;
    let rules = rules_path()?;
    std::fs::write(&rules, json).map_err(|e| format!("{}: {e}", rules.display()))?;
    let meta = RulesMeta {
        fetched_at_ms: Some(now_ms()),
        etag,
    };
    let meta_file = meta_path()?;
    std::fs::write(
        &meta_file,
        serde_json::to_string_pretty(&meta).map_err(|e| e.to_string())?,
    )
    .map_err(|e| format!("{}: {e}", meta_file.display()))
}

fn touch_meta(etag: Option<String>) {
    let mut meta = load_meta();
    meta.fetched_at_ms = Some(now_ms());
    if etag.is_some() {
        meta.etag = etag;
    }
    if let (Ok(path), Ok(text)) = (meta_path(), serde_json::to_string_pretty(&meta)) {
        let _ = std::fs::write(path, text);
    }
}

fn status_of(db: Option<&RulesDb>) -> RulesDbStatus {
    let meta = load_meta();
    match db {
        Some(db) => RulesDbStatus {
            cached: true,
            fetched_at_ms: meta.fetched_at_ms.or(db.timestamp.map(|t| t * 1000)),
            rule_count: db.rules.len(),
        },
        None => RulesDbStatus::default(),
    }
}

/// `get_rules_db_status` command body — cache state only, never touches the
/// network.
pub async fn get_rules_db_status() -> Result<RulesDbStatus, String> {
    Ok(status_of(load_cached().as_ref()))
}

/// `refresh_rules_db` command body — force a re-fetch.
pub async fn refresh_rules_db() -> Result<RulesDbStatus, String> {
    let meta = load_meta();
    let client = reqwest::Client::builder()
        .user_agent(concat!("RimForge/", env!("CARGO_PKG_VERSION")))
        .build()
        .map_err(|e| format!("http client: {e}"))?;

    let mut req = client.get(COMMUNITY_RULES_URL);
    if let Some(etag) = meta.etag.as_deref() {
        if rules_path().map(|p| p.is_file()).unwrap_or(false) {
            req = req.header("If-None-Match", etag);
        }
    }

    let resp = req
        .send()
        .await
        .map_err(|e| format!("could not reach the community rules database: {e}"))?;

    if resp.status() == reqwest::StatusCode::NOT_MODIFIED {
        let etag = resp
            .headers()
            .get(reqwest::header::ETAG)
            .and_then(|v| v.to_str().ok())
            .map(str::to_string);
        touch_meta(etag);
        return Ok(status_of(load_cached().as_ref()));
    }
    if !resp.status().is_success() {
        return Err(format!(
            "community rules database returned HTTP {}",
            resp.status()
        ));
    }

    let etag = resp
        .headers()
        .get(reqwest::header::ETAG)
        .and_then(|v| v.to_str().ok())
        .map(str::to_string);
    let body = resp
        .text()
        .await
        .map_err(|e| format!("could not read the community rules database: {e}"))?;

    // Validate before we overwrite a working cache.
    let db = parse_rules(&body)?;
    store(&body, etag)?;
    Ok(status_of(Some(&db)))
}

/// Rules for sorting: the cache, fetching once if it is missing. Never fails —
/// `None` means "sort with About.xml data only".
pub async fn rules_for_sort() -> Option<RulesDb> {
    if let Some(db) = load_cached() {
        return Some(db);
    }
    match refresh_rules_db().await {
        Ok(_) => load_cached(),
        Err(e) => {
            eprintln!("rimforge: community rules unavailable ({e})");
            None
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const SAMPLE: &str = r#"{
      "timestamp": 1777950016,
      "rules": {
        "Krkr.RocketMan": {
          "loadBottom": { "comment": "always last", "value": true }
        },
        "imranfish.xmlextensions": {
          "loadTop": { "comment": "tier 1", "value": true }
        },
        "3tes.cgtwaa": {
          "loadAfter": { "GT.Sam.GlitterTech": { "name": ["Glitter Tech"] } }
        },
        "ferny.betterarchitect": {
          "loadBefore": { "some.other": { "name": ["Other"] } },
          "incompatibleWith": { "deadmano.rimanoarchitecticons": { "comment": "included" } }
        }
      }
    }"#;

    #[test]
    fn parses_the_real_upstream_schema() {
        let db = parse_rules(SAMPLE).unwrap();
        assert_eq!(db.timestamp, Some(1777950016));
        assert_eq!(db.rules.len(), 4);

        let rocketman = &db.rules["krkr.rocketman"];
        assert!(rocketman.load_bottom);
        assert!(!rocketman.load_top);

        assert!(db.rules["imranfish.xmlextensions"].load_top);
        assert_eq!(
            db.rules["3tes.cgtwaa"].load_after,
            vec!["gt.sam.glittertech"]
        );

        let arch = &db.rules["ferny.betterarchitect"];
        assert_eq!(arch.load_before, vec!["some.other"]);
        assert_eq!(
            arch.incompatible_with,
            vec!["deadmano.rimanoarchitecticons"]
        );
    }

    #[test]
    fn accepts_a_bare_top_level_rules_object() {
        let db = parse_rules(r#"{"a.b": {"loadAfter": {"c.d": {}}}}"#).unwrap();
        assert_eq!(db.rules["a.b"].load_after, vec!["c.d"]);
    }

    #[test]
    fn rejects_malformed_json() {
        assert!(parse_rules("{not json").is_err());
        assert!(parse_rules("[1,2,3]").is_err());
    }
}
