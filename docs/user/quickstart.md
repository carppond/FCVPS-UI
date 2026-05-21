# 快速开始 (Quickstart)

本文档帮助你在 5 分钟内完成拾光VPS 的部署与初始配置。

---

## 系统要求

| 项目 | 最低要求 |
|------|----------|
| 内存 (RAM) | 256MB（hub 稳态 < 50MB，agent 稳态 < 30MB） |
| 磁盘 | 100MB（含数据库和日志） |
| 操作系统 | Linux（amd64 / arm64）、macOS（amd64 / arm64）、Windows（amd64） |
| Docker | 20.10+（可选） |
| 浏览器 | Chrome 110+ / Firefox 115+ / Safari 16+ / Edge 110+ |

---

## 三种部署方式

### 方式一：Docker（推荐）

适合已有 Docker 环境的用户，一行命令即可启动。

```bash
docker run -d \
  --name shiguang \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data/shiguang:/data \
  ghcr.io/shiguang-vps/shiguang-vps:latest
```

**说明**：
- `-p 8080:8080`：将容器 8080 端口映射到宿主机。如需使用其他端口，修改左侧数字，例如 `-p 9090:8080`。
- `-v /data/shiguang:/data`：将宿主机 `/data/shiguang` 目录挂载为容器数据目录。数据库（`shiguang.db`）、日志等均存放在此。

**使用 docker-compose**（推荐生产环境）：

```yaml
# docker-compose.yml
services:
  shiguang:
    image: ghcr.io/shiguang-vps/shiguang-vps:latest
    container_name: shiguang
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - /data/shiguang:/data
    environment:
      # 可选：指定时区
      - TZ=Asia/Shanghai
```

```bash
docker compose up -d
```

---

### 方式二：一键安装脚本

适合 Linux 服务器，脚本会自动下载二进制、注册 systemd 服务并启动。

```bash
curl -fsSL https://get.shiguang-vps.example/install.sh | bash
```

**脚本执行的操作**：
1. 检测系统架构（amd64 / arm64）和包管理器
2. 从 GitHub Release 下载最新二进制到 `/opt/shiguang-vps/`
3. 校验 SHA-256 校验和
4. 创建 systemd 服务文件（`/etc/systemd/system/shiguang-vps.service`）
5. 启动服务并设置开机自启

**其他脚本选项**：

```bash
# 升级到最新版本
curl -fsSL https://get.shiguang-vps.example/install.sh | bash -s -- --upgrade

# 卸载
curl -fsSL https://get.shiguang-vps.example/install.sh | bash -s -- --uninstall
```

---

### 方式三：手动二进制

适合离线环境或需要精细控制部署路径的用户。

**步骤 1：下载二进制**

