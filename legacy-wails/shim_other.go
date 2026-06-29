//go:build !windows

package main

func resolveNpmShim(cmdPath string) (string, string, bool) {
	return "", "", false
}
