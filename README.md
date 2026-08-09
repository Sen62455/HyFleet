# HyFleet

HyFleet is a lightweight control plane for a small fleet of Hysteria2 VPS nodes.
It provides centralized node, user, traffic, subscription, and health management
without replacing Hysteria2 or sing-box.

The repository contains the **v1.1.2 release** with native convergence and host
monitoring. Native Hysteria2 and compatible S-UI nodes share one user, traffic,
status, credential, and subscription control plane.
Standalone sing-box supports bounded operations, split-directory backup, and
health monitoring, while its user and subscription adapter remains out of scope.

## Current capabilities

- Single-administrator bootstrap, Argon2id login, server-side sessions, and CSRF
  protection.
- Node create, update, archive, status, and one-time Agent enrollment.
- Outbound-only Agent heartbeat, host metrics, service status, and desired-state
  polling skeleton.
- Responsive Chinese management console embedded in one Go server binary.
- Pure-Go SQLite WAL storage, native systemd units, CI, and Linux amd64/arm64
  release builds.
- Global user CRUD, expiry and enable controls, and native-node assignments.
- Independent encrypted credentials for each user and node assignment.
- Hysteria2 HTTP authentication backed by an atomic Agent-side verifier cache.
- Controlled `/etc/hysteria/config.yaml` migration with backup, service checks,
  automatic failure recovery, and an explicit rollback command.
- Persistent Agent traffic Outbox, idempotent control-plane accounting, online
  snapshots, kick generations, and global/per-node quotas.
- Hashed per-user subscription Tokens with Hysteria2 URI, Base64, Clash Meta,
  and sing-box outputs.
- Optional TLS certificate and public-key pins carried through Hysteria2 URI,
  Mihomo/Clash, and sing-box subscriptions for self-signed endpoints.
- Applied-only subscription eligibility and staged single/all-assignment
  credential rotation.
- S-UI v1.5.x compatibility probing, Hysteria2 inbound discovery, explicit
  read-only import and guarded adoption.
- Agent-local S-UI ownership mapping, managed client reconciliation, online
  state, and durable traffic accounting without sending the S-UI Token to the
  controller.
- Clash Meta output with a default `HyFleet` selector and `MATCH` rule, so rule
  mode routes through the generated proxy group.
- Signed GitHub Release builds and a local parallel updater for a small-node
  fleet with per-component health checks and rollback.
- Durable bounded-operation delivery with Agent-side result Outbox, manual
  retries, restricted core restart/log/backup actions, and restart rollback.
- Active alert reconciliation for offline, core, traffic, synchronization, and
  operation failures, with acknowledgement and automatic recovery.
- Beszel-style fleet overview and per-node CPU, memory, swap, disk I/O, network,
  load, uptime, host, kernel, and core details with bounded 30-day history.
- Dedicated filtered and paginated operation history; node details retain only
  the three most recent results and the operation controls.
- Heartbeat isolation through a small SQLite WAL connection pool and Agent
  operation execution that cannot block its heartbeat scheduler.
- Native Debian/Ubuntu bootstrap, a rootless Server container, consistent SQLite
  backup/restore, SPDX SBOMs, and keyless Sigstore release signatures.

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

## Phase 2 documents

- [Native Hysteria2 users, upgrade, migration, and acceptance](docs/11-phase-2-native-users.md)

## Phase 3 documents

- [Traffic, online state, quotas, and fleet updates](docs/12-phase-3-traffic-and-updates.md)

## Phase 4 documents

- [Unified subscriptions and credential rotation](docs/13-phase-4-unified-subscriptions.md)

## Phase 5 documents

- [S-UI adapter and DMIT onboarding](docs/14-phase-5-sui-adapter.md)

## Phase 6 documents

- [Operations, recovery, backups, and alerts](docs/15-phase-6-operations.md)

## Phase 7 documents

- [Public installation, upgrade, and disaster recovery](docs/16-phase-7-public-release.md)
- [Compatibility matrix](docs/compatibility.md)
- [Security policy](SECURITY.md)
- [Contributing](CONTRIBUTING.md)

## Post-v1 documents

- [Native convergence, host monitoring, and simultaneous-offline recovery](docs/17-native-convergence-and-monitoring.md)

## Install on Debian or Ubuntu

For a public repository, download and inspect the bootstrap from the same tag
you intend to install:

```bash
curl --fail --location --proto '=https' --tlsv1.2 \
  -o install.sh \
  https://raw.githubusercontent.com/Sen62455/HyFleet/v1.1.2/install.sh
less install.sh
sudo bash install.sh server \
  --version v1.1.2 \
  --public-url https://panel.example.com
```

While this repository is private, download the Release from an authenticated
workstation with `gh release download`, then upload the verified bundle to the
VPS. Existing v1.0 fleets should use `scripts/deploy-fleet.ps1`; the
[v1.1 upgrade and native convergence guide](docs/17-native-convergence-and-monitoring.md)
contains the exact private-repository workflow.

The native Server binds to loopback and requires an HTTPS reverse proxy. Agents
use the same bootstrap with the `agent` component after their node and one-time
enrollment Token have been created in the control plane. See the Phase 7 guide
before installing or restoring a production host.

For a containerized control plane, use [docker/compose.yaml](docker/compose.yaml).
Agents remain native systemd services and never receive the Docker socket.

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
  -Architecture amd64 -Version v1.1.2
```

The archive, external checksum, Linux ELF binaries, idempotent installers, and
diagnostic script are written to `output/releases/`. Follow the
[Ubuntu systemd deployment guide](docs/10-systemd-deployment.md); do not upload
a Windows preview executable to a VPS.

## Status

v1.1 is release-ready for native Debian/Ubuntu Server and Agent deployment and a
Docker Server deployment. Host monitoring, dedicated operation history, and the
heartbeat-isolation fixes are included. S-UI read-only imports remain excluded
from quotas and subscriptions until explicitly adopted and successfully applied.
Standalone sing-box can be enrolled for health, restart, limited logs, bounded
file or directory configuration backup, and alerts; its user and subscription
membership remains gated on a future adapter phase.
Secrets, IP addresses, API tokens, private keys, full subscription URLs, and
unredacted configurations must not be committed.
