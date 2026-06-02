package main

import (
	"log"
	"net/http"

	"github.com/user/ocpp-simulator/apps/api/internal/api"
	"github.com/user/ocpp-simulator/apps/api/internal/config"
	"github.com/user/ocpp-simulator/apps/api/internal/db"
	"github.com/user/ocpp-simulator/apps/api/internal/ocpp"
	"github.com/user/ocpp-simulator/apps/api/internal/ocpp/v16"
	"github.com/user/ocpp-simulator/apps/api/internal/realtime"
	"github.com/user/ocpp-simulator/apps/api/internal/simulator"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	eventBus := realtime.NewEventBus()
	hub := realtime.NewHub(eventBus)

	factory := ocpp.NewFactory()
	factory.Register(v16.NewProtocol())

	runtime := simulator.NewRuntime(eventBus, factory)
	server := api.NewServer(database, runtime, hub)

	addr := ":" + cfg.Port
	log.Printf("OCPP Simulator API listening on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
