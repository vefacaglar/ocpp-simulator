.PHONY: dev csms simulator dev-stack dev-web down down-v kill-ports test wire-dump help
# dev-backend:* targets intentionally use a colon in their
# name; make's .PHONY list rejects colons, so they are not
# listed here — they are treated as phony automatically because
# their recipes are run unconditionally (no produced file).

# Vite dev-server port; the only port the Makefile owns. Backend
# ports are managed by docker compose (see docker-compose.yml).
VITE_PORT := 5173

# dev is the local-first entry point. It brings up the backend
# OCPP stack (postgres, mqtt, ocpp-core, message-processor,
# ocpp-gateway) in containers and then runs the Vue UI
# outside the stack with hot-reload. The web container is
# intentionally excluded so the UI sees source edits without
# a rebuild.
#
# Resulting port map (all bound to localhost):
#   postgres:5432, mqtt:1883, ocpp-gateway:7080,
#   ocpp-core:7090, message-processor:7091, web:5173 (vite)
dev: kill-ports
	@echo "Cleaning up any lingering backend containers..."
	@docker compose down mqtt ocpp-core message-processor ocpp-gateway 2>/dev/null || true
	@echo "Building and starting backend stack via docker compose..."
	@docker compose up -d --build postgres mqtt ocpp-core message-processor ocpp-gateway
	@echo ""
	@echo "Backend ports:"
	@echo "  postgres:5432, mqtt:1883, ocpp-gateway:7080,"
	@echo "  ocpp-core:7090, message-processor:7091"
	@echo ""
	@echo "Waiting for backend health..."
	@docker compose up -d --wait ocpp-gateway ocpp-core message-processor
	@echo ""
	@echo "Backends up. Starting Vue dev server with hot-reload..."
	@echo "  Web → http://localhost:$(VITE_PORT)"
	@exec pnpm --filter web dev

csms: dev-stack

simulator: dev-web

# dev-stack brings up only the backend services (no UI). Use
# this if you want to develop the UI separately or hit the
# backends directly with curl.
dev-stack:
	@echo "Building and starting backend stack via docker compose..."
	@docker compose up -d --build --wait postgres mqtt ocpp-core message-processor ocpp-gateway
	@echo ""
	@docker compose ps

# dev-web runs only the Vue dev server. Use this when the
# backends are already up (e.g. via dev-stack or in a separate
# shell) and you only want hot-reload on the UI.
dev-web: kill-ports
	@echo "Starting Vue dev server on :$(VITE_PORT)..."
	@exec pnpm --filter web dev

# dev-backend:<svc> runs a single Go service locally with
# `go run` against its own module directory. These bypass
# docker compose and are useful for tight inner-loop work on
# a single service. The env vars match the compose topology
# (mqtt + ocpp-core) so the service can talk to its peers.
# Use `make dev-backend:ocpp-gateway` etc. The colon in the
# target name is escaped from make's target:prereq parsing
# with a backslash so the rule works on a single colon.
dev-backend\:ocpp-gateway:
	@echo "Starting ocpp-gateway on :7080..."
	@cd apps/ocpp-gateway && MQTT_BROKER_URL=tcp://localhost:1883 MQTT_CLIENT_ID=ocpp-gateway-local GATEWAY_ADDR=:7080 go run ./cmd/gateway

dev-backend\:ocpp-core:
	@echo "Starting ocpp-core on :7090..."
	@cd apps/ocpp-core && DATABASE_URL=postgres://postgres:postgres@localhost:5432/ocpp_core?sslmode=disable MQTT_BROKER_URL=tcp://localhost:1883 MQTT_CLIENT_ID=ocpp-core-local HTTP_ADDR=:7090 go run ./cmd/core

dev-backend\:message-processor:
	@echo "Starting message-processor on :7091..."
	@cd apps/message-processor && MQTT_BROKER_URL=tcp://localhost:1883 MQTT_CLIENT_ID=message-processor-local OCPP_CORE_URL=http://localhost:7090 HEALTH_ADDR=:7091 go run ./cmd/processor

dev-backend\:csms:
	@echo "Starting csms on :8080..."
	@cd apps/csms && go run ./cmd/server

down:
	@echo "Stopping backend stack (containers kept, use 'make down-v' to remove)..."
	@docker compose stop

down-v:
	@echo "Stopping backend stack and removing containers + volumes (DBs)..."
	@docker compose down -v

# kill-ports frees ports owned by the Makefile. The Vue dev
# server is the only port we manage directly; backend ports
# are compose's responsibility.
kill-ports:
	@echo "Killing processes on port $(VITE_PORT) (vite)..."
	@-lsof -ti:$(VITE_PORT) | xargs kill -9 2>/dev/null || true

# test runs each Go module's tests from its own directory,
# with GOWORK=off so the workspace is not consulted. This
# mirrors what CI and contributors run locally.
test:
	@set -e; for d in apps/csms apps/ocpp-gateway apps/ocpp-core apps/message-processor packages/ocpp-protocol packages/ocpp-schemas; do \
		echo "=== $$d ==="; \
		(cd "$$d" && GOWORK=off go test ./...); \
	done

# wire-dump runs a fake charge point against the local stack
# and captures the raw OCPP-J frames that traverse MQTT. The
# output is written to docs/wire-dumps/wire-dump-<timestamp>.log.
# Requires a running backend stack (make dev or make dev-stack).
wire-dump:
	@./scripts/wire-dump.sh

help:
	@echo "v4.3 dev entry points:"
	@echo "  make dev                 - bring up backends in compose, then vite with HMR"
	@echo "  make csms                - backend OCPP stack only (compose)"
	@echo "  make simulator           - Vue simulator only (vite)"
	@echo "  make dev-stack           - backends only (no UI), waits for health"
	@echo "  make dev-web             - vite only (assumes backends are up)"
	@echo "  make dev-backend:<svc>   - one Go service via go run, bypasses compose"
	@echo "                             (ocpp-gateway, ocpp-core, message-processor, csms)"
	@echo "  make down                - stop containers (keeps them, fast restart)"
	@echo "  make down-v              - stop + remove containers & volumes (DBs)"
	@echo "  make kill-ports          - free vite port ($(VITE_PORT))"
	@echo "  make test                - run all 6 Go module tests with GOWORK=off"
	@echo "  make wire-dump           - run a fake CP and capture MQTT frames"
	@echo "  make help                - this message"
