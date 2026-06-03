module github.com/user/ocpp-simulator/apps/message-processor

go 1.26.3

require (
	github.com/eclipse/paho.mqtt.golang v1.5.0
	github.com/user/ocpp-simulator/packages/ocpp-protocol v0.0.0
	github.com/user/ocpp-simulator/packages/ocpp-schemas v0.0.0
)

replace github.com/user/ocpp-simulator/packages/ocpp-protocol => ../../packages/ocpp-protocol

replace github.com/user/ocpp-simulator/packages/ocpp-schemas => ../../packages/ocpp-schemas
