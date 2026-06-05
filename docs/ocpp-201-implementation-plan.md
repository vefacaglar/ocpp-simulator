# OCPP 2.0.1 Implementation Plan (multi-version, core transaction flow)

> Self-contained handoff document. The implementing model has NOT seen the design
> discussion that produced this — everything needed is below, including verified
> facts about the current codebase. **Read `AGENTS.md` and `plan.md` first**; the
> hard rules there override anything here.

## 0. Non-negotiable constraints (from `AGENTS.md` / `plan.md` §2b)

- **Strict OCPP compliance on the wire.** Every byte on the wire (and on MQTT) must
  conform character-for-character to the official OCPP JSON schema for the negotiated
  version: field names/casing, types, required/optional, enum strings, value ranges,
  OCPP-J framing (`[2,...]` CALL, `[3,...]` CALLRESULT, `[4,...]` CALLERROR).
- **Never wrap wire messages.** MQTT payloads stay raw OCPP-J arrays. Topic names may
  carry routing metadata (`chargePointId`, direction, and — new in this plan — version),
  but the payload is never an envelope/DTO.
- **Internal models never leak to the wire.** The version `codec`/`protocol` package is
  the single boundary translating internal state ↔ spec-exact payloads.
- **Official schemas in `packages/ocpp-schemas` are the single source of truth.** Tests
  validate every built/parsed payload against them. A deliberately non-spec payload must
  be *rejected* by the validator in tests.
- Goal: **1.6J and 2.0.1 supported simultaneously** (e.g. CP-001 on 1.6J and CP-002 on
  2.0.1 at the same time). 1.6J behavior must not regress.
- Scope: **core transaction flow only** (Boot, Heartbeat, Status, Authorize, transaction
  start/stop, MeterValues) plus the CSMS-initiated calls already supported for 1.6J.
  No Smart Charging, no full device-model management beyond what core flow needs.

## 1. Verified current state (do not re-derive — confirmed by inspection)

Repo layout (Go workspace via `go.work`; modules built per-directory, not a single root
`go test ./...`):

```
apps/ocpp-gateway/      # dumb WebSocket edge + MQTT bridge
apps/message-processor/ # CP→server response producer (stdout audit only, no DB)
apps/ocpp-core/         # canonical OCPP log + transactions/business (owns Postgres)
apps/web/               # Vue 3 simulator; builds its OWN TS wire frames (see note)
packages/ocpp-protocol/ # Go: version-agnostic Protocol interface + v16 + v201
packages/ocpp-schemas/  # Go: official schemas + embed + Validate()
```

Key facts confirmed in code:

1. **`packages/ocpp-schemas`**
   - `validator.go` embeds only `//go:embed v16/*.json`. The `V201` case currently
     returns `"ocpp-schemas: v2.0.1 not yet supported"`.
   - `v201/` exists but contains only `BootNotification.json`. All other v2.0.1 schemas
     are missing.

2. **`packages/ocpp-protocol`**
   - `pkg/protocol/protocol.go` defines ONE monolithic `Protocol` interface shaped
     entirely around 1.6J semantics: `BuildStartTransaction`, `BuildStopTransaction`,
     `BuildAuthorize(idTag)`, `ParseGetConfigurationRequest`,
     `ParseChangeConfigurationRequest`, etc. There is a `Factory` that maps a version
     string → `Protocol` (`NewFactory`, `Register`, `Create`).
   - `pkg/v16/protocol.go` fully implements it.
   - `pkg/v201/protocol.go` exists but every method returns `errNotImplemented`
     ("ocpp 2.0.1 protocol not implemented yet"). It is forced to satisfy the 1.6J-shaped
     interface.
   - **Critical, verified:** the CP-initiated `Build*` methods and the `Parse*Request` /
     `Build*Response` methods have **zero production callers in Go** — they are exercised
     **only by tests** (`pkg/v16/protocol_test.go`). This makes the interface refactor
     low-risk (mostly mechanical + test updates).

