#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
bundle_dir="$(cd -- "${script_dir}/.." && pwd)"
source_binary="${bundle_dir}/bin/hyfleet-agent"
source_unit="${script_dir}/systemd/hyfleet-agent.service"

server_url=""
node_name=""
adapter_type=""
service_unit=""
core_name=""
replace_config=false
temporary_dir=""

usage() {
  cat <<'EOF'
Usage:
  sudo bash deploy/install-agent.sh \
    --server-url https://panel.example.com \
    --node-name LisaHost \
    --adapter native-hysteria2

Adapters:
  native-hysteria2    Hysteria2 systemd service
  standalone-sing-box
  s-ui

Options:
  --server-url URL    HyFleet HTTPS origin (required initially)
  --node-name NAME    Local node label using letters, numbers, dot, dash, underscore
  --adapter TYPE      One adapter from the list above
  --service-unit UNIT Override the adapter's default systemd unit
  --replace-config    Replace /etc/hyfleet/agent.yaml with generated settings
  -h, --help          Show this help
EOF
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [[ -n "${temporary_dir}" && -d "${temporary_dir}" ]]; then
    rm -rf -- "${temporary_dir}"
  fi
}
trap cleanup EXIT
trap 'printf "ERROR: installation failed at line %s.\n" "$LINENO" >&2' ERR

