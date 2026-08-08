# 阶段 3：流量、在线状态与三机更新

## 交付范围

阶段 3 为原生 Hysteria2 Adapter 增加：

- Agent 本地 SQLite 流量基线与持久 Outbox；
- 控制面批次 ID 和来源序列双重幂等入账；
- 用户全局额度、节点分配额度以及命中后的鉴权阻断；
- 在线用户快照、单节点与全局踢线；
- HY2 统计接口健康、待上报批次和未归属流量观察；
- GitHub Release 自动构建与三台 VPS 一条命令更新。

`native_hysteria2` 使用 Hysteria 自带的 `/traffic`、`/online` 和 `/kick`。
`s_ui` 与 `standalone_sing_box` 在各自 Adapter 阶段接入用量接口；阶段 3 不会把主机网卡流量
错误地当作用户流量。

## 计量语义

接口格式以 [Hysteria 2 Traffic Stats API](https://v2.hysteria.network/docs/advanced/Traffic-Stats-API/)
为准：

- Hysteria 的 `tx` 记为用户上传，`rx` 记为用户下载。
- Hysteria `/online` 的数值是客户端实例（设备）数量，不是活跃代理流数量。
- 全局已用量是用户在所有节点的上传与下载之和。
- 单节点已用量是该用户在对应分配上的上传与下载之和。
- 额度为 `0` 时不限额；已用量大于或等于额度时立即限制。
- 任一额度命中都会生成单调递增的踢线 generation，并和新快照在同一事务提交。
- Agent 第一次启用统计时只建立基线，不追溯计入 HY2 进程此前的累计流量。
- Agent 不调用 `/traffic?clear=1`。HY2 重启导致计数器归零时，Agent 轮换来源 epoch。
- 未知、已归档或未分配的用户流量进入审计明细和节点未归属总量，不计入用户额度。

控制面收到重复 batch ID 会返回 `duplicate`；同一来源 epoch 和 sequence 的不同负载会被拒绝。
Agent 只有收到 `accepted` 或 `duplicate` 后才删除本地 Outbox，因此控制面暂时离线不会丢失用量。

## 在 LisaHost 启用统计接口

先更新 Agent，再进入同版本发布包的解压目录执行：

```bash
sudo bash deploy/configure-hysteria.sh
```

该命令会：

1. 在 `/etc/hyfleet/hy2-stats.env` 生成仅 root 和 `hyfleet-agent` 组可读的随机密钥；
2. 保留原始 `/etc/hysteria/config.yaml` 备份；
3. 将 HTTP 鉴权和 `trafficStats` 写为两个仅回环监听的接口；
4. 重启 HY2，等待服务稳定，并用本地密钥请求 `/traffic`；
5. 检查失败时自动恢复旧配置并再次启动 HY2；
6. 重启 Agent，使其开始采样。

最终配置的结构如下；实际密钥不要输出或提交：

```yaml
auth:
  type: http
  http:
    url: http://127.0.0.1:18081/hysteria/auth
trafficStats:
  listen: 127.0.0.1:18082
  secret: <local-random-secret>
```

验证时不要显示环境文件内容：

```bash
sudo systemctl is-active hysteria-server hyfleet-agent
sudo stat -c '%a %U:%G %n' /etc/hyfleet/hy2-stats.env
sudo journalctl -u hyfleet-agent -b -n 50 --no-pager
```

权限应为 `640 root:hyfleet-agent`。面板中的 LisaHost 应显示“统计正常”，首次采样后
待上报批次应回到 `0`。

## GitHub Release

`.github/workflows/release.yml` 在推送 `v*` tag 后执行完整 Go、Vue 和 shell 检查，构建
Linux `amd64` 与 `arm64` 两套包，并发布到当前私有仓库的 Release。开发版示例：

```powershell
git tag v0.3.0-dev
git push origin v0.3.0-dev
gh release view v0.3.0-dev --repo Sen62455/HyFleet
```

Release 中每个架构都有 `.tar.gz` 和对应的 `.tar.gz.sha256`。压缩包和校验文件必须来自
同一版本；不能混用 `v0.1.0` 压缩包和 `v0.2.0` 校验文件。

## 三台 VPS 一条命令更新

本机的 `scripts/fleet.local.psd1` 已被 Git 忽略。只需首次填写三台机器的 SSH Target；
DMIT 的 Components 保持 `server, agent`，另外两台保持 `agent`。SSH 主机密钥应事先进入
本机 `known_hosts`，私钥可以通过可选的 `IdentityFile` 字段指定。

本机需已完成：

```powershell
gh auth status
ssh root@DMIT_IP true
ssh root@BANDWAGONHOST_IP true
ssh root@LISAHOST_IP true
```

之后每次更新只运行：

```powershell
.\scripts\deploy-fleet.ps1 -Version v0.3.0-dev
```

脚本会从私有 GitHub Release 下载指定版本、校验本地文件、并行上传三台 VPS、再次进行远端
校验、执行包内 `SHA256SUMS` 与 `bash -n`，最后更新对应组件。Server 必须通过本机
`/healthz`；Agent 必须持续为 `active`。某个组件失败时，该机器会恢复更新前的二进制和
systemd 单元并重启旧版本，其余机器的结果会在命令结束时统一报告。

VPS 不连接 GitHub，也不保存 GitHub token。若要回退，使用同一命令部署一个仍保留在
Release 中的旧版本。

## 阶段验收

- 相同流量 batch 重放不会增加累计值；冲突序列不会入账。
- 控制面断开时 Agent Outbox 保留，恢复后只入账一次。
- HY2 重启后新流量继续累加，不出现负数或重复旧累计值。
- 单节点额度只限制对应分配；全局额度限制用户的全部分配。
- 禁用、到期、归档、取消分配和额度命中都会持久生成踢线 generation。
- 手动单节点或全局踢线成功后，符合鉴权条件的用户仍可重新连接。
- 旧在线快照不能覆盖较新的状态。
- 桌面与移动端均可查看流量、在线数、额度、统计健康和 Outbox。
- 三机更新任一健康检查失败时，该组件自动恢复旧版本。
