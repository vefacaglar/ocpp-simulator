# OCPP Simulator Architecture & Development Plan

## 1. Project Summary

This project is a local-first, web-based OCPP simulator for developers who need to test EV charging backends without relying on physical charge points or limited third-party simulators.

The application should support multiple simulated charge points, multiple connectors per charge point, dynamic unit and connector management, live OCPP message logs, runtime state inspection, and realistic charging session simulation.

The first production target is OCPP 1.6J, but the internal architecture must be designed for multiple OCPP protocol versions from the beginning. OCPP 2.0.1 support should be possible later without rewriting the simulator runtime.

The application runs locally and consists of:

- Go API backend
- Go simulator runtime
- Go WebSocket client for OCPP Central System communication
- Go WebSocket server for live UI updates
- Go mock OCPP Central System (a standalone test server, not connected to any real backend)
- Vue web client
- SQLite local database
- Turborepo monorepo structure

The intended final form is a developer tool that can be started locally, opened in the browser, and used to create, connect, control, and inspect simulated OCPP charge points.

---

## 2. Core Product Goal

The goal is not to build a full EVCMS.

The goal is to build a reliable, developer-friendly OCPP charge point simulator.

The tool should answer these questions clearly:

- What did my simulated charge point send to the backend?
- What did the backend respond with?
- What is the current state of each charge point?
- What is the current state of each connector?
- Which connector is charging?
- Which transaction is active?
- Are MeterValues being sent correctly?
- Can I simulate disconnects, reconnects, faults, and abnormal session flows?
- Can I run many charge points against the same backend?

---

## 3. Target Users

Primary users:

- Backend developers building OCPP Central Systems
- QA engineers testing charging backends
- EV charging platform developers
- Developers learning OCPP
- Teams that need repeatable local simulation scenarios

Secondary users:

- Developers preparing demos
- Developers testing WebSocket behavior
- Developers validating charging session lifecycle logic

---

## 4. Initial Scope

### 4.1 MVP Scope

The MVP should support:

- Local web application
- Vue-based dashboard
- Go API backend
- SQLite persistence
- Multiple charge points
- Multiple connectors per charge point
- Dynamic charge point creation
- Dynamic charge point deletion
- Dynamic connector creation
- Dynamic connector deletion
- OCPP 1.6J WebSocket client connection
- Charge point connect/disconnect
- BootNotification
- Heartbeat
- StatusNotification
- Authorize
- StartTransaction
- MeterValues
- StopTransaction
- Live message logs through WebSocket
- Selected charge point log subscription from the Vue client
- Active session display
- Basic runtime events
- Message history persisted to SQLite
- Session history persisted to SQLite

### 4.2 Explicit Non-MVP Scope

These should not be implemented in the first MVP unless the core system is already stable:

- Full OCPP 2.0.1 support
- OCPI
- Payment flows
- Billing
- Invoice generation
- RFID card management system
- User management
- Multi-tenant backend
- Cloud hosting
- Mobile app
- Complex scenario runner
- Load testing mode
- Advanced tariff engine
- Full EVCMS features

---

## 5. Technology Stack

### 5.1 Monorepo

Use Turborepo with pnpm workspaces.

Recommended structure:

```txt
ocpp-simulator/
├─ apps/
│  ├─ api/              # Go API + simulator runtime
│  ├─ web/              # Vue client
│  └─ csms/             # Go mock OCPP Central System (test server)
│
├─ packages/
│  ├─ shared/           # Generated TypeScript API client or shared types
│  └─ config/           # Shared frontend config if needed
│
├─ docs/
├─ docker/
├─ turbo.json
├─ package.json
├─ pnpm-workspace.yaml
└─ README.md
```

### 5.2 Backend

Recommended backend stack:

- Go
- SQLite
- sqlc or lightweight database access layer
- goose or golang-migrate for migrations
- gorilla/websocket or nhooyr.io/websocket for WebSocket handling
- chi, echo, fiber, or standard net/http for routing

Preferred direction:

- Use a simple HTTP router.
- Keep the simulator runtime independent from the HTTP framework.
- Keep OCPP protocol implementation independent from persistence.
- Avoid over-frameworking the project.

### 5.3 Frontend

Recommended frontend stack:

- Vue 3
- Vite
- TypeScript
- Pinia
- Vue Router
- Native WebSocket client
- Lightweight component structure

Optional UI choices:

- Tailwind CSS
- shadcn-vue
- Radix Vue
- Headless UI

The UI should feel like a simulator control room, not like a generic CRUD admin panel.

---

## 6. Backend Architecture

### 6.1 Backend Folder Structure

Recommended Go backend structure:

```txt
apps/api/
├─ cmd/
│  └─ server/
│     └─ main.go
│
├─ internal/
│  ├─ api/                  # HTTP route handlers
│  ├─ app/                  # Application services / command orchestration
│  ├─ db/                   # SQLite queries and repositories
│  ├─ migrations/           # Database migrations
│  ├─ simulator/            # Runtime registry and charge point instances
│  ├─ ocpp/                 # OCPP protocol abstraction and version packages
│  ├─ realtime/             # UI WebSocket hub
│  ├─ config/               # App configuration
│  └─ common/               # Shared internal utilities
│
├─ go.mod
└─ go.sum
```

### 6.2 Runtime Design

The simulator runtime is the core of the backend.

