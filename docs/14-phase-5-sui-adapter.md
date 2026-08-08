# 阶段 5：S-UI 适配器与 DMIT 上线

## 交付范围

`v0.5.0-dev` 增加：

- S-UI `/apiv2` 兼容性、版本与 sing-box 状态探测；
- Hysteria2 入站和脱敏客户端发现；
- 现有 S-UI 客户端到全局用户的只读映射；
- 等待 Agent 确认后的显式接管；
- 受管客户端创建、修改、启用、停用、到期、凭据轮换和删除；
- S-UI 在线状态与累计流量接入 Agent Outbox；
- S-UI 节点加入统一 Hysteria2 订阅；
- 节点详情中的探测、目标入站、映射和接管界面；
- Clash Meta 默认选择组和 `MATCH` 规则，修复完整配置在规则模式下不走代理的问题。

阶段五不会管理独立 `standalone_sing_box` 用户，也不会提供任意 SSH 命令、S-UI
数据库直写或全量配置导入。

## 兼容范围

当前适配器针对 S-UI `v1.5.3` 的 `/apiv2` 契约实现，接受版本范围为：

```text
>= v1.5.3 且 < v1.6.0
```

低于该范围会显示 `sui_version_unsupported`。未来版本即使 API 看似可用，也不会自动
越过兼容范围写入；必须先增加契约测试。探测和写入使用：

- `Token: <local-token>` 请求头；
- `GET /status?r=sys,sbd`；
- `GET /inbounds`、`GET /clients`、`GET /onlines`；
- `GET /clients?id=<id>`；
- `POST /save`，表单字段为 `object=clients`、`action` 和 `data`。

列表发现端点不读取客户端 `config`、links 或密码。只有受管客户端协调时，Agent 才在
DMIT 本机读取单客户端详情；原始响应不会离开节点。

## 安全边界

S-UI API Token 只存在 DMIT 的：

```text
/etc/hyfleet/agent.env  root:hyfleet-agent 0640
```

Token 不进入控制面 SQLite、Agent SQLite、API 响应、诊断输出、fleet 配置或 GitHub。
不要把 Token 粘贴到聊天、issue、提交、截图、shell 命令参数或订阅链接中。

Agent 只接受字面量 loopback HTTP 地址，并要求路径以 `/apiv2` 结尾。控制面不会直接访问
S-UI，S-UI 管理端口也不应因为 HyFleet 而新增公网暴露。

受管用户密码仍由控制面加密保存。S-UI Agent 仅能针对当前节点、当前 desired version、
当前 snapshot hash 请求当前凭据。响应带 `Cache-Control: no-store`；Agent 将密码直接传给
loopback S-UI 后丢弃，不写入本地 SQLite。S-UI 自身必须保存该节点密码，因此 S-UI
数据库及备份仍属于敏感数据。

## 准备 S-UI Token

1. 登录 DMIT 的 S-UI 管理界面。
2. 打开 `Admin` 页面并创建专用 `API Token`。
3. 设置合理到期时间并记录轮换日期。
4. 在只显示一次时安全保存 Token。

不要使用 HyFleet 管理员密码或订阅 Token 代替 S-UI API Token。

确认 S-UI 实际端口和面板路径。默认安装常见地址是：

```text
http://127.0.0.1:2095/app/apiv2
```

如果面板路径不是 `/app/`，必须填写实际路径。例如面板根路径为 `/manage/` 时，API 地址
应以 `/manage/apiv2` 结尾。不要在 URL 中放用户名、密码或 Token。

## DMIT 首次安装

先在 HyFleet 创建 Adapter 为 `S-UI` 的 DMIT 节点并生成一次性注册 Token。在
`v0.5.0-dev` 发布包目录执行：

```bash
sudo bash deploy/install-agent.sh \
  --server-url https://panel.example.com \
  --node-name DMIT \
  --adapter s-ui \
  --s-ui-api-url http://127.0.0.1:2095/app/apiv2
```

安装器依次无回显读取本机 S-UI API Token 和 HyFleet 一次性注册 Token。注册成功后，
一次性 Token 被删除，S-UI Token 保留。检查结果时不要输出环境文件内容：

```bash
sudo stat -c '%a %U:%G %n' /etc/hyfleet/agent.env
sudo bash deploy/diagnose.sh agent
sudo systemctl status hyfleet-agent --no-pager -l
```

应看到环境文件为 `640 root:hyfleet-agent`，诊断只显示 S-UI URL/Token 是否配置，不显示值。

## 从 v0.4 升级 DMIT

`deploy/update-component.sh` 会拒绝在缺少长期 S-UI Token 时升级 S-UI Agent，避免部署后
只留下一个无权限的适配器。先备份控制面数据库、WAL、master key 和 DMIT Agent SQLite，
再使用 `v0.5.0-dev` 解压目录完成一次本机安装器升级。

默认 API 地址正确时直接运行：

```bash
sudo bash deploy/install-agent.sh
```

安装器保留现有 `/etc/hyfleet/agent.yaml` 和 Agent 身份，只提示输入缺少的 S-UI Token。
若 API 地址不是默认值，先执行：

```bash
sudoedit /etc/hyfleet/agent.yaml
```

加入或修改：

```yaml
local_database_path: /var/lib/hyfleet-agent/agent.db
s_ui_api_url: http://127.0.0.1:2095/app/apiv2
s_ui_token_env: HYFLEET_SUI_TOKEN
```

