# 原生节点切换与旧核心退役 Runbook

本文用于已经在临时 UDP 端口验证 HyFleet 原生 Hysteria2 的节点：先停止旧核心，确认
UDP 443 空闲，再把原生 Hysteria2 切换到 UDP 443，更新 HyFleet 公网端口和客户端订阅，
经过观察期后清理旧服务。所有地址、端口和路径都必须以目标主机的实际盘点结果为准。

切换和删除是两个维护窗口。第一次只停止旧服务并切换端口；至少观察 24 小时且确认异机
备份可用后，第二次才删除旧 unit、配置和二进制。停止进程会立即释放内存，删除文件只会
释放对应文件占用的磁盘；`df` 使用率高不代表旧代理核心就是主要原因。

## 1. 安全边界与占位符

先在 SSH 会话中设置并检查变量。下列值是占位符，不能原样执行：

```bash
OLD_UNIT='REPLACE_OLD_SERVICE.service'
NEW_UNIT='hysteria-server.service'
NEW_CONFIG='/etc/hysteria/config.yaml'
TEMP_PORT='REPLACE_TEMP_UDP_PORT'
OLD_PORT='REPLACE_OLD_CORE_UDP_PORT'
TARGET_PORT='443'
OLD_CONFIG_ROOT='/etc/sing-box'
```

变量必须满足：

```bash
[[ "${OLD_UNIT}" =~ ^[A-Za-z0-9_.@-]+\.service$ ]] || exit 1
[[ "${NEW_UNIT}" == 'hysteria-server.service' ]] || exit 1
[[ "${NEW_CONFIG}" == '/etc/hysteria/config.yaml' ]] || exit 1
[[ "${TEMP_PORT}" =~ ^[0-9]+$ ]] || exit 1
[[ "${OLD_PORT}" =~ ^[0-9]+$ ]] || exit 1
[[ "${TARGET_PORT}" == '443' ]] || exit 1
[[ "$(readlink -e -- "${OLD_CONFIG_ROOT}")" == '/etc/sing-box' ]] || exit 1
```

最后一项只适用于旧 sing-box 确实位于 `/etc/sing-box` 的主机。其他布局不要修改该判断
后直接套用删除命令，应先按本文盘点真实边界。整个流程不要在命令行、截图、工单或 Git
中写入用户密码、Agent Token、订阅 URL、证书私钥或未脱敏配置。

必须使用独立于代理节点的 SSH 管理连接。建议保留第二个 SSH 会话，用于在第一个会话中断
时回滚。云厂商控制台、防火墙和 DNS 也应可以独立访问。

## 2. 切换前完成标准

开始维护窗口前必须同时满足：

- 原生节点在临时 UDP 端口显示在线，期望版本和已应用版本一致。
- 至少一个测试用户通过真实外部网络完成连接、流量统计、在线状态和统一订阅验收。
- 原生 Hysteria2 的证书名称与公网地址匹配，证书和私钥不依赖即将删除的旧目录。
- 旧节点和旧客户端仍有可验证的回滚配置；没有只存在于浏览器中的未保存密码。
- Server 数据库与 master key 已分别备份到加密异机存储。
- 维护窗口允许短暂断连；客户端订阅可以立即刷新。

如果 Hysteria2 配置中的证书或密钥仍位于 `/etc/sing-box`，先复制到
`/etc/hysteria/certs` 等受控目录、收紧权限、更新配置并在临时端口重新验收。否则删除旧
目录会让新核心下次重启失败。

## 3. 只读盘点旧服务

先记录服务、监听、资源和文件布局，不要立即卸载：

