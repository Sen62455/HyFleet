# Phase 7: public release and disaster recovery

HyFleet v1.0 provides a public, checksum-verified native installer, a rootless
Server container, data-aware upgrade rollback, consistent Server backup/restore,
clean-host CI, an SBOM, release signatures, and a private security-reporting
policy.

## Native Server installation

Use a dedicated HTTPS origin and terminate TLS in Caddy, Nginx, or another
reverse proxy. Download the bootstrap from the release tag so the reviewed script
and requested binaries have the same version:

```bash
curl --fail --location --proto '=https' --tlsv1.2 \
  -o install.sh \
  https://raw.githubusercontent.com/Sen62455/HyFleet/v1.0.0/install.sh
less install.sh
sudo bash install.sh server \
  --version v1.0.0 \
  --public-url https://panel.example.com
```

The bootstrap accepts only Debian/Ubuntu with systemd, installs a small package
set, detects amd64/arm64, downloads both release files, checks the external
SHA-256, extracts the archive, checks every packaged file, and invokes the native
installer. It does not configure DNS, TLS, or the reverse proxy.

The installer prints a one-time bootstrap Token. Create the administrator over
HTTPS and remove `/etc/hyfleet/server.env` as instructed. Do not store the Token
in shell history, screenshots, or issue reports.

## Native Agent installation

Create the node and one-time enrollment Token in HyFleet first. Install the Agent
on the node that already runs its proxy core:

```bash
sudo bash install.sh agent \
  --version v1.0.0 \
  --server-url https://panel.example.com \
  --node-name example-node \
  --adapter native-hysteria2
```

For a split sing-box configuration, pass the actual directory without a trailing
slash:

```bash
sudo bash install.sh agent \
  --version v1.0.0 \
  --server-url https://panel.example.com \
  --node-name example-sing-box \
  --adapter standalone-sing-box \
  --core-config-path /etc/sing-box/conf
```

The installer rejects missing paths, symbolic links, paths outside the adapter's
fixed `/etc` tree, and a directory written with a trailing slash. A directory
backup is a bounded `tar.gz`; the helper never follows links or includes nearby
sing-box binaries, databases, certificates, logs, or subscription files.

## Docker Server installation

Only the control plane is containerized. Copy `docker/compose.yaml` and
`docker/.env.example` to the server, rename the latter to `.env`, set the HTTPS
origin, and generate the bootstrap Token:

```bash
openssl rand -hex 32
docker compose --env-file .env -f compose.yaml up -d
docker compose --env-file .env -f compose.yaml ps
```

The published image runs as UID/GID 10001, drops all capabilities, uses a
read-only root filesystem, and binds port 8080 to host loopback. Persisted state
is in the `hyfleet-data` volume. After creating the administrator, remove
`HYFLEET_BOOTSTRAP_TOKEN` from `.env` and recreate the container.

Do not mount the Docker socket or host `/etc` into this container. Agents remain
native systemd services.

## Fleet upgrade and rollback

For an existing three-node installation on Windows:

```powershell
.\scripts\deploy-fleet.ps1 -Version v1.0.0
```

The updater checks both checksum layers before upload. On each VPS it stores the
old binary, unit, configuration, and standard data files under
`/var/lib/hyfleet-updates`, stops the component briefly, installs the new files,
and runs configuration and health checks. A failed update automatically restores
both code and data. Successful snapshots contain secrets and are root-only;
remove obsolete snapshots after a verified off-VPS backup.

Manual single-host upgrade remains available from an extracted release:

```bash
sudo bash deploy/update-component.sh server
sudo bash deploy/update-component.sh agent
```

Upgrade Server before Agents. v1.0 accepts the preceding v0.6 Agent during the
short rolling-upgrade interval.

## Server backup

Create a live, transactionally consistent SQLite snapshot:

```bash
sudo bash deploy/backup-server.sh --output-dir /var/backups/hyfleet
```

Four root-only files are produced: archive, archive checksum, master key, and
master-key checksum. The archive contains `server.db`, `server.yaml`, and a
manifest with internal hashes. The master key is deliberately separate. Copy the
archive and key to separately protected encrypted off-VPS storage; neither alone
is a complete credential backup.

## Restore on a clean VPS

1. Install the same or a newer stable HyFleet release with the native Server
   installer.
2. Copy the four backup files to a root-only directory.
3. Run:

```bash
sudo bash deploy/restore-server.sh \
  --archive /root/restore/hyfleet-server-backup-TIME.tar.gz \
  --checksum /root/restore/hyfleet-server-backup-TIME.tar.gz.sha256 \
  --master-key /root/restore/hyfleet-server-master-key-TIME.key \
  --master-key-checksum /root/restore/hyfleet-server-master-key-TIME.key.sha256
```

The restore validates outer and inner hashes, the 32-byte key, SQLite integrity,
foreign keys, HyFleet schema identity, and configuration paths before stopping
the service. If the restored service fails its local health check, the script
restores the pre-restore database, key, and configuration automatically.

After a successful restore, point the original HTTPS DNS name to the new VPS,
verify login, node heartbeats, a subscription, and a fresh backup, then remove the
root-only pre-restore snapshot reported by the script.

## Release verification

Each architecture has an archive and `.sha256` file. Stable releases also attach
an SPDX JSON SBOM and Sigstore bundle. Verify the checksum before extraction and
verify a v1.0.0 blob with the exact GitHub workflow identity:

```bash
cosign verify-blob hyfleet-v1.0.0-linux-amd64.tar.gz \
  --bundle hyfleet-v1.0.0-linux-amd64.tar.gz.sigstore.json \
  --certificate-identity \
    https://github.com/Sen62455/HyFleet/.github/workflows/release.yml@refs/tags/v1.0.0 \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Internal `SHA256SUMS` must pass before any installer is run. The GHCR image is
also signed by the same workflow and should be verified by immutable digest.

## Known boundaries

- S-UI database backup is not attempted while S-UI is online.
- Standalone sing-box users, traffic accounting, and subscription membership are
  not part of v1.0.
- Docker disaster recovery requires backing up the `hyfleet-data` volume while
  the Server is stopped; the native two-file backup workflow is the tested
  cross-host recovery path.
- Email, Telegram, and webhook alert delivery remain post-v1 work.
