module github.com/user/ocpp-simulator/apps/api

go 1.26.3

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/mattn/go-isatty v0.0.21 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/pressly/goose/v3 v3.27.1
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	github.com/user/ocpp-simulator/packages/ocpp-protocol v0.0.0
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	modernc.org/libc v1.72.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.51.0
)

replace github.com/user/ocpp-simulator/packages/ocpp-protocol => ../../packages/ocpp-protocol

replace github.com/user/ocpp-simulator/packages/ocpp-schemas => ../../packages/ocpp-schemas
