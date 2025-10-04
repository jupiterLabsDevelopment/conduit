# syntax=docker/dockerfile:1.6

FROM cgr.dev/chainguard/go:1.25 AS build
WORKDIR /src
COPY go.work ./
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY agents/mc-agent/go.mod ./agents/mc-agent/
COPY . .
WORKDIR /src/apps/api
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/conduit-api ./cmd/api

FROM gcr.io/distroless/base-nonroot:latest
ENV PORT=8080
WORKDIR /app
COPY --from=build /out/conduit-api /usr/local/bin/conduit-api
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/conduit-api"]
