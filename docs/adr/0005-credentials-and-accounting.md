# ADR 0005: Encrypted Credentials and Idempotent Traffic Outbox

- Status: Accepted
- Date: 2026-08-07

## Context

The controller must generate subscription links, so it needs recoverable proxy
credentials. Native Agents can authenticate with verifiers, but S-UI/sing-box
requires the actual client password when writing its configuration. Reusing one
password across nodes would make one node compromise affect the entire fleet.
Traffic APIs expose counters that reset on process restart, while unreliable
networks cause report retries.

## Decision

- Generate an independent high-entropy Hysteria2 secret centrally for every
  user-node assignment.
- Encrypt recoverable secrets with XChaCha20-Poly1305 and an external versioned
  master key, with credential/user/node/protocol identity as associated data.
- Send only SHA-256 verifiers to native Agents.
- Let an authenticated S-UI Agent retrieve only the plaintext credential
  referenced by its exact current desired version and hash. Mark the response
  `no-store`; never persist the material in snapshots or Agent state.
- Disallow weak user-chosen secrets in v1.
- Read cumulative traffic counters without clearing them.
- Persist local baselines and create immutable delta batches in a transactional
  Agent outbox.
- Insert batches and update controller totals transactionally under a unique
  batch-ID constraint.

## Consequences

- A database backup without the master key does not disclose credentials.
- Losing the master key prevents subscription regeneration and rotation.
- High-entropy-only credentials are a deliberate v1 usability constraint.
- One node compromise exposes at most that node's assignment credentials, while
  one subscription URL can still render every assigned endpoint.
- Desired and applied credential references must be tracked separately so a
  subscription never renders an unacknowledged credential.
- S-UI necessarily retains its node-specific client passwords; its datastore and
  backups are credential-bearing secrets.
- Agent and Hysteria2 restarts require tested epoch/reset handling.
- Exactly-once accounting is achievable despite at-least-once delivery.

## Rejected alternatives

- **Store plaintext secrets:** unacceptable backup and database exposure.
- **Reuse one proxy credential across a user's nodes:** unnecessarily expands the
  impact of a compromised node.
- **Put plaintext in desired snapshots:** persists secrets in controller and Agent
  history and makes routine diagnostics dangerous.
- **Argon2 on every HY2 connection:** stronger for weak passwords but creates an
  avoidable CPU denial-of-service surface on small nodes.
- **Call `/traffic?clear=1` then report:** an Agent crash after clearing but before
  durable reporting can permanently lose usage.
- **Trust a last-seen timestamp only:** does not prevent duplicate traffic totals.
