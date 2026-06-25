//go:build windows

package main

import (
	"os/exec"
	"syscall"

	gopty "github.com/aymanbagabas/go-pty"
)

func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}

func hidePtyChild(cmd *gopty.Cmd) {
	// PTY children must NOT get CREATE_NO_WINDOW. ConPTY attaches a
	// pseudo-console to the child via STARTUPINFOEX; CREATE_NO_WINDOW
	// blocks console allocation and the child's stdin/stdout never get
	// wired to the PTY pipes — terminal stays blank even though the
	// process is running.
	//
	// HideWindow alone is safe: STARTF_USESHOWWINDOW + SW_HIDE only hides
	// any GUI window the child might pop, it does not interfere with the
	// ConPTY attach.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
