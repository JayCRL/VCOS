// Command agentui is the Wails desktop GUI for AgentOS. It exposes a
// staged wizard (user intent → project intent → UI design → tech plan →
// permissions → decision style → execute) on top of the existing kernel,
// memory, and feedback subsystems. Every stage product is persisted via
// the memory store (see desktop/wizard), not in a bespoke entity.
package main

import (
	"embed"
	"log"

	"mobilevc/data"
	"mobilevc/desktop/notify"
	"mobilevc/kernel"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	store, err := data.NewFileStore("")
	if err != nil {
		log.Fatalf("create session store: %v", err)
	}

	k := kernel.New(store)
	defer k.Stop()

	notifier := notify.NewDesktop()
	nb := newNotifyBridge(notifier)
	nb.Start(k.Bus)
	defer nb.Stop()

	app := NewApp(k)

	err = wails.Run(&options.App{
		Title:  "AgentOS",
		Width:  1280,
		Height: 840,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 26, B: 31, A: 1},
		OnStartup:        app.OnStartup,
		OnShutdown:       app.OnShutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			About: &mac.AboutInfo{
				Title:   "AgentOS",
				Message: "AgentOS Desktop · 五层架构上的统一交互入口",
			},
		},
	})
	if err != nil {
		log.Fatalf("wails: %v", err)
	}
}
