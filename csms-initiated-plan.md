# CSMS-Initiated Messages + REST API — Implementation Plan

> Superseded note: the standalone mock Central System described in this historical plan was removed. The accepted implementation is `ocpp-core` internal API → MQTT `ocpp/{chargePointId}/out` → `ocpp-gateway` → target charge point WebSocket.

## Context

The simulator currently only supports **CP-initiated** messages (BootNotification, Heartbeat, StatusNotification, Authorize, StartTransaction, MeterValues, StopTransaction). The CSMS-initiated direction — where the Central System sends a CALL and the Charge Point responds — is entirely unimplemented at every layer: schemas, protocol interface, OCPP client, runtime, and API.

This plan adds support for 8 CSMS-initiated messages and their REST API mapping, following the specification provided.

## Scope

**8 CSMS-initiated OCPP 1.6J messages:**
1. RemoteStartTransaction
2. RemoteStopTransaction
3. Reset
4. UnlockConnector
5. ChangeConfiguration
6. GetConfiguration
7. TriggerMessage
8. ChangeAvailability

**Two integration points:**
- **Simulator (CP side):** receive inbound CALLs, execute actions, return CALLRESULT
- **Mock CSMS (CS side):** REST endpoints that send CALLs to connected CPs, await CALLRESULT, return HTTP response

---

## Implementation Steps

### Step 1: OCPP JSON Schemas (packages/ocpp-schemas)

Add 16 schema files to `packages/ocpp-schemas/v16/`:

| File | Purpose |
|------|---------|
| `RemoteStartTransaction.json` | Request schema |
| `RemoteStartTransactionResponse.json` | Response schema |
| `RemoteStopTransaction.json` | Request schema |
| `RemoteStopTransactionResponse.json` | Response schema |
| `Reset.json` | Request schema |
| `ResetResponse.json` | Response schema |
| `UnlockConnector.json` | Request schema |
| `UnlockConnectorResponse.json` | Response schema |
| `ChangeConfiguration.json` | Request schema |
| `ChangeConfigurationResponse.json` | Response schema |
| `GetConfiguration.json` | Request schema |
| `GetConfigurationResponse.json` | Response schema |
| `TriggerMessage.json` | Request schema |
| `TriggerMessageResponse.json` | Response schema |
| `ChangeAvailability.json` | Request schema |
| `ChangeAvailabilityResponse.json` | Response schema |

Schemas follow the same pattern as existing ones (draft-04, `urn:ocpp:1.6:1.6:ActionRequest`, `additionalProperties: false`). Source: official OCA OCPP 1.6J specification.

### Step 2: Protocol Interface Extension (apps/api/internal/ocpp/protocol.go)

Add to the `Protocol` interface:

```go
// CSMS-initiated: parse inbound request payloads
ParseRemoteStartTransactionRequest(payload json.RawMessage) (*RemoteStartTransactionRequest, error)
ParseRemoteStopTransactionRequest(payload json.RawMessage) (*RemoteStopTransactionRequest, error)
ParseResetRequest(payload json.RawMessage) (*ResetRequest, error)
ParseUnlockConnectorRequest(payload json.RawMessage) (*UnlockConnectorRequest, error)
ParseChangeConfigurationRequest(payload json.RawMessage) (*ChangeConfigurationRequest, error)
ParseGetConfigurationRequest(payload json.RawMessage) (*GetConfigurationRequest, error)
ParseTriggerMessageRequest(payload json.RawMessage) (*TriggerMessageRequest, error)
ParseChangeAvailabilityRequest(payload json.RawMessage) (*ChangeAvailabilityRequest, error)

// CSMS-initiated: build response payloads
BuildRemoteStartTransactionResponse(status string) (json.RawMessage, error)
BuildRemoteStopTransactionResponse(status string) (json.RawMessage, error)
BuildResetResponse(status string) (json.RawMessage, error)
BuildUnlockConnectorResponse(status string) (json.RawMessage, error)
BuildChangeConfigurationResponse(status string) (json.RawMessage, error)
BuildGetConfigurationResponse(configKeys []ConfigurationKey, unknownKeys []string) (json.RawMessage, error)
BuildTriggerMessageResponse(status string) (json.RawMessage, error)
BuildChangeAvailabilityResponse(status string) (json.RawMessage, error)
```

