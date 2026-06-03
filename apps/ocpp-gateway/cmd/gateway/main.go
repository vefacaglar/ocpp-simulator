package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/user/ocpp-simulator/apps/ocpp-gateway/internal/gateway"
)

func main() {
	addr := envOr("GATEWAY_ADDR", ":7080")
	brokerURL := envOr("MQTT_BROKER_URL", "tcp://localhost:1883")
	clientID := envOr("MQTT_CLIENT_ID", "ocpp-gateway")

	broker, err := gateway.NewPahoBroker(brokerURL, clientID)
	if err != nil {
		log.Fatalf("create mqtt broker: %v", err)
	}
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelConnect()
	if err := broker.Connect(connectCtx); err != nil {
		log.Printf("[ocpp-gateway] mqtt initial connect failed (will retry in background): %v", err)
	}

	g, err := gateway.New(broker)
	if err != nil {
		log.Fatalf("create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws/", g.WSHandler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[ocpp-gateway] listening on %s (mqtt=%s)", addr, brokerURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-stop
	log.Printf("[ocpp-gateway] shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = broker.Disconnect()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
