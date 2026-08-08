# Phase 2 Native Hysteria2 Users

## Status

- Implementation date: 2026-08-08
- Target version: `v0.2.0`
- Local implementation and automated tests: complete
- LisaHost production migration: pending owner-run acceptance
- Supported write adapter: `native_hysteria2` only
- Unchanged adapters: standalone sing-box and S-UI remain read-only

Phase 2 adds global users without making Hysteria2 restart for each user change.
The only Hysteria2 restart is the controlled one-time migration from static
password authentication to the Agent's loopback HTTP endpoint.

## Runtime flow

```text
HyFleet Server on DMIT
  -> encrypted per-assignment credential in SQLite
  -> desired snapshot containing only SHA-256 verifier
  -> outbound Agent poll on LisaHost
  -> atomic /var/lib/hyfleet-agent/auth-cache.json
  -> 127.0.0.1:18081/hysteria/auth
  -> Hysteria2 accepts or rejects the client
```

The Agent loads the cache before contacting DMIT and never expires it merely
because the control plane is offline. User enable state, assignment enable state,
expiry, and the reserved quota state are evaluated against LisaHost's local UTC
clock for every authentication request.

## Delivered behavior

- Create, list, edit, disable, expire, and archive global users.
- Assign a user to one or more native Hysteria2 nodes.
- Enable or disable one node assignment without disabling the global user.
- Generate a different 32-byte random credential for every user-node pair.
- Reveal a credential only through an authenticated POST request with CSRF and
  `Cache-Control: no-store`.
- Encrypt credential plaintext in SQLite with XChaCha20-Poly1305 and the existing
  control-plane master key.
- Send only a SHA-256 verifier, fingerprint, state, and expiry to the Agent.
- Persist the Agent verifier cache atomically with mode `0600`.
- Reject stale snapshots, same-version hash conflicts, duplicate users, duplicate
  verifiers, invalid IDs, and malformed cache documents.
- Keep authenticating from the last applied cache across DMIT outage and Agent
  restart.
- Display user state, expiry, assigned nodes, desired/applied versions, and apply
  failures in the management console.

## API surface

| Method | Path | Purpose |
| --- | --- | --- |
| `GET/POST` | `/api/v1/users` | List or create users |
| `GET/PUT/DELETE` | `/api/v1/users/{userID}` | Read, edit, or archive one user |
| `POST` | `/api/v1/users/{userID}/assignments` | Assign one native node |
| `PUT/DELETE` | `/api/v1/users/{userID}/assignments/{nodeID}` | Toggle or remove an assignment |
| `POST` | `/api/v1/users/{userID}/assignments/{nodeID}/credential` | Explicitly reveal one credential |
| `POST` | `http://127.0.0.1:18081/hysteria/auth` | Local Hysteria2 authentication |

All management routes require an administrator session. Every mutation and
credential reveal also requires the session CSRF token. Phase 2 returns `422`
when a user is assigned to an S-UI or standalone sing-box node.

## Build the Phase 2 package

From Windows PowerShell in the repository root:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1 `
  -Architecture amd64 `
  -Version v0.2.0-dev
```

The two files to upload are:

```text
output/releases/hyfleet-v0.2.0-dev-linux-amd64.tar.gz
output/releases/hyfleet-v0.2.0-dev-linux-amd64.tar.gz.sha256
```

Do not upload `数据.txt`, the SQLite database, the master key, a credential,
or an unredacted Hysteria configuration to GitHub.

## 1. Back up and upgrade DMIT

Upload and verify the archive as described in the Phase 1 systemd guide. Before
running the new binary, make a consistent database, key, and binary backup:

```bash
backup_dir="/root/hyfleet-before-v0.2-$(date -u +%Y%m%dT%H%M%SZ)"
sudo install -d -m 0700 "${backup_dir}"
sudo systemctl stop hyfleet-server
sudo cp --preserve=mode,ownership,timestamps \
  /var/lib/hyfleet/server.db /var/lib/hyfleet/master.key "${backup_dir}/"
sudo cp --preserve=mode,ownership,timestamps \
  /usr/local/bin/hyfleet-server "${backup_dir}/hyfleet-server-v0.1"
sudo systemctl start hyfleet-server
```

Run the installer from the extracted `v0.2.0-dev` bundle without
`--replace-config`:

```bash
sudo bash deploy/install-server.sh
curl --fail http://127.0.0.1:8080/healthz
sudo systemctl status hyfleet-server --no-pager -l
```

Startup applies migration `0003_native_users.sql`. Keep `server.db` and
`master.key` together; the database credential ciphertext cannot be recovered
from only one of them.

## 2. Upgrade the LisaHost Agent

Upload and verify the same bundle on LisaHost, then run from its extracted
directory:

```bash
sudo bash deploy/install-agent.sh
sudo systemctl status hyfleet-agent --no-pager -l
sudo ss -ltnp | grep '127.0.0.1:18081'
```

The installer preserves the existing Agent identity and configuration. An older
Phase 1 `agent.yaml` may omit the three auth settings; the Phase 2 Agent safely
uses these defaults:

```yaml
auth_listen: 127.0.0.1:18081
auth_path: /hysteria/auth
auth_cache_path: /var/lib/hyfleet-agent/auth-cache.json
```

Never expose TCP `18081` through a firewall, reverse proxy, or public listener.
The Agent rejects a non-loopback `auth_listen` during configuration loading.

## 3. Create and synchronize a test user

In the web console, open **用户**, add a test user, select only LisaHost, and
retain the displayed LisaHost credential. Each assigned node has a different
credential; a credential from one node must not be reused on another node.

Open the user's node drawer and wait until LisaHost shows **已同步** with equal
applied and desired versions. On LisaHost, verify that the cache exists and the
local endpoint rejects an invalid credential:

```bash
sudo stat -c '%a %U:%G %s %n' /var/lib/hyfleet-agent/auth-cache.json
curl --silent --show-error \
  --header 'Content-Type: application/json' \
  --data '{"addr":"127.0.0.1:1","auth":"invalid","tx":0}' \
  http://127.0.0.1:18081/hysteria/auth
