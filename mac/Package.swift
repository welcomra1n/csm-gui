// swift-tools-version: 6.0
//
// csm macOS shell. Uses SwiftTerm for the native PTY terminal view
// (proper NSTextInputClient → ghostty-grade Korean IME) and links the
// Rust core via a system library target that points at
// ../core/target/release/libcsm_core.dylib.

import PackageDescription

let package = Package(
    name: "csm-mac",
    platforms: [
        .macOS(.v13)
    ],
    dependencies: [
        .package(
            url: "https://github.com/migueldeicaza/SwiftTerm.git",
            from: "1.2.0"
        )
    ],
    targets: [
        // C shim that exposes the cdylib symbols to Swift. The actual
        // .dylib is built by `cargo build --release` in ../core.
        .systemLibrary(
            name: "CCsmCore",
            path: "Sources/CCsmCore"
        ),
        .executableTarget(
            name: "csm-mac",
            dependencies: [
                .product(name: "SwiftTerm", package: "SwiftTerm"),
                "CCsmCore",
            ],
            // Link against the Rust core's dylib + tell the linker where
            // to find it at build time. Runtime path is patched by
            // install_name_tool in the bundle step.
            linkerSettings: [
                .unsafeFlags([
                    "-L../core/target/release",
                    "-lcsm_core",
                    "-Xlinker", "-rpath", "-Xlinker", "@executable_path/../Frameworks",
                    "-Xlinker", "-rpath", "-Xlinker", "@loader_path/../core/target/release",
                ])
            ]
        ),
        .testTarget(
            name: "csm-macTests",
            dependencies: ["csm-mac"]
        ),
    ]
)
