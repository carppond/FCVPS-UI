# 拾光VPS API 契约文档

版本：v1.0  
日期：2026-05-20  
输入：`docs/01-requirements.md` + `docs/02-ui-design.md` + `docs/03-architecture.md` + `docs/CONTEXT.md`  
范围：全部 102 个 HTTP endpoint + WebSocket 协议 + 错误码 + 数据模型 DTO

**修改本文件时必须同步更新：**
- `internal/types/api.go`
- `internal/types/wsproto.go`
- `web/src/types/api.ts`
- `web/src/types/wsproto.ts`
- `pkg/agentlib/protocol.go`

---

## 1. 接口总览（按模块分组）

### 全局约定

- **Base URL**（静默模式开启时）：`/_app/<32hex>/api/`；关闭时：`/api/`。前端通过 `lib/silent-prefix.ts` 自动注入前缀。
- **鉴权**：HTTP header `Authorization: Bearer <token>`；WebSocket query `?token=<token>`。
- **响应格式**：统一 `APIResponse<T>`，见第 2 节。
- **时间戳**：所有时间字段均为 Unix 毫秒整数（`int64`）。
- **分页**：`?page=1&page_size=20`，响应体含 `total`。

鉴权标签说明：

| 标签 | 含义 |
|------|------|
| `[public]` | 无需登录，但仍受静默模式 404 保护 |
| `[pending2fa]` | 已通过密码登录，但未完成 2FA 验证 |
| `[user]` | 完整登录（已过 2FA 或未启用 2FA） |
| `[admin]` | 已登录且 role=admin |
| `[agent-token]` | agent 专用 token，错误则 404 |
| `[public+token]` | 无需登录 session，但须携带专用 token |

---

### M-USER：认证与用户

#### 5.1.2 认证端点

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 1 | POST | `/api/auth/login` | 用户名密码登录 | `[public]` |
| 2 | POST | `/api/auth/verify-totp` | 验证 TOTP 六位码 | `[pending2fa]` |
| 3 | POST | `/api/auth/verify-recovery` | 用备份码登录 | `[pending2fa]` |
| 4 | POST | `/api/auth/logout` | 登出（作废 token） | `[user]` |
| 5 | POST | `/api/auth/refresh` | 滑动续期 session（延长 TTL） | `[user]` |
| 6 | GET  | `/api/me` | 获取当前用户信息 | `[user]` |
| 7 | PATCH | `/api/me` | 修改用户名 / locale / email | `[user]` |
| 8 | POST | `/api/me/password` | 修改密码 | `[user]` |
| 9 | DELETE | `/api/me` | 删除自己账号 | `[user]` |
| 10 | GET | `/api/me/totp/setup` | 生成 TOTP secret + QR Code URI | `[user]` |
| 11 | POST | `/api/me/totp/enable` | 验证 code，启用 2FA | `[user]` |
| 12 | POST | `/api/me/totp/disable` | 验证 code+密码，关闭 2FA | `[user]` |
| 13 | POST | `/api/me/totp/recovery-codes` | 重新生成备份码（需验证密码） | `[user]` |
| 14 | GET | `/api/me/sessions` | 列出活跃 session | `[user]` |
| 15 | DELETE | `/api/me/sessions/:id` | 吊销指定 session | `[user]` |

**POST /api/auth/login 详情**

请求 Body：
```json
{
  "username": "admin",
  "password": "Aj7$kK9pXm2Q"
}
```

响应（未启用 2FA）：
```json
{
  "data": {
    "access_token": "base64url_32bytes",
    "expires_at": 1716278400000,
    "user": { /* UserPublicProfile */ }
  }
}
```

响应（已启用 2FA，进入 pending 状态）：
```json
{
  "data": {
    "pending_token": "base64url_32bytes",
    "expires_in": 300,
    "totp_required": true
  }
}
```

**POST /api/auth/verify-totp 详情**

请求 Body：
```json
{
  "pending_token": "base64url_32bytes",
  "code": "123456"
}
```

响应：同 login 成功响应。

---

#### 5.1.3 用户管理端点（admin 专属）

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 16 | GET | `/api/admin/users` | 列出所有用户（支持分页+搜索） | `[admin]` |
| 17 | POST | `/api/admin/users` | 创建用户 | `[admin]` |
| 18 | GET | `/api/admin/users/:id` | 用户详情 | `[admin]` |
| 19 | PATCH | `/api/admin/users/:id` | 修改用户（角色/状态/邮件） | `[admin]` |
| 20 | DELETE | `/api/admin/users/:id` | 删除用户（级联删除所有资源） | `[admin]` |
| 21 | POST | `/api/admin/users/:id/reset-password` | 重置用户密码（返回新临时密码） | `[admin]` |
| 22 | POST | `/api/admin/users/:id/disable-2fa` | 强制关闭某用户 2FA | `[admin]` |

---

### M-SUB：订阅管理

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 23 | GET | `/api/subscriptions` | 列出我的订阅（admin 可看全部） | `[user]` |
| 24 | POST | `/api/subscriptions` | 创建订阅（url/upload/manual） | `[user]` |
| 25 | GET | `/api/subscriptions/:id` | 订阅详情（含节点统计） | `[user]` |
| 26 | PATCH | `/api/subscriptions/:id` | 修改订阅元数据（名称/标签/UA/周期） | `[user]` |
| 27 | DELETE | `/api/subscriptions/:id` | 删除订阅（级联删除节点） | `[user]` |
| 28 | POST | `/api/subscriptions/:id/sync` | 立即触发同步 | `[user]` |
| 29 | GET | `/api/subscriptions/:id/raw` | 查看原始内容（YAML/Base64） | `[user]` |
| 30 | GET | `/api/subscriptions/:id/output` | 输出 Clash YAML（经流水线处理） | `[user]` |
| 31 | POST | `/api/subscriptions/upload` | 上传 YAML 文件（multipart/form-data） | `[user]` |
| 32 | GET | `/api/subscriptions/:id/pipelines` | 列出绑定的流水线 | `[user]` |
| 33 | PUT | `/api/subscriptions/:id/pipelines` | 重置绑定（全量替换，含排序） | `[user]` |

