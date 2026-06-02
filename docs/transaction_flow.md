# OCPP 1.6-J — Transaction Cycle Specification (LLM context)
### Authorize · StartTransaction · MeterValues · StopTransaction

## ROLE DEFINITIONS
- `CP` (Charge Point) = the station. INITIATES all four messages below.
- `CS` (Central System) = the server/gateway. RESPONDS to all four.

## TRANSPORT
- Same WebSocket connection and OCPP-J framing as before:
  - CALL (request): `[2, "<uniqueId>", "<Action>", <payloadObject>]`
  - CALLRESULT (response): `[3, "<uniqueId>", <payloadObject>]`
- RULE: CALLRESULT `uniqueId` MUST equal the CALL `uniqueId`.
- PRECONDITION: all four are only sent after BootNotification returned `status: "Accepted"`.

## SHARED TYPE: IdTagInfo
Returned inside several responses. Fields:
- `status` — REQUIRED. enum `AuthorizationStatus`:
  - `Accepted` — idTag is valid, charging allowed.
  - `Blocked` — idTag blocked.
  - `Expired` — idTag expired.
  - `Invalid` — idTag unknown/invalid.
  - `ConcurrentTx` — idTag already in an active transaction elsewhere. (valid only in StartTransaction context)
- `expiryDate` — optional. ISO 8601 UTC. When the authorization expires.
- `parentIdTag` — optional. string, max 20. Parent/group idTag.

---

# Authorize

## PURPOSE
- Asks CS whether a given idTag (RFID card / token) is allowed to charge, BEFORE starting a transaction.
- Optional step: stations with a local authorization list/cache may skip it, but the canonical flow includes it.

## REQUEST (CP → CS)
- Action string is exactly `"Authorize"`.
- Payload:
  - `idTag` — REQUIRED. string, max 20. The token presented by the user.

## RESPONSE (CS → CP)
- Payload:
  - `idTagInfo` — REQUIRED. IdTagInfo object (see above).

## DETERMINISTIC RULES
1. CP starts charging only if `idTagInfo.status == "Accepted"`.
2. Any other status → CP refuses, returns connector toward `Available`/`Finishing`.

## EXAMPLE
```json
[2, "auth-001", "Authorize", {"idTag": "ABC12345"}]
```
```json
[3, "auth-001", {"idTagInfo": {"status": "Accepted", "expiryDate": "2026-12-31T23:59:59Z"}}]
```

---

# StartTransaction

## PURPOSE
- Informs CS that a charging session has begun. CS assigns a unique `transactionId`.

## REQUEST (CP → CS)
- Action string is exactly `"StartTransaction"`.
- Payload fields:
  - `connectorId` — REQUIRED. integer > 0. (must be a real connector, NOT 0)
  - `idTag` — REQUIRED. string, max 20.
  - `meterStart` — REQUIRED. integer. Energy meter reading at start, in **Wh**.
  - `timestamp` — REQUIRED. ISO 8601 UTC. Start time.
  - `reservationId` — optional. integer. If this start consumes a reservation.

## RESPONSE (CS → CP)
- Payload fields:
  - `transactionId` — REQUIRED. integer. Unique id assigned by CS; CP MUST store and use it in MeterValues and StopTransaction.
  - `idTagInfo` — REQUIRED. IdTagInfo object.

## DETERMINISTIC RULES
1. CP sends StartTransaction after authorization succeeds and energy delivery is about to begin.
2. CP MUST persist `transactionId`; it is the key for all subsequent messages of this session.
3. If `idTagInfo.status != "Accepted"` in the response, CS still created the transaction id, but CP SHOULD stop charging (deauthorized). Handle per status (e.g. `ConcurrentTx`, `Blocked`).
4. After this, connector status transitions to `Charging` (reported via StatusNotification).

## EXAMPLE
```json
[2, "start-001", "StartTransaction", {
  "connectorId": 1,
  "idTag": "ABC12345",
  "meterStart": 1200340,
  "timestamp": "2026-06-02T10:16:00Z"
}]
```
```json
[3, "start-001", {
  "transactionId": 778812,
  "idTagInfo": {"status": "Accepted"}
}]
```

---

# MeterValues

## PURPOSE
- Periodic sampled meter data DURING a transaction (energy, power, current, voltage, SoC, etc.).
- Sent on the configured `MeterValueSampleInterval`.

