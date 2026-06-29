mod pty;

use std::sync::Arc;

use tauri::{AppHandle, Manager, State};

use pty::PtyManager;

struct AppState {
    pty: Arc<PtyManager>,
}

#[tauri::command]
fn pty_start(
    app: AppHandle,
    state: State<'_, AppState>,
    tab_id: String,
    cmd: String,
    args: Vec<String>,
    cwd: Option<String>,
    rows: u16,
    cols: u16,
) -> Result<(), String> {
    state
        .pty
        .spawn(app, tab_id, cmd, args, cwd, rows, cols)
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn pty_write(
    state: State<'_, AppState>,
    tab_id: String,
    data: String,
) -> Result<(), String> {
    state.pty.write(&tab_id, &data).map_err(|e| e.to_string())
}

#[tauri::command]
fn pty_resize(
    state: State<'_, AppState>,
    tab_id: String,
    rows: u16,
    cols: u16,
) -> Result<(), String> {
    state
        .pty
        .resize(&tab_id, rows, cols)
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn pty_kill(state: State<'_, AppState>, tab_id: String) {
    state.pty.kill(&tab_id);
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let state = AppState {
        pty: Arc::new(PtyManager::new()),
    };

    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .manage(state)
        .invoke_handler(tauri::generate_handler![
            pty_start, pty_write, pty_resize, pty_kill,
        ])
        .setup(|_app| Ok(()))
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
