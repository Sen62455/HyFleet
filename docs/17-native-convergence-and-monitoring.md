# 原生节点收敛与主机监控

本阶段将代理核心统一收敛到原生 Hysteria2，同时在迁移期间保留旧的 S-UI 和独立
sing-box 适配器。HyFleet Agent 也将成为主机状态的数据来源；三台机器完成监控观察后，
即可移除 Beszel。

## 从 v1.0.0 或 v1.1.x 升级到 v1.1.3

升级顺序为 Server 在前、Agent 在后。升级程序会校验两层 SHA-256，在每台 VPS 的
`/var/lib/hyfleet-updates` 下保存旧二进制、systemd 单元、配置和标准数据文件；健康检查
失败时会自动回滚。

仓库设为 Private 后，不要让 VPS 使用未认证的 `curl` 直接下载 GitHub Release。在已经
通过 `gh auth login` 登录 GitHub 的 Windows 管理机上运行：

```powershell
gh auth status
gh release view v1.1.3 --repo Sen62455/HyFleet
.\scripts\deploy-fleet.ps1 -Version v1.1.3
```

`deploy-fleet.ps1` 会通过本机 GitHub 身份下载私有 Release、验证外层校验和，再并行上传
到 `scripts/fleet.local.psd1` 中登记的三台 VPS。远端解压后还会验证包内
`SHA256SUMS`，然后按各主机的 `Components` 设置更新 Server 或 Agent。

升级结束后检查：

```bash
sudo systemctl status hyfleet-server --no-pager --full
sudo systemctl status hyfleet-agent --no-pager --full
sudo journalctl -u hyfleet-server -n 100 --no-pager
sudo journalctl -u hyfleet-agent -n 100 --no-pager
```

DMIT 同时运行 Server 和 Agent，因此它的 `Components` 应保持 `@("server", "agent")`；
另外两台机器只需要 `@("agent")`。不要把 GitHub Token、SSH 私钥或节点凭据写入仓库。

如果不使用三机更新脚本，可以在本机通过 `gh release download` 下载相应架构的压缩包和
`.sha256`，上传至 VPS，解压并验证 `SHA256SUMS` 后，在解压目录执行：

```bash
sudo bash deploy/update-component.sh server
sudo bash deploy/update-component.sh agent
```

单机上仍然应先更新 Server，再更新 Agent。成功更新后，脚本会输出可用于人工回滚的快照
目录；确认稳定并完成异机备份前，不要删除该目录。

`v1.1.1` 修复了 `v1.1.0` Agent systemd 沙箱把 `/proc` 限制为仅进程目录的问题。
该限制会让主机指标采集报 `open /proc/stat: no such file or directory`，进而使三台节点都被
标记为离线，但不会停止 Hysteria2 或影响已有订阅。新单元允许只读访问主机级 `/proc`
指标，并在启动前验证所需文件可读；即使某次指标采集失败，Agent 仍会发送基础心跳。

`v1.1.2` 增加证书 SHA-256 指纹和公钥 SHA-256 Pin。自签名端点可在保持
`tls_insecure` 的同时，让 Hysteria2 URI、Mihomo/Clash 和 sing-box 校验固定的证书或
公钥。数据库迁移 `0009_tls_pins.sql` 会自动执行，原有节点的两个字段默认为空。

`v1.1.3` 默认只在用户详情中展示有效订阅 Token。已撤销和已到期的记录仍保留用于审计，
但收纳在按需展开的“历史 Token”中，避免长期使用后侧边栏不断增长。

## 修复 Agent 在线但 Hysteria2 超时

Agent 在线只代表控制面链路和主机监控正常，不代表客户端能访问公网 UDP 端点。节点刚从
sing-box 迁移到原生 Hysteria2 时，先在节点主机核对真实监听端口：

```bash
sudo systemctl status hysteria-server --no-pager --full
sudo awk '/^[[:space:]]*listen:/ { print }' /etc/hysteria/config.yaml
sudo ss -lunp | grep hysteria
sudo ufw status
```

HyFleet 中该节点的“UDP 端口”必须与 `ss` 显示的端口完全一致。防火墙需要放行 UDP，
放行同号 TCP 不能替代 UDP。若云厂商还有独立防火墙或安全组，也要同步放行该 UDP
端口。

自签名证书分别生成证书指纹和 sing-box 公钥 Pin：