while (($# > 0)); do
  case "$1" in
    --server-url)
      (($# >= 2)) || fail "--server-url requires a value"
      server_url="$2"
      shift 2
      ;;
    --node-name)
      (($# >= 2)) || fail "--node-name requires a value"
      node_name="$2"
      shift 2
      ;;
    --adapter)
      (($# >= 2)) || fail "--adapter requires a value"
      adapter_type="$2"
      shift 2
      ;;
    --service-unit)
      (($# >= 2)) || fail "--service-unit requires a value"
      service_unit="$2"
      shift 2
      ;;
    --replace-config)
      replace_config=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

[[ "${EUID}" -eq 0 ]] || fail "run this installer with sudo"

for command_name in curl getent groupadd install runuser systemctl systemd-analyze useradd; do
  command -v "${command_name}" >/dev/null 2>&1 || fail "required command is missing: ${command_name}"
done

[[ -f "${source_binary}" ]] || fail "missing ${source_binary}; extract the complete release archive"
[[ -f "${source_unit}" ]] || fail "missing ${source_unit}; extract the complete release archive"

elf_magic="$(od -An -t x1 -N 4 "${source_binary}" | tr -d '[:space:]')"
[[ "${elf_magic}" == "7f454c46" ]] || fail "hyfleet-agent is not a Linux ELF binary"
elf_machine="$(od -An -t x1 -j 18 -N 2 "${source_binary}" | tr -d '[:space:]')"
case "$(uname -m)" in
  x86_64) expected_machine="3e00" ;;
  aarch64|arm64) expected_machine="b700" ;;
  *) fail "unsupported host architecture: $(uname -m)" ;;
esac
[[ "${elf_machine}" == "${expected_machine}" ]] ||
  fail "hyfleet-agent architecture does not match host $(uname -m)"

config_path="/etc/hyfleet/agent.yaml"
state_path="/var/lib/hyfleet-agent/agent-state.json"
if [[ "${replace_config}" == true && -f "${state_path}" ]] &&
  grep -Eq '"node_credential"[[:space:]]*:[[:space:]]*"[^\"]+' "${state_path}"; then
  fail "Agent is already enrolled; refusing to replace its identity configuration"
fi
if [[ ! -f "${config_path}" || "${replace_config}" == true ]]; then
  [[ "${server_url}" =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] ||
    fail "--server-url must be an HTTPS origin without a path"
  [[ "${node_name}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] ||
    fail "--node-name contains unsupported characters"

  case "${adapter_type}" in
    native-hysteria2)
      adapter_type="native_hysteria2"
      core_name="hysteria"
      : "${service_unit:=hysteria-server.service}"
      ;;
    standalone-sing-box)
      adapter_type="standalone_sing_box"
      core_name="sing-box"
      : "${service_unit:=sing-box.service}"
      ;;
    s-ui)
      adapter_type="s_ui"
      core_name="sing-box"
      : "${service_unit:=s-ui.service}"
      ;;
    *)
      fail "unsupported --adapter value: ${adapter_type}"
      ;;
  esac
  [[ "${service_unit}" =~ ^[A-Za-z0-9_.@:-]+$ ]] || fail "invalid systemd service unit"
elif [[ -n "${server_url}${node_name}${adapter_type}${service_unit}" ]]; then
  printf 'Keeping existing %s; supplied configuration options were not applied.\n' "${config_path}"
fi

getent group hyfleet-agent >/dev/null 2>&1 || groupadd --system hyfleet-agent
if ! id -u hyfleet-agent >/dev/null 2>&1; then
  useradd --system --gid hyfleet-agent --home-dir /var/lib/hyfleet-agent \
    --shell /usr/sbin/nologin hyfleet-agent
fi

install -d -o root -g root -m 0755 /etc/hyfleet
install -d -o hyfleet-agent -g hyfleet-agent -m 0700 /var/lib/hyfleet-agent
install -o root -g root -m 0755 "${source_binary}" /usr/local/bin/hyfleet-agent
/usr/local/bin/hyfleet-agent -version

temporary_dir="$(mktemp -d)"
if [[ ! -f "${config_path}" || "${replace_config}" == true ]]; then
  cat > "${temporary_dir}/agent.yaml" <<EOF
server_url: ${server_url}
node_name: ${node_name}
adapter_type: ${adapter_type}
core_name: ${core_name}
service_unit: ${service_unit}
state_path: /var/lib/hyfleet-agent/agent-state.json
heartbeat_every: 15s
desired_every: 10s
EOF
  install -o root -g hyfleet-agent -m 0640 "${temporary_dir}/agent.yaml" "${config_path}"
fi

if grep -q 'hyfleet\.example\.com' "${config_path}"; then
  fail "${config_path} still contains the example hostname; rerun with --replace-config"
fi

install -o root -g root -m 0644 "${source_unit}" \
  /etc/systemd/system/hyfleet-agent.service

runuser -u hyfleet-agent -g hyfleet-agent -- /usr/local/bin/hyfleet-agent \
  -config "${config_path}" -check-config

configured_server="$(awk '$1 == "server_url:" { print $2; exit }' "${config_path}")"
[[ "${configured_server}" =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] ||
  fail "agent server_url must be a simple HTTPS origin"
curl --fail --silent --show-error "${configured_server}/healthz" >/dev/null ||
  fail "cannot reach ${configured_server}/healthz with trusted TLS"

environment_path="/etc/hyfleet/agent.env"
has_credential=false
if [[ -f "${state_path}" ]] &&
  grep -Eq '"node_credential"[[:space:]]*:[[:space:]]*"[^\"]+' "${state_path}"; then
  has_credential=true
fi

if [[ "${has_credential}" != true ]]; then
  printf 'Paste the unexpired one-time enrollment token, then press Enter: ' > /dev/tty
  IFS= read -r -s enrollment_token < /dev/tty
  printf '\n' > /dev/tty
  [[ -n "${enrollment_token}" && ${#enrollment_token} -le 256 ]] || fail "invalid enrollment token"
  printf 'HYFLEET_ENROLLMENT_TOKEN=%s\n' "${enrollment_token}" > "${temporary_dir}/agent.env"
  install -o root -g hyfleet-agent -m 0640 "${temporary_dir}/agent.env" "${environment_path}"
  unset enrollment_token
else
  rm -f -- "${environment_path}"
fi

systemctl daemon-reload
systemd-analyze verify /etc/systemd/system/hyfleet-agent.service
systemctl enable hyfleet-agent
systemctl restart hyfleet-agent

if [[ "${has_credential}" != true ]]; then
  for _ in {1..30}; do
    if [[ -f "${state_path}" ]] &&
      grep -Eq '"node_credential"[[:space:]]*:[[:space:]]*"[^\"]+' "${state_path}"; then
      has_credential=true
      break
    fi
    sleep 1
  done
fi

if [[ "${has_credential}" != true ]]; then
  systemctl stop hyfleet-agent || true
  journalctl -u hyfleet-agent -b -n 80 --no-pager || true
  fail "Agent enrollment did not complete; the service was stopped and the token file was retained"
fi

rm -f -- "${environment_path}"
systemctl restart hyfleet-agent
for _ in {1..10}; do
  if systemctl is-active --quiet hyfleet-agent; then
    break
  fi
  sleep 1
done
systemctl is-active --quiet hyfleet-agent || {
  journalctl -u hyfleet-agent -b -n 80 --no-pager || true
  fail "Agent did not remain active after removing the one-time token"
}

printf 'HyFleet Agent is enrolled and active. The one-time token file was removed.\n'
printf 'Confirm that the node becomes online in the HyFleet dashboard.\n'
