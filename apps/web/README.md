# web

Vue 3 dashboard and per-charge-point OCPP simulator. The browser is the control surface for simulated charge point behavior; each simulated unit drives its own OCPP session.

Stack: Vue 3 (`<script setup>`), Vite, TypeScript, Pinia, vue-router, browser IndexedDB, native WebSocket.

## Role

- **One WebSocket per simulated charge point** to `ocpp-gateway` at the configured Central System URL (`ws://localhost:7080/ws/{chargePointId}`). This is never a single shared socket for all units.
- **Browser-local state**: charge point/connector config, selected unit, client-side connector state, meter generation, transaction holder state, and UI live logs all live in the browser (IndexedDB / Pinia). There is no UI management API; a browser refresh losing local CP simulation state is accepted for the local simulator.
- Sends CP-initiated actions (plug/unplug, Authorize, StartTransaction, StopTransaction, MeterValues, StatusNotification) as **raw OCPP-J frames** over that unit's own socket.

(`AGENTS.md` → target service split #4.)

## Wire contract

- Frames are sent/received as raw text JSON arrays (`[2,…]`, `[3,…]`, `[4,…]`) — no wrappers.
- The WebSocket subprotocol is the OCPP one (`ocpp1.6` / `ocpp2.0.1`), always sent on connect.
- Pending-CALL correlation and response building live in `src/ocpp/`, not in the dumb socket client.

## Source layout

| Path | Responsibility |
|---|---|
| `src/ocpp/gatewayClient.ts` | Per-CP WebSocket client; raw frame send/receive, no protocol awareness |
| `src/ocpp/callResponses.ts` | Pending-call correlation, response building |
| `src/ocpp/uniqueId.ts` | OCPP unique-id generation |
| `src/stores/chargePointStore.ts` | Charge points, connectors, simulation state (Pinia) |
| `src/stores/realtimeStore.ts` | Live socket/log state |
| `src/stores/settingsStore.ts` | Central System URL and UI settings |
| `src/db/browserDb.ts` | IndexedDB persistence for config + logs |
| `src/components/` | Dashboard UI: CP list/detail, add-CP modal, live log panel, status badge, settings |
| `src/pages/DashboardPage.vue` | Main control-room page |

## Dev

```bash
pnpm dev:web          # vite at http://localhost:5173 (assumes backends are up)
# or from repo root:
make dev              # backends in docker compose + vite with HMR
```

Scripts (`package.json`): `dev` (vite), `build` (`vue-tsc -b && vite build`), `preview`.

> Node 20's corepack ships an 11.x pnpm that crashes here; pin pnpm to 9.15.4 (see `Dockerfile`).

The container build serves the static `dist/` via nginx on port 80, mapped to `5173` in `docker-compose.yml`.