It should manage active charge point instances in memory.

SQLite stores configuration and history, but active WebSocket connections, timers, loops, subscriptions, and transaction state should live in memory.

High-level runtime structure:

```txt
SimulatorRuntime
 ├─ ChargePointRegistry
 ├─ ChargePointInstance CP-001
 │   ├─ OcppWebSocketClient
 │   ├─ ConnectorRegistry
 │   ├─ HeartbeatLoop
 │   ├─ MeterValueLoop
 │   └─ ActiveTransactionState
 │
 ├─ ChargePointInstance CP-002
 │   ├─ OcppWebSocketClient
 │   ├─ ConnectorRegistry
 │   ├─ HeartbeatLoop
 │   ├─ MeterValueLoop
 │   └─ ActiveTransactionState
 │
 └─ EventBus
```

The runtime should expose command-style methods:

```go
type SimulatorRuntime interface {
    StartChargePoint(ctx context.Context, chargePointID string) error
    StopChargePoint(ctx context.Context, chargePointID string) error
    RestartChargePoint(ctx context.Context, chargePointID string) error

    AddChargePoint(ctx context.Context, input CreateChargePointInput) error
    RemoveChargePoint(ctx context.Context, chargePointID string) error

    AddConnector(ctx context.Context, chargePointID string, input AddConnectorInput) error
    RemoveConnector(ctx context.Context, chargePointID string, connectorID int) error

    SetConnectorStatus(ctx context.Context, chargePointID string, connectorID int, status ConnectorStatus) error

    SendBootNotification(ctx context.Context, chargePointID string) error
    SendHeartbeat(ctx context.Context, chargePointID string) error
    SendStatusNotification(ctx context.Context, chargePointID string, connectorID int) error

    StartTransaction(ctx context.Context, input StartTransactionInput) error
    StopTransaction(ctx context.Context, input StopTransactionInput) error
    SendMeterValues(ctx context.Context, chargePointID string, connectorID int) error
}
```

---

## 7. Important Runtime Rules

### 7.1 Charge Point Creation

When a charge point is created from the UI:

1. Persist charge point configuration to SQLite.
2. Create connectors in SQLite.
3. Register the charge point in the in-memory runtime registry.
4. Do not connect automatically unless `autoConnect` is enabled.
5. Publish a runtime event.

### 7.2 Charge Point Deletion

When a charge point is deleted:

1. If connected, disconnect it first.
2. Stop heartbeat loop.
3. Stop meter value loops.
4. Close WebSocket connection.
5. Remove from runtime registry.
6. Soft-delete or delete DB records depending on persistence policy.
7. Preserve message/session history if possible.

Recommended behavior: soft-delete charge points to avoid breaking historical logs.

### 7.3 Connector Creation

When a connector is added to a charge point:

1. Assign the next available connector number by default.
2. Persist connector to SQLite.
3. Add connector state to the in-memory charge point instance.
4. Default status should be `Available`, unless specified otherwise.
5. Publish a runtime event.
6. If the charge point is connected, optionally send StatusNotification for the new connector.

### 7.4 Connector Deletion

A connector should not be deleted if it has an active transaction.

Allowed deletion flow:

1. Check active transaction.
2. If active transaction exists, return validation error.
3. If no active transaction, remove from runtime state.
4. Soft-delete connector in SQLite.
5. Publish a runtime event.

### 7.5 Active Transactions

Only one active transaction should exist per connector.

Before starting a transaction:

- Charge point must be connected.
- Connector must exist.
- Connector must not already have an active transaction.
- Connector should be in a valid state, usually `Available` or `Preparing`.

Before stopping a transaction:

- Connector must have an active transaction.
- StopTransaction should include transaction ID, meter stop value, timestamp, and reason.

### 7.6 Asynchronous Transaction ID Assignment (OCPP 1.6J)

In OCPP 1.6J the `transactionId` is **not** known when the simulator starts a transaction. It is assigned by the Central System and returned in `StartTransaction.conf`. The runtime must therefore model transaction start as a two-step asynchronous flow:

1. User triggers Start Transaction on a connector.
2. Runtime validates state (see 7.5) and moves the connector to a pending state.
3. Runtime sends `StartTransaction.req` and records a pending call (see 10.3).
4. `ActiveTransactionState` is created in a `pending_start` status with no `transactionId` yet.
5. When `StartTransaction.conf` arrives:
   - Match by unique ID.
   - Read `transactionId` and `idTagInfo.status`.
   - If `Accepted`, fill `transactionId`, move connector to `Charging`, start the meter value loop, persist the session row, and publish `transaction.started`.
   - If rejected (`Blocked`, `Expired`, `Invalid`, `ConcurrentTx`), abort the pending transaction, revert connector status, and publish `transaction.failed`.
6. If the `StartTransaction.conf` times out, abort the pending transaction, revert connector status, and publish `transaction.failed` + `ocpp.response_timeout`.

`StopTransaction` is simpler because the `transactionId` is already known locally, but its `.conf` should still be correlated and may update the session row's final state.

Implication for state design: `ActiveTransactionState` must support a `transactionId` that is initially nil/zero and is populated asynchronously. UI must tolerate a short window where a connector is "starting" but has no transaction ID yet.

---

## 7b. Connector State Machine

