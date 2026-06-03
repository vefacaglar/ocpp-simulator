# Dockerfile for apps/csms. Optional mock OCPP Central System
# used for end-to-end testing against a single WebSocket peer.
# Started via the `csms` profile:
#   docker compose --profile csms up csms
#
# Built only from this module; no local replace directives.

# syntax=docker/dockerfile:1.6

FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY apps/csms/go.mod apps/csms/go.sum ./apps/csms/
WORKDIR /src/apps/csms
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY apps/csms/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/server /app/server
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
