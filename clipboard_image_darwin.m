//go:build darwin
//
// Tiny Objective-C helper invoked from clipboard_image_darwin.go via cgo.
// Replaces the previous `osascript -e "set the clipboard to ..."` flow so
// macOS does not surface AppleScript-related TCC prompts (Automation,
// Photos, etc.) every time the user pastes an image.
//
// Return values:
//   0 = success
//   1 = NSImage failed to load from the supplied path
//   2 = NSPasteboard write returned 0 items

#import <Cocoa/Cocoa.h>

int csmWriteImagePathToClipboard(const char *path) {
    @autoreleasepool {
        NSString *nsPath = [NSString stringWithUTF8String:path];
        NSImage *image = [[NSImage alloc] initWithContentsOfFile:nsPath];
        if (image == nil) {
            return 1;
        }
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
        NSArray *objects = @[image];
        BOOL ok = [pb writeObjects:objects];
        if (!ok) {
            return 2;
        }
        return 0;
    }
}
