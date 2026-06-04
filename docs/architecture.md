# Architecture & Message Flows

How the OCPP simulator is wired, what connects to what, and how a device's message gets a response. All facts here are derived from the actual code in `apps/` and `packages/`.

## Big picture — who connects to what

```
                        BROWSER (web)
              one separate WebSocket per CP
                          │  raw OCPP-J  [2,…]/[3,…]/[4,…]
                          ▼
                   ┌─────────────┐
                   │ ocpp-gateway│  :7080  /ws/{cpId}
                   │ (dumb edge) │  never parses the bytes
                   └──────┬──────┘
            publish ocpp/{cp}/in │ ▲ subscribe ocpp/{cp}/out
                          ▼ │
                   ┌─────────────────┐
                   │   MQTT broker   │  :1883  (mosquitto)
                   │ payload = raw OCPP-J, never wrapped │
                   └──┬───────────┬──┘
        sub ocpp/+/in │           │ sub ocpp/+/in & ocpp/+/out
                      ▼           ▼
         ┌────────────────┐   ┌──────────────┐
         │message-processor│  │  ocpp-core   │ :7090
         │ response producer│ │ canonical log│ + Postgres
         │  no DB, stdout  │  │ + transaction│ :5432
         └───────┬────────┘   │ + CSMS CALL  │
                 │  HTTP       └──────┬───────┘
                 └──── /internal/transactions/* ──┘
```

There are three distinct communication paths that never get mixed:

1. **Browser → gateway**: each simulated charge point opens its *own* WebSocket. There is no single shared socket.
2. **gateway ↔ MQTT ↔ backend**: the gateway publishes inbound frames to `ocpp/{cp}/in` and subscribes to `ocpp/{cp}/out`. MQTT payloads are always raw OCPP-J arrays.
3. **ocpp-core ↔ Postgres + internal HTTP**: business decisions and the canonical log live here.

The key invariant: **the gateway is dumb.** It does not parse OCPP, owns no state, and has no database. It only copies bytes (`apps/ocpp-gateway/internal/gateway/ws.go` → `pumpInbound` / `pumpOutbound`).

---

## Flow 1 — Device sends a message, how the response is produced

Example: the device sends `BootNotification` or `Heartbeat`.

```
1. The browser's CP socket writes a raw CALL:
   [2,"uid-1","BootNotification",{"chargePointVendor":"…","chargePointModel":"…"}]

2. ocpp-gateway reads it from the WS → publishes UNCHANGED:  ocpp/CP-1/in

3. message-processor receives the frame from its  ocpp/+/in  subscription
   (internal/processor/processor.go → subscribe loop)

4. handler.go → Codec.Decode(rawFrame)  → message.Message
   MessageTypeID == CALL  →  respondToCall()

5. dispatch by action (handler.go → respondToCall switch):
      BootNotification/Heartbeat/StatusNotification/MeterValues
        → LOCAL response (does not call core)
      Authorize/StartTransaction/StopTransaction
        → asks ocpp-core over HTTP

6. the response payload is VALIDATED against the official schema
   (ocpp-schemas.Validate(V16, action, Response, payload))
   if it fails → CALLERROR

7. Codec.BuildResult(uid, payload) → Codec.Encode → raw [3,"uid-1",{…}]
   publish:  ocpp/CP-1/out

8. ocpp-gateway receives from its  ocpp/CP-1/out  subscription →
   writes UNCHANGED to that CP's WebSocket

9. the browser's gatewayClient receives the frame and correlates it
   with the pending call (src/ocpp/callResponses.ts)
```

The `uid` (unique id) is the correlation key: the response comes back with the request's `uniqueId`; neither the gateway nor MQTT interprets it — it is carried end to end.

The crucial rule: **the place that produces the "response" to a message is message-processor, not the gateway.** The gateway only transports.

---

## Flow 2 — A message that requires a business decision (Authorize / StartTransaction)

Difference from `BootNotification`: message-processor *cannot* invent the response, because the transaction id and the idTag decision belong to the backend.

```
Device → [2,"uid-9","StartTransaction",{connectorId,idTag,meterStart,timestamp}]
  → gateway → ocpp/CP-1/in → message-processor

message-processor handler.go → handleStartTransaction():
  HTTP POST  ocpp-core  /internal/transactions/start
  body (INTERNAL envelope, NEVER appears on the wire):
     { "chargePointId":"CP-1", "payload":{connectorId,idTag,meterStart,…} }

ocpp-core (internal/api/server.go → handleStartTransaction):
  - transactionSvc.Start():
      * mints a new GUID  (transactions.id)
      * assigns numeric_id from the in-memory counter  ← 1.6J wire id
      * writes a row into the transactions table
  - returns the spec-exact StartTransaction.conf:
      { "transactionId": <numeric>, "idTagInfo":{"status":"Accepted"} }

message-processor:
  - takes the payload from core, validates it against the schema,
    publishes [3,"uid-9",{transactionId,idTagInfo}] to ocpp/CP-1/out
  → gateway → the device's socket
```

