# OCPP 1.6-J — CS-Initiated Messages + REST API Mapping (LLM context)

## DIRECTION (READ FIRST)
- Previous messages (BootNotification, Heartbeat, StatusNotification, Authorize, StartTransaction, MeterValues, StopTransaction) are **CP-initiated**: the station sends the CALL, the Central System (CS) answers.
- The messages in THIS document are **CS-initiated**: the CS/gateway sends the CALL, and the station (CP) answers with the CALLRESULT.
- These are triggered from OUTSIDE — by a REST API call into the gateway (operator portal, mobile app, backend job).

## ROLE DEFINITIONS
- `CS` (Central System / gateway) = INITIATES the CALL here.
- `CP` (Charge Point) = the station. RESPONDS with CALLRESULT.

## TRANSPORT
- Same single WebSocket connection and OCPP-J framing:
  - CALL (request): `[2, "<uniqueId>", "<Action>", <payloadObject>]`
  - CALLRESULT (response): `[3, "<uniqueId>", <payloadObject>]`
  - CALLERROR: `[4, "<uniqueId>", "<errorCode>", "<errorDescription>", <errorDetails>]`
- RULE: CALLRESULT `uniqueId` MUST equal the CALL `uniqueId`. The gateway generates the `uniqueId` for CS-initiated CALLs and matches the incoming CALLRESULT against it.

---

# PART 1 — CS-INITIATED OCPP MESSAGES

## RemoteStartTransaction
PURPOSE: ask the station to begin a charging session on behalf of a user (e.g. mobile "start charging" button).

REQUEST (CS → CP), Action `"RemoteStartTransaction"`:
- `idTag` — REQUIRED. string, max 20.
- `connectorId` — optional. integer > 0. If omitted, CP picks an available connector.
- `chargingProfile` — optional. ChargingProfile object (power/current limits for the session).

RESPONSE (CP → CS):
- `status` — REQUIRED. enum `RemoteStartStopStatus`: `Accepted` | `Rejected`.

NOTE: `Accepted` means the CP accepted the COMMAND, not that charging started. Actual start is confirmed asynchronously by the CP-initiated `StartTransaction` + `StatusNotification(Charging)` that follow.

EXAMPLE:
```json
[2, "rs-001", "RemoteStartTransaction", {"idTag": "ABC12345", "connectorId": 1}]
```
```json
[3, "rs-001", {"status": "Accepted"}]
```

---

## RemoteStopTransaction
PURPOSE: ask the station to stop an ongoing session by transactionId.

REQUEST (CS → CP), Action `"RemoteStopTransaction"`:
- `transactionId` — REQUIRED. integer. The active transaction to stop.

RESPONSE (CP → CS):
- `status` — REQUIRED. enum `RemoteStartStopStatus`: `Accepted` | `Rejected`.

NOTE: `Accepted` = command accepted. The actual stop is confirmed by the CP-initiated `StopTransaction` that follows.

EXAMPLE:
```json
[2, "rstop-001", "RemoteStopTransaction", {"transactionId": 778812}]
```
```json
[3, "rstop-001", {"status": "Accepted"}]
```

---

## Reset
PURPOSE: reboot the station (soft or hard).

REQUEST (CS → CP), Action `"Reset"`:
- `type` — REQUIRED. enum `ResetType`: `Soft` (graceful) | `Hard` (immediate).

RESPONSE (CP → CS):
- `status` — REQUIRED. enum `ResetStatus`: `Accepted` | `Rejected`.

NOTE: after a Hard reset, expect a new `BootNotification` from the CP once it comes back up.

EXAMPLE:
```json
[2, "reset-001", "Reset", {"type": "Soft"}]
```
```json
[3, "reset-001", {"status": "Accepted"}]
```

---

## UnlockConnector
PURPOSE: force-unlock a connector's cable (e.g. cable stuck after a session).

REQUEST (CS → CP), Action `"UnlockConnector"`:
- `connectorId` — REQUIRED. integer > 0. (cannot be 0)

RESPONSE (CP → CS):
- `status` — REQUIRED. enum `UnlockStatus`: `Unlocked` | `UnlockFailed` | `NotSupported`.

EXAMPLE:
```json
[2, "unlock-001", "UnlockConnector", {"connectorId": 1}]
```
```json
[3, "unlock-001", {"status": "Unlocked"}]
```

---

## ChangeConfiguration
PURPOSE: set a single configuration key on the station.

REQUEST (CS → CP), Action `"ChangeConfiguration"`:
- `key` — REQUIRED. string, max 50.
- `value` — REQUIRED. string, max 500.

