# Agent Protocol Draft

- Status: design draft for Phase 1
- Base path: `/agent/v1`
- Encoding: JSON over HTTPS
- Time: RFC 3339 UTC in wire payloads

This is a semantic contract, not a final OpenAPI specification. Phase 1 will add
machine-readable schemas and conformance tests.

## 1. Transport decisions

- Agents initiate every connection; nodes expose no public management port.
- HTTPS certificate verification is mandatory outside explicit local tests.
- Normal operation uses short requests and polling, not WebSocket or a broker.
- Poll interval defaults: desired state 10 seconds, heartbeat 15 seconds, traffic
  30 seconds, with bounded jitter.
- Failure uses exponential backoff with jitter, capped at five minutes, while
  local authentication and outbox processing continue.
- Request and response bodies are size limited and decompression bombs rejected.

## 2. Authentication

Enrollment uses a random, short-lived, single-use token. Successful enrollment
returns a random node credential once. Subsequent requests use:

```http
Authorization: Bearer <node-credential>
X-HyFleet-Protocol: 1
X-HyFleet-Agent: <semantic-version>
X-Request-ID: <uuid>
```

The server stores a credential hash for long-term verification. During
enrollment only, it may retain an encrypted response capsule for a short
idempotent-retry window; the capsule is bound to installation/request IDs and is
erased after the first authenticated request or expiry. Rotation supports a
short overlap window so an acknowledgement cannot strand a node. A node
credential is scoped to exactly one node.

## 3. Common error shape

```json
{
  "error": {
    "code": "desired_version_conflict",
    "message": "desired state changed; poll again",
    "request_id": "019..."
  }
}
```

Messages are safe for logs and contain no credential, token, raw adapter output,
or configuration body. Error codes are stable; messages may change.

## 4. Endpoints

### `POST /agent/v1/enroll`

Request includes enrollment token, installation ID, Agent version, OS,
architecture, and supported adapter capabilities. Response includes node ID,
node credential, protocol version, polling policy, and server time.

Properties:

- Consumes the enrollment token exactly once.
- A retry with the same installation ID and request ID may return the original
  result only from the short encrypted replay capsule.
- A different request or installation cannot reuse a consumed token.
- Does not accept an S-UI token or other adapter secret.

### `POST /agent/v1/heartbeat`

Request fields:

```json
{
  "installation_id": "019...",
  "applied_version": 12,
  "agent": {"version": "0.1.0", "protocol": 1},
  "core": {"name": "hysteria2", "version": "v2.x", "running": true},
  "host": {
    "uptime_seconds": 12345,
    "cpu_percent": 2.5,
    "memory_used_bytes": 123,
    "memory_total_bytes": 456,
    "disk_used_bytes": 789,
    "disk_total_bytes": 1024,
    "network_rx_bps": 0,
    "network_tx_bps": 0
  },
  "sampled_at": "2026-08-07T00:00:00Z"
}
```

Response returns server time, current desired version, credential-rotation hint,
and optionally a poll-backoff override. Host values are validated for finiteness
and plausible bounds.

### `GET /agent/v1/desired?after=<version>`

- Returns `204 No Content` when no newer state exists.
- Returns canonical desired JSON, version, schema version, SHA-256 hash, creation
  time, and optional detached signature when newer state exists.
- Snapshot content is node-scoped. Native snapshots contain credential
  verifiers; S-UI snapshots contain only credential references/fingerprints.
  Neither contains plaintext assignment credentials.
- `ETag` may be used in addition to the explicit version.

### `POST /agent/v1/credential-material`

This endpoint exists only because S-UI/sing-box must receive the actual client
password when applying a managed client. Request:

```json
{
  "credential_ref": "019...",
  "desired_version": 12,
  "snapshot_sha256": "base64url-digest"
}
```

Response:

```json
{
  "credential_ref": "019...",
  "secret": "base64url-high-entropy-secret"
}
```

The response always includes `Cache-Control: no-store`. The server returns
material only when all of these conditions hold:

- The caller is the authenticated Agent for the referenced node.
- The node uses the S-UI adapter.
- The reference is the non-revoked `desired_credential_id` of an assignment for
  that node.
- The exact reference occurs in the node's current desired version and hash.

