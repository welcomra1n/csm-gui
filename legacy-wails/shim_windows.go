//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// resolveNpmShim takes a path like ~/.npm-global/bin/claude.cmd, reads the
// .cmd file, and extracts the real node script path. Returns (nodeExe, jsPath, true)
// if successful so the caller can spawn node directly and bypass the cmd.exe
// console flash that npm's shim creates.
func resolveNpmShim(cmdPath string) (string, string, bool) {
	if !strings.HasSuffix(strings.ToLower(cmdPath), ".cmd") {
		return "", "", false
	}
	data, err := os.ReadFile(cmdPath)
	if err != nil {
		return "", "", false
	}
	body := string(data)

	// npm shims contain a line like:
	//   "%~dp0\node.exe"  "%~dp0\node_modules\@scope\pkg\bin\cli.js" %*
	// or with $basedir on PowerShell. We match the JS path portion.
	re := regexp.MustCompile(`"%~dp0\\([^"]+\.js)"`)
	matches := re.FindStringSubmatch(body)
	if len(matches) < 2 {
		return "", "", false
	}
	dir := filepath.Dir(cmdPath)
	jsPath := filepath.Join(dir, matches[1])
	if _, err := os.Stat(jsPath); err != nil {
		return "", "", false
	}
	nodeExe, err := exec.LookPath("node.exe")
	if err != nil {
		nodeExe, err = exec.LookPath("node")
		if err != nil {
			return "", "", false
		}
	}
	return nodeExe, jsPath, true
}