Add request/response structs:

```go
type RemoteStartTransactionRequest struct {
    IDTag            string           `json:"idTag"`
    ConnectorID      *int             `json:"connectorId,omitempty"`
    ChargingProfile  json.RawMessage  `json:"chargingProfile,omitempty"`
}

type RemoteStopTransactionRequest struct {
    TransactionID int `json:"transactionId"`
}

type ResetRequest struct {
    Type string `json:"type"` // "Soft" | "Hard"
}

type UnlockConnectorRequest struct {
    ConnectorID int `json:"connectorId"`
}

type ChangeConfigurationRequest struct {
    Key   string `json:"key"`
    Value string `json:"value"`
}

type GetConfigurationRequest struct {
    Key []string `json:"key,omitempty"`
}

type TriggerMessageRequest struct {
    RequestedMessage string `json:"requestedMessage"`
    ConnectorID      *int   `json:"connectorId,omitempty"`
}

type ChangeAvailabilityRequest struct {
    ConnectorID int    `json:"connectorId"`
    Type        string `json:"type"` // "Operative" | "Inoperative"
}

type ConfigurationKey struct {
    Key      string  `json:"key"`
    Readonly bool    `json:"readonly"`
    Value    *string `json:"value,omitempty"`
}
```

### Step 3: v16 Protocol Implementation (apps/api/internal/ocpp/v16/protocol.go)

Implement all 16 new methods on `v16.Protocol`:

- `Parse*Request` methods: unmarshal JSON payload into the typed struct, validate required fields
- `Build*Response` methods: construct spec-exact response payloads

Pattern follows existing `Parse*Response` / `Build*` methods. Each `Build*Response` returns `json.RawMessage` (the CALLRESULT payload).

### Step 4: Inbound CALL Handler (apps/api/internal/simulator/inbound_handler.go)

New file. Contains the dispatch logic for CSMS-initiated CALLs received by the simulator.

```go
// InboundCallHandler handles CSMS-initiated CALL messages.
type InboundCallHandler struct {
    runtime *Runtime
}

func (h *InboundCallHandler) Handle(cpID string, msg ocpp.Message) (json.RawMessage, error) {
    protocol := /* get protocol for cpID */
    switch msg.Action {
    case "RemoteStartTransaction":
        req, err := protocol.ParseRemoteStartTransactionRequest(msg.Payload)
        // ... execute action, build response
    case "RemoteStopTransaction":
        // ...
    case "Reset":
        // ...
    case "UnlockConnector":
        // ...
    case "ChangeConfiguration":
        // ...
    case "GetConfiguration":
        // ...
    case "TriggerMessage":
        // ...
    case "ChangeAvailability":
        // ...
    default:
        // return CALLERROR NotImplemented
    }
}
```

### Step 5: Runtime Methods for CSMS Commands (apps/api/internal/simulator/runtime.go)

Add runtime methods that the inbound handler calls:

```go
func (r *Runtime) HandleRemoteStartTransaction(cpID string, req *ocpp.RemoteStartTransactionRequest) (string, error)
    // Returns status: "Accepted" or "Rejected"
    // If Accepted: connector transitions Available→Preparing, triggers async StartTransaction flow
    // If connectorId omitted, picks first available connector

func (r *Runtime) HandleRemoteStopTransaction(cpID string, req *ocpp.RemoteStopTransactionRequest) (string, error)
    // Returns status: "Accepted" or "Rejected"
    // If Accepted: triggers async StopTransaction flow

func (r *Runtime) HandleReset(cpID string, req *ocpp.ResetRequest) (string, error)
    // Returns status: "Accepted" or "Rejected"
    // Soft: graceful restart; Hard: disconnect + reconnect cycle

func (r *Runtime) HandleUnlockConnector(cpID string, connectorID int) (string, error)
    // Returns: "Unlocked", "UnlockFailed", "NotSupported"
    // If connector has active transaction, transitions through Finishing→Available

func (r *Runtime) HandleChangeConfiguration(cpID string, key, value string) (string, error)
    // Returns: "Accepted", "Rejected", "RebootRequired", "NotSupported"
    // Simulates config storage; "HeartbeatInterval" accepted, unknown keys rejected

func (r *Runtime) HandleGetConfiguration(cpID string, keys []string) ([]ocpp.ConfigurationKey, []string, error)
    // Returns configKeys and unknownKeys

func (r *Runtime) HandleTriggerMessage(cpID string, requestedMessage string, connectorID *int) (string, error)
    // Returns: "Accepted", "Rejected", "NotImplemented"
    // Triggers the requested CP-initiated message as a separate CALL

func (r *Runtime) HandleChangeAvailability(cpID string, connectorID int, availType string) (string, error)
    // Returns: "Accepted", "Rejected", "Scheduled"
    // connectorId=0 means whole station
```

