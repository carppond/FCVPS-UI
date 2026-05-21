# 从 Nezha 迁移 (Migrating from Nezha)

拾光VPS 内置 Nezha v2 协议兼容层，原 Nezha agent **不改二进制**，只修改两处配置即可接入。

---

## 兼容原理

拾光VPS hub 暴露与 Nezha v2 协议兼容的端点：

```
POST /api/v1/nezha/heartbeat
POST /api/v1/nezha/report
```

原 Nezha agent 向这些端点上报心跳时，拾光VPS 会将 Nezha v2 协议字段映射到内部 `AgentRecord` 格式，并在面板中正常展示 CPU / 内存 / 磁盘 / 流量等指标。

---

## 迁移步骤（不改 agent 二进制）

### 步骤 1：在拾光VPS 创建 Agent

1. 登录拾光VPS，导航到 **探针** 页面
2. 点击 **添加 Agent**
3. 在 **类型** 字段选择 `nezha_compat`（Nezha 兼容模式）

<!-- screenshot: 添加 Agent 对话框，类型选择 nezha_compat -->

4. 填写名称（建议与 Nezha 中的服务器名保持一致，便于对应）
5. 点击 **创建**，系统生成一个 `secret`（格式类似 `sg_abc123def456...`）
6. 记录下面板显示的 **secret** 值，后续步骤中使用

<!-- screenshot: Agent 创建成功弹窗，显示 secret 字段 -->

### 步骤 2：修改 Nezha agent 配置

找到原 Nezha agent 的配置文件（默认路径因安装方式而异）：

**systemd 方式**：
```bash
# 通常在 /etc/nezha-agent/ 或 /opt/nezha-agent/
cat /etc/nezha-agent/config.yml
```

**docker-compose 方式**：
```bash
# 查看 docker-compose.yml 中的环境变量
```

修改以下两个字段：

```yaml
# 原配置（示例）
server: "nezha.example.com:5555"
client_secret: "原来的 Nezha secret"

# 改为
server: "your-hub.example/_app/<32hex>/api/v1/nezha"
client_secret: "sg_abc123def456..."   # 拾光VPS 生成的 secret
```

> 将 `your-hub.example` 替换为你的拾光VPS 实际域名或 IP。
> 静默模式开启时，路径前缀 `/_app/<32hex>` 是必须的（从日志或面板获取完整 URL）。

**环境变量方式配置的 Nezha agent**：

```bash
# 原配置
NEZHA_SERVER=nezha.example.com:5555
NEZHA_KEY=原来的_secret

# 改为
NEZHA_SERVER=your-hub.example/_app/<32hex>/api/v1/nezha
NEZHA_KEY=sg_abc123def456...
```

### 步骤 3：重启 Nezha agent

```bash
# systemd
sudo systemctl restart nezha-agent

# Docker
docker restart nezha-agent

# 直接运行
kill $(pgrep nezha-agent) && ./nezha-agent &
```

### 步骤 4：验证连接

1. 在拾光VPS 面板的 **探针** 页面，查看该 Agent 是否变为「在线」状态（绿色）
2. 刷新页面，确认 CPU / 内存 / 流量数据正常上报
3. 通常 30-60 秒内即可看到第一条心跳数据（Nezha agent 默认心跳间隔 30s）

<!-- screenshot: 探针列表，Agent 显示在线状态，带 CPU/内存指标 -->

---

## 字段映射表

下表列出 Nezha v2 心跳字段与拾光VPS 内部字段的对应关系：

| Nezha v2 字段 | 拾光VPS 字段 | 说明 |
|---------------|-------------|------|
| `State.CPU` | `cpu_percent` | CPU 使用率（%） |
| `State.MemUsed` / `State.MemTotal` | `mem_used` / `mem_total` | 内存已用 / 总量（字节） |
| `State.SwapUsed` / `State.SwapTotal` | `swap_used` / `swap_total` | Swap 已用 / 总量（字节） |
| `State.DiskUsed` / `State.DiskTotal` | `disk_used` / `disk_total` | 磁盘已用 / 总量（字节） |
| `State.NetInSpeed` | `net_in_speed` | 网络入速（字节/秒） |
| `State.NetOutSpeed` | `net_out_speed` | 网络出速（字节/秒） |
| `State.NetInTransfer` | `net_in_transfer` | 累计入流量（字节） |
| `State.NetOutTransfer` | `net_out_transfer` | 累计出流量（字节） |
| `State.Uptime` | `uptime` | 系统运行时长（秒） |
| `State.Load1` / `Load5` / `Load15` | `load_1` / `load_5` / `load_15` | 负载均值 |
| `State.TcpConnCount` | `tcp_conn_count` | TCP 连接数 |
| `State.UdpConnCount` | `udp_conn_count` | UDP 连接数 |
| `State.ProcessCount` | `process_count` | 进程数 |

> 拾光VPS 仅兼容 Nezha v2 最小字段集，扩展字段会被忽略并记录 warning 日志（不影响基础指标展示）。

---

## 已知差异与限制

| 功能 | Nezha | 拾光VPS v1 |
|------|-------|-----------|
| CPU / 内存 / 磁盘 / 网络监控 | 支持 | 支持 |
| 流量趋势图 | 支持 | 支持（按日/月） |
| 服务监控（HTTP / TCP / Ping） | 支持 | 不支持（v1 范围外） |
| 定时任务（Cron Job） | 支持 | **不支持**，v1 不做 |
| Web Terminal（远程登录 agent） | 支持 | **v1 暂不支持**，列入 P2 路线图 |
| 多用户查看权限 | 支持 | 支持（admin / user 二分角色） |
| 主题替换 | 支持（第三方主题生态） | **v1 不做**，列入 P2 |
| agent 脚本下发 | 支持 | 不支持（v1 范围外） |
| Telegram Bot 通知 | 支持 | 支持（含双向交互） |

**Web Terminal** 功能已在 P2 路线图中规划，拾光VPS agent 已预留协议接口，待 P2 阶段实现后可无缝升级。

---

## 常见问题

**Q: 使用 Nezha v1（旧版）的 agent 能接入吗？**

A: 拾光VPS 主要兼容 Nezha v2 协议。Nezha v1 agent 使用 gRPC 协议，兼容性未经完整验证。推荐升级到 Nezha v2 agent，或直接切换到拾光VPS 自有 agent（性能更好，二进制更小）。

**Q: token 错误时会显示什么？**

A: 拾光VPS 的静默模式会对 token 错误的连接返回 404，不显示"token invalid"等明文错误，防止被扫描器探测。

**Q: 迁移后能同时保留 Nezha 面板吗？**

A: 可以。拾光VPS 与 Nezha 完全独立，但一个 agent 同时只能连接一个 hub。如需观望期，可以在不同 VPS 上分别试用，逐步迁移。

**Q: 上报频率和 Nezha 一样吗？**

A: Nezha agent 的上报间隔由 agent 自身配置决定（通常 3-5s），拾光VPS 接收端无限制。拾光VPS 自有 agent 默认心跳 30s（可配 5-300s）。
