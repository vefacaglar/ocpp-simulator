# CLAUDE.md

This project's guidance for Claude Code is maintained in a single shared agent guide to avoid drift. Read it in full:

@AGENTS.md

## Quick reminders

- **Hard rule:** strict OCPP standard compliance on the wire — no custom/simplified models, fields, casing, enums, or framing. Internal models stay internal; only the version `codec` touches the wire. Details in `AGENTS.md` and `plan.md` §2b.
- OCPP unit ↔ CSMS traffic must stay raw OCPP-J JSON arrays (`[2,...]`, `[3,...]`, `[4,...]`). Never wrap these messages in object envelopes or internal DTOs on the wire.
- This is an OCPP simulator, not a generic web app. The target model keeps CP simulation state in Vue per charge point, with one WebSocket per CP to `ocpp-gateway`; browser refresh losing local CP simulation state is accepted for the local simulator.
- Target split is browser-local web config/logs, `ocpp-gateway` as a dumb WebSocket edge, `message-processor` as CP-to-server response producer with stdout audit only, and `ocpp-core` for canonical raw logs, transactions, business decisions, and CSMS-initiated CALL publishing.
- `ocpp-core` owns StartTransaction transaction UUID/numeric ID creation and initializes its numeric counter from DB `MAX(numeric_id)` on startup.
- Local target should use Docker Compose for MQTT plus web and the four backend services.
- The authoritative architecture/development plan is [`plan.md`](plan.md). Consult it before non-trivial changes.
- Run Go tests per module directory; do not assume a single root `go test ./...` works. Target modules are `apps/ocpp-gateway`, `apps/message-processor`, `apps/ocpp-core`, `apps/csms`, `packages/ocpp-protocol`, and `packages/ocpp-schemas`.
- Work is tracked via the task list; respect `blockedBy` dependencies. The CSMS/schema track can progress in parallel with the simulator track.
