//go:build !windows

package main

func spawnWindowsRelauncher(exe string) {
	// no-op on non-Windows
}
