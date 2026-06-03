# OCPP Simulator Communication Architecture Work Plan

## Purpose

This plan exists to prevent the project from drifting into an incorrect architecture where all simulator traffic is pushed through a single shared WebSocket stream or where OCPP communication is replaced by REST command endpoints.

The simulator must model real OCPP behavior. Each simulated charge point communicates with the central system through its own OCPP WebSocket connection. The web UI is allowed to act as the control surface for local charge point behavior, but every unit still needs its own OCPP session.

## Core Rule

There must be a strict separation between:

- OCPP communication plane
- API control plane
- UI observation plane
- internal event/log streaming plane

These layers can observe or command each other, but they must not be merged.

Current accepted model:

- CP-initiated actions such as plug, unplug, Authorize, StartTransaction, StopTransaction, MeterValues, and StatusNotification are driven from the simulator UI and sent over `GET /api/ws/{chargePointId}`. The API proxies that unit's OCPP frames to `centralSystemUrl/{chargePointId}`. This is the simulated charge point's OCPP session.
- CSMS-initiated actions such as RemoteStartTransaction enter through the mock CSMS REST API and are sent by the mock CSMS over the target charge point's existing OCPP WebSocket.
- `/api/realtime` is only for logs, state, and runtime events.
- Unit ↔ CSMS traffic is raw OCPP-J JSON array frames only. Do not wrap OCPP messages in objects before sending over the OCPP WebSocket.

Target service split:

- `simulator-api`: UI management API. It owns simulator configuration such as charge point and connector create/delete/list. It is independent from OCPP message processing and must not generate OCPP frames.
- `ocpp-gateway`: WebSocket edge. It accepts charge point connections, publishes inbound raw OCPP frames to MQTT, subscribes to outbound MQTT topics, and writes outbound raw frames to the correct WebSocket.
- `message-processor`: MQTT consumer/router. It reads inbound raw frames, routes by OCPP message type/action, and publishes raw response frames.
- `ocpp-core`: DB and business owner. First phase is message logging only; later it owns transaction state, connector state, authorization, remote commands, and session history.

Target MQTT topics:

```txt
ocpp/{chargePointId}/in
ocpp/{chargePointId}/out
```

MQTT payloads are the same raw OCPP-J JSON arrays that travel over WebSocket. Do not use `{ chargePointId, direction, payload }` wrapper objects.

## Non-Negotiable Architecture Rules

### 1. Each simulated charge point owns its own OCPP WebSocket connection

Every simulated charge point must establish and manage its own WebSocket connection to the configured Central System endpoint.

Correct:

```text
ChargePoint-001 -> ws://central-system/ocpp/CP-001
ChargePoint-002 -> ws://central-system/ocpp/CP-002
ChargePoint-003 -> ws://central-system/ocpp/CP-003
```

Incorrect:

```text
Simulator Backend -> one shared WebSocket -> all charge point messages
```

The simulator may run all charge points inside the same backend/browser/API process combination, but logically and architecturally each charge point must behave as a separate OCPP client.

### 1b. OCPP wire messages are raw JSON arrays, never wrappers

Correct OCPP WebSocket traffic:

```json
[2, "uid-1", "BootNotification", { "chargePointVendor": "Simulator", "chargePointModel": "OCPP-Sim" }]
```

```json
[3, "uid-1", { "status": "Accepted", "currentTime": "2026-06-03T12:00:00Z", "interval": 30 }]
```

Incorrect:

```json
{ "type": "CALL", "uniqueId": "uid-1", "action": "BootNotification", "payload": { "chargePointVendor": "Simulator" } }
```

The OCPP socket must not carry DTO envelopes, command wrappers, event wrappers, UI models, or internal simulator models. If a wrapper is useful for UI state, REST responses, logs, or storage, unwrap it before crossing the OCPP transport boundary.

The same rule applies to MQTT. Topics carry routing metadata; payloads remain raw OCPP frames.

### 2. REST API must not transport OCPP messages

REST endpoints are allowed for management actions and for test hooks that represent an external actor, especially the mock CSMS API.

Allowed API responsibilities:

- create charge point
- delete charge point
- update charge point configuration
- start simulated charge point
- stop simulated charge point
- create or close a simulated charge point session
- query current state
- query stored logs
- query transaction history
- query connector status
- ask the mock CSMS to send a CSMS-initiated OCPP CALL to a connected charge point