Connector status drives StatusNotification correctness and the validity of transaction commands. The simulator should model an explicit state machine (`ConnectorStateMachine`) rather than free-form status strings.

### 7b.1 OCPP 1.6J Connector States

```txt
Available
Preparing
Charging
SuspendedEV
SuspendedEVSE
Finishing
Reserved
Unavailable
Faulted
```

### 7b.2 Allowed Transitions (MVP focus)

```txt
Available     -> Preparing        (plug in / start requested)
Preparing     -> Charging         (StartTransaction accepted)
Charging      -> SuspendedEV      (vehicle pauses draw)
Charging      -> SuspendedEVSE    (station pauses supply)
SuspendedEV   -> Charging
SuspendedEVSE -> Charging
Charging      -> Finishing        (StopTransaction)
Finishing     -> Available        (session cleared)
Available     -> Unavailable      (Set Unavailable)
Unavailable   -> Available
any           -> Faulted          (Trigger Fault)
Faulted       -> Available        (fault cleared)
```

### 7b.3 Rules

- Each accepted transition should emit a StatusNotification when the charge point is connected.
- Invalid transitions must be rejected with a clear API error and a `runtime.warning` event (see section 18).
- The state machine is OCPP-version-agnostic at the domain level; version packages map domain states to wire-level status enums.
- `Reserved` is modeled for completeness but not required for MVP transaction flows.

### 7b.4 MeterValue Generation Model

`MeterValueGenerator` produces the energy register reported in MeterValues and StopTransaction.

For MVP, use a simple deterministic model:

- Each charging connector has a configurable nominal power (default `11000 W`).
- The energy register (`Energy.Active.Import.Register`, Wh) advances linearly: `deltaWh = power_w * elapsed_seconds / 3600`.
- `start_meter_wh` is captured at StartTransaction; the current register is `start_meter_wh + accumulated`.
- The generator emits a sample on the configured MeterValues interval (default `60s`) and on manual trigger.
- Sampled measurands for MVP: `Energy.Active.Import.Register` (Wh) and optionally instantaneous `Power.Active.Import` (W). Other measurands (voltage, current, SoC) are out of MVP scope.

Future (non-MVP): ramp-up curves, SoC-based taper, configurable noise, multi-measurand profiles.

---

## 8. OCPP Version Architecture

The project should support OCPP 1.6J first, but the design must be multi-version ready.

Do not hardcode OCPP 1.6-specific payload models into the entire simulator domain.

Recommended structure:

```txt
internal/ocpp/
├─ core/
│  ├─ message.go
│  ├─ protocol.go
│  ├─ transport.go
│  ├─ codec.go
│  └─ errors.go
│
├─ v16/
│  ├─ actions.go
│  ├─ messages.go
│  ├─ codec.go
│  ├─ protocol.go
│  └─ handlers.go
│
└─ v201/
   ├─ actions.go
   ├─ messages.go
   ├─ codec.go
   ├─ protocol.go
   └─ handlers.go
```

### 8.1 Protocol Interface

Each OCPP version should implement a common interface.

```go
type Protocol interface {
    Version() string

    BuildBootNotification(input BootNotificationInput) (Message, error)
    BuildHeartbeat(input HeartbeatInput) (Message, error)
    BuildStatusNotification(input StatusNotificationInput) (Message, error)
    BuildAuthorize(input AuthorizeInput) (Message, error)
    BuildStartTransaction(input StartTransactionInput) (Message, error)
    BuildMeterValues(input MeterValuesInput) (Message, error)
    BuildStopTransaction(input StopTransactionInput) (Message, error)

    Encode(message Message) ([]byte, error)
    Decode(raw []byte) (Message, error)
}
```

### 8.2 Version Selection

Each charge point should store an OCPP version:

```json
{
  "id": "CP-001",
  "name": "Demo Charge Point 1",
  "ocppVersion": "1.6J",
  "centralSystemUrl": "ws://localhost:8080/ocpp",
  "connectorCount": 2,
  "autoConnect": false
}
```

At runtime:

1. ChargePointInstance loads its configured version.
2. Protocol factory returns the matching implementation.
3. Runtime calls protocol methods without caring about payload differences.

```go
protocol := protocolFactory.Create(chargePoint.OcppVersion)
message, err := protocol.BuildBootNotification(input)
```

### 8.3 Internal Domain Model

Use a generic simulator domain model.

Avoid exposing only OCPP 1.6 concepts everywhere.

Recommended internal shape:

```txt
ChargePoint
 └─ EVSE / ConnectorGroup
     └─ Connector
```

For MVP, the UI can simply display:

```txt
Charge Point > Connectors
```

But internally, leave room for OCPP 2.0.1 concepts such as EVSE, device model, and TransactionEvent.

---

## 9. OCPP 1.6J MVP Message Support

The first protocol implementation should support the following OCPP 1.6J messages:

### 9.1 Charge Point to Central System

- BootNotification
- Heartbeat
- StatusNotification
- Authorize
- StartTransaction
- MeterValues
- StopTransaction

### 9.2 Central System to Charge Point

For MVP, the simulator should decode and log responses for outgoing calls.

Later, it may support inbound Central System commands such as:

- RemoteStartTransaction
- RemoteStopTransaction
- ChangeAvailability
- Reset
- UnlockConnector
- TriggerMessage
- ChangeConfiguration

These should be planned but not required for the first MVP.

---

