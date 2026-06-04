# 常见问题排查 (Troubleshooting)

---

## 问题一：静默模式下打开任何页面都是 404

**现象**：访问 `http://your-server:8080/` 或 `/login` 等路径，均返回 404 页面。

**原因**：静默模式（默认开启）会将所有不含正确 32 位十六进制前缀的请求返回 404，这是防止扫描器发现面板的正常行为。

**解决方法**：

**方法 1：查看启动日志获取登录 URL**

```bash
# Docker
docker logs shiguang 2>&1 | grep "login_url\|Silent mode"

# systemd
journalctl -u shiguang-vps --since "today" | grep "login_url\|Silent mode"

# 直接运行（查看终端输出）
```

日志中应有类似输出：
```
level=INFO msg="Silent mode enabled" login_url="http://your-server:8080/_app/a3f8c2d1e4b7096f5a2c3e8d1f4a7b09/login"
```

**方法 2：使用恢复 CLI**（忘记 URL 时）

```bash
# 停止服务
sudo systemctl stop shiguang-vps

# 运行恢复命令获取静默模式前缀
./shiguang-vps-linux-amd64 --show-prefix

# 输出示例：
# Silent mode prefix: /_app/a3f8c2d1e4b7096f5a2c3e8d1f4a7b09/
```

**方法 3：临时关闭静默模式**（仅用于调试，不推荐长期开启）

```bash
# 通过环境变量关闭
SHIGUANG_SILENT_MODE=false ./shiguang-vps-linux-amd64
```

---

## 问题二：忘记密码 / 丢失两步验证（账户重置）

hub 二进制内置恢复模式 `-reset-password <用户名>`：在 **VPS 本机**（SSH 登录后）对同一数据目录执行，会生成一个新的强密码、**关闭该账户的 TOTP 两步验证**、恢复账户为启用状态，并吊销所有旧会话。账户本身和所有数据（订阅、节点、agent、资产）原样保留，**服务无需停止**，新密码即时生效。

> 安全模型：能读写数据目录（即能 SSH 到这台机器）即视为有权重置——和 Linux 单用户模式改 root 密码同理。

**方法 A：deploy.sh 部署（最省事，本地电脑直接跑）**

```bash
./scripts/deploy.sh --reset-password          # 重置 admin
./scripts/deploy.sh --reset-password alice    # 重置指定用户
```

脚本会 SSH 到 VPS、自动找到二进制和数据目录并执行重置，新密码打印在终端。

**方法 B：SSH 到 VPS 手动执行（deploy.sh / 二进制部署）**

```bash
/opt/shiguang-vps/hub --data-dir /opt/shiguang-vps/data -reset-password admin
# 二进制手动部署的，把路径换成你自己的 --data-dir
```

**方法 C：Docker 部署**

```bash
docker compose exec hub hub -reset-password admin
# 或不在 compose 目录时：
docker exec shiguang-vps hub -reset-password admin
```

成功后输出：

```
==================== ACCOUNT RESET ====================
  username : admin
  password : <新的随机强密码>
  totp     : disabled
  status   : active (all existing sessions revoked)
  note     : log in and change this password immediately
=======================================================
```

用新密码登录后，请立即在「设置 → 账户」里改成自己的密码，需要的话重新绑定两步验证。

---

## 问题三：Agent 无法连接到 Hub

**现象**：Agent 启动后在面板中显示"离线"或从不出现。

**排查步骤**：

**检查 1：Agent token 是否正确**

```bash
# 查看 agent 日志（systemd 安装）
journalctl -u shiguang-agent -n 50

# 常见错误信息
# "connection refused" → Hub 不可达
# "404 Not Found" → token 错误或静默模式前缀不对
# "certificate verify failed" → TLS 证书问题
```

确认 `hub_url` 和 `agent_token` 与面板中的配置完全一致：

```bash
# 查看 agent 配置
cat /etc/shiguang-agent/config.yml
# 或查看环境变量
env | grep SHIGUANG
```

**检查 2：防火墙是否放行 8080 端口**

```bash
# 在 Hub 服务器上检查端口监听
ss -tlnp | grep 8080

# 从 Agent 服务器测试连通性
curl -v https://your-hub.example:8080/healthz
# 或（如有静默前缀）
curl -v https://your-hub.example:8080/_app/<32hex>/healthz
```

**检查 3：静默模式 WebSocket 路径**

静默模式开启时，Agent WebSocket 连接路径包含前缀：

```
# 正确格式
wss://your-hub.example/_app/<32hex>/api/agent/ws

# 错误格式（忘记前缀）
wss://your-hub.example/api/agent/ws
```

**检查 4：TLS 证书问题**

```bash
# 测试 TLS 握手
openssl s_client -connect your-hub.example:8080 -servername your-hub.example

# 如果使用自签名证书，agent 需要跳过验证（不推荐生产环境）
SHIGUANG_TLS_SKIP_VERIFY=true ./shiguang-agent
```

**Nezha 兼容模式额外检查**：

确认 `server` 字段格式正确（包含 API 路径）：

```yaml
# 正确
server: "your-hub.example/_app/<32hex>/api/v1/nezha"

# 错误（缺少路径）
server: "your-hub.example"
```

---

## 问题四：订阅同步失败

**现象**：订阅显示"同步失败"或节点数为 0。

**排查步骤**：

**检查 1：查看同步日志**

在拾光VPS 面板中，导航到 **订阅详情 → 同步日志**，查看具体错误信息。

**检查 2：确认远端 URL 可达**

```bash
# 从 Hub 服务器直接 curl 测试
curl -v -A "clash.meta" "https://your-subscription-url.example/sub?..."
```

