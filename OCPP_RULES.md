# OCPP Rules Reference for Simulator Development

> Practical implementation reference for building an OCPP simulator.
> This document is not a replacement for the official Open Charge Alliance specification. Use it as a developer-facing rulebook for architecture, behavior, validation, and edge cases.

## Scope

This document covers the rules a simulator should understand for:

- OCPP 1.6J, JSON over WebSocket
- OCPP 2.0.1
- OCPP 2.1, at a high level
- multi-charge-point simulation
- multi-connector charge points
- WebSocket connection behavior
- central-system / CSMS interaction
- transaction lifecycle
- authorization
- status reporting
- metering
- remote commands
- configuration
- firmware and diagnostics
- smart charging
- security profiles
- simulator-specific behavior

The initial implementation should target OCPP 1.6J first. OCPP 2.0.1 and 2.1 should be added behind version-specific adapters because their object model is different enough that sharing too much domain code will create fragile abstractions.

---

# 1. OCPP Concepts

## 1.1 Main Actors

### Charge Point / Charging Station

In OCPP 1.6, the field name is usually `ChargePoint`.

In OCPP 2.x, the terminology shifts toward `ChargingStation`, `EVSE`, and `Connector`.

For simulator purposes:

- a Charge Point is the simulated physical charger unit
- it owns one or more connectors
- it opens a WebSocket connection to the Central System / CSMS
- it sends telemetry, status, meter data, boot messages, and transaction events
- it receives commands such as reset, unlock connector, remote start, remote stop, configuration changes, and charging profile updates

### Central System / CSMS

In OCPP 1.6, this is usually called the Central System.

In OCPP 2.x, this is usually called CSMS: Charging Station Management System.

For simulator purposes:

- the backend endpoint accepts WebSocket connections from charge points
- it validates OCPP messages
- it responds with CALLRESULT or CALLERROR
- it may send server-initiated commands to the charger
- it stores charger state, connector state, transactions, meter values, diagnostics and logs

## 1.2 Version Difference Summary

### OCPP 1.6J

OCPP 1.6J is the most common practical starting point. It uses JSON messages over WebSocket and a relatively simple charge-point / connector model.

Important traits:

- charger connects to central system
- one charger may have multiple connectors
- connector `0` means the whole charge point, not a physical plug
- transactions are started with `StartTransaction`
- transactions are stopped with `StopTransaction`
- status changes are sent with `StatusNotification`
- metering is sent with `MeterValues`
- authorization is usually done with `Authorize`
- configuration is managed with `GetConfiguration` and `ChangeConfiguration`

### OCPP 2.0.1

OCPP 2.0.1 changes the model significantly.

Important traits:

- introduces stronger security concepts
- uses `TransactionEvent` instead of `StartTransaction` / `StopTransaction`
- uses a richer device model
- separates station, EVSE, and connector more explicitly
- adds improved smart charging and ISO 15118 related concepts
- uses more structured reason codes and payloads

### OCPP 2.1

OCPP 2.1 builds on 2.0.1 and adds newer energy and grid-related capabilities. It should not be the first simulator target unless the goal is specifically modern V2G / DER scenarios.

For this simulator, OCPP 2.1 should be treated as a later compatibility layer after 2.0.1 support is stable.

---

# 2. Transport Rules

## 2.1 WebSocket Connection

OCPP JSON uses WebSocket transport.

General rules:

- the charge point initiates the connection
- the central system listens for charge point connections
- each charge point should have a stable identity
- the identity is usually encoded in the WebSocket URL path
- one WebSocket connection usually represents one charge point
- reconnects should preserve charge point identity
- the simulator must support connection loss and reconnection

Example connection shape:

```text
ws://localhost:7080/ws/{chargePointId}
```

or:

```text
wss://csms.example.com/ocpp/{chargePointId}
```

## 2.2 Subprotocol Rule

The WebSocket connection should negotiate the correct OCPP subprotocol.

Common subprotocols:

```text
ocpp1.6
ocpp2.0.1
ocpp2.1
```

Simulator rule:

- reject unsupported subprotocols
- allow the selected version to determine message schema, routing, and behavior
- keep protocol handling version-specific

## 2.3 Message Framing

