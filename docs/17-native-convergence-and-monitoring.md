# 原生节点收敛与主机监控

本阶段将代理核心统一收敛到原生 Hysteria2，同时在迁移期间保留旧的 S-UI 和独立
sing-box 适配器。HyFleet Agent 也将成为主机状态的数据来源；三台机器完成监控观察后，
即可移除 Beszel。

## 从 v1.0.0 升级到 v1.1.0

升级顺序为 Server 在前、Agent 在后。升级程序会校验两层 SHA-256，在每台 VPS 的
`/var/lib/hyfleet-updates` 下保存旧二进制、systemd 单元、配置和标准数据文件；健康检查
失败时会自动回滚。

仓库设为 Private 后，不要让 VPS 使用未认证的 `curl` 直接下载 GitHub Release。在已经
通过 `gh auth login` 登录 GitHub 的 Windows 管理机上运行：

```powershell
gh auth status
gh release view v1.1.0 --repo Sen62455/HyFleet
.\scripts\deploy-fleet.ps1 -Version v1.1.0
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

迁移 DMIT 时，应在第 3 步前导出 S-UI 入站和客户端信息；观察期内保持 S-UI 已停止但仍
安装。迁移 BandwagonHost 时，应保留完整 sing-box 配置目录和防火墙规则；切换生产端口
前，原生 Hysteria 服务不得监听与旧服务相同的 UDP 端口。

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
