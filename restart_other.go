//go:build !windows && !darwin

package main

func spawnWindowsRelauncher(exe string) {
	// no-op on non-Windows
}

func spawnWindowsUpdater(exe string) {
	// no-op on non-Windows
}

func spawnMacUpdater(pid int) {
	// no-op on non-Darwin
}
