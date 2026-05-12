package main

import (
	"log"
	"net/http"

	"mobilevc/internal/config"
	"mobilevc/internal/dashboard"
	"mobilevc/internal/data"
	"mobilevc/internal/gateway"
	"mobilevc/internal/kernel"
	"mobilevc/internal/logx"
	"mobilevc/internal/push"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	store, err := data.NewFileStore("")
	if err != nil {
		log.Fatalf("create session store: %v", err)
	}

	// Create the agent kernel.
	k := kernel.New(store)

	handler := gateway.NewHandler(cfg.AuthToken, store)

	// Wire push notifications if configured.
	if cfg.TTS.Enabled {
		_ = push.NewAPNsService // reserved for APNs integration
	}

	// Mount the observability dashboard on /dashboard/.
	dash := dashboard.NewHandler(k.Bus, k.MemStore, k, k.Feedback)
	http.Handle("/dashboard/", dash)

	http.Handle("/ws", handler)
	logx.Info("main", "dashboard at http://localhost:%s/dashboard/", cfg.Port)
	logx.Info("main", "mobilevc WebSocket server listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