Forbidden API responsibilities:

- replacing a charge point's OCPP WebSocket with REST for BootNotification
- replacing a charge point's OCPP WebSocket with REST for Heartbeat
- replacing a charge point's OCPP WebSocket with REST for StartTransaction
- replacing a charge point's OCPP WebSocket with REST for MeterValues
- replacing a charge point's OCPP WebSocket with REST for StopTransaction
- routing Central System commands through generic API message passing

The API may proxy a unit-specific OCPP WebSocket or expose mock-CSMS command endpoints. The resulting OCPP message must still travel over the target charge point's OCPP WebSocket connection.

Example:

```text
UI opens /api/ws/CP-001
        ↓
API proxies that session to ws://central-system/ocpp/CP-001
        ↓
UI plug-in action causes CP-001 to send StatusNotification over that OCPP socket
```

Not:

```text
POST /ocpp/send
        ↓
API sends StatusNotification directly
```

### 3. UI realtime WebSocket is only for observation

The frontend may use `/api/realtime` to observe logs, state changes, and live events.

This realtime socket is not an OCPP socket. It is separate from `/api/ws/{chargePointId}`, which is the unit-specific OCPP session proxy.

Correct UI stream usage:

```text
Backend -> UI WebSocket -> logs, state updates, transaction updates, connector updates
```

Incorrect realtime stream usage:

```text
/api/realtime -> carries OCPP messages for all charge points
```

The UI stream must be treated as a read-model/event-feed layer.

### 4. Central System commands must enter through charge point OCPP sockets

If the Central System sends commands such as:

- RemoteStartTransaction
- RemoteStopTransaction
- Reset
- UnlockConnector
- ChangeAvailability
- ChangeConfiguration
- GetConfiguration
- ClearCache
- ReserveNow
- CancelReservation

They must arrive through the specific charge point WebSocket connection.

The simulator must not fake these as local UI actions. The accepted test helper is the mock CSMS REST API: a caller asks the mock CSMS to send `RemoteStartTransaction`, then the mock CSMS sends the OCPP CALL over the charge point's WebSocket.

### 5. One internal event bus is allowed, but it is not the transport boundary

The backend may use an internal event bus for coordination.

Allowed:

```text
ChargePointConnection emits OcppMessageReceived
ConnectorStateMachine emits ConnectorStatusChanged
TransactionEngine emits TransactionStarted
LogService emits LogCreated
UIHub broadcasts LogCreated
```

Forbidden:

```text
InternalEventBus replaces OCPP WebSocket communication
```

The event bus is for internal decoupling only. It must not erase the distinction between OCPP sockets, API commands, and UI streams.

## Required Simulator Modules

### simulator-api

The simulator API is the UI-facing management service. It is independent from OCPP message processing.

Responsibilities:

- Create, list, update, and delete simulated charge points.
- Create, list, update, and delete connector definitions.
- Store simulator configuration and dashboard read models.
- Provide UI management endpoints.
- Avoid OCPP frame generation and OCPP business decisions.

### ocpp-gateway

The gateway is the WebSocket edge service. It has one job: keep charge point WebSocket connections alive and bridge raw OCPP frames to/from MQTT.

Responsibilities:

- Accept WebSocket connections from charge points.
- Track connection by `chargePointId`.
- Publish inbound WebSocket messages unchanged to `ocpp/{chargePointId}/in`.
- Subscribe to `ocpp/{chargePointId}/out`.
- Write outbound MQTT messages unchanged to the matching charge point WebSocket.
- Handle connect, disconnect, ping/pong, close, and reconnect visibility.
- Avoid business logic, persistence decisions, and wrapper payloads.

### message-processor

The processor consumes inbound OCPP frames from MQTT and produces outbound frames.

Responsibilities:

- Subscribe to `ocpp/+/in`.
- Parse raw OCPP-J array frames.
- Route by message type and action.
- Generate spec-exact CALLRESULT/CALLERROR frames for supported flows.
- Publish responses to `ocpp/{chargePointId}/out`.
- In early phases, produce happy-path responses directly.
- Later, delegate business decisions to `ocpp-core`.

### ocpp-core

The core service owns data and business.

Responsibilities:

- Own its own database.
- First phase: subscribe to `ocpp/+/in` and `ocpp/+/out`, then persist raw message logs.
- Later phases: own transaction state, connector state, authorization decisions, session history, and remote command APIs.
- Never expose internal business models on the OCPP WebSocket or MQTT payload.

