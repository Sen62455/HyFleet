# Domain and Data Model

This document defines the logical model. SQL details and indexes will be frozen
in Phase 1 after query prototypes. Controller IDs are UUIDv7 strings; timestamps
are UTC Unix milliseconds; byte counts are unsigned values validated before
storage in SQLite signed integers.

## 1. Aggregate boundaries

### User aggregate

A user is a stable global identity. Username, expiry, quota, and enabled state do
not determine identity. Each assigned endpoint has an independent Hysteria2
credential; the subscription service hides that distinction behind one user
subscription URL.

### Node aggregate

A node represents one Agent installation and one adapter instance. Public proxy
endpoint metadata belongs to the node; local adapter secrets do not. Reinstalling
an Agent creates a new installation identity but may be explicitly reattached to
the existing node by an administrator.

### Assignment aggregate

An assignment is the intent to make one user available on one node. It has its
own credential and desired/applied state because a global user can be healthy on
one node and pending or failed on another.

### Usage aggregate

Traffic batches are immutable ingestion records. User/node totals are derived
transactionally and can be rebuilt from retained deltas if the retention policy
allows it.

## 2. Controller tables

### `admins`

| Column | Meaning |
| --- | --- |
| `id` | Stable UUIDv7 |
| `username` | Case-normalized unique login name |
| `password_hash` | Versioned Argon2id encoding |
| `disabled_at` | Optional UTC timestamp |
| `created_at`, `updated_at` | Audit timestamps |

### `admin_sessions`

Stores only a hash of the random session token, expiry, creation IP prefix,
last-seen time, and revocation time. Browser cookies contain the opaque token.

### `nodes`

| Column | Meaning |
| --- | --- |
| `id` | Stable UUIDv7 |
| `name` | Unique display name |
| `provider`, `region` | Non-secret inventory labels |
| `adapter_type` | `native_hysteria2`, `standalone_sing_box`, or `s_ui` |
| `enabled` | Whether reconciliation/subscription use is allowed |
| `public_host`, `public_port` | Proxy endpoint, never Agent management endpoint |
| `sni`, `tls_insecure` | Client-facing TLS metadata |
| `tls_cert_fingerprint` | Optional SHA-256 certificate pin for HY2 URI and Mihomo |
| `tls_public_key_sha256` | Optional Base64 SHA-256 public-key pin for sing-box |
| `obfs_type`, `obfs_password_enc` | Optional client configuration; secret encrypted |
| `tags_json` | Small validated tag object |
| `desired_version`, `applied_version` | Monotonic node revisions |
| `agent_installation_id` | Current Agent installation identity |
| `agent_credential_hash` | Verifier for Agent bearer credential |
| `agent_version`, `protocol_version` | Last reported versions |
| `core_name`, `core_version` | Last reported adapter facts |
| `status`, `status_reason` | `pending`, `online`, `stale`, `offline`, `degraded`, `disabled` |
| `last_seen_at`, `last_applied_at` | Operational timestamps |
| `created_at`, `updated_at` | Audit timestamps |

Full IP addresses may be operationally necessary in `public_host`, but inventory
exports intended for GitHub must redact them.

### `node_enrollment_tokens`

Stores node ID, token hash, expiry, consumed time, creation administrator,
installation/request binding, and a failed-attempt counter. Plain tokens are
shown once. A successfully consumed token may temporarily hold the encrypted
enrollment response for idempotent replay to the same installation and request.
That replay capsule expires quickly and is erased after the first request
authenticated with the issued node credential.

### `users`

| Column | Meaning |
| --- | --- |
| `id` | Stable UUIDv7 returned as native HY2 auth identity |
| `username` | Unique display/login label; mutable |
| `display_name`, `notes` | Optional administrator metadata |
| `enabled` | Global administrative state |
| `expires_at` | Optional UTC expiry |
| `traffic_limit_bytes` | Optional global upload + download limit |
| `traffic_used_bytes` | Cached aggregate updated transactionally |
| `quota_state` | `unlimited`, `active`, `limited` |
| `archived_at` | Soft deletion marker |
| `created_at`, `updated_at` | Audit timestamps |

### `user_credentials`

| Column | Meaning |
| --- | --- |
| `id`, `user_id`, `node_id` | Stable IDs and credential scope |
| `protocol` | `hysteria2` in v1 |
| `secret_ciphertext` | XChaCha20-Poly1305 ciphertext including nonce |
| `verifier_sha256` | Full digest used only by the native auth adapter |
| `secret_fingerprint` | Non-reversible short fingerprint for diagnostics |
| `key_version` | External encryption-key version |
| `state` | `staged`, `applied`, `retired`, or `revoked` |
| `created_at`, `applied_at`, `retired_at`, `revoked_at` | Lifecycle timestamps |

Only generated high-entropy credentials are supported in v1. Ciphertext is bound
to credential, user, node, and protocol identifiers as authenticated associated
data. A native Agent receives only a SHA-256 verifier. An S-UI Agent can retrieve
plaintext only for the non-revoked desired credential in its exact current
snapshot, because sing-box requires it when configuring the client. Raw material
is never stored in canonical snapshots or the Agent database.