```bash
sudo systemctl status "${OLD_UNIT}" --no-pager --full
sudo systemctl cat "${OLD_UNIT}"
sudo systemctl show "${OLD_UNIT}" \
  -p FragmentPath -p DropInPaths -p ExecStart -p EnvironmentFiles
sudo systemctl status "${NEW_UNIT}" --no-pager --full
sudo ss -luntp
free -h
df -h /
sudo du -xhd1 "${OLD_CONFIG_ROOT}" 2>/dev/null
sudo find "${OLD_CONFIG_ROOT}" -xdev -maxdepth 3 \
  -printf '%y %m %u:%g %s %p\n' 2>/dev/null | sort
```

检查旧目录是否是挂载点、是否含符号链接，以及是否被其他 unit 引用：

```bash
sudo findmnt -R "${OLD_CONFIG_ROOT}" || true
sudo find "${OLD_CONFIG_ROOT}" -xdev -type l -printf '%p -> %l\n'
sudo grep -RFl -- "${OLD_CONFIG_ROOT}" \
  /etc/systemd/system /usr/lib/systemd/system /lib/systemd/system \
  2>/dev/null || true
sudo grep -F -- "${OLD_CONFIG_ROOT}" \
  /etc/hysteria/config.yaml /etc/hyfleet/agent.yaml 2>/dev/null || true
```

`systemctl cat` 中的 `ExecStart` 才是旧核心真实二进制和配置入口。不要根据进程名称猜路径。
对 unit、二进制和配置入口逐个检查是否由 Debian 包拥有：

```bash
dpkg-query -S /path/from/FragmentPath 2>/dev/null || true
dpkg-query -S /path/from/ExecStart 2>/dev/null || true
```

由包管理器拥有的文件应通过对应包卸载，不能手工删除 `/usr/bin` 或 `/usr/lib/systemd`
中的单个文件。`apt-get -s purge PACKAGE_NAME` 只做预演；确认它不会移除 Hysteria2、HyFleet、
反向代理或其他共享组件后，才可以在最终清理窗口执行真实 purge。

## 4. 创建可验证备份

在当前工作目录创建 root-only 备份。若 Server 与节点共机，还要执行 Server 一致性备份：

```bash
umask 077
BACKUP_DIR="${PWD}/hyfleet-cutover-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p -- "${BACKUP_DIR}"

sudo systemctl cat "${OLD_UNIT}" > "${BACKUP_DIR}/old-core.service.txt"
sudo systemctl cat "${NEW_UNIT}" > "${BACKUP_DIR}/hysteria.service.txt"
sudo cp -a -- "${NEW_CONFIG}" "${BACKUP_DIR}/hysteria-config.pre-cutover.yaml"
sudo cp -a -- /etc/hyfleet "${BACKUP_DIR}/hyfleet-config"
sudo tar --xattrs --acls --numeric-owner -czf \
  "${BACKUP_DIR}/old-core-files.tar.gz" "${OLD_CONFIG_ROOT}"
sudo ufw status verbose > "${BACKUP_DIR}/ufw.txt" 2>&1 || true
sudo iptables-save > "${BACKUP_DIR}/iptables.rules" 2>/dev/null || true
sudo nft list ruleset > "${BACKUP_DIR}/nftables.rules" 2>/dev/null || true
```

若旧核心是手工安装，还要备份真实 unit、drop-in 和二进制。先把盘点出的绝对路径填入
占位符；二进制只接受常见的手工安装目录：