3. **`apps/ocpp-gateway`**
   - `internal/gateway/ws.go` advertises `Subprotocols: []string{"ocpp1.6", "ocpp2.0.1"}`
     but **never reads `conn.Subprotocol()`** — the negotiated version is currently thrown
     away. No rejection when the client negotiates no OCPP subprotocol.
   - `internal/gateway/gateway.go` builds topics `ocpp/{cpId}/in` and `ocpp/{cpId}/out`
     (version-agnostic). `Connect(cpID)` takes no version.

4. **`apps/message-processor`**
   - `internal/processor/handler.go` dispatches on the OCPP **action string**
     (`switch msg.Action` ~line 110: `BootNotification`, `Heartbeat`, `StatusNotification`,
     `MeterValues`, `Authorize`, `StartTransaction`, `StopTransaction`).
   - `handleStartTransaction`/`handleStopTransaction` forward the **raw payload** to
     `ocpp-core` (`h.Core.StartTransaction`) — they do NOT call the typed `Build*` methods.
   - `validateResponseSchema` hardcodes `ocppschemas.V16` (~line 255). Version is not
     plumbed in.

5. **`apps/web`** builds its OCPP-J frames in TypeScript (`chargePointStore.ts`) and does
   **not** consume the Go `protocol` package. So the Go interface refactor does not touch
   the frontend; the frontend gets its own parallel v201 work (Phase 5).

> Net consequence of facts 2 & 5: the often-feared "we must change every call site of the
> Protocol interface" is largely false. In Go the typed methods are test-only; the web is
> independent. The refactor is cheap and should be done test-first.

## 2. The central design decision: capability-segregated Protocol interface

The single monolithic `Protocol` interface is the root blocker. 2.0.1 does not have
`StartTransaction`/`StopTransaction` (replaced by one `TransactionEvent`) nor
`Get/ChangeConfiguration` (replaced by `Get/SetVariables`). Forcing v201 to implement
1.6J-only methods is wrong.

**Do NOT** "fake" v201 by mapping `BuildStartTransaction` → `TransactionEvent` internally
(hidden state, `seqNo` buried, technical debt every version). **Do NOT** recombine the
capabilities back into one fat `Protocol` super-interface (that re-forces v201 to
implement 1.6J methods — same problem returns).

**Decision: split into small capability interfaces, segregated by SEMANTIC version
boundary, and never recombine into a fat interface.**

