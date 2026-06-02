# OCPP Simulator — Agent Guide

A local-first, web-based **OCPP charge point simulator** for developers testing EV charging backends. Go backend + Vue frontend + SQLite, in a Turborepo/pnpm + Go workspace monorepo. Targets OCPP 1.6J first, designed for OCPP 2.0.1 later.

The authoritative design lives in [`plan.md`](plan.md). Read it before non-trivial work. This file is the quick operational guide.

## ⚠️ Hard rule: strict OCPP compliance

**Everything that goes on the wire MUST conform exactly to the official OCPP spec for the negotiated version.** No custom, simplified, renamed, or localized models, fields, casing, enums, or message framing — ever. See `plan.md` §2b.

- Wire payloads are spec-exact: field names/casing (`chargePointVendor`, `idTagInfo`, `meterStart`), types, required/optional, value ranges, enum strings all match the official JSON schema character-for-character.
- OCPP-J framing exactly: `[2, uniqueId, action, payload]` (CALL), `[3, uniqueId, payload]` (CALLRESULT), `[4, uniqueId, errorCode, errorDescription, errorDetails]` (CALLERROR). Standard error-code set only.
- WebSocket subprotocol is the standard one (`ocpp1.6`, `ocpp2.0.1`).
- **Internal models never leak to the wire.** The generic domain model (transaction GUID + `numericId` dual identity, `ConnectorStateMachine`, runtime events) is internal only. The version `codec` is the single boundary that translates internal state ↔ spec-exact wire payloads.
- The mock CSMS (`apps/csms`) is also bound by this: it may choose *which* valid response to send, but every response must be valid OCPP.
- Official schemas are the single source of truth in `packages/ocpp-schemas`; code conforms to them, never the other way around. Payloads are validated against them in tests.

If a value isn't defined by the OCPP spec, it must not appear in an OCPP message.

## Architecture

Two WebSocket layers — keep them separate:
1. **Go simulator → OCPP Central System** (OCPP protocol traffic, one client connection per charge point).
2. **Vue client → Go API** (`/api/realtime`, live logs/state).

Runtime vs. persistence: active connections, timers, loops, and transaction state live **in memory**; SQLite stores config + history. SQLite is not the source of truth for live runtime state.

Transaction identity is **dual** (`plan.md` §7.6, §8.4, §13): every transaction has an internal GUID (`transactions.id`, also the 2.0.1 wire id) and a `numeric_id` (1.6J wire id, assigned asynchronously by the CSMS in `StartTransaction.conf`).

## Repo layout

```
apps/
  api/              # Go API + simulator runtime (cmd/server, internal/{api,app,db,migrations,simulator,ocpp,realtime,config,common})
  web/              # Vue 3 + Vite + TS + Pinia
  csms/             # Mock OCPP Central System (standalone test server, no DB)
packages/
  ocpp-schemas/     # Go module: official OCPP JSON schemas (single source of truth) + embed + Validate()
  shared/           # Shared TS types / generated client
  config/           # Shared frontend config
go.work             # Links apps/api, apps/csms, packages/ocpp-schemas
plan.md             # Full architecture & development plan (authoritative)
```

## Dev commands

```
pnpm install            # install JS deps
pnpm dev                # turbo dev (web + api)
pnpm dev:api            # Go API only
pnpm dev:web            # Vue only
pnpm dev:csms           # mock Central System
go build ./...          # build all Go modules (run from repo root via go.work)
go test ./...           # run Go tests (apps/api, apps/csms, packages/ocpp-schemas)
```

Default ports/URLs: API/UI WS `ws://localhost:7070/api/realtime`; mock CSMS `ws://localhost:8080/ocpp/{chargePointId}`.

## Conventions

- **Backend stack**: Go, simple HTTP router, sqlc/lightweight DB layer, goose/golang-migrate migrations, gorilla or nhooyr WebSocket. Keep the simulator runtime independent of the HTTP framework and OCPP independent of persistence.
- **Frontend stack**: Vue 3, Vite, TS, Pinia, Vue Router, native WebSocket. REST = command/query layer; WebSocket = live event layer. The dashboard is a "control room", not a generic CRUD panel.
- **Naming** (`plan.md` §20): use specific names — `ChargePointInstance`, `ChargePointRegistry`, `OcppWebSocketClient`, `ConnectorStateMachine`, `MeterValueGenerator`, `RealtimeHub`, `RuntimeEventBus`, `ProtocolFactory`, `PendingCallRegistry`. Avoid `Manager`/`Helper`/`Util`/`Processor`.
- **Errors are visible in the UI** (`plan.md` §18): clear API error + `runtime.error` realtime event + persisted runtime event when useful.
- **Two log kinds** kept conceptually separate (`plan.md` §19): OCPP message logs vs. runtime events.

## Testing

- Tests are the primary enforcement of the strict-compliance rule. Build → encode → validate against the official schema for every action; cover CALL/CALLRESULT/CALLERROR framing, decode, round-trip, correlation, and golden fixtures (`plan.md` Phase 4).
- A deliberately non-spec payload must be **rejected** by the validator in tests.
- The mock CSMS's responses are validated against `...Response.json` schemas in its tests.

## Working with tasks

Work is tracked in [`tasks.md`](tasks.md) — the persistent handoff point between sessions/models. Read it first to see where things stand, update statuses as you go (`[ ]`→`[~]`→`[x]`), and keep it in sync with reality. Two parallel tracks:
- **Simulator track**: bootstrap → SQLite/CRUD → runtime → realtime → OCPP client → transactions → UX.
- **CSMS/schema track**: csms skeleton → official schemas + validator → csms handlers → response schema tests. This track unblocks end-to-end testability and can advance in parallel.

Respect task `blockedBy` dependencies; prefer lowest available ID. Do not mark a task complete with failing tests or partial implementation.

## Scope guardrails

This is a simulator, **not** an EVCMS. No billing, tariffs, RFID management, user management, multi-tenant, or OCPI. Keep packages small and testable. See `plan.md` §4.2 for the explicit non-goals and §22 for deferred features.
