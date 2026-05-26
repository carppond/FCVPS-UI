# VPS 资产管理 · 需求文档（PRD）

版本：v1.0
日期：2026-05-22
模块编号：M-ASSET

---

## 1. 需求概述

为拾光VPS 新增"VPS 资产管理"模块——让用户在同一个面板内记录和管理名下所有 VPS 的费用、配置、到期信息，自动计算到期状态并通过已有的 10 渠道通知系统推送到期提醒。解决 VPS 持有者"散落在各处的购买邮件/Excel 表格/脑子里"的管理痛点。

## 2. 目标用户

- **主要用户**：拥有 2-20 台 VPS 的个人技术爱好者 / 小机场主
- **痛点**：
  1. VPS 分散在多个商家（搬瓦工/Hetzner/腾讯云/...），到期日记不住
  2. 不知道每月/每年总花费多少
  3. VPS 过期被删数据才发现忘续费
  4. 想看每台 VPS 的探针指标但得跳多个页面

## 3. 用户故事

### admin 视角

| # | 用户故事 | 优先级 |
|---|---|---|
| 1 | 作为 admin，我希望新增一条 VPS 记录（名称/IP/卖家/价格/到期日等），以便集中管理我的资产 | **P0** |
| 2 | 作为 admin，我希望编辑和删除 VPS 记录，以便信息保持最新 | **P0** |
| 3 | 作为 admin，我希望看到每台 VPS 的到期状态（正常/即将到期/已到期），以便快速判断续费优先级 | **P0** |
| 4 | 作为 admin，我希望在 Dashboard 看到即将到期和已到期的 VPS 高亮显示（橙色/红色），以便不遗漏 | **P0** |
| 5 | 作为 admin，我希望到期前自动收到通知（邮件/Telegram/Bark 等），以便及时续费 | **P0** |
| 6 | 作为 admin，我希望给 VPS 关联一个已有的探针 agent，以便在 VPS 详情里直接看实时 CPU/内存/网络 | **P1** |
| 7 | 作为 admin，我希望记录 VPS 的 SSH 信息（IP/端口/用户名），以便快速复制去连接 | **P1** |
| 8 | 作为 admin，我希望看到名下所有 VPS 的月度/年度总费用，以便做预算 | **P1** |
| 9 | 作为 admin，我希望按卖家/状态/机房筛选 VPS 列表，以便快速找到目标 | **P1** |
| 10 | 作为 user，我希望只能看到自己创建的 VPS 记录，以便多用户互不干扰 | **P1** |

## 4. 功能清单

### 4.1 VPS 资产 CRUD（M-ASSET-1）

**字段定义**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| name | string | ✅ | VPS 名称（如 "hk-vps-01"） |
| ip | string | | IP 地址（IPv4 / IPv6） |
| ssh_port | int | | SSH 端口，默认 22 |
| ssh_user | string | | SSH 登录用户名 |
| os | string | | 操作系统（如 "Ubuntu 22.04"） |
| location | string | | 机房位置（如 "Hong Kong / 香港"） |
| provider | string | ✅ | 卖家/商家名称（如 "搬瓦工 / Hetzner / 腾讯云"） |
| price | decimal | ✅ | 价格数值 |
| currency | string | | 货币（CNY / USD / EUR / ...），默认 CNY |
| billing_cycle | enum | ✅ | 计费周期：monthly / quarterly / semi_annual / annual / biennial / triennial |
| bandwidth | string | | 带宽（如 "1Gbps" / "500Mbps"） |
| monthly_traffic | int | | 月流量配额 GB（0 = 不限） |
| cpu | string | | CPU 配置（如 "2 Core"） |
| memory | string | | 内存（如 "4 GB"） |
| disk | string | | 硬盘（如 "80 GB SSD"） |
| expire_at | date | ✅ | 到期日期 |
| notes | text | | 备注 |
| agent_id | FK | | 关联的探针 agent（可选） |
| tags | string[] | | 标签 |

**操作**：
- 新增：Dialog 表单
- 编辑：同表单
- 删除：确认 Dialog（不级联删除 agent）
- 列表：卡片网格（与订阅/节点页风格统一）

### 4.2 到期状态自动计算（M-ASSET-2）

后端在查询时动态计算，不存字段：

| 条件 | 状态 | 颜色 |
|---|---|---|
| 距到期 > 7 天 | `normal` 正常 | 绿色 |
| 0 < 距到期 ≤ 7 天 | `expiring` 即将到期 | 橙色 |
| 距到期 ≤ 0 | `expired` 已到期 | 红色 |

API 响应额外返回：
- `days_until_expiry: int`（负数 = 已过期天数）
- `status: "normal" | "expiring" | "expired"`

### 4.3 Dashboard 集成（M-ASSET-3）

在现有 Dashboard 的功能卡片区（4 卡片行）增加一张"VPS 资产"卡片：
- 显示：总数 / 即将到期数（橙）/ 已到期数（红）
- 点击跳转 `/vps-assets`

如果有即将到期或已到期的 VPS，在 Dashboard 事件流中也显示。

### 4.4 到期通知（M-ASSET-4）

