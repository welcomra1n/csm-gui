// csm v1 — macOS entry point.
//
// Goal of this commit: prove SwiftTerm hosts a /bin/zsh PTY with
// native NSTextInputClient Korean IME. Single window, no sidebar yet —
// that lands in the next commit once the IME story is confirmed
// ghostty-grade.

import AppKit
import SwiftTerm

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    var window: NSWindow!

    func applicationDidFinishLaunching(_ notification: Notification) {
        let style: NSWindow.StyleMask = [.titled, .closable, .miniaturizable, .resizable]
        window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 960, height: 600),
            styleMask: style,
            backing: .buffered,
            defer: false
        )
        window.title = "csm"
        window.center()
        window.setFrameAutosaveName("csm-main-window")

        let term = LocalProcessTerminalView(frame: window.contentLayoutRect)
        term.autoresizingMask = [.width, .height]

        // Match the legacy theme so the v1 prototype is recognisably
        // csm and not a stock Terminal clone.
        let bg = NSColor(red: 0, green: 0, blue: 0, alpha: 1)
        let fg = NSColor(red: 0, green: 1, blue: 0.4, alpha: 1)
        term.nativeBackgroundColor = bg
        term.nativeForegroundColor = fg
        term.installColors([
            Color(red: 0, green: 0, blue: 0),
            Color(red: 0xff, green: 0x4d, blue: 0x8b),
            Color(red: 0x00, green: 0xff, blue: 0x66),
            Color(red: 0xff, green: 0xd6, blue: 0x0a),
            Color(red: 0x4a, green: 0x9e, blue: 0xff),
            Color(red: 0xd9, green: 0x76, blue: 0xff),
            Color(red: 0x10, green: 0xe0, blue: 0xd0),
            Color(red: 0xcc, green: 0xcc, blue: 0xcc),
            Color(red: 0x44, green: 0x44, blue: 0x44),
            Color(red: 0xff, green: 0x7e, blue: 0xb6),
            Color(red: 0x88, green: 0xff, blue: 0x88),
            Color(red: 0xff, green: 0xe6, blue: 0x6d),
            Color(red: 0x7e, green: 0xc0, blue: 0xff),
            Color(red: 0xe9, green: 0xa6, blue: 0xff),
            Color(red: 0x7d, green: 0xff, blue: 0xff),
            Color(red: 0xff, green: 0xff, blue: 0xff),
        ])

        window.contentView = term

        // Inherit the user's shell + login environment so claude / brew
        // are on PATH the same way they would be in Ghostty.
        let shell = ProcessInfo.processInfo.environment["SHELL"] ?? "/bin/zsh"
        let env = Terminal.getEnvironmentVariables(termName: "xterm-256color")
        term.startProcess(executable: shell, args: ["-l"], environment: env)

        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