## 10. OCPP WebSocket Client

Each charge point should have its own WebSocket connection to the Central System.

If the simulator runs 10 charge points, it should open 10 independent OCPP WebSocket client connections.

Recommended behavior:

```txt
ChargePointInstance CP-001 -> WebSocket connection 1
ChargePointInstance CP-002 -> WebSocket connection 2
ChargePointInstance CP-003 -> WebSocket connection 3
```

### 10.1 Connection URL

Support configurable URL behavior.

Common OCPP 1.6J format:

```txt
ws://localhost:8080/ocpp/CP-001
```

The UI should allow the user to define:

- Central System base URL
- Charge Point ID
- Final connection URL preview

Example:

```txt
Base URL: ws://localhost:8080/ocpp
Charge Point ID: CP-001
Final URL: ws://localhost:8080/ocpp/CP-001
```

### 10.2 Connection Lifecycle

The OCPP client should support:

- Connect
- Disconnect
- Reconnect
- Connection status tracking
- Connection failure events
- Response timeout handling
- Graceful shutdown

### 10.3 Message Correlation

OCPP messages use unique IDs. The runtime must track pending calls.

Recommended pending call map:

```go
type PendingCall struct {
    UniqueID  string
    Action    string
    SentAt    time.Time
    TimeoutAt time.Time
}
```

On response:

1. Match response by unique ID.
2. Mark pending call as completed.
3. Persist inbound response log.
4. Publish realtime event.

On timeout:

1. Mark pending call as timed out.
2. Persist runtime warning.
3. Publish realtime event.

---

## 10b. Mock OCPP Central System (Test Server)

The simulator needs something to connect to. To avoid depending on a real EV charging backend during development, the project ships a small standalone Go server that acts as a mock OCPP Central System.

It is **not** an EVCMS. It has no database, no business logic, and no UI. Its only job is to accept OCPP WebSocket connections, parse incoming CALL messages, and reply with a protocol-valid CALLRESULT based on the message action.

### 10b.1 Goals

- Let a developer run the simulator end-to-end with zero external setup.
- Accept many charge point connections simultaneously.
- Respond to every MVP OCPP 1.6J action with a well-formed, accepting response.
- Stay tiny and dependency-light.

Explicit non-goals: persistence, authentication logic, tariffs, remote commands, realistic rejection scenarios. (Configurable rejection/fault responses are a future enhancement, see below.)

### 10b.2 Placement & Independence

- Lives in `apps/csms` as its own Go module / binary.
- Has no dependency on `apps/api`, the simulator runtime, or SQLite.
- Communicates only over the OCPP WebSocket wire protocol, exactly like a real Central System would.

### 10b.3 Connection

- Listens on a configurable address, default `ws://localhost:8080/ocpp`.
- Accepts connections at `ws://localhost:8080/ocpp/{chargePointId}`.
- Uses the `ocpp1.6` WebSocket subprotocol where offered.
- Logs each connect/disconnect and every message to stdout (human-readable), so it can double as a debugging surface.

### 10b.4 Message Handling

The server parses the OCPP JSON array frame `[MessageTypeId, UniqueId, Action, Payload]` and dispatches by action:

```txt
[2, UniqueId, Action, Payload]   CALL      -> handle, reply [3, UniqueId, ResultPayload]
[3, UniqueId, Payload]           CALLRESULT-> (inbound result, just log)
[4, UniqueId, Code, Desc, Det]   CALLERROR -> log
```

Default responses for MVP 1.6J actions (all "happy path"):

```txt
BootNotification    -> { "status": "Accepted", "currentTime": <now>, "interval": 300 }
Heartbeat           -> { "currentTime": <now> }
StatusNotification  -> {}
Authorize           -> { "idTagInfo": { "status": "Accepted" } }
StartTransaction    -> { "transactionId": <server-assigned int>, "idTagInfo": { "status": "Accepted" } }
MeterValues         -> {}
StopTransaction     -> { "idTagInfo": { "status": "Accepted" } }
```

The server assigns `transactionId` from a simple in-memory incrementing counter. This is what closes the loop with the simulator's asynchronous transaction-ID flow (section 7.6).

Unknown / unsupported actions reply with a CALLERROR (`NotImplemented` / `NotSupported`) rather than crashing.

### 10b.5 Minimal Structure

```txt
apps/csms/
├─ cmd/
│  └─ server/
│     └─ main.go
├─ internal/
│  ├─ wsserver/      # WebSocket accept loop, per-connection read/write
│  └─ handlers/      # action -> response mapping
├─ go.mod
└─ go.sum
```

### 10b.6 Future Enhancements (non-MVP)

- Configurable responses (force `Rejected`, `Blocked`, delayed, or CALLERROR per action) to test the simulator's error handling.
- Sending inbound Central System commands (RemoteStartTransaction, Reset, TriggerMessage) to exercise the simulator's future inbound handling.
- A `--scenario` flag to script response sequences.

---

## 11. Realtime WebSocket for Vue UI

The Go backend must also expose a WebSocket server for the Vue client.

This is separate from the OCPP WebSocket client.

There are two WebSocket layers:

```txt
1. Go simulator -> OCPP Central System
   Used for OCPP protocol traffic.

2. Vue client -> Go API
   Used for live logs and runtime state updates.
```

### 11.1 UI WebSocket Endpoint

Recommended endpoint:

