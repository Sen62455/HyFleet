#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
bundle_dir="$(cd -- "${script_dir}/.." && pwd)"
component="${1:-}"
backup_root="/var/lib/hyfleet-updates"
backup_dir=""
committed=false

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

case "${component}" in
  server)
    binary_name="hyfleet-server"
    service_name="hyfleet-server.service"
    service_user="hyfleet"
    service_group="hyfleet"
    config_path="/etc/hyfleet/server.yaml"
    ;;
  agent)
    binary_name="hyfleet-agent"
    service_name="hyfleet-agent.service"
    service_user="hyfleet-agent"
    service_group="hyfleet-agent"
    config_path="/etc/hyfleet/agent.yaml"
    ;;
  *)
    fail "usage: sudo bash deploy/update-component.sh server|agent"
    ;;
esac

source_binary="${bundle_dir}/bin/${binary_name}"
source_unit="${script_dir}/systemd/${service_name}"
target_binary="/usr/local/bin/${binary_name}"
target_unit="/etc/systemd/system/${service_name}"

rollback() {
  local status="$1"
  if [[ "${committed}" != true && -n "${backup_dir}" && -d "${backup_dir}" ]]; then
    set +e
    printf 'Update failed; restoring %s.\n' "${component}" >&2
    install -o root -g root -m 0755 "${backup_dir}/${binary_name}" "${target_binary}"
    install -o root -g root -m 0644 "${backup_dir}/${service_name}" "${target_unit}"
    systemctl daemon-reload
    systemctl restart "${service_name}"
  fi
  exit "${status}"
}
trap 'rollback $?' EXIT

[[ "${EUID}" -eq 0 ]] || fail "run this updater with sudo"
for command_name in curl install mktemp od runuser systemctl systemd-analyze; do
  command -v "${command_name}" >/dev/null 2>&1 || fail "required command is missing: ${command_name}"
done
[[ -f "${source_binary}" && -f "${source_unit}" ]] || fail "release bundle is incomplete"
[[ -x "${target_binary}" && -f "${target_unit}" && -f "${config_path}" ]] ||
  fail "${component} is not installed; use the initial installer first"

elf_magic="$(od -An -t x1 -N 4 "${source_binary}" | tr -d '[:space:]')"
[[ "${elf_magic}" == "7f454c46" ]] || fail "${binary_name} is not a Linux ELF binary"
elf_machine="$(od -An -t x1 -j 18 -N 2 "${source_binary}" | tr -d '[:space:]')"
case "$(uname -m)" in
  x86_64) expected_machine="3e00" ;;
  aarch64|arm64) expected_machine="b700" ;;
  *) fail "unsupported host architecture: $(uname -m)" ;;
esac
[[ "${elf_machine}" == "${expected_machine}" ]] || fail "binary architecture does not match this host"

install -d -o root -g root -m 0700 "${backup_root}"
backup_dir="$(mktemp -d "${backup_root}/${component}.XXXXXXXX")"
[[ "${backup_dir}" == "${backup_root}/"* && "${backup_dir}" != "${backup_root}" ]] ||
  fail "invalid update backup directory"
install -o root -g root -m 0755 "${target_binary}" "${backup_dir}/${binary_name}"
install -o root -g root -m 0644 "${target_unit}" "${backup_dir}/${service_name}"

install -o root -g root -m 0755 "${source_binary}" "${target_binary}"
install -o root -g root -m 0644 "${source_unit}" "${target_unit}"
"${target_binary}" -version
runuser -u "${service_user}" -g "${service_group}" -- \
  "${target_binary}" -config "${config_path}" -check-config
systemctl daemon-reload
systemd-analyze verify "${target_unit}"
systemctl restart "${service_name}"

active=false
for _ in {1..20}; do
  if systemctl is-active --quiet "${service_name}"; then
    active=true
    break
  fi
  sleep 1
done
[[ "${active}" == true ]] || fail "${service_name} did not become active"

if [[ "${component}" == "server" ]]; then
  listen_address="$(awk '$1 == "listen:" { print $2; exit }' "${config_path}")"
  [[ "${listen_address}" =~ ^127\.0\.0\.1:[0-9]{1,5}$ ]] || fail "invalid server listen address"
  healthy=false
  for _ in {1..20}; do
    if curl --fail --silent --show-error "http://${listen_address}/healthz" >/dev/null 2>&1; then
      healthy=true
      break
    fi
    sleep 1
  done
  [[ "${healthy}" == true ]] || fail "HyFleet Server health check failed"
else
  sleep 3
  systemctl is-active --quiet "${service_name}" || fail "HyFleet Agent did not remain active"
fi

committed=true
rm -rf -- "${backup_dir}"
backup_dir=""
printf '%s update completed.\n' "${component}"