新增通知事件类型 `vps_expiry`：
- 默认三档：提前 7 天 / 提前 3 天 / 当天
- 全局设置可调整天数
- 走现有 10 渠道通知系统（用户在事件订阅矩阵勾选 `vps_expiry`）
- 通知内容模板：`VPS "{name}" ({provider}) 将在 {days} 天后到期（{expire_at}）`
- 后台每天 00:00 跑一次检查

### 4.5 探针关联（M-ASSET-5）

- VPS 创建/编辑时，下拉选择已有的 agent（可选）
- VPS 详情页底部展示关联 agent 的实时指标（CPU/MEM/磁盘/网络 4 个指标卡）
- 未关联时显示"配置探针 →"引导按钮
- 不创建新 agent，只做关联（agent 在 /agents 页面创建）

### 4.6 费用统计（M-ASSET-6）

VPS 列表顶部汇总条显示：
- VPS 总数
- 月度总费用（所有 VPS 折算为月费）
- 即将到期数
- 已到期数

折算逻辑：
- monthly → 原价
- quarterly → 价格 / 3
- semi_annual → 价格 / 6
- annual → 价格 / 12
- biennial → 价格 / 24
- triennial → 价格 / 36

多币种暂不做汇率换算——按币种分组显示（如 "¥320/月 + $12.5/月"）。

## 5. 验收标准

| # | 验收条件 |
|---|---|
| AC-1 | 新增一条 VPS 记录，列表中出现且到期状态正确计算 |
| AC-2 | 编辑 VPS 的到期日为明天，状态变为"即将到期"（橙色） |
| AC-3 | 编辑到期日为昨天，状态变为"已到期"（红色） |
| AC-4 | Dashboard 的 VPS 卡片显示正确的即将到期/已到期数量 |
| AC-5 | 配置通知渠道 + 勾选 vps_expiry 事件 + 创建一个明天到期的 VPS → 次日 00:00 收到提醒 |
| AC-6 | VPS 关联 agent 后，详情页展示实时 CPU/MEM/磁盘/网络 |
| AC-7 | VPS 未关联 agent 时显示"配置探针"引导 |
| AC-8 | 汇总条的月度总费用折算正确（年付 $49.99 显示 $4.17/月） |
| AC-9 | 删除 VPS 不影响关联的 agent（agent 继续运行） |
| AC-10 | IP 字段旁有复制按钮，点击复制到剪贴板 |

## 6. 边界与约束

### 不做什么
- ❌ 不做 SSH 连接/Web Terminal（VPS 详情只记录 SSH 信息，不直接连）
- ❌ 不做自动续费/支付集成
- ❌ 不做多币种汇率换算（按币种分组显示）
- ❌ 不做 JSON 导入导出（用户明确砍掉）
- ❌ 不做 VPS 与节点的关联（节点来自订阅，跟 VPS 无直接关系）

### 技术约束
- 复用现有 SQLite 数据库
- 复用现有通知系统（新增事件类型 `vps_expiry`）
- 复用现有 agent 模块（只做关联，不新建 agent）
- 后端到期检查用 cron（每天 00:00），不做实时推送

### 与现有模块的关系
- Dashboard（M-OPS-29）：新增一张 VPS 卡片
- 通知系统（M-NOTIFY）：新增 `vps_expiry` 事件类型
- 探针（M-AGENT）：VPS 可选关联 agent，只读引用
- 其他模块：无关联

## 7. 开放问题

| # | 问题 | 当前假设 |
|---|---|---|
| 1 | 是否需要"批量操作"（批量延期/删除）？ | 暂不做，单条操作足够（VPS 数量通常 < 20） |
| 2 | 到期通知的去抖：同一台 VPS 7 天 + 3 天 + 当天发 3 次会不会烦？ | 用户可在事件矩阵取消勾选，或调整天数档位 |
| 3 | 是否需要"续费记录"（每次续费记一笔）？ | 暂不做，用户编辑到期日即可 |

## 8. 数据模型（参考）

```sql
CREATE TABLE IF NOT EXISTS vps_assets (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    ip TEXT,
    ssh_port INTEGER DEFAULT 22,
    ssh_user TEXT,
    os TEXT,
    location TEXT,
    provider TEXT NOT NULL,
    price REAL NOT NULL,
    currency TEXT DEFAULT 'CNY',
    billing_cycle TEXT NOT NULL CHECK (billing_cycle IN ('monthly','quarterly','semi_annual','annual','biennial','triennial')),
    bandwidth TEXT,
    monthly_traffic INTEGER DEFAULT 0,
    cpu TEXT,
    memory TEXT,
    disk TEXT,
    expire_at TEXT NOT NULL,
    notes TEXT,
    agent_id TEXT,
    tags TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL
);
CREATE INDEX idx_vps_assets_user ON vps_assets(user_id);
CREATE INDEX idx_vps_assets_expire ON vps_assets(expire_at);
```

## 9. 页面规划

| 页面 | 路径 | 功能 |
|---|---|---|
| VPS 列表 | `/vps-assets` | 汇总条 + 筛选 + 卡片网格 |
| VPS 详情 | `/vps-assets/:id` | 完整信息 + 关联探针指标 |
| 新增/编辑 | Dialog 弹窗 | 表单 |
| Dashboard 卡片 | `/dashboard` 内嵌 | 到期统计 + 跳转 |
