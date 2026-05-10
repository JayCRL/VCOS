package main

import (
	"log"
	"net/http"

	"mobilevc/internal/config"
	"mobilevc/internal/data"
	"mobilevc/internal/gateway"
	"mobilevc/internal/logx"
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

	handler := gateway.NewHandler(cfg.AuthToken, store)

	http.Handle("/ws", handler)
	logx.Info("main", "mobilevc WebSocket server listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
