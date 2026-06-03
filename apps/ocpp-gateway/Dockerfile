# Dockerfile for apps/ocpp-gateway. Dumb WebSocket-to-MQTT
# bridge. Built only from this module; the local replace
# directive to packages/ocpp-protocol in apps/ocpp-gateway/go.mod
# (if any) resolves to the in-tree packages directory at build
# time. (As of the v4.3 split, ocpp-gateway does not import the
# ocpp-protocol packages — it only carries paho + gorilla. The
# build still mirrors the repo layout so the go.mod replace
# directive, should it be added later, is honored.)
#
# Build: docker build -t ocpp-gateway -f apps/ocpp-gateway/Dockerfile .

# syntax=docker/dockerfile:1.6

FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY apps/ocpp-gateway/go.mod apps/ocpp-gateway/go.sum ./apps/ocpp-gateway/
# Local replace target must exist for `go mod download`. Create
# a placeholder if go.mod has any replace directives pointing
# inside the repo. (Currently none, but the directory keeps
# future-proofing cheap.)
RUN mkdir -p packages/ocpp-protocol/pkg/codec packages/ocpp-protocol/pkg/message packages/ocpp-protocol/pkg/pendingcalls packages/ocpp-protocol/pkg/protocol packages/ocpp-protocol/pkg/v16 packages/ocpp-protocol/pkg/v201 \
 && for d in packages/ocpp-protocol/pkg/codec packages/ocpp-protocol/pkg/message packages/ocpp-protocol/pkg/pendingcalls packages/ocpp-protocol/pkg/protocol packages/ocpp-protocol/pkg/v16 packages/ocpp-protocol/pkg/v201; do touch "$d/placeholder.go"; done
COPY packages/ocpp-protocol/ ./packages/ocpp-protocol/
RUN rm -f packages/ocpp-protocol/pkg/codec/placeholder.go packages/ocpp-protocol/pkg/message/placeholder.go packages/ocpp-protocol/pkg/pendingcalls/placeholder.go packages/ocpp-protocol/pkg/protocol/placeholder.go packages/ocpp-protocol/pkg/v16/placeholder.go packages/ocpp-protocol/pkg/v201/placeholder.go

WORKDIR /src/apps/ocpp-gateway
COPY apps/ocpp-gateway/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/gateway /app/gateway
USER nonroot
EXPOSE 7080
ENTRYPOINT ["/app/gateway"]