## REQUEST (CP → CS)
- Action string is exactly `"MeterValues"`.
- Payload fields:
  - `connectorId` — REQUIRED. integer >= 0.
  - `transactionId` — optional. integer. The transaction these samples belong to. Include it for transaction-related samples.
  - `meterValue` — REQUIRED. array of MeterValue objects (at least 1). Each:
    - `timestamp` — REQUIRED. ISO 8601 UTC.
    - `sampledValue` — REQUIRED. array of SampledValue objects. Each:
      - `value` — REQUIRED. string (numeric content).
      - `context` — optional. enum, e.g. `Sample.Periodic`, `Transaction.Begin`, `Transaction.End`, `Sample.Clock`.
      - `format` — optional. enum: `Raw` (default) or `SignedData`.
      - `measurand` — optional. enum, e.g. `Energy.Active.Import.Register`, `Power.Active.Import`, `Current.Import`, `Voltage`, `SoC`, `Temperature`. Default = `Energy.Active.Import.Register`.
      - `phase` — optional. enum, e.g. `L1`, `L2`, `L3`, `N`, `L1-N`.
      - `location` — optional. enum: `Outlet` (default), `Inlet`, `Body`, `Cable`, `EV`.
      - `unit` — optional. enum, e.g. `Wh` (default), `kWh`, `W`, `kW`, `A`, `V`, `Percent`, `Celsius`.

## RESPONSE (CS → CP)
- Payload: EMPTY object `{}`. Acknowledgement only.

## DETERMINISTIC RULES
1. CP sends MeterValues at the configured sample interval while `Charging`.
2. Include `transactionId` so CS can attribute samples to the session.
3. CS responds with empty `{}`.

## EXAMPLE
```json
[2, "mv-001", "MeterValues", {
  "connectorId": 1,
  "transactionId": 778812,
  "meterValue": [{
    "timestamp": "2026-06-02T10:21:00Z",
    "sampledValue": [
      {"value": "1203500", "measurand": "Energy.Active.Import.Register", "unit": "Wh", "context": "Sample.Periodic"},
      {"value": "7360", "measurand": "Power.Active.Import", "unit": "W"}
    ]
  }]
}]
```
```json
[3, "mv-001", {}]
```

---

# StopTransaction

## PURPOSE
- Informs CS that the charging session has ended. Carries final meter reading and reason.

## REQUEST (CP → CS)
- Action string is exactly `"StopTransaction"`.
- Payload fields:
  - `transactionId` — REQUIRED. integer. The id from StartTransaction.
  - `meterStop` — REQUIRED. integer. Final energy meter reading, in **Wh**.
  - `timestamp` — REQUIRED. ISO 8601 UTC. Stop time.
  - `idTag` — optional. string, max 20. The token that stopped the session (may differ from the one that started it).
  - `reason` — optional. enum `Reason`:
    - `EmergencyStop`, `EVDisconnected`, `HardReset`, `Local`, `Other`,
      `PowerLoss`, `Reboot`, `Remote`, `SoftReset`, `UnlockCommand`, `DeAuthorized`.
    - If omitted, default reason is `Local`.
  - `transactionData` — optional. array of MeterValue objects (same shape as in MeterValues). Final/summary samples, typically with `context: "Transaction.End"`.

## RESPONSE (CS → CP)
- Payload fields:
  - `idTagInfo` — optional. IdTagInfo object. Present when an `idTag` was supplied in the request (to confirm the stopping token).

## DETERMINISTIC RULES
1. Energy consumed for the session = `meterStop - meterStart` (Wh).
2. After StopTransaction, the `transactionId` is closed; CP MUST NOT send further MeterValues for it.
3. Connector transitions toward `Finishing` then `Available` (reported via StatusNotification).
4. If `idTag` was provided, CS MAY return `idTagInfo`; CP need not block on it (session already ended).

## EXAMPLE
```json
[2, "stop-001", "StopTransaction", {
  "transactionId": 778812,
  "meterStop": 1255340,
  "timestamp": "2026-06-02T11:05:00Z",
  "idTag": "ABC12345",
  "reason": "Local",
  "transactionData": [{
    "timestamp": "2026-06-02T11:05:00Z",
    "sampledValue": [
      {"value": "1255340", "measurand": "Energy.Active.Import.Register", "unit": "Wh", "context": "Transaction.End"}
    ]
  }]
}]
```
```json
[3, "stop-001", {"idTagInfo": {"status": "Accepted"}}]
```

---

# TYPICAL FULL TRANSACTION SEQUENCE
1. User presents card → CP → `Authorize` (idTag) → CS replies `idTagInfo.status: Accepted`.
2. CP → `StatusNotification` (connectorId 1, `Preparing`).
3. CP → `StartTransaction` (connectorId, idTag, meterStart, timestamp) → CS replies `transactionId`.
4. CP → `StatusNotification` (connectorId 1, `Charging`).
5. CP → `MeterValues` (with transactionId) repeatedly at the sample interval.
6. Charging stops (cable unplugged / stop pressed / remote) →
   CP → `StatusNotification` (connectorId 1, `Finishing`).
7. CP → `StopTransaction` (transactionId, meterStop, timestamp, reason) → CS replies `{}` / `idTagInfo`.
8. CP → `StatusNotification` (connectorId 1, `Available`).
9. Heartbeat resumes as the idle fallback.

# KEY INVARIANTS
- `transactionId` is created by CS in StartTransaction and is the join key for MeterValues and StopTransaction.
- Meter readings (`meterStart`, `meterStop`, register samples) are all in **Wh** unless `unit` says otherwise.
- Every state change of a connector is mirrored by a `StatusNotification`.
- Energy billed = `meterStop - meterStart`.