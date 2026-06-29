//! Persistent metadata store at `~/.claude/csm-metadata.json`.
//!
//! Mirrors the Wails-era schema 1:1 so a user upgrading from v0.9.x
//! keeps every tag / folder / alias / recap they had before. New fields
//! are append-only with `#[serde(default)]` so older JSON files still
//! deserialise.

use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SavedTab {
    #[serde(rename = "sessionId", default, skip_serializing_if = "Option::is_none")]
    pub session_id: Option<String>,
    pub title: String,
    #[serde(default)]
    pub provider: String,
    #[serde(default, skip_serializing_if = "is_false")]
    pub pinned: bool,
}

fn is_false(b: &bool) -> bool {
    !*b
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Metadata {
    #[serde(default)]
    pub folders: Vec<String>,
    #[serde(default)]
    pub session_folders: HashMap<String, String>,
    #[serde(default)]
    pub session_tags: HashMap<String, Vec<String>>,
    #[serde(default)]
    pub folder_collapsed: HashMap<String, bool>,
    #[serde(default)]
    pub folder_colors: HashMap<String, String>,
    #[serde(default)]
    pub temp_sessions: HashMap<String, bool>,
    #[serde(default)]
    pub recaps: HashMap<String, String>,
    #[serde(default)]
    pub prefs: HashMap<String, bool>,
    #[serde(default)]
    pub open_tabs: Vec<SavedTab>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_seen_version: Option<String>,
}

fn metadata_path() -> PathBuf {
    let home = std::env::var_os("HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("."));
    home.join(".claude").join("csm-metadata.json")
}

pub fn load() -> Metadata {
    let path = metadata_path();
    let Ok(bytes) = fs::read(&path) else {
        return Metadata::default();
    };
    serde_json::from_slice(&bytes).unwrap_or_default()
}

pub fn save(meta: &Metadata) -> std::io::Result<()> {
    let path = metadata_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let bytes = serde_json::to_vec_pretty(meta)?;
    fs::write(path, bytes)
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::tempdir;

    #[test]
    fn unknown_fields_in_old_metadata_dont_break_load() {
        // Simulate an older file with a now-removed key.
        let dir = tempdir().unwrap();
        std::env::set_var("HOME", dir.path());
        let path = dir.path().join(".claude").join("csm-metadata.json");
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(
            &path,
            r#"{"folders":["proj-a"],"removed_field":42,"prefs":{"notif":true}}"#,
        )
        .unwrap();
        let meta = load();
        assert_eq!(meta.folders, vec!["proj-a"]);
        assert_eq!(meta.prefs.get("notif"), Some(&true));
    }
}
