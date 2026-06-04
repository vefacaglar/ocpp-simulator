# OCPP Simulator — Agent Guide

A local-first, web-based **OCPP charge point simulator** for developers testing EV charging backends. Go backend + Vue frontend + PostgreSQL, in a Turborepo/pnpm + Go workspace monorepo. Targets OCPP 1.6J first, designed for OCPP 2.0.1 later.

The authoritative design lives in [`plan.md`](plan.md). Read it before non-trivial work. This file is the quick operational guide.

## ⚠️ Hard rule: strict OCPP compliance

**Everything that goes on the wire MUST conform exactly to the official OCPP spec for the negotiated version.** No custom, simplified, renamed, or localized models, fields, casing, enums, or message framing — ever. See `plan.md` §2b.

- Wire payloads are spec-exact: field names/casing (`chargePointVendor`, `idTagInfo`, `meterStart`), types, required/optional, value ranges, enum strings all match the official JSON schema character-for-character.
- OCPP-J framing exactly: `[2, uniqueId, action, payload]` (CALL), `[3, uniqueId, payload]` (CALLRESULT), `[4, uniqueId, errorCode, errorDescription, errorDetails]` (CALLERROR). Standard error-code set only.
- **Never wrap OCPP wire messages.** Communication between a simulated unit and the CSMS must be raw OCPP-J JSON array frames only. Do not convert them into wrapper objects such as `{ "type": "CALL", "action": "...", "payload": ... }`, DTO envelopes, command models, or localized/internal shapes on the wire.
- WebSocket subprotocol is the standard one (`ocpp1.6`, `ocpp2.0.1`).
- **Internal models never leak to the wire.** The generic domain model (transaction GUID + `numericId` dual identity, `ConnectorStateMachine`, runtime events) is internal only. The version `codec` is the single boundary that translates internal state ↔ spec-exact wire payloads.
- Official schemas are the single source of truth in `packages/ocpp-schemas`; code conforms to them, never the other way around. Payloads are validated against them in tests.

If a value isn't defined by the OCPP spec, it must not appear in an OCPP message.

## Architecture

This is an **OCPP simulator**, not a generic web app. The browser is a control surface for simulated charge point behavior.

Keep these communication paths separate:
1. **Vue simulated charge point → ocpp-gateway**: one OCPP WebSocket session per simulated unit. The Vue simulator opens one socket per charge point and sends raw OCPP-J frames over that unit's connection. This must never become one shared socket for all units.
2. **ocpp-gateway → MQTT → message-processor/ocpp-core**: the gateway publishes inbound raw frames to `ocpp/{chargePointId}/in`, subscribes to `ocpp/{chargePointId}/out`, and writes outbound raw frames to the matching charge point WebSocket. MQTT payloads are raw OCPP-J arrays only.
3. **Vue client → browser IndexedDB**: charge point config, connector config, selected unit, and UI live logs are browser-local. This is not a replacement for the OCPP socket.
4. **External/test client → ocpp-core**: CSMS-initiated commands such as `RemoteStartTransaction` enter through `ocpp-core` internal APIs, then `ocpp-core` publishes the raw OCPP CALL to `ocpp/{chargePointId}/out`.

Local charge point actions such as plug/unplug, Authorize, StartTransaction, StopTransaction, MeterValues, and StatusNotification must be emitted by that simulated charge point over its own OCPP WebSocket. Do not reroute those actions through generic runtime command APIs as the primary behavior.

Target service split:
1. **ocpp-gateway**: dumb WebSocket edge. Charge point clients connect here. It publishes inbound raw OCPP frames unchanged to MQTT `ocpp/{chargePointId}/in`, subscribes to `ocpp/{chargePointId}/out`, and writes outbound raw OCPP frames unchanged to the correct WebSocket. It MUST NOT own business state, connector state machines, meter generators, transactions, or DB writes.
2. **message-processor**: response producer. It reads `ocpp/+/in`, parses raw OCPP-J arrays, responds to supported CP-to-server OCPP 1.6J CALLs, calls `ocpp-core` for business decisions such as Authorize/StartTransaction/StopTransaction, and publishes raw CALLRESULT/CALLERROR frames to `ocpp/{chargePointId}/out`. It has no DB and writes consume/publish audit events only to stdout/log.
3. **ocpp-core**: canonical log and business owner. It owns the `ocpp_core` database, persists raw `ocpp/+/in` and `ocpp/+/out` topic messages as the source-of-truth OCPP log, answers message-processor business callbacks, initializes transaction numeric IDs from `MAX(numeric_id)`, and publishes CSMS-initiated CALLs to `ocpp/{chargePointId}/out`.
4. **web**: UI plus per-charge-point local OCPP simulation. It opens one WebSocket per simulated charge point to `ocpp-gateway`, owns browser-local charge point/connector config, client-side connector state, meter generation, transaction holder state, and UI logs.

