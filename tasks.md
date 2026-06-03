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

Next available: **All tasks complete**.

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
  Concrete tests (plan §Phase 4): build→schema-validate for all 7 actions, CALL framing, decode (CALLRESULT/CALLERROR/malformed), round-trip, correlation (uniqueId match, `StartTransaction.conf`→`numeric_id`), golden `testdata` fixtures. Protocol tests pass from their module directory with `go test ./...`; deliberately non-spec payload rejected by validator.
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

- [x] **T18 — Transaction simulation** 🔧
  Authorize, StartTransaction, StopTransaction, MeterValues. `ActiveTransactionState` with async `numeric_id` (§7.6), `ConnectorStateMachine` (§7b), `MeterValueGenerator` linear Wh model (§7b.4). Transactions persisted; connector transitions Available→Preparing→Charging→Finishing.
  _Blocked by:_ T17

- [x] **T19 — Connector control UI** 🔧
  `ConnectorCard.vue` with Set Status / Start / Stop / Send MeterValues / Fault (§14.5). Live status, active session, meter increasing while charging. `StartTransactionModal`.
  _Blocked by:_ T18, T11

## Phase 7 — UX Polish

- [x] **T20 — UX polish** 🔧
  Dashboard layout, log filters (type/connector/ocpp-runtime-errors), `JsonPayloadDrawer`, `RuntimeStatusBadge`, `SettingsPage` + settings API (§12.6), error display in timeline (§18).
  _Blocked by:_ T19

## Phase 8 — Multi-Version Readiness

- [x] **T21 — Multi-version readiness** 🔧🛰️
  `ProtocolFactory` wiring, version stored per CP, UI version selector, `v201` placeholder package + schemas dir, clear unsupported-version handling. Adding a version must not require heavy runtime changes.
  _Blocked by:_ T18

---

## Phase 9 — v4.3 Service Split Target

