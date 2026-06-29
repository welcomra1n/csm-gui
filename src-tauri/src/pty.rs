use std::collections::HashMap;
use std::io::{Read, Write};
use std::sync::Arc;

use anyhow::Result;
use parking_lot::Mutex;
use portable_pty::{CommandBuilder, MasterPty, PtySize};
use serde::Serialize;
use tauri::{AppHandle, Emitter};

pub struct PtySession {
    pub master: Box<dyn MasterPty + Send>,
    pub writer: Box<dyn Write + Send>,
}

pub struct PtyManager {
    sessions: Mutex<HashMap<String, Arc<Mutex<PtySession>>>>,
}

impl PtyManager {
    pub fn new() -> Self {
        Self {
            sessions: Mutex::new(HashMap::new()),
        }
    }

    pub fn spawn(
        &self,
        app: AppHandle,
        tab_id: String,
        cmd: String,
        args: Vec<String>,
        cwd: Option<String>,
        rows: u16,
        cols: u16,
    ) -> Result<()> {
        let pty_system = portable_pty::native_pty_system();
        let pair = pty_system.openpty(PtySize {
            rows,
            cols,
            pixel_width: 0,
            pixel_height: 0,
        })?;

        let mut command = CommandBuilder::new(cmd);
        for a in args {
            command.arg(a);
        }
        if let Some(d) = cwd {
            command.cwd(d);
        }
        // Make sure children think they're in a real terminal.
        command.env("TERM", "xterm-256color");

        let mut child = pair.slave.spawn_command(command)?;
        drop(pair.slave);

        let writer = pair.master.take_writer()?;
        let reader = pair.master.try_clone_reader()?;

        let sess = Arc::new(Mutex::new(PtySession {
            master: pair.master,
            writer,
        }));
        self.sessions.lock().insert(tab_id.clone(), sess.clone());

        // Output pump.
        let app_for_output = app.clone();
        let tab_for_output = tab_id.clone();
        std::thread::spawn(move || {
            let mut reader = reader;
            let mut buf = [0u8; 8192];
            loop {
                match reader.read(&mut buf) {
                    Ok(0) => break,
                    Ok(n) => {
                        let s = String::from_utf8_lossy(&buf[..n]).to_string();
                        let _ = app_for_output.emit(
                            &format!("pty:output:{}", tab_for_output),
                            s,
                        );
                    }
                    Err(_) => break,
                }
            }
        });

        // Exit watcher.
        let app_for_exit = app.clone();
        let tab_for_exit = tab_id.clone();
        std::thread::spawn(move || {
            let _ = child.wait();
            let _ = app_for_exit.emit(&format!("pty:exit:{}", tab_for_exit), ());
        });

        Ok(())
    }

    pub fn write(&self, tab_id: &str, data: &str) -> Result<()> {
        let sess = self
            .sessions
            .lock()
            .get(tab_id)
            .cloned()
            .ok_or_else(|| anyhow::anyhow!("tab not found"))?;
        let mut sess = sess.lock();
        sess.writer.write_all(data.as_bytes())?;
        sess.writer.flush()?;
        Ok(())
    }

    pub fn resize(&self, tab_id: &str, rows: u16, cols: u16) -> Result<()> {
        let sess = self
            .sessions
            .lock()
            .get(tab_id)
            .cloned()
            .ok_or_else(|| anyhow::anyhow!("tab not found"))?;
        let sess = sess.lock();
        sess.master.resize(PtySize {
            rows,
            cols,
            pixel_width: 0,
            pixel_height: 0,
        })?;
        Ok(())
    }

    pub fn kill(&self, tab_id: &str) {
        self.sessions.lock().remove(tab_id);
    }
}

#[derive(Serialize)]
pub struct ErrString(pub String);

impl<E: std::fmt::Display> From<E> for ErrString {
    fn from(e: E) -> Self {
        ErrString(e.to_string())
    }
}
