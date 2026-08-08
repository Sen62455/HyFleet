# Ubuntu systemd 完整部署指南

## 当前结论

本指南覆盖当前 `v0.5.0-dev` 的控制面、原生 Hysteria2 Agent、独立
sing-box Agent 和 S-UI Agent。首次部署仍应先确认 Server 与三台 Agent
稳定在线，再进行原生认证迁移或 S-UI 接管，便于区分部署故障和适配器故障。

本指南适用于当前三台 `x86_64`、Ubuntu 24.04 主机：

| 主机 | 阶段 1 角色 | Agent 类型 | 被观察的服务 |
| --- | --- | --- | --- |
| DMIT | 控制面和节点 | `s-ui` | `s-ui.service` |
| LisaHost | 节点 | `native-hysteria2` | `hysteria-server.service` |
| BandwagonHost | 节点 | `standalone-sing-box` | `sing-box.service` |

未接管时，Agent 只读取主机与适配器状态。原生 Hysteria2 配置迁移必须单独显式执行；
S-UI 客户端也必须先只读导入、等待映射同步，再显式接管。普通安装和升级不会自动接管
现有客户端。

## 部署前准备

需要准备：

- 一个专用于 HyFleet 的域名，例如 `panel.example.com`；
- 该域名的 A/AAAA 记录指向 DMIT；
- DMIT 的 TCP 80 和 443 可用于签发证书和访问面板；
- 三台主机的 root 或 sudo SSH 权限；
- Windows 本机的 Go 1.26、Node.js 22 和 pnpm 11。

Hysteria2 使用 UDP。即使 Hysteria2 正在使用 UDP 443，Nginx 仍可使用 TCP 443，
两者不会因为端口号相同而冲突。不要把 HyFleet 的 `127.0.0.1:8080` 直接开放到公网，
也不要复用 S-UI 的管理监听地址。

## 1. 在 Windows 生成 Linux 发布包

在项目根目录运行：

```powershell
cd D:\zuoye\AI\Codex\Desktop\VPS\hyfleet
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1 `
  -Architecture amd64 `
  -Version v0.5.0-dev
```

输出文件位于：

```text
output/releases/hyfleet-v0.5.0-dev-linux-amd64.tar.gz
output/releases/hyfleet-v0.5.0-dev-linux-amd64.tar.gz.sha256
```

打包脚本会构建前端、交叉编译 Linux ELF、检查 ELF 架构，并为包内文件生成
`SHA256SUMS`。不要上传 Windows 本地预览使用的 `.exe` 文件。

## 2. 上传并校验 DMIT 安装包

在 Windows PowerShell 中，把 `DMIT_IP` 替换为 DMIT 的 SSH 地址：

```powershell
scp .\output\releases\hyfleet-v0.5.0-dev-linux-amd64.tar.gz `
  root@DMIT_IP:/root/
scp .\output\releases\hyfleet-v0.5.0-dev-linux-amd64.tar.gz.sha256 `
  root@DMIT_IP:/root/
```

登录 DMIT 后执行：

```bash
cd /root
sha256sum -c hyfleet-v0.5.0-dev-linux-amd64.tar.gz.sha256
tar -xzf hyfleet-v0.5.0-dev-linux-amd64.tar.gz
cd hyfleet-v0.5.0-dev-linux-amd64
sha256sum -c SHA256SUMS
file bin/hyfleet-server bin/hyfleet-agent
bash -n deploy/*.sh
```

两份二进制都应显示 `ELF 64-bit` 和 `x86-64`。如果显示 `PE32`、`Windows`、
`ARM aarch64`，不要继续安装。

## 3. 在 DMIT 安装控制面

安装基础命令：

```bash
sudo apt update
sudo apt install -y ca-certificates curl file openssl
```

首次安装，把示例域名替换为实际的专用域名：

```bash
sudo bash deploy/install-server.sh \
  --public-url https://panel.example.com
```

安装器会执行以下操作：

- 验证上传的是 Linux ELF；
- 幂等创建 `hyfleet` 系统用户；
- 安装并检查配置和 systemd 单元；
- 创建 SQLite 状态目录和 64 位十六进制 bootstrap token；
- 启动 Server 并检查本机 `/healthz`；
- 在终端显示一次 bootstrap token。

不要把 bootstrap token 发到聊天、GitHub 或截图中。建立管理员账户前，可在 DMIT 上重新查看：

```bash
sudo sed -n 's/^HYFLEET_BOOTSTRAP_TOKEN=//p' /etc/hyfleet/server.env
```

如果此前失败的安装已经留下错误的 `server.yaml`，使用下列命令只重建 Server 配置。
它不会删除 `/var/lib/hyfleet/server.db` 或 `master.key`：

