# OCPP 1.6-J — Heartbeat & StatusNotification Specification (LLM context)

## ROLE DEFINITIONS
- `CP` (Charge Point) = the station. INITIATES both Heartbeat and StatusNotification.
- `CS` (Central System) = the server/gateway. RESPONDS to both.

## TRANSPORT
- Same WebSocket connection and OCPP-J framing as BootNotification:
  - CALL (request): `[2, "<uniqueId>", "<Action>", <payloadObject>]`
  - CALLRESULT (response): `[3, "<uniqueId>", <payloadObject>]`
- RULE: CALLRESULT `uniqueId` MUST equal the CALL `uniqueId`.
- PRECONDITION: both messages are only sent after BootNotification returned `status: "Accepted"`.

---

# Heartbeat

## PURPOSE
- Keeps the connection alive and lets the CP re-sync its clock.
- The Heartbeat `interval` comes from the BootNotification `Accepted` response.

## REQUEST (CP → CS)
- Action string is exactly `"Heartbeat"`.
- Payload: EMPTY object `{}`. No fields.

## RESPONSE (CS → CP)
- Payload, REQUIRED:
  - `currentTime` — ISO 8601 UTC timestamp. CP uses this to sync its clock.

## DETERMINISTIC RULES
1. After BootNotification `Accepted`, CP starts a timer = `interval` seconds.
2. CP sends Heartbeat ONLY when no other message has been sent within the interval. ANY message sent to CS (StatusNotification, MeterValues, etc.) resets the heartbeat timer. Heartbeat is a fallback, not an unconditional periodic ping.
3. On each Heartbeat response, CP MAY re-sync its clock to `currentTime`.

## EXAMPLE
Request:
```json
[2, "heartbeat-001", "Heartbeat", {}]
```
Response:
```json
[3, "heartbeat-001", {"currentTime": "2026-06-02T10:20:00Z"}]
```

---

# StatusNotification

## PURPOSE
- Reports the status of the CP and of each individual connector.
- Sent on state change AND can be requested by CS via TriggerMessage.

## CONNECTOR MODEL (IMPORTANT)
- `connectorId = 0` refers to the CHARGE POINT itself (the whole station / main controller).
- `connectorId = 1..N` refers to each physical connector.
- RULE: on boot / first connection, the CP MUST send one StatusNotification PER CONNECTOR. A station with N connectors sends N StatusNotification messages (connectorId 1..N), plus optionally one for connectorId 0 (the station itself). So total = N (one per connector), or N+1 if the station-level status is also reported.
- Each connector is reported independently; their statuses are not shared.

## REQUEST (CP → CS)
- Action string is exactly `"StatusNotification"`.
- Payload fields:
  - `connectorId` — REQUIRED. integer >= 0. (0 = the CP itself; 1..N = connectors)
  - `errorCode` — REQUIRED. enum (see below).
  - `status` — REQUIRED. enum (see below).
  - `timestamp` — optional. ISO 8601 UTC, time the status was reported.
  - `info` — optional. string, max 50. Free-text additional info.
  - `vendorId` — optional. string, max 255.
  - `vendorErrorCode` — optional. string, max 50.

### `status` enum (ChargePointStatus)
- `Available` — connector free and ready.
- `Preparing` — cable plugged / authorizing, not yet charging.
- `Charging` — actively delivering energy.
- `SuspendedEVSE` — paused by the station/CS side.
- `SuspendedEV` — paused by the vehicle side.
- `Finishing` — session ending, cleanup.
- `Reserved` — reserved via ReserveNow.
- `Unavailable` — out of service (admin/maintenance).
- `Faulted` — error condition present.

### `errorCode` enum (ChargePointErrorCode)
- `NoError` — normal, no fault.
- `ConnectorLockFailure`
- `EVCommunicationError`
- `GroundFailure`
- `HighTemperature`
- `InternalError`
- `LocalListConflict`
- `OverCurrentFailure`
- `OverVoltage`
- `PowerMeterFailure`
- `PowerSwitchFailure`
- `ReaderFailure`
- `ResetFailure`
- `UnderVoltage`
- `WeakSignal`
- `OtherError`
- RULE: if `status` is not `Faulted`, `errorCode` is normally `NoError`.

## RESPONSE (CS → CP)
- Payload: EMPTY object `{}`. No fields. (Acknowledgement only.)

## DETERMINISTIC RULES
1. On connection (after BootNotification `Accepted`), CP sends one StatusNotification per connector (connectorId 1..N), reporting current status. Optionally also connectorId 0 for station-level status.
2. CP sends a new StatusNotification whenever a connector's `status` or `errorCode` changes.
3. On fault: set `status: "Faulted"` and a specific `errorCode`; optionally fill `info` / `vendorErrorCode`.
4. CS MUST respond with an empty CALLRESULT `{}` to every StatusNotification.

## EXAMPLES
Connector 1 available:
```json
[2, "status-001", "StatusNotification", {
  "connectorId": 1,
  "errorCode": "NoError",
  "status": "Available",
  "timestamp": "2026-06-02T10:15:31Z"
}]
```
Connector 2 charging:
```json
[2, "status-002", "StatusNotification", {
  "connectorId": 2,
  "errorCode": "NoError",
  "status": "Charging",
  "timestamp": "2026-06-02T10:16:05Z"
}]
```
Connector 1 faulted:
```json
[2, "status-003", "StatusNotification", {
  "connectorId": 1,
  "errorCode": "OverCurrentFailure",
  "status": "Faulted",
  "info": "Overcurrent on phase L1",
  "timestamp": "2026-06-02T10:18:40Z"
}]
```
Response (same for all):
```json
[3, "status-001", {}]
```

---

# TYPICAL POST-BOOT SEQUENCE (2-connector station)
1. WebSocket opens.
2. CP → BootNotification → CS replies `Accepted`, `interval: 300`.
3. CP → StatusNotification (connectorId 1, Available) → CS replies `{}`.
4. CP → StatusNotification (connectorId 2, Available) → CS replies `{}`.
   - (optionally connectorId 0 for station-level status)
5. CP → Heartbeat every 300s IF no other message was sent in that window.
6. Any connector state change → new StatusNotification for that connectorId.