**Dual identity** is the heart of this flow:
- `transactions.id` → GUID, also the OCPP 2.0.1 wire id.
- `numeric_id` → the OCPP 1.6J wire id, assigned asynchronously via `StartTransaction.conf`.
- The counter is seeded from the DB with `MAX(numeric_id)` at startup (`main.go` → `transactionSvc.InitCounter`), so ids don't collide after a restart.

`StopTransaction` follows the same path but resolves in reverse: it scans for the incoming numeric `transactionId` via `ListByChargePoint` (`server.go` → `findTransactionByNumericID`), closes the row, and returns `StopTransaction.conf`.

---

## Flow 3 — CSMS initiates a command (RemoteStart, Reset, etc.) — the reverse direction

So far the device was always talking. In this flow the **server** (ocpp-core, acting in the CSMS role) initiates a CALL.

```
External/test client →  ocpp-core  POST /internal/csms/remote-start
   { "chargePointId":"CP-1", "payload":{"idTag":"ABC"} }

ocpp-core (internal/csms/service.go):
  - builds a spec-exact CALL:  [2,"<uid>","RemoteStartTransaction",{idTag,…}]
  - validates it against the official schema
  - publish:  ocpp/CP-1/out      ← direction toward the device
  - returns 202 {"status":"queued"} immediately (fire-and-forget)

ocpp-gateway:  ocpp/CP-1/out → the device's WebSocket

The device (browser) handles the command and returns a CALLRESULT:
  [3,"<uid>",{"status":"Accepted"}] → gateway → ocpp/CP-1/in

message-processor: this is a CALLRESULT, not a CALL →
   respondToCall is NOT called; audit "noop" only
   (handler.go: no response is produced for CALLRESULT/CALLERROR)
```

So direction matters: a **CALL** on `/in` → message-processor responds. A **CALLRESULT/CALLERROR** on `/in` → the device is answering the server's command; message-processor does not touch it, it is only logged.

`ocpp-core` can initiate these CSMS commands (all under `/internal/csms/*`): RemoteStart/StopTransaction, Reset, UnlockConnector, Change/GetConfiguration, TriggerMessage, ChangeAvailability.

---

## Flow 4 — Canonical logging (everything is recorded in parallel)

`ocpp-core` subscribes to **both** `ocpp/+/in` **and** `ocpp/+/out` (`internal/canonlog/service.go`). So in all three flows above, *every* raw frame in either direction is persisted to the `ocpp_message_logs` table independently of the main business path. This table is the source of truth for OCPP history.

Two kinds of logs are kept conceptually separate:
- **OCPP message logs** → `ocpp_message_logs` (raw wire traffic).
- **Runtime / business events** → `runtime_events` (e.g. `transaction.authorized`, with severity).

Important distinction: **runtime state (live connections, timers, counters) lives in memory.** Postgres only holds *history*; it is not the source of truth for live runtime state. On the browser side, CP config + UI logs live in IndexedDB — losing them on refresh is an accepted tradeoff.

---

## The layer boundary — why it is this strict

A single place produces/parses OCPP bytes: **`packages/ocpp-protocol/pkg/codec`**. Above it, the `protocol.Protocol` interface is the version-agnostic boundary. The internal domain model (GUID + numeric dual identity, connector state machine, runtime events) **never** crosses this boundary — only spec-exact payloads appear on the wire (`plan.md` §2b).

In practice the chain is:
```
internal model → codec/protocol (single translation point)
              → ocpp-schemas.Validate (conform to the official schema)
              → raw [2/3/4,…] frame → MQTT/WS
```
The validation is enforced by tests: a deliberately non-spec payload **must be rejected** by the validator.

---

## One-sentence summary

**The gateway transports, message-processor produces responses to device messages, ocpp-core makes the business decision and canonically logs everything, and the browser simulates each device over its own socket.**

## Service reference

| Service | Port | DB | Role |
|---|---|---|---|
| `ocpp-gateway` | `:7080` | no | Dumb WebSocket↔MQTT bridge, one `/ws/{cpId}` per CP |
| `message-processor` | `:7091` (health) | no (stdout audit) | CP→server response producer; calls core for business decisions |
| `ocpp-core` | `:7090` | Postgres `:5432` | Canonical log, transactions, CSMS-initiated CALLs |
| `web` | `:5173` | IndexedDB | Vue dashboard; one OCPP WebSocket per simulated CP |
| MQTT (mosquitto) | `:1883` | — | Raw OCPP-J transport between gateway and backend |

See each service's `README.md` for env vars and endpoints, and `plan.md` for the full design rationale.