### Step 6: OCPP Client Inbound CALL Handling (apps/api/internal/simulator/ocpp_client.go)

Modify `readLoop` to handle `case ocpp.CALL`:

```go
case ocpp.CALL:
    responsePayload, err := c.inboundHandler.Handle(c.cpID, msg)
    if err != nil {
        // Send CALLERROR
        errMsg, _ := c.codec.BuildError(msg.UniqueID, ocpp.ErrorCodeNotImplemented, err.Error(), nil)
        c.Send(errMsg)
    } else {
        // Send CALLRESULT
        resultMsg, _ := c.codec.BuildResult(msg.UniqueID, responsePayload)
        c.Send(resultMsg)
    }
```

Add `SetInboundHandler(handler *InboundCallHandler)` to `OcppWebSocketClient`.

Wire the handler in `Runtime.Connect()` when creating the client.

### Step 7: ocpp-core — Internal Endpoints + CALL Publisher

Add internal endpoints to `ocpp-core` that send CSMS-initiated CALLs:

```
POST /api/chargepoints/{id}/remote-start     → RemoteStartTransaction CALL
POST /api/chargepoints/{id}/remote-stop       → RemoteStopTransaction CALL
POST /api/chargepoints/{id}/reset             → Reset CALL
POST /api/chargepoints/{id}/unlock            → UnlockConnector CALL
PUT  /api/chargepoints/{id}/config            → ChangeConfiguration CALL
GET  /api/chargepoints/{id}/config            → GetConfiguration CALL
POST /api/chargepoints/{id}/trigger           → TriggerMessage CALL
POST /api/chargepoints/{id}/availability      → ChangeAvailability CALL
```

**Bridge pattern** (per the spec):

1. HTTP handler receives REST request
2. Finds the target CP's live WebSocket connection by chargePointId
3. Generates a `uniqueId`
4. Registers a pending request with timeout (30s)
5. Sends the OCPP CALL `[2, uniqueId, action, payload]` over WS
6. Awaits the CALLRESULT `[3, uniqueId, responsePayload]`
7. Maps OCPP status → HTTP status code
8. Returns HTTP response

Implementation: produce a schema-valid raw OCPP CALL in `ocpp-core` and publish it to `ocpp/{chargePointId}/out`. The gateway forwards it to the existing charge point WebSocket.

**Files in ocpp-core:**
- `internal/csms/service.go` — CSMS→CP CALL producer
- `internal/api/server.go` — internal endpoint registration

### Step 8: Tests

**packages/ocpp-schemas/validator_test.go:**
- Add schema validation tests for all 8 new request/response pairs
- Verify correct payloads pass, malformed ones fail

**apps/api/internal/ocpp/v16/protocol_test.go:**
- Test `Parse*Request` for all 8 actions
- Test `Build*Response` produces schema-valid payloads
- Round-trip: build→encode→decode→parse

**apps/api/internal/simulator/inbound_handler_test.go:**
- Test each inbound action dispatches correctly
- Test unknown action returns CALLERROR NotImplemented
- Test validation errors return appropriate errors

**apps/ocpp-core/internal/api/server_test.go:**
- Test each internal endpoint publishes the expected raw OCPP CALL
- Test invalid payloads are rejected before publish

---

## File Change Summary