```txt
GET /api/realtime
```

The Vue client connects to:

```txt
ws://localhost:7070/api/realtime
```

### 11.2 Subscription Model

The Vue client should subscribe to the selected charge point.

Example subscribe message:

```json
{
  "type": "subscribe",
  "chargePointId": "CP-001"
}
```

Example unsubscribe message:

```json
{
  "type": "unsubscribe",
  "chargePointId": "CP-001"
}
```

When selected unit changes in the UI:

1. Send unsubscribe for the previous charge point.
2. Send subscribe for the new charge point.
3. Clear or preserve local log view depending on UX decision.
4. Start receiving live events for the selected unit.

### 11.3 Realtime Event Model

Recommended event shape:

```go
type RealtimeEvent struct {
    ID            string          `json:"id"`
    Type          string          `json:"type"`
    ChargePointID string          `json:"chargePointId"`
    ConnectorID   *int            `json:"connectorId,omitempty"`
    Direction     *string         `json:"direction,omitempty"`
    Action        *string         `json:"action,omitempty"`
    Payload       json.RawMessage `json:"payload,omitempty"`
    Message       string          `json:"message,omitempty"`
    Timestamp     time.Time       `json:"timestamp"`
}
```

Recommended event types:

```txt
charge_point.created
charge_point.deleted
charge_point.connected
charge_point.disconnected
charge_point.connection_failed
charge_point.reconnecting

connector.created
connector.deleted
connector.status_changed

transaction.started
transaction.stopped
transaction.failed

meter_value.generated
meter_value.sent

ocpp.message.sent
ocpp.message.received
ocpp.call_error.received
ocpp.response_timeout

runtime.warning
runtime.error
```

### 11.4 Event Bus

The simulator runtime should publish all important state changes to an internal event bus.

Flow:

```txt
Simulator Runtime -> EventBus -> Realtime Hub -> Vue Clients
```

The realtime hub should filter events by chargePointId and client subscription.

---

## 12. API Design

### 12.1 Charge Point APIs

```txt
GET    /api/charge-points
POST   /api/charge-points
GET    /api/charge-points/{id}
PUT    /api/charge-points/{id}
DELETE /api/charge-points/{id}
```

### 12.2 Runtime APIs

```txt
POST /api/charge-points/{id}/connect
POST /api/charge-points/{id}/disconnect
POST /api/charge-points/{id}/reconnect
POST /api/charge-points/{id}/boot
POST /api/charge-points/{id}/heartbeat
```

### 12.3 Connector APIs

```txt
POST   /api/charge-points/{id}/connectors
DELETE /api/charge-points/{id}/connectors/{connectorId}
POST   /api/charge-points/{id}/connectors/{connectorId}/status
```

### 12.4 Transaction APIs

```txt
POST /api/charge-points/{id}/connectors/{connectorId}/start-transaction
POST /api/charge-points/{id}/connectors/{connectorId}/stop-transaction
POST /api/charge-points/{id}/connectors/{connectorId}/meter-values
```

### 12.5 Logs and Sessions

```txt
GET /api/messages
GET /api/messages?chargePointId=CP-001
GET /api/messages?chargePointId=CP-001&connectorId=1
GET /api/sessions
GET /api/sessions?chargePointId=CP-001
GET /api/sessions/active
```

### 12.6 Settings

```txt
GET /api/settings
PUT /api/settings
```

Settings may include:

- Default Central System URL
- Default OCPP version
- Default heartbeat interval
- Default MeterValues interval
- Default connector count
- Auto-connect on startup
- Log retention limit

---

## 13. SQLite Persistence

SQLite should store persistent configuration and historical records.

Runtime-only objects should not be treated as the source of truth after restart.

### 13.1 Suggested Tables

#### charge_points

```txt
id                  TEXT PRIMARY KEY
name                TEXT NOT NULL
ocpp_version        TEXT NOT NULL
central_system_url  TEXT NOT NULL
auto_connect        INTEGER NOT NULL DEFAULT 0
is_enabled          INTEGER NOT NULL DEFAULT 1
is_deleted          INTEGER NOT NULL DEFAULT 0
created_at          TEXT NOT NULL
updated_at          TEXT NOT NULL
deleted_at          TEXT NULL
```

#### connectors

```txt
id                  INTEGER PRIMARY KEY AUTOINCREMENT
charge_point_id     TEXT NOT NULL
evse_id             INTEGER NOT NULL DEFAULT 1
connector_number    INTEGER NOT NULL
status              TEXT NOT NULL
is_enabled          INTEGER NOT NULL DEFAULT 1
is_deleted          INTEGER NOT NULL DEFAULT 0
created_at          TEXT NOT NULL
updated_at          TEXT NOT NULL
deleted_at          TEXT NULL

UNIQUE(charge_point_id, connector_number)
```

> `evse_id` is always `1` in OCPP 1.6J (flat connector model). It is reserved now so OCPP 2.0.1's EVSE → Connector hierarchy can be represented without a breaking migration later. The MVP UI ignores it.

#### charging_sessions

```txt
id                  TEXT PRIMARY KEY
charge_point_id     TEXT NOT NULL
connector_number    INTEGER NOT NULL
transaction_id      INTEGER NULL
id_tag              TEXT NOT NULL
status              TEXT NOT NULL
started_at          TEXT NOT NULL
stopped_at          TEXT NULL
start_meter_wh      INTEGER NOT NULL
stop_meter_wh       INTEGER NULL
stop_reason         TEXT NULL
created_at          TEXT NOT NULL
updated_at          TEXT NOT NULL
```