A stale version, revoked credential, adapter mismatch, or cross-node reference is
denied without revealing which condition failed. The controller decrypts one
credential in memory and never logs the request/response body. The Agent passes
the secret directly to the loopback S-UI API, then discards it without writing it
to its snapshot cache, database, diagnostic output, or operation result. Retrieval
metadata may be audited using only request ID, node ID, credential reference, and
fingerprint.

### `POST /agent/v1/desired/{version}/ack`

Reports `applied` or `failed`, snapshot hash, adapter name/version, duration,
stable error code, and a redacted message. The server refuses an acknowledgement
whose node, version, or hash does not match.

### `POST /agent/v1/traffic-batches`

Request contains one or more bounded batches. Each batch has ID, installation
ID, source epoch, sequence, sample time, and per-user upload/download deltas.

Response identifies every batch as `accepted`, `duplicate`, or `rejected`.
`accepted` and `duplicate` are both safe for the Agent to remove from its outbox.
A single invalid batch does not make valid sibling batches ambiguous.

### `POST /agent/v1/online-snapshot`

Best-effort online user IDs and connection counts. This endpoint is not an
accounting source and may be sampled less frequently on constrained nodes.

### `POST /agent/v1/s-ui-report`

An authenticated `s_ui` Agent reports its compatibility probe and sanitized
Hysteria2 discovery snapshot. The report contains:

- S-UI version, compatibility status, stable error code, and probe time;
- sing-box running state;
- typed Hysteria2 inbound IDs, tags, listen addresses, and ports;
- typed client IDs, names, enabled/expiry state, cumulative upload/download,
  online state, and optional HyFleet mapping metadata.

The Agent obtains discovery clients from the S-UI endpoint that omits `config`
and links. The protocol rejects unexpected group, description, credential
fingerprint, duplicate ID, invalid size, and timestamps more than ten minutes in
the future. Raw S-UI responses, links, configuration objects, API Token, and
client passwords are never accepted by this endpoint.

### `GET /agent/v1/operations?after=<sequence>`

Reserved for Phase 6. Returns only typed operations such as `probe_core`,
`restart_core`, `kick_user`, or `tail_core_log`. There is no `command` string.

### `POST /agent/v1/operations/{id}/result`

Reserved for Phase 6. Idempotently acknowledges typed operation results with a
redacted bounded output.

## 5. Desired-state schema v1

Conceptual payload:

```json
{
  "schema_version": 1,
  "node_id": "019...",
  "version": 12,
  "adapter": "native_hysteria2",
  "users": [
    {
      "id": "019...",
      "username": "alice",
      "credential": {
        "ref": "019...",
        "fingerprint": "fp_7H3K2M",
        "verifier_sha256": "base64url-digest"
      },
      "enabled": true,
      "expires_at": null,
      "quota_state": "active"
    }
  ],
  "generated_at": "2026-08-07T00:00:00Z"
}
```

The example is for `native_hysteria2`. For `s_ui`, `credential` omits
`verifier_sha256` and each user also has `management_mode` and an optional
`remote_client_id`. A `read_only` entry establishes only the local ownership
mapping; it never requests credential material or modifies the remote client. A
`managed` entry uses `ref` only with the credential-material endpoint while
applying that exact revision. The Agent computes effective denial for managed
entries from `enabled`, `expires_at`, and `quota_state`. Local time skew is
reported; a large skew marks the node degraded.

The snapshot always describes the desired credential. The controller keeps the
assignment's desired and applied references separate and does not render the new
credential in subscriptions until this snapshot is acknowledged as applied.

## 6. Idempotency and ordering

- Desired snapshots are immutable and ordered by version.
- Applying the same version and hash again is a no-op.
- Credential-material authorization is bound to the current desired version and
  hash; a conflict requires polling and replanning before another apply.
- Traffic batches use unique IDs and server-side unique constraints.
- Enrollment and mutating admin API requests accept idempotency keys in Phase 1.
- Agent requests may arrive out of order; only explicit sequence/version fields
  establish ordering.

## 7. Version negotiation

- Protocol major mismatch is incompatible.
- Minor additive fields are allowed only when marked optional.
- The server advertises minimum and maximum Agent protocol versions.
- An incompatible Agent continues cached authentication but stops applying new
  desired state and displays a clear degraded status.

## 8. Clock and replay handling

TLS is the primary transport protection. Request IDs, bounded timestamp skew,
monotonic versions, and idempotency keys limit replay effects. Security-sensitive
mutations are never authorized solely by a client-supplied timestamp.
