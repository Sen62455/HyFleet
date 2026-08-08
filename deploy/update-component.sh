#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
bundle_dir="$(cd -- "${script_dir}/.." && pwd)"
component="${1:-}"
backup_root="/var/lib/hyfleet-updates"
backup_dir=""
committed=false
data_backed_up=false
had_ops_binary=false
had_ops_socket=false
had_ops_service=false

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

restore_optional_agent_file() {
  local snapshot_name="$1"
  local target_path="$2"
  local owner="$3"
  local mode="$4"
  if [[ -f "${backup_dir}/${snapshot_name}" ]]; then
    install -o "${owner}" -g hyfleet-agent -m "${mode}" \
      "${backup_dir}/${snapshot_name}" "${target_path}"
  else
    rm -f -- "${target_path}"
  fi
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

if [[ "${component}" == "agent" ]]; then
  ops_source_binary="${bundle_dir}/bin/hyfleet-agent-ops"
  ops_source_socket="${script_dir}/systemd/hyfleet-agent-ops.socket"
  ops_source_service="${script_dir}/systemd/hyfleet-agent-ops@.service"
  ops_target_binary="/usr/local/libexec/hyfleet-agent-ops"
  ops_target_socket="/etc/systemd/system/hyfleet-agent-ops.socket"
  ops_target_service="/etc/systemd/system/hyfleet-agent-ops@.service"
fi

rollback() {
  local status="$1"
  local rollback_listen=""
  local rollback_ready=false
  if [[ "${committed}" != true && -n "${backup_dir}" && -d "${backup_dir}" ]]; then
    set +e
    printf 'Update failed; restoring %s.\n' "${component}" >&2
    systemctl stop "${service_name}"
    install -o root -g root -m 0755 "${backup_dir}/${binary_name}" "${target_binary}"
    install -o root -g root -m 0644 "${backup_dir}/${service_name}" "${target_unit}"
    if [[ "${component}" == "agent" ]]; then
      if [[ "${had_ops_binary}" == true ]]; then
        install -o root -g root -m 0755 "${backup_dir}/hyfleet-agent-ops" "${ops_target_binary}"
      else
        rm -f -- "${ops_target_binary}"
      fi
      if [[ "${had_ops_socket}" == true ]]; then
        install -o root -g root -m 0644 "${backup_dir}/hyfleet-agent-ops.socket" "${ops_target_socket}"
      else
        systemctl disable --now hyfleet-agent-ops.socket
        rm -f -- "${ops_target_socket}"
      fi
      if [[ "${had_ops_service}" == true ]]; then
        install -o root -g root -m 0644 "${backup_dir}/hyfleet-agent-ops@.service" "${ops_target_service}"
      else
        rm -f -- "${ops_target_service}"
      fi
    fi
    if [[ "${data_backed_up}" == true && "${component}" == "server" ]]; then
      install -o hyfleet -g hyfleet -m 0600 "${backup_dir}/server.db" /var/lib/hyfleet/server.db
      install -o hyfleet -g hyfleet -m 0600 "${backup_dir}/master.key" /var/lib/hyfleet/master.key
      install -o root -g hyfleet -m 0640 "${backup_dir}/server.yaml" /etc/hyfleet/server.yaml
      if [[ -f "${backup_dir}/server.db-wal" ]]; then
        install -o hyfleet -g hyfleet -m 0600 \
          "${backup_dir}/server.db-wal" /var/lib/hyfleet/server.db-wal
      else
        rm -f -- /var/lib/hyfleet/server.db-wal
      fi
      rm -f -- /var/lib/hyfleet/server.db-shm
    elif [[ "${data_backed_up}" == true ]]; then
      install -o root -g hyfleet-agent -m 0640 "${backup_dir}/agent.yaml" /etc/hyfleet/agent.yaml
      restore_optional_agent_file agent.env /etc/hyfleet/agent.env root 0640
      restore_optional_agent_file hy2-stats.env /etc/hyfleet/hy2-stats.env root 0640
      restore_optional_agent_file \
        agent-state.json /var/lib/hyfleet-agent/agent-state.json hyfleet-agent 0600
      restore_optional_agent_file agent.db /var/lib/hyfleet-agent/agent.db hyfleet-agent 0600
      restore_optional_agent_file agent.db-wal /var/lib/hyfleet-agent/agent.db-wal hyfleet-agent 0600
      restore_optional_agent_file auth-cache.json /var/lib/hyfleet-agent/auth-cache.json hyfleet-agent 0600
      rm -f -- /var/lib/hyfleet-agent/agent.db-shm
    fi
    systemctl daemon-reload
    systemctl restart "${service_name}"
    if [[ "${component}" == "server" ]]; then
      rollback_listen="$(awk '$1 == "listen:" { print $2; exit }' /etc/hyfleet/server.yaml)"
    fi
    for _ in {1..20}; do
      if systemctl is-active --quiet "${service_name}"; then
        if [[ "${component}" != "server" ]]; then
          rollback_ready=true
          break
        fi
        if [[ "${rollback_listen}" =~ ^127\.0\.0\.1:[0-9]{1,5}$ ]] &&
          curl --fail --silent --show-error "http://${rollback_listen}/healthz" >/dev/null 2>&1; then
          rollback_ready=true
          break
        fi
      fi
      sleep 1
    done
    if [[ "${component}" == "agent" && "${rollback_ready}" == true ]]; then
      sleep 3
      systemctl is-active --quiet "${service_name}" || rollback_ready=false
    fi
    if [[ "${rollback_ready}" == true ]]; then
      printf '%s rollback completed and passed its health check.\n' "${component}" >&2
    else
      printf 'ERROR: %s rollback did not pass its health check.\n' "${component}" >&2
      systemctl status "${service_name}" --no-pager --full >&2 || true
      status=1
    fi
  fi
  exit "${status}"
}
trap 'rollback $?' EXIT

[[ "${EUID}" -eq 0 ]] || fail "run this updater with sudo"
for command_name in awk basename curl grep install mktemp od runuser sleep systemctl systemd-analyze tr uname; do
  command -v "${command_name}" >/dev/null 2>&1 || fail "required command is missing: ${command_name}"
done
[[ -f "${source_binary}" && -f "${source_unit}" ]] || fail "release bundle is incomplete"
if [[ "${component}" == "agent" ]]; then
  [[ -f "${ops_source_binary}" && -f "${ops_source_socket}" && -f "${ops_source_service}" ]] ||
    fail "release bundle has no operations helper"
fi
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
if [[ "${component}" == "agent" ]]; then
  ops_elf_magic="$(od -An -t x1 -N 4 "${ops_source_binary}" | tr -d '[:space:]')"
  [[ "${ops_elf_magic}" == "7f454c46" ]] || fail "hyfleet-agent-ops is not a Linux ELF binary"
  ops_elf_machine="$(od -An -t x1 -j 18 -N 2 "${ops_source_binary}" | tr -d '[:space:]')"
  [[ "${ops_elf_machine}" == "${expected_machine}" ]] ||
    fail "hyfleet-agent-ops architecture does not match this host"
fi

install -d -o root -g root -m 0700 "${backup_root}"
backup_dir="$(mktemp -d "${backup_root}/${component}.XXXXXXXX")"
[[ "${backup_dir}" == "${backup_root}/"* && "${backup_dir}" != "${backup_root}" ]] ||
  fail "invalid update backup directory"
install -o root -g root -m 0755 "${target_binary}" "${backup_dir}/${binary_name}"
install -o root -g root -m 0644 "${target_unit}" "${backup_dir}/${service_name}"
if [[ "${component}" == "agent" ]]; then
  if [[ -x "${ops_target_binary}" ]]; then
    had_ops_binary=true
    install -o root -g root -m 0755 "${ops_target_binary}" "${backup_dir}/hyfleet-agent-ops"
  fi
  if [[ -f "${ops_target_socket}" ]]; then
    had_ops_socket=true
    install -o root -g root -m 0644 "${ops_target_socket}" "${backup_dir}/hyfleet-agent-ops.socket"
  fi
  if [[ -f "${ops_target_service}" ]]; then
    had_ops_service=true
    install -o root -g root -m 0644 "${ops_target_service}" "${backup_dir}/hyfleet-agent-ops@.service"
  fi
fi

systemctl stop "${service_name}"
if [[ "${component}" == "server" ]]; then
  [[ -f /var/lib/hyfleet/server.db && ! -L /var/lib/hyfleet/server.db &&
    -f /var/lib/hyfleet/master.key && ! -L /var/lib/hyfleet/master.key ]] ||
    fail "server database or master key is missing or unsafe"
  install -o root -g root -m 0600 /var/lib/hyfleet/server.db "${backup_dir}/server.db"
  if [[ -f /var/lib/hyfleet/server.db-wal && ! -L /var/lib/hyfleet/server.db-wal ]]; then
    install -o root -g root -m 0600 /var/lib/hyfleet/server.db-wal "${backup_dir}/server.db-wal"
  fi
  install -o root -g root -m 0600 /var/lib/hyfleet/master.key "${backup_dir}/master.key"
  install -o root -g root -m 0600 /etc/hyfleet/server.yaml "${backup_dir}/server.yaml"
else
  install -o root -g root -m 0600 /etc/hyfleet/agent.yaml "${backup_dir}/agent.yaml"
  for optional_path in \
    /etc/hyfleet/agent.env \
    /etc/hyfleet/hy2-stats.env \
    /var/lib/hyfleet-agent/agent-state.json \
    /var/lib/hyfleet-agent/agent.db \
    /var/lib/hyfleet-agent/agent.db-wal \
    /var/lib/hyfleet-agent/auth-cache.json; do
    if [[ -f "${optional_path}" && ! -L "${optional_path}" ]]; then
      install -o root -g root -m 0600 "${optional_path}" "${backup_dir}/$(basename -- "${optional_path}")"
    fi
  done
fi
data_backed_up=true

install -o root -g root -m 0755 "${source_binary}" "${target_binary}"
install -o root -g root -m 0644 "${source_unit}" "${target_unit}"
if [[ "${component}" == "agent" ]]; then
  install -d -o root -g root -m 0755 /usr/local/libexec
  install -d -o root -g root -m 0700 /var/lib/hyfleet-backups /var/lib/hyfleet-agent-ops
  install -o root -g root -m 0755 "${ops_source_binary}" "${ops_target_binary}"
  install -o root -g root -m 0644 "${ops_source_socket}" "${ops_target_socket}"
  install -o root -g root -m 0644 "${ops_source_service}" "${ops_target_service}"
fi
"${target_binary}" -version
runuser -u "${service_user}" -g "${service_group}" -- \
  "${target_binary}" -config "${config_path}" -check-config
if [[ "${component}" == "agent" ]]; then
  "${ops_target_binary}" -version
  "${ops_target_binary}" -config "${config_path}" -check-config
  configured_adapter="$(awk '$1 == "adapter_type:" { print $2; exit }' "${config_path}")"
  if [[ "${configured_adapter}" == "s_ui" ]]; then
    grep -Eq '^HYFLEET_SUI_TOKEN=[^[:space:]]+$' /etc/hyfleet/agent.env ||
      fail "S-UI Agent requires HYFLEET_SUI_TOKEN in /etc/hyfleet/agent.env"
  fi
fi
systemctl daemon-reload
if [[ "${component}" == "agent" ]]; then
  systemd-analyze verify "${target_unit}" "${ops_target_socket}" "${ops_target_service}"
  systemctl enable --now hyfleet-agent-ops.socket
else
  systemd-analyze verify "${target_unit}"
fi
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
printf '%s update completed. Rollback snapshot: %s\n' "${component}" "${backup_dir}"
