//go:build !windows

package main

import (
	"os/exec"

	gopty "github.com/aymanbagabas/go-pty"
)

func hideConsole(cmd *exec.Cmd) {
	// no-op
}

func hidePtyChild(cmd *gopty.Cmd) {
	// no-op
}