### Docker Compose

Local development should be runnable with Docker Compose.

Expected services:

- `mqtt`
- `simulator-api`
- `ocpp-gateway`
- `message-processor`
- `ocpp-core`
- `web`

The compose setup is orchestration only. It must not introduce wrapper payloads or change the raw OCPP-J frame contract.

### Charge Point Session

Responsible for one simulated charge point session.

Responsibilities:

- owns charge point identity
- owns connector collection
- owns current availability state
- owns transaction state
- owns authorization cache if enabled
- owns OCPP connection lifecycle
- sends OCPP calls through its own socket
- receives OCPP calls from Central System through its own socket
- applies OCPP responses to internal state

Suggested name:

```text
ChargePointSession
```

### OCPP Connection

Responsible only for WebSocket protocol handling for one charge point.

Responsibilities:

- connect
- disconnect
- reconnect
- send CALL
- send CALLRESULT
- send CALLERROR
- correlate request/response by unique id
- handle timeout
- validate message envelope
- expose incoming messages to the charge point runtime

Suggested name:

```text
OcppConnection
```

There must be one connection instance per running simulated charge point. In the current implementation this can be the `/api/ws/{chargePointId}` proxy session used by the UI-controlled simulator.

### OCPP Message Router

Responsible for dispatching incoming OCPP actions to handlers.

Responsibilities:

- route BootNotification response
- route RemoteStartTransaction
- route RemoteStopTransaction
- route Reset
- route ChangeConfiguration
- route GetConfiguration
- route UnlockConnector
- route TriggerMessage
- route other supported actions

Suggested name:

```text
OcppMessageRouter
```

This router must be scoped to a charge point session or receive the charge point context explicitly.

### Connector State Machine

Responsible for connector-level behavior.

Responsibilities:

- Available
- Preparing
- Charging
- SuspendedEV
- SuspendedEVSE
- Finishing
- Reserved
- Unavailable
- Faulted

The connector state machine emits state changes. It does not send OCPP messages directly unless explicitly designed as part of the charge point runtime boundary.

Preferred flow:

```text
ConnectorStateMachine changes state
        ↓
The simulated charge point session observes change
        ↓
That charge point sends StatusNotification over its own OCPP socket
```

### Transaction Engine

Responsible for transaction lifecycle.

Responsibilities:

- prepare transaction
- start transaction
- track meter values
- stop transaction
- handle idTag
- handle transactionId from Central System
- handle local transaction records
- handle abnormal stop reasons

Preferred flow:

```text
User/API triggers plug-in or remote start
        ↓
ChargePointRuntime validates connector state
        ↓
TransactionEngine starts local transaction process
        ↓
ChargePointRuntime sends StartTransaction over OCPP socket
        ↓
Central System returns transactionId
        ↓
TransactionEngine stores transactionId
```

### API Layer

Responsible for management and manual simulation commands.

Allowed API examples:

```text
POST /charge-points
POST /charge-points/{id}/start
POST /charge-points/{id}/stop
POST /charge-points/{id}/connectors
POST /charge-points/{id}/connectors/{connectorId}/plug-in
POST /charge-points/{id}/connectors/{connectorId}/unplug
POST /charge-points/{id}/connectors/{connectorId}/fault
POST /charge-points/{id}/connectors/{connectorId}/recover
POST /charge-points/{id}/connectors/{connectorId}/start-local-transaction
POST /charge-points/{id}/connectors/{connectorId}/stop-transaction
GET /charge-points
GET /charge-points/{id}
GET /charge-points/{id}/logs
GET /charge-points/{id}/transactions
```

Forbidden API examples:

```text
POST /ocpp/BootNotification
POST /ocpp/Heartbeat
POST /ocpp/StartTransaction
POST /ocpp/StopTransaction
POST /ocpp/MeterValues
POST /ocpp/send
POST /messages/send
```

If a generic testing endpoint is temporarily needed, it must be marked as dev-only and must not be used by production simulator flows.

### UI Live Stream

Responsible for frontend observation.

Possible stream events:

```text
ChargePointStarted
ChargePointStopped
OcppConnected
OcppDisconnected
OcppMessageSent
OcppMessageReceived
ConnectorStatusChanged
TransactionStarted
TransactionStopped
MeterValueGenerated
ErrorOccurred
```