```bash
OLD_UNIT_FILE='/etc/systemd/system/REPLACE_OLD_SERVICE.service'
OLD_DROPIN_DIR='/etc/systemd/system/REPLACE_OLD_SERVICE.service.d'
OLD_BINARY='/REPLACE/ABSOLUTE/PATH/TO/sing-box'

case "${OLD_UNIT_FILE}" in
  /etc/systemd/system/*.service|/usr/local/lib/systemd/system/*.service) ;;
  *) exit 1 ;;
esac
case "${OLD_BINARY}" in
  /usr/local/bin/*|/usr/local/sbin/*|/etc/sing-box/*) ;;
  *) exit 1 ;;
esac
[[ "${OLD_UNIT_FILE}" == "$(realpath -m -- "${OLD_UNIT_FILE}")" ]] || exit 1
[[ "${OLD_DROPIN_DIR}" == "$(realpath -m -- "${OLD_DROPIN_DIR}")" ]] || exit 1
[[ "${OLD_BINARY}" == "$(realpath -m -- "${OLD_BINARY}")" ]] || exit 1
[[ -f "${OLD_UNIT_FILE}" && ! -L "${OLD_UNIT_FILE}" ]] || exit 1
[[ -f "${OLD_BINARY}" && ! -L "${OLD_BINARY}" ]] || exit 1
if dpkg-query -S "${OLD_UNIT_FILE}" >/dev/null 2>&1; then exit 1; fi
if dpkg-query -S "${OLD_BINARY}" >/dev/null 2>&1; then exit 1; fi

MANUAL_FILES=("${OLD_UNIT_FILE}" "${OLD_BINARY}")
if [[ -d "${OLD_DROPIN_DIR}" && ! -L "${OLD_DROPIN_DIR}" ]]; then
  MANUAL_FILES+=("${OLD_DROPIN_DIR}")
fi
sudo tar --xattrs --acls --numeric-owner -czf \
  "${BACKUP_DIR}/old-manual-install.tar.gz" "${MANUAL_FILES[@]}"
```

包管理安装不执行上面的手工文件归档，恢复时使用同版本包。最后为实际生成的归档建立相对
路径校验清单：

```bash
CHECKSUM_FILES=(old-core-files.tar.gz hysteria-config.pre-cutover.yaml)
if [[ -f "${BACKUP_DIR}/old-manual-install.tar.gz" ]]; then
  CHECKSUM_FILES+=(old-manual-install.tar.gz)
fi
(cd "${BACKUP_DIR}" && sudo sha256sum "${CHECKSUM_FILES[@]}" > SHA256SUMS)
```

检查归档而不解压覆盖当前文件：

```bash
(cd "${BACKUP_DIR}" && sudo sha256sum -c SHA256SUMS)
sudo tar -tzf "${BACKUP_DIR}/old-core-files.tar.gz" | sed -n '1,120p'
sudo du -sh "${BACKUP_DIR}"
```

本流程不替换 Agent 身份，不要移动、删除或直接复制正在写入的
`/var/lib/hyfleet-agent` SQLite 状态。若另有节点重装计划，应在维护窗口停止 Agent 后按
对应版本的备份说明处理 Outbox 和本地状态，并在恢复鉴权服务后再继续端口切换。

将备份复制到另一台受控机器并再次核对 SHA-256。归档可能含代理密码、证书和 Token，必须
加密保存，不得作为 GitHub Release、Issue 附件或普通聊天文件。

Server 与节点共机时，在已验证 Release 目录额外执行：

```bash
sudo bash deploy/backup-server.sh --output-dir /var/backups/hyfleet
```

确认数据库归档和 master key 的两个校验文件都通过，并将归档和 key 分开保存。

## 5. 放行目标端口并停止旧核心

先放行 UDP 443。TCP 443 可以继续由面板反向代理使用，两种传输协议不会互相占用；但
Caddy HTTP/3、其他 QUIC 服务或另一个代理可能已占用 UDP 443。

```bash
sudo ufw allow 443/udp
sudo ufw status verbose
```

若云厂商另有安全组或外部防火墙，也应先放行 UDP 443。随后停止并禁用旧服务：

```bash
sudo systemctl disable --now "${OLD_UNIT}"
sudo systemctl is-active "${OLD_UNIT}" || true
sudo systemctl is-enabled "${OLD_UNIT}" || true
sudo ss -H -lunp | grep -E ':(443)([[:space:]]|$)' || true
```

