# syntax=docker/dockerfile:1.6

FROM cgr.dev/chainguard/go:1.25 AS build
WORKDIR /src
COPY go.work ./
COPY agents/mc-agent/go.mod ./agents/mc-agent/
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY . .
WORKDIR /src/agents/mc-agent
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/conduit-agent .

FROM gcr.io/distroless/base-nonroot:latest
ENV CONDUIT_API_WS="ws://api:8080/agent/connect"
WORKDIR /app
COPY --from=build /out/conduit-agent /usr/local/bin/conduit-agent
USER nonroot
ENTRYPOINT ["/usr/local/bin/conduit-agent"]
