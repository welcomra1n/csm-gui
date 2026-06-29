//! C-ABI surface for Swift (mac) and C# (win) hosts.
//!
//! Strategy: every call returns a UTF-8 JSON string. Memory belongs to
//! Rust; callers MUST free it via `csm_string_free`. This sidesteps the
//! complexity of mapping Rust enums / Vec / etc through a thinner ABI
//! and keeps the FFI layer auto-versionable (add a JSON field, neither
//! side has to recompile the bridge).

use std::ffi::{c_char, CStr, CString};

use crate::{metadata, session};

/// Returns a JSON array of every discovered Session, sorted newest
/// first. Caller MUST free the returned pointer with `csm_string_free`.
#[no_mangle]
pub extern "C" fn csm_list_sessions() -> *mut c_char {
    let sessions = session::list_sessions_sorted();
    let json = serde_json::to_string(&sessions).unwrap_or_else(|_| "[]".into());
    CString::new(json).unwrap().into_raw()
}

/// Loads counts / first-last user message / token usage for a single
/// session and returns the updated record as JSON. Returns `null` (the
/// 4-byte string `"null"`) on lookup or IO failure.
#[no_mangle]
pub extern "C" fn csm_load_session_detail(session_id: *const c_char) -> *mut c_char {
    let id = unsafe { CStr::from_ptr(session_id) }
        .to_string_lossy()
        .into_owned();
    let mut sessions = session::list_sessions_sorted();
    let Some(s) = sessions.iter_mut().find(|s| s.id == id) else {
        return CString::new("null").unwrap().into_raw();
    };
    let _ = session::load_detail(s);
    let json = serde_json::to_string(&s).unwrap_or_else(|_| "null".into());
    CString::new(json).unwrap().into_raw()
}

/// Returns the metadata file as JSON.
#[no_mangle]
pub extern "C" fn csm_metadata_load() -> *mut c_char {
    let meta = metadata::load();
    let json = serde_json::to_string(&meta).unwrap_or_else(|_| "{}".into());
    CString::new(json).unwrap().into_raw()
}

/// Writes the supplied JSON back to disk. Returns 0 on success, non-zero
/// on parse / IO failure.
#[no_mangle]
pub extern "C" fn csm_metadata_save(json: *const c_char) -> i32 {
    let raw = unsafe { CStr::from_ptr(json) };
    let Ok(s) = raw.to_str() else { return 1 };
    let Ok(meta) = serde_json::from_str::<metadata::Metadata>(s) else {
        return 2;
    };
    metadata::save(&meta).map(|_| 0).unwrap_or(3)
}

/// Reclaims a string previously returned by any of the `csm_*`
/// functions above. Passing a pointer not produced by Rust is
/// undefined behaviour.
#[no_mangle]
pub extern "C" fn csm_string_free(ptr: *mut c_char) {
    if ptr.is_null() {
        return;
    }
    unsafe {
        let _ = CString::from_raw(ptr);
    }
}
