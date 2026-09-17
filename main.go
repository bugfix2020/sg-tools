package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"sg-tools/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	application := app.NewApp(assets)
	err := wails.Run(&options.App{
		Title:       "SG Tools",
		Width:       1000,
		Height:      720,
		MinWidth:    760,
		MinHeight:   560,
		AssetServer: &assetserver.Options{Assets: assets},
		Bind:        []interface{}{application.Clicker, application.FollowSync},
		OnStartup:   application.Startup,
		OnShutdown:  application.Shutdown,
	})
	if err != nil {
		log.Fatal(err)
	}
}
