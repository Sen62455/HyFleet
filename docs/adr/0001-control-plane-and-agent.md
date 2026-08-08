# ADR 0001: Separate Control Plane and Outbound Agent

- Status: Accepted
- Date: 2026-08-07

## Context

The fleet contains both raw Hysteria2 servers and an S-UI-managed server. Direct
SSH orchestration would require high privilege and make management dependent on
network reachability. Installing a full panel on every VPS duplicates databases
and still does not create a source of truth.

## Decision

Use one central control plane and a small Agent on every managed VPS. Agents make
outbound HTTPS requests, cache desired users, interact with the local data plane,
and report state. Nodes expose no public HyFleet management port.

## Consequences

- Controller outage does not erase cached authentication state.
- Raw and panel-managed nodes share one transport and reporting model.
- Agent availability becomes important for native HY2 HTTP authentication and
  requires strict supervision.
- Installation adds one binary and local state file to every node.
- Arbitrary SSH execution is excluded; operations must be typed and allowlisted.

## Rejected alternatives

- **Install S-UI everywhere:** easy initially but duplicates panels, increases
  resource use, and couples the project to S-UI internals.
- **Controller calls public S-UI/SSH endpoints:** expands attack surface and does
  not cover native nodes cleanly.
- **Central remote Hysteria2 auth endpoint:** new connections fail whenever the
  controller or cross-region network fails.
