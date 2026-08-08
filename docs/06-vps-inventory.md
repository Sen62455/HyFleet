# VPS Inventory

- Status: platform inventory confirmed; adapter details partly pending
- Last updated: 2026-08-07

This document contains only non-secret inventory suitable for Git. Real IPs,
SSH coordinates, API tokens, user passwords, private keys, complete config files,
and subscription URLs belong in an ignored local inventory or a secret manager.

## 1. Known fleet

| ID | Provider | Current data plane | Current management | Planned adapter | Confirmed |
| --- | --- | --- | --- | --- | --- |
| `dmit-01` | DMIT | HY2 through sing-box | S-UI v1.5.3 | `s_ui` | Platform confirmed |
| `lisa-01` | LisaHost | Hysteria2 v2.12.0 | systemd | `native_hysteria2` | Core confirmed |
| `bwh-01` | BandwagonHost | HY2 through sing-box | systemd | `standalone_sing_box` | Type confirmed |

Do not assume that a node called "Hysteria2" runs the official `hysteria`
binary. It may run sing-box or an install-script wrapper. Adapter selection is
final only after binary, service, and config ownership are confirmed.

## 2. Required non-secret facts per node

| Category | Required fields | Why needed |
| --- | --- | --- |
| Platform | Distribution/version, kernel, amd64/arm64, systemd availability | Release artifact and service hardening |
| Capacity | vCPU, RAM, swap, root disk free space | Resource budget and controller placement |
| Network | IPv4/IPv6 availability, public HY2 UDP port/range, controller egress | Endpoint and firewall design |
| Time | Timezone, NTP synchronized state | Expiry, batches, and TLS correctness |
| Core | Official Hysteria2/sing-box, exact version, executable path | Adapter compatibility |
| Service | systemd unit name, running/enabled state, service account | Safe status/restart allowlist |
| Config | Config path only, format, auth type, traffic-stats state | Migration plan; do not commit content |
| TLS | ACME/file/self-signed mode, certificate path owner, renewal mechanism | Avoid certificate outage |
| Client metadata | Public domain, SNI behavior, obfs type, port hopping yes/no | Subscription rendering |
| S-UI only | S-UI version, local port/base path, API token support, inbound ID/tag | Compatibility and adoption mapping |
| Operations | Current backup method, firewall tool, auto-update policy | Rollback and safe deployment |

## 3. Current unknowns

### DMIT (`dmit-01`)

- Confirmed: Ubuntu 24.04.4, amd64, systemd 255, 1 vCPU, 1.9 GiB RAM,
  1 GiB swap, and 16 GiB root-disk free space.
- Confirmed: `s-ui.service` is enabled/running and S-UI reports v1.5.3.
- Confirmed from the supplied panel view: Hysteria2 inbound tag
  `hysteria2-443`, UDP port 443, TLS enabled, no obfuscation/port hopping.
- Installation type is likely native but is not yet proven; local API base path,
  sing-box version, and TLS/ACME ownership remain unknown.
- Existing credentials may be rotated with advance notice; automatic secret
  import is not required.
- Selected control-plane host: native systemd deployment on this VPS.
- Security action: the S-UI panel was observed over public HTTP. Restrict it to
  loopback/VPN/trusted IPs or place it behind authenticated HTTPS before entering
  or rotating API credentials.

### LisaHost (`lisa-01`)

- Confirmed: Ubuntu 24.04.1, amd64, systemd 255, 1 vCPU, 961 MiB RAM,
  no swap, and 16 GiB root-disk free space.
- Confirmed official Hysteria2 v2.12.0, release commit `23c90861add3`, running as
  enabled `hysteria-server.service`.
- Executable path and configuration path still require confirmation.
- Current auth type, Traffic Stats availability, and restart behavior.
- Publicly trusted endpoint certificate confirmed; no obfuscation or port
  hopping. TLS/ACME ownership remains unknown.

### BandwagonHost (`bwh-01`)

- Confirmed: Ubuntu 24.04.4, amd64, systemd 255, 2 vCPU, 1 GiB RAM,
  2.5 GiB swap, and 5.6 GiB root-disk free space (68% used).
- Confirmed standalone `sing-box.service` is enabled/running. Exact binary path,
  version, start command, config path/layout, and reload behavior remain unknown.
- Publicly trusted endpoint certificate confirmed; no obfuscation or port
  hopping. Existing credentials may be rotated with advance notice.
- Whether UDP or provider firewall restrictions affect status testing.

## 4. Owner decisions

- The fleet is personal-only initially but must support multiple independent
  proxy users later.
- Existing passwords and subscription URLs do not need to be preserved, but all
  rotations require a clear change notice and rollback instructions.
- Global quota is scheduled for v0.3; v0.2 may ship enable/disable and expiry.
- Docker is currently present only on BandwagonHost. Native systemd is selected
  for the DMIT controller to avoid adding a runtime solely for HyFleet.
- Every Hysteria2 endpoint uses a domain with a publicly trusted certificate.
- No endpoint currently uses obfuscation or UDP port hopping.

## 5. Safe read-only collection

Run the following on each VPS as an account allowed to query systemd. These
commands do not intentionally read configuration contents or secret files.
Redact hostnames, public IPs, domains, usernames, machine/boot IDs, and unexpected
environment values before sharing output.

```bash
export LC_ALL=C

printf 'PLATFORM\n'
cat /etc/os-release
uname -m
uname -r
systemd --version | sed -n '1p'

printf 'CAPACITY\n'
nproc
free -h
df -h /

printf 'TIME\n'
timedatectl status

printf 'SERVICES\n'
systemctl list-units --type=service --all 'hysteria*' 'sing-box*' 's-ui*' --no-pager
systemctl list-unit-files 'hysteria*' 'sing-box*' 's-ui*' --no-pager

printf 'VERSIONS\n'
command -v hysteria >/dev/null 2>&1 && hysteria version
command -v sing-box >/dev/null 2>&1 && sing-box version
command -v s-ui >/dev/null 2>&1 && s-ui version

printf 'CLOCK\n'
date -u --iso-8601=seconds
```

`s-ui version` may be unsupported or interactive on some installations. If so,
report the version visible in the panel without running further commands.

Do **not** send the output of `cat` on Hysteria2/S-UI configuration, `.env`
files, process environments, certificate keys, token files, shell history, or a
full `ss`/firewall dump in a public issue.

## 6. Owner questionnaire

The owner should answer these behavior questions without providing secrets:

1. Which VPS should host the control plane, or should placement be decided from
   the capacity inventory?
2. Are all three nodes personal-only, or will multiple independent users receive
   accounts?
3. Must existing client passwords and subscription URLs remain valid during
   migration?
4. Is each HY2 endpoint a domain with a publicly trusted certificate?
5. Does either native node use obfuscation or UDP port hopping?
6. Is global quota required for the first users, or can v0.2 ship with expiry and
   enable/disable before quota accounting arrives in v0.3?
7. Is Docker already installed anywhere, and is a native systemd controller
   preferred over Docker?

## 7. Inventory completion gate

Inventory is complete when every required field has an owner-confirmed value,
adapter type is proven by executable/service evidence, controller placement is
chosen, and a rollback/backup path exists for each node. Secrets may remain
unknown to the design document and are created or supplied locally during the
relevant implementation phase.