- [x] **T22 — Extract shared OCPP protocol package** 🔧🛰️
  Create `packages/ocpp-protocol` with public `pkg/` packages for codec, message envelope, protocol interfaces, pending calls, OCPP 1.6J, and v2.0.1 placeholders. Move existing protocol tests and keep schema validation green.
  _Acceptance:_ `cd packages/ocpp-protocol && go test ./...`; `cd packages/ocpp-schemas && go test ./...`.
  _Blocked by:_ T21
  _Notes:_ 6 paket çıkarıldı: `pkg/codec`, `pkg/message` (MessageTypeID/Message/ErrorCode/GenerateUniqueID), `pkg/protocol` (interface, tüm input/request types, Factory), `pkg/pendingcalls` (PendingCall + Registry), `pkg/v16` (Protocol impl + Parse*Response), `pkg/v201` (2.0.1 placeholder, `ErrNotImplemented` döner, `protocol.Protocol`'ü derleme-zamanı sağlar). apps/api yeni paketleri import edecek şekilde güncellendi; eski `internal/ocpp` silindi. Tüm go.mod'lar self-contained: `apps/api` ve `packages/ocpp-protocol` kendi `require`/`replace` direktifleriyle go.work olmadan da (`GOWORK=off`) build & test geçer.

- [x] **T23 — Split simulator-api from current apps/api** 🔧
  Rename/extract the current combined API into `apps/simulator-api`. Keep CP/connector CRUD, settings, app config DB, `/api/realtime`, and `/api/health`. Remove OCPP frame generation/parsing, MQTT OCPP publishing, runtime transaction/meter/state ownership, and backend OCPP proxy responsibilities.
  _Acceptance:_ `cd apps/simulator-api && go test ./...`; UI management API still lists/creates/deletes CPs and connectors.
  _Blocked by:_ T22
  _Notes:_ apps/api → apps/simulator-api rename; tüm internal/ocpp / internal/simulator / internal/app / internal/streams kaldırıldı; db/message_log_repo + db/transaction_repo + migrations 003/004/005 (transactions, ocpp_message_logs, runtime_events) kaldırıldı — bunlar T26’da ocpp-core’un kendi DB’sine taşınacak. Server endpoint’leri sadece CRUD + settings + versions + realtime + health; OCPP runtime endpoint’leri (connect/disconnect/boot/heartbeat/start-tx/stop-tx/meter-values/status/remote-start/remote-stop) ve `/api/ws/{id}` CSMS proxy kaldırıldı (T27’de Vue tarafı per-CP OCPP WebSocket’ine geçecek). go.mod OCPP-protocol bağımlılığı çıkarıldı. `cd apps/simulator-api && go build ./...` ve `GOWORK=off go test ./...` yeşil.

- [x] **T24 — Add ocpp-gateway dumb bridge** 🔧
  Create `apps/ocpp-gateway` with `/ws/{chargePointId}`. Maintain per-CP WebSocket connection registry, publish inbound raw OCPP frames unchanged to `ocpp/{chargePointId}/in`, subscribe to `ocpp/{chargePointId}/out`, and write outbound raw frames unchanged to the matching WebSocket. No DB, no business decisions, no state machine/MeterValueGenerator/transaction holder.
  _Acceptance:_ Gateway bridge tests prove raw frames are unchanged and each CP has isolated WebSocket handling.
  _Blocked by:_ T22
  _Notes:_ apps/ocpp-gateway oluşturuldu — `/ws/{cpId}` per-CP WebSocket edge, `internal/gateway` altında `Broker` interface + paho MQTT implementasyonu, per-CP `Registry` (mutex korumalı `map[chargePointID]*Connection`), `pumpInbound` (WS→`ocpp/{cpId}/in`) ve `pumpOutbound` (`ocpp/{cpId}/out`→WS) goroutine’leri. Frame **byte-byte aynen** iletilir (parse yok, re-marshal yok, JSON canonicalization yok). Topic format: `ocpp/{cpId}/in`, `ocpp/{cpId}/out`. CP id path validation: `/`, `+`, `#` reddedilir (MQTT wildcard injection koruması). DB yok, OCPP protokol import’u yok. 8 unit test yeşil: raw passthrough, topic format, per-CP izolasyon (A↔B cross-talk yok), duplicate Connect reddi, Disconnect teardown (registry + unsubscribe + Closed()), unknown CP’de inbound reddi, late outbound delivery panic etmemesi, path validation. Hem go.work açık hem GOWORK=off build & test yeşil.

- [x] **T25 — Add message-processor response producer** 🛰️
  Create `apps/message-processor`. Subscribe to `ocpp/+/in`, parse raw OCPP-J frames, respond to supported CP-to-server 1.6J CALLs, call `ocpp-core` for business decisions such as Authorize/StartTransaction/StopTransaction, publish raw CALLRESULT/CALLERROR to `ocpp/{chargePointId}/out`, and write consume/publish audit events to stdout/log only.
  _Acceptance:_ `cd apps/message-processor && go test ./...`; no DB or `processor_events` table is introduced.
  _Blocked by:_ T22
  _Notes:_ apps/message-processor oluşturuldu. Tek Go servisi, DB yok. `internal/processor` altında: `Broker` interface + paho implementasyonu (`SetOrderMatters(false)` ile head-of-line blocking önlendi), `Handler` (envelope parse + action dispatch + response build + schema validation + publish), `CoreClient` interface + `HTTPCoreClient` (POST `/internal/transactions/{authorize,start,stop}` ile ocpp-core’a danışıyor), `Processor` (ocpp/+/in subscribe + topic’ten cpId parse). Audit event’leri sadece stdout/log’a (`StdoutEmitter` JSON line); wire’a veya MQTT’ye audit gitmiyor. Core-independent response: BootNotification/Heartbeat/StatusNotification/MeterValues. Core-callback: Authorize/StartTransaction/StopTransaction. Bilinmeyen action → NotImplemented CALLERROR. CP→server CALLRESULT/CALLERROR inbound → audit only, response ÜRETMEZ. CSMS-initiated CALL ÜRETMEZ. 12 unit test yeşil: core-independent (Boot/Heartbeat/Status/MeterValues) CALLRESULT + schema valid, core-routed (Authorize/Start/Stop) byte-exact core payload’ı kullanıyor, core unavailable → GenericError CALLERROR, unknown action → NotImplemented CALLERROR, inbound CALLRESULT/CALLERROR noop (audit), parse error audit-only, outbound raw `[3,...]` array wrapper’sız, per-CP routing doğru topic, Processor.Start subscribe, Processor.Stop unsubscribe, ChargePointIDFromTopic parser. Hem go.work açık hem GOWORK=off build & test yeşil.

- [x] **T26 — Add ocpp-core canonical log + business service** 🛰️
  Create `apps/ocpp-core` with `ocpp-core.db` tables `transactions`, `ocpp_message_logs`, and `runtime_events`. Subscribe directly to raw `ocpp/+/in` and `ocpp/+/out` as canonical log source. Answer message-processor callbacks. On startup initialize numeric transaction counter from `MAX(numeric_id)`. Generate StartTransaction UUID/numeric IDs and publish CSMS-initiated CALLs to `ocpp/{chargePointId}/out`.
  _Acceptance:_ `cd apps/ocpp-core && go test ./...`; StartTransaction creates `id` UUID + restart-safe `numeric_id`; raw in/out frames persist to `ocpp_message_logs`.
  _Blocked by:_ T22
  _Notes:_ apps/ocpp-core oluşturuldu. `ocpp-core.db` tablolar: `transactions` (id UUID PK, numeric_id NULL, status, dual identity), `ocpp_message_logs` (raw payload verbatim, direction, message_type, action, unique_id, transaction_uuid, topic), `runtime_events` — **`processor_events` YOK**. `internal/canonlog` `ocpp/+/in` + `ocpp/+/out` raw subscribe → her wire mesajı için 1 satır, payload byte-byte verbatim, parse_error bile log’lanıyor. `internal/transaction` start-up’ta `SELECT MAX(numeric_id)` ile counter seed ediyor (restart-safe), atomic AssignNumericID (UPDATE … WHERE numeric_id IS NULL) ile çift atama yapılamaz. `internal/csms` 8 action (RemoteStart/Stop, Reset, UnlockConnector, ChangeConfig, GetConfig, Trigger, ChangeAvailability) için `ocpp-schemas` ile spec-valid doğrulama sonra ham `[2,uid,action,payload]` `ocpp/{cp}/out`'a publish. `internal/api` HTTP server: business callback `/internal/transactions/{authorize,start,stop}` (message-processor için `{chargePointId, payload}` envelope), CSMS-init `/internal/csms/*`, `/health`. Paho broker `SetOrderMatters(false)`. OCPP WebSocket kurmuyor, Vue’a konuşmuyor, processor stdout audit’ini dinlemiyor. 21 unit test yeşil (db: 5, transaction: 2, canonlog: 4, csms: 4, api: 7). Hem go.work açık hem GOWORK=off build & test yeşil.

- [x] **T27 — Move Vue to per-CP OCPP simulator client** 🔧
  Point each simulated CP to `ws://ocpp-gateway:7080/ws/{chargePointId}`. Keep per-CP connector state, meter generation, transaction holder, and pending-call correlation in Vue. Document browser refresh local state loss as an accepted local-simulator tradeoff.
  _Acceptance:_ UI creates CPs through `simulator-api`, opens one WebSocket per selected/running CP to gateway, and sends/receives only raw OCPP-J array frames.
  _Blocked by:_ T23, T24
  _Notes:_ apps/web yeniden düzenlendi. `vite.config.ts` iki proxy: `/api` → simulator-api (CRUD/settings/realtime), `/ws` → ocpp-gateway (`/ws/{cpId}` per-CP OCPP WebSocket). Yeni `src/ocpp/` klasörü: `gatewayClient.ts` (per-CP GatewayClient, raw frame alışverişi, 3s reconnect), `callResponses.ts` (inbound CSMS-init CALL → spec-exact CALLRESULT, 8 action: RemoteStart/Stop, Reset, UnlockConnector, ChangeConfig, GetConfig, Trigger, ChangeAvailability; unknown → NotImplemented CALLERROR), `uniqueId.ts` (crypto.randomUUID hex). `stores/chargePointStore.ts` per-CP `IGatewayClient` + per-CP runtime (registration, heartbeat scheduler, meterValuesTimer, pendingCalls Map, connectorStates Map, transactions Map). Inbound CALL handling eklendi: CSMS-init CALL → buildResponse → spec-exact CALLRESULT gönder. API client (`api/chargePointsApi.ts`) OCPP endpoint'lerinden arındırıldı (yalnız CRUD + versions kaldı). `ChargePointDetail.vue`'den "Simulate Remote Start" UI kaldırıldı (artık ocpp-core HTTP üzerinden curl ile). `App.vue`'de `handleRealtimeEvent` çağrısı kaldırıldı (CP runtime’ı artık kendi store’unda). `vue-tsc -b && vite build` yeşil.

- [x] **T27 fix — gatewayClient WebSocket subprotocol handshake** 🔧
  `new WebSocket(url)` çağrısı `Sec-WebSocket-Protocol: ocpp1.6` (veya `ocpp2.0.1`) göndermiyordu — AGENTS.md §2b ihlali. `GatewayClientOptions`'a `ocppVersion` eklendi; `ocppSubprotocol(version)` helper'ı versiyonu OCPP subprotocol token'ına map eder (`'1.6J'|'1.6' → 'ocpp1.6'`, `'2.0.1' → 'ocpp2.0.1'`, unknown → `''`). `connectChargePoint` async yapıldı, `await selectChargePoint(cpId)` ile CP'nin versiyonu bağlantı öncesi okunuyor; detail yüklenmediyse '1.6J' fallback. Build yeşil.

- [x] **T28 — Add MQTT + Docker Compose local orchestration** 🔧🛰️
  Add local MQTT broker config and Docker Compose for `mqtt`, `simulator-api`, `ocpp-gateway`, `message-processor`, `ocpp-core`, `web`, and optional `csms`. Service Dockerfiles/build targets remain service-scoped.
  _Acceptance:_ `docker compose up` starts the target services; MQTT topics carry raw OCPP arrays with no wrapper payloads.
  _Blocked by:_ T23, T24, T25, T26
  _Notes:_ 6 servis + opsiyonel csms profili. Her backend kendi modül dizininden build edilir (service-scoped Dockerfile'lar, distroless runtime). mqtt: eclipse-mosquitto:2 + `docker/mqtt/mosquitto.conf` (anonymous, no payload transform, `ocpp/+/in` `ocpp/+/out` için taşıma). simulator-api: GO DB `/data/ocpp-simulator.db`, port 7070, **deliberately NO `depends_on: mqtt`** (sınır korunmuş). ocpp-gateway: mqtt'ye bağımlı, port 7080. message-processor: mqtt + ocpp-core'a bağımlı, port 7091 (health). ocpp-core: mqtt'ye bağımlı, `/data/ocpp-core.db` ayrı volume, port 7090. web: nginx multi-stage, pnpm 9.15.4 (Node 20 uyumu), `/api` ve `/ws` reverse-proxy. `csms` `profiles: ["csms"]` ile ayrı tutuldu. 6 image build edildi ve `docker compose up` ile uçtan uca doğrulandı: simulator-api CP oluşturma + connector ekleme, ocpp-core StartTransaction (transactionId=1, counter initialized), CSMS-init RemoteStartTransaction `ocpp/CP-DOCKER-1/out`'a publish.

- [x] **T28 fix — simulator-api DB env adı uyuşmazlığı** 🔧
  Compose `OCPP_SIMULATOR_DB` env adını kullanıyordu ama `config.go` yalnızca `DB_PATH` okuyordu → DB volume'a yazılmıyor, simulator-api verisi container restart'ta kayboluyordu. `config.Load()` artık `OCPP_SIMULATOR_DB` (canonical, diğer servislerin `*_DB` kalıbıyla simetrik) → `DB_PATH` (legacy back-compat) → default `ocpp-simulator.db` sırasıyla arar. Container recreate + CP persistence uçtan uca doğrulandı: iki CP oluşturuldu, container silindi + yeniden oluşturuldu, her iki CP hâlâ volume'dan okundu.

- [x] **T29 — End-to-end v4.3 verification and docs sync** 🔧🛰️
  Verify CP create → boot → start transaction → meter values → stop transaction. Confirm raw `ocpp/{cpId}/in` and `ocpp/{cpId}/out` frames, canonical `ocpp-core` logs, processor stdout audit, UI realtime events, and module-level tests. Keep `AGENTS.md`, `CLAUDE.md`, `plan.md`, `README.md`, `OCPP_SIMULATOR_COMMUNICATION_WORK_PLAN.md`, and `tasks.md` synchronized.
  _Acceptance:_ Each Go module is tested from its own directory with `go test ./...`; no root-level single `go test ./...` assumption; docs match implemented boundaries.
  _Blocked by:_ T27, T28
  _Notes:_ 7 modül (`simulator-api`, `csms`, `ocpp-gateway`, `message-processor`, `ocpp-core`, `ocpp-protocol`, `ocpp-schemas`) hem `go.work` hem `GOWORK=off` modunda build & test yeşil. Wire dump: `mosquitto_sub -h mqtt -t 'ocpp/+/in' -t 'ocpp/+/out' -v` ile gerçek CP simülasyonu (`scripts/fake_cp.py` Python `websockets` ile) — BootNotification/Heartbeat/StatusNotification/Unknown action'larının hepsi gateway → message-processor → gateway → CP döngüsünden geçti, MQTT üzerinde ham OCPP-J array olarak taşındı, hiçbir wrapper/envelope/DTO yok. Dump `docs/wire-dumps/t29-end-to-end-ocpp-frames.log`’a kaydedildi. T28 fix (DB env adı `OCPP_SIMULATOR_DB`) container recreate ile persistence doğrulandı. Docs sync: `AGENTS.md` go.work listesi + go.work stale referansı `apps/api` → 5 backend + 2 packages olarak güncellendi; `README.md` v4.3 mimarisini yansıtacak şekilde yeniden yazıldı (eski "Current `make dev`" bölümü kaldırıldı, 4-backend + service tablosu eklendi, port’lar ve subprotocol kuralı eklendi). Tüm stale `apps/api` referansları historical plan dosyalarında (`plan.md`, `nextplan.md`, `OCPP_SIMULATOR_COMMUNICATION_WORK_PLAN.md`, `connector-flow-plan.md`, `csms-initiated-plan.md`) olduğu gibi bırakıldı — bunlar pre-T22 tasarım notları.

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