此时 `OLD_UNIT` 应为 `inactive` 和 `disabled`，UDP 443 查询必须没有监听者。若仍有输出，
用 `ss` 显示的进程定位占用者；不要停止未知服务，也不要继续修改 Hysteria2。只检查 UDP
监听，TCP 443 的 Nginx 或 Caddy 不构成冲突。

停止后记录内存变化，确认删除磁盘前的真实收益：

```bash
free -h
df -h /
sudo systemd-cgtop --depth=2 --iterations=1
```

## 6. 将原生 Hysteria2 切换到 UDP 443

保留 `auth`、`trafficStats`、TLS、带宽和其他已验收配置，只把 `listen` 从临时端口改为
`:443`：

```bash
sudoedit /etc/hysteria/config.yaml
sudo awk '/^[[:space:]]*listen:/ { print }' /etc/hysteria/config.yaml
sudo systemctl restart hysteria-server.service
sudo systemctl is-active hysteria-server.service
sudo systemctl status hysteria-server.service --no-pager --full
sudo ss -H -lunp | grep -E ':(443)([[:space:]]|$)'
sudo journalctl -u hysteria-server.service --since '-5 minutes' --no-pager
```

必须看到 Hysteria2 监听 UDP 443，日志没有配置、证书、鉴权端点或端口冲突错误。不要在
日志命令后附带会输出完整配置或密码的调试参数。

如果重启失败，立即执行“回滚”一节，不要一边修改更多配置一边尝试恢复。

## 7. 更新 HyFleet 与客户端订阅

在 HyFleet 管理界面编辑当前原生节点，只把“UDP 端口”从临时端口改为 `443`，公网主机、
TLS 校验、SNI、证书 Pin 和用户分配保持不变。保存后等待：

- Agent 与核心保持在线；
- 节点期望版本与已应用版本一致；
- 用户分配仍显示可纳入订阅；
- 没有新的同步、核心或节点离线告警。

使用现有订阅 URL 主动刷新 Provider；必要时重新导入同一订阅 URL。确认生成的节点端口为
443，但不要把包含凭据的订阅正文粘贴到日志、Issue 或聊天中。从 VPS 外部依次验收：

1. 真实客户端延迟测试和网页访问；
2. 规则模式和全局模式；
3. HyFleet 在线用户变化；
4. 上传、下载计数持续增加；
5. Agent 和 Hysteria2 各重启一次后自动恢复；
6. 有限日志、配置备份和一次正常核心重启。

客户端延迟测试是小样本，可能受 QUIC 会话复用、DNS 缓存、测试顺序和瞬时丢包影响。
同一 VPS 上两个节点记录出现几十毫秒差异不等于路由发生变化；应统一地址、端口和 TLS
设置，交替多次测试并比较中位数。

## 8. 观察期

旧服务保持 `disabled`，但暂时保留 unit、二进制和配置。建议观察 24 至 72 小时，并至少
覆盖一次 Server、Agent 和 Hysteria2 重启。观察以下项目：

- 所有预期用户只通过原生节点连接，旧节点没有活动订阅引用。
- Hysteria2 始终监听 UDP 443，Agent 心跳和主机指标连续。
- 流量没有计数回退或异常倍增，在线状态可恢复。
- 内存、Swap、根分区和主要进程占用合理。
- 没有核心、同步、额度或运维失败告警。
- 异机备份可以列出文件，校验和仍通过，回滚步骤已经过书面核对。

观察通过后，在 HyFleet 中解除旧节点的用户分配并归档旧节点记录。归档节点记录与删除
主机文件是两件事；确认订阅只输出当前原生记录后再做最终清理。

## 9. 最终清理旧 sing-box

### 9.1 目录型安装的删除条件

旧 sing-box 常把 `conf/`、证书、辅助程序、日志、数据库和订阅文件放在同一
`/etc/sing-box` 目录。只有以下条件全部满足，整个目录才可删除：