从 [GitHub Releases](https://github.com/shiguang-vps/shiguang-vps/releases/latest) 下载对应平台的二进制文件：

| 平台 | 文件名 |
|------|--------|
| Linux amd64 | `shiguang-vps-linux-amd64` |
| Linux arm64 | `shiguang-vps-linux-arm64` |
| macOS (Apple Silicon) | `shiguang-vps-darwin-arm64` |
| macOS (Intel) | `shiguang-vps-darwin-amd64` |
| Windows amd64 | `shiguang-vps-windows-amd64.exe` |

同时下载 `sha256sums.txt` 并验证校验和：

```bash
sha256sum -c sha256sums.txt
```

**步骤 2：运行**

```bash
chmod +x shiguang-vps-linux-amd64
./shiguang-vps-linux-amd64
```

默认监听 `:8080`，数据存放在 `./data/` 目录。

**自定义配置**（通过环境变量或命令行参数）：

```bash
# 自定义端口和数据目录
SHIGUANG_PORT=9090 SHIGUANG_DATA_DIR=/var/lib/shiguang ./shiguang-vps-linux-amd64

# 或使用命令行参数
./shiguang-vps-linux-amd64 --port 9090 --data /var/lib/shiguang
```

**步骤 3：注册 systemd 服务**

```ini
# /etc/systemd/system/shiguang-vps.service
[Unit]
Description=拾光VPS Hub
After=network.target

[Service]
Type=simple
User=nobody
WorkingDirectory=/opt/shiguang-vps
ExecStart=/opt/shiguang-vps/shiguang-vps-linux-amd64
Restart=on-failure
RestartSec=5
Environment=SHIGUANG_DATA_DIR=/var/lib/shiguang

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now shiguang-vps
```

---

## 首次访问

### 查找初始密码和登录 URL

首次启动时，拾光VPS 会自动创建 admin 账号并在日志中打印初始密码和登录 URL：

```
level=INFO msg="ADMIN BOOTSTRAPPED" username=admin password=Aj7$kK9pXm2Q
level=INFO msg="Silent mode enabled" login_url="http://your-server:8080/_app/a3f8c2d1e4b7096f5a2c3e8d1f4a7b09/login"
```

**重要说明（静默模式）**：
- 默认开启静默模式，登录页面隐藏在 32 位随机十六进制前缀下（如 `/_app/a3f8c2d1e4b7096f5a2c3e8d1f4a7b09/`）
- 访问 `/`、`/login`、`/api/` 等常规路径均返回 404（防止被扫描器发现）
- 务必保存日志中打印的完整登录 URL

**各平台查看日志的方式**：

```bash
# Docker
docker logs shiguang 2>&1 | grep -E "ADMIN|Silent mode"

# systemd
journalctl -u shiguang-vps -n 50 | grep -E "ADMIN|Silent mode"

# 直接运行（日志在终端输出）
```

<!-- screenshot: 日志输出示例，高亮 ADMIN BOOTSTRAPPED 和 login_url 行 -->

### 登录

1. 用浏览器打开日志中打印的完整登录 URL（包含 `/_app/<32hex>/login`）
2. 用户名输入 `admin`，密码输入日志中打印的初始密码
3. 登录成功后立即修改密码（个人设置 → 修改密码）

<!-- screenshot: 登录页面截图，输入用户名和密码 -->

### 强烈推荐：启用 TOTP 2FA

登录后，前往 **个人设置 → 安全 → 启用两步验证**：

1. 使用 Google Authenticator、Authy 或任意 TOTP 应用扫描二维码
2. 输入 6 位验证码确认绑定
3. **保存备份码**：系统会生成 8 个 8 位十六进制备份码，请立即保存到安全位置（密码管理器、纸质备份等）
4. 备份码一次性有效，丢失设备时可用于恢复登录

<!-- screenshot: TOTP 设置页面，二维码 + 备份码列表 -->

---

## 添加第一个订阅

<!-- screenshot: 订阅列表页，点击"添加订阅"按钮 -->

1. 导航到 **订阅** 页面，点击 **添加订阅**
2. 选择订阅类型：
   - **URL 订阅**：填入订阅链接，系统自动定期拉取（默认每 6 小时更新一次）
   - **上传文件**：上传本地的 Clash YAML 文件
   - **手动添加**：逐节点手动录入
3. 填入名称，可选填标签、备注、流量额度
4. 点击 **保存**，随即点击 **立即同步** 拉取节点

<!-- screenshot: 添加订阅对话框 -->

---

## 添加第一个 Agent

<!-- screenshot: Agent 列表页，点击"添加 Agent"按钮 -->

**方式一：使用拾光VPS 自有 agent**

1. 导航到 **探针** 页面，点击 **添加 Agent**
2. 在弹出的对话框中填入名称，点击 **创建** 后系统会生成 `agent_token`
3. 在你的 VPS 上执行安装命令（页面会自动生成，类似）：

```bash
curl -fsSL https://your-hub.example/_app/<prefix>/agent/install.sh | \
  HUB_URL=wss://your-hub.example/_app/<prefix>/api/agent/ws \
  AGENT_TOKEN=<your_agent_token> \
  bash
```

**方式二：使用已有 Nezha agent（零改动）**

参见 [从 Nezha 迁移](./migration-from-nezha.md)。

---

## 添加第一个通知渠道

<!-- screenshot: 通知页面，渠道列表 -->

1. 导航到 **通知** 页面，点击 **添加渠道**
2. 选择渠道类型（如 Telegram）
3. 填入必要配置（如 bot_token + chat_id）
4. 点击 **测试** 验证消息是否送达
5. 在 **事件订阅** 标签页选择需要接收哪些事件的通知（节点离线 / 流量告警等）

详细每个渠道的配置方法，参见 [通知渠道配置](./notification-setup.md)。
