# Dockerfile for apps/simulator-api. The service is a CRUD +
# realtime HTTP API: no MQTT, no OCPP frame parsing, no
# transactions. Built only from this module's go.mod, which has
# no local replace directives in the v4.3 split (simulator-api
# does not depend on packages/ocpp-protocol anymore).
#
# Build: docker build -t ocpp-simulator-api -f apps/simulator-api/Dockerfile .

# syntax=docker/dockerfile:1.6

FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY apps/simulator-api/go.mod apps/simulator-api/go.sum ./apps/simulator-api/
WORKDIR /src/apps/simulator-api
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY apps/simulator-api/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/server /app/server
# SQLite WAL files live next to the DB; mount a volume at /data.
WORKDIR /data
USER nonroot
EXPOSE 7070
ENTRYPOINT ["/app/server"]