```go
// packages/ocpp-protocol/pkg/protocol/protocol.go

// Common to every version.
type BaseProtocol interface {
    Version() string
    BuildBootNotification(ctx context.Context, in BootNotificationInput) (message.Message, error)
    BuildHeartbeat(ctx context.Context) (message.Message, error)
    BuildStatusNotification(ctx context.Context, in StatusNotificationInput) (message.Message, error)
    BuildMeterValues(ctx context.Context, in MeterValuesInput) (message.Message, error)
    BuildAuthorize(ctx context.Context, in AuthorizeInput) (message.Message, error)
}

// 1.6J ONLY — implemented by v16, NOT by v201.
type LegacyTransactionProtocol interface {
    BuildStartTransaction(ctx context.Context, in StartTransactionInput) (message.Message, error)
    BuildStopTransaction(ctx context.Context, in StopTransactionInput) (message.Message, error)
}

// 2.0.1 ONLY — implemented by v201, NOT by v16.
type TransactionEventProtocol interface {
    BuildTransactionEvent(ctx context.Context, in TransactionEventInput) (message.Message, error)
}

// 1.6J ONLY.
type LegacyConfigProtocol interface {
    ParseGetConfigurationRequest(p json.RawMessage) (*GetConfigurationRequest, error)
    ParseChangeConfigurationRequest(p json.RawMessage) (*ChangeConfigurationRequest, error)
    BuildGetConfigurationResponse(keys []ConfigurationKey, unknown []string) (json.RawMessage, error)
    BuildChangeConfigurationResponse(status string) (json.RawMessage, error)
}

// 2.0.1 ONLY (device-model variables).
type VariableProtocol interface {
    ParseGetVariablesRequest(p json.RawMessage) (*GetVariablesRequest, error)
    ParseSetVariablesRequest(p json.RawMessage) (*SetVariablesRequest, error)
    BuildGetVariablesResponse(...) (json.RawMessage, error)
    BuildSetVariablesResponse(...) (json.RawMessage, error)
}

// CSMS-initiated calls whose RESPONSE is just a status string and whose request
// shapes are structurally close across versions: Reset, UnlockConnector,
// TriggerMessage, ChangeAvailability. Shared interface.
// RESIDUAL (Phase E): the Parse*Request structs for these four still differ in 2.0.1
// (Reset: type enum + optional evse; UnlockConnector: evseId+connectorId;
// TriggerMessage: requestedMessage enum incl. TransactionEvent + evse;
// ChangeAvailability: operationalStatus + evse). For MVP they stay 1.6J-shaped with a
// documented TODO; make the request structs version-aware when 2.0.1 CSMS-initiated
// parsing is actually implemented. The shared Build*Response(status) are genuinely fine.
type RemoteControlProtocol interface {
    ParseResetRequest(p json.RawMessage) (*ResetRequest, error)
    ParseUnlockConnectorRequest(p json.RawMessage) (*UnlockConnectorRequest, error)
    ParseTriggerMessageRequest(p json.RawMessage) (*TriggerMessageRequest, error)
    ParseChangeAvailabilityRequest(p json.RawMessage) (*ChangeAvailabilityRequest, error)
    BuildResetResponse(status string) (json.RawMessage, error)
    BuildUnlockConnectorResponse(status string) (json.RawMessage, error)
    BuildTriggerMessageResponse(status string) (json.RawMessage, error)
    BuildChangeAvailabilityResponse(status string) (json.RawMessage, error)
}

// CSMS-initiated transaction control diverges by version (different action NAMES and
// payload shapes) — split, do NOT share. Same reasoning as the transaction split above.
type LegacyRemoteTxProtocol interface { // 1.6J: RemoteStart/StopTransaction
    ParseRemoteStartTransactionRequest(p json.RawMessage) (*RemoteStartTransactionRequest, error)
    ParseRemoteStopTransactionRequest(p json.RawMessage) (*RemoteStopTransactionRequest, error)
    BuildRemoteStartTransactionResponse(status string) (json.RawMessage, error)
    BuildRemoteStopTransactionResponse(status string) (json.RawMessage, error)
}
type RemoteTxProtocol interface { // 2.0.1: RequestStart/StopTransaction (idToken, remoteStartId)
    ParseRequestStartTransactionRequest(p json.RawMessage) (*RequestStartTransactionRequest, error)
    ParseRequestStopTransactionRequest(p json.RawMessage) (*RequestStopTransactionRequest, error)
    BuildRequestStartTransactionResponse(status string) (json.RawMessage, error)
    BuildRequestStopTransactionResponse(status string) (json.RawMessage, error)
}
```

**Factory: one `Register` + capability discovery via type assertion at registration time**
(not per call site). Each version is a single struct implementing several capabilities;
register it once and the factory populates the relevant maps:

```go
func (f *Factory) Register(p BaseProtocol) {
    v := p.Version()
    f.base[v] = p
    if c, ok := p.(LegacyTransactionProtocol); ok { f.legacyTransaction[v] = c }
    if c, ok := p.(TransactionEventProtocol);  ok { f.transactionEvent[v] = c }
    if c, ok := p.(LegacyConfigProtocol);      ok { f.legacyConfig[v] = c }
    if c, ok := p.(VariableProtocol);          ok { f.variable[v] = c }
    if c, ok := p.(RemoteControlProtocol);     ok { f.remoteControl[v] = c }
    if c, ok := p.(LegacyRemoteTxProtocol);    ok { f.legacyRemoteTx[v] = c }
    if c, ok := p.(RemoteTxProtocol);          ok { f.remoteTx[v] = c }
}
// Getters all return (T, bool): Base, LegacyTransaction, TransactionEvent,
// LegacyConfig, Variable, RemoteControl, LegacyRemoteTx, RemoteTx.
```