**GET /api/subscriptions 查询参数：**
- `page`、`page_size`（默认 20）
- `owner_id`（admin 专用，过滤特定用户）
- `keyword`（名称模糊搜索）

---

### M-NODE：节点管理

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 34 | GET | `/api/subscriptions/:id/nodes` | 列出订阅下节点（支持 tag/protocol 过滤） | `[user]` |
| 35 | POST | `/api/subscriptions/:id/nodes` | 手动添加节点（URI 字符串） | `[user]` |
| 36 | GET | `/api/nodes/:id` | 节点详情（含 raw_uri） | `[user]` |
| 37 | PATCH | `/api/nodes/:id` | 修改 tags / chain_parent_id | `[user]` |
| 38 | DELETE | `/api/nodes/:id` | 删除节点 | `[user]` |
| 39 | POST | `/api/nodes/tcping` | 批量 TCPing（最多 200 节点，并发 50） | `[user]` |
| 40 | POST | `/api/nodes/:id/chain` | 设置/取消链式代理出口 | `[user]` |

**POST /api/nodes/tcping 请求 Body：**
```json
{
  "node_ids": ["uuid1", "uuid2"],
  "timeout_ms": 5000,
  "concurrency": 50
}
```

响应：
```json
{
  "data": {
    "results": [
      { "node_id": "uuid1", "latency_ms": 42, "reachable": true },
      { "node_id": "uuid2", "latency_ms": -1, "reachable": false }
    ]
  }
}
```

---

### M-PIPE：算子流水线

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 41 | GET | `/api/pipelines` | 列出我的流水线 | `[user]` |
| 42 | POST | `/api/pipelines` | 创建流水线 | `[user]` |
| 43 | GET | `/api/pipelines/:id` | 详情（含 yaml_content + ast_json） | `[user]` |
| 44 | PUT | `/api/pipelines/:id` | 整体保存（双更 yaml 和 ast） | `[user]` |
| 45 | DELETE | `/api/pipelines/:id` | 删除流水线 | `[user]` |
| 46 | POST | `/api/pipelines/:id/run` | Dry-run（在指定订阅上预览，不写库） | `[user]` |
| 47 | POST | `/api/pipelines/yaml-to-ast` | YAML → AST 转换（无副作用） | `[user]` |
| 48 | POST | `/api/pipelines/ast-to-yaml` | AST → YAML 转换（无副作用） | `[user]` |
| 49 | GET | `/api/pipelines/operators` | 列出可用算子 + 参数 JSON schema | `[user]` |

**POST /api/pipelines/:id/run 请求 Body：**
```json
{
  "subscription_id": "uuid",
  "debug": true
}
```

响应（debug=true 时含每算子 diff）：
```json
{
  "data": {
    "total_ms": 287,
    "output_count": 38,
    "steps": [
      {
        "operator": "filter",
        "input_count": 56,
        "output_count": 38,
        "removed": ["HK-US-01"],
        "added": [],
        "modified": []
      }
    ]
  }
}
```

---

### M-RULE：规则系统

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 50 | GET | `/api/rules` | 列出自定义规则（按 type 过滤） | `[user]` |
| 51 | POST | `/api/rules` | 创建规则 | `[user]` |
| 52 | PATCH | `/api/rules/:id` | 修改规则内容/模式 | `[user]` |
| 53 | DELETE | `/api/rules/:id` | 删除规则 | `[user]` |
| 54 | PUT | `/api/rules/order` | 批量修改 sort（全量替换排序） | `[user]` |
| 55 | GET | `/api/rules/preview/:subID` | 预览注入后的完整 Clash YAML | `[user]` |
| 56 | GET | `/api/rules/templates` | 预设模板列表 | `[user]` |

---

### M-SCRIPT：脚本扩展

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 57 | GET | `/api/scripts` | 列出脚本（按 hook 过滤） | `[user]` |
| 58 | POST | `/api/scripts` | 创建脚本 | `[user]` |
| 59 | PATCH | `/api/scripts/:id` | 修改脚本代码/名称 | `[user]` |
| 60 | DELETE | `/api/scripts/:id` | 删除脚本 | `[user]` |
| 61 | POST | `/api/scripts/:id/test` | 用样例数据 dry-run（不写库） | `[user]` |
| 62 | GET | `/api/scripts/:id/logs` | 查最近执行错误日志 | `[user]` |

---

### M-AGENT：探针 Agent

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 63 | GET | `/api/agents` | 列出我的 agent | `[user]` |
| 64 | POST | `/api/agents` | 创建 agent（生成 token，仅此一次返回明文） | `[user]` |
| 65 | GET | `/api/agents/:id` | agent 详情（含最新指标快照） | `[user]` |
| 66 | PATCH | `/api/agents/:id` | 修改名称/标签 | `[user]` |
| 67 | DELETE | `/api/agents/:id` | 删除 agent + 吊销 token；`?uninstall=true` 时若 agent 在线先下发自卸载命令（best-effort） | `[user]` |
| 68 | POST | `/api/agents/:id/rotate-token` | 轮换 token（旧 token 立即失效） | `[user]` |
| 69 | POST | `/api/agents/:id/restart` | 下发 restart 命令（WebSocket 通道） | `[user]` |
| 70 | GET | `/api/agents/:id/records` | 高频原始记录（支持 from/to 时间范围） | `[user]` |
| 71 | GET | `/api/admin/agents` | admin 查看全系统 agent 列表 | `[admin]` |

---