```bash
sudo bash deploy/install-server.sh \
  --public-url https://panel.example.com \
  --replace-config
```

本机检查必须成功：

```bash
sudo systemctl status hyfleet-server --no-pager -l
curl --fail http://127.0.0.1:8080/healthz
```

## 4. 配置 Nginx 和可信 HTTPS

如果 DMIT 尚未安装 Nginx 和 Certbot：

```bash
sudo apt install -y nginx certbot python3-certbot-nginx
```

创建 `/etc/nginx/sites-available/hyfleet`，将域名替换为实际值：

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name panel.example.com;

    client_max_body_size 2m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

启用站点并签发证书：

```bash
sudo test -L /etc/nginx/sites-enabled/hyfleet || \
  sudo ln -s /etc/nginx/sites-available/hyfleet /etc/nginx/sites-enabled/hyfleet
sudo nginx -t
sudo systemctl reload nginx
sudo certbot --nginx -d panel.example.com
```

若已经有可信证书和 HTTPS server block，只需把上述 `location /` 合并到对应域名中，
然后执行 `nginx -t` 和 reload。

从 Windows 验证外部 HTTPS：

```powershell
curl.exe --fail https://panel.example.com/healthz
```

返回 JSON 且包含 `"status":"ok"` 后才能安装 Agent。不要使用 `curl -k` 绕过证书检查，
Agent 也不会绕过证书校验。

## 5. 创建管理员

浏览器打开 `https://panel.example.com`，输入安装器显示的 bootstrap token，然后创建管理员。
管理员用户名为 3 到 32 个字母、数字、点、下划线或连字符，密码长度为 12 到 128。

创建成功后，立即在 DMIT 删除一次性 token 并重启：

```bash
sudo rm -f /etc/hyfleet/server.env
sudo systemctl restart hyfleet-server
curl --fail http://127.0.0.1:8080/api/v1/setup/status
```

返回结果应包含：

```json
{"bootstrap_token_configured":false,"setup_required":false}
```

字段顺序可能不同，不影响结果。

## 6. 在面板创建三个节点

先在 HyFleet 中创建：

| 面板节点名 | Adapter |
| --- | --- |
| DMIT | S-UI |
| LisaHost | Native Hysteria2 |
| BandwagonHost | Standalone sing-box |

每次准备安装一台 Agent 时，再为对应节点生成一次性注册 token。token 有效期为 10 分钟，
一个节点生成新 token 后，旧 token 会立即失效。面板 Adapter 必须和安装命令匹配。

## 7. 安装 LisaHost Agent

把同一发布包和校验文件上传到 LisaHost，按第 2 节解压和校验，然后在解压目录运行：

```bash
sudo apt update
sudo apt install -y ca-certificates curl file
sudo bash deploy/install-agent.sh \
  --server-url https://panel.example.com \
  --node-name LisaHost \
  --adapter native-hysteria2
```

安装器提示时粘贴 LisaHost 对应的注册 token。输入不会回显。注册成功后，安装器会自动删除
`/etc/hyfleet/agent.env`，重启 Agent，并用持久凭据验证它仍能启动。

## 8. 安装 BandwagonHost Agent

上传、解压和校验发布包后运行：

```bash
sudo apt update
sudo apt install -y ca-certificates curl file
sudo bash deploy/install-agent.sh \
  --server-url https://panel.example.com \
  --node-name BandwagonHost \
  --adapter standalone-sing-box
```

提示时粘贴 BandwagonHost 节点的新注册 token。

## 9. 安装 DMIT Agent

先登录 S-UI，在 `Admin` 页面创建一个有合理到期时间的专用 `API Token`。该 Token
只显示一次；不要把它发送到聊天、写进 Git、放进 `fleet.local.psd1` 或作为命令行参数。

确认 S-UI 的实际本机端口和面板路径。默认值通常对应
`http://127.0.0.1:2095/app/apiv2`，但面板路径不是 `/app/` 时必须同步修改；地址必须使用
明文 loopback IP、包含端口并以 `/apiv2` 结尾。DMIT 已有解压目录后运行：

```bash
sudo bash deploy/install-agent.sh \
  --server-url https://panel.example.com \
  --node-name DMIT \
  --adapter s-ui \
  --s-ui-api-url http://127.0.0.1:2095/app/apiv2
```

安装器会先无回显读取本机 S-UI API Token，再读取一次性 HyFleet 注册 Token。注册完成后，
只删除后者；S-UI Token 长期保存在 `/etc/hyfleet/agent.env`，权限为
`root:hyfleet-agent 0640`。控制面、浏览器和 fleet 配置都不会收到该 Token。

Server 和 Agent 共存时，`/etc/hyfleet` 保持 `root:root 0755`；`server.yaml`、
`agent.yaml` 和各自环境文件仍分别使用 `0640` 和独立服务组，不会出现目录组所有权冲突。