**Compile-time contract — write the `var _` assertions explicitly** (Go does not check
this for you). In `v16/protocol.go`:
```go
var (
    _ protocol.BaseProtocol              = (*Protocol)(nil)
    _ protocol.LegacyTransactionProtocol = (*Protocol)(nil)
    _ protocol.LegacyConfigProtocol      = (*Protocol)(nil)
    _ protocol.RemoteControlProtocol     = (*Protocol)(nil)
    _ protocol.LegacyRemoteTxProtocol    = (*Protocol)(nil)
)
```
In `v201/protocol.go`: `BaseProtocol`, `TransactionEventProtocol`, `VariableProtocol`,
`RemoteControlProtocol`, `RemoteTxProtocol`.

**Auth note:** `BuildAuthorize` is in `BaseProtocol` but the input differs (1.6J `idTag`
string vs 2.0.1 `idToken {type, idToken}`). Use a unified `AuthorizeInput` that carries
both an `IDTag string` and an optional structured `IDToken` (type + value); each version's
codec reads what it needs. Alternatively give v201 its own auth capability — pick the
unified-input approach to keep `BaseProtocol` stable. Document the choice in code.

### Factory shape (decide explicitly)

The Factory must NOT return a fat `Protocol`. Two acceptable shapes:

- **Option F1 (capability getters, type-safe, preferred):**
  `f.Base(version)`, `f.TransactionEvent(version) (TransactionEventProtocol, bool)`,
  `f.LegacyTransaction(version) (LegacyTransactionProtocol, bool)`, etc. Callers ask for
  exactly the capability they need; the compiler enforces availability.
- **Option F2 (return `BaseProtocol`, assert for extras):** `f.Create(version) BaseProtocol`
  then `if tx, ok := p.(TransactionEventProtocol); ok { ... }`. Less ceremony, but pushes
  runtime assertions to call sites.

**Recommended: F1.** It keeps version mismatches as compile/registration errors rather
than runtime assertions, and there are almost no call sites to update.

### Routing seam (how this is consumed)

Do not scatter `proto.(SomeCapability)` assertions across the handler. Instead route by
**version-scoped handler**, keyed off the version that the gateway tags onto the message
(see §3). `message-processor` already switches on the action string; give it a per-version
handler/router so the v16 handler knows the `StartTransaction` case and the v201 handler
knows the `TransactionEvent` case. This also removes the hardcoded `V16` in
`validateResponseSchema`.

## 3. Version discovery: gateway tags the version onto the MQTT topic

The gateway is the ONLY component that sees the WebSocket subprotocol handshake
(`ocpp1.6` / `ocpp2.0.1`). Version is fixed per session. Therefore the gateway is the
correct owner of version discovery, and doing so does NOT violate the "dumb edge" rule:
it reads transport-level handshake metadata, parses no payloads, owns no business state.

**Topic schema change (chosen approach — topic segment, not MQTT5 property):**

```
old:  ocpp/{cpId}/in            ocpp/{cpId}/out
new:  ocpp/{version}/{cpId}/in  ocpp/{version}/{cpId}/out
      e.g. ocpp/2.0.1/CP-002/in
```

Rationale for topic segment over an MQTT5 user property: greppable, broker-agnostic
(works on MQTT 3.1.1), trivial to debug, and the canonical-log `ocpp_version` column fills
itself from the topic. Payload stays a raw OCPP-J array → compliance intact.

Gateway changes (`apps/ocpp-gateway`):
- In `ws.go`, after `upgrader.Upgrade`, read `conn.Subprotocol()`. Map `ocpp1.6`→`1.6J`
  (match the version string the Factory/schemas use — verify the exact casing used in
  `ocpp-schemas`/`protocol`, currently `"2.0.1"` / `V201`) and `ocpp2.0.1`→`2.0.1`.
  **Reject the connection** (close with a protocol error) if no OCPP subprotocol was
  negotiated — this is also spec-correct.
- Thread the version into `Connect(cpID, version)` and the topic builders in
  `gateway.go` (`inTopic`/`outTopic` gain a version segment).

Consumer subscription changes:
- `message-processor`: `ocpp/+/in` → `ocpp/+/+/in`; extract version from the topic and
  select the version-scoped handler; drop hardcoded `V16` in schema validation.
