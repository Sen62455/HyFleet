# Phase 1 Foundation

## Status

- Implementation date: 2026-08-07
- Target version: `v0.1.0`
- Local implementation: complete
- Hosted CI: workflow defined; first GitHub run pending repository publication
- Production mutation: prohibited
- Remaining production validation: read-only Agent observation on a designated
  Linux test node

## Delivered surface

The control plane is a single Go process with an embedded Vue management UI and
a SQLite WAL database. The Agent is a second static Go binary. It initiates all
network communication and exposes no listener.

Phase 1 includes:

- first-run administrator bootstrap through an environment-only token;
- Argon2id password storage and constant-time password verification;
- server-side sessions, strict cookies, same-origin checks, and CSRF tokens;
- per-IP login and enrollment rate limits;
- node creation, editing, disabling, archiving, and adapter identity;
- single-use, ten-minute Agent enrollment tokens;
- encrypted five-minute enrollment response replay for lost HTTP responses;
- hashed long-lived Agent credentials;
- heartbeat and one-minute host metric samples;
- online, stale, offline, degraded, disabled, and pending status presentation;
- immutable empty desired snapshots and acknowledgement handling;
- atomic Agent state persistence and restart recovery;
- Linux host, network, disk, memory, CPU, load, uptime, and systemd service probes;
- responsive setup, login, node list, detail, edit, archive, and enrollment flows;
- systemd hardening, example configuration, CI, and amd64/arm64 release builds.

## Read-only boundary

The Phase 1 Agent does not read proxy configuration files, call S-UI, change
users, restart a core, or write any core-owned path. Its only core interaction is
the fixed command below, with a validated unit name:

```text
systemctl is-active --quiet <configured-unit>
```

An Agent receiving a desired snapshot containing any user record rejects it and
acknowledges `foundation_read_only`. This prevents an accidental future server
change from turning a Phase 1 binary into a configuration writer.

## HTTP surface

| Method | Path | Authentication | Purpose |
| --- | --- | --- | --- |
| `GET` | `/healthz` | none | Database health |
| `GET` | `/api/v1/setup/status` | none | First-run state |
| `POST` | `/api/v1/setup/bootstrap` | bootstrap token | Create the only administrator |
| `POST` | `/api/v1/auth/login` | password | Create a session |
| `GET` | `/api/v1/auth/session` | session | Restore browser state |
| `POST` | `/api/v1/auth/logout` | session and CSRF | Revoke the session |
| `GET/POST` | `/api/v1/nodes` | session and CSRF for writes | List and create nodes |
| `GET/PUT/DELETE` | `/api/v1/nodes/{id}` | session and CSRF for writes | Manage one node |
| `POST` | `/api/v1/nodes/{id}/enrollment-token` | session and CSRF | Issue one token |
| `POST` | `/agent/v1/enroll` | enrollment token | Bind an Agent installation |
| `POST` | `/agent/v1/heartbeat` | Agent bearer token | Report current state |
| `GET` | `/agent/v1/desired` | Agent bearer token | Poll desired state |
| `POST` | `/agent/v1/desired/{version}/ack` | Agent bearer token | Report apply result |

Every API response is non-cacheable. Static hashed assets use immutable caching;
the SPA document does not.

## Storage invariants

- SQLite uses WAL, normal synchronization, foreign keys, and a five-second busy
  timeout.
- One database connection keeps every connection-scoped PRAGMA authoritative and
  bounds memory on the selected small fleet.
- The `0002` migration replaces the original global node-name constraint with a
  case-insensitive partial unique index for non-archived nodes. Parent-table
  rebuilds run transactionally, execute `foreign_key_check`, and restore foreign
  key enforcement before startup continues.
- The database permits only one administrator row.
- Node names are unique case-insensitively among non-archived records.
- A registered node cannot change adapter type. Recreate the node to change its
  adapter identity.
- Only one unconsumed enrollment token exists per node.
- An authenticated heartbeat erases the encrypted enrollment replay capsule.
- Agent state is mode `0600` on Linux and replaced atomically.

## Test coverage

The Go suite covers password hashing, unsafe hash rejection, strict configuration
parsing, administrator bootstrap, session cookies, CSRF, node CRUD, adapter
identity locking, login rate limiting, enrollment replay and conflicts,
archived-name reuse from an existing `0001` database without losing child rows,
heartbeat metrics, desired polling and acknowledgement, state replacement, Agent
restart, and the read-only desired-state refusal.

The frontend suite covers metric formatting and is additionally checked with
`vue-tsc`, a production Vite build, and browser interaction at desktop and mobile
viewports.

## Visual process note

The frontend design workflow normally begins with an image-generated full-screen
concept. No image-generation capability was exposed in this development session,
so Phase 1 uses a documented code-native design baseline: a neutral operational
workspace, table-first desktop layout, mobile node list, 6 px geometry, and
green, amber, and red semantic states. Browser screenshots remain the visual
acceptance source.

## Known Phase 1 limits

- There is one administrator and no proxy-user model yet.
- Host samples are stored, but the UI shows only the latest values.
- Native Hysteria2, standalone sing-box, and S-UI adapters are observation-only.
- S-UI API compatibility, user reconciliation, traffic, subscriptions, quotas,
  and online sessions remain in later phases.
- Automated database backup and restore tooling remains in Phase 6.

## Local acceptance evidence

The final Windows functional preview used the same persisted Agent credential
through two Agent restarts and three control-plane restarts. After settling, the
observed working set was approximately 14.1 MiB for the Agent and 15.8 MiB for
the server. Unstripped cross-compiled Linux binaries were 9.0-9.7 MiB for the
Agent and 15.3-16.2 MiB for the server across arm64 and amd64.

These figures are useful smoke-test evidence, but Windows working-set values do
not satisfy the Linux RSS gate. Unsupported non-Linux collectors intentionally
leave unavailable host totals at zero instead of reporting process memory as
host memory.

## Exit checklist

- [x] All Go unit and integration tests pass.
- [ ] The Go race detector passes in Linux CI; the local Windows host has no CGO
  compiler.
- [x] Frontend type checking, unit tests, and production build pass.
- [x] Linux amd64 and arm64 server and Agent binaries cross-compile.
- [x] Markdown, YAML, and local repository-source secret pattern checks pass;
  full Gitleaks scanning is configured in CI.
- [x] Desktop and mobile browser flows pass without console errors or overlap.
- [ ] A designated non-production Agent remains online through a restart.
- [ ] Reported metrics are compared with the same test host's local tools.
- [ ] Measured server and Agent RSS remain inside the Phase 1 budget.

The final three checks require an owner-designated test installation. They do not
authorize a deployment to DMIT, LisaHost, or BandwagonHost.