## 10. 验收阶段 1

每台节点执行：

```bash
sudo systemctl status hyfleet-agent --no-pager -l
sudo journalctl -u hyfleet-agent -b -n 50 --no-pager
sudo stat -c '%a %U:%G %n' /var/lib/hyfleet-agent/agent-state.json
```

状态文件应为 `600 hyfleet-agent:hyfleet-agent`。在面板确认三台节点在 90 秒内变为在线，
然后把 UI 数据和节点本机数据比较：

```bash
awk '/MemTotal|MemAvailable/ { print }' /proc/meminfo
df -B1 /
cat /proc/loadavg
cat /proc/uptime
```

最后重启每台 Agent，确认无需新 token 也会重新上线：

```bash
sudo systemctl restart hyfleet-agent
sudo journalctl -u hyfleet-agent -b -n 30 --no-pager
```

满足以下条件后，阶段 1 才通过：

- Server 的本机和公网 `/healthz` 都成功；
- 三台 Agent 都是 `active`，重启后可恢复；
- 三台节点的主机指标合理，原代理服务仍正常；
- 管理员创建后 `/etc/hyfleet/server.env` 不存在；LisaHost/BandwagonHost 的一次性
  `/etc/hyfleet/agent.env` 已删除；DMIT 的该文件只保留 `HYFLEET_SUI_TOKEN`；
- `server.db` 和 `master.key` 已规划为成对备份。

## 故障诊断

不要先卸载或删除数据库。进入解压后的发布目录，按失败组件执行：

```bash
sudo bash deploy/diagnose.sh server > /tmp/hyfleet-server-diagnostic.txt 2>&1
sudo bash deploy/diagnose.sh agent > /tmp/hyfleet-agent-diagnostic.txt 2>&1
```

诊断脚本不会输出 HyFleet 环境文件或 Agent 长期凭据。反馈问题时提供：

- 最初执行的完整命令；
- 对应诊断文件的完整内容；
- 域名和公网 IP 可以打码；
- 不要提供 token、密码、私钥或订阅 URL。

常见错误：

| 错误 | 原因 | 处理 |
| --- | --- | --- |
| `status=203/EXEC` | Windows 文件、架构错误或二进制缺失 | 重新上传本指南生成的 ELF 包 |
| `status=217/USER` | systemd 服务用户不存在 | 重新执行对应安装脚本 |
| `permission denied` | 配置、状态目录或父目录权限错误 | 重新执行安装脚本并附诊断结果 |
| `address already in use` | `127.0.0.1:8080` 被占用 | 改用 `--listen 127.0.0.1:18080` 并同步 Nginx |
| `bootstrap_not_configured` | 首次启动缺少 `server.env` | 重新执行 Server 安装脚本 |
| `HYFLEET_ENROLLMENT_TOKEN is empty` | Agent 未注册且 token 文件缺失 | 在面板生成新 token 后重新执行 Agent 安装脚本 |
| `enrollment_rejected` | token 错误、失效或属于旧请求 | 生成新 token 后重试 |
| `enrollment_conflict` | 面板 Adapter 不匹配或节点已绑定 | 核对 Adapter；未注册节点可用 `--replace-config` 修正 |
| `sui_token_not_configured` | DMIT 缺少长期 S-UI Token | 重新执行安装器并在本机无回显输入新 Token |
| `sui_api_unavailable` | loopback 地址、端口、路径或 Token 错误 | 核对实际面板路径和 `/apiv2`，不要把 Token 放进诊断输出 |
| `sui_version_unsupported` | S-UI 不在已验证的 `v1.5.3` 至 `<v1.6.0` 范围 | 保持只读并使用受支持版本，升级适配器测试后再放宽 |
| `x509` | 域名、证书链或有效期错误 | 修复可信证书，不要使用跳过校验参数 |
| Nginx `502` | Server 未启动或 proxy_pass 端口错误 | 先检查本机 `/healthz` 和 Server 日志 |
| `226/NAMESPACE` | VPS 不支持某项 systemd 隔离能力 | 提供完整诊断，不要直接删除全部加固项 |

## 回滚和数据保护

阶段 1 Agent 与代理核心解耦。遇到问题可直接停止 Agent，不影响现有节点流量：

```bash
sudo systemctl disable --now hyfleet-agent
```

保留 `/var/lib/hyfleet-agent/agent-state.json`，重新安装时可继续使用现有注册身份。只有明确要
重新绑定节点时才删除它。

控制面的 `/var/lib/hyfleet/server.db` 和 `/var/lib/hyfleet/master.key` 必须成对备份。
只有数据库而没有主密钥，无法恢复后续阶段中的加密数据。不要把这两个文件提交到 GitHub。
