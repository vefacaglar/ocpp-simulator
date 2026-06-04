# ocpp-core

The canonical log and business owner. It owns the `ocpp_core` PostgreSQL database, answers `message-processor` business callbacks, persists the source-of-truth OCPP message log, and publishes CSMS-initiated CALLs.

## Role

- **Canonical log**: subscribes to `ocpp/+/in` and `ocpp/+/out` and persists every raw frame as the source-of-truth OCPP history (`internal/canonlog`).
- **Business decisions**: serves `/internal/transactions/*` so `message-processor` can resolve Authorize / StartTransaction / StopTransaction.
- **Transaction identity**: creates the transaction row, mints the GUID (`transactions.id`), and assigns the numeric 1.6J `numeric_id` from an in-memory counter seeded at startup from `MAX(numeric_id)`.
- **CSMS-initiated CALLs**: `/internal/csms/*` endpoints build spec-exact OCPP CALL frames, validate them against the official schema, and publish them to `ocpp/{chargePointId}/out` (`internal/csms`).

(`AGENTS.md` → target service split #3.)

## Internal HTTP API

All `/internal/*` endpoints use the **internal** envelope `{ "chargePointId": "...", "payload": {…} }`. This envelope is internal only and never reaches the OCPP wire — the inner `payload` is the spec-exact OCPP body.

Business decisions (called by message-processor):

| Method + path | Purpose |
|---|---|
| `POST /internal/transactions/authorize` | Returns `idTagInfo.status` (MVP: any non-empty idTag → `Accepted`) |
| `POST /internal/transactions/start` | Creates tx, returns `StartTransaction.conf` (`transactionId`, `idTagInfo`) |
| `POST /internal/transactions/stop` | Finalizes tx by numeric id, returns `StopTransaction.conf` |

CSMS-initiated commands (called by external/test clients; publish a CALL to `/out` and return `202 {"status":"queued"}`):

| Path | OCPP action |
|---|---|
| `POST /internal/csms/remote-start` | `RemoteStartTransaction` |
| `POST /internal/csms/remote-stop` | `RemoteStopTransaction` |
| `POST /internal/csms/reset` | `Reset` |
| `POST /internal/csms/unlock-connector` | `UnlockConnector` |
| `POST /internal/csms/change-configuration` | `ChangeConfiguration` |
| `POST /internal/csms/get-configuration` | `GetConfiguration` |
| `POST /internal/csms/trigger-message` | `TriggerMessage` |
| `POST /internal/csms/change-availability` | `ChangeAvailability` |

Plus `GET /health`.

## Config (env)

| Var | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/ocpp_core?sslmode=disable` | PostgreSQL DSN |
| `MQTT_BROKER_URL` | `tcp://localhost:1883` | MQTT broker |
| `HTTP_ADDR` | `:7090` | Internal HTTP API listen address |
| `MQTT_CLIENT_ID` | `ocpp-core` | MQTT client id |

`TEST_DATABASE_URL` (default `…/ocpp_core_test`) is used by the DB tests.

## Database

Migrations are embedded and run on startup via goose (`internal/db/migrations`):

| Table | Purpose |
|---|---|
| `transactions` | Dual identity: `id` (GUID / 2.0.1 wire id) + `numeric_id` (1.6J wire id), status, meter, timestamps |
| `ocpp_message_logs` | Canonical raw OCPP log: direction, action, unique id, payload, topic, tx links |
| `runtime_events` | Runtime/business events (e.g. `transaction.authorized`) with severity |

Runtime state (connections, timers, live counters) lives in memory; PostgreSQL is the source of truth only for the canonical OCPP **history**, not live runtime state.

## Key files

| File | Responsibility |
|---|---|
| `cmd/core/main.go` | Wiring, migrations, counter init, MQTT canon-log start, HTTP server |
| `internal/api/server.go` | `/internal/*` router + handlers, envelope contract |
| `internal/transaction/service.go` | Start/Stop, numeric-id counter (`InitCounter` from `MAX(numeric_id)`) |
| `internal/csms/service.go` | Build/validate/publish CSMS-initiated CALLs |
| `internal/canonlog/service.go` | Persist `ocpp/+/in` and `ocpp/+/out` frames |
| `internal/db/*` | Open/migrate + repos for transactions, message logs, runtime events |

## Run & test

```bash
docker compose up -d ocpp-core   # brings postgres up too
# or
cd apps/ocpp-core && go run ./cmd/core
go test ./...                    # needs TEST_DATABASE_URL reachable
```
