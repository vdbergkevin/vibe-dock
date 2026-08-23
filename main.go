package main

import (
	"context"
	"embed"
	"log"
	"os/exec"

	"github.com/vdbergkevin/vibe-dock/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var darkDockIcon []byte

//go:embed build/appicon-light.png
var lightDockIcon []byte

func main() {
	databasePath, err := appDataPath()
	if err != nil {
		log.Fatal(err)
	}
	dataStore, err := store.Open(databasePath)
	if err != nil {
		log.Fatal(err)
	}
	acpPath, _ := exec.LookPath("vibe-acp")
	service := NewAppService(dataStore, acpPath)
	app := application.New(application.Options{
		Name:        "VibeDock",
		Description: "A fast native desktop client for Mistral Vibe",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Services:   []application.Service{application.NewService(service)},
		Mac:        application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		OnShutdown: service.Close,
	})
	service.SetApp(app)
	initialTheme := dataStore.Setting(context.Background(), "theme")
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if initialTheme == "light" {
			app.SetIcon(lightDockIcon)
			return
		}
		app.SetIcon(darkDockIcon)
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "main", Title: "VibeDock", Width: 1420, Height: 920, MinWidth: 980, MinHeight: 650,
		BackgroundColour: application.NewRGBA(18, 18, 20, 255),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInsetUnified,
			InvisibleTitleBarHeight: 22,
		},
		DefaultContextMenuDisabled: true,
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