RESPONSE (CP → CS):
- `status` — REQUIRED. enum `ConfigurationStatus`: `Accepted` | `Rejected` | `RebootRequired` | `NotSupported`.

EXAMPLE:
```json
[2, "cfg-001", "ChangeConfiguration", {"key": "HeartbeatInterval", "value": "300"}]
```
```json
[3, "cfg-001", {"status": "Accepted"}]
```

---

## GetConfiguration
PURPOSE: read configuration keys from the station.

REQUEST (CS → CP), Action `"GetConfiguration"`:
- `key` — optional. array of strings (max 50 each). If omitted/empty, CP returns ALL keys.

RESPONSE (CP → CS):
- `configurationKey` — optional. array of: `{ key, readonly (bool), value? }`.
- `unknownKey` — optional. array of strings (requested keys the CP doesn't know).

EXAMPLE:
```json
[2, "gcfg-001", "GetConfiguration", {"key": ["HeartbeatInterval", "MeterValueSampleInterval"]}]
```
```json
[3, "gcfg-001", {
  "configurationKey": [
    {"key": "HeartbeatInterval", "readonly": false, "value": "300"},
    {"key": "MeterValueSampleInterval", "readonly": false, "value": "60"}
  ],
  "unknownKey": []
}]
```

---

## TriggerMessage
PURPOSE: ask the station to proactively send a specific CP-initiated message NOW (useful to refresh status).

REQUEST (CS → CP), Action `"TriggerMessage"`:
- `requestedMessage` — REQUIRED. enum `MessageTrigger`: `BootNotification` | `Heartbeat` | `StatusNotification` | `MeterValues` | `DiagnosticsStatusNotification` | `FirmwareStatusNotification`.
- `connectorId` — optional. integer > 0. Scope the trigger to one connector.

RESPONSE (CP → CS):
- `status` — REQUIRED. enum `TriggerMessageStatus`: `Accepted` | `Rejected` | `NotImplemented`.

NOTE: on `Accepted`, the CP then sends the requested CP-initiated message as a separate CALL.

EXAMPLE:
```json
[2, "trig-001", "TriggerMessage", {"requestedMessage": "StatusNotification", "connectorId": 1}]
```
```json
[3, "trig-001", {"status": "Accepted"}]
```

---

## ChangeAvailability
PURPOSE: take a connector (or the whole station) in/out of service.

REQUEST (CS → CP), Action `"ChangeAvailability"`:
- `connectorId` — REQUIRED. integer >= 0. (0 = whole station)
- `type` — REQUIRED. enum `AvailabilityType`: `Operative` | `Inoperative`.

RESPONSE (CP → CS):
- `status` — REQUIRED. enum `AvailabilityStatus`: `Accepted` | `Rejected` | `Scheduled`.

NOTE: `Scheduled` = will change after the current transaction finishes.

EXAMPLE:
```json
[2, "avail-001", "ChangeAvailability", {"connectorId": 1, "type": "Inoperative"}]
```
```json
[3, "avail-001", {"status": "Accepted"}]
```

---

# PART 2 — REST API ↔ OCPP MAPPING

## CORE ARCHITECTURE RULE
A REST endpoint is **synchronous HTTP**, but the underlying OCPP message is an **asynchronous WebSocket CALL**. The gateway MUST bridge these two. For every CS-initiated endpoint, the handler does:

1. Resolve the target CP's live WebSocket connection by chargePointId.
   - If NO open connection → return `409 Conflict` (or `503`) with `"charge point offline"`. Do NOT queue silently.
2. Generate a fresh `uniqueId`.
3. Register a pending request keyed by `uniqueId` (e.g. a `TaskCompletionSource` stored in `ConcurrentDictionary<string, Pending>`).
4. Send the OCPP CALL over the WS.
5. `await` the pending request WITH A TIMEOUT (typical 30s).
   - On CALLRESULT arrival → match by `uniqueId`, complete the task, map the OCPP `status` to an HTTP response.
   - On CALLERROR → complete with an error, return `502`/`400` as appropriate.
   - On timeout → remove the pending entry, return `504 Gateway Timeout`.
6. NEVER leave a pending request unbounded — always clean up on timeout/disconnect.

## SEMANTIC RULE: COMMAND-ACCEPT vs ACTUAL-RESULT
For Remote(Start|Stop)Transaction, the OCPP `Accepted` only means the COMMAND was accepted. The real outcome (charging actually started/stopped) arrives LATER as a separate CP-initiated `StartTransaction` / `StopTransaction` + `StatusNotification`. The API has two valid designs:
- **Fire-and-confirm (recommended):** endpoint returns `202 Accepted` once the CP returns OCPP `Accepted`; the actual start/stop is observed via the follow-up CP-initiated messages (push to client via webhook/WebSocket/polling).
- **Block-until-result:** endpoint waits for the follow-up StartTransaction/StopTransaction too. Heavier; needs correlation of the follow-up message back to the original request.

## ENDPOINT MAP

| REST endpoint | OCPP CALL | Request DTO (body) | Maps OCPP status → HTTP |
|---|---|---|---|
| `POST /chargepoints/{id}/remote-start` | RemoteStartTransaction | `{ idTag, connectorId?, chargingProfile? }` | `Accepted` → 202 ; `Rejected` → 409 |
| `POST /chargepoints/{id}/remote-stop` | RemoteStopTransaction | `{ transactionId }` | `Accepted` → 202 ; `Rejected` → 409 |
| `POST /chargepoints/{id}/reset` | Reset | `{ type: "Soft"\|"Hard" }` | `Accepted` → 200 ; `Rejected` → 409 |
| `POST /chargepoints/{id}/connectors/{cid}/unlock` | UnlockConnector | `{}` (cid from path) | `Unlocked` → 200 ; `UnlockFailed`/`NotSupported` → 409 |
| `PUT /chargepoints/{id}/config` | ChangeConfiguration | `{ key, value }` | `Accepted` → 200 ; `RebootRequired` → 200 (+flag) ; `Rejected`/`NotSupported` → 409 |
| `GET /chargepoints/{id}/config` | GetConfiguration | query `?keys=a,b` (optional) | returns `{ configurationKey[], unknownKey[] }` → 200 |
| `POST /chargepoints/{id}/trigger` | TriggerMessage | `{ requestedMessage, connectorId? }` | `Accepted` → 202 ; `Rejected`/`NotImplemented` → 409 |
| `POST /chargepoints/{id}/availability` | ChangeAvailability | `{ connectorId, type }` | `Accepted` → 200 ; `Scheduled` → 202 ; `Rejected` → 409 |

## STANDARD HTTP ERROR MAPPING (apply to all endpoints)
- CP WebSocket not connected → `409 Conflict` body `{ "error": "charge_point_offline" }`.
- No CALLRESULT within timeout → `504 Gateway Timeout`.
- CALLERROR received from CP → `502 Bad Gateway` with the OCPP errorCode/description.
- Invalid request body (missing required field, bad enum) → `400 Bad Request` BEFORE sending any OCPP CALL.
- Unknown chargePointId → `404 Not Found`.

## PSEUDOCODE (single CS-initiated endpoint)
```
POST /chargepoints/{id}/remote-start { idTag, connectorId? }

handler:
  cp = registry.find(id)
  if cp == null            -> 404
  if !cp.isConnected       -> 409 charge_point_offline
  validate(body)           -> on fail 400

  uid = newUniqueId()
  pending = registerPending(uid, timeout = 30s)
  cp.send([2, uid, "RemoteStartTransaction", { idTag, connectorId }])

  try:
    result = await pending            // resolved when CALLRESULT [3, uid, {...}] arrives
  catch Timeout:
    removePending(uid); return 504

  switch result.status:
    "Accepted" -> 202 { commandAccepted: true }   // actual start arrives via later StartTransaction
    "Rejected" -> 409 { commandAccepted: false }
```

## CALLRESULT DISPATCH (gateway WS receive loop)
```
on websocket message [3, uid, payload]:
  pending = pendingMap.remove(uid)
  if pending != null:
     pending.complete(payload)        // CS-initiated command result
  else:
     // unmatched uniqueId -> log/ignore (stale or duplicate)

on websocket message [2, uid, action, payload]:
  // CP-initiated (BootNotification, StatusNotification, StartTransaction, etc.)
  result = handleCpInitiated(action, payload)
  cp.send([3, uid, result])
```

# KEY INVARIANTS
- Gateway owns `uniqueId` generation and the pending-request map for CS→CP CALLs; CP owns it for CP→CS CALLs.
- Every CS-initiated endpoint MUST handle three failure modes explicitly: CP offline, timeout, CALLERROR.
- OCPP `Accepted` on Remote(Start|Stop) ≠ charging started/stopped; that truth comes from the follow-up CP-initiated transaction messages.
- Reset(Hard) is expected to be followed by a fresh BootNotification.