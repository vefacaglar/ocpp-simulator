.PHONY: dev dev-api dev-csms dev-web kill-ports

dev: kill-ports
	@echo "Starting all services..."
	@(sleep 3 && printf "\n\033[1;32m✓ All services running\033[0m\n\n  Vue Frontend  → http://localhost:5173\n  Go API        → http://localhost:7070\n  Mock CSMS     → ws://localhost:8080/ocpp/{chargePointId}\n\n") &
	@make -j3 dev-api dev-csms dev-web

dev-api:
	@echo "Starting API on :7070..."
	cd apps/api && go run ./cmd/server

dev-csms:
	@echo "Starting CSMS on :8080..."
	cd apps/csms && go run ./cmd/server

dev-web:
	@echo "Starting Web on :5173..."
	pnpm --filter web dev

kill-ports:
	@echo "Killing processes on ports 7070, 8080, 5173..."
	@-lsof -ti:7070 | xargs kill -9 2>/dev/null || true
	@-lsof -ti:8080 | xargs kill -9 2>/dev/null || true
	@-lsof -ti:5173 | xargs kill -9 2>/dev/null || true
