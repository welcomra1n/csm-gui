//go:build !darwin

package main

import "fmt"

// writeImageToClipboardNative is a no-op on non-Darwin builds; the
// caller (CopyImageToClipboard) has its own platform branch.
func writeImageToClipboardNative(path string) error {
	return fmt.Errorf("native clipboard image write is darwin-only")
}