- `ocpp-core` canonical log: `ocpp/+/in` and `ocpp/+/out` → `ocpp/+/+/in` / `ocpp/+/+/out`;
  persist `ocpp_version` from the topic segment.
- `ocpp-core` CSMS publisher: publish to `ocpp/{version}/{cpId}/out` (it knows the CP's
  version from its own records).

> This is the dispatch key that feeds §2's routing seam: gateway tags version → processor
> picks the version handler → handler uses the right capabilities. The two are
> complementary, not alternatives. Neither replaces the other.

## 4. OCPP 2.0.1 message specifics (core flow) — spec-exact, watch these

| 1.6J | 2.0.1 |
|------|-------|
| `BootNotification` `chargePointVendor`,`chargePointModel` | `chargingStation` object (`model`,`vendorName`, optional `serialNumber`,`firmwareVersion`,`modem`) + `reason` enum |
| `StatusNotification` `connectorId`,`status`,`errorCode` | `timestamp`,`connectorStatus`,`evseId`,`connectorId` (no `errorCode`) |
| `Authorize` `idTag` | `idToken {idToken, type}` (type enum: `ISO14443`,`ISO15693`,`Central`,`KeyCode`,`Local`,`MacAddress`,`NoAuthorization`,...) |
| `StartTransaction` + `StopTransaction` | **one** `TransactionEvent` with `eventType` ∈ {`Started`,`Updated`,`Ended`} |
| `MeterValues` `connectorId`,`transactionId?`,`meterValue[]` | `evseId`,`meterValue[]` (tx context lives in the TransactionEvent / `transactionInfo`) |
| `Heartbeat` `{}` (resp `currentTime`) | `{}` (resp `currentTime`) |

**`TransactionEventRequest` mandatory/important fields (commonly missed):**
- `eventType` (Started/Updated/Ended)
- `timestamp`
- `triggerReason` (enum: `Authorized`,`CablePluggedIn`,`ChargingStateChanged`,
  `MeterValuePeriodic`,`RemoteStart`,`RemoteStop`,`EVCommunicationLost`,`StopAuthorized`,
  `EVDeparted`, ...)
- **`seqNo`** — REQUIRED. A per-transaction monotonically increasing counter starting at 0.
  This forces transaction-scoped state on the sender (the simulator). Model it explicitly
  in `TransactionEventInput` (`SeqNo int`) — do NOT hide it.
- `transactionInfo` — object containing `transactionId` (string/GUID, chosen by the CP),
  optionally `chargingState`, `stoppedReason`, `remoteStartId`.
- `evse {id, connectorId?}`
- `idToken` — optional on `Started`/`Updated`; when present on `Started` it is the
  "combined authorization" path (CP need not send a separate `Authorize`). The
  `TransactionEventResponse` then carries `idTokenInfo` with the auth status.
- `offline` (bool), `numberOfPhasesUsed?`, `cableMaxCurrent?`, `reservationId?`,
  `meterValue[]?`.

**`TransactionEventResponse`:** may carry `totalCost`, `chargingPriority`,
`idTokenInfo {status,...}`, `updatedPersonalMessage`. The simulator must accept these.

**Transaction identity (aligns with `plan.md` §7.6/§8.4/§13 dual identity):** in 2.0.1 the
CP generates the transaction GUID locally and puts it on the wire as
`transactionInfo.transactionId`. This maps cleanly to the project's internal
`transactions.id` GUID. There is **no async numeric_id** in 2.0.1 — the 1.6J
`StartTransaction.conf`-assigned `numeric_id` simply does not apply. Keep `numeric_id`
NULL/unused for 2.0.1 rows.

**EVSE/connector MVP mapping:** `evseId = connectorNumber` (≥1), `connectorId = 1`. This is
an internal modeling choice; the wire output must still be spec-exact.

**CSMS-initiated 2.0.1 differences:** `RequestStartTransaction` (carries `idToken`,
`remoteStartId`, optional `evseId`), `RequestStopTransaction` (`transactionId` string).
Note the 2.0.1 action names differ from 1.6J `RemoteStart/StopTransaction` — register them
under their 2.0.1 names. `TriggerMessage` `requestedMessage` enum includes
`TransactionEvent`.

## 5. Phased execution

