# syntax=docker/dockerfile:1.6

FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.work go.work.sum ./
COPY agents/mc-agent/go.mod agents/mc-agent/go.sum ./agents/mc-agent/
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
RUN --mount=type=cache,target=/go/pkg/mod go work sync || true

COPY . .
WORKDIR /src/agents/mc-agent
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/agent ./

FROM gcr.io/distroless/static-debian12:nonroot
USER nonroot:nonroot
COPY --from=builder /out/agent /agent
ENTRYPOINT ["/agent"]
