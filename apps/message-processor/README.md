# message-processor

The CP-to-server **response producer**. It reads inbound charge-point CALLs from MQTT, produces spec-exact OCPP 1.6J responses, and publishes them back. It has **no database** — its only side output besides the wire is a stdout audit log.

## Role

- Subscribes to `ocpp/+/in`, decodes raw OCPP-J frames via the `ocpp-protocol` codec.
- For each CP→server **CALL**, builds the OCPP 1.6J response, validates it against the official schema (`ocpp-schemas`), and publishes a `CALLRESULT`/`CALLERROR` to `ocpp/{chargePointId}/out`.
- For inbound `CALLRESULT`/`CALLERROR` (CP answering a CSMS command), it does nothing but audit — it never responds to those.
- Calls `ocpp-core` over HTTP for the three business decisions: Authorize, StartTransaction, StopTransaction.

(`AGENTS.md` → target service split #2.)

## Action handling

| Action | Source of response |
|---|---|
| `BootNotification` | Local — `Accepted`, `currentTime`, `interval=300` |
| `Heartbeat` | Local — `currentTime` |
| `StatusNotification` | Local — `{}` |
| `MeterValues` | Local — `{}` |
| `Authorize` | `ocpp-core` `/internal/transactions/authorize` |
| `StartTransaction` | `ocpp-core` `/internal/transactions/start` |
| `StopTransaction` | `ocpp-core` `/internal/transactions/stop` |
| anything else | `CALLERROR` with `NotImplemented` |

If a business handler is invoked but `ocpp-core` is unavailable, the action becomes a `CALLERROR` (`GenericError`). Every produced response is schema-validated before publish; a validation failure also becomes a `CALLERROR`.

## Config (env)

| Var | Default | Purpose |
|---|---|---|
| `MQTT_BROKER_URL` | `tcp://localhost:1883` | MQTT broker |
| `OCPP_CORE_URL` | `http://localhost:7090` | ocpp-core base URL for business callbacks |
| `MQTT_CLIENT_ID` | `message-processor` | MQTT client id |
| `HEALTH_ADDR` | `:7091` | Health HTTP listener (never carries OCPP data) |

## HTTP surface

- `GET /health` — `{"status":"ok"}` only.

## Audit log

Each consumed/published frame emits a structured stdout `AuditEvent` (`internal/processor/audit.go`) with kind, direction, action, unique id, decision, and optionally the raw frame. This is the service's only persistence; there is no DB.

## Key files

| File | Responsibility |
|---|---|
| `cmd/processor/main.go` | Wiring, env, MQTT + health server |
| `internal/processor/handler.go` | Decode → decide → build → validate → publish; per-action builders |
| `internal/processor/processor.go` | MQTT subscribe loop over `ocpp/+/in` |
| `internal/processor/core_client.go` | HTTP client for ocpp-core business callbacks |
| `internal/processor/audit.go` | `AuditEmitter` / stdout audit events |
| `internal/processor/paho_broker.go` | MQTT broker (Paho) implementation |

## Run & test

```bash
docker compose up -d message-processor
# or
cd apps/message-processor && go run ./cmd/processor
go test ./...
```
