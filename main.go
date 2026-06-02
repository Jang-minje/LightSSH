package main

import (
	"embed"
	"log"

	"lightssh/backend/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:      "LightSSH",
		Width:      1280,
		Height:     820,
		MinWidth:   1100,
		MinHeight:  720,
		OnStartup:  application.Startup,
		OnShutdown: application.Shutdown,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []interface{}{
			application,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
