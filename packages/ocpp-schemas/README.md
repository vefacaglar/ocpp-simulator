# ocpp-schemas

Go module holding the **official OCPP JSON schemas** plus a validator. These schemas are the single source of truth for the strict-compliance rule: code conforms to them, never the other way around (`AGENTS.md` → Hard rule).

Import path: `github.com/user/ocpp-simulator/packages/ocpp-schemas` (package `ocppschemas`).

## What's here

- `v16/*.json` — the OCPP **1.6J** request/response schemas (one `<Action>.json` and one `<Action>Response.json` per action). ~30 files, embedded via `//go:embed`.
- `v201/*.json` — OCPP **2.0.1** schemas, currently a single `BootNotification.json` placeholder; the validator rejects v2.0.1 until the set is complete.
- `validator.go` — `Validate`, version/direction constants, and a compiled-schema cache.

## API

```go
err := ocppschemas.Validate(
    ocppschemas.V16,        // Version: V16 | V201
    "BootNotification",     // action
    ocppschemas.Response,   // Direction: Request | Response
    payloadBytes,           // raw JSON payload
)
```

- Looks up `v16/<action>[Response].json`, compiles it (cached per `version/action/direction`), and validates the payload.
- Returns an error for unknown JSON, schema-validation failures, missing schema files, or unsupported versions (`V201` currently errors).
- Compilation uses `github.com/santhosh-tekuri/jsonschema/v5`.

`SchemaAction(filename)` strips `.json`/`Response` suffixes to recover the action name.

## How it's used

This package is the enforcement point for strict OCPP compliance:

- `message-processor` validates every response payload it produces before publishing.
- `ocpp-core`'s CSMS service validates every CSMS-initiated CALL before publishing.
- Tests build → encode → validate against these schemas for every action, and assert that a deliberately non-spec payload is **rejected**.

## Updating schemas

Replace files in `v16/` (or fill in `v201/`) with the official OCPP JSON schemas verbatim. Do not hand-edit them to match code — the dependency direction is always code → schema.

## Test

```bash
cd packages/ocpp-schemas && go test ./...
```