> `status` reflects the async lifecycle from section 7.6: `pending_start` → `active` → `stopping` → `stopped`, plus terminal `failed`. `transaction_id` stays NULL while `pending_start` and is filled once `StartTransaction.conf` is accepted.

#### ocpp_message_logs

```txt
id                  TEXT PRIMARY KEY
charge_point_id     TEXT NOT NULL
connector_number    INTEGER NULL
session_id          TEXT NULL
transaction_id      INTEGER NULL
unique_id           TEXT NULL
message_type        TEXT NOT NULL
message_type_id     INTEGER NULL
direction           TEXT NOT NULL
ocpp_version        TEXT NOT NULL
action              TEXT NULL
payload_json        TEXT NOT NULL
status              TEXT NOT NULL
error_code          TEXT NULL
error_description   TEXT NULL
created_at          TEXT NOT NULL
```

> `session_id` / `transaction_id` link a protocol message to its session so the UI can answer "which transaction does this message belong to". Both are NULL for non-transactional messages (BootNotification, Heartbeat). For the initial `StartTransaction.req`, `session_id` is set but `transaction_id` is NULL until the `.conf` correlates it (section 7.6).

#### runtime_events

```txt
id                  TEXT PRIMARY KEY
charge_point_id     TEXT NULL
connector_number    INTEGER NULL
event_type          TEXT NOT NULL
severity            TEXT NOT NULL
message             TEXT NOT NULL
payload_json        TEXT NULL
created_at          TEXT NOT NULL
```

#### app_settings

```txt
key                 TEXT PRIMARY KEY
value               TEXT NOT NULL
updated_at          TEXT NOT NULL
```

---

## 14. Frontend Architecture

### 14.1 Frontend Folder Structure

Recommended Vue structure:

```txt
apps/web/
├─ src/
│  ├─ api/
│  │  ├─ chargePointsApi.ts
│  │  ├─ connectorsApi.ts
│  │  ├─ sessionsApi.ts
│  │  └─ settingsApi.ts
│  │
│  ├─ realtime/
│  │  ├─ realtimeClient.ts
│  │  └─ realtimeTypes.ts
│  │
│  ├─ stores/
│  │  ├─ chargePointStore.ts
│  │  ├─ realtimeStore.ts
│  │  ├─ sessionStore.ts
│  │  └─ settingsStore.ts
│  │
│  ├─ components/
│  │  ├─ ChargePointList.vue
│  │  ├─ ChargePointCard.vue
│  │  ├─ ChargePointDetail.vue
│  │  ├─ ConnectorCard.vue
│  │  ├─ LiveLogPanel.vue
│  │  ├─ JsonPayloadDrawer.vue
│  │  ├─ AddChargePointModal.vue
│  │  ├─ AddConnectorModal.vue
│  │  ├─ StartTransactionModal.vue
│  │  └─ RuntimeStatusBadge.vue
│  │
│  ├─ pages/
│  │  ├─ DashboardPage.vue
│  │  ├─ SessionsPage.vue
│  │  ├─ MessagesPage.vue
│  │  └─ SettingsPage.vue
│  │
│  ├─ router/
│  └─ main.ts
│
├─ package.json
└─ vite.config.ts
```

### 14.2 Main UI Concept

The UI should feel like a simulator control room.

Recommended main dashboard layout:

```txt
┌──────────────────────────────────────────────────────────────┐
│ Top Bar: OCPP Simulator | Runtime Status | Global Settings   │
├────────────────┬──────────────────────────────┬──────────────┤
│ Charge Points  │ Selected Charge Point Detail │ Live Logs    │
│                │                              │              │
│ CP-001 🟢       │ CP-001                       │ Boot sent    │
│ CP-002 🔴       │ Connected                    │ Accepted     │
│ CP-003 🟡       │ OCPP 1.6J                    │ Status sent  │
│                │                              │              │
│ + Add Unit     │ Connectors                   │              │
│                │ [1 Available] [2 Charging]   │              │
└────────────────┴──────────────────────────────┴──────────────┘
```

### 14.3 Left Panel: Charge Point List

The left panel should show all units.

Each charge point card should display:

- Charge Point ID
- Connection status
- OCPP version
- Connector count
- Active transaction count
- Quick actions

Example:

```txt
CP-001
Connected · OCPP 1.6J · 2 connectors
[Disconnect] [Duplicate]
```

### 14.4 Center Panel: Selected Charge Point Control

The selected charge point detail should show:

- Charge Point ID
- Name
- OCPP version
- Central System URL
- Connection status
- Last connected time
- Runtime actions

Runtime actions:

- Connect
- Disconnect
- Reconnect
- Send BootNotification
- Send Heartbeat
- Add Connector
- Delete Unit

### 14.5 Connector Cards

Each connector should appear as an independent control card.

Connector card fields:

- Connector number
- Current status
- Active session if any
- Current meter value
- Last event timestamp

Actions:

- Set Status
- Start Transaction
- Stop Transaction
- Send MeterValues
- Trigger Fault
- Set Unavailable

Example:

```txt
Connector 1
Status: Available
No active session
[Set Status] [Start Transaction] [Fault]

Connector 2
Status: Charging
Session: TX-1021
Meter: 12.42 kWh
[Stop Transaction] [Send MeterValues]
```

### 14.6 Right Panel: Live Logs

The right panel should show live logs for the selected charge point.

Features:

- Auto-scroll by default
- Pause auto-scroll when user scrolls up
- Filter by event type
- Filter by connector
- Filter by OCPP/runtime/errors
- Click log row to open JSON payload drawer

Log row example:

```txt
09:42:10  outbound  BootNotification
09:42:10  inbound   BootNotification.conf Accepted
09:42:18  outbound  StatusNotification Connector 1 Available
09:42:24  runtime   Connector 2 started transaction
```

### 14.7 JSON Payload Drawer

When the user clicks a log row, open a side drawer showing:

- Direction
- Action
- Message type
- Unique ID
- Timestamp
- Raw payload JSON
- Response status
- Error details if any

---

## 15. Dynamic UX Requirements

The UI should feel alive and responsive.

Required dynamic behavior:

- Selected charge point changes WebSocket subscription.
- Connector state updates arrive without refresh.
- Transaction start/stop updates connector cards immediately.
- Meter value increases live while charging.
- OCPP messages appear in the timeline immediately.
- Runtime errors appear in the same timeline.
- Charge point connection status updates immediately.
- Add/remove connector updates UI immediately.
- Add/remove unit updates UI immediately.
- Avoid forcing manual refresh.

Recommended rule:

> The dashboard should be usable as the main simulator surface without opening browser devtools or server logs.

---

## 16. State Management

Use Pinia stores.

Recommended stores:

```txt
chargePointStore
- chargePoints
- selectedChargePointId
- selectedChargePointDetail
- loadChargePoints()
- selectChargePoint(id)
- createChargePoint(input)
- deleteChargePoint(id)
- connect(id)
- disconnect(id)

realtimeStore
- socket
- connectionStatus
- eventsByChargePoint
- subscribe(chargePointId)
- unsubscribe(chargePointId)
- appendEvent(event)

sessionStore
- activeSessions
- sessionHistory
- loadActiveSessions()

settingsStore
- settings
- loadSettings()
- updateSettings(input)
```

The frontend should treat REST API as the command/query layer and WebSocket as the live event layer.

---

## 17. Development Phases

### Phase 0: Repository Bootstrap

Deliverables:

- Turborepo setup
- pnpm workspace
- apps/api Go service
- apps/web Vue app
- apps/csms Go mock Central System skeleton (accepts a WebSocket connection, logs frames)
- root dev scripts
- basic README
- basic Docker ignore files

Acceptance criteria:

- `pnpm install` works
- `pnpm dev` starts web and api
- API exposes health endpoint
- Web app loads dashboard shell
- `pnpm dev:csms` starts the mock Central System and accepts a raw WebSocket connection

### Phase 1: SQLite and Basic CRUD

Deliverables:

- SQLite setup
- migrations
- charge_points table
- connectors table
- app_settings table
- charge point CRUD
- connector CRUD
- basic Vue UI for adding/removing units and connectors

Acceptance criteria:

- User can create a charge point from UI
- User can delete a charge point from UI
- User can add connectors from UI
- User can delete connectors from UI
- Data persists after restart

### Phase 2: Simulator Runtime Registry

Deliverables:

- Runtime registry
- ChargePointInstance model
- Connector runtime state
- Runtime event bus
- Connect/disconnect commands without real OCPP yet

Acceptance criteria:

- API can start/stop a charge point instance
- UI shows runtime status
- Runtime events are produced internally

### Phase 3: UI Realtime WebSocket

Deliverables:

- `/api/realtime` WebSocket endpoint
- client subscriptions by chargePointId
- event filtering by chargePointId
- Vue realtime client
- LiveLogPanel

Acceptance criteria:

- Vue client can subscribe to selected charge point
- Runtime events appear live in the log panel
- Changing selected unit changes subscription

### Phase 4: OCPP 1.6J Protocol Core

Deliverables:

- OCPP core message model
- Protocol interface
- OCPP 1.6J implementation
- Encode/decode support for CALL, CALLRESULT, CALLERROR
- Unique ID generation
- Pending call tracking

Acceptance criteria:

- BootNotification can be encoded as valid OCPP 1.6J JSON array
- Heartbeat can be encoded
- StatusNotification can be encoded
- Responses can be matched by unique ID
- Sent and received messages are logged

### Phase 5: OCPP WebSocket Client

Deliverables:

- Per-charge-point WebSocket client
- Connect/disconnect/reconnect
- Message send
- Response receive loop
- Timeout handling
- Message persistence
- Realtime event publishing

Note: the mock Central System (`apps/csms`, section 10b) should now return real happy-path responses, so the simulator can be tested end-to-end without any external backend.

Acceptance criteria:

- Charge point connects to the mock Central System URL
- BootNotification is sent after command and the mock replies `Accepted`
- Response is received and shown in UI
- Disconnect closes connection cleanly

### Phase 6: Transaction Simulation

Deliverables:

- Authorize
- StartTransaction
- StopTransaction
- MeterValues
- Active session model
- Meter value generator
- Connector status transitions

Acceptance criteria:

