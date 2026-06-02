# OCPP Simulator

A local-first, web-based OCPP charge point simulator for developers testing EV charging backends.

## Stack

- **Backend**: Go (API + simulator runtime + mock CSMS)
- **Frontend**: Vue 3 + Vite + TypeScript + Pinia
- **Database**: SQLite
- **Monorepo**: Turborepo + pnpm + Go workspace

## Quick Start

```bash
pnpm install
pnpm dev
```

This starts all services via Turborepo:
- Vue dashboard: http://localhost:5173
- Go API: http://localhost:7070
- Mock CSMS: ws://localhost:8080/ocpp

### Individual Services

```bash
pnpm dev:api    # Go API only
pnpm dev:web    # Vue only
pnpm dev:csms   # Mock Central System only
```

### Go Build & Test

```bash
go build ./...   # build all Go modules
go test ./...    # run all Go tests
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