```bash
sudo openssl x509 -noout -fingerprint -sha256 \
  -in /etc/hysteria/server.crt | cut -d= -f2

sudo openssl x509 -in /etc/hysteria/server.crt -pubkey -noout \
  | openssl pkey -pubin -outform der \
  | openssl dgst -sha256 -binary \
  | openssl enc -base64
```

这两个命令不会输出私钥。然后编辑原生节点：

1. “公网域名或 IP”填写客户端实际访问的地址；
2. “UDP 端口”填写当前 Hysteria2 的真实监听端口；
3. 没有基于 SNI 的服务端路由时，不要沿用旧 sing-box 的伪装 SNI；
4. 自签名证书启用“跳过证书验证”；
5. 将第一条命令结果填入“证书 SHA-256 指纹”；
6. 将第二条命令结果填入“公钥 SHA-256（Base64）”。

保存后等待节点配置版本重新一致，再刷新 Clash Provider。生成的 Clash 节点应包含正确的
`port`、`skip-cert-verify: true` 和 `fingerprint`。如果仍超时，在服务端执行以下命令并
同时触发客户端测速：

```bash
sudo journalctl -u hysteria-server --since '-5 minutes' --follow
```

完全没有新日志通常表示公网地址、UDP 端口或上游防火墙仍不匹配；出现 TLS 或鉴权日志则
按具体错误检查 Pin、SNI 或用户同步状态。

## 本阶段变化

- 新节点应使用 `native_hysteria2`。
- Agent 上报 CPU 核心数和使用率、内存、Swap、根分区、磁盘 I/O、系统负载、当前网络
  速率、网络累计流量、主机名、内核与运行时间。
- 保留 30 天的一分钟采样。为控制低内存 Server 的开销，每次 API 查询最多返回 360 个
  聚合点。
- 节点详情支持 1 小时、6 小时、24 小时、7 天和 30 天时间范围。
- 运维操作使用独立的筛选和分页历史页面；节点详情只显示最近 3 条操作。
- S-UI 和独立 sing-box 仍作为兼容适配器保留，不会被自动删除，并可用于迁移回滚。

## 为什么使用替换节点记录

节点注册后，其适配器类型、Agent 身份、期望状态历史和凭据会绑定在一起。直接修改现有
记录，可能让从 S-UI 导入的用户或只读 sing-box 端点看起来已由 HyFleet 接管，而 HyFleet
实际上并不持有它们的凭据材料。

稳妥做法是在同一台 VPS 上创建临时的原生替换节点，完成验证后归档旧记录，再把替换节点
改回原名称。

迁移过程中，已有订阅 URL 可以发生变化。最终节点分配正确后再创建或轮换订阅 Token，
撤销旧 Token 前先向使用者提供新 URL。

## 单台主机迁移流程

一次只迁移一台 VPS。优先从影响最小的主机开始，迁移期间始终保留另外两个可用节点。

1. 备份当前代理配置、证书文件、systemd 单元、防火墙规则和 HyFleet Server 数据库。
2. 记录当前公网主机、UDP 端口、证书/SNI 和全部已分配用户。不要把密码写入 Git 或
   Shell 历史。
3. 在旧服务旁安装原生 Hysteria2。旧服务占用生产端口时，验证阶段使用临时 UDP 端口。
4. 在 HyFleet 新建名为 `<旧名称>-native` 的节点，选择 `原生 Hysteria2`，填写临时公网
   端点。
5. 为新记录安装或重新配置 Agent：

   ```bash
   sudo bash install.sh agent --version <release> \
     --server-url https://panel.example.com \
     --node-name <旧名称>-native \
     --adapter native-hysteria2 \
     --core-config-path /etc/hysteria/config.yaml \
     --replace-config
   ```

   如果 `/var/lib/hyfleet-agent/agent-state.json` 中仍保留已注册的 Agent 身份，安装器会故意
   拒绝 `--replace-config`。应停止旧 Agent，将旧状态和配置移到备份目录，作为回滚依据，
   然后再注册替换节点。迁移验收完成前不要删除旧状态。

6. 启用 HyFleet HTTP 鉴权和回环流量统计：

   ```bash
   sudo bash deploy/configure-hysteria.sh
   ```

   此命令会创建带时间戳的 Hysteria 配置备份，在改写前探测本机鉴权端点，重启
   Hysteria，验证统计端点，并在验证失败时恢复旧配置。
