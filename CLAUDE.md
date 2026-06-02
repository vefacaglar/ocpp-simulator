# CLAUDE.md

This project's guidance for Claude Code is maintained in a single shared agent guide to avoid drift. Read it in full:

@AGENTS.md

## Quick reminders

- **Hard rule:** strict OCPP standard compliance on the wire — no custom/simplified models, fields, casing, enums, or framing. Internal models stay internal; only the version `codec` touches the wire. Details in `AGENTS.md` and `plan.md` §2b.
- The authoritative architecture/development plan is [`plan.md`](plan.md). Consult it before non-trivial changes.
- Run `go test ./...` (covers `apps/api`, `apps/csms`, `packages/ocpp-schemas`) — schema-validation tests are the proof of compliance and must stay green.
- Work is tracked via the task list; respect `blockedBy` dependencies. The CSMS/schema track can progress in parallel with the simulator track.