- `OLD_UNIT` 已停止、禁用并完成观察期；没有 sing-box 进程或旧 UDP 监听。
- `systemctl show` 已记录唯一的 FragmentPath、DropInPaths 和 ExecStart。
- `/etc/sing-box` 不是独立挂载点，内部没有仍需保留或指向外部的符号链接。
- Hysteria2、HyFleet、证书续期、Cloudflare 辅助服务和其他 systemd unit 都不引用该目录。
- 目录内的证书、脚本、数据库或订阅文件没有被其他服务共享。
- 整个目录的加密异机归档和 SHA-256 已验证。
- 包管理器归属已查清；package-owned 文件将通过 package manager 删除。

特别检查目录内的 `cloudflared`、证书和定时任务。即使它们位于 sing-box 目录，也可能由
另一个 tunnel、续期 hook 或 systemd timer 使用。存在引用时只删除确认属于旧核心的文件，
不能删除整个目录。

### 9.2 包管理安装

先预演并阅读将被删除的包：

```bash
sudo apt-get -s purge PACKAGE_NAME
```

确认预演只涉及旧 sing-box 包后，才执行：

```bash
sudo apt-get purge PACKAGE_NAME
sudo systemctl daemon-reload
```

`PACKAGE_NAME` 必须来自 `dpkg-query -S` 或 `dpkg-query -W`，不能根据二进制名称猜测。

### 9.3 手工安装

把 `systemctl show` 和 `ExecStart` 得到的真实路径逐一填入下列占位符：

```bash
OLD_UNIT_FILE='/etc/systemd/system/REPLACE_OLD_SERVICE.service'
OLD_DROPIN_DIR='/etc/systemd/system/REPLACE_OLD_SERVICE.service.d'
OLD_BINARY='/REPLACE/ABSOLUTE/PATH/TO/sing-box'
```

先验证它们是绝对路径、不是符号链接，并且不由 Debian 包拥有：

```bash
case "${OLD_UNIT_FILE}" in
  /etc/systemd/system/*.service|/usr/local/lib/systemd/system/*.service) ;;
  *) exit 1 ;;
esac
case "${OLD_DROPIN_DIR}" in
  /etc/systemd/system/*.service.d|/usr/local/lib/systemd/system/*.service.d) ;;
  *) exit 1 ;;
esac
case "${OLD_BINARY}" in
  /usr/local/bin/*|/usr/local/sbin/*|/etc/sing-box/*) ;;
  *) exit 1 ;;
esac
[[ "${OLD_UNIT_FILE}" == "$(realpath -m -- "${OLD_UNIT_FILE}")" ]] || exit 1
[[ "${OLD_DROPIN_DIR}" == "$(realpath -m -- "${OLD_DROPIN_DIR}")" ]] || exit 1
[[ "${OLD_BINARY}" == "$(realpath -m -- "${OLD_BINARY}")" ]] || exit 1
[[ -f "${OLD_BINARY}" && ! -L "${OLD_BINARY}" ]] || exit 1
if dpkg-query -S "${OLD_UNIT_FILE}" >/dev/null 2>&1; then
  exit 1
fi
if dpkg-query -S "${OLD_BINARY}" >/dev/null 2>&1; then
  exit 1
fi
sudo ls -la -- "${OLD_UNIT_FILE}" "${OLD_DROPIN_DIR}" "${OLD_BINARY}" 2>/dev/null
```

确认显示的三个对象正是备份中的旧服务文件后，删除单个 unit、drop-in 和二进制：

```bash
sudo rm -f -- "${OLD_UNIT_FILE}"
if [[ -d "${OLD_DROPIN_DIR}" && ! -L "${OLD_DROPIN_DIR}" ]]; then
  sudo rm -rf --one-file-system -- "${OLD_DROPIN_DIR}"
fi
sudo rm -f -- "${OLD_BINARY}"
sudo systemctl daemon-reload
sudo systemctl reset-failed "${OLD_UNIT}" || true
```

只有 9.1 的所有条件都满足，且真实配置根目录恰好为 `/etc/sing-box`，才执行目录删除：

