# Dockerfile for apps/message-processor. CP→server response
# producer with no database. Depends on ocpp-protocol and
# ocpp-schemas; the build mirrors the repo layout.
#
# Build: docker build -t message-processor -f apps/message-processor/Dockerfile .

# syntax=docker/dockerfile:1.6

FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY apps/message-processor/go.mod apps/message-processor/go.sum ./apps/message-processor/
RUN mkdir -p packages/ocpp-protocol/pkg/codec packages/ocpp-protocol/pkg/message packages/ocpp-protocol/pkg/pendingcalls packages/ocpp-protocol/pkg/protocol packages/ocpp-protocol/pkg/v16 packages/ocpp-protocol/pkg/v201 packages/ocpp-schemas/v16 packages/ocpp-schemas/v201 \
 && for d in packages/ocpp-protocol/pkg/codec packages/ocpp-protocol/pkg/message packages/ocpp-protocol/pkg/pendingcalls packages/ocpp-protocol/pkg/protocol packages/ocpp-protocol/pkg/v16 packages/ocpp-protocol/pkg/v201 packages/ocpp-schemas/v16 packages/ocpp-schemas/v201; do touch "$d/placeholder.go"; done
COPY packages/ocpp-protocol/ ./packages/ocpp-protocol/
COPY packages/ocpp-schemas/ ./packages/ocpp-schemas/
RUN find packages/ocpp-protocol packages/ocpp-schemas -name placeholder.go -delete

WORKDIR /src/apps/message-processor
COPY apps/message-processor/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/processor ./cmd/processor

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/processor /app/processor
USER nonroot
EXPOSE 7091
ENTRYPOINT ["/app/processor"]
