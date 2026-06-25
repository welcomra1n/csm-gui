package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

var version = "dev"

func main() {
	// Create an instance of the app structure
	app := NewApp()
	app.version = version

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "csm",
		Width:     1200,
		Height:    800,
		MinWidth:  640,
		MinHeight: 400,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:         &options.RGBA{R: 0, G: 0, B: 0, A: 1},
		OnStartup:                app.startup,
		EnableDefaultContextMenu: true,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
			CSSDropProperty:    "--wails-drop-target",
			CSSDropValue:       "drop",
		},
		Windows: &windows.Options{
			Theme:                             windows.Dark,
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			BackdropType:                      windows.Acrylic,
			CustomTheme: &windows.ThemeSettings{
				DarkModeTitleBar:           windows.RGB(0, 0, 0),
				DarkModeTitleBarInactive:   windows.RGB(0, 0, 0),
				DarkModeTitleText:          windows.RGB(0, 255, 102),
				DarkModeTitleTextInactive:  windows.RGB(0, 204, 82),
				DarkModeBorder:             windows.RGB(0, 255, 102),
				DarkModeBorderInactive:     windows.RGB(0, 90, 35),
				LightModeTitleBar:          windows.RGB(0, 0, 0),
				LightModeTitleBarInactive:  windows.RGB(0, 0, 0),
				LightModeTitleText:         windows.RGB(0, 255, 102),
				LightModeTitleTextInactive: windows.RGB(0, 204, 82),
				LightModeBorder:            windows.RGB(0, 255, 102),
				LightModeBorderInactive:    windows.RGB(0, 90, 35),
			},
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
