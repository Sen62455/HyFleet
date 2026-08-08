# HyFleet

HyFleet is a lightweight control plane for a small fleet of Hysteria2 VPS nodes.
It provides centralized node, user, subscription, traffic, and health management
without replacing Hysteria2 or sing-box.

The working name is provisional. The repository now contains the local
**Phase 1: foundation** implementation. The control plane and Agent are strictly
read-only toward proxy cores in this phase; no production proxy configuration
should be modified from this tree yet.

## Phase 1 capabilities

- Single-administrator bootstrap, Argon2id login, server-side sessions, and CSRF
  protection.
- Node create, update, archive, status, and one-time Agent enrollment.
- Outbound-only Agent heartbeat, host metrics, service status, and desired-state
  polling skeleton.
- Responsive Chinese management console embedded in one Go server binary.
- Pure-Go SQLite WAL storage, native systemd units, CI, and Linux amd64/arm64
  release builds.
- Local tests proving enrollment replay, Agent restart, auth rate limiting, and
  the Phase 1 read-only boundary.

## Initial scope

- Native Hysteria2 nodes using local HTTP authentication and traffic APIs.
- Standalone sing-box nodes using a separately tested configuration adapter.
- Existing S-UI nodes using the token-authenticated `/apiv2` API.
- One control-plane deployment managing Linux `amd64` and `arm64` agents.
- Global users assignable to one or more nodes with isolated per-node
  credentials.
- Unified subscriptions, traffic totals, online state, and basic host health.
- Reliable operation on low-resource VPS instances.

## Explicit non-goals for v1

- Reimplementing Hysteria2, sing-box, or S-UI.
- Billing, payments, sales plans, or a customer marketplace.
- Arbitrary remote shell access or a general-purpose RMM platform.
- Kubernetes, Redis, a message broker, or mandatory PostgreSQL.
- Multi-tenant administration or protocols other than Hysteria2.

## Phase 0 documents

- [Product requirements](docs/00-product-requirements.md)
- [System architecture](docs/01-system-architecture.md)
- [Domain and data model](docs/02-domain-and-data-model.md)
- [Agent protocol](docs/03-agent-protocol.md)
- [Security threat model](docs/04-security-threat-model.md)
- [Deployment and resource budget](docs/05-deployment-and-resource-budget.md)
- [VPS inventory](docs/06-vps-inventory.md)
- [Development stages](docs/07-development-stages.md)
- [Phase 0 review](docs/08-phase-0-review.md)
- [Architecture decision records](docs/adr/README.md)

## Phase 1 documents

- [Foundation implementation and acceptance](docs/09-phase-1-foundation.md)
- [Native systemd deployment](docs/10-systemd-deployment.md)

## Local development

Required tools are Go 1.26, Node.js 22, and pnpm 11.16.

```bash
cd web
pnpm install --frozen-lockfile
pnpm build
cd ..
go test ./...
go run -tags webui ./cmd/server -config /path/to/server.yaml
```

Set `HYFLEET_BOOTSTRAP_TOKEN` before the first server start. Example files are in
[`configs/`](configs/); deployment units are in
[`deploy/systemd/`](deploy/systemd/).

## Build a Linux deployment bundle on Windows

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1 `
  -Architecture amd64 -Version v0.1.0-dev
```

The archive, external checksum, Linux ELF binaries, idempotent installers, and
diagnostic script are written to `output/releases/`. Follow the
[Ubuntu systemd deployment guide](docs/10-systemd-deployment.md); do not upload
a Windows preview executable to a VPS.

## Status

Phase 1 is implemented and locally testable without production access. Its exit
gate still requires a read-only test Agent to report real host metrics before a
`v0.1.0` tag. Secrets, IP addresses, API tokens, private keys, full subscription
URLs, and unredacted configurations must not be committed.
