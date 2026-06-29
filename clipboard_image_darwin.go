//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

// Forward declaration; defined in clipboard_image_darwin.m.
int csmWriteImagePathToClipboard(const char *path);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// writeImageToClipboardNative places the image at `path` onto the OS
// pasteboard via NSPasteboard directly. Compared with the previous
// `osascript -e "set the clipboard to ..."` flow this:
//
//   - Never spawns an AppleScript host, so the macOS "Automation" TCC
//     prompt (and the cascading Photos / Files prompts AppleScript can
//     surface when the source path lives inside Photos Library) does
//     not fire.
//   - Runs in-process, so the only TCC consent macOS asks for is
//     csm.app reading the source file — which happens once, then is
//     remembered for the lifetime of the signing identity.
//
// On any failure the caller falls back to the AppleScript path.
func writeImageToClipboardNative(path string) error {
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	if rc := C.csmWriteImagePathToClipboard(c); rc != 0 {
		return fmt.Errorf("NSPasteboard write failed (code %d)", int(rc))
	}
	return nil
}
