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
- `message-processor`: response producer. It reads inbound raw frames from `ocpp/+/in`, responds only to supported CP-to-server OCPP 1.6J CALLs, calls `ocpp-core` for business decisions, publishes raw CALLRESULT/CALLERROR frames, and writes consume/publish audit events to stdout/log only.
- `ocpp-core`: canonical log and business owner. It persists raw `ocpp/+/in` and `ocpp/+/out` topic messages as the source of truth, owns transactions/runtime events, answers message-processor callbacks, initializes numeric transaction IDs from DB `MAX(numeric_id)`, and publishes CSMS-initiated CALLs to `ocpp/{chargePointId}/out`.
- `web`: Vue UI plus per-CP local OCPP simulation. It opens one WebSocket per simulated CP to `ocpp-gateway`, owns local connector state, meter generation, transaction holder state, and accepts browser-refresh state loss as a local-simulator tradeoff.

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

The processor consumes inbound OCPP frames from MQTT and produces outbound responses only for CP-to-server supported OCPP 1.6J CALLs.

Responsibilities:

- Subscribe to `ocpp/+/in`.
- Parse raw OCPP-J array frames.
- For CP-to-server CALLs, generate spec-exact CALLRESULT/CALLERROR frames for supported flows.
- Call `ocpp-core` over internal HTTP for business decisions such as Authorize, StartTransaction, and StopTransaction.
- Publish responses to `ocpp/{chargePointId}/out`.
- Write consume/publish audit events to stdout/log only; these events are not OCPP wire payloads and may be wrapped.
- Do not write a DB.
- Do not produce CSMS-initiated CALLs.
- Do not respond to CP-to-server CALLRESULT/CALLERROR frames.

### ocpp-core

The core service owns canonical logs, data, and business.

Responsibilities:

- Own its own database.
- Subscribe to raw `ocpp/+/in` and `ocpp/+/out`, then persist canonical OCPP message logs.
- Own `transactions`, `ocpp_message_logs`, and `runtime_events`; there is no `processor_events` table.
- Answer message-processor business decision callbacks.
- On startup, initialize the OCPP 1.6 numeric transaction counter with `SELECT COALESCE(MAX(numeric_id), 0) FROM transactions`.
- Create transaction UUIDs and numeric transaction IDs for StartTransaction.
- Publish CSMS-initiated CALLs such as RemoteStart, Reset, ChangeConfiguration, GetConfiguration, TriggerMessage, ChangeAvailability, and UnlockConnector to `ocpp/{chargePointId}/out`.
- Publish UI-ready events to `realtime/{chargePointId}.event`.
- Never expose internal business models on the OCPP WebSocket or MQTT payload.
- Never open OCPP WebSocket connections or consume message-processor stdout audit logs.

### web

The Vue app is both the UI and the local per-charge-point simulator.

Responsibilities:

- Open one WebSocket per simulated charge point to `ocpp-gateway`.
- Keep local connector state, meter generation, transaction holder state, and pending-call correlation.
- Produce CP-to-server OCPP CALLs as raw OCPP-J arrays.
- Respond to server-to-CP CALLs with raw OCPP-J CALLRESULT/CALLERROR arrays.
- Accept browser-refresh state loss as a current local-simulator tradeoff.

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

## Required Target Flows

### StartTransaction

```text
Vue CP simulator sends raw [2, uid, "StartTransaction", payload]
        ↓
ocpp-gateway publishes unchanged to ocpp/{chargePointId}/in
        ↓
message-processor parses and validates StartTransaction.req
        ↓
message-processor POSTs /internal/transactions/start to ocpp-core
        ↓
ocpp-core creates transaction UUID + restart-safe numeric_id
        ↓
message-processor publishes raw [3, uid, { transactionId, idTagInfo }] to ocpp/{chargePointId}/out
        ↓
ocpp-gateway writes unchanged to the matching CP WebSocket
```

### CSMS-Initiated RemoteStart

```text
External test helper or scheduler calls ocpp-core /internal/remote-start
        ↓
ocpp-core publishes raw [2, uid, "RemoteStartTransaction", payload] to ocpp/{chargePointId}/out
        ↓
ocpp-gateway writes unchanged to the matching CP WebSocket
        ↓
Vue CP simulator returns raw [3, uid, { status }] over its CP WebSocket
        ↓
ocpp-gateway publishes unchanged to ocpp/{chargePointId}/in
        ↓
message-processor parses/audits but does not produce another response
        ↓
ocpp-core logs the raw response and updates remote command state if needed
```

### Canonical Log vs Processor Audit

```text
Canonical OCPP log:
ocpp-core subscribes to raw ocpp/+/in and ocpp/+/out -> ocpp_message_logs

Processor audit:
message-processor stdout/log per consume/publish -> debug only, no DB table
```

## Migration Phases

1. Extract `packages/ocpp-protocol` with public `pkg/` packages for codec, message, protocol interfaces, pending calls, v16, and v201 placeholders.
2. Rename the current combined `apps/api` to `apps/simulator-api` and remove OCPP runtime responsibilities from it.
3. Create `apps/ocpp-gateway` as a dumb `/ws/{chargePointId}` WebSocket edge and raw MQTT bridge.
4. Create `apps/message-processor` as the CP-to-server response producer with stdout audit and no DB.
5. Create `apps/ocpp-core` with `transactions`, `ocpp_message_logs`, and `runtime_events`; no `processor_events`.
6. Move canonical OCPP logging to `ocpp-core` raw topic subscriptions.
7. Move StartTransaction transaction UUID/numeric ID generation to `ocpp-core`.
8. Keep per-CP simulation state in Vue and point each CP WebSocket at `ocpp-gateway`.
9. Add Docker Compose for `mqtt`, `simulator-api`, `ocpp-gateway`, `message-processor`, `ocpp-core`, `web`, and optional `csms`.
10. Update docs/tasks and run each Go module's tests from its own directory.

## Forbidden Patterns

Do not implement:

```text
SingleGlobalOcppSocket
GlobalMessageSocket
OcppMessageApiController
GenericOcppSendEndpoint
AllMessagesHub
OneSocketForEverything
ApiAsOcppTransport
GatewayWithBusinessState
GatewayTransactionEngine
ProcessorEventsDatabase
WrappedOcppMqttPayload
```

Do not design target flows like:

```text
simulator-api -> OCPP frame
ocpp-gateway -> transaction/meter/state decision
message-processor -> DB write
ocpp-core -> OCPP WebSocket
```

Correct target flow:

```text
Vue CP simulator -> ocpp-gateway -> MQTT ocpp/{chargePointId}/in
message-processor -> optional ocpp-core callback -> MQTT ocpp/{chargePointId}/out
ocpp-gateway -> Vue CP simulator
ocpp-core -> raw topic subscriptions -> canonical DB logs/business
```

## Agent Instruction

When implementing this project, do not simplify the target model into REST-based OCPP messaging or a gateway that owns business state.

`simulator-api` is not OCPP transport. `ocpp-gateway` is not business logic. `message-processor` is not persistence. `ocpp-core` is not a WebSocket edge. Vue owns local CP simulation state for the target local simulator, including the accepted browser-refresh state-loss tradeoff.