7. 给替换节点分配一个测试用户，验证 HTTP 鉴权、URI、Clash 和 sing-box 订阅、流量
   增量、在线状态以及一次受控的核心重启。
8. 迁移其余用户。凭据值可以变化；只通过 HyFleet 查看或轮换凭据，需要时签发新的订阅
   Token。
9. 在维护窗口把原生服务切换到生产 UDP 端口，更新 HyFleet 公网端点并从外部验证。
10. 停止旧 S-UI 或 sing-box 服务，但暂时不要卸载；观察原生节点至少 24 小时。
11. 从旧节点解除用户分配，归档旧记录，再将替换节点改回原显示名称。

迁移 BandwagonHost 时，应保留完整 sing-box 配置目录和防火墙规则；切换生产端口前，
原生 Hysteria 服务不得监听与旧服务相同的 UDP 端口。

## DMIT 从 S-UI 迁移到原生 Hysteria2

DMIT 同时承载 HyFleet Server 和 Agent。整个迁移过程中不要停止
`hyfleet-server`，也不要移动或删除 `/etc/hyfleet`、`/var/lib/hyfleet/server.db` 和
`/var/lib/hyfleet/master.key`。只替换 Agent 身份和 Agent 本地状态。

先在已解压的 `v1.1.3` 发布目录中创建一致性备份，并保存 S-UI 与网络配置：

```bash
cd /root/hyfleet-v1.1.3-linux-amd64
sudo bash deploy/backup-server.sh --output-dir /root/hyfleet-server-backup

MIGRATION_DIR="/root/dmit-native-$(date -u +%Y%m%dT%H%M%SZ)"
sudo install -d -m 0700 "$MIGRATION_DIR"
sudo cp -a /etc/hyfleet "$MIGRATION_DIR/hyfleet-config"
sudo cp -a /var/lib/hyfleet-agent "$MIGRATION_DIR/hyfleet-agent-state"
sudo systemctl cat s-ui > "$MIGRATION_DIR/s-ui.service.txt"
sudo systemctl cat hyfleet-server > "$MIGRATION_DIR/hyfleet-server.service.txt"
sudo systemctl cat hyfleet-agent > "$MIGRATION_DIR/hyfleet-agent.service.txt"
sudo cp -a /usr/local/s-ui "$MIGRATION_DIR/s-ui" 2>/dev/null || true
sudo ufw status verbose > "$MIGRATION_DIR/ufw.txt"
sudo iptables-save > "$MIGRATION_DIR/iptables.rules"
```

把备份归档和 master key 分开复制到 DMIT 之外。另从 S-UI 导出入站和客户端信息，并记录
当前生产 UDP 端口、域名、证书和密钥路径。不要把客户端密码、Token 或私钥提交到 Git。

安装原生 Hysteria2 后，先选择一个空闲的临时 UDP 端口。若准备使用 UDP 443，要先确认
Caddy 或其他程序没有占用 HTTP/3：

```bash
sudo ss -lunp
sudo ss -lunp | grep ':443 ' || true
```

在 `/etc/hysteria/config.yaml` 中使用临时端口和现有可信证书。下例以 `24443` 为例，
必须改成配置文件中的实际端口。初次启动可使用临时随机
密码鉴权；注册新 Agent 后，`configure-hysteria.sh` 会把它替换为 HyFleet HTTP 鉴权，
并配置回环流量统计。确认服务只监听临时端口：

```bash
sudo systemctl enable --now hysteria-server
sudo systemctl status hysteria-server --no-pager --full
sudo ss -lunp | grep hysteria
HY2_TEMP_PORT=24443
sudo ufw allow "${HY2_TEMP_PORT}/udp"
```

在 HyFleet 新建 `DMIT-native`，选择“原生 Hysteria2”，填写临时公网端点与正确 TLS
设置，并生成一次性 Agent 注册 Token。然后只移走旧 Agent 文件；以下命令不会改动 Server
数据库、Server 配置或 master key：

