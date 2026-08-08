# Product Requirements

- Status: Accepted for Phase 0
- Target release: v1.0
- Primary deployment: three privately owned Linux VPS nodes

## 1. Problem statement

The initial fleet consists of three Hysteria2 nodes with different management
surfaces:

- DMIT: Hysteria2 managed by S-UI and sing-box.
- LisaHost: Hysteria2 deployed without a confirmed central panel.
- BandwagonHost: Hysteria2 inbound provided by standalone sing-box without a
  panel.

Managing users, credentials, expiry, traffic, subscriptions, service health,
and VPS status separately is error-prone. HyFleet must provide one management
surface while preserving the working proxy cores.

## 2. Product principles

1. **Keep the data plane independent.** A control-plane outage must not stop a
   healthy node from authenticating cached users or carrying traffic.
2. **Be small by default.** Production requires one control-plane binary, one
   embedded SQLite database, and one small Agent per node.
3. **Reconcile desired state.** The controller records intent; Agents converge
   asynchronously and report the applied version.
4. **Do not own the whole machine.** HyFleet observes the VPS and performs only
   allowlisted proxy-service operations.
5. **Preserve unmanaged resources.** The S-UI adapter must never alter a client
   it did not create or explicitly adopt.
6. **Make failure visible.** Pending, degraded, stale, failed, and healthy states
   must be distinct in both API and UI.

## 3. Users and actors

### Administrator

The administrator registers nodes, creates users, assigns users to nodes,
rotates one or more assignment credentials, sets expiry and traffic limits,
reviews status, and obtains audit and diagnostic information.

### Subscription consumer

A subscription consumer accesses an opaque subscription URL and receives only
the nodes assigned to that user. A consumer has no management account in v1.

### Node Agent

The Agent authenticates to the control plane, caches desired state, applies it
through an adapter, performs local Hysteria2 authentication, and reports health,
online state, and traffic.

## 4. Functional requirements

### Node management

- Register a node with a one-time enrollment token.
- Support `native_hysteria2`, `standalone_sing_box`, and `s_ui` adapter types.
- Store a human name, provider, region, tags, public endpoint metadata, and
  non-secret capability information.
- Show last heartbeat, Agent/core version, desired/applied revision, CPU, memory,
  disk, network rate, uptime, and core state.
- Mark a node stale after 45 seconds without a heartbeat and offline after 90
  seconds. Thresholds are configurable.
- Allow only explicit operations such as core status, restart, and log tail.

### User management

- Create, edit, enable, disable, and archive a global user.
- Assign a user to any subset of nodes.
- Set an optional UTC expiry and global traffic quota.
- Generate an independent high-entropy Hysteria2 credential for every user-node
  assignment and rotate one or all assignment credentials deliberately.
- Default migrations to generated credentials. Preserving an existing password
  requires an explicit administrator import into the controller; Agents never
  discover and upload it automatically.
- Preserve stable user IDs when usernames change.
- Display desired and applied state for each user-node assignment.

### Native Hysteria2 integration

- Serve Hysteria2 HTTP authentication on loopback.
- Cache user verifiers locally and load the last valid snapshot after restart.
- Return the global user ID as the Hysteria2 traffic identity.
- Read traffic and online state from the loopback Traffic Stats API.
- Kick a disabled user when supported.

### S-UI integration

- Call the local S-UI `/apiv2` API with a dedicated token; do not assume S-UI
  supports fine-grained token scopes.
- Discover version, clients, inbounds, online users, server status, and logs.
- Start with read-only import and explicit adoption.
- Default adopted clients to credential rotation; preservation requires the same
  explicit secret-import path and a local fingerprint match.
- Create and reconcile only clients owned by HyFleet.
- Keep a local mapping from global user ID to S-UI client ID.
- Retrieve plaintext credential material only for the current node assignment
  while applying it, without persisting it in the Agent state database.
- Parse S-UI responses into bounded typed models and discard returned password
  fields; never forward or log raw API responses.
- Treat unsupported S-UI versions as incompatible, not as an invitation to make
  speculative writes.

### Traffic and quota

- Report upload and download separately per user and node.
- Accept each Agent batch exactly once using a globally unique batch ID.
- Survive Agent retries and controller restarts without double counting.
- Detect local counter resets and create a new source epoch.
- Aggregate node totals into a user total.
- Disable an over-quota user on every assigned node using eventual consistency.
- Show the enforcement delay and last successful usage report.

### Subscription

- Issue opaque, rotatable, revocable per-user subscription tokens.
- Generate Hysteria2 share links without exposing management addresses.
- Initial formats: plain URI list, Base64 URI list, Clash Meta, and sing-box JSON.
- Include only enabled, applied, compatible nodes assigned to the user.
- Hide endpoint-specific credentials behind one user subscription URL.
- Omit an endpoint during credential cutover until its Agent acknowledges the new
  applied credential; never render a credential that is only desired.
- Never log the full token or generated credential.

### Audit and backup

- Audit administrator mutations, Agent enrollment, credential rotation, node
  operations, imports, and failed reconciliation.
- Create consistent SQLite backups using the SQLite backup API, not a raw copy of
  a live database file.
- Document restore and encryption-key recovery before v1.0.

## 5. Quality attributes and acceptance targets

- Online desired-state propagation: normally within 30 seconds.
- Stale-node indication: within 45 seconds by default.
- Agent cached-auth startup: no controller connection required.
- Duplicate traffic report: changes totals exactly once.
- Agent idle RSS target: at most 30 MiB on Linux amd64.
- Control-plane idle RSS target: at most 80 MiB excluding reverse proxy.
- Idle Agent CPU target: below 1% averaged over five minutes.
- Supported initial OS: Debian 12 and Ubuntu 22.04/24.04 with systemd.
- Supported architectures: Linux amd64 and arm64.

These are engineering budgets and release gates, not claims made before
measurement.

## 6. v1 non-goals

- Payments, plans, email campaigns, or reseller features.
- Multi-admin RBAC beyond one administrator account.
- Automatic VPS purchasing, DNS provisioning, or full OS patch management.
- Arbitrary commands, browser terminals, file managers, or SSH key custody.
- HA controllers, clustered databases, or more than 50 nodes.
- Device limits, IP limits, load balancing, or protocols other than Hysteria2.
- Importing secret material from untrusted third-party subscriptions.

## 7. Compatibility policy

- Agent/control protocol is versioned independently from the UI API.
- The controller supports the current and immediately previous Agent protocol
  after v1.0.
- Each adapter publishes a tested compatibility matrix.
- Unknown fields are ignored only when explicitly documented as forward-safe.
- Destructive adapter behavior is disabled on version mismatch.