### M-TRAFFIC：流量聚合

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 72 | GET | `/api/traffic/summary` | 当月流量概览（限额/已用/剩余） | `[user]` |
| 73 | GET | `/api/traffic/chart` | 趋势图数据（query: range=day\|month, from, to） | `[user]` |
| 74 | GET | `/api/traffic/by-agent` | 按 agent 拆分的流量数据 | `[user]` |
| 75 | POST | `/api/traffic/threshold` | 设置流量告警阈值 | `[user]` |

---

### M-NOTIFY：通知系统

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 76 | GET | `/api/notify/channels` | 列出通知通道 | `[user]` |
| 77 | POST | `/api/notify/channels` | 创建通知通道 | `[user]` |
| 78 | PATCH | `/api/notify/channels/:id` | 修改通道配置 | `[user]` |
| 79 | DELETE | `/api/notify/channels/:id` | 删除通道 | `[user]` |
| 80 | POST | `/api/notify/channels/:id/test` | 发送测试消息 | `[user]` |
| 81 | GET | `/api/notify/channel-kinds` | 列出所有可用通道类型 + 参数 schema | `[user]` |
| 82 | GET | `/api/notify/event-types` | 列出所有可订阅的事件类型 | `[user]` |
| 83 | GET | `/api/notify/events` | 投递历史（query: status, from, to） | `[user]` |
| 84 | POST | `/api/notify/telegram/webhook/:token` | Telegram Bot Webhook 接收端点 | `[public+token]` |
| 85 | GET | `/api/notify/stream` | 前端 SSE 实时事件流 | `[user]` |

---

### M-OPS：短链 / OTA / 设置 / 审计

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 86 | GET | `/api/shortlinks` | 我的短链列表 | `[user]` |
| 87 | POST | `/api/shortlinks` | 创建短链 | `[user]` |
| 88 | DELETE | `/api/shortlinks/:fileCode/:userCode` | 删除短链 | `[user]` |
| 89 | GET | `/api/admin/ota/check` | 检查是否有新版本 | `[admin]` |
| 90 | POST | `/api/admin/ota/apply` | 触发 OTA 自升级 | `[admin]` |
| 91 | GET | `/api/admin/ota/history` | OTA 升级历史 | `[admin]` |
| 92 | GET | `/api/admin/settings` | 读取系统设置 | `[admin]` |
| 93 | PATCH | `/api/admin/settings` | 修改系统设置 | `[admin]` |
| 94 | POST | `/api/admin/settings/silent-mode` | 开关静默模式 / 轮换前缀 | `[admin]` |
| 95 | GET | `/api/admin/audit` | 审计日志（支持分页+过滤） | `[admin]` |
| 96 | POST | `/api/admin/backup` | 触发备份（下载或 S3） | `[admin]` |
| 97 | POST | `/api/admin/restore` | 从备份恢复 | `[admin]` |

---

### 公开 / 兼容路径

| # | 方法 | 路径 | 用途 | 鉴权 |
|---|------|------|------|------|
| 98 | GET | `/_app/<32hex>/*` | 前端 SPA（embed.FS 服务） | `[public]` |
| 99 | GET | `/healthz` | 健康检查（可配置关闭） | `[public]` |
| 100 | GET | `/s/:code` | 短链跳转（302） | `[public]` |
| 101 | GET | `/download/:name` | sub-store 兼容订阅下载 | `[public+token]` |
| 102 | GET | `/api/agent/ws` | agent ↔ hub WebSocket 长连接 | `[agent-token]` |

（Nezha 兼容：`POST /api/v1/nezha/heartbeat`、`POST /api/v1/nezha/report` 计入 public+token 类别，合计共 102 个端点。）

---

## 2. 数据模型（DTO）

### 2.1 通用响应包装

```
APIResponse<T> {
  code?:       string    // 成功时省略；错误时为 ERR_XXX_YYY
  data?:       T         // 成功载荷
  message?:    string    // 人类可读消息（前端 i18n 渲染 code 替代此字段）
  details?:    object    // 可选额外信息（如校验错误字段列表）
  request_id?: string    // trace UUID
}
```

分页响应：

```
PagedResponse<T> {
  items:      T[]
  total:      int64
  page:       int
  page_size:  int
}
```

---

### 2.2 User

| 字段 | 类型 | 必填 | 约束 | 示例 |
|------|------|------|------|------|
| `id` | string | 是 | UUID v7 | `"01j8k4..."` |
| `username` | string | 是 | 3-32 字符，唯一 | `"admin"` |
| `role` | UserRole | 是 | `"admin"` \| `"user"` | `"admin"` |
| `is_active` | boolean | 是 | - | `true` |
| `email` | string | 否 | 邮件格式 | `"a@b.com"` |
| `locale` | string | 是 | `"zh-CN"` \| `"en"` \| `"ja"` \| `"ko"` | `"zh-CN"` |
| `totp_enabled` | boolean | 是 | - | `false` |
| `created_at` | int64 | 是 | unix ms | `1716190000000` |
| `updated_at` | int64 | 是 | unix ms | `1716190000000` |

### 2.3 UserPublicProfile（客户端可见子集）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | - |
| `username` | string | 是 | - |
| `role` | UserRole | 是 | - |
| `email` | string | 否 | - |
| `locale` | string | 是 | - |
| `totp_enabled` | boolean | 是 | - |
| `created_at` | int64 | 是 | - |

---

### 2.4 Subscription

