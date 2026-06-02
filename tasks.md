# OCPP Simulator — Task Tracker

Persistent task list so any model/contributor can see where things stand. The authoritative design is in [`plan.md`](plan.md); operational guidance in [`AGENTS.md`](AGENTS.md).

## How to use this file

- Status legend: `[ ]` todo · `[~]` in progress · `[x]` done.
- When you start a task, set it to `[~]`; when fully done (tests green, no partial work), set `[x]` and note anything notable.
- **Respect `blocked by`** — don't start a task until its blockers are done.
- Two parallel tracks run side by side (see below). With one worker, follow IDs in order; with two, the **CSMS/Schema track** can advance independently and only needs to meet the simulator track at T17 (end-to-end).
- Keep this file in sync with reality. It is the handoff point between sessions/models.

## Tracks

- **🔧 Simulator track** — the main app: bootstrap → SQLite/CRUD → runtime → realtime → OCPP client → transactions → UX.
- **🛰️ CSMS/Schema track** — mock Central System + official OCPP schemas. Unblocks end-to-end testability; runs in parallel. Tasks: T4, T5, T12, T15.
- The tracks converge at **T17** (end-to-end connect + BootNotification): the OCPP client (T16) is first verified against the running mock CSMS (T15).

## Status overview

Next available: **T18**.

---

## Phase 0 — Repository Bootstrap

- [x] **T1 — Bootstrap monorepo** 🔧🛰️
  Turborepo + pnpm workspaces + `go.work` (apps/api, apps/csms, packages/ocpp-schemas). Root dev scripts (`dev`, `dev:api`, `dev:web`, `dev:csms`), basic README, docker ignore files.
  _Acceptance:_ `pnpm install` works; `go build ./...` works across the workspace.
  _Blocked by:_ —

- [x] **T2 — apps/api skeleton + health endpoint** 🔧
  Go service (`cmd/server/main.go`), `internal/` layout per plan §6.1, simple HTTP router, `/api/health`. Runtime independent of HTTP framework.
  _Blocked by:_ T1

- [x] **T3 — apps/web Vue dashboard shell** 🔧
  Vue 3 + Vite + TS + Pinia + Vue Router. Three-panel control-room layout placeholder (§14.2). Loads via `pnpm dev:web`.
  _Blocked by:_ T1

- [x] **T4 — apps/csms skeleton** 🛰️
  Mock CSMS skeleton (§10b): WebSocket accept at `ws://localhost:8080/ocpp/{cpId}`, `ocpp1.6` subprotocol, human-readable stdout frame logging. No handlers yet.
  _Blocked by:_ T1

- [x] **T5 — packages/ocpp-schemas scaffold** 🛰️
  Go module scaffold (§8.5): empty `validator.go` (`Validate(version, action, direction, payload)` signature), README placeholder for provenance, `go.mod`, wired into `go.work`. Schema files land in T12.
  _Blocked by:_ T1

## Phase 1 — SQLite & Basic CRUD

- [x] **T6 — SQLite migrations + tables** 🔧
  Migrations (goose/golang-migrate). All tables per §13: `charge_points`, `connectors` (+`evse_id`), `transactions` (dual identity `id` GUID + `numeric_id`), `ocpp_message_logs` (+`transaction_uuid`, `transaction_numeric_id`), `runtime_events`, `app_settings`. Data persists after restart.
  _Blocked by:_ T2

- [x] **T7 — Charge point + connector CRUD (API + repos)** 🔧
  DB repos + HTTP handlers (§12.1, §12.3). Soft-delete, next-available connector number, `evse_id` defaults to 1, active-transaction guard on connector deletion (§7.4).
  _Blocked by:_ T6

- [x] **T8 — CRUD Vue UI** 🔧
  Add/remove charge points and connectors: `ChargePointList`, `AddChargePointModal`, `AddConnectorModal`, `chargePointStore` ↔ REST.
  _Blocked by:_ T7, T3

## Phase 2 — Simulator Runtime Registry

- [x] **T9 — Simulator runtime registry** 🔧
  In-memory runtime (§6.2): `SimulatorRuntime`, `ChargePointRegistry`, `ChargePointInstance`, connector runtime state, `RuntimeEventBus`. Connect/disconnect as no-op state changes (no real OCPP yet). API can start/stop instances; runtime events produced internally.
  _Blocked by:_ T7

## Phase 3 — UI Realtime WebSocket

- [x] **T10 — UI realtime WebSocket hub** 🔧
  `/api/realtime` endpoint (§11): `RealtimeHub`, subscribe/unsubscribe by `chargePointId`, event filtering, `RealtimeEvent` model. Bridges `RuntimeEventBus` → subscribed clients.
  _Blocked by:_ T9

