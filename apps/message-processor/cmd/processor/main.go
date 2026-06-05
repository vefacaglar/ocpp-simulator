package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vefacaglar/ocpp-simulator/apps/message-processor/internal/processor"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
)

func main() {
	brokerURL := envOr("MQTT_BROKER_URL", "tcp://localhost:1883")
	coreURL := envOr("OCPP_CORE_URL", "http://localhost:7090")
	clientID := envOr("MQTT_CLIENT_ID", "message-processor")

	broker, err := processor.NewPahoBroker(brokerURL, clientID)
	if err != nil {
		log.Fatalf("create mqtt broker: %v", err)
	}
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelConnect()
	if err := broker.Connect(connectCtx); err != nil {
		log.Printf("[message-processor] mqtt initial connect failed (will retry in background): %v", err)
	}

	core := processor.NewHTTPCoreClient(coreURL)
	handler := processor.NewHandler(protocol.Version1_6, core, broker, processor.StdoutEmitter{IncludeFrame: true})
	p := processor.New(broker, handler)
	if err := p.Start(context.Background()); err != nil {
		log.Fatalf("processor start: %v", err)
	}

	// Health server: a tiny HTTP listener useful for Docker
	// healthcheck and Kubernetes probes. It never carries OCPP
	// wire data.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := &http.Server{Addr: envOr("HEALTH_ADDR", ":7091"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("[message-processor] health on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[message-processor] health server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Printf("[message-processor] shutting down")

	p.Stop()
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