The UI stream must be explicitly named so it is not confused with OCPP.

Good names:

```text
LiveEventHub
SimulatorEventStream
LogStream
RuntimeEventStream
```

Bad names:

```text
OcppWebSocket
MessageSocket
MainSocket
GlobalSocket
```

## Required Runtime Flow Examples

### Boot Flow

```text
API: POST /charge-points/CP-001/start
        ↓
ChargePointRuntime starts
        ↓
OcppConnection opens WebSocket to Central System
        ↓
ChargePointRuntime sends BootNotification through OcppConnection
        ↓
Central System returns BootNotification.conf
        ↓
ChargePointRuntime stores registration status and heartbeat interval
        ↓
UI receives log/state events through SimulatorEventStream
```

### Heartbeat Flow

```text
ChargePointRuntime has accepted BootNotification
        ↓
HeartbeatScheduler starts using interval from Central System
        ↓
ChargePointRuntime sends Heartbeat over its own OCPP WebSocket
        ↓
UI receives log event only
```

### Local Start Transaction Flow

```text
API: POST /charge-points/CP-001/connectors/1/start-local-transaction
        ↓
ChargePointRuntime checks connector state
        ↓
Authorize is sent over CP-001 OCPP WebSocket if required
        ↓
StartTransaction is sent over CP-001 OCPP WebSocket
        ↓
Central System returns transactionId
        ↓
TransactionEngine stores active transaction
        ↓
MeterValues scheduler starts
        ↓
UI receives state/log updates
```

### Remote Start Transaction Flow

```text
Central System sends RemoteStartTransaction to CP-001 WebSocket
        ↓
OcppConnection receives CALL
        ↓
OcppMessageRouter dispatches to RemoteStartTransaction handler
        ↓
ChargePointRuntime validates connector availability
        ↓
ChargePointRuntime replies with RemoteStartTransaction.conf
        ↓
TransactionEngine starts transaction process
        ↓
StartTransaction is sent over CP-001 OCPP WebSocket
        ↓
UI receives state/log updates
```

### Meter Values Flow

```text
TransactionEngine has active transaction
        ↓
MeterValueScheduler generates readings
        ↓
ChargePointRuntime sends MeterValues over CP-001 OCPP WebSocket
        ↓
UI receives generated meter value as observation event
```

### Stop Transaction Flow

```text
API or Central System command triggers stop
        ↓
ChargePointRuntime validates active transaction
        ↓
TransactionEngine closes local transaction
        ↓
ChargePointRuntime sends StopTransaction over CP-001 OCPP WebSocket
        ↓
Connector state changes to Finishing or Available
        ↓
UI receives updates
```

## Implementation Phases

### Phase 1 — Lock the communication boundaries

Goal:

Prevent architecture drift before feature implementation continues.

Tasks:

- Create `docs/COMMUNICATION_ARCHITECTURE.md`
- Add this rule set to the repository
- Define OCPP connection as a per-charge-point runtime dependency
- Define API as control plane only
- Define UI socket as observation plane only
- Remove or reject any design that introduces a global OCPP WebSocket
- Remove or reject any design that sends OCPP messages directly through REST

Acceptance criteria:

- No endpoint named `/ocpp/send`
- No single shared socket responsible for all charge point OCPP traffic
- Each running charge point has its own OCPP connection instance
- UI socket does not send OCPP protocol messages
- API actions trigger simulator behavior, not protocol transport

### Phase 2 — Implement charge point runtime lifecycle

Goal:

Create the runtime object that owns one simulated charge point.

Tasks:

- Implement `ChargePointRuntime`
- Implement start/stop lifecycle
- Implement connector collection
- Implement transaction state holder
- Implement runtime event emission
- Implement runtime registry

Acceptance criteria:

- Multiple charge points can run in the same backend process
- Each charge point can be started and stopped independently
- Each charge point has isolated connector and transaction state
- Runtime state can be queried through API
- Runtime events can be streamed to UI

### Phase 3 — Implement per-charge-point OCPP WebSocket client

Goal:

Make each simulated charge point behave like a real OCPP client.

Tasks:

- Implement `OcppConnection`
- Add WebSocket connect/disconnect/reconnect logic
- Add OCPP CALL/CALLRESULT/CALLERROR envelope support
- Add unique id correlation
- Add request timeout handling
- Add incoming message dispatch hook

