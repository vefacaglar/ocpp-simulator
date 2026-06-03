# OCPP Simulator

A local-first, web-based OCPP charge point simulator for developers testing EV charging backends.

## Stack

- **Backend**: Go (current API/runtime + mock CSMS; target split: `simulator-api`, `ocpp-gateway`, `message-processor`, `ocpp-core`)
- **Frontend**: Vue 3 + Vite + TypeScript + Pinia
- **Messaging**: MQTT target for gateway/processor/core communication
- **Database**: SQLite now; `ocpp-core` owns its own DB in the target split
- **Local orchestration**: Docker Compose target for MQTT + web + backend services
- **Monorepo**: Turborepo + pnpm + Go workspace

Target message flow:

```text
Web UI -> simulator-api -> simulator config DB
Charge Point WS -> ocpp-gateway -> MQTT ocpp/{chargePointId}/in
MQTT ocpp/{chargePointId}/in -> message-processor -> MQTT ocpp/{chargePointId}/out
MQTT ocpp/{chargePointId}/out -> ocpp-gateway -> Charge Point WS
MQTT in/out topics -> ocpp-core -> canonical DB logs + transactions/business
```

MQTT payloads are raw OCPP-J JSON array frames only. `chargePointId` and direction live in the topic, not in a wrapper payload.
`message-processor` has no DB; it writes consume/publish audit events to stdout/log only. Canonical message logs come from `ocpp-core` subscribing directly to raw `ocpp/+/in` and `ocpp/+/out`.

## Prerequisites

- **Go 1.21+**
- **Node.js 20+** and **pnpm** (`npm i -g pnpm`)
- **make**
  - macOS: ships with Xcode Command Line Tools
  - Linux: `apt install make` / `dnf install make`
- **lsof** (used by the port-kill step)
  - macOS: preinstalled
  - Linux: `apt install lsof` / `dnf install lsof`

## Quick Start

Local dev (current combined services):

```bash
pnpm install
make dev
```

Local v4.3 stack via Docker Compose (6 services + optional csms profile):

```bash
docker compose up -d --build
# UI:       http://localhost:5173
# API:      http://localhost:7070
# Gateway:  ws://localhost:7080/ws/{chargePointId}
# Core:     http://localhost:7090
# Processor health: http://localhost:7091
# MQTT:     tcp://localhost:1883
docker compose --profile csms up csms -d  # optional mock CSMS on :8080
```

Current `make dev` first frees ports `7070`, `8080`, `5173` (via `make kill-ports`) and then starts the current combined dev services in parallel:
- Vue dashboard: http://localhost:5173
- Go API: http://localhost:7070
- Mock CSMS: ws://localhost:8080/ocpp

> **Alternative:** `pnpm dev` runs the same services through Turborepo but does **not** clear the ports beforehand, so a stale process from a previous run can block startup. Prefer `make dev`.

### Individual Services

Each `make` target also kills its own port first, so restarting a single service is safe:

```bash
make dev-api    # Go API only  (port 7070)
make dev-csms   # Mock CSMS    (port 8080)
make dev-web    # Vue only     (port 5173)
```

Plain pnpm equivalents (no port cleanup):

```bash
pnpm dev:api
pnpm dev:csms
pnpm dev:web
```

### Manual Port Cleanup

If a service is running outside the dev scripts and is holding a port, free them by hand:

```bash
make kill-ports
```

### Go Build & Test

```bash
cd apps/simulator-api && go test ./...
cd apps/ocpp-gateway && go test ./...
cd apps/message-processor && go test ./...
cd apps/ocpp-core && go test ./...
cd apps/csms && go test ./...
cd packages/ocpp-protocol && go test ./...
cd packages/ocpp-schemas && go test ./...
```

## Project Structure

```
apps/
  api/              # Current Go API + simulator runtime; will split into target services
  simulator-api/    # Target UI management API/config DB/realtime
  ocpp-gateway/     # Target dumb WebSocket edge and MQTT bridge
  message-processor/ # Target CP-to-server response producer + stdout audit
  ocpp-core/        # Target canonical logs + transactions/business
  web/              # Vue 3 dashboard + per-CP OCPP simulation
  csms/             # Optional legacy/mock Central System
packages/
  ocpp-protocol/    # Target public OCPP codec/message/protocol package
  ocpp-schemas/     # Official OCPP JSON schemas + validator
  shared/           # Shared TypeScript types
  config/           # Shared frontend config
```

## License

MIT