OCPP JSON messages are arrays.

There are three base message types:

```text
2 = CALL
3 = CALLRESULT
4 = CALLERROR
```

### CALL

A request message.

Shape:

```json
[2, "unique-id", "ActionName", { "payload": "fields" }]
```

Simulator rule: this array shape is the wire message. Never wrap it in an object envelope such as `{ "type": "CALL", "action": "...", "payload": ... }` when sending between a unit and the CSMS.

Example:

```json
[2, "19223201", "Heartbeat", {}]
```

### CALLRESULT

A successful response.

Shape:

```json
[3, "unique-id", { "payload": "fields" }]
```

Example:

```json
[3, "19223201", { "currentTime": "2026-06-03T09:00:00Z" }]
```

### CALLERROR

An error response.

Shape:

```json
[4, "unique-id", "ErrorCode", "ErrorDescription", { "details": "optional" }]
```

Example:

```json
[4, "19223201", "FormationViolation", "Payload is not valid JSON", {}]
```

## 2.4 Unique ID Rules

Every CALL must include a unique message id.

Rules:

- unique id should be unique at least for the lifetime of the WebSocket connection
- CALLRESULT and CALLERROR must use the same unique id as the original CALL
- pending requests should be tracked by unique id
- duplicate unique ids while an earlier request is still pending should be treated as invalid or suspicious
- after reconnect, pending request state should normally be cleared unless the simulator explicitly models retry behavior

## 2.5 Request / Response Correlation

The simulator must correlate responses to requests.

Rules:

- when sending a CALL, store `uniqueId`, `action`, `sentAt`, `timeoutAt`
- when receiving CALLRESULT, match by `uniqueId`
- when receiving CALLERROR, match by `uniqueId`
- if no pending request exists, log it as unexpected
- if timeout expires, mark request as failed

## 2.6 Error Handling Rules

Common OCPP error codes include:

- `NotImplemented`
- `NotSupported`
- `InternalError`
- `ProtocolError`
- `SecurityError`
- `FormationViolation`
- `PropertyConstraintViolation`
- `OccurenceConstraintViolation`
- `TypeConstraintViolation`
- `GenericError`

Simulator behavior:

- malformed JSON => `FormationViolation`
- valid JSON but wrong array shape => `ProtocolError` or `FormationViolation`
- unknown action => `NotImplemented` or `NotSupported`
- wrong field type => `TypeConstraintViolation`
- missing required field => `OccurenceConstraintViolation`
- invalid enum value => `PropertyConstraintViolation`
- unauthorized command => `SecurityError`
- unexpected runtime failure => `InternalError`

---

# 3. OCPP 1.6J Core Model

## 3.1 Charge Point

A simulated OCPP 1.6 charge point should have:

- `chargePointId`
- vendor
- model
- serial number
- firmware version
- connection status
- boot status
- heartbeat interval
- list of connectors
- local authorization list
- configuration key/value store
- active transactions
- diagnostics state
- firmware update state

## 3.2 Connector

A connector should have:

- `connectorId`
- current status
- current error code
- optional transaction id
- meter value
- availability state
- reserved state
- last status notification time

Connector id rules:

- `0` means whole charge point
- `1..n` means physical connectors
- commands targeting connector `0` usually affect the entire charge point
- commands targeting connector `1..n` affect that specific connector

## 3.3 Connector Status Values

Typical OCPP 1.6 connector statuses:

- `Available`
- `Preparing`
- `Charging`
- `SuspendedEVSE`
- `SuspendedEV`
- `Finishing`
- `Reserved`
- `Unavailable`
- `Faulted`

Simulator state rules:

- disconnected cable usually means `Available`
- plugged but not yet charging usually means `Preparing`
- energy transfer active means `Charging`
- charger-side pause means `SuspendedEVSE`
- vehicle-side pause means `SuspendedEV`
- transaction stopping / cable still attached means `Finishing`
- reservation active means `Reserved`
- disabled connector means `Unavailable`
- hardware or simulated fault means `Faulted`

---

# 4. OCPP 1.6J Boot Flow

## 4.1 BootNotification

A charge point should send `BootNotification` after connection.

Basic payload fields usually include:

- `chargePointVendor`
- `chargePointModel`
- optional serial numbers
- optional firmware version
- optional meter information

Central system response includes:

- `status`
- `currentTime`
- `interval`

Possible statuses:

- `Accepted`
- `Pending`
- `Rejected`

## 4.2 Boot Accepted Rules

If boot is accepted:

- mark charge point as booted
- store central system time
- set heartbeat interval from response
- start heartbeat loop
- send initial status notifications for connectors if configured

## 4.3 Boot Pending Rules

If boot is pending:

- charge point is known but not fully accepted yet
- simulator should retry `BootNotification` after the interval
- avoid sending normal operational messages until accepted unless deliberately testing edge cases

## 4.4 Boot Rejected Rules

If boot is rejected:

- simulator should not start normal operations
- may retry later based on configured policy
- should display rejected state in UI

---

# 5. OCPP 1.6J Heartbeat Rules

## 5.1 Heartbeat

After accepted boot, charger sends `Heartbeat` periodically.

Rules:

- interval comes from `BootNotification` response
- central system returns current time
- simulator should support manual heartbeat trigger
- heartbeat should stop when WebSocket is disconnected
- heartbeat should restart after reconnect and accepted boot

## 5.2 Time Sync

The central system response includes `currentTime`.

Simulator rules:

- store the latest central system time
- optionally calculate clock drift
- do not mutate host system time
- show simulated station time separately if needed

---

# 6. OCPP 1.6J Authorization Rules

## 6.1 Authorize

Before starting a transaction, the charge point may send `Authorize` with an `idTag`.

Response contains `idTagInfo`.

Common authorization statuses:

- `Accepted`
- `Blocked`
- `Expired`
- `Invalid`
- `ConcurrentTx`

Simulator rules:

- if authorization is accepted, allow transaction start
- if blocked, expired, invalid, or concurrent, deny transaction start
- allow configurable behavior for testing bad flows

## 6.2 Local Authorization

A charge point may support local authorization list behavior.

Simulator rules:

- local list can allow starting a transaction without online authorization
- local authorization must be configurable
- simulator should show whether a transaction was locally or centrally authorized
- local list version should be tracked if implementing `SendLocalList` / `GetLocalListVersion`

---

# 7. OCPP 1.6J Transaction Lifecycle

## 7.1 Normal Start Flow

Typical local start flow:

```text
User plugs cable
Connector => Preparing
Authorize(idTag)
StartTransaction(connectorId, idTag, meterStart, timestamp)
Central System returns transactionId + idTagInfo
Connector => Charging
MeterValues periodically
```

Rules:

- connector must exist
- connector should not already have an active transaction
- connector should not be unavailable or faulted
- authorization should be accepted unless test mode overrides it
- transaction id comes from central system response
- local simulator should store both local transaction record and central transaction id

## 7.2 StartTransaction

Important payload fields:

- `connectorId`
- `idTag`
- `meterStart`
- `timestamp`
- optional `reservationId`

Response contains:

- `transactionId`
- `idTagInfo`

Simulator rules:

- if response authorization is not accepted, do not enter Charging state
- if response is accepted, bind transaction to connector
- meterStart should be current simulated meter value
- timestamp must be ISO 8601

## 7.3 MeterValues During Transaction

During charging, send `MeterValues` periodically.

Rules:

- include `connectorId`
- include `transactionId` when related to active transaction
- use monotonically increasing energy values for normal charging
- support configurable sampled measurands
- support invalid / edge-case meter values only in test mode

Common measurands:

- `Energy.Active.Import.Register`
- `Power.Active.Import`
- `Current.Import`
- `Voltage`
- `SoC`
- `Temperature`

## 7.4 Normal Stop Flow

Typical stop flow:

```text
Stop requested by user / EV / CSMS
Connector => Finishing
StopTransaction(transactionId, meterStop, timestamp, reason)
Central System accepts response
Connector => Available or Preparing depending on cable state
```

## 7.5 StopTransaction

Important payload fields:

- `transactionId`
- `meterStop`
- `timestamp`
- optional `idTag`
- optional `reason`
- optional transaction data

Common stop reasons:

