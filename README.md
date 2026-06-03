# OCPP Simulator

A local-first, web-based OCPP charge point simulator for developers testing EV charging backends.

## Stack

- **Backend**: Go (API + simulator runtime + mock CSMS)
- **Frontend**: Vue 3 + Vite + TypeScript + Pinia
- **Database**: SQLite
- **Monorepo**: Turborepo + pnpm + Go workspace

## Prerequisites

- **Go 1.21+**
- **Node.js 20+** and **pnpm** (`npm i -g pnpm`)
- **make**
  - macOS: ships with Xcode Command Line Tools
  - Linux: `apt install make` / `dnf install make`
- **lsof** (used by the port-kill step)
  - macOS: preinstalled
  - Linux: `apt install lsof` / `dnf install lsof`

## Quick Start

```bash
pnpm install
make dev
```

`make dev` first frees ports `7070`, `8080`, `5173` (via `make kill-ports`) and then starts all three services in parallel:
- Vue dashboard: http://localhost:5173
- Go API: http://localhost:7070
- Mock CSMS: ws://localhost:8080/ocpp

> **Alternative:** `pnpm dev` runs the same services through Turborepo but does **not** clear the ports beforehand, so a stale process from a previous run can block startup. Prefer `make dev`.

### Individual Services

Each `make` target also kills its own port first, so restarting a single service is safe:

```bash
make dev-api    # Go API only  (port 7070)
make dev-csms   # Mock CSMS    (port 8080)
make dev-web    # Vue only     (port 5173)
```

Plain pnpm equivalents (no port cleanup):

```bash
pnpm dev:api
pnpm dev:csms
pnpm dev:web
```

### Manual Port Cleanup

If a service is running outside the dev scripts and is holding a port, free them by hand:

```bash
make kill-ports
```

### Go Build & Test

```bash
cd apps/api && go test ./...
cd apps/csms && go test ./...
cd packages/ocpp-schemas && go test ./...
```

## Project Structure

```
apps/
  api/              # Go API + simulator runtime
  web/              # Vue 3 dashboard
  csms/             # Mock OCPP Central System
packages/
  ocpp-schemas/     # Official OCPP JSON schemas + validator
  shared/           # Shared TypeScript types
  config/           # Shared frontend config
```

## License

MIT
