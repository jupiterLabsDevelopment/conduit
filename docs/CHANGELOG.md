# Changelog

## [Unreleased]

### Added

- Workspace wiring for API, agent, UI, and TypeScript SDK under Go 1.25 toolchain.
- React UI authentication, server list/detail flows, event stream, and SDK-backed RPC actions.
- Audit logging pipeline and JSON-RPC relay between API hub and agent.
- Server detail tabs for players, game rules, settings, and audit log with refreshed SDK data fetches.
- API key CRUD endpoints with owner UI for issuance and revocation, including one-time secret reveal.
- Session lifecycle hardening with hashed tokens, logout revocation endpoint, and CSV audit export with UI download controls.
- Bulk game-rule presets with API, SDK, and UI support, including per-field validation feedback.
- Agent telemetry snapshots plus configurable reconnect backoff and TLS controls exposed via new operator documentation.
- Operator guide updates covering security considerations, upgrade notes, and deployment guidance for the hardened agent.

### Changed

- Adopted shared `ConduitClient` in the UI and generated ESM/CJS bundles for `@conduit/sdk`.
- Added ESLint configuration to `apps/ui` to keep lint scripts passing.
- Agent now defaults to strict TLS verification; `MC_TLS_MODE` supersedes the legacy `MC_TLS_INSECURE` flag.
- Reconnect loop uses jittered exponential backoff with guardrails configurable via environment.

### Pending

- Expanded automated test coverage (Go + React).