Acceptance criteria:

- CP-001 and CP-002 can connect to the same Central System independently
- Messages from CP-001 never use CP-002 connection
- Responses are correlated to the correct pending request
- Connection loss affects only the related charge point
- UI can observe connection state changes

### Phase 4 — Implement OCPP 1.6J core flows

Goal:

Support the essential simulator flows.

Tasks:

- BootNotification
- Heartbeat
- StatusNotification
- Authorize
- StartTransaction
- MeterValues
- StopTransaction
- RemoteStartTransaction
- RemoteStopTransaction
- Reset
- UnlockConnector
- ChangeAvailability
- ChangeConfiguration
- GetConfiguration

Acceptance criteria:

- BootNotification is sent only over the charge point OCPP socket
- Heartbeat interval follows Central System response
- StatusNotification is emitted after connector state changes
- StartTransaction and StopTransaction use the correct connector and transaction state
- Remote commands are received from the OCPP socket, not API
- API can trigger user-like actions that result in OCPP messages

### Phase 5 — Implement UI live observation

Goal:

Give the frontend real-time visibility without turning the UI stream into OCPP transport.

Tasks:

- Implement `SimulatorEventStream`
- Stream logs
- Stream charge point state changes
- Stream connector state changes
- Stream transaction updates
- Stream sent/received OCPP message summaries
- Add filtering by charge point id
- Add filtering by connector id
- Add filtering by message type

Acceptance criteria:

- UI can observe all active charge points
- UI can filter logs per charge point
- UI stream does not carry command transport responsibility
- Closing UI does not affect OCPP connections
- Multiple browser clients can observe the same simulator state

### Phase 6 — Add guardrails for future LLM-generated code

Goal:

Prevent future agents from reintroducing the wrong architecture.

Tasks:

- Add an architecture decision record
- Add code comments at key boundaries
- Add tests for per-charge-point connection isolation
- Add tests preventing REST-based OCPP transport
- Add naming conventions
- Add forbidden patterns section to contributor docs

Acceptance criteria:

- A test fails if OCPP messages are routed through REST transport
- A test fails if multiple charge points share one OCPP connection instance
- Documentation explicitly says API is not OCPP transport
- Documentation explicitly says UI WebSocket is not OCPP transport
- New agents can understand the architecture without guessing

## Forbidden Patterns

Do not implement:

```text
SingleGlobalOcppSocket
GlobalMessageSocket
OcppMessageApiController
GenericOcppSendEndpoint
AllMessagesHub
OneSocketForEverything
FrontendAsOcppProxy
ApiAsCentralSystemProxy
```

Do not design flows like:

```text
UI -> WebSocket -> Backend -> Central System OCPP message
```

unless the UI action is explicitly a management command and the actual OCPP message is sent by the related `ChargePointRuntime`.

Correct:

```text
UI -> API command -> ChargePointRuntime -> CP-specific OCPP socket
```

Incorrect:

```text
UI -> global WebSocket -> OCPP message
```

## Naming Rules

Use names that make architectural boundaries obvious.

Preferred:

```text
OcppConnection
ChargePointRuntime
ChargePointRuntimeRegistry
SimulatorCommandController
SimulatorEventStream
RuntimeEventPublisher
ConnectorStateMachine
TransactionEngine
OcppMessageRouter
```

Avoid:

```text
MessageService
SocketService
WebSocketManager
CommunicationService
OcppService
MainHub
GlobalHub
```

Generic names cause LLM agents to merge responsibilities.

## Agent Instruction

When implementing this project, do not simplify the communication model into one WebSocket or REST-based OCPP messaging.

This simulator must behave like many independent charge points. Each charge point has its own OCPP WebSocket connection to the Central System. The REST API controls simulator state and user-like actions. The frontend live stream observes logs and runtime events only.

Never send OCPP protocol messages directly through the REST API. Never route all charge point traffic through one shared WebSocket. Never use the UI live stream as the OCPP transport.

If a user action should cause an OCPP message, the API must call the relevant `ChargePointRuntime`, and that runtime must send the OCPP message through its own `OcppConnection`.

If a Central System command is received, it must arrive through the specific charge point's OCPP WebSocket connection and be routed to that charge point runtime.

Architecture correctness is more important than reducing the number of sockets or creating a generic communication abstraction.