- `Local`
- `Remote`
- `EVDisconnected`
- `HardReset`
- `SoftReset`
- `UnlockCommand`
- `DeAuthorized`
- `EmergencyStop`
- `PowerLoss`
- `Reboot`
- `Other`

Simulator rules:

- `meterStop` should be greater than or equal to `meterStart`
- transaction must be active before it can be stopped
- after stop, clear connector transaction id
- final connector status depends on cable state and availability

## 7.6 Abnormal Transaction Cases

Simulator should support these test cases:

- StartTransaction without prior Authorize
- StartTransaction rejected by central system
- StopTransaction for unknown transaction
- duplicate StartTransaction for occupied connector
- MeterValues without transaction id
- MeterValues with unknown transaction id
- connector disconnect during transaction
- charger reboot during transaction
- WebSocket disconnect during transaction
- remote stop while local stop is already in progress

---

# 8. OCPP 1.6J StatusNotification Rules

## 8.1 StatusNotification

Status notification reports connector state.

Important fields:

- `connectorId`
- `status`
- `errorCode`
- optional `info`
- optional `timestamp`
- optional vendor fields

Rules:

- send when connector status changes
- optionally send initial status after boot
- include `NoError` when status is not faulted
- use `Faulted` with specific error code when simulating faults
- do not spam duplicate status notifications unless configured

## 8.2 Fault Rules

When connector enters `Faulted`:

- active transaction should usually stop or suspend depending on configured scenario
- send `StatusNotification` with `Faulted`
- include relevant error code
- block new transactions until fault is cleared

Common error codes:

- `NoError`
- `ConnectorLockFailure`
- `EVCommunicationError`
- `GroundFailure`
- `HighTemperature`
- `InternalError`
- `LocalListConflict`
- `OverCurrentFailure`
- `PowerMeterFailure`
- `PowerSwitchFailure`
- `ReaderFailure`
- `ResetFailure`
- `UnderVoltage`
- `OverVoltage`
- `WeakSignal`
- `OtherError`

---

# 9. OCPP 1.6J Remote Commands

## 9.1 RemoteStartTransaction

Central system can request the charger to start a transaction.

Rules:

- connector must be available or preparing depending on configuration
- idTag must be provided
- if connector id is omitted, charger may choose a connector
- if connector is busy, unavailable, reserved by another idTag, or faulted, reject
- if accepted, start normal transaction flow

Possible response statuses:

- `Accepted`
- `Rejected`

## 9.2 RemoteStopTransaction

Central system can request a transaction stop.

Rules:

- transaction id must exist
- if exists, accept and stop transaction
- if unknown, reject
- stop reason should be `Remote`

## 9.3 Reset

Central system can request reset.

Types:

- `Soft`
- `Hard`

Simulator rules:

- soft reset should reconnect or reinitialize without wiping configuration
- hard reset should simulate deeper reboot
- active transactions should be stopped or recovered based on scenario
- send boot notification after reset/reconnect if configured

## 9.4 UnlockConnector

Central system can request connector unlock.

Rules:

- connector must exist
- if locked and unlock succeeds, respond `Unlocked`
- if unlock fails, respond `UnlockFailed`
- if connector is not supported, respond `NotSupported`
- if unlock stops active transaction, StopTransaction reason should be `UnlockCommand`

## 9.5 ChangeAvailability

Central system can change charger or connector availability.

Rules:

- connector `0` applies to whole charge point
- specific connector id applies to one connector
- `Inoperative` should make target unavailable
- `Operative` should make target available when no fault blocks it
- if transaction is active, availability may become scheduled rather than immediate

Possible statuses:

- `Accepted`
- `Rejected`
- `Scheduled`

## 9.6 ReserveNow

Central system can reserve a connector for an idTag.

Rules:

- connector must exist or charger must support choosing one
- connector must not be occupied, faulted, or unavailable
- reservation expiry must be tracked
- matching idTag should be allowed to start transaction
- non-matching idTag should be rejected while reservation is active

Possible statuses:

- `Accepted`
- `Faulted`
- `Occupied`
- `Rejected`
- `Unavailable`

## 9.7 CancelReservation

Rules:

- reservation id must exist
- cancel clears reservation
- connector status should return to `Available` if no other state blocks it