```

Expected response:

```json
{"ok":false}
```

Test the new credential without placing it literally in shell history:

```bash
read -r -s -p 'LisaHost test credential: ' HYFLEET_TEST_AUTH; echo
curl --silent --show-error \
  --header 'Content-Type: application/json' \
  --data "{\"addr\":\"127.0.0.1:1\",\"auth\":\"${HYFLEET_TEST_AUTH}\",\"tx\":0}" \
  http://127.0.0.1:18081/hysteria/auth
unset HYFLEET_TEST_AUTH
```

Expected response has `"ok":true` and an opaque user UUID in `id`.

## 4. Migrate Hysteria2 once

Only continue after the valid local credential test succeeds. Run:

```bash
sudo /usr/local/bin/hyfleet-agent \
  -config /etc/hyfleet/agent.yaml \
  -migrate-hysteria
```

The command performs these bounded actions:

1. Probes the Agent endpoint with an invalid credential and requires HTTP 200
   with `{"ok":false}`.
2. Parses `/etc/hysteria/config.yaml` as YAML and replaces only the `auth` value.
3. Creates a timestamped `0600` backup beside the original config.
4. Atomically installs the new config while preserving original owner and mode.
5. Restarts the configured `hysteria-server.service` and requires it to remain
   active for two seconds.
6. Restores the old file and restarts the old service automatically if the new
   service does not remain active.

The resulting Hysteria block is:

```yaml
auth:
  type: http
  http:
    url: http://127.0.0.1:18081/hysteria/auth
```

Hysteria's official CLI currently has no validation-only command. Running a
second candidate server would also collide with the live UDP port, so HyFleet
uses structural YAML validation followed by a checked restart and automatic
restore. The official invocation remains `hysteria server -c <config>`:
[Hysteria server guide](https://v2.hysteria.network/docs/getting-started/Server/)
and [official server command source](https://github.com/apernet/hysteria/blob/master/app/cmd/server.go).

This migration intentionally removes the old static `auth.password`. Existing
clients using that password stop authenticating after the successful restart.
No later user create, edit, enable, disable, expire, assign, or unassign action
restarts Hysteria2.

Check the service and then connect a client with the new LisaHost credential:

```bash
sudo systemctl status hysteria-server --no-pager -l
sudo journalctl -u hysteria-server -b -n 50 --no-pager
```

## 5. Prove controller-outage authentication

First connect successfully with the new credential. Then stop only the control
plane on DMIT for a short test:

```bash
sudo systemctl stop hyfleet-server
```

Reconnect the client to LisaHost or rerun the valid loopback auth request. It
must still succeed because LisaHost uses its persisted cache. Restore DMIT:

```bash
sudo systemctl start hyfleet-server
curl --fail http://127.0.0.1:8080/healthz
```

Restart `hyfleet-agent` once while DMIT is online, confirm the test credential
still succeeds, then repeat with DMIT offline if a full restart-recovery drill is
desired. Do not delete `auth-cache.json` during the outage test.

## 6. Verify disable and expiry

With DMIT online:

1. Disable only the LisaHost assignment and wait for **已同步**; authentication
   must return `{"ok":false}`.
2. Re-enable the assignment and wait for **已同步**; authentication must succeed.
3. Set the user expiry in the past and wait for **已同步**; authentication must
   return `{"ok":false}`.
4. Restore the intended expiry and verify successful authentication again.

The consistency window is the Agent desired poll interval plus network delay,
normally about ten seconds. The UI's applied/desired versions are authoritative;
do not test a state change while it still shows **等待同步**.

## Rollback

The migration prints the exact backup path. Restore it with:

```bash
sudo /usr/local/bin/hyfleet-agent \
  -config /etc/hyfleet/agent.yaml \
  -rollback-hysteria /etc/hysteria/config.yaml.hyfleet-backup-TIMESTAMP
```

Rollback first backs up the current config, atomically restores the selected
backup, restarts Hysteria, and automatically returns to the pre-rollback config
if that restart fails. After a successful rollback, the old static password is
valid again and HyFleet-created credentials are no longer consulted by
Hysteria2. The Agent and user records may remain installed for diagnosis.

## Automated evidence

The Go tests cover encrypted-at-rest credentials, cross-node isolation, native
snapshot verifier content, disable and expiry propagation, acknowledgement
state, unassignment, cache restart recovery, controller-offline authentication,
snapshot rollback rejection, HTTP auth contract, API lifecycle, CSRF, secret
non-disclosure, migration backup, and migration failure recovery.

The frontend is checked with `vue-tsc`, Vitest, a production Vite build, and the
browser acceptance workflow at desktop and mobile widths.

## Exit checklist

- [x] Go unit, integration, and migration tests pass locally.
- [x] `go vet ./...` passes locally.
- [x] Frontend type checking, unit tests, and production build pass locally.
- [x] Linux amd64 release bundle builds with checksums.
- [ ] DMIT Server and LisaHost Agent are upgraded from the Phase 2 bundle.
- [ ] LisaHost test user authenticates without a per-user Hysteria restart.
- [ ] Authentication survives DMIT Server outage and Agent restart.
- [ ] Assignment disable and past expiry both reject after snapshot apply.
- [ ] The documented Hysteria rollback succeeds once on LisaHost.

The final five checks change the owner's live machines and must be run from the
owner's SSH sessions. Do not paste credentials, backup contents, or unredacted
configuration into an issue or chat.