| 字段 | 类型 | 必填 | 约束 | 示例 |
|------|------|------|------|------|
| `id` | string | 是 | UUID v7 | - |
| `user_id` | string | 是 | - | - |
| `name` | string | 是 | 最长 100 字符 | `"机场A"` |
| `type` | SubType | 是 | `"url"` \| `"upload"` \| `"manual"` | `"url"` |
| `source_url` | string | 否 | 当 type=url 时必填 | - |
| `ua` | string | 否 | 自定义 User-Agent | - |
| `sync_interval` | int32 | 是 | 秒，默认 21600（6h） | `21600` |
| `last_synced_at` | int64 | 否 | unix ms | - |
| `last_sync_status` | SyncStatus | 否 | `"ok"` \| `"error"` \| `"pending"` | - |
| `last_sync_error` | string | 否 | 错误信息 | - |
| `expire_at` | int64 | 否 | unix ms | - |
| `traffic_total` | int64 | 否 | 字节；0=未知 | `536870912000` |
| `traffic_used` | int64 | 否 | 字节 | `236273664` |
| `tags` | string[] | 是 | - | `["HK","游戏"]` |
| `remark` | string | 否 | - | - |
| `node_count` | int32 | 是 | 当前节点数 | `56` |
| `created_at` | int64 | 是 | - | - |
| `updated_at` | int64 | 是 | - | - |

### 2.5 SubscriptionDetail（含节点列表）

继承 `Subscription` 全部字段，额外增加：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `nodes` | Node[] | 是 | 分页节点列表 |
| `nodes_total` | int32 | 是 | 总节点数 |
| `pipeline_bindings` | PipelineBinding[] | 是 | 挂载的流水线 |

---

### 2.6 Node

| 字段 | 类型 | 必填 | 约束 | 示例 |
|------|------|------|------|------|
| `id` | string | 是 | UUID v7 | - |
| `subscription_id` | string | 是 | - | - |
| `raw_uri` | string | 是 | 原始协议 URI | `"vmess://..."` |
| `protocol` | NodeProtocol | 是 | 12 种协议之一 | `"vless"` |
| `server` | string | 是 | - | `"hk1.example.com"` |
| `port` | int32 | 是 | 1-65535 | `443` |
| `tag` | string | 是 | proxy name | `"HK-Premium-01"` |
| `tags` | string[] | 是 | 用户/算子标签 | `["HK","游戏"]` |
| `is_chain_proxy` | boolean | 是 | - | `false` |
| `chain_parent_id` | string | 否 | 链式代理父节点 ID | - |
| `parsed_config` | object | 是 | 协议特定字段（见各子类型） | - |
| `position` | int32 | 是 | 在订阅内的顺序 | `0` |
| `created_at` | int64 | 是 | - | - |
| `updated_at` | int64 | 是 | - | - |

### 2.7 NodeWithLatency

继承 `Node` 全部字段，额外增加：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `latency_ms` | int32 | 是 | -1 = 不可达 |
| `reachable` | boolean | 是 | - |
| `tested_at` | int64 | 否 | unix ms |

---

### 2.8 Pipeline

| 字段 | 类型 | 必填 | 约束 | 示例 |
|------|------|------|------|------|
| `id` | string | 是 | UUID v7 | - |
| `user_id` | string | 是 | - | - |
| `name` | string | 是 | 最长 100 字符 | `"机场A 流水线"` |
| `yaml_content` | string | 是 | apiVersion: shiguang/v1 | - |
| `ast_json` | string | 是 | 编译后 AST（JSON 字符串） | - |
| `version` | int32 | 是 | 乐观锁版本 | `1` |
| `schema_version` | string | 是 | 固定 `"shiguang/v1"` | - |
| `created_at` | int64 | 是 | - | - |
| `updated_at` | int64 | 是 | - | - |

### 2.9 PipelineOperator（AST 算子节点）

| 字段 | 类型 | 必填 | 约束 | 示例 |
|------|------|------|------|------|
| `id` | string | 是 | 客户端生成 UUID（用于拖拽标识） | - |
| `type` | OperatorType | 是 | `"filter"` \| `"map"` \| `"sort"` \| `"dedupe"` \| `"regex_rename"` \| `"output"` | `"filter"` |
| `enabled` | boolean | 是 | - | `true` |
| `params` | object | 是 | 各算子专属参数（见 2.10） | - |
| `position` | int32 | 是 | 执行顺序 | `0` |

### 2.10 算子 Params

**FilterArgs**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `expr` | string | 是 | 过滤表达式，如 `"region in ['hk','jp']"` |

**MapArgs**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `field` | string | 是 | 目标字段名 |
| `value` | string | 是 | 新值（支持模板变量 `{{.Name}}`） |

**SortArgs**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `key` | string | 是 | 排序字段（`"latency"` \| `"name"` \| `"tag"`） |
| `order` | string | 是 | `"asc"` \| `"desc"` |

**DedupeArgs**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `fields` | string[] | 是 | 去重依据字段列表，如 `["server","port"]` |

**RegexRenameArgs**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `pattern` | string | 是 | 正则表达式（Go RE2 语法） |
| `replacement` | string | 是 | 替换串（支持 `$1` 捕获组） |

**OutputArgs**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `format` | string | 是 | `"clash"` \| `"clash_meta"` \| `"raw"` |
| `max_nodes` | int32 | 否 | 最大输出节点数，0=不限 |

---

### 2.11 PipelineBinding

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `subscription_id` | string | 是 | - |
| `pipeline_id` | string | 是 | - |
| `position` | int32 | 是 | 执行顺序 |
| `enabled` | boolean | 是 | - |

---

### 2.12 CustomRule

| 字段 | 类型 | 必填 | 约束 | 示例 |
|------|------|------|------|------|
| `id` | string | 是 | UUID v7 | - |
| `user_id` | string | 是 | - | - |
| `name` | string | 是 | 最长 100 字符 | - |
| `type` | RuleType | 是 | `"dns"` \| `"rules"` \| `"rule-providers"` | - |
| `mode` | RuleMode | 是 | `"replace"` \| `"prepend"` \| `"append"` | - |
| `content` | string | 是 | YAML 片段 | - |
| `enabled` | boolean | 是 | - | `true` |
| `sort` | int32 | 是 | 注入顺序 | `0` |
| `created_at` | int64 | 是 | - | - |
| `updated_at` | int64 | 是 | - | - |

