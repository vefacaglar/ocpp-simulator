# OCPP 1.6-J — BootNotification Specification (LLM context)

## ROLE DEFINITIONS
- `CP` (Charge Point) = the station. INITIATES BootNotification.
- `CS` (Central System) = the server/gateway. RESPONDS to BootNotification.

## TRANSPORT
- Messages travel over a single WebSocket connection.
- Wire format is a JSON array (OCPP-J framing):
  - CALL (request): `[2, "<uniqueId>", "<Action>", <payloadObject>]`
  - CALLRESULT (response): `[3, "<uniqueId>", <payloadObject>]`
  - CALLERROR: `[4, "<uniqueId>", "<errorCode>", "<errorDescription>", <errorDetails>]`
- RULE: the `uniqueId` in CALLRESULT MUST exactly equal the `uniqueId` of the CALL it answers. This is the ONLY mechanism for matching responses to requests.

## REQUEST (CP → CS)
- Action string is exactly `"BootNotification"`.
- Payload fields:
  - `chargePointVendor` — REQUIRED. string, max 20 chars.
  - `chargePointModel` — REQUIRED. string, max 20 chars.
  - `chargePointSerialNumber` — optional. string, max 25.
  - `chargeBoxSerialNumber` — optional. string, max 25. (legacy; deprecated in 1.6 but still accepted)
  - `firmwareVersion` — optional. string, max 50.
  - `iccid` — optional. string, max 20. (SIM card ICCID)
  - `imsi` — optional. string, max 20. (SIM card IMSI)
  - `meterType` — optional. string, max 25.
  - `meterSerialNumber` — optional. string, max 25.
- CONSTRAINT: if a required field is missing or exceeds max length, CS responds with CALLERROR (`FormationViolation` or `TypeConstraintViolation`), NOT a CALLRESULT.

## RESPONSE (CS → CP)
- Payload fields, ALL THREE REQUIRED:
  - `status` — enum, one of: `"Accepted"`, `"Pending"`, `"Rejected"`.
  - `currentTime` — ISO 8601 UTC timestamp (e.g. `"2026-06-02T10:15:30Z"`). CP uses this to sync its clock.
  - `interval` — integer, seconds. MEANING DEPENDS ON `status` (see state table).

## STATE TRANSITION TABLE (core logic)

| status | meaning of `interval` | CP behavior after receiving | CS allowed to send requests to CP? |
|---|---|---|---|
| `Accepted` | Heartbeat interval (seconds) | CP is registered. Starts Heartbeat loop every `interval` sec. May send StatusNotification. Transactions allowed. | Yes |
| `Pending` | Min seconds before CP MAY resend BootNotification | CP does NOT auto-send other messages. Waits. CS is expected to push configuration first. | Yes (this is the purpose of Pending) |
| `Rejected` | Min seconds CP MUST wait before resending BootNotification | CP is NOT registered. Must NOT send any other message. Retries BootNotification after waiting. | No |

## DETERMINISTIC RULES (for an agent to follow)
1. CP MUST send BootNotification as its first OCPP message after the WebSocket opens, before any other Action.
2. Until `status: "Accepted"` is received, CP MUST NOT initiate Authorize, StartTransaction, StopTransaction, or MeterValues. (Heartbeat/StatusNotification rules per table above.)
3. On `Accepted`: set heartbeatInterval = `interval`. Begin Heartbeat timer. Sync local clock to `currentTime`.
4. On `Pending`: do NOT start Heartbeat. Wait. Respond to any request CS sends. If `interval` elapses with no resolution, CP MAY resend BootNotification.
5. On `Rejected`: do NOT start Heartbeat. Wait at least `interval` seconds, then resend BootNotification. If `interval == 0`, CP falls back to its own default retry interval.
6. CS decides status from CP identity (e.g. whitelist/registration lookup by vendor+model+serial or by the connection's chargePointId). Unknown/unregistered → `Rejected`. Known but needs provisioning → `Pending`. Known and ready → `Accepted`.

## EXAMPLES

Request:
```json
[2, "19223201", "BootNotification", {
  "chargePointVendor": "VendorX",
  "chargePointModel": "ModelY",
  "chargePointSerialNumber": "SN-12345",
  "firmwareVersion": "1.4.2"
}]
```

Accepted response:
```json
[3, "19223201", {"status": "Accepted", "currentTime": "2026-06-02T10:15:30Z", "interval": 300}]
```

Pending response:
```json
[3, "19223201", {"status": "Pending", "currentTime": "2026-06-02T10:15:30Z", "interval": 60}]
```

Rejected response:
```json
[3, "19223201", {"status": "Rejected", "currentTime": "2026-06-02T10:15:30Z", "interval": 600}]
```