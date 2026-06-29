//! csm_core — shared business logic across macOS / Windows GUIs.
//!
//! Owns:
//!   * Session discovery (claude / codex JSONL on disk)
//!   * Per-session JSONL streaming parser (counts, first/last msg, usage)
//!   * Metadata store (aliases, tags, folders, recaps, pins, prefs)
//!
//! All public types are `#[derive(Serialize)]` so the host GUI can hand
//! them straight to JSON, or read the JSON via the FFI surface below.

pub mod session;
pub mod metadata;

// FFI surface for non-Rust hosts (Swift / C#). Each function returns
// a JSON string allocated by Rust; callers must free it via
// csm_string_free.
pub mod ffi;