常见错误和对应原因：

| 错误 | 原因 |
|------|------|
| `connection refused` | 远端服务器不可达或被防火墙拦截 |
| `HTTP 403` | UA 被机场拦截，需自定义 User-Agent |
| `HTTP 429` | 请求过于频繁，降低同步频率（改为 12h 或 24h） |
| `HTTP 5xx` | 远端服务器内部错误，稍后重试 |
| `parse error` | 订阅内容格式不支持，检查是否为 Clash YAML / Base64 编码 |

**检查 3：自定义 User-Agent**

部分机场对 UA 有限制，在订阅设置中指定 UA：

- **常见有效 UA**：`clash.meta`、`Clash/1.18.0`、`mihomo/1.18.0`
- 在 **订阅设置 → User-Agent** 字段填写

<!-- screenshot: 订阅编辑对话框，User-Agent 字段 -->

---

## 问题五：流水线运行缓慢

**现象**：点击"运行预览"后长时间无响应，或订阅同步时间超过预期。

**常见原因和解决方法**：

**节点数量过多**：

- 单次流水线建议节点数 ≤ 500
- 使用 `filter` 算子提前过滤，减少后续算子的处理量
- 将 `filter` 放在流水线的第一位（效果最明显）

**正则表达式过于复杂**：

- `regex-rename` 的 `pattern` 避免使用回溯过深的正则（如 `.*.*.*`）
- 拾光VPS 会在正则编译失败时给出 UI 提示，不会 panic

**服务器资源不足**：

- 查看 Hub 服务器 CPU / 内存
- 流水线执行超过 500ms 时日志会有 `slow pipeline` 警告

---

## 问题六：OTA 升级失败后如何回滚

**现象**：触发 OTA 升级后服务无法启动，需要恢复。

**拾光VPS OTA 升级流程**：
1. 下载新二进制到 `./shiguang-vps.new`
2. SHA-256 校验通过后，将原二进制重命名为 `./shiguang-vps.bak`
3. 将新二进制移动为 `./shiguang-vps`
4. 执行 `wal_checkpoint` 后重启

**回滚步骤**：

```bash
# 停止服务
sudo systemctl stop shiguang-vps

# 恢复备份二进制
# 默认安装在 /opt/shiguang-vps/
ls -la /opt/shiguang-vps/
# 应能看到 shiguang-vps（新版）和 shiguang-vps.bak（旧版）

# 恢复旧版本
mv /opt/shiguang-vps/shiguang-vps /opt/shiguang-vps/shiguang-vps.failed
mv /opt/shiguang-vps/shiguang-vps.bak /opt/shiguang-vps/shiguang-vps

# 重启服务
sudo systemctl start shiguang-vps

# 确认启动成功
sudo systemctl status shiguang-vps
journalctl -u shiguang-vps -n 20
```

**Docker 用户**：

```bash
# 回滚到指定版本（如 v1.2.3）
docker pull ghcr.io/shiguang-vps/shiguang-vps:v1.2.3
docker stop shiguang
docker rm shiguang
docker run -d --name shiguang \
  -p 8080:8080 \
  -v /data/shiguang:/data \
  ghcr.io/shiguang-vps/shiguang-vps:v1.2.3
```

---

## 问题七：Telegram Bot 在国内无法接收通知

**现象**：Telegram 渠道配置正确，测试按钮也能发送，但之后的通知收不到；或测试时就返回超时错误。

**原因**：国内网络无法直接访问 Telegram API（`api.telegram.org`）。

**解决方法一：为 Hub 配置 HTTP 代理**（推荐）

```bash
# 在 Hub 服务器上设置代理（需要可用的代理地址）
HTTPS_PROXY=http://proxy.example.com:7890 ./shiguang-vps-linux-amd64

# 或在 systemd 服务中设置
# 编辑 /etc/systemd/system/shiguang-vps.service
[Service]
Environment=HTTPS_PROXY=http://proxy.example.com:7890
```

**解决方法二：切换到国内可用的通知渠道**

如果无法配置代理，可使用以下国内友好的替代渠道：

- **Server酱**：基于微信推送，国内完全可用，延迟低
- **PushDeer**：支持苹果推送，不依赖 Telegram
- **Email**：通过国内 SMTP 服务（如 QQ 邮件）发送

同时配置 Telegram + Server酱，按事件类型分别订阅，互为备份。

**解决方法三：部署反向代理**

在海外服务器部署一个 Nginx 反向代理，将 `api.telegram.org` 的请求转发，然后通过 Webhook 模式配置 Telegram Bot。

---

## 查看日志的通用方法

```bash
# Docker
docker logs shiguang --tail 100 -f

# systemd
journalctl -u shiguang-vps -f --output=cat

# 直接运行（日志输出到 stdout，JSON 格式）
./shiguang-vps 2>&1 | jq '.'

# 过滤错误日志
journalctl -u shiguang-vps | grep '"level":"ERROR"'
```

拾光VPS 使用 JSON 结构化日志，每条日志包含 `level`、`msg`、`time` 等字段，便于过滤和分析。

---

## 获取帮助

如果以上排查步骤无法解决问题：

1. 收集日志：`journalctl -u shiguang-vps --since "1 hour ago" > shiguang.log`
2. 在 [GitHub Issues](https://github.com/shiguang-vps/shiguang-vps/issues) 提交 issue，附上日志和系统信息
3. 提交 issue 前搜索是否已有相同问题

提交 issue 时请包含：
- 拾光VPS 版本（面板右下角或 `./shiguang-vps --version`）
- 操作系统和架构
- 部署方式（Docker / systemd / 直接运行）
- 完整错误日志（脱敏后）
