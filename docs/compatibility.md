# Compatibility matrix

## Hosts

| Component | Supported host | Architectures | Supervisor |
| --- | --- | --- | --- |
| Server, native | Ubuntu 24.04 LTS; Debian 12 and 13 | amd64, arm64 | systemd |
| Agent | Ubuntu 24.04 LTS; Debian 12 and 13 | amd64, arm64 | systemd |
| Server, container | Linux with Docker Engine and Compose v2 | amd64, arm64 | Docker |

Native CI performs clean-host installation and backup/restore drills on Ubuntu
24.04 and Debian. Other systemd distributions may work but are not release
gates. The bootstrap installer intentionally rejects unsupported distributions.

The Docker image contains only HyFleet Server. Agent containers are unsupported
because safe operation requires local systemd, core configuration paths, and a
root-owned Unix socket helper.

## Core adapters

| Adapter | Tested contract | User management | Traffic/subscription |
| --- | --- | --- | --- |
| Native Hysteria2 | Hysteria2 v2 HTTP auth and traffic APIs | Managed | Managed |
| S-UI | S-UI v1.5.3 through versions below v1.6.0, `/apiv2` | Explicit adoption | Managed after adoption |
| Standalone sing-box | `sing-box.service`, fixed file or directory configuration | Observation only | Not yet managed |

Standalone sing-box supports core status, restart, bounded logs, alerts, and
node-local configuration backup. A configured directory may contain at most 512
regular files/directories, be at most 16 levels deep, and contain at most 8 MiB
of uncompressed data. Symbolic links and special files are rejected.

## Deployment contracts

- Server native layout: `/etc/hyfleet` and `/var/lib/hyfleet`.
- Agent native layout: `/etc/hyfleet`, `/var/lib/hyfleet-agent`,
  `/var/lib/hyfleet-backups`, and `/var/lib/hyfleet-agent-ops`.
- HTTPS is mandatory between Agent and Server.
- Existing S-UI clients remain read-only until explicitly adopted.
- Forward restore into the same or a newer stable Server is supported. Downgrade
  across a database migration requires the pre-upgrade database snapshot.