```bash
[[ "$(readlink -e -- /etc/sing-box)" == '/etc/sing-box' ]] || exit 1
sudo findmnt -R /etc/sing-box && exit 1 || true
sudo grep -RFl -- '/etc/sing-box' \
  /etc/systemd/system /usr/lib/systemd/system /lib/systemd/system \
  2>/dev/null && exit 1 || true
sudo grep -F -- '/etc/sing-box' \
  /etc/hysteria/config.yaml /etc/hyfleet/agent.yaml 2>/dev/null && exit 1 || true
sudo find /etc/sing-box -xdev -maxdepth 3 -printf '%y %s %p\n' | sort
```

最后一次人工检查列表和异机归档后，使用经过验证的字面量路径：

```bash
sudo rm -rf --one-file-system -- /etc/sing-box
```

不要把上述命令改成针对 `/etc`、`/usr/local`、`/root`、空变量、通配符或命令替换结果的
递归删除。若目录不是精确的 `/etc/sing-box`，应重新制定逐文件清单。

## 10. 清理旧 UDP 规则并复核收益

UDP 443 是新生产端口，必须保留。只删除已确认属于临时原生端口或退役旧核心端口的规则：

```bash
sudo ufw status numbered
sudo ufw delete allow "${TEMP_PORT}/udp"
if [[ "${OLD_PORT}" != '443' && "${OLD_PORT}" != "${TEMP_PORT}" ]]; then
  sudo ufw delete allow "${OLD_PORT}/udp"
fi
sudo ufw status verbose
```

如果 UFW 报规则不存在，先重新查看编号，不要为了“清理干净”删除含义不明的规则。云安全
组和外部防火墙也只移除明确的旧 UDP 端口，保留 SSH、面板 HTTPS 和 UDP 443。

最终检查：

```bash
sudo systemctl status hysteria-server.service --no-pager --full
sudo systemctl status hyfleet-agent.service --no-pager --full
sudo ss -H -lunp | grep -E ':(443)([[:space:]]|$)'
free -h
df -h /
sudo du -xhd1 /etc /var /usr/local 2>/dev/null | sort -h
```

对比切换前后的 `free`、`df` 和 `du`。如果磁盘使用率仍高，继续对 `/var/log`、容器镜像、
包缓存和应用数据做只读盘点，不能假设删除更多 HyFleet 或系统文件会解决问题。

## 11. 回滚

### 端口切换失败且旧文件仍在

先停止新核心，恢复切换前 Hysteria2 配置，再启动旧核心：

```bash
sudo systemctl stop hysteria-server.service
sudo cp -a -- "${BACKUP_DIR}/hysteria-config.pre-cutover.yaml" \
  /etc/hysteria/config.yaml
sudo systemctl start "${OLD_UNIT}"
sudo systemctl status "${OLD_UNIT}" --no-pager --full
sudo ss -luntp
```

确认旧生产节点恢复后，可重新启动临时端口的 Hysteria2：

```bash
sudo systemctl start hysteria-server.service
sudo systemctl status hysteria-server.service --no-pager --full
```

在 HyFleet 中把原生节点公网 UDP 端口改回临时端口，等待同步，并刷新订阅。若旧防火墙规则
已移除，按备份记录恢复对应 UDP 规则。不要让旧核心和新核心同时监听同一 UDP 端口。

### 最终删除后回滚

最终删除后必须从经过验证的异机归档恢复。包管理安装先安装原版本包；手工安装则按备份
清单恢复 unit、drop-in、二进制和配置的原属主与权限，执行 `systemctl daemon-reload`，
再启动旧服务。恢复前先停止占用旧生产端口的 Hysteria2。

如果归档没有在另一台机器验证过，或恢复步骤依赖已经删除的证书、脚本或包版本，则尚未
满足最终清理条件，应继续保留已停用的旧文件。
