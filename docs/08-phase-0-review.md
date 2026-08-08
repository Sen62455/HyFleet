# Phase 0 Review

- Review date: 2026-08-07
- Status: Accepted for local Phase 1; production adapter details pending

## Deliverables

- [x] Product problem, users, functional requirements, non-goals, and budgets.
- [x] Control-plane/Agent architecture and failure behavior.
- [x] Accepted ADRs for topology, stack, transport, adapters, credentials, and
  traffic accounting.
- [x] Controller and Agent-local logical data model with invariants.
- [x] Versioned Agent protocol draft and idempotency rules.
- [x] Security threat model, privilege boundary, and release gates.
- [x] Low-resource deployment, retention, backup, and rollback constraints.
- [x] Known three-node inventory and a secret-safe collection template.
- [x] Phase 1 through v1.0 delivery and exit gates.
- [x] Owner-confirmed platform/capacity/service inventory for all three VPS
  instances.
- [x] Controller placement selected: native systemd on DMIT.
- [x] Migration policy confirmed: credentials may rotate with notice.
- [ ] BandwagonHost sing-box and DMIT local S-UI/core contract details confirmed.

## Accepted decisions

1. Go server and Agent; Vue static UI embedded in the server.
2. SQLite WAL by default; no Redis, broker, or PostgreSQL for v1.
3. Agent-initiated HTTPS polling and eventual desired-state reconciliation.
4. Native Hysteria2, standalone sing-box, and S-UI adapters run locally in the
   Agent.
5. Native Hysteria2 uses local HTTP auth and cumulative Traffic Stats reads.
6. S-UI adoption is explicit; unmanaged clients are never reconciled.
7. Every user-node assignment has an independent high-entropy credential.
   Native nodes receive a digest; S-UI nodes retrieve only their current
   assignment material during apply. Desired/applied references prevent a
   subscription from rendering an unacknowledged credential.
8. Traffic uses a durable Agent outbox and idempotent controller batches.
9. Machine management is observation plus typed proxy operations, not remote
   shell access.
10. Apache-2.0 is the planned project license; no S-UI GPL source is copied.

## Risks requiring implementation validation

| Risk | Validation |
| --- | --- |
| Native Agent auth outage blocks new HY2 sessions | Cached startup and systemd failure/restart tests |
| S-UI API changes or insufficient write semantics | Compatibility probe and real-container contract tests |
| Low-resource targets are optimistic | Automated RSS/CPU benchmark on smallest node class |
| Global quota briefly exceeds limit | Measure/report enforcement lag and document semantics |
| Existing credentials are weak/shared | Inventory fingerprint and planned rotation, never commit secret |
| Secret-preserving migration becomes an exfiltration path | Explicit admin import only; Agent never uploads discovered passwords/configs |
| S-UI requires plaintext client passwords | Node/version-bound retrieval, no-store response, Agent non-persistence tests |
| SQLite write contention | Three-node load test before adding any queue/database service |

## Gate decision

Repository design work is accepted for local Phase 1 scaffolding. Production
installation or proxy configuration migration remains blocked until the final
sing-box/S-UI contract details are collected and the public S-UI management
surface is protected. Phase 1 proceeds without secrets or production access.
