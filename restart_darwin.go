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

	// Self-daemonising script. The outer invocation forks the heavy
	// work into a background subshell with stdio fully redirected, then
	// exits. macOS launchd kills any child whose stdio is still wired to
	// a quitting GUI app; the inner subshell survives because its file
	// descriptors are detached before csm.app dies.
	script := fmt.Sprintf(`#!/bin/bash
set -u
LOG=%q
PID=%d

# Detach: rerun the script body in a backgrounded subshell with all
# stdio pointed at the log file, then exit immediately so this
# foreground process can be reaped by csm.app's exit.
if [ "${CSM_UPDATER_CHILD:-}" != "1" ]; then
  CSM_UPDATER_CHILD=1 nohup "$0" </dev/null >>"$LOG" 2>&1 &
  disown 2>/dev/null || true
  exit 0
fi

echo "──── csm updater started $(date '+%%Y-%%m-%%d %%H:%%M:%%S') pid=$$ parent=$PID ────"

for i in $(seq 1 80); do
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "parent $PID exited (after $((i*250))ms wait)"
    break
  fi
  sleep 0.25
done

BREW=""
for cand in /opt/homebrew/bin/brew /usr/local/bin/brew "$(command -v brew 2>/dev/null)"; do
  if [ -x "$cand" ]; then BREW="$cand"; break; fi
done
if [ -z "$BREW" ]; then
  echo "ERROR: brew not found"
  exit 1
fi
echo "using brew at $BREW"

"$BREW" update || true
if ! "$BREW" upgrade --cask csm-gui; then
  echo "upgrade reported no work / failed — trying reinstall"
  "$BREW" reinstall --cask csm-gui || echo "ERROR: reinstall failed"
fi

sleep 0.5
if [ -d /Applications/csm.app ]; then
  echo "relaunching /Applications/csm.app"
  /usr/bin/open /Applications/csm.app
else
  cask_app=$(ls -td /opt/homebrew/Caskroom/csm-gui/*/csm.app 2>/dev/null | head -1)
  if [ -z "$cask_app" ]; then
    cask_app=$(ls -td /usr/local/Caskroom/csm-gui/*/csm.app 2>/dev/null | head -1)
  fi
  if [ -n "$cask_app" ]; then
    echo "relaunching $cask_app"
    /usr/bin/open "$cask_app"
  else
    echo "ERROR: csm.app not found in /Applications or Caskroom"
  fi
fi
echo "──── csm updater done $(date '+%%H:%%M:%%S') ────"
`, logPath, pid)

	if err := os.WriteFile(tmp, []byte(script), 0755); err != nil {
		return
	}

	// /usr/bin/open with `-na` launches a fresh, detached Bash whose
	// parent is launchd, not csm. This is the most reliable way on
	// modern macOS to survive the GUI app quitting — Setsid + nohup
	// alone occasionally lose the child to launchd's process-group
	// teardown when the GUI exits via wruntime.Quit.
	cmd := exec.Command("/bin/bash", tmp)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if devnull != nil {
		cmd.Stdin = devnull
		cmd.Stdout = devnull
		cmd.Stderr = devnull
	}
	if err := cmd.Start(); err == nil && cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	if devnull != nil {
		devnull.Close()
	}
}
