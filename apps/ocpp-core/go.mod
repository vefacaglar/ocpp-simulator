module github.com/vefacaglar/ocpp-simulator/apps/ocpp-core

go 1.26.3

require (
	github.com/eclipse/paho.mqtt.golang v1.5.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.9.2
	github.com/pressly/goose/v3 v3.27.1
	github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol v0.0.0
	github.com/vefacaglar/ocpp-simulator/packages/ocpp-schemas v0.0.0
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.36.0 // indirect
)

replace github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol => ../../packages/ocpp-protocol

replace github.com/vefacaglar/ocpp-simulator/packages/ocpp-schemas => ../../packages/ocpp-schemas