Possible statuses:

- `Accepted`
- `Rejected`

---

# 10. OCPP 1.6J Configuration Rules

## 10.1 GetConfiguration

Central system can read charger configuration.

Simulator rules:

- maintain a key/value configuration store
- return known keys with readonly flag
- return unknown requested keys separately
- support query for all keys when no key list is provided

## 10.2 ChangeConfiguration

Central system can update a configuration key.

Possible statuses:

- `Accepted`
- `Rejected`
- `RebootRequired`
- `NotSupported`

Simulator rules:

- readonly keys must reject updates
- unknown keys return `NotSupported`
- invalid values return `Rejected`
- some keys may require reboot
- updated values should affect runtime behavior where applicable

Useful simulator configuration keys:

- heartbeat interval
- meter value sample interval
- supported measurands
- authorization behavior
- local auth enabled
- number of connectors
- default meter increment
- transaction message retry behavior
- websocket reconnect behavior

---

# 11. OCPP 1.6J Firmware and Diagnostics

## 11.1 UpdateFirmware

Central system can request firmware update.

Simulator rules:

- accept or reject based on configuration
- store firmware update target location
- simulate download phase
- send `FirmwareStatusNotification`
- optionally reconnect after successful install

Common firmware statuses:

- `Downloaded`
- `DownloadFailed`
- `Downloading`
- `Idle`
- `InstallationFailed`
- `Installing`
- `Installed`

## 11.2 GetDiagnostics

Central system can request diagnostics upload.

Simulator rules:

- accept request if diagnostics supported
- generate fake diagnostics file or log bundle
- simulate upload progress
- send `DiagnosticsStatusNotification`

Common diagnostics statuses:

- `Idle`
- `Uploaded`
- `UploadFailed`
- `Uploading`

---

# 12. OCPP 1.6J Smart Charging Rules

## 12.1 Charging Profiles

Smart charging controls limits for charging power/current.

Concepts:

- charging profile
- stack level
- charging profile purpose
- charging profile kind
- charging schedule
- charging rate unit

Common purposes:

- `ChargePointMaxProfile`
- `TxDefaultProfile`
- `TxProfile`

Common kinds:

- `Absolute`
- `Recurring`
- `Relative`

Simulator rules:

- store charging profiles by connector / transaction / purpose
- validate connector id
- validate stack level conflicts
- apply most specific active profile first
- `TxProfile` applies to a transaction
- `TxDefaultProfile` applies by default to new transactions
- `ChargePointMaxProfile` applies globally
- meter simulation should respect active charging limit if enabled

## 12.2 SetChargingProfile

Possible statuses:

- `Accepted`
- `Rejected`
- `NotSupported`

Rules:

- reject unsupported profile kinds
- reject invalid connector id
- reject malformed schedules
- accept and store valid profiles

## 12.3 ClearChargingProfile

Rules:

- remove matching profiles
- if no matching profile exists, response may still be accepted depending on implementation policy
- update active charging limit after clearing

---

# 13. OCPP 1.6J Local List Rules

## 13.1 SendLocalList

Central system can send local authorization list.

Update types:

- `Full`
- `Differential`

Rules:

- full update replaces list
- differential update patches list
- version number should be stored
- invalid version or malformed list should fail

Possible statuses:

- `Accepted`
- `Failed`
- `NotSupported`
- `VersionMismatch`

## 13.2 GetLocalListVersion

Rules:

- return current local list version
- return `0` or configured default when no list exists

---

# 14. OCPP 2.0.1 Core Differences

## 14.1 Entity Model

OCPP 2.x uses a richer hierarchy:

```text
ChargingStation
  EVSE
    Connector
```

Simulator rule:

- do not map OCPP 2.x directly onto the OCPP 1.6 connector model without an adapter
- create a version-neutral internal model only for concepts that truly overlap
- keep protocol payloads version-specific

## 14.2 TransactionEvent

OCPP 2.0.1 replaces `StartTransaction` and `StopTransaction` with `TransactionEvent`.

Transaction event types:

- `Started`
- `Updated`
- `Ended`

Simulator rules:

- start transaction with `TransactionEvent(Started)`
- send periodic or event-driven updates with `TransactionEvent(Updated)`
- stop transaction with `TransactionEvent(Ended)`
- transaction id is part of transaction info
- event sequence number matters

## 14.3 BootNotification in 2.x

Boot remains important but payload structure differs.

Simulator rules:

- send 2.x payload shape for 2.x sessions
- do not reuse 1.6 payload objects
- apply interval from CSMS response
- handle accepted, pending, rejected states

## 14.4 StatusNotification in 2.x

OCPP 2.x status model is different and more structured around EVSE and connector.

Simulator rules:

- status must map to EVSE/connector structure
- avoid blindly translating 1.6 statuses
- retain enough internal state to produce correct 2.x status payloads

## 14.5 Authorization in 2.x

OCPP 2.x uses richer id token structures.

Simulator rules:

- model id tokens as structured objects
- support authorization cache if needed
- support certificate / ISO 15118 related flows later, not in the first version

## 14.6 Device Model

OCPP 2.x includes a device model for components and variables.

Simulator rules:

- represent station capabilities as components and variables
- allow UI to inspect and modify simulated variables
- implement simple device model first
- avoid hardcoding every variable in business logic

## 14.7 Security Events

OCPP 2.x includes stronger security event reporting.

Simulator rules:

- support fake security event generation
- log failed authentication attempts
- log certificate issues if certificate features are implemented
- expose security logs in UI

---

# 15. Security Rules

## 15.1 Development Security Baseline

For local simulator development:

- allow `ws://` for local mode
- allow simple token/basic auth only as optional dev feature
- do not require TLS in local mode

For hosted simulator mode:

- use `wss://`
- require authentication
- isolate tenants/users if multi-user
- do not expose unrestricted command endpoints
- avoid storing secrets in logs

## 15.2 OCPP 1.6 Security Profiles

OCPP 1.6 originally had weaker security assumptions, but security profile guidance exists separately.

Simulator rule:

- keep security behavior configurable
- support unsecured local mode
- support TLS/basic-auth-like mode for integration testing
- do not mix security concerns into core protocol parsing

## 15.3 Message Validation

Always validate:

- message type
- unique id
- action name
- payload shape
- required fields
- field types
- enum values
- connector/EVSE existence
- transaction existence
- state transition validity

---

# 16. Simulator Architecture Rules

## 16.1 Version Adapter Pattern

Recommended architecture:

```text
WebSocket Session
  -> Protocol Version Detector
    -> OCPP 1.6 Handler
    -> OCPP 2.0.1 Handler
    -> OCPP 2.1 Handler
```

Each version handler owns:

- schemas
- action routing
- payload validation
- message serialization
- message deserialization
- version-specific state translation

Shared domain may include:

- simulated charger
- connector/EVSE state
- transaction state
- meter simulation
- logs
- command scheduler
- fault injection

## 16.2 Do Not Over-Abstract Too Early

Avoid creating one giant universal OCPP model.

Safer rule:

- use shared domain for simulation state
- use protocol adapters for external OCPP messages
- keep OCPP 1.6 and 2.x request/response DTOs separate

## 16.3 Event Log

Every simulator action should produce a log event.

Log fields:

- timestamp
- charge point id
- connector id or EVSE id
- direction: inbound / outbound / internal
- protocol version
- action
- unique id
- raw payload
- parsed payload
- result
- error if any

## 16.4 UI State Rules

UI should display:

- connected / disconnected charge points
- protocol version
- boot state
- heartbeat timer
- connector statuses
- active transactions
- latest meter values
- reservations
- availability
- firmware status
- diagnostics status
- inbound/outbound OCPP logs
- active charging profiles

---

# 17. Database Rules

For local simulator, SQLite is enough.

Recommended tables:

- charge_points
- connectors
- evses, for OCPP 2.x
- transactions
- meter_values
- ocpp_messages
- configuration_keys
- reservations
- local_auth_list
- charging_profiles
- firmware_jobs
- diagnostics_jobs
- simulator_scenarios

Rules:

- store raw OCPP messages for debugging
- store normalized state for UI
- do not rely only on logs for current state
- allow resetting simulator data easily
- support seed scenarios

---

