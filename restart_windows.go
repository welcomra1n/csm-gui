//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func spawnWindowsRelauncher(exe string) {
	// Write a small batch file so the relauncher fully detaches and survives
	// the parent quitting. cmd /c chained with `start` plus DETACHED_PROCESS
	// flags ensures the new csm.exe is not killed when the current one exits.
	tmp := filepath.Join(os.TempDir(), "csm-restart.bat")
	script := "@echo off\r\n" +
		"timeout /t 2 /nobreak >nul\r\n" +
		"start \"\" \"" + exe + "\"\r\n"
	_ = os.WriteFile(tmp, []byte(script), 0644)

	cmd := exec.Command("cmd", "/c", tmp)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	}
	_ = cmd.Start()
}
