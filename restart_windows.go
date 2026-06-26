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

// spawnMacUpdater is a no-op on Windows — kept so app.go can reference
// it unconditionally.
func spawnMacUpdater(pid int) {}

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

// spawnWindowsUpdater writes a VBS that:
//  1. waits ~3s for the GUI to exit so scoop can replace csm.exe
//  2. runs `scoop update csm-gui` synchronously (hidden window)
//  3. launches the (now-updated) csm.exe
//
// The script is dispatched via wscript.exe with DETACHED|BREAKAWAY_FROM_JOB so
// it outlives the parent. Without this, scoop fails because csm.exe is locked
// by the running process.
func spawnWindowsUpdater(exe string) {
	if exe == "" {
		return
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("csm-update-%d.vbs", os.Getpid()))
	esc := strings.ReplaceAll(exe, `"`, `""`)
	// scoop is a PowerShell function loaded by the user's profile, so we
	// MUST NOT pass -NoProfile or the function is undefined and the update
	// silently no-ops. Use the scoop.cmd shim path directly via cmd.exe
	// as a fallback that doesn't depend on PS profile.
	script := `Set sh = CreateObject("WScript.Shell")
WScript.Sleep 3000
Dim home, shim
home = sh.ExpandEnvironmentStrings("%USERPROFILE%")
shim = home & "\scoop\shims\scoop.cmd"
Dim fso
Set fso = CreateObject("Scripting.FileSystemObject")
If fso.FileExists(shim) Then
  sh.Run "cmd /c """ & shim & """ update csm-gui", 0, True
Else
  sh.Run "powershell -WindowStyle Hidden -Command ""scoop update csm-gui""", 0, True
End If
WScript.Sleep 500
sh.Run """` + esc + `""", 1, False
`
	if err := os.WriteFile(tmp, []byte(script), 0644); err != nil {
		return
	}
	cmd := exec.Command("wscript.exe", "//B", "//Nologo", tmp)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200 | 0x01000000,
	}
	_ = cmd.Start()
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
}
