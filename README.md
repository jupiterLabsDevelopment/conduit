# Conduit · Secure control channel for Minecraft servers

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![Node.js](https://img.shields.io/badge/Node.js-18+-339933?logo=node.js)](https://nodejs.org/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/license-MIT-blue)](#license)

## Description

Conduit is a self-hosted control plane for Minecraft servers. It bridges the Minecraft Management API to a secure REST/WebSocket API, provides a modern admin UI, and records every action for audit. Owners and moderators can operate servers remotely without exposing management secrets or sacrificing visibility.

**Built for** server admins who need a lightweight but secure way to manage remote Minecraft instances. **Key features include:**

- Go API with RBAC (owner/moderator/viewer) and full audit logging
- Agent bridge that relays JSON-RPC calls and streams events with resilient reconnects
- React admin console with live telemetry, rule management (including bulk presets), and exportable audit history
- TypeScript SDK for automation or custom dashboards
- Docker Compose deployment plus standalone binaries for advanced setups

## Table of Contents

1. [Getting Started / Installation](#getting-started--installation)
2. [Usage](#usage)
3. [Configuration & Operations](#configuration--operations)
4. [Architecture / How It Works](#architecture--how-it-works)
5. [Contributing](#contributing)
6. [Roadmap / Planned Features](#roadmap--planned-features)
7. [FAQ / Troubleshooting](#faq--troubleshooting)
8. [Community / Support](#community--support)
9. [Credits / Acknowledgments](#credits--acknowledgments)
10. [License](#license)

## Getting Started / Installation

### Prerequisites

- Go 1.22+
- Node.js 18+ and npm
- Docker & Docker Compose (optional but recommended for local development)
- Access to a Minecraft server with the Management API enabled and a bearer token configured

### Quick start with Docker Compose

```bash
cp deploy/.env.example deploy/.env
# Update secrets (JWT, agent tokens, management token) inside deploy/.env
cd deploy
docker compose up --build
```

Services exposed locally:

- UI: <http://localhost:5173>
- API: <http://localhost:8080>
- Postgres: `postgres://conduit:conduit@localhost:5432/conduit`
- Agent: runs alongside Minecraft to bridge JSON-RPC

Shut down with `docker compose down`.

### Manual builds

```bash
# API
cd apps/api && go build ./cmd/api

# Agent
cd agents/mc-agent && go build .

# UI
cd apps/ui && npm install && npm run build

# TypeScript SDK
cd packages/sdk-ts && npm install && npm run build
```

Run the bootstrap script `deploy/migrations/init_db.sql` against a fresh Postgres instance (for example: `psql -f deploy/migrations/init_db.sql postgres://conduit:conduit@localhost:5432/postgres`).

## Usage

1. Review the [operator guide](docs/OPERATORS.md) for Minecraft server configuration and token setup.
2. Bootstrap the first owner via the UI or by calling:

   ```bash
   curl -X POST http://localhost:8080/v1/users/bootstrap \
     -H "Content-Type: application/json" \
     -d '{"email":"owner@example.com","password":"super-secret"}'
   ```

3. Log in at <http://localhost:5173>, create a server entry, and copy the generated agent token.
4. Start the agent close to the Minecraft process:

   ```bash
   export CONDUIT_API_WS="wss://conduit.local/agent/connect"
   export CONDUIT_AGENT_TOKEN="<from UI>"
   export MC_MGMT_WS="wss://127.0.0.1:24464"
   export MC_MGMT_TOKEN="<minecraft management token>"
   ./mc-agent
   ```

5. Apply a bulk game-rule preset from the UI (**Game Rules → Bulk presets**). The UI previews planned changes and surfaces per-field success/failure after the agent executes the batch.

6. Export an audit trail from the Audit tab or via the SDK:

   ```ts
   import { createClient } from "@conduit/sdk";

   const client = createClient({ apiBase: "https://conduit.local" });
   client.setToken(process.env.CONDUIT_TOKEN!);

   const csv = await client.exportAuditLogs(serverId, { limit: 500 });
   console.log(csv);
   ```

## Configuration & Operations

The full catalogue of environment variables, operational runbooks, and telemetry guidance lives in [`docs/OPERATORS.md`](docs/OPERATORS.md). Refer to that guide for:

- API and UI environment variables and deployment tips
- Agent TLS/backoff/telemetry settings, including how to ship JSON telemetry logs into centralized monitoring
- Security considerations, upgrade notes, and troubleshooting flows

> Tip: load secrets via your orchestration platform (Kubernetes, Nomad, systemd) rather than exporting plain-text files.

## Architecture / How It Works

```text
[ Browser UI ] ⇄ HTTPS ⇄ [ Conduit API ] ⇄ WS ⇄ [ Agent ] ⇄ WS ⇄ [ Minecraft Management API ]
                                            │
                                            └── Postgres (users, sessions, audit, schema)
```

- The **UI** authenticates with JWTs, calls REST endpoints, and subscribes to server events via WebSocket.
- The **API** enforces RBAC, maintains an agent hub, relays JSON-RPC calls, and records audit entries.
- The **Agent** maintains dual WebSocket connections (API + Minecraft), forwards requests/responses, and gathers telemetry.
- **Postgres** persists users, servers, session hashes, audit logs, and cached Minecraft schema (`rpc.discover`).

## Contributing

We welcome issues and pull requests:

1. Fork the repo and create a feature branch.
2. Run `npm run lint` in `apps/ui`, `go test ./...` in Go modules, and `npm run build` in `packages/sdk-ts` before submitting.
3. Provide context in the PR description (feature scope, manual test notes).

No formal CLA is required for MVP contributions.

## Roadmap / Planned Features

- Expanded automated test coverage (Go unit, React component, integration smoke tests)
- Hardened production tooling: TLS certificate pinning options, richer agent telemetry exports
- Documentation automation and versioned release notes
- Future (`conduit-cloud`): multi-tenant SaaS, SSO/SCIM, hosted agent relays

## FAQ / Troubleshooting

**Agent reports TLS errors with self-signed certs.**
Set `MC_TLS_MODE=skip` temporarily or provide a PEM bundle via `MC_TLS_ROOT_CA`.

**UI shows “Agent not connected.”**
Double-check `CONDUIT_AGENT_TOKEN`, outbound firewall rules, and that the agent can reach the API WebSocket.

**Bulk preset application failed for some rules.**
Hover the status pill in the results table—the error message is mirrored in the audit log for diagnosis.

More scenarios are covered in the [Troubleshooting section](docs/OPERATORS.md#8-troubleshooting).

## Community / Support

- Open an issue in this repository for bugs or feature requests
- Share anonymised logs (API or agent) when requesting help
- Follow updates in the `docs/CHANGELOG.md`

## Credits / Acknowledgments

- Maintained by the Conduit team @ Jupiter Labs
- Inspired by the official Minecraft Management API and community tooling
- Built with Go, React, Vite, Tailwind CSS, and PostgreSQL

## License

This project is licensed under the [MIT License](LICENSE).
