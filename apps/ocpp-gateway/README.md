# ocpp-gateway

Dumb WebSocket-to-MQTT bridge for OCPP charge point sessions. It is the only service a simulated charge point connects to directly.

## Role

- Accepts one OCPP WebSocket per charge point at `ws://host:7080/ws/{chargePointId}`.
- Publishes every inbound raw OCPP-J frame **unchanged** to MQTT `ocpp/{chargePointId}/in`.
- Subscribes to `ocpp/{chargePointId}/out` and writes those frames **unchanged** back to the matching WebSocket.

It never parses OCPP, owns no business state, no connector state machines, no meter generators, no transactions, and no database. It copies bytes in both directions. (`AGENTS.md` → target service split #1.)

## Wire contract

- WebSocket subprotocols advertised: `ocpp1.6`, `ocpp2.0.1` (the client's selected subprotocol is echoed per the OCPP handshake).
- MQTT payloads are raw OCPP-J arrays only — `[2,…]` CALL, `[3,…]` CALLRESULT, `[4,…]` CALLERROR. No wrapper objects or envelopes.
- One active session per charge point id: a new connection for an already-registered id gracefully closes the old one (`registry.go`).

## Config (env)

| Var | Default | Purpose |
|---|---|---|
| `GATEWAY_ADDR` | `:7080` | HTTP/WS listen address |
| `MQTT_BROKER_URL` | `tcp://localhost:1883` | MQTT broker |
| `MQTT_CLIENT_ID` | `ocpp-gateway` | MQTT client id |

Initial MQTT connect failures are non-fatal; the broker reconnects in the background.

## HTTP surface

- `GET /ws/{chargePointId}` — WebSocket upgrade (one per CP).
- `GET /health` — `{"status":"ok"}` for Docker/k8s probes.

## Key files

| File | Responsibility |
|---|---|
| `cmd/gateway/main.go` | Wiring, env, HTTP server, graceful shutdown |
| `internal/gateway/gateway.go` | Connect / PublishInbound / Disconnect; `InTopic`/`OutTopic` helpers |
| `internal/gateway/ws.go` | WebSocket upgrade, inbound/outbound pumps |
| `internal/gateway/registry.go` | Per-CP connection registry, single-session enforcement |
| `internal/gateway/paho_broker.go` | MQTT broker implementation (Paho) |
| `internal/gateway/broker.go` | `Broker` interface (publish/subscribe) |

## Run & test

```bash
# via docker compose (from repo root)
docker compose up -d ocpp-gateway

# directly
cd apps/ocpp-gateway && go run ./cmd/gateway
go test ./...
```
