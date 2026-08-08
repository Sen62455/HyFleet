# ADR 0004: Agent-Side HY2 Data-Plane Adapters

- Status: Accepted
- Date: 2026-08-07

## Context

One initial node runs official Hysteria2, one runs standalone sing-box, and one
uses S-UI with sing-box. S-UI tokens and local traffic/configuration interfaces
should not be published on the internet or stored centrally when they can remain
local.

## Decision

Adapters run inside the Agent process:

- `native_hysteria2` serves loopback HTTP authentication and reads loopback
  traffic/online APIs.
- `standalone_sing_box` begins read-only and may reconcile only an explicitly
  owned fragment or adopted inbound after version-specific contract tests.
- `s_ui` calls the local token-authenticated `/apiv2` API and stores ownership
  mappings locally.

The controller exchanges a common desired-user model with every Agent, with
adapter-specific credential fields. S-UI client credential material uses a
separate node/version-bound endpoint; the controller does not model S-UI's
internal database.

## Consequences

- Local adapter access secrets such as the S-UI API token never leave the node.
- One Agent transport covers both node types.
- Compatibility logic ships with Agent releases and must be reported to the
  controller.
- An Agent update may be required when an S-UI API changes.
- Adapter ownership and adoption need strong tests.

## Rejected alternatives

- **Controller-side S-UI adapter:** requires exposing or tunneling S-UI and stores
  powerful local tokens centrally.
- **Normalize all nodes to S-UI immediately:** introduces a risky migration before
  the project's control semantics are tested.
- **Implement a new proxy core:** outside scope and substantially increases
  security and protocol risk.