---

### 2.13 Script

| 字段 | 类型 | 必填 | 约束 | 示例 |
|------|------|------|------|------|
| `id` | string | 是 | UUID v7 | - |
| `user_id` | string | 是 | - | - |
| `name` | string | 是 | - | - |
| `hook` | HookType | 是 | `"pre_save_nodes"` \| `"post_fetch"` | - |
| `code` | string | 是 | JS 代码（goja 沙箱） | - |
| `enabled` | boolean | 是 | - | `true` |
| `last_run_at` | int64 | 否 | unix ms | - |
| `last_error` | string | 否 | 最近一次执行错误 | - |
| `created_at` | int64 | 是 | - | - |
| `updated_at` | int64 | 是 | - | - |

---

### 2.14 Agent

| 字段 | 类型 | 必填 | 约束 | 示例 |
|------|------|------|------|------|
| `id` | string | 是 | UUID v7 | - |
| `user_id` | string | 是 | - | - |
| `name` | string | 是 | - | `"vps-hk-01"` |
| `kind` | AgentKind | 是 | `"native"` \| `"nezha_compat"` | `"native"` |
| `version` | string | 否 | semver | `"1.0.0"` |
| `os` | string | 否 | - | `"linux"` |
| `arch` | string | 否 | - | `"amd64"` |
| `public_ip` | string | 否 | - | `"1.2.3.4"` |
| `last_seen_at` | int64 | 否 | unix ms | - |
| `status` | AgentStatus | 是 | `"online"` \| `"offline"` \| `"degraded"` | `"online"` |
| `created_at` | int64 | 是 | - | - |
| `updated_at` | int64 | 是 | - | - |

**创建 agent 时额外返回（仅一次）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `token` | string | 明文 token（base64url 32 bytes），仅此一次 |
| `install_command` | string | 一键安装命令示例 |

### 2.15 AgentMetric（实时指标快照）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `agent_id` | string | 是 | - |
| `recorded_at` | int64 | 是 | unix ms |
| `cpu_percent` | float64 | 是 | 0-100 |
| `mem_used` | int64 | 是 | 字节 |
| `mem_total` | int64 | 是 | 字节 |
| `swap_used` | int64 | 是 | 字节 |
| `swap_total` | int64 | 是 | 字节 |
| `disk_used` | int64 | 是 | 字节 |
| `disk_total` | int64 | 是 | 字节 |
| `net_in` | int64 | 是 | 累计字节 |
| `net_out` | int64 | 是 | 累计字节 |
| `net_in_speed` | int64 | 是 | B/s 瞬时 |
| `net_out_speed` | int64 | 是 | B/s 瞬时 |
| `load1` | float64 | 是 | - |
| `load5` | float64 | 是 | - |
| `load15` | float64 | 是 | - |
| `conn_tcp` | int32 | 是 | TCP 连接数 |
| `conn_udp` | int32 | 是 | UDP 连接数 |
| `uptime` | int64 | 是 | 秒 |
| `process_count` | int32 | 否 | 进程数 |

---

### 2.16 TrafficRecord

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `date` | string | 是 | `YYYY-MM-DD` |
| `user_id` | string | 是 | - |
| `agent_id` | string | 否 | - |
| `total_limit` | int64 | 否 | 字节；null=无限额 |
| `total_used` | int64 | 是 | 字节 |
| `total_in` | int64 | 是 | 字节 |
| `total_out` | int64 | 是 | 字节 |

### 2.17 TrafficSummary

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | string | 是 | - |
| `period_start` | string | 是 | `YYYY-MM-DD` |
| `period_end` | string | 是 | `YYYY-MM-DD` |
| `total_limit` | int64 | 否 | 字节 |
| `total_used` | int64 | 是 | 字节 |
| `total_in` | int64 | 是 | 字节 |
| `total_out` | int64 | 是 | 字节 |
| `usage_percent` | float64 | 是 | 0-100 |
| `agents` | AgentTrafficSummary[] | 是 | 按 agent 拆分 |

---

### 2.18 NotificationChannel

| 字段 | 类型 | 必填 | 约束 | 示例 |
|------|------|------|------|------|
| `id` | string | 是 | UUID v7 | - |
| `user_id` | string | 是 | - | - |
| `kind` | ChannelKind | 是 | 10 种渠道之一 | `"telegram"` |
| `name` | string | 是 | 用户自定义名称 | `"我的 TG Bot"` |
| `config` | object | 是 | 渠道特定配置（见 2.19） | - |
| `template` | string | 否 | Go template 字符串；空=用默认 | - |
| `event_types` | EventType[] | 是 | 订阅的事件类型列表 | - |
| `enabled` | boolean | 是 | - | `true` |
| `created_at` | int64 | 是 | - | - |
| `updated_at` | int64 | 是 | - | - |

### 2.19 通知渠道 Config 类型

**TelegramConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `bot_token` | string | 是 | BotFather 下发的 token |
| `chat_id` | string | 是 | 个人/群组 Chat ID |
| `parse_mode` | string | 否 | `"HTML"` \| `"Markdown"` |

**DiscordConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `webhook_url` | string | 是 | Discord Webhook URL |
| `username` | string | 否 | 显示名称 |

**SlackConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `webhook_url` | string | 是 | Slack Incoming Webhook URL |
| `channel` | string | 否 | 频道名（#general） |

**EmailConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `smtp_host` | string | 是 | - |
| `smtp_port` | int32 | 是 | - |
| `smtp_user` | string | 是 | - |
| `smtp_password` | string | 是 | 加密存储 |
| `smtp_tls` | boolean | 是 | 是否 STARTTLS/TLS |
| `from` | string | 是 | 发件人地址 |
| `to` | string[] | 是 | 收件人列表 |

**BarkConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `device_key` | string | 是 | Bark App device key |
| `server_url` | string | 否 | 自建 Bark 服务地址；默认 https://api.day.app |

**GotifyConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `server_url` | string | 是 | Gotify server URL |
| `app_token` | string | 是 | 应用 token |

**WebhookConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 是 | Webhook URL |
| `method` | string | 否 | `"POST"` \| `"GET"`；默认 POST |
| `headers` | map[string]string | 否 | 自定义请求头 |

**ServerChanConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `send_key` | string | 是 | Server酱 SendKey |

**PushDeerConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `push_key` | string | 是 | PushDeer PushKey |
| `server_url` | string | 否 | 自建地址 |

**IFTTTConfig**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `webhook_key` | string | 是 | IFTTT Webhook Key |
| `event_name` | string | 是 | 触发器事件名 |

---

### 2.20 NotificationEvent（投递记录）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | int64 | 是 | 自增 ID |
| `user_id` | string | 是 | - |
| `channel_id` | string | 否 | 关联通道（删除后为 null） |
| `event_type` | EventType | 是 | 事件类型 |
| `payload` | object | 是 | 事件原始 payload |
| `status` | EventStatus | 是 | `"pending"` \| `"sent"` \| `"failed"` \| `"skipped_dedupe"` |
| `sent_at` | int64 | 否 | unix ms |
| `error` | string | 否 | 失败原因 |
| `created_at` | int64 | 是 | - |

---