# 18. Scenario Rules

The simulator should support scripted scenarios.

Useful scenarios:

- normal boot and heartbeat
- normal local transaction
- normal remote transaction
- authorization rejected
- connector fault before transaction
- connector fault during transaction
- WebSocket disconnect during transaction
- central system timeout
- duplicate message id
- invalid payload
- remote stop unknown transaction
- reservation then matching idTag start
- reservation then wrong idTag rejected
- change availability during active transaction
- firmware update success
- firmware update failure
- diagnostics upload success
- diagnostics upload failure
- smart charging limit applied

---

# 19. Validation Matrix

## 19.1 Boot

- charge point connects
- sends BootNotification
- handles Accepted
- handles Pending retry
- handles Rejected
- starts heartbeat only after accepted boot

## 19.2 Heartbeat

- sends at interval
- handles current time
- stops on disconnect
- restarts after reconnect

## 19.3 Authorization

- accepted idTag starts transaction
- invalid idTag blocks transaction
- expired idTag blocks transaction
- blocked idTag blocks transaction
- concurrent transaction behavior is configurable

## 19.4 Transactions

- cannot start on nonexistent connector
- cannot start on unavailable connector
- cannot start on faulted connector
- cannot start duplicate transaction on same connector
- stop clears transaction
- meter values bind to active transaction
- abnormal stop reason is stored

## 19.5 Remote Commands

- remote start accepted when connector is usable
- remote start rejected when connector is busy
- remote stop accepted for active transaction
- remote stop rejected for unknown transaction
- reset causes expected reboot flow
- unlock connector updates state correctly

## 19.6 Configuration

- known readonly key rejects update
- known writable key accepts update
- unknown key returns not supported
- reboot-required key marks charger pending reboot

## 19.7 Smart Charging

- valid profile accepted
- invalid connector rejected
- unsupported profile rejected
- active charging limit affects meter simulation
- clearing profile restores default behavior

---

# 20. Implementation Priority

## Phase 1: OCPP 1.6J Minimum Useful Simulator

Implement:

- WebSocket server/client support
- charge point registration
- connector model
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
- raw message log
- Vue UI for charge points, connectors, logs and manual actions

## Phase 2: OCPP 1.6J Operational Coverage

Add:

- ChangeAvailability
- GetConfiguration
- ChangeConfiguration
- ReserveNow
- CancelReservation
- SendLocalList
- GetLocalListVersion
- UpdateFirmware
- FirmwareStatusNotification
- GetDiagnostics
- DiagnosticsStatusNotification
- SetChargingProfile
- ClearChargingProfile

## Phase 3: OCPP 2.0.1 Adapter

Add:

- OCPP 2.0.1 WebSocket subprotocol
- 2.x BootNotification
- 2.x Heartbeat
- 2.x StatusNotification
- TransactionEvent
- structured id token authorization
- EVSE model
- basic device model
- 2.x message validation

## Phase 4: OCPP 2.1 Compatibility

Add only after 2.0.1 is stable:

- OCPP 2.1 subprotocol
- 2.1 schema support
- new 2.1 energy/grid-related features as needed
- compatibility test scenarios

---

# 21. Non-Goals

Do not implement these first:

- full certification test harness
- full ISO 15118 stack
- real payment processing
- production-grade PKI management
- hardware-level charger emulation
- every obscure vendor extension
- deep electricity grid simulation

These can be added later if the simulator becomes more than a development/testing tool.

---

# 22. Practical Rule of Thumb

For every OCPP action, implement five things:

1. parse message
2. validate payload
3. validate charger state
4. apply state change
5. emit log + response

For every state change, ask:

- does connector/EVSE status change?
- does transaction state change?
- should a notification be sent?
- should UI update immediately?
- should the raw OCPP message be stored?
- should this survive reconnect?

---

# 23. Official References

Use the official Open Charge Alliance specification packages as the source of truth.

- Open Charge Alliance OCPP protocol overview
- OCPP 1.6 official package and errata
- OCPP 2.0.1 official package and errata
- OCPP 2.1 official package and errata
- OCPP 1.6 Security Whitepaper

This document intentionally summarizes practical implementation rules and does not copy the full official specification.