Run Go tests per module (`cd apps/... && go test ./...`; same for `packages/...`). Tests
are the primary enforcement of strict compliance — for every action: build → encode →
validate against the official schema; cover CALL/CALLRESULT/CALLERROR framing, decode,
round-trip, correlation, and golden fixtures. A non-spec payload must be rejected.

### Phase A — Schemas (`packages/ocpp-schemas`)
1. Download the official 2.0.1 schemas. **Source (verified working):**
   **`github.com/mobilityhouse/ocpp`** under `ocpp/v201/schemas/` — these are the canonical
   OCA "OCPP 2.0.1 FINAL" JSON schemas (`"comment": "OCPP 2.0.1 FINAL"` inside each file),
   vendored verbatim. Alternative byte-identical source: `github.com/EVerest/libocpp`.
   The OCA itself distributes these only in a registration-gated spec package and has no
   public cloneable schema repo, so use the vendored copy above; content is identical —
   provenance/naming is the only difference.

   **Naming mismatch to handle:** official files are `{Action}Request.json` /
   `{Action}Response.json`, but the validator loader (`validator.go` builds
   `{filename}.json` where response appends `Response`) expects the v16 convention
   `{Action}.json` (request) / `{Action}Response.json` (response). So rename the request
   file on copy. Verified working fetch (use `curl -f` so a 404 errors instead of leaving a
   14-byte `"404: Not Found"` junk file):
   ```bash
   cd packages/ocpp-schemas/v201
   BASE="https://raw.githubusercontent.com/mobilityhouse/ocpp/master/ocpp/v201/schemas"
   for a in BootNotification Heartbeat StatusNotification Authorize TransactionEvent \
            MeterValues RequestStartTransaction RequestStopTransaction Reset \
            UnlockConnector TriggerMessage ChangeAvailability GetVariables SetVariables; do
     curl -fsSL "$BASE/${a}Request.json"  -o "${a}.json"
     curl -fsSL "$BASE/${a}Response.json" -o "${a}Response.json"
   done
   ```
   Replace the existing `v201/BootNotification.json` placeholder. Delete any leftover
   14-byte `*Request.json` junk files from earlier failed downloads.
2. Add `//go:embed v201/*.json`; implement the `V201` case in `validator.go` (remove the
   "not yet supported" error). 2.0.1 schemas are **draft-06** (v16 is draft-04) and
   **self-contained** — each file carries its own `definitions` block and refs them with
   intra-file `#/definitions/...`, so there is NO cross-file `$ref` to resolve. The bundled
   `santhosh-tekuri/jsonschema/v5` supports draft-06; add a test that compiles a draft-06
   schema to confirm.
3. Tests: known-good v201 payloads pass; malformed (missing `seqNo`, bad enum) rejected.

### Phase B — Protocol interface refactor (`packages/ocpp-protocol`) — DO FIRST, test-first
1. Split the monolithic `Protocol` into the capability interfaces of §2. Keep the
   version-specific request/input structs.
2. Update `v16/protocol.go` to implement `BaseProtocol + LegacyTransactionProtocol +
   LegacyConfigProtocol + RemoteControlProtocol + LegacyRemoteTxProtocol`. Behavior
   unchanged; only the interface set it satisfies changes. Add explicit `var _` assertions
   (see §2). Update `v16/protocol_test.go` accordingly (these are the only real callers).
3. Factory **F1** with a single `Register(BaseProtocol)` that discovers capabilities via
   type assertion at registration time (see §2); add the eight `(T, bool)` getters.
4. Unified inputs: `AuthorizeInput{ IDTag string; IDToken *IDToken }` and
   `MeterValuesInput{ ConnectorID int; EVSEID int; TransactionID *int; MeterValues []MeterValue }`.
5. Leave v201 stubs compiling but unimplemented for now (next phase fills them); add v201
   `var _` assertions for `BaseProtocol, TransactionEventProtocol, VariableProtocol,
   RemoteControlProtocol, RemoteTxProtocol`.

   **Phase B test-first targets:** `TestFactoryRegisterDiscoversCapabilities`
   (`Register(v16.New())` → all five v16 getters `(impl, true)`; `TransactionEvent("1.6J")`
   → `(nil, false)`), existing v16 golden fixtures unchanged (proves no behavior drift),
   and `TransactionEvent("2.0.1")` → `(nil, false)` until Phase C registers v201. Use plain
   stdlib `testing` (the ocpp-protocol module does NOT depend on testify — do not add it).