### `node_user_assignments`

| Column | Meaning |
| --- | --- |
| `id`, `node_id`, `user_id` | Stable IDs and unique node/user pair |
| `desired_credential_id` | Credential referenced by the current desired revision |
| `applied_credential_id` | Last Agent-acknowledged credential, nullable before first apply |
| `enabled` | Per-node override |
| `desired_version`, `applied_version` | Assignment revisions |
| `state` | `pending`, `applied`, `failed`, `removing` |
| `remote_ref` | Optional opaque S-UI reference; never used as global identity |
| `last_error_code`, `last_error_message` | Redacted apply diagnostic |
| `last_attempt_at`, `applied_at` | Reconciliation timestamps |

### `node_snapshots`

Stores node ID, version, canonical JSON, SHA-256 hash, creation time, and optional
superseded time. Snapshots include only data needed by that node: global user ID,
username, credential reference/fingerprint, effective enabled state, expiry,
quota state, and adapter metadata. Native snapshots additionally contain the
SHA-256 verifier; S-UI snapshots never contain plaintext credential material.
Retain the current and a bounded number of previous snapshots.

### `traffic_batches`

| Column | Meaning |
| --- | --- |
| `id` | Agent-generated UUID; globally unique idempotency key |
| `node_id`, `agent_installation_id` | Authenticated source |
| `source_epoch` | Changes when the local source counter resets |
| `sequence` | Monotonic within Agent installation |
| `sampled_at`, `received_at` | Source and server timestamps |
| `item_count`, `upload_bytes`, `download_bytes` | Batch checks |

`id` is unique. The server validates totals and inserts the batch, item rows, and
aggregate changes in one transaction.

### `traffic_batch_items`

Stores batch ID, node ID, user ID, upload delta, and download delta. Unknown or
unassigned users are quarantined rather than silently attributed.

### `traffic_totals`

Stores one row per node/user with upload, download, last batch, and updated time.
Global totals remain cached on `users` for efficient listing and are checked by a
periodic invariant job.

### `node_metric_samples`

One-minute samples: CPU percentage, memory used/total, disk used/total, network
upload/download rate, load averages, core state, and sample time. One-hour
rollups are stored separately before raw samples expire.

### `subscription_tokens`

Stores user ID, SHA-256 token hash, short display prefix, created/last-used/
expires/revoked timestamps, and optional allowed formats. Plain tokens are shown
only at creation or rotation.

### `audit_logs`

Append-only records containing actor type/ID, action, target type/ID, request ID,
success, redacted metadata JSON, IP prefix, and timestamp. No secret fields are
accepted by the audit serializer.

### `operations`

Reserved for Phase 6. Stores allowlisted operation type, node, desired version,
status, request/finish timestamps, actor, and redacted result. It never stores a
free-form shell command.

## 3. Agent-local tables

The Agent uses a private SQLite database or equivalent transactional store under
`/var/lib/hyfleet`.

| Table | Purpose |
| --- | --- |
| `agent_meta` | Node ID, installation ID, protocol version, desired/applied version |
| `desired_snapshots` | Last valid and limited previous signed snapshots |
| `auth_users` | Native-only user ID, username, verifier, effective state, expiry/quota snapshot |
| `adapter_mappings` | Global user ID, credential fingerprint, and adapter-owned remote reference |
| `usage_baselines` | Source epoch and last cumulative upload/download per user |
| `traffic_outbox` | Immutable unacknowledged traffic batches and retry metadata |
| `operation_results` | Unacknowledged allowlisted operation results |

The long-lived Agent credential remains in a service-owned `0600` state file so
it can be rotated atomically. The S-UI token remains in a root-controlled file
readable only by the Agent service account. Neither is stored in ordinary
database rows. Assignment plaintext is never stored in the Agent database. S-UI
itself necessarily retains the password for its node-specific managed client in
its protected datastore.

## 4. Required invariants

1. One current desired and at most one applied Hysteria2 credential exist per
   user/node/protocol tuple; they may differ only during a controlled cutover.
2. One assignment per node/user pair.
3. Every desired/applied credential reference matches the assignment's user,
   node, and protocol. A desired credential is `staged` or `applied`; an applied
   reference points to the currently `applied` credential.
4. Applied version never exceeds desired version.
5. Node snapshot versions increase monotonically.
6. A traffic batch ID affects totals at most once.
7. Upload and download deltas are non-negative and bounded by a configurable
   sanity limit per interval.
8. An Agent can report or retrieve credential material only for its authenticated
   node ID.
9. A subscription contains only assignments currently eligible and fully
   applied, and renders each endpoint with `applied_credential_id`, never a
   merely desired credential.
10. S-UI adapter deletion is allowed only when a local ownership mapping and the
   expected managed marker both match.
11. Archived users cannot receive new assignments or active subscription tokens.

## 5. Deletion policy

Nodes and users are disabled/archived first. Hard deletion is an explicit
maintenance operation after dependent history retention. Traffic and audit data
must not cascade-delete accidentally. Assignment credentials are revoked when
the assignment is removed or its user/node is archived. Agent tokens are revoked
when their node is archived.
