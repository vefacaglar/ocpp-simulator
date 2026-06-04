# packages/ocpp-protocol

Version-agnostic OCPP protocol code, extracted from the original `apps/api/internal/ocpp` so it can be imported by backend services in the v4.3 split (`ocpp-gateway`, `message-processor`, `ocpp-core`) and any future protocol-aware package.

Public packages:

- `pkg/message` — wire envelope types: `Message`, `MessageTypeID` (CALL=2, CALLRESULT=3, CALLERROR=4), the standard `ErrorCode` set, and `GenerateUniqueID`.
- `pkg/codec` — encodes/decodes raw OCPP-J frames. Version-agnostic; only knows the `[2/3/4, uid, ...]` envelope.
- `pkg/protocol` — `Protocol` interface, all CP-initiated input types, all CSMS-initiated request types, `ConfigurationKey`, and a `Factory` keyed by version string.
- `pkg/pendingcalls` — `PendingCall` and `Registry` for correlating inbound CALLRESULT/CALLERROR back to the originating CALL.
- `pkg/v16` — OCPP 1.6J implementation of `protocol.Protocol` plus `Parse*Response` helpers for the CP-to-server `.conf` payloads.
- `pkg/v201` — OCPP 2.0.1 placeholder. Implements `protocol.Protocol` and returns `ErrNotImplemented` from every Build/Parse entry point. The 2.0.1 wire format (string `transactionId`, `TransactionEvent` flow, device model) and `packages/ocpp-schemas/v201` land in a follow-up. The `Factory` can register this stub today.

Multi-version readiness is satisfied by the version string returned by each `Protocol` and the `Factory` registration pattern: `main()` registers the versions it supports at startup.

The package depends only on `github.com/user/ocpp-simulator/packages/ocpp-schemas` indirectly (test-only). No DB, no MQTT, no HTTP — the only side effects are JSON encoding and time/random for unique IDs.

Run tests:

```bash
cd packages/ocpp-protocol && go test ./...
```