6. **Cross-module ripple — Phase B is NOT isolated to the protocol package.**
   `apps/message-processor/internal/processor/handler.go` references the monolithic
   `protocol.Protocol` as a struct field (`Protocol protocol.Protocol`) and constructor
   param (`NewHandler(p protocol.Protocol, ...)`), and `cmd/processor/main.go` passes
   `v16.NewProtocol()`. The field is **dead** — no `h.Protocol.<method>()` call exists
   (the handler dispatches on the action string and forwards to ocpp-core). Removing the
   monolithic `Protocol` interface therefore breaks message-processor's build even though
   the field is unused. Fix as part of Phase B: drop the dead field entirely (cleanest) or
   retype it to `protocol.BaseProtocol`; update `NewHandler`, `main.go`, and
   `handler_test.go`. (`Factory.Create` has no production callers, so removing it is safe.)
   Verification MUST include `cd apps/message-processor && go build ./... && go test ./...`,
   not just the protocol package — a green protocol package can hide a broken workspace.
   The v201 types to ADD in step 1 (none exist yet): `IDToken`, `TransactionEventInput`,
   `GetVariablesRequest`, `SetVariablesRequest`, `RequestStartTransactionRequest`,
   `RequestStopTransactionRequest`.

### Phase C — v201 protocol implementation (`packages/ocpp-protocol/pkg/v201`)
1. Implement `BaseProtocol`, `TransactionEventProtocol`, `VariableProtocol`,
   `RemoteControlProtocol` per §4. Remove the 1.6J-only methods (v201 must NOT implement
   `LegacyTransactionProtocol`/`LegacyConfigProtocol`).
2. Add `TransactionEventInput` with explicit `SeqNo`, `EventType`, `TriggerReason`,
   `Offline`, `TransactionID` (string), `EVSEID`, `ConnectorID`, optional `IDToken`,
   `MeterValue[]`, `StoppedReason`.
3. Tests + golden fixtures under `testdata/v201/` with exact-byte assertions; validate
   against Phase A schemas.

### Phase D — Gateway version tagging (`apps/ocpp-gateway`)
Implement §3: read `conn.Subprotocol()`, reject non-OCPP, thread version into
`Connect`/topics (`ocpp/{version}/{cpId}/{dir}`). Update gateway tests.

### Phase E — Backend integration (`message-processor`, `ocpp-core`)
1. `message-processor`: subscribe `ocpp/+/+/in`; derive version from topic; version-scoped
   handler routing; handle `TransactionEvent` (route to ocpp-core), 2.0.1 `Authorize`
   (idToken); validate responses against the **correct** version schema (kill hardcoded
   `V16`). Register a v201 protocol in the factory at startup.
2. `ocpp-core`: transaction service accepts a 2.0.1 path (string `transactionId` from CP,
   `evseId`, no async numeric assignment; `numeric_id` stays NULL). `TransactionEvent`
   `eventType=Ended` is the stop path — no separate Stop call. CSMS service builds v201
   CALL frames (RequestStart/Stop...) validated against v201 schemas. Canonical log
   persists `ocpp_version` from the topic; publish CSMS calls to
   `ocpp/{version}/{cpId}/out`.

### Phase F — Frontend simulator (`apps/web`)
The web builds its own TS frames (independent of the Go refactor).
1. `chargePointStore.ts`: version-aware runtime. Per-CP version; route Boot/Auth/
   transaction/MeterValues/Status to v16 vs v201 builders.
2. Implement v201 TS builders: `sendBootNotificationV201` (chargingStation+reason),
   `sendAuthorizeV201` (idToken), `sendTransactionEventStarted/Updated/Ended` (with
   **per-transaction `seqNo` state**, `triggerReason`, string `transactionId` GUID,
   `evse`), `sendStatusNotificationV201` (evseId+connectorId+connectorStatus),
   `sendMeterValuesV201` (evseId).
