# OCPP Simulator — Connector Start Flow Revision Plan

## 1. PROBLEM (current state)
In the current UI, every connector has a direct **Start TX** button. This button starts a transaction in one click, skipping the preconditions of a real charging flow. In the real world this is impossible:

- A connector in `Available` state cannot start a transaction without a cable plugged in.
- Starting a transaction MUST be tied to an authorization (local Authorize **or** RemoteStartTransaction).
- There is no "start directly" concept on the unit; starting is triggered either by card (auth) or remotely (remote).

**Goal:** remove the "Start TX" button and turn the connector into a component that behaves according to the real OCPP state machine, with state-dependent controls.

---

## 2. TARGET CONNECTOR STATE MACHINE

```
Available ──Plug In──> Preparing ──(Auth OK | RemoteStart)──> Charging
   ^                       │                                     │
   │                       └──Unplug──> Available                │
   │                                                             │
   └──────────────── Finishing <──Stop / Unplug─────────────────┘

Available/Preparing/Charging ──Fault──> Faulted ──Clear──> (previous valid state)
Available ──Disable──> Unavailable ──Enable──> Available
```

States and their OCPP `status` mapping:
- `Available` — idle, no cable plugged.
- `Preparing` — cable plugged, not yet authorized/started.
- `Charging` — active charging (transaction open).
- `SuspendedEV` / `SuspendedEVSE` — paused (optional, later phase).
- `Finishing` — session ending, cable still plugged.
- `Faulted` — error condition.
- `Unavailable` — out of service.

---

## 3. STATE-DEPENDENT UI CONTROLS

Each connector card must show DIFFERENT buttons depending on the state it is in. Instead of a fixed button set like "Start TX", the actions change with the state:

| Connector state | Buttons shown | Triggered event |
|---|---|---|
| `Available` | **Plug In**, Fault, Disable | Plug In → `Preparing` (+ StatusNotification) |
| `Preparing` | **Authorize** (with idTag input), **Unplug**, Fault | Authorize flow or Unplug → `Available` |
| `Charging` | **Stop**, (Suspend EV / Suspend EVSE — optional), Fault | Stop → StopTransaction → `Finishing` |
| `Finishing` | **Unplug** | Unplug → `Available` |
| `Faulted` | **Clear Fault** | → returns to the appropriate previous state |
| `Unavailable` | **Enable** | → `Available` |

**Critical:** in `Available` state there must be NO Authorize/Start button — Plug In is required first. Authorize is only meaningful in the `Preparing` state.

---

## 4. THE TWO START FLOWS

### 4A. LOCAL AUTHORIZATION (Auth flow) — CP-initiated
The flow the simulator initiates itself. The card/token is presented by the user.

UI: an **idTag input** + an **Authorize** button on the `Preparing` connector card.

Sequence:
1. User clicks **Plug In** → connector goes `Preparing` → `StatusNotification(Preparing)` is sent.
2. User enters idTag + clicks **Authorize** → CP sends `Authorize` CALL.
3. CS returns `idTagInfo.status`:
   - If NOT `Accepted` → UI shows "authorization rejected", connector returns to `Preparing`/`Available`. Transaction does NOT start.
   - If `Accepted` → CP sends `StartTransaction`, receives `transactionId`.
4. Connector goes `Charging` → `StatusNotification(Charging)` is sent.
5. `MeterValues` start flowing periodically.

### 4B. REMOTE START (Remote start) — CS-initiated
This CALL **comes from the CS**; the simulator RECEIVES it and reacts. So logically there is no "remote start" button in the simulator; the message arrives over the WS.

Sequence:
1. (Before or after) Plug In → `Preparing`.
2. CS → `RemoteStartTransaction(idTag, connectorId?)` CALL arrives over the WS.
3. Simulator responds with `status: Accepted` (command accepted).
4. Simulator automatically sends `StartTransaction` → receives `transactionId`.
5. Connector goes `Charging` → `StatusNotification(Charging)`.

**Testability note:** since remote start is triggered by the CS, there are two options to exercise it from the simulator UI:
- (Preferred) the CS/server side's own UI/endpoint sends RemoteStartTransaction; the simulator just reacts and the UI shows the state change.
- (Practical convenience) a small "Simulate Remote Start (incoming)" test button can be added to the simulator — but this must be marked as a DEV tool that mimics a message coming from the server, not as a real CP action.

---

## 5. MESSAGE SEQUENCES (summary)

Full flow with local auth:
```
[Plug In]      CP → StatusNotification(connectorId, Preparing)   ← CS {}
[Authorize]    CP → Authorize(idTag)                             ← CS {idTagInfo: Accepted}
               CP → StartTransaction(connectorId, idTag, meterStart, ts) ← CS {transactionId, idTagInfo}
               CP → StatusNotification(connectorId, Charging)     ← CS {}
[charging]     CP → MeterValues(... transactionId ...)  (periodic)
[Stop]         CP → StopTransaction(transactionId, meterStop, ts, reason) ← CS {idTagInfo}
               CP → StatusNotification(connectorId, Finishing)    ← CS {}
[Unplug]       CP → StatusNotification(connectorId, Available)     ← CS {}
```

With remote start:
```
[Plug In]      CP → StatusNotification(connectorId, Preparing)   ← CS {}
               CS → RemoteStartTransaction(idTag, connectorId)   → CP {status: Accepted}
               CP → StartTransaction(...)                        ← CS {transactionId}
               CP → StatusNotification(connectorId, Charging)     ← CS {}
...stop flow works the same way via RemoteStopTransaction or local Stop...
```

---

## 6. IMPLEMENTATION CHECKLIST (frontend)

State / model:
- [ ] Extend the connector model's `status` field to the OCPP enum (Available, Preparing, Charging, Finishing, Faulted, Unavailable; later SuspendedEV/EVSE).
- [ ] Keep `transactionId`, `idTag`, `meterStart`, `startedAt` on the connector for the active transaction.
- [ ] A `cablePluggedIn` flag for plug state (precondition for Preparing).

UI components:
- [ ] Remove the fixed `Start TX` / `Fault` / `Disable` trio.
- [ ] Turn the connector card into a component that renders state-dependent buttons (see Section 3 table).
- [ ] Show an idTag input + Authorize button in the `Preparing` state.
- [ ] Show a Stop button + (optional) live meter/duration indicator in the `Charging` state.
- [ ] Block invalid actions (e.g. Authorize in `Available`) at the UI level.

Behavior:
- [ ] Plug In → `Preparing` transition + StatusNotification send.
- [ ] Authorize → if Accepted, auto StartTransaction chain; if rejected, revert state.
- [ ] Listen for incoming RemoteStartTransaction → respond Accepted → run the StartTransaction chain.
- [ ] Stop / RemoteStop → StopTransaction → Finishing → Unplug → Available.
- [ ] Automatic StatusNotification on every state change.

Live Logs:
- [ ] Verify that the new flow's messages (Authorize, StartTransaction, MeterValues, StopTransaction, RemoteStart) land in the logs under the correct category (Ocpp / Transaction).

---

## 7. OUT OF SCOPE (for now)
- SuspendedEV / SuspendedEVSE pause flows (second phase).
- ChargingProfile / smart charging limits.
- Reservation (ReserveNow → Reserved state).
- Auth-first ordering (card first, cable later). The first phase implements only the plug-first → Preparing model; auth-first can be deferred to a second phase.