MQTT payloads must also stay raw OCPP-J arrays. Topic names carry `chargePointId` and direction; payloads must not be wrapped.

Local development uses Docker Compose for PostgreSQL, MQTT broker, `ocpp-gateway`, `message-processor`, `ocpp-core`, and `web`. Service Dockerfiles should stay service-scoped.

Runtime vs. persistence: active connections, timers, loops, and transaction state live **in memory**; browser IndexedDB stores UI config/logs; PostgreSQL stores `ocpp-core` canonical OCPP history. PostgreSQL is not the source of truth for live runtime state.

Transaction identity is **dual** (`plan.md` §7.6, §8.4, §13): every transaction has an internal GUID (`transactions.id`, also the 2.0.1 wire id) and a `numeric_id` (1.6J wire id, assigned asynchronously by the CSMS in `StartTransaction.conf`).

## Repo layout

```
apps/
  ocpp-gateway/     # Target: dumb WebSocket edge and MQTT bridge
  message-processor/ # Target: CP-to-server response producer + stdout audit
  ocpp-core/        # Target: canonical OCPP logs + transactions/business
  web/              # Vue 3 + Vite + TS + Pinia + browser IndexedDB
packages/
  ocpp-protocol/    # Go module: public OCPP codec/message/protocol package
  ocpp-schemas/     # Go module: official OCPP JSON schemas (single source of truth) + embed + Validate()
  shared/           # Shared TS types / generated client
  config/           # Shared frontend config
go.work             # Links apps/ocpp-gateway, apps/ocpp-core, apps/message-processor, packages/ocpp-protocol, packages/ocpp-schemas
plan.md             # Full architecture & development plan (authoritative)
```

## Dev commands

```
pnpm install                              # install JS deps
pnpm dev                                  # make dev: OCPP stack + web
pnpm dev:web                              # Vue only
cd apps/ocpp-gateway && go test ./...       # target
cd apps/message-processor && go test ./...  # target
cd apps/ocpp-core && go test ./...          # target
cd packages/ocpp-protocol && go test ./...  # target
cd packages/ocpp-schemas && go test ./...
```

Default target URL: OCPP gateway session `ws://localhost:7080/ws/{chargePointId}`.

## Conventions

- **Backend stack**: Go, simple HTTP router, sqlc/lightweight DB layer, goose/golang-migrate migrations, gorilla or nhooyr WebSocket. Keep the simulator runtime independent of the HTTP framework and OCPP independent of persistence.
- **Frontend stack**: Vue 3, Vite, TS, Pinia, browser IndexedDB, native WebSocket. The dashboard is a simulator control room. For CP-initiated behavior it drives the selected unit's own OCPP session via `ocpp-gateway`.
- **Naming** (`plan.md` §20): use specific names — `ChargePointInstance`, `ChargePointRegistry`, `OcppWebSocketClient`, `ConnectorStateMachine`, `MeterValueGenerator`, `RealtimeHub`, `RuntimeEventBus`, `ProtocolFactory`, `PendingCallRegistry`. Avoid `Manager`/`Helper`/`Util`/`Processor`.
- **Errors are visible in the UI** (`plan.md` §18): clear UI error + `runtime.error` log event when useful.
- **Two log kinds** kept conceptually separate (`plan.md` §19): OCPP message logs vs. runtime events.

## Testing

- Tests are the primary enforcement of the strict-compliance rule. Build → encode → validate against the official schema for every action; cover CALL/CALLRESULT/CALLERROR framing, decode, round-trip, correlation, and golden fixtures (`plan.md` Phase 4).
- A deliberately non-spec payload must be **rejected** by the validator in tests.
- `message-processor` responses and `ocpp-core` CSMS-initiated CALLs are validated against official schemas in tests.

## Current state

The current testable OCPP path is `web`/fake CP → `ocpp-gateway` → MQTT → `message-processor`/`ocpp-core`.

## Scope guardrails

This is a simulator, **not** an EVCMS. No billing, tariffs, RFID management, user management, multi-tenant, or OCPI. Keep packages small and testable. See `plan.md` §4.2 for the explicit non-goals and §22 for deferred features.
