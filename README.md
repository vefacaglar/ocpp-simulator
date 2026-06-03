# OCPP Simulator

A local-first, web-based OCPP charge point simulator for developers testing EV charging backends. Targets OCPP 1.6J first; the v4.3 split is designed for OCPP 2.0.1 follow-up.

## Architecture (v4.3)

```
Vue per-CP simulator --[WS /ws/{cpId}]--> ocpp-gateway
                                         |
                                         v
                              MQTT ocpp/{cpId}/in  +  ocpp/{cpId}/out
                              (raw OCPP-J arrays, no wrapper)
                              ^                                ^
                              |                                |
                       message-processor <---HTTP---> ocpp-core
                       (response producer,         (canonical log +
                        no DB, stdout audit)        transactions, business
                                                     + CSMS-init CALL)
```

| Service | Role | Has DB | Talks MQTT |
|---|---|---|---|
| `simulator-api` | UI management API (CRUD, settings, realtime, health) | yes (`ocpp-simulator.db`) | **no** |
| `ocpp-gateway` | Dumb WebSocket↔MQTT bridge, per-CP `/ws/{cpId}` | no | yes |
| `message-processor` | CP→server response producer; asks `ocpp-core` for business decisions | no (stdout audit only) | yes |
| `ocpp-core` | Canonical log source; StartTransaction counter; CSMS-init CALL publisher | yes (`ocpp-core.db`) | yes |
| `web` | Vue 3 UI; one WebSocket per simulated charge point to `ocpp-gateway` | no | no |
| `csms` (optional) | Legacy mock Central System for end-to-end testing | no | no |

The hard rules of the OCPP wire are enforced throughout:
- WebSocket subprotocol is `ocpp1.6` or `ocpp2.0.1` (per negotiated version).
- MQTT payloads are raw OCPP-J arrays (`[2, uid, action, payload]` etc.) — **no wrappers, no envelopes, no `{type:...}` objects**.
- The `ocpp-protocol` package is the only place that parses/produces wire bytes; the `codec` package is its sole boundary.

See `plan.md` for the full design rationale and `nextplan.md` for the v4.3 split decisions.

## Prerequisites

- **Go 1.21+** (workspace uses 1.26.3)
- **Node.js 20+** and **pnpm 9.x** (Node 20's corepack ships a 11.x pnpm that crashes on `ERR_UNKNOWN_BUILTIN_MODULE`; pin to 9.15.4 — see `apps/web/Dockerfile`)
- **Docker + Docker Compose** (for the v4.3 stack)
- **make** (for the legacy combined dev path; not required for the v4.3 stack)

## Quick Start

### v4.3 stack via Docker Compose

```bash
docker compose up -d --build
# UI:       http://localhost:5173
# API:      http://localhost:7070/api
# Gateway:  ws://localhost:7080/ws/{chargePointId}
# Core:     http://localhost:7090
# Processor health: http://localhost:7091
# MQTT:     tcp://localhost:1883
docker compose --profile csms up csms -d  # optional mock CSMS on :8080
```

End-to-end verification with a fake CP (writes to `docs/wire-dumps/t29-end-to-end-ocpp-frames.log`):

```bash
docker compose up -d
docker run --rm --network ocpp-simulator_default -v "$PWD/scripts:/scripts" \
  python:3.12-alpine sh -c "pip install --quiet websockets && python3 /scripts/fake_cp.py CP-WIRE"
```

### Local dev with vite HMR + backends in compose

The default dev workflow: `make dev` brings up the 5 backends in
docker compose (mqtt, simulator-api, ocpp-gateway,
message-processor, ocpp-core) and then runs the Vue dev server
with hot-reload. The web container is intentionally excluded
so the UI sees source edits without a rebuild.

```bash
make dev                # backends in compose + vite at :5173
make down               # tear down the backend stack
make dev-stack          # backends only (no UI)
make dev-web            # vite only (assumes backends are up)
make dev-backend:simulator-api    # single Go service via go run,
make dev-backend:ocpp-gateway      # bypasses compose. Each target
make dev-backend:ocpp-core         # uses the env vars that match the
make dev-backend:message-processor  # compose topology.
make dev-backend:csms
make test               # 7 Go module tests with GOWORK=off
make wire-dump          # run a fake CP, capture raw OCPP frames
```

Run `make help` for the full list.

## Per-module build & test

Every Go module is self-contained: `go.mod` has the `require`/`replace` directives it needs, so tests work both with `go.work` and with `GOWORK=off` from inside the module directory.

```bash
cd apps/simulator-api   && go test ./...
cd apps/ocpp-gateway    && go test ./...
cd apps/message-processor && go test ./...
cd apps/ocpp-core       && go test ./...
cd apps/csms            && go test ./...
cd packages/ocpp-protocol && go test ./...
cd packages/ocpp-schemas  && go test ./...
```

## Project Structure

```
apps/
  simulator-api/    # UI management API + config DB + /api/realtime
  ocpp-gateway/     # Dumb WebSocket edge; per-CP /ws/{cpId}; raw frame bridge
  message-processor/ # CP→server response producer; no DB; stdout audit
  ocpp-core/        # Canonical log + transactions + business + CSMS-init CALL
  web/              # Vue 3 dashboard + per-CP OCPP simulation
  csms/             # Optional legacy/mock Central System
packages/
  ocpp-protocol/    # Public OCPP codec/message/protocol package
  ocpp-schemas/     # Official OCPP JSON schemas + validator
  shared/           # Shared TypeScript types
  config/           # Shared frontend config
```

## License

MIT
