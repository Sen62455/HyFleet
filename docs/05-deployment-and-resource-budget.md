# Deployment and Resource Budget

## 1. Production topology

The controller may share a VPS with a proxy node, but it remains a separate
unprivileged process. A controller outage must not stop node data planes. Initial
placement will be selected only after the inventory is complete.

### Control plane

- One `hyfleet-server` binary with embedded frontend assets.
- SQLite WAL database on persistent local storage.
- HTTPS through an existing reverse proxy or optional Caddy sidecar.
- No production Node.js runtime; Node is needed only to build the UI.
- No Redis, broker, PostgreSQL, or container requirement.

### Node

- One `hyfleet-agent` binary managed by systemd.
- Outbound HTTPS access to the controller.
- Loopback access to Hysteria2 Traffic Stats or S-UI `/apiv2`.
- No inbound Agent firewall rule.

## 2. Filesystem layout

### Server

```text
/usr/local/bin/hyfleet-server
/etc/hyfleet/server.yaml               root:hyfleet-server 0640, no user secrets
/etc/hyfleet/master.key                root:hyfleet-server 0640, separate backup
/var/lib/hyfleet/server.db             SQLite database
/var/lib/hyfleet/backups/              encrypted/controlled backups
```

### Agent

```text
/usr/local/bin/hyfleet-agent
/etc/hyfleet/agent.yaml                root:hyfleet-agent 0640, settings
/etc/hyfleet/agent.env                 root:hyfleet-agent 0640, S-UI Token only after enrollment
/var/lib/hyfleet/agent.credentials     hyfleet-agent 0600, node/native tokens
/var/lib/hyfleet/agent.db              snapshots, mappings, baselines, outbox
```

The `/etc/hyfleet` directory is root-controlled. `/var/lib/hyfleet` is accessible
only to the relevant service account so credentials can be atomically rotated.
The Agent database never contains plaintext user-node assignment credentials.

## 3. Native Hysteria2 handoff

The exact paths and service name are inventory-dependent. The intended server
configuration shape is:

```yaml
auth:
  type: http
  http:
    url: http://127.0.0.1:19090/v1/hysteria2/auth/LOCAL_RANDOM_PATH_TOKEN

trafficStats:
  listen: 127.0.0.1:19091
  secret: LOCAL_RANDOM_STATS_SECRET
```

Both secrets are generated on the node and remain in the Agent credentials file
and Hysteria2 configuration. The controller never receives them. Existing client
credentials can be retained only through an explicit administrator import into
the controller; the Agent never reads and uploads them. Weak or shared imported
credentials must be rotated before v1 production use.

The Agent queries `GET /traffic` without clearing counters and calculates durable
deltas. It uses `/online` and `/kick` only when supported by the installed core.

## 4. S-UI handoff

The Agent configuration holds a loopback base URL and token, for example:

```yaml
adapter_type: s_ui
s_ui_api_url: http://127.0.0.1:2095/app/apiv2
s_ui_token_env: HYFLEET_SUI_TOKEN
```

Actual port and base path may differ. The persistent environment file is root-owned,
group-readable only by `hyfleet-agent`, and excluded from backups unless the
backup is explicitly encrypted. Use a dedicated token and treat it as
full-control unless the tested S-UI version proves narrower authorization. The
adapter first calls read-only endpoints and validates the S-UI compatibility
version before writes.

When applying a managed client, the Agent fetches only the current credential
bound to the authenticated node and desired revision. It holds the value in
memory only and sends it over loopback to S-UI. S-UI/sing-box necessarily stores
that node-specific client password, so S-UI data files and their backups must be
treated as credential-bearing secrets.

## 5. Resource budgets

| Resource | Agent target | Server target |
| --- | ---: | ---: |
| Idle RSS | <= 30 MiB | <= 80 MiB |
| Idle CPU, 5-minute average | < 1% | < 1% for three nodes |
| Installed binary/assets | <= 35 MiB | <= 50 MiB |
| Routine write rate | <= 1 Agent batch / 30 s | <= 1 heartbeat / node / 15 s; metrics / node / 60 s |
| Minimum practical RAM | 128 MiB for Agent alone | 256 MiB |
| Recommended RAM with data plane | 256 MiB+ | 512 MiB if colocated |

These are release targets to benchmark, not guaranteed values. Hysteria2,
sing-box, S-UI, the OS, and reverse proxy require additional memory.

## 6. Sampling and retention

- Heartbeat: 15 seconds with jitter; current state kept in the node row.
- Host metrics: sampled every 15 seconds, persisted as one-minute aggregates.
- Traffic: sampled every 30 seconds, durable outbox until acknowledged.
- Online state: 30 seconds by default and not used for accounting.
- One-minute metrics: retain seven days.
- One-hour metric rollups: retain 90 days.
- Traffic totals: retain for the user lifecycle; detailed deltas use a separately
  documented retention policy before v1.0.
- Audit logs: retain at least 180 days by default with configurable pruning.

Retention jobs delete in bounded batches to avoid long SQLite write locks.

## 7. SQLite operation

- Enable WAL, foreign keys, and a measured busy timeout.
- Keep transactions short; never hold a transaction across a network call.
- Run one logical writer queue only if benchmarks show contention.
- Use migrations with a schema version and pre-upgrade backup.
- Use the SQLite backup API or `VACUUM INTO` for consistent backups.
- Periodically test `PRAGMA integrity_check` against a restored copy, not the live
  busy database during peak time.

## 8. Service supervision

Systemd units use restart-on-failure, bounded restart delay, a dedicated user,
private temporary directories, `NoNewPrivileges`, and filesystem protections
compatible with required paths. The Agent must be ordered after networking but
must not block Hysteria2 boot indefinitely.

Native Hysteria2 depends on Agent HTTP authentication for new sessions. The Agent
therefore loads cached users before declaring itself ready and receives a strict
systemd restart policy. Health checks distinguish Agent health from core health.

## 9. Updates and rollback

- GitHub releases provide amd64/arm64 artifacts, SHA-256 checksums, and eventually
  signatures/SBOM.
- No unattended binary self-update in the MVP.
- Upgrade takes a local database backup and retains the previous binary.
- Agent protocol compatibility is checked before controller upgrade.
- Database migrations document downgrade limitations; binary rollback is not
  claimed safe across irreversible migrations.

## 10. Backup scope

Server backups include SQLite, non-secret configuration, and a manifest with
version/checksum. The master key is backed up separately. Agent state is mostly
reconstructable, but its outbox and S-UI mapping should be included in encrypted
node backup because losing them can lose unreported traffic or ownership data.

HyFleet backup archives never include TLS keys, S-UI tokens, Agent credentials,
or raw assignment credentials in plaintext. If an operator separately includes
the S-UI datastore, that archive must be encrypted because S-UI retains its
node-specific client passwords. A restore drill is a v1.0 gate.