```bash
sudo systemctl stop hyfleet-agent hyfleet-agent-ops.socket
sudo mv /etc/hyfleet/agent.yaml "$MIGRATION_DIR/agent.yaml"
sudo mv /etc/hyfleet/agent.env "$MIGRATION_DIR/agent.env" 2>/dev/null || true
sudo mv /var/lib/hyfleet-agent "$MIGRATION_DIR/hyfleet-agent-old"

cd /root/hyfleet-v1.1.3-linux-amd64
sudo bash deploy/install-agent.sh \
  --server-url https://panel.example.com \
  --node-name DMIT-native \
  --adapter native-hysteria2 \
  --core-config-path /etc/hysteria/config.yaml \
  --replace-config
sudo bash deploy/configure-hysteria.sh
```

安装器会在终端安全提示中要求粘贴一次性 Token。将 `panel.example.com` 替换为实际控制面
域名，但不要把 Token 写在命令参数或 Shell 历史里。随后确认控制面仍健康：

```bash
sudo systemctl is-active hyfleet-server hyfleet-agent hysteria-server
curl --fail --show-error --silent http://127.0.0.1:8080/healthz
sudo journalctl -u hyfleet-agent -n 100 --no-pager
```

先给 `DMIT-native` 分配一个测试用户，在临时端口验收鉴权、Clash 订阅、流量、在线状态、
核心重启和配置备份。通过后再迁移其余用户。维护窗口中停止 S-UI，确认生产 UDP 端口已
释放，只修改 Hysteria 的 `listen`，重启后同步更新 HyFleet 节点端口：

```bash
sudo systemctl stop s-ui
sudo ss -lunp
sudoedit /etc/hysteria/config.yaml
sudo systemctl restart hysteria-server
sudo systemctl status hysteria-server --no-pager --full
sudo ss -lunp | grep hysteria
```

再次从外部验证 DMIT 订阅节点。观察至少 24 小时后，再解除旧 `DMIT` 节点的用户分配并
归档旧记录；S-UI 在观察期只停止、不卸载。若验证失败，停止原生 Hysteria，恢复备份的
Agent 文件和 `/var/lib/hyfleet-agent`，启动 `s-ui` 与旧 Agent，即可回滚代理管理链路，
HyFleet Server 数据不需要回滚。

## 删除旧面板

三台节点都至少积累 24 小时 HyFleet 指标后，才能删除 Beszel。应分别使用 `free`、`df`、
`uptime`、`/proc/net/dev` 和 `/proc/diskstats` 对照数据是否合理。

只有满足以下条件后，才能删除 S-UI 或旧 sing-box 部署：

- 所有预期用户都能通过原生 HTTP 鉴权；
- 订阅输出包含预期的三个原生节点；
- 流量和在线用户状态持续更新；
- 重启、有限日志和配置备份操作均能完成；
- 没有活动订阅继续引用旧节点；
- 已检查或实际测试保存的回滚配置。

## 迁移回滚

如果验证失败，停止替换用的原生服务，恢复保存的 Agent 状态、Agent 配置和旧代理配置，
启动旧服务，并把用户重新分配到旧节点记录。原生节点完成观察前，不要归档旧记录或撤销
旧订阅 Token。

## 三台节点同时离线的排查

`重新同步` 只会生成新的期望状态快照，不能修复 Agent 传输、鉴权、已停止的 Agent 服务，
也不能解除 Server 数据库阻塞。

本阶段将 Server 从单个 SQLite 连接改为包含 4 个连接的 WAL 池，并为后台维护任务增加
超时边界。操作历史和配置备份查询也会在检查“空列表是否属于现有节点”之前关闭结果游标。

旧实现中，任意一个空列表查询都可能占用唯一的 SQLite 连接，又等待自己占用的连接。
旧节点抽屉会同时请求这两个列表，因此可能阻塞全部心跳。现在 Agent 运维操作也与心跳
调度分离，缓慢的核心重启只会影响该节点的运维工作线程。

升级 Server 和 Agent 后，如果再次发生三台节点同时离线，应首先按共享链路故障处理，
优先检查控制面：

```bash
sudo systemctl status hyfleet-server --no-pager --full
sudo journalctl -u hyfleet-server --since '-10 minutes' --no-pager
curl --fail --show-error https://panel.example.com/healthz
```

然后在不更改状态的情况下检查每台 Agent：

```bash
sudo systemctl status hyfleet-agent --no-pager --full
sudo journalctl -u hyfleet-agent --since '-10 minutes' --no-pager
sudo bash deploy/diagnose.sh agent
```

根据请求 ID 和时间戳关联日志。单台机器重启核心时，只应影响该节点的代理连接，不应影响
其他主机的心跳。
