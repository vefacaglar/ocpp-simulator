# Dockerfile for apps/ocpp-core. Owns the canonical OCPP
# message log and the StartTransaction UUID/numeric id counter.
# Depends on ocpp-protocol and ocpp-schemas; those are local
# replace targets, so the build mirrors the repo layout.
#
# Build: docker build -t ocpp-core -f apps/ocpp-core/Dockerfile .

# syntax=docker/dockerfile:1.6

FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY apps/ocpp-core/go.mod apps/ocpp-core/go.sum ./apps/ocpp-core/
RUN mkdir -p packages/ocpp-protocol/pkg/codec packages/ocpp-protocol/pkg/message packages/ocpp-protocol/pkg/pendingcalls packages/ocpp-protocol/pkg/protocol packages/ocpp-protocol/pkg/v16 packages/ocpp-protocol/pkg/v201 packages/ocpp-schemas/v16 packages/ocpp-schemas/v201 \
 && for d in packages/ocpp-protocol/pkg/codec packages/ocpp-protocol/pkg/message packages/ocpp-protocol/pkg/pendingcalls packages/ocpp-protocol/pkg/protocol packages/ocpp-protocol/pkg/v16 packages/ocpp-protocol/pkg/v201 packages/ocpp-schemas/v16 packages/ocpp-schemas/v201; do touch "$d/placeholder.go"; done
COPY packages/ocpp-protocol/ ./packages/ocpp-protocol/
COPY packages/ocpp-schemas/ ./packages/ocpp-schemas/
RUN find packages/ocpp-protocol packages/ocpp-schemas -name placeholder.go -delete

WORKDIR /src/apps/ocpp-core
COPY apps/ocpp-core/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/core ./cmd/core

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/core /app/core
# SQLite WAL files live next to the DB; mount a volume at /data.
WORKDIR /data
USER nonroot
EXPOSE 7090
ENTRYPOINT ["/app/core"]
