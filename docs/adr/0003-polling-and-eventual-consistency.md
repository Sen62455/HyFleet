# ADR 0003: HTTPS Polling and Eventual Consistency

- Status: Accepted
- Date: 2026-08-07

## Context

Three nodes do not require a message broker or permanently connected WebSocket.
Nodes may sit behind firewalls, change addresses, or lose connectivity. A user
change cannot be made atomically across independent VPS instances.

## Decision

Agents poll desired state over authenticated HTTPS and post heartbeat, metrics,
and durable traffic batches. The controller stores monotonically versioned
desired snapshots, while Agents acknowledge applied versions asynchronously.

Configuration and quota enforcement are explicitly eventually consistent.
Traffic transport is at-least-once; database accounting is exactly-once by batch
ID.

## Consequences

- No broker, WebSocket state, or inbound node port is required.
- Normal propagation may take one polling interval.
- UI must show pending/applied/failed per node rather than claim instant success.
- Jitter/backoff and payload bounds are required to avoid synchronized load.
- A future push channel can be an optimization without changing desired-state
  semantics.

## Rejected alternatives

- **Synchronous fan-out in an admin request:** partial failures are inevitable and
  make the browser request unreliable.
- **Shared database across nodes:** creates unsafe WAN coupling and breaks local
  independence.
- **NATS/Redis streams:** unnecessary infrastructure for the target scale.
