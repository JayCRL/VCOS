package main

import (
	"log"
	"net/http"

	"mobilevc/internal/config"
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
		_ = push.NewAPNSService // reserved for APNs integration
	}

	_ = k // kernel available for direct orchestration

	http.Handle("/ws", handler)
	logx.Info("main", "mobilevc WebSocket server listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
