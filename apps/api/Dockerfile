# syntax=docker/dockerfile:1.6

FROM golang:1.25-bookworm AS builder
WORKDIR /src

# Prime module cache
COPY go.work go.work.sum ./
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY agents/mc-agent/go.mod ./agents/mc-agent/
RUN --mount=type=cache,target=/go/pkg/mod go work sync

# Copy full workspace and build binary
COPY . .
WORKDIR /src/apps/api
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
ENV PORT=8080
WORKDIR /app
COPY --from=builder /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