- [x] **T11 — Vue realtime client + LiveLogPanel** 🔧
  `realtimeClient.ts` + `realtimeStore` + `LiveLogPanel.vue`. Subscribe to selected CP; live events appear; changing selected unit changes subscription.
  _Blocked by:_ T10, T3

## Phase 4 — OCPP 1.6J Protocol Core

- [x] **T12 — Official 1.6J schemas + validator** 🛰️
  Add official OCPP 1.6J JSON schemas (7 MVP actions, request + response) to `packages/ocpp-schemas/v16/`. `//go:embed` + `Validate()`. README records provenance (OCA source, spec revision, license). Validator tests: wrong casing, missing required field, out-of-spec enum, unknown action all fail.
  _Note:_ official schema files come from OCA — confirm source with the user before adding.
  _Blocked by:_ T5

- [x] **T13 — OCPP core + 1.6J codec** 🔧
  `internal/ocpp` core (message model, `Protocol` interface, transport, codec, errors) + v16 impl (§8, §9). `Build*` for 7 actions, encode/decode CALL/CALLRESULT/CALLERROR, unique ID generation, `PendingCallRegistry`. **STRICT:** wire payloads spec-exact, internal models never leak (§2b).
  _Blocked by:_ T12

- [x] **T14 — Protocol core test suite** 🔧
  Concrete tests (plan §Phase 4): build→schema-validate for all 7 actions, CALL framing, decode (CALLRESULT/CALLERROR/malformed), round-trip, correlation (uniqueId match, `StartTransaction.conf`→`numeric_id`), golden `testdata` fixtures. `go test ./...` passes; deliberately non-spec payload rejected by validator.
  _Blocked by:_ T13

- [x] **T15 — CSMS happy-path handlers** 🛰️
  Mock CSMS action→response handlers for all 7 actions (§10b.4), in-memory incrementing `transactionId` counter (closes async loop §7.6), CALLERROR for unknown actions. Responses spec-exact and validated against `...Response.json` in tests (§8.5).
  _Blocked by:_ T4, T12

## Phase 5 — OCPP WebSocket Client

- [x] **T16 — OCPP WebSocket client** 🔧
  Per-charge-point `OcppWebSocketClient` (§10): connect/disconnect/reconnect, send, response receive loop, timeout handling, persistence to `ocpp_message_logs`, realtime publishing, connection URL building.
  _Blocked by:_ T13, T9

- [x] **T17 — End-to-end connect + BootNotification** 🔧🛰️ (tracks converge)
  Wire runtime → OCPP client → mock CSMS. CP connects to CSMS URL, BootNotification sent on command, CSMS replies Accepted, response shown live, clean disconnect.
  _Blocked by:_ T16, T15

## Phase 6 — Transaction Simulation

- [ ] **T18 — Transaction simulation** 🔧
  Authorize, StartTransaction, StopTransaction, MeterValues. `ActiveTransactionState` with async `numeric_id` (§7.6), `ConnectorStateMachine` (§7b), `MeterValueGenerator` linear Wh model (§7b.4). Transactions persisted; connector transitions Available→Preparing→Charging→Finishing.
  _Blocked by:_ T17

- [ ] **T19 — Connector control UI** 🔧
  `ConnectorCard.vue` with Set Status / Start / Stop / Send MeterValues / Fault (§14.5). Live status, active session, meter increasing while charging. `StartTransactionModal`.
  _Blocked by:_ T18, T11

## Phase 7 — UX Polish

- [ ] **T20 — UX polish** 🔧
  Dashboard layout, log filters (type/connector/ocpp-runtime-errors), `JsonPayloadDrawer`, `RuntimeStatusBadge`, `SettingsPage` + settings API (§12.6), error display in timeline (§18).
  _Blocked by:_ T19

## Phase 8 — Multi-Version Readiness

- [ ] **T21 — Multi-version readiness** 🔧🛰️
  `ProtocolFactory` wiring, version stored per CP, UI version selector, `v201` placeholder package + schemas dir, clear unsupported-version handling. Adding a version must not require heavy runtime changes.
  _Blocked by:_ T18

---

## Dependency map

```
T1 ─┬─ T2 ── T6 ── T7 ─┬─ T9 ── T10 ── T11 ─┐
    │                  │                     │
    │                  └─ (T7) ──────────────┼─ T8 (needs T3)
    ├─ T3 ─────────────────────────────────┘
    │
    ├─ T4 ─────────────┐
    └─ T5 ── T12 ─┬────┴─ T15 ─┐
                  ├─ T13 ── T14 │
                  └─ T13 ── T16 ┴─ T17 ── T18 ─┬─ T19 (needs T11) ── T20
                       (T16 needs T9)          └─ T21
```

Convergence point: **T17** (T16 simulator client × T15 mock CSMS).
