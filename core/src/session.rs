//! Session discovery + JSONL streaming parser.
//!
//! Sources:
//!   - claude: `~/.claude/projects/<encoded-cwd>/<uuid>.jsonl`
//!   - codex (chat): `~/.codex/sessions/<YYYY>/<MM>/<DD>/<rollout>.jsonl`
//!
//! The parser reads each line as JSON, classifies it as user / assistant
//! message / metadata, and pulls just enough out to populate a Session
//! row in the session browser. We deliberately do NOT load the full
//! message corpus eagerly — `Session::messages` stays empty until the
//! caller asks for it via `load_detail`.

use std::fs;
use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};
use std::time::SystemTime;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use unicode_normalization::UnicodeNormalization;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum Provider {
    Claude,
    Codex,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Session {
    pub id: String,
    pub provider: Provider,
    pub project_dir: String,
    pub project_name: String,
    pub session_file: PathBuf,
    pub mod_time: DateTime<Utc>,
    pub file_size: u64,
    pub message_count: u32,
    pub user_msg_count: u32,
    pub asst_msg_count: u32,
    pub first_user_msg: String,
    pub last_user_msg: String,
    pub git_branch: Option<String>,
    pub cwd: Option<String>,
    pub alias: Option<String>,
    pub input_tokens: u64,
    pub output_tokens: u64,
    /// True after `load_detail` has populated counts/usage/messages.
    pub loaded: bool,
}

impl Session {
    fn empty(id: String, provider: Provider, session_file: PathBuf) -> Self {
        Self {
            id,
            provider,
            project_dir: String::new(),
            project_name: String::new(),
            session_file,
            mod_time: DateTime::<Utc>::from(SystemTime::UNIX_EPOCH),
            file_size: 0,
            message_count: 0,
            user_msg_count: 0,
            asst_msg_count: 0,
            first_user_msg: String::new(),
            last_user_msg: String::new(),
            git_branch: None,
            cwd: None,
            alias: None,
            input_tokens: 0,
            output_tokens: 0,
            loaded: false,
        }
    }
}

/// Home expansion: returns `~/.claude/projects` style paths in absolute form.
fn home() -> PathBuf {
    if let Some(h) = std::env::var_os("HOME") {
        return PathBuf::from(h);
    }
    PathBuf::from("/")
}

/// claude encodes a project's working directory into the folder name by
/// replacing `/` with `-`. We reverse that to recover the original path.
fn decode_project_path(encoded: &str) -> String {
    if encoded.starts_with('-') {
        format!("/{}", encoded[1..].replace('-', "/"))
    } else {
        encoded.replace('-', "/")
    }
}

fn last_segment(p: &str) -> String {
    p.rsplit(['/', '\\']).next().unwrap_or(p).to_string()
}

/// Normalises filesystem-derived strings (NFD on macOS) to NFC so the
/// GUI doesn't render detached Korean jamo.
fn nfc(s: &str) -> String {
    s.nfc().collect()
}

/// Walk `~/.claude/projects` and `~/.codex/sessions` and return one
/// lightweight Session per discovered JSONL. Heavy parsing is deferred
/// to `load_detail`.
pub fn discover_sessions() -> Vec<Session> {
    let mut out = Vec::new();
    discover_claude(&mut out);
    discover_codex(&mut out);
    out
}

fn discover_claude(out: &mut Vec<Session>) {
    let root = home().join(".claude").join("projects");
    let Ok(rd) = fs::read_dir(&root) else { return };
    for entry in rd.flatten() {
        let dir = entry.path();
        if !dir.is_dir() {
            continue;
        }
        let folder_name = entry.file_name().to_string_lossy().to_string();
        let project_dir = nfc(&decode_project_path(&folder_name));
        let project_name = if project_dir == home().to_string_lossy() {
            "미분류".to_string()
        } else {
            nfc(&last_segment(&project_dir))
        };

        let Ok(files) = fs::read_dir(&dir) else { continue };
        for f in files.flatten() {
            let path = f.path();
            if path.extension().and_then(|e| e.to_str()) != Some("jsonl") {
                continue;
            }
            let Ok(meta) = f.metadata() else { continue };
            let id = path
                .file_stem()
                .and_then(|s| s.to_str())
                .unwrap_or("")
                .to_string();
            let mut s = Session::empty(id, Provider::Claude, path);
            s.project_dir = project_dir.clone();
            s.project_name = project_name.clone();
            s.file_size = meta.len();
            if let Ok(modified) = meta.modified() {
                s.mod_time = modified.into();
            }
            out.push(s);
        }
    }
}

fn discover_codex(out: &mut Vec<Session>) {
    let root = home().join(".codex").join("sessions");
    let Ok(years) = fs::read_dir(&root) else { return };
    for y in years.flatten() {
        let Ok(months) = fs::read_dir(y.path()) else { continue };
        for m in months.flatten() {
            let Ok(days) = fs::read_dir(m.path()) else { continue };
            for d in days.flatten() {
                let Ok(files) = fs::read_dir(d.path()) else { continue };
                for f in files.flatten() {
                    let path = f.path();
                    if path.extension().and_then(|e| e.to_str()) != Some("jsonl") {
                        continue;
                    }
                    let Ok(meta) = f.metadata() else { continue };
                    let id = path
                        .file_stem()
                        .and_then(|s| s.to_str())
                        .unwrap_or("")
                        .to_string();
                    let mut s = Session::empty(id, Provider::Codex, path);
                    s.file_size = meta.len();
                    if let Ok(modified) = meta.modified() {
                        s.mod_time = modified.into();
                    }
                    out.push(s);
                }
            }
        }
    }
}

/// Streaming JSONL parser. Reads each line, updates the session's
/// counts and snapshots the first/last user message text. Bounded
/// to a few MB per file via BufReader; we deliberately avoid loading
/// the whole file into memory because long sessions can be huge.
pub fn load_detail(s: &mut Session) -> std::io::Result<()> {
    let file = fs::File::open(&s.session_file)?;
    let reader = BufReader::new(file);
    for line in reader.lines() {
        let Ok(line) = line else { break };
        if line.trim().is_empty() {
            continue;
        }
        let Ok(v) = serde_json::from_str::<serde_json::Value>(&line) else { continue };

        if let Some(branch) = v.get("gitBranch").and_then(|x| x.as_str()) {
            s.git_branch = Some(branch.to_string());
        }
        if let Some(cwd) = v.get("cwd").and_then(|x| x.as_str()) {
            s.cwd = Some(nfc(cwd));
        }

        let ty = v.get("type").and_then(|x| x.as_str()).unwrap_or("");
        match ty {
            "user" => {
                s.message_count += 1;
                s.user_msg_count += 1;
                let text = extract_text(&v);
                if !text.is_empty() {
                    if s.first_user_msg.is_empty() {
                        s.first_user_msg = nfc(text.chars().take(200).collect::<String>().as_str());
                    }
                    s.last_user_msg = nfc(text.chars().take(200).collect::<String>().as_str());
                }
            }
            "assistant" => {
                s.message_count += 1;
                s.asst_msg_count += 1;
                if let Some(usage) = v.get("message").and_then(|m| m.get("usage")) {
                    if let Some(it) = usage.get("input_tokens").and_then(|x| x.as_u64()) {
                        s.input_tokens += it;
                    }
                    if let Some(ot) = usage.get("output_tokens").and_then(|x| x.as_u64()) {
                        s.output_tokens += ot;
                    }
                }
            }
            "custom-title" => {
                if let Some(t) = v.get("customTitle").and_then(|x| x.as_str()) {
                    if !t.is_empty() {
                        s.alias = Some(nfc(t));
                    }
                }
            }
            _ => {}
        }
    }
    s.loaded = true;
    Ok(())
}

/// Pulls the first plain-text fragment out of a message envelope.
/// Handles both string-form content (claude format) and array-form
/// content blocks (also claude — `[{type:"text", text:"..."}]`).
fn extract_text(line: &serde_json::Value) -> String {
    let Some(msg) = line.get("message") else {
        return String::new();
    };
    let Some(content) = msg.get("content") else {
        return String::new();
    };
    if let Some(s) = content.as_str() {
        return s.to_string();
    }
    if let Some(arr) = content.as_array() {
        for block in arr {
            if block.get("type").and_then(|t| t.as_str()) == Some("text") {
                if let Some(text) = block.get("text").and_then(|t| t.as_str()) {
                    return text.to_string();
                }
            }
        }
    }
    String::new()
}

/// Convenience for the FFI / host UI: discover + sort newest first.
pub fn list_sessions_sorted() -> Vec<Session> {
    let mut v = discover_sessions();
    v.sort_by(|a, b| b.mod_time.cmp(&a.mod_time));
    v
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn decode_project_handles_absolute_paths() {
        // claude encodes `/Users/foo/repo` as `-Users-foo-repo`
        assert_eq!(
            decode_project_path("-Users-welcomra1n-csm-gui"),
            "/Users/welcomra1n/csm/gui"
        );
    }

    #[test]
    fn nfc_combines_korean_jamo() {
        // NFD: ㅎ + ㅏ + ㄴ (3 code points) → "한" (1 code point)
        let nfd = "\u{1112}\u{1161}\u{11ab}";
        assert_eq!(nfc(nfd), "한");
    }
}