然后重新运行无参数安装器。DMIT 准备完成后，其余节点可使用：

```powershell
.\scripts\deploy-fleet.ps1 -Version v0.5.0-dev
```

`scripts/fleet.example.psd1` 中 DMIT 的 `RequiredEnvironment` 只保存变量名，用来远程检查
Token 是否存在；不要把值加入该文件。

## 面板操作顺序

### 1. 选择目标入站

打开“节点 -> DMIT”，S-UI 状态应为“兼容”，并显示版本和发现时间。在“受管
Hysteria2 入站”中选择需要 HyFleet 管理的入站并保存。等待节点配置版本显示已应用。

目标入站是所有受管客户端的持续期望状态：新建客户端使用这些入站，已接管客户端也会在
目标变化后同步更新 Hysteria2 入站成员。客户端原有的非 Hysteria2 入站成员会保留。只读和
未映射客户端不会因为选择目标而被修改；存在受管客户端时不能清空全部目标入站。

### 2. 只读导入现有客户端

在未映射客户端旁选择一个已有全局用户，然后执行“只读导入”。此阶段只建立：

```text
全局用户 ID <-> S-UI remote client ID
```

只读分配会采集导入后的流量和在线状态，但不会读取密码、进入订阅、修改远端启用状态、
到期时间或额度。导入时以当前累计计数作为基线，导入前历史流量不会被当成新增流量。

### 3. 显式接管

只有只读分配显示“已同步”后，接管按钮才可用。接管对话框要求输入当前远端客户端原名。
成功后 HyFleet 会：

- 将远端名称统一为全局用户名；
- 用独立高熵 HyFleet 凭据替换远端 Hysteria2 密码；
- 应用用户启用、到期和额度状态；
- 将 applied 的端点加入统一订阅。

S-UI 保存客户端时会重载关联入站，现有连接可能断开。应在可接受的维护窗口逐个接管，
并提前说明旧密码和旧订阅地址会失效。

### 4. 创建新的受管用户

目标入站至少选择一个后，添加用户或用户详情中的节点分配会显示 DMIT S-UI。新建分配由
Agent 在目标入站创建 S-UI 客户端。等待状态从“等待同步”变为“已同步”后，统一订阅才会
输出该节点。

## 所有权与删除保护

Agent SQLite 保存 remote client ID、最近名称、管理模式和 HyFleet 凭据指纹，不保存明文。
删除受管客户端前会同时验证本地 ownership、remote ID、远端名称和凭据指纹。任一项不匹配
都会返回 `sui_ownership_guard_failed`，不会删除远端资源。

S-UI 数据库迁移导致 client ID 改变时，Agent 只会在“名称或 desired 用户名 + 凭据指纹”
唯一匹配时恢复 ownership。零个或多个候选都失败关闭，不猜测归属。

不要删除 `/var/lib/hyfleet-agent/agent.db`。该文件包含未上报流量 Outbox 和 S-UI ownership；
丢失后不会自动把未知远端客户端当成 HyFleet 所有。

## 流量与在线状态

S-UI `up` 映射为用户上传，`down` 映射为用户下载。Agent 保存累计基线，只把增量放入已有
Outbox；控制面按 batch ID、source epoch 和 sequence 幂等入账。控制面断线期间不会丢弃
已落盘批次。

未映射客户端使用稳定的 `sui:<remote-id>` 审计标识上报，因此会进入节点的未归属流量与
未知在线用户汇总，不会计入任何全局用户额度。只读导入应用时从当时累计值重新建立用户基线。

当 S-UI 重置单个或全部客户端计数时，Agent 轮换 source epoch，只从发生下降的用户当前值
重新计数，不会重复计算未重置用户。S-UI `/onlines` 只提供名称集合，因此当前在线连接数按
每个在线映射用户 `1` 上报，不能表示真实设备数量。

## 当前限制

- 已验证范围仅为 S-UI `v1.5.3` 至 `<v1.6.0`。
- S-UI `/onlines` 无连接数明细。
- 手动“踢下线”仅支持原生 Hysteria2 Traffic Stats API；S-UI 管理通过停用或配置重载生效。
- 只读导入不执行启用、到期或额度限制；接管后才受控。
- S-UI API Token 应按全控制权限保护并定期轮换。
- S-UI Token 到期或轮换后，需在 DMIT 本机更新 `/etc/hyfleet/agent.env` 并重启 Agent。

## 阶段验收

- DMIT 显示 S-UI 兼容版本、sing-box 状态、目标入站和最近发现时间；
- 未映射客户端发现响应不包含 `config`、links、密码或 Token；
- 只读导入不调用 S-UI `save`，不进入统一订阅；
- 只读映射应用后才允许接管，且必须输入准确原名；
- 新建、启停、到期和轮换只改变 HyFleet ownership 下的客户端；
- 手工修改受管客户端密码后，删除被 ownership guard 拒绝；
- Agent/控制面断线重连后，流量批次不重复、不丢失；
- 控制面停止时 S-UI 和 sing-box 数据面继续工作；
- Clash Verge 完整配置在规则模式下通过 `HyFleet` 组路由；
- 桌面和移动视口均可完成目标选择、只读导入和显式接管；
- Git 历史、Release、诊断和日志中不存在真实 Token、密码、IP 或完整订阅 URL。
