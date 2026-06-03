package main

import (
	"log"
	"net/http"

	"github.com/user/ocpp-simulator/apps/simulator-api/internal/api"
	"github.com/user/ocpp-simulator/apps/simulator-api/internal/config"
	"github.com/user/ocpp-simulator/apps/simulator-api/internal/db"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	server := api.NewServer(database)

	addr := ":" + cfg.Port
	log.Printf("OCPP Simulator API listening on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