- User can start transaction from connector card
- Connector moves to Charging
- Active session is shown
- MeterValues can be sent manually
- MeterValues can be generated automatically
- User can stop transaction
- Session is persisted

### Phase 7: UX Polish

Deliverables:

- Better dashboard layout
- Log filters
- JSON payload drawer
- Runtime status badges
- Settings page
- Basic error display

Acceptance criteria:

- User can inspect messages clearly
- User can filter logs
- User can view raw payloads
- Common runtime errors are understandable

### Phase 8: Multi-Version Readiness

Deliverables:

- Protocol factory
- Version stored per charge point
- UI version selector
- OCPP 2.0.1 placeholder package
- Clear unsupported version handling

Acceptance criteria:

- OCPP 1.6J works through protocol abstraction
- OCPP 2.0.1 appears as planned/disabled or unsupported
- Adding a new version does not require changing simulator runtime flow heavily

---

## 18. Error Handling Principles

Errors should be visible in the UI.

Do not hide important runtime failures only in server logs.

Examples:

- WebSocket connection failed
- Central System returned CALLERROR
- Response timeout
- Invalid connector status transition
- Transaction already active
- Connector has no active transaction
- Charge point is disconnected
- Unsupported OCPP version

Each important error should:

1. Return a clear API error.
2. Publish a realtime runtime.error event.
3. Persist a runtime event when useful.

---

## 19. Logging Principles

The application has two kinds of logs:

### 19.1 OCPP Message Logs

These are protocol-level logs.

Examples:

- BootNotification sent
- BootNotification response received
- StatusNotification sent
- MeterValues sent
- StopTransaction response received
- CALLERROR received

### 19.2 Runtime Events

These are simulator-level logs.

Examples:

- Charge point connected
- Connector status changed
- Transaction started
- Meter loop started
- Backend response timeout
- WebSocket disconnected

The UI can show both in a single timeline, but the backend should keep them conceptually separate.

---

## 20. Naming Guidelines

Use clear names.

Avoid vague names such as:

- Manager
- Helper
- Util
- Processor

Prefer specific names:

- ChargePointInstance
- ChargePointRegistry
- OcppWebSocketClient
- ConnectorStateMachine
- MeterValueGenerator
- RealtimeHub
- RuntimeEventBus
- ProtocolFactory
- PendingCallRegistry

---

## 21. Suggested Root Scripts

Root `package.json` example:

```json
{
  "scripts": {
    "dev": "turbo dev",
    "dev:api": "cd apps/api && go run ./cmd/server",
    "dev:web": "pnpm --filter web dev",
    "dev:csms": "cd apps/csms && go run ./cmd/server",
    "build": "turbo build",
    "lint": "turbo lint",
    "test": "turbo test"
  }
}
```

`turborepo` task example:

```json
{
  "tasks": {
    "dev": {
      "cache": false,
      "persistent": true
    },
    "build": {
      "dependsOn": ["^build"],
      "outputs": ["dist/**", "bin/**"]
    },
    "lint": {},
    "test": {}
  }
}
```

---

## 22. Future Features

After MVP:

- Scenario runner
- Bulk charge point creation
- Import/export simulator configuration
- RemoteStartTransaction support
- RemoteStopTransaction support
- ChangeAvailability support
- Reset support
- TriggerMessage support
- OCPP 2.0.1 support
- Fault simulation presets
- Random disconnect/reconnect simulation
- Load testing mode
- Headless CLI mode
- Single binary distribution with embedded Vue build
- Docker image
- Mock Central System enhancements: configurable rejection/fault responses, scripted scenarios, and inbound command sending (see section 10b.6)

---

## 23. LLM Development Instructions

When using an LLM agent to implement this project, follow these rules:

1. Do not implement the full EVCMS domain.
2. Keep the project focused on simulation.
3. Keep runtime state and persistence clearly separated.
4. Do not hardcode OCPP 1.6 concepts across the whole app.
5. Implement OCPP 1.6J first through the protocol abstraction.
6. Keep UI WebSocket separate from OCPP WebSocket.
7. Every charge point must have its own OCPP WebSocket client connection.
8. Vue should subscribe to live events for the selected charge point.
9. Connector add/remove must work dynamically.
10. Charge point add/remove must work dynamically.
11. Active transactions must prevent unsafe connector deletion.
12. Important runtime errors must appear in the UI timeline.
13. Keep the UI simple, fast, and operational.
14. Avoid turning the dashboard into a generic CRUD panel.
15. Favor small, testable backend packages.

---

## 24. MVP Definition of Done

The MVP is complete when:

- The app runs locally.
- User opens the Vue dashboard in browser.
- User creates a charge point.
- User adds multiple connectors.
- User connects the charge point to an OCPP Central System.
- User sends BootNotification.
- User sees outbound and inbound OCPP messages live.
- User starts a transaction on a connector.
- User sees connector status change to Charging.
- User sends MeterValues.
- User stops the transaction.
- User sees all relevant logs in the live log panel.
- User can disconnect the charge point.
- Data persists in SQLite.
- The architecture still allows adding OCPP 2.0.1 later.

---

## 25. Short Project Description

A local-first, web-based OCPP simulator built with Go, Vue, SQLite, and Turborepo. It supports multiple simulated charge points, multiple connectors, live WebSocket logs, dynamic runtime control, and OCPP 1.6J message flows, while being designed for future multi-version OCPP support.