### New files
| File | Description |
|------|-------------|
| `packages/ocpp-schemas/v16/RemoteStartTransaction.json` | Schema |
| `packages/ocpp-schemas/v16/RemoteStartTransactionResponse.json` | Schema |
| `packages/ocpp-schemas/v16/RemoteStopTransaction.json` | Schema |
| `packages/ocpp-schemas/v16/RemoteStopTransactionResponse.json` | Schema |
| `packages/ocpp-schemas/v16/Reset.json` | Schema |
| `packages/ocpp-schemas/v16/ResetResponse.json` | Schema |
| `packages/ocpp-schemas/v16/UnlockConnector.json` | Schema |
| `packages/ocpp-schemas/v16/UnlockConnectorResponse.json` | Schema |
| `packages/ocpp-schemas/v16/ChangeConfiguration.json` | Schema |
| `packages/ocpp-schemas/v16/ChangeConfigurationResponse.json` | Schema |
| `packages/ocpp-schemas/v16/GetConfiguration.json` | Schema |
| `packages/ocpp-schemas/v16/GetConfigurationResponse.json` | Schema |
| `packages/ocpp-schemas/v16/TriggerMessage.json` | Schema |
| `packages/ocpp-schemas/v16/TriggerMessageResponse.json` | Schema |
| `packages/ocpp-schemas/v16/ChangeAvailability.json` | Schema |
| `packages/ocpp-schemas/v16/ChangeAvailabilityResponse.json` | Schema |
| `apps/api/internal/simulator/inbound_handler.go` | Inbound CALL dispatch |
| `apps/ocpp-core/internal/csms/service.go` | CSMS-initiated CALL producer |

### Modified files
| File | Changes |
|------|---------|
| `apps/api/internal/ocpp/protocol.go` | Add 16 new methods + request/response structs |
| `apps/api/internal/ocpp/v16/protocol.go` | Implement all 16 new methods |
| `apps/api/internal/simulator/ocpp_client.go` | Add `case ocpp.CALL` in readLoop, add inbound handler |
| `apps/api/internal/simulator/runtime.go` | Add 8 Handle* methods, wire inbound handler |
| `packages/ocpp-schemas/validator_test.go` | Add tests for 8 new schemas |
| `apps/api/internal/ocpp/v16/protocol_test.go` | Add tests for new methods |
| `apps/ocpp-core/internal/api/server_test.go` | Add tests for internal CSMS endpoints |

---

## Dependency Order

```
Step 1 (schemas) ──┐
                    ├─→ Step 3 (v16 impl) ──→ Step 6 (client) ──→ Step 8 (tests)
Step 2 (interface) ─┘         │
                              ├─→ Step 4 (inbound handler)
                              └─→ Step 5 (runtime methods) ──→ Step 4
                                                    
Step 7 (CSMS REST) ── independent, can run in parallel ──→ Step 8 (CSMS tests)
```

Steps 1-2 are foundational. Steps 3-6 build the simulator-side handling. Step 7 builds the CSMS-side REST API. Both converge at testing (Step 8).

---

## Key Design Decisions

1. **`Parse*Request` / `Build*Response` pattern** — mirrors the existing `Build*` / `Parse*Response` pattern for CP-initiated messages, keeping the interface symmetric.

2. **Inbound CALL handling lives in `OcppWebSocketClient.readLoop`** — the client already owns the read loop; adding `case ocpp.CALL` is the minimal change.

3. **`InboundCallHandler` as a separate struct** — avoids bloating `Runtime` with dispatch logic; handler calls runtime methods.

4. **CSMS uses its own PendingCallRegistry** — same pattern as the simulator's, but CSMS-initiated. The CSMS generates `uniqueId` and awaits CALLRESULT.

5. **Fire-and-confirm for RemoteStart/Stop** — REST endpoints return `202 Accepted` once CP returns OCPP `Accepted`. Actual start/stop confirmed via follow-up CP-initiated messages (already handled by existing code).

6. **Simulated config store** — `ChangeConfiguration` / `GetConfiguration` use an in-memory map on `ChargePointInstance`. `HeartbeatInterval` is accepted; unknown keys can be accepted or rejected based on a configurable policy.
