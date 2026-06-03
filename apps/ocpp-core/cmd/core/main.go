package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/api"
	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/broker"
	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/canonlog"
	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/csms"
	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/db"
	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/transaction"
)

func main() {
	databaseURL := envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ocpp_core?sslmode=disable")
	brokerURL := envOr("MQTT_BROKER_URL", "tcp://localhost:1883")
	httpAddr := envOr("HTTP_ADDR", ":7090")
	clientID := envOr("MQTT_CLIENT_ID", "ocpp-core")

	database, err := db.Open(databaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	txRepo := db.NewTransactionRepo(database)
	logRepo := db.NewMessageLogRepo(database)
	rtRepo := db.NewRuntimeEventRepo(database)

	b, err := broker.NewPahoBroker(brokerURL, clientID)
	if err != nil {
		log.Fatalf("create mqtt broker: %v", err)
	}
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelConnect()
	if err := b.Connect(connectCtx); err != nil {
		log.Printf("[ocpp-core] mqtt initial connect failed (will retry in background): %v", err)
	}

	transactionSvc := transaction.NewService(txRepo)
	if err := transactionSvc.InitCounter(context.Background()); err != nil {
		log.Fatalf("init transaction counter: %v", err)
	}

	csmsSvc := csms.New(b)
	canonLog := canonlog.New(b, logRepo)
	stopLog, err := canonLog.Start(context.Background())
	if err != nil {
		log.Fatalf("canonlog start: %v", err)
	}
	defer stopLog()

	srv := api.NewServer(transactionSvc, csmsSvc, rtRepo, txRepo)
	httpSrv := &http.Server{Addr: httpAddr, Handler: srv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("[ocpp-core] HTTP on %s (mqtt=%s db=%s)", httpAddr, brokerURL, databaseURL)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Printf("[ocpp-core] shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	_ = b.Disconnect()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