3. Handle inbound 2.0.1 CSMS calls (RequestStart/Stop, TriggerMessage incl.
   `TransactionEvent`).
4. UI: enable 2.0.1 in the version selector; connector cards show evseId+connectorId;
   transaction logs show string `transactionId` + `eventType`.

### Phase G — E2E + test matrix
| Test | 1.6J | 2.0.1 |
|------|------|-------|
| BootNotification | ✓ | chargingStation+reason |
| Heartbeat | ✓ | ✓ |
| Authorize | ✓ | idToken |
| Tx start | StartTransaction | TransactionEvent(Started), seqNo=0 |
| Tx meter | MeterValues | TransactionEvent(Updated)/MeterValues, evseId |
| Tx stop | StopTransaction | TransactionEvent(Ended) |
| StatusNotification | ✓ | evseId+connectorId |
| Schema validation | ✓ | ✓ (reject missing seqNo) |
| CSMS RemoteStart | RemoteStartTransaction | RequestStartTransaction |
| **Mixed**: CP-001 (1.6J) + CP-002 (2.0.1) simultaneously | — | must both work |

## 6. Ordering, risks, definition of done

- **Order:** B → A/C → D/E → F → G. Phase B (interface refactor) is the unblock; it is
  cheap because the Go typed methods are test-only. Do it first and test-first.
- **Risk — 2.0.1 schema `$ref` resolution:** v2.0.1 schemas cross-reference shared
  definitions; the embedded validator must resolve them. Validate this in Phase A before
  building on it.
- **Risk — version string consistency:** the subprotocol token (`ocpp2.0.1`), the schema
  version (`V201 = "2.0.1"`), the Factory key, and the MQTT topic segment must all agree.
  Pick canonical strings (`"1.6J"`, `"2.0.1"`) and map at the gateway boundary.
- **Risk — topic migration is cross-service:** gateway, message-processor, and ocpp-core
  must change the topic schema together; a half-migrated broker silently drops traffic.
- **Do NOT:** wrap payloads, leak internal models, fake `BuildStartTransaction` on v201,
  recombine capabilities into a fat interface, or regress 1.6J.
- **Done when:** the mixed-version E2E (CP-001 1.6J + CP-002 2.0.1) runs a full
  authorize→start→meter→stop on both concurrently, every wire/MQTT frame validates against
  its official schema in tests, and `go test ./...` passes in each module.

## 7. File-change summary

| File | Change |
|------|--------|
| `packages/ocpp-schemas/v201/*.json` | Add official 2.0.1 schemas |
| `packages/ocpp-schemas/validator.go` | Embed v201, implement `V201` case |
| `packages/ocpp-protocol/pkg/protocol/protocol.go` | Split into capability interfaces; F1 Factory |
| `packages/ocpp-protocol/pkg/v16/protocol.go` (+`_test.go`) | Satisfy split interfaces (behavior unchanged) |
| `packages/ocpp-protocol/pkg/v201/protocol.go` (+`_test.go`, `testdata/v201/`) | Implement Base/TxEvent/Variable/RemoteControl |
| `apps/ocpp-gateway/internal/gateway/ws.go` | Read `conn.Subprotocol()`, reject non-OCPP, pass version |
| `apps/ocpp-gateway/internal/gateway/gateway.go` | Version segment in topics; `Connect(cpID, version)` |
| `apps/message-processor/.../handler.go` + `main.go` | Version-scoped routing; `ocpp/+/+/in`; per-version schema validation; register v201 |
| `apps/ocpp-core/internal/transaction/service.go` | 2.0.1 tx path (string id, no numeric assignment) |
| `apps/ocpp-core/internal/csms/service.go` | v201 CSMS CALL builders + validation |
| `apps/ocpp-core/...canonical log + publisher` | `ocpp/+/+/...`; persist+publish version |
| `apps/web/src/stores/chargePointStore.ts` | Version-aware runtime + v201 builders (incl. seqNo state) |
| `apps/web/src/api/chargePointsApi.ts` | Enable 2.0.1 in version list |