### 2.21 ShortLink

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file_code` | string | 是 | 文件码 |
| `user_code` | string | 是 | 用户码 |
| `user_id` | string | 是 | - |
| `target_url` | string | 是 | 目标长 URL |
| `short_url` | string | 是 | 完整短链（`/s/<file_code><user_code>`） |
| `expires_at` | int64 | 否 | unix ms；null=永久 |
| `created_at` | int64 | 是 | - |

---

### 2.22 AuditLog

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | int64 | 是 | 自增 ID |
| `user_id` | string | 否 | 操作者 |
| `action` | string | 是 | 操作名称（如 `"login"` / `"create_sub"`） |
| `resource_type` | string | 否 | 资源类型 |
| `resource_id` | string | 否 | 资源 ID |
| `ip` | string | 否 | 客户端 IP |
| `user_agent` | string | 否 | - |
| `payload` | object | 否 | 详细 JSON |
| `success` | boolean | 是 | - |
| `created_at` | int64 | 是 | - |

---

### 2.23 SystemSettings

| Key | 值类型 | 默认值 | 说明 |
|-----|--------|--------|------|
| `silent_mode_enabled` | boolean | `true` | 是否开启静默模式 |
| `silent_mode_prefix` | string | 随机 32hex | 登录前缀 |
| `session_ttl_seconds` | int64 | `86400` | Session TTL |
| `monthly_reset_day` | int32 | `1` | 流量月度重置日 |
| `ota_check_interval` | int32 | `86400` | OTA 检查间隔（秒） |
| `agent_heartbeat_interval` | int32 | `30` | agent 心跳（5-300 秒） |
| `notification_debounce` | int32 | `300` | 通知去抖窗口（秒） |

---

## 3. 错误码定义

所有错误响应格式：

```json
{
  "code": "ERR_AUTH_INVALID_PASSWORD",
  "message": "用户名或密码错误",
  "details": {},
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| `ERR_AUTH_INVALID_PASSWORD` | 401 | 用户名或密码错误 |
| `ERR_AUTH_USER_INACTIVE` | 403 | 用户已被禁用 |
| `ERR_AUTH_TOTP_REQUIRED` | 202 | 密码通过但需 2FA（返回 pending_token） |
| `ERR_AUTH_TOTP_INVALID` | 401 | TOTP 验证码错误 |
| `ERR_AUTH_TOTP_EXPIRED` | 401 | TOTP pending_token 已过期（5 分钟） |
| `ERR_AUTH_RECOVERY_CODE_INVALID` | 401 | 备份码无效或已使用 |
| `ERR_AUTH_RECOVERY_CODE_EXHAUSTED` | 403 | 备份码已全部用尽 |
| `ERR_AUTH_TOKEN_INVALID` | 401 | Bearer token 无效 |
| `ERR_AUTH_TOKEN_EXPIRED` | 401 | Bearer token 已过期 |
| `ERR_AUTH_RATE_LIMITED` | 429 | 登录频率超限（5 次/小时） |
| `ERR_AUTH_BRUTE_FORCE_BLOCKED` | 429 | IP 暴力破解被封锁（封禁 1 小时） |
| `ERR_AUTH_FORBIDDEN` | 403 | 权限不足（需要 admin） |
| `ERR_VALIDATION_REQUIRED_FIELD` | 400 | 必填字段缺失（details 含字段名） |
| `ERR_VALIDATION_INVALID_FORMAT` | 400 | 字段格式错误（details 含字段名和期望格式） |
| `ERR_VALIDATION_OUT_OF_RANGE` | 400 | 字段值超出合法范围 |
| `ERR_VALIDATION_REGEX_COMPILE` | 400 | 正则表达式编译失败 |
| `ERR_VALIDATION_YAML_PARSE` | 400 | YAML 解析失败 |
| `ERR_VALIDATION_SCHEMA_MISMATCH` | 400 | Pipeline YAML schema 版本不匹配 |
| `ERR_NOT_FOUND_USER` | 404 | 用户不存在 |
| `ERR_NOT_FOUND_SUBSCRIPTION` | 404 | 订阅不存在 |
| `ERR_NOT_FOUND_NODE` | 404 | 节点不存在 |
| `ERR_NOT_FOUND_PIPELINE` | 404 | 流水线不存在 |
| `ERR_NOT_FOUND_RULE` | 404 | 规则不存在 |
| `ERR_NOT_FOUND_SCRIPT` | 404 | 脚本不存在 |
| `ERR_NOT_FOUND_AGENT` | 404 | Agent 不存在 |
| `ERR_NOT_FOUND_CHANNEL` | 404 | 通知通道不存在 |
| `ERR_CONFLICT_USERNAME` | 409 | 用户名已被占用 |
| `ERR_CONFLICT_PIPELINE_VERSION` | 409 | 流水线版本冲突（乐观锁，需刷新后重试） |
| `ERR_CONFLICT_LAST_ADMIN` | 409 | 最后一个管理员账号不可删除（自删除 / 管理员删除均拦截） |
| `ERR_PIPELINE_OPERATOR_UNKNOWN` | 422 | 未知算子类型 |
| `ERR_PIPELINE_OPERATOR_PARAMS` | 422 | 算子参数校验失败 |
| `ERR_PIPELINE_RUN_TIMEOUT` | 408 | 流水线执行超时（> 5s） |
| `ERR_SCRIPT_TIMEOUT` | 408 | 脚本执行超时（> 5s） |
| `ERR_SCRIPT_SANDBOX_VIOLATION` | 422 | 沙箱违规（fs/net 访问被拒） |
| `ERR_SCRIPT_RUNTIME_ERROR` | 422 | goja 运行时错误（details 含堆栈） |
| `ERR_AGENT_TOKEN_INVALID` | 404 | Agent token 无效（静默模式下返 404） |
| `ERR_AGENT_OFFLINE` | 409 | Agent 当前离线，无法下发命令 |
| `ERR_AGENT_COMMAND_TIMEOUT` | 408 | 命令下发超时 |
| `ERR_INTERNAL_DATABASE` | 500 | 数据库操作失败 |
| `ERR_INTERNAL_UNKNOWN` | 500 | 未预期的内部错误（details 含 request_id） |

---

## 4. 认证机制

### 4.1 登录流程

```
1. 前端 POST /api/auth/login { username, password }
        ↓
2a. 未启用 2FA：
   hub 验证密码 → 生成 access_token（随机 32 字节 base64url）
   → 写 sessions 表（token_hash = sha256(token), pending_2fa=0）
   → 响应 { access_token, expires_at, user }

2b. 已启用 2FA：
   hub 验证密码 → 生成 pending_token（随机 32 字节 base64url，5 分钟 TTL）
   → 写 sessions 表（pending_2fa=1）
   → 响应 { pending_token, totp_required: true }

3. 用户提交 TOTP code：
   POST /api/auth/verify-totp { pending_token, code }
   → hub 验证 TOTP → 更新 sessions（pending_2fa=0）
   → 响应 { access_token, expires_at, user }

   （或使用备份码：POST /api/auth/verify-recovery { pending_token, code }）
```

### 4.2 Token 格式

- **不是 JWT**；格式：`base64url(random_32_bytes)`，示例：`dGhpcyBpcyBhIHRlc3Q`。
- 数据库存储：`sha256(token)` 的 hex 字符串。
- 默认 TTL：24 小时（`system_settings.session_ttl_seconds`）。
- 每次请求自动滑动续期（`POST /api/auth/refresh` 或 middleware 自动）。

### 4.3 Token 传递

| 场景 | 方式 |
|------|------|
| HTTP API | `Authorization: Bearer <token>` |
| WebSocket (agent) | `?token=<token>` query param |
| WebSocket (前端 SSE) | `Authorization: Bearer <token>` header 或 cookie |
| sub-store 兼容 | `/download/:name?token=<token>` query param |

### 4.4 备份码救援流程

```
1. 用户首次启用 2FA 时，系统生成 8 个 8 位 hex 备份码。
2. 前端强制要求用户勾选"我已保存"后才能关闭对话框。
3. 备份码以 sha256 哈希数组形式存入 users.recovery_codes_hash（JSON array）。
4. 登录时选择"用备份码"：
   POST /api/auth/verify-recovery { pending_token, code: "a3f8b9c2" }
5. hub 验证 sha256(code) 是否在数组中 → 命中则从数组删除（一次性消耗）。
6. 最后一个备份码用完后返回 ERR_AUTH_RECOVERY_CODE_EXHAUSTED。
7. 用户可在 /profile/2fa 重新生成一批新备份码（需验证当前密码）。
```

---

## 5. WebSocket 协议

### 5.1 hub ↔ agent 协议

**连接建立**

```
agent → GET /api/agent/ws?token=<token>
hub   → 101 Switching Protocols（token 无效则 404）
```

**消息信封格式（所有消息共用）**

```json
{
  "type": "hello",
  "id": "msg-uuid-v4",
  "payload": { /* 消息体 */ },
  "ts": 1716190000000
}
```

**消息类型及 Payload**

#### `hello`（agent → hub，握手）

```json
{
  "agent_id": "uuid",
  "token": "base64url_32bytes",
  "os": "linux",
  "arch": "amd64",
  "version": "1.0.0",
  "kind": "native",
  "capabilities": ["metrics", "restart"]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `agent_id` | string | agent UUID（必须与注册的 ID 匹配） |
| `token` | string | 明文 token |
| `os` | string | 操作系统 |
| `arch` | string | 架构 |
| `version` | string | agent 版本号（semver） |
| `kind` | string | `"native"` \| `"nezha_compat"` |
| `capabilities` | string[] | 支持的能力列表 |

#### `hello_ack`（hub → agent，握手响应）

```json
{
  "ok": true,
  "heartbeat_interval": 30,
  "hub_version": "1.0.0"
}
```

#### `heartbeat`（agent → hub，每 30 秒）

```json
{
  "agent_id": "uuid",
  "uptime": 86400
}
```

#### `metrics`（agent → hub，随心跳发送）

```json
{
  "agent_id": "uuid",
  "cpu_percent": 12.5,
  "mem_used": 524288000,
  "mem_total": 2147483648,
  "swap_used": 0,
  "swap_total": 0,
  "disk_used": 10737418240,
  "disk_total": 53687091200,
  "net_in": 1073741824,
  "net_out": 536870912,
  "net_in_speed": 102400,
  "net_out_speed": 51200,
  "load1": 0.5,
  "load5": 0.3,
  "load15": 0.2,
  "conn_tcp": 128,
  "conn_udp": 32,
  "uptime": 86400,
  "process_count": 95
}
```

#### `cmd`（hub → agent，命令下发）

```json
{
  "cmd": "restart",
  "args": {}
}
```

| cmd 值 | v1 状态 | 说明 |
|--------|---------|------|
| `restart` | v1 | 重启 agent 进程 |
| `refresh_subscription` | v1 | 触发订阅立即同步 |
| `collect_now` | P2 预留 | 立即采集并上报 |
| `shutdown` | P2 预留 | 关闭 agent |

#### `cmd_ack`（agent → hub）

```json
{
  "cmd_id": "msg-uuid",
  "ok": true,
  "error": ""
}
```

#### `bye`（任意方向，断开通知）

```json
{
  "reason": "token_rotated"
}
```

---

### 5.2 前端 ↔ hub（SSE 实时推流）

**端点**：`GET /api/notify/stream`（Content-Type: `text/event-stream`）

**事件格式**：

```
event: agent_status
data: {"agent_id":"uuid","status":"online","ts":1716190000000}

event: notification_event
data: {"event_type":"node_offline","channel_id":"uuid","status":"sent","ts":1716190000000}

event: subscription_sync
data: {"subscription_id":"uuid","status":"ok","node_count":56,"ts":1716190000000}

event: system
data: {"kind":"ota_available","version":"1.1.0","ts":1716190000000}
```

---

## 6. sub-store 兼容路由

### GET /download/:name?token=\<token\>

- `name`：订阅名称（对应 subscriptions.name，URL encoded）
- `token`：订阅专用 token（与登录 token 不同，从短链或订阅详情页获取）
- 响应：Clash YAML 格式（Content-Type: `text/yaml`）
- token 错误或订阅不存在：返回 404（静默模式联动）

### GET /api/utils/env（sub-store 客户端环境探测）

响应：
```json
{
  "args": {
    "target": "clash",
    "url": "...",
    "insert": "false"
  },
  "headers": {
    "User-Agent": "Clash/...",
    "X-Sub-Store-Version": "2.x"
  }
}
```

---

## 7. Nezha agent 兼容路由

### POST /api/v1/nezha/heartbeat

Nezha agent v2 心跳接收端点。payload 必须与原 Nezha v2 协议最小字段集一致：

```json
{
  "State": {
    "CPU": 12.5,
    "MemUsed": 524288000,
    "SwapUsed": 0,
    "DiskUsed": 10737418240,
    "NetInTransfer": 1073741824,
    "NetOutTransfer": 536870912,
    "NetInSpeed": 102400,
    "NetOutSpeed": 51200,
    "Load1": 0.5,
    "Load5": 0.3,
    "Load15": 0.2,
    "TcpConnCount": 128,
    "UdpConnCount": 32,
    "ProcessCount": 95
  },
  "Host": {
    "Platform": "linux",
    "PlatformVersion": "Ubuntu 22.04",
    "CPU": ["Intel Xeon E5-2670 x4"],
    "MemTotal": 2147483648,
    "DiskTotal": 53687091200,
    "SwapTotal": 0,
    "Arch": "amd64",
    "Virtualization": "kvm",
    "BootTime": 1716103600
  }
}
```

响应：
```json
{ "code": 0, "message": "ok" }
```

token 通过 HTTP header `Authorization: Bearer <token>` 或 query `?secret=<token>` 传递（兼容 Nezha 两种方式）。

---

## 附录：枚举类型速查

| 枚举类型 | 值列表 |
|----------|--------|
| `UserRole` | `"admin"` \| `"user"` |
| `SubType` | `"url"` \| `"upload"` \| `"manual"` |
| `SyncStatus` | `"ok"` \| `"error"` \| `"pending"` |
| `NodeProtocol` | `"vmess"` \| `"vless"` \| `"ss"` \| `"ssr"` \| `"trojan"` \| `"hysteria"` \| `"hysteria2"` \| `"tuic"` \| `"wireguard"` \| `"anytls"` \| `"socks5"` \| `"naive"` |
| `OperatorType` | `"filter"` \| `"map"` \| `"sort"` \| `"dedupe"` \| `"regex_rename"` \| `"output"` |
| `RuleType` | `"dns"` \| `"rules"` \| `"rule-providers"` |
| `RuleMode` | `"replace"` \| `"prepend"` \| `"append"` |
| `HookType` | `"pre_save_nodes"` \| `"post_fetch"` |
| `AgentKind` | `"native"` \| `"nezha_compat"` |
| `AgentStatus` | `"online"` \| `"offline"` \| `"degraded"` |
| `ChannelKind` | `"telegram"` \| `"discord"` \| `"slack"` \| `"email"` \| `"bark"` \| `"gotify"` \| `"webhook"` \| `"serverchan"` \| `"pushdeer"` \| `"ifttt"` |
| `EventType` | `"node_offline"` \| `"traffic_threshold"` \| `"subscription_sync_failed"` \| `"backup_completed"` \| `"login_anomaly"` \| `"ota_available"` \| `"script_alert"` |
| `EventStatus` | `"pending"` \| `"sent"` \| `"failed"` \| `"skipped_dedupe"` |
| `WsMessageType` | `"hello"` \| `"hello_ack"` \| `"heartbeat"` \| `"metrics"` \| `"cmd"` \| `"cmd_ack"` \| `"bye"` |
