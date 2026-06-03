module github.com/user/ocpp-simulator/packages/ocpp-protocol

go 1.26.3

require github.com/user/ocpp-simulator/packages/ocpp-schemas v0.0.0

require github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect

replace github.com/user/ocpp-simulator/packages/ocpp-schemas => ../ocpp-schemas
