//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// spawnWindowsRelauncher uses a VBScript via wscript.exe (Windows-subsystem,
// no console) to wait 2 seconds then launch csm. wscript.exe survives parent
// death because it is not attached to any console and runs in its own job.
func spawnWindowsRelauncher(exe string) {
	if exe == "" {
		return
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("csm-restart-%d.vbs", os.Getpid()))
	esc := strings.ReplaceAll(exe, `"`, `""`)
	script := fmt.Sprintf(`Set sh = CreateObject("WScript.Shell")
WScript.Sleep 2000
sh.Run """%s""", 1, False
`, esc)
	if err := os.WriteFile(tmp, []byte(script), 0644); err != nil {
		return
	}
	cmd := exec.Command("wscript.exe", "//B", "//Nologo", tmp)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200 | 0x01000000, // DETACHED | NEW_PROCESS_GROUP | BREAKAWAY_FROM_JOB
	}
	_ = cmd.Start()
	if cmd.Process != nil {
		// release so we don't wait on it
		_ = cmd.Process.Release()
	}
}
