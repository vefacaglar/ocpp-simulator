module github.com/user/ocpp-simulator/apps/message-processor

go 1.26.3

require (
	github.com/eclipse/paho.mqtt.golang v1.5.0
	github.com/user/ocpp-simulator/packages/ocpp-protocol v0.0.0
	github.com/user/ocpp-simulator/packages/ocpp-schemas v0.0.0
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
)

replace github.com/user/ocpp-simulator/packages/ocpp-protocol => ../../packages/ocpp-protocol

replace github.com/user/ocpp-simulator/packages/ocpp-schemas => ../../packages/ocpp-schemas
