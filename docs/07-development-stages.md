# Development Stages

Each stage is independently reviewable and ends with tests and an explicit exit
gate. A stage does not mutate production nodes until its failure and rollback
behavior has been tested locally or on the designated test node.

## Phase 0: Design and inventory (`v0.0`)

Deliver product boundary, ADRs, architecture, threat model, domain model, Agent
protocol draft, deployment budget, inventory, and phase gates.

Exit gate: Phase 0 review accepted and the live non-secret inventory confirmed.

## Phase 1: Foundation (`v0.1`)

Deliver Go module/repository layout, embedded migrations, admin bootstrap/login,
session/CSRF baseline, node CRUD, one-time Agent enrollment, heartbeat, desired
poll skeleton, host metrics, structured logging, CI, and release builds.

Tests: API/store unit tests, enrollment replay, auth rate limit, Agent restart,
protocol schema tests, measured resource baseline.

Exit gate: one non-production/test Agent reliably appears online/offline with
correct host metrics; no proxy configuration is changed.

## Phase 2: Native Hysteria2 users (`v0.2`)

Deliver global user CRUD, independent generated encrypted credentials per
assignment, native desired snapshots with verifiers, Agent cache, native HTTP
auth, expiry/enable enforcement, adapter health, and controlled migration tooling
for LisaHost.

Tests: valid/invalid/expired/disabled auth, controller outage, Agent restart,
snapshot replay/rollback, constant-time verifier path, cross-node credential
isolation, redaction.

Exit gate: LisaHost supports a test/global user without per-user core restarts;
the existing client remains usable when preservation is requested; rollback to
the original auth config is documented and tested.

## Phase 3: Traffic and online state (`v0.3`)

Deliver native Traffic Stats collection, durable baselines/outbox, source epochs,
idempotent controller ingestion, node/global totals, online list, kick, expiry and
global quota evaluation, metric charts, and retention jobs.

Tests: duplicate/out-of-order batch, Agent crash around outbox commit, core reset,
controller restart, unknown user quarantine, quota propagation delay.

Exit gate: injected duplicate/restart scenarios preserve exact totals and a
limited user is denied on every online assigned test node within the documented
consistency window.

## Phase 4: Unified subscriptions (`v0.4`)

Deliver subscription token lifecycle, endpoint eligibility, URI/Base64/Clash
Meta/sing-box renderers, caching headers without secret leakage,
endpoint-specific credentials, and selected/all-assignment credential rotation
workflow for applied native Hysteria2 assignments.

Tests: format fixtures, escaping, revoked token, disabled/pending node exclusion,
desired-versus-applied credential cutover, per-endpoint password/endpoint
rotation, subscription log redaction.

Exit gate: a working native Hysteria2 assignment renders in all four formats,
desired credentials remain withheld until applied, and rotating/revoking a Token
behaves predictably. Standalone sing-box membership remains gated on its Adapter.

## Phase 5: S-UI adapter and DMIT onboarding (`v0.5`)

Deliver S-UI compatibility probe, read-only discovery/import, explicit client
adoption, local ownership mapping, managed create/update/disable/delete, status,
online and traffic integration, node/version-bound credential-material retrieval,
and DMIT onboarding.

Tests: real supported S-UI container/version, incompatible version, API outage,
manual unmanaged clients, ID/name changes, repeated reconciliation, ownership
deletion guard, stale/cross-node credential reference denial, no-store headers,
secret non-persistence/redaction, and sentinel secrets in discovery responses.

Exit gate: the same global test user is applied to all three nodes, traffic and
status are visible, and every field of every unmanaged S-UI client remains
unchanged.

## Phase 6: Bounded operations and recovery (`v0.6`)

Deliver offline catch-up, retry controls, typed restart/probe/kick/log operations,
configuration backup metadata, notifications, database backup/restore, and
documented node recovery.

Tests: partitions, stale operations, repeated restart request, bounded log output,
backup consistency, restore drill, Agent credential rotation.

Exit gate: an offline node catches up safely, a failed apply preserves prior
state, and a clean controller can be restored from the documented artifacts.

## Phase 7: Public release (`v1.0`)

Deliver native and Docker installation, systemd hardening, upgrade/rollback
documentation, amd64/arm64 releases, checksums/signatures/SBOM, compatibility
matrix, security policy, contribution guide, screenshots, and clean-host E2E.

Exit gate: a new supported VPS can install from the release documentation; CI and
security gates pass; no real fleet secret or identifying inventory is in Git.

## Post-v1 candidates

Multi-admin RBAC/TOTP, Telegram/webhook alerts, approved Ansible jobs, PostgreSQL,
more sing-box protocols, device limits, and larger-fleet push optimization. Each
requires a new ADR and measured need.
