# CLAUDE.md

This project's guidance for Claude Code is maintained in a single shared agent guide to avoid drift. Read it in full:

@AGENTS.md

## Quick reminders

- **Hard rule:** strict OCPP standard compliance on the wire — no custom/simplified models, fields, casing, enums, or framing. Internal models stay internal; only the version `codec` touches the wire. Details in `AGENTS.md` and `plan.md` §2b.
- OCPP unit ↔ CSMS traffic must stay raw OCPP-J JSON arrays (`[2,...]`, `[3,...]`, `[4,...]`). Never wrap these messages in object envelopes or internal DTOs on the wire.
- This is an OCPP simulator, not a generic web app. CP-initiated UI actions drive the selected simulated unit's own OCPP WebSocket session (`/api/ws/{chargePointId}` proxied to `centralSystemUrl/{chargePointId}`), while `/api/realtime` is observation-only.
- CSMS-initiated flows such as `RemoteStartTransaction` enter through the mock CSMS API and are sent over the target charge point's existing OCPP WebSocket.
- Target split is `simulator-api` for UI management/config, `ocpp-gateway` for WebSocket edge, `message-processor` for MQTT routing/response generation, and `ocpp-core` for DB/business. First `ocpp-core` responsibility is message logging; later it owns business decisions.
- Local target should use Docker Compose for MQTT plus web and the four backend services.
- The authoritative architecture/development plan is [`plan.md`](plan.md). Consult it before non-trivial changes.
- Run Go tests per workspace module (`apps/api`, `apps/csms`, `packages/ocpp-schemas`) — schema-validation tests are the proof of compliance and must stay green.
- Work is tracked via the task list; respect `blockedBy` dependencies. The CSMS/schema track can progress in parallel with the simulator track.
