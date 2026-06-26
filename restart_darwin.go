//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Stubs so app.go can reference the Windows helpers unconditionally.
func spawnWindowsRelauncher(exe string) {}
func spawnWindowsUpdater(exe string)    {}

// spawnMacUpdater writes a shell script that:
//  1. waits for the running csm.app process to exit so brew can replace
//     the .app bundle without yanking files out from under us
//  2. runs `brew update` + `brew upgrade --cask csm-gui` (falls back to
//     `reinstall` if upgrade reports no work)
//  3. relaunches /Applications/csm.app
//
// The script is dispatched via `nohup bash ... &` with setsid and a
// fresh session ID so it outlives this process. Without detachment the
// brew run dies with us the moment the GUI quits, leaving the cask
// half-installed.
func spawnMacUpdater(pid int) {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("csm-update-%d.sh", os.Getpid()))
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("csm-update-%d.log", os.Getpid()))

	// brew lives in /opt/homebrew on Apple Silicon and /usr/local on Intel.
	// Probe at runtime so the script works on both.
	script := fmt.Sprintf(`#!/bin/bash
set -u
exec >"%s" 2>&1
echo "csm updater pid=$$"

# Wait for the GUI to exit so brew can replace the .app bundle.
pid=%d
for i in $(seq 1 80); do
  if ! kill -0 "$pid" 2>/dev/null; then break; fi
  sleep 0.25
done

BREW=""
for cand in /opt/homebrew/bin/brew /usr/local/bin/brew "$(command -v brew 2>/dev/null)"; do
  if [ -x "$cand" ]; then BREW="$cand"; break; fi
done
if [ -z "$BREW" ]; then
  echo "brew not found"
  exit 1
fi

"$BREW" update || true
if ! "$BREW" upgrade --cask csm-gui; then
  "$BREW" reinstall --cask csm-gui || true
fi

# Give Finder a beat, then relaunch.
sleep 0.5
open -a /Applications/csm.app || open -a csm
`, logPath, pid)

	if err := os.WriteFile(tmp, []byte(script), 0755); err != nil {
		return
	}

	cmd := exec.Command("/bin/bash", tmp)
	// New session + detach so the script survives parent exit.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err == nil && cmd.Process != nil {
		_ = cmd.Process.Release()
	}
}
