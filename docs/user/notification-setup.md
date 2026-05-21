# 通知渠道配置 (Notification Setup)

拾光VPS 支持 10 个通知渠道，每个渠道可独立配置，并按事件类型选择接收哪些通知。

**进入通知配置**：登录后导航到 **通知** 页面 → **添加渠道** → 选择渠道类型。

---

## 1. Telegram

<!-- screenshot: Telegram 渠道配置表单，填入 bot_token 和 chat_id -->

**所需信息**：`bot_token`、`chat_id`

### 获取 bot_token

1. 在 Telegram 中搜索并打开 [@BotFather](https://t.me/BotFather)
2. 发送命令 `/newbot`
3. 按提示输入 bot 名称（展示名）和用户名（必须以 `bot` 结尾）
4. BotFather 会回复一行 token，格式类似：`1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi`
5. 复制该 token

### 获取 chat_id

1. 向你刚创建的 bot 发送任意消息（先发 `/start`）
2. 访问以下 URL（将 `<YOUR_BOT_TOKEN>` 替换为实际 token）：
   ```
   https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates
   ```
3. 在返回的 JSON 中找到 `message.chat.id` 字段，这就是你的 `chat_id`
4. 如果用于群组通知，将 bot 加入群组后，`chat_id` 为负数（如 `-1001234567890`）

### 在拾光VPS 中填写

| 字段 | 说明 | 示例 |
|------|------|------|
| `bot_token` | BotFather 提供的 token | `1234567890:ABCxxx` |
| `chat_id` | 个人或群组的 chat ID | `98765432` 或 `-1001234567890` |

点击 **测试** 按钮，确认收到测试消息后保存。

### 常见问题

- **收不到消息**：国内网络可能无法直接连接 Telegram API，参见 [troubleshooting.md 的代理配置](#)。
- **chat_id 获取失败**：确认已先向 bot 发送消息（必须先发消息，getUpdates 才有数据）。
- **群组中 bot 不发言**：确认 bot 已被添加到群组，且群组中已有人 @ 过 bot 或 bot 有发送消息权限。

---

## 2. Discord

<!-- screenshot: Discord 创建 Webhook 的界面，服务器设置 → 集成 → Webhooks -->

**所需信息**：`webhook_url`

### 获取 Webhook URL

1. 打开 Discord 服务器，进入目标频道
2. 点击频道名旁边的 **齿轮图标**（编辑频道）
3. 左侧菜单选择 **集成（Integrations）**
4. 点击 **Webhooks** → **创建 Webhook**
5. 填写 Webhook 名称（如 `拾光VPS 通知`），可选择头像
6. 点击 **复制 Webhook URL**

### 在拾光VPS 中填写

| 字段 | 说明 |
|------|------|
| `webhook_url` | Discord 提供的完整 Webhook URL |

点击 **测试** 验证消息是否出现在 Discord 频道中。

### 常见问题

- **消息不显示**：检查 Webhook URL 是否完整复制（Discord Webhook URL 较长）。
- **发送失败 10008 错误**：Webhook 可能已被删除，重新创建一个新 Webhook。

---

## 3. Slack

<!-- screenshot: Slack Incoming Webhook 配置页面 -->

**所需信息**：`webhook_url`

### 获取 Incoming Webhook URL

1. 访问 [api.slack.com/apps](https://api.slack.com/apps)，点击 **Create New App**
2. 选择 **From scratch**，填写 App 名称，选择工作区
3. 在左侧菜单找到 **Incoming Webhooks**，点击开启（toggle on）
4. 点击 **Add New Webhook to Workspace**，选择目标频道（channel）
5. 复制生成的 Webhook URL（格式：`https://hooks.slack.com/services/T.../B.../xxx`）

### 在拾光VPS 中填写

| 字段 | 说明 |
|------|------|
| `webhook_url` | Slack 提供的 Incoming Webhook URL |

### 常见问题

- **restricted_action 错误**：工作区管理员可能限制了 Webhook 创建，联系管理员开放权限。
- **channel_not_found 错误**：Bot 没有该频道的权限，在 Slack 中将 Bot 添加到频道。

---

## 4. Email（邮件）

<!-- screenshot: Email 渠道配置表单，SMTP 服务器设置 -->

**所需信息**：SMTP 服务器信息

### 在拾光VPS 中填写

| 字段 | 说明 | 示例 |
|------|------|------|
| `smtp_host` | SMTP 服务器地址 | `smtp.gmail.com` |
| `smtp_port` | SMTP 端口 | `587`（STARTTLS）或 `465`（SSL） |
| `smtp_username` | SMTP 用户名（通常是邮箱地址） | `user@gmail.com` |
| `smtp_password` | SMTP 密码或应用专用密码 | |
| `smtp_from` | 发件人显示名称和地址 | `拾光VPS <user@gmail.com>` |
| `to` | 收件人地址（可多个，逗号分隔） | `admin@example.com` |
| `tls` | 是否启用 TLS | `true`（推荐） |

**常见邮件服务商 SMTP 配置**：

| 服务商 | host | port | 备注 |
|--------|------|------|------|
| Gmail | `smtp.gmail.com` | `587` | 需开启"应用专用密码" |
| QQ 邮箱 | `smtp.qq.com` | `587` | 需在 QQ 邮箱开启 SMTP，用授权码 |
| 163 邮箱 | `smtp.163.com` | `465` | 需开启 SMTP 并设置客户端授权码 |
| Outlook | `smtp.office365.com` | `587` | |
| 自建 | 自建服务器地址 | 自定义 | |

### 常见问题

- **Gmail 拒绝登录**：Gmail 要求使用"应用专用密码"而非账号密码，在 [Google 账号安全设置](https://myaccount.google.com/security) 中生成。
- **连接超时**：检查服务器防火墙是否放行 587 / 465 端口。

---

## 5. Bark（iOS）

<!-- screenshot: Bark App 主界面，显示设备 key -->

**所需信息**：`device_key`、`bark_server`（可选，默认官方服务器）

**Bark** 是一款 iOS 推送通知 App，支持自建服务器。

### 获取 device_key

1. 在 iPhone 上从 App Store 安装 [Bark](https://apps.apple.com/app/bark-customed-notifications/id1403753865)
2. 打开 App，首页会显示你的推送 URL，格式为：
   ```
   https://api.day.app/<device_key>/
   ```
3. 复制 `<device_key>` 部分（约 22 位字符）

### 在拾光VPS 中填写

| 字段 | 说明 | 示例 |
|------|------|------|
| `device_key` | Bark App 提供的设备密钥 | `ABcDeFgHiJkLmNoPqRsT` |
| `bark_server` | Bark 服务器地址（可选） | `https://api.day.app`（默认） |

### 常见问题

- **国内推送延迟**：Bark 依赖 Apple APNs，国内网络偶有延迟；自建 Bark Server 可改善。
- **收不到通知**：检查 iPhone 通知设置中 Bark 是否被允许推送（设置 → 通知 → Bark）。

---

## 6. Gotify

<!-- screenshot: Gotify 管理界面，应用列表和 token -->

**所需信息**：`server_url`、`token`

**Gotify** 是一个自建的推送通知服务器。

### 获取 token

1. 登录你的 Gotify 服务器管理界面（如 `https://gotify.example.com`）
2. 点击右上角 **Apps** → **Create Application**
3. 填写应用名称（如 `拾光VPS`），点击 **Create**
4. 复制生成的 **Token**

### 在拾光VPS 中填写

| 字段 | 说明 | 示例 |
|------|------|------|
| `server_url` | Gotify 服务器地址 | `https://gotify.example.com` |
| `token` | 应用 Token | `Abc123DefGhi...` |
| `priority` | 消息优先级（可选，1-10） | `5` |

### 常见问题

- **TLS 证书错误**：如果使用自签名证书，在拾光VPS 中可选择跳过 TLS 验证（不推荐生产环境）。
- **没有 Gotify 服务器**：参考 [Gotify 官方文档](https://gotify.net/docs/install) 一键 Docker 安装。

---

## 7. Webhook（自定义）

<!-- screenshot: Webhook 渠道配置，包含 URL、方法和 body 模板 -->

**所需信息**：目标 URL、HTTP 方法、Body 模板（可选）

Webhook 渠道允许你将通知发送到任意 HTTP 端点，适合对接自建系统、企业消息平台等。

### 在拾光VPS 中填写

| 字段 | 说明 | 示例 |
|------|------|------|
| `url` | 目标 HTTP(S) 端点 | `https://your-service.example/webhook` |
| `method` | HTTP 方法 | `POST` 或 `PUT` |
| `headers` | 自定义请求头（JSON 格式） | `{"Authorization": "Bearer xxx"}` |
| `body_template` | Go template 格式的 Body | 见下方示例 |
| `content_type` | Content-Type | `application/json`（默认） |

**Body 模板示例**（Go template 语法）：

```
{
  "text": "【拾光VPS】{{.EventType}}: {{.Message}}",
  "timestamp": "{{.Timestamp}}"
}
```

模板可用变量：

| 变量 | 说明 |
|------|------|
| `{{.EventType}}` | 事件类型（如 `node_offline`） |
| `{{.Message}}` | 事件描述文本 |
| `{{.Timestamp}}` | Unix 毫秒时间戳 |
| `{{.AgentName}}` | agent 名称（agent 相关事件） |
| `{{.SubName}}` | 订阅名称（订阅相关事件） |

### 常见问题

- **收不到请求**：确认 URL 可从拾光VPS 服务器公网访问，不要填写 `localhost` 类地址。
- **签名验证失败**：如果目标服务有签名验证，通过自定义 Header 传递签名密钥。

---

## 8. Server酱

<!-- screenshot: Server酱网站 SendKey 获取页面 -->

**所需信息**：`send_key`

**Server酱**（方糖）是国内常用的微信推送服务，适合不方便使用 Telegram 的用户。

### 获取 SendKey

1. 访问 [sct.ftqq.com](https://sct.ftqq.com/)
2. 用微信扫码登录
3. 在 **SendKey** 页面复制你的 SendKey（格式：`SCT123456Tabcdef...`）

### 在拾光VPS 中填写

| 字段 | 说明 | 示例 |
|------|------|------|
| `send_key` | Server酱 SendKey | `SCT123456Txxx...` |

### 常见问题

- **免费版有频率限制**：Server酱 免费版每天限 5 条，付费版限制更高。
- **消息延迟**：微信推送通常有 1-5 分钟延迟，属正常现象。

---

## 9. PushDeer

<!-- screenshot: PushDeer 配置界面，显示 pushkey -->

**所需信息**：`push_key`

**PushDeer** 是一款支持苹果 Apple Silicon Mac / iOS / 剪贴板的无服务商推送工具。

### 获取 push_key

1. 访问 [www.pushdeer.com](https://www.pushdeer.com/) 或从 App Store 安装 PushDeer
2. 登录后进入 **Key** 页面
3. 点击 **Add Key**，复制生成的 pushkey

### 在拾光VPS 中填写

| 字段 | 说明 | 示例 |
|------|------|------|
| `push_key` | PushDeer 提供的 pushkey | `PDU123...` |
| `server` | 自建 PushDeer 服务器地址（可选） | 留空使用官方服务器 |

---

## 10. IFTTT

<!-- screenshot: IFTTT Webhooks 服务配置界面 -->

**所需信息**：`webhook_key`、`event_name`

**IFTTT**（If This Then That）可以将拾光VPS 的通知与数百个第三方服务联动（如发邮件、控制智能家居等）。

### 获取 webhook_key

1. 登录 [ifttt.com](https://ifttt.com/)，点击右上角头像 → **Create**
2. 点击 **If This** → 搜索 `Webhooks` → 选择 **Receive a web request**
3. 填写 **Event Name**（如 `shiguang_alert`），点击 **Create trigger**
4. 点击 **Then That** 配置触发后的动作（如发邮件、推送到手机等）
5. 访问 [ifttt.com/maker_webhooks](https://ifttt.com/maker_webhooks)，点击 **Documentation**
6. 页面中会显示你的 **webhook_key**（格式：`dXXXXXXXXXXXXXXXXXXXXX`）

### 在拾光VPS 中填写

| 字段 | 说明 | 示例 |
|------|------|------|
| `webhook_key` | IFTTT Webhooks 文档页中的 key | `dAbcDeFg...` |
| `event_name` | 你在 IFTTT 中设置的事件名 | `shiguang_alert` |

IFTTT Webhook 调用格式（拾光VPS 自动处理）：
```
POST https://maker.ifttt.com/trigger/<event_name>/with/key/<webhook_key>
Body: {"value1": "事件类型", "value2": "消息内容", "value3": "时间戳"}
```

### 常见问题

- **事件名不匹配**：拾光VPS 中填写的 `event_name` 必须与 IFTTT Applet 中的 Event Name 完全一致（区分大小写）。
- **延迟较高**：IFTTT 免费版有时延迟 5-15 分钟，属正常现象。

---

## 事件类型说明

所有渠道都可以按事件类型 opt-in 订阅：

| 事件类型 | 触发条件 |
|----------|----------|
| `node_offline` | Agent 心跳超时（连续 N 次未收到心跳） |
| `traffic_alert` | 本月流量超过设定阈值（如 80%） |
| `sub_sync_failed` | 订阅自动同步失败（远端 5xx / 网络不可达） |
| `backup_done` | 备份导出完成 |
| `login_anomaly` | 登录异常（错误次数超限 / 新 IP 登录） |
| `ota_done` | OTA 升级完成 |
| `script_alert` | JS 脚本执行失败或主动调用 `alert()` |

**去抖机制**：同一事件在 5 分钟内只发送一条通知，避免频繁告警风暴（可在渠道设置中调整去抖时间）。
