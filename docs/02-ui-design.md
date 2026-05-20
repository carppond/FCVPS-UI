# 拾光VPS UI/UX 设计方案

版本：v1.0
日期：2026-05-20
设计师角色：资深 UI/UX 设计师
对应需求文档：`docs/01-requirements.md`（PRD v1.0）
风格定位：**技术工具感 > 消费品质感**，服务于"读密集表格 + 实时图表 + 拖拽编辑器"的复合场景。

---

## 视觉风格

### 选定风格：Minimalism / Swiss（瑞士国际主义设计风格）

**强制选定，不允许混搭其它风格。**

### 选择理由

1. **场景适配度最高**：项目核心场景是"表格密集 + 实时数据 + 拖拽流水线 + 终端输出"，全部都是**信息密度高、需要长时间注视**的工作流。Swiss 风格的"网格 + 大量留白 + 单一强调色"是这种场景在 UI 史上最被验证过的解。
2. **暗色友好**：技术爱好者偏好暗色面板，而 Swiss 风格的"细 hairline 边框 + 极简扁平 + 中性灰阶"在暗色下不会因为透明度叠加导致信息丢失（Glassmorphism / Neumorphism 在暗色下表格几乎不可读）。
3. **多语言友好**：4 套语言（zh-CN / en / ja / ko），Swiss 风格强调 **functional typography + grid alignment**，文案换语言时长度差异不会破坏视觉平衡；CJK 字体在大量留白衬托下识读性最佳。
4. **延续行业惯例**：参考项目（妙妙屋 / Nezha / Beszel / Komari）和事实标准库 shadcn/ui、Radix UI、Vercel Dashboard、Linear、GitHub Primer 几乎都收敛到 Swiss / Minimalism 这条线，社区资源最丰富、技术债最小。
5. **差异化核心组件友好**：算子流水线编辑器（拖拽 + YAML diff）和命令面板（Cmd+K）这类"工具感"组件，在 Swiss 风格下最自然——任何彩色装饰都会让"专业工具"变成"玩具"。

### 风格关键词（10 条）

1. `generous whitespace` —— 大量留白，间距优先于装饰
2. `functional typography` —— 字体即层级，禁止用色彩区分层级
3. `1px hairline border` —— 用 1px 细线分隔区域，禁止粗边框
4. `monochrome with single accent` —— 90% 中性灰 + 一个强调色（青蓝）
5. `grid-based layout` —— 8 列 / 12 列网格对齐，禁止飘字符位
6. `subtle elevation` —— 阴影只用于 popover / modal，不用于卡片
7. `numeric-first` —— 数字（流量 / 延迟 / 时间戳）用 tabular-nums 等宽对齐
8. `tonal hierarchy` —— 同色相不同明度表达层级（bg / surface / elevated）
9. `data-ink ratio first` —— 图表去除多余轴线刻度，墨水都给数据
10. `keyboard-first` —— 所有核心动作都有快捷键，鼠标是辅助

### Anti-patterns（5 条，绝对不做）

1. **禁止 gradient / 渐变装饰**：仅图表数据填充允许 alpha 渐变，按钮 / 卡片 / banner 不允许任何渐变。
2. **禁止彩色硬阴影 + 阴影 ≥ 4 层**：暗色下黑阴影叠黑无效；阴影只用 `rgba(0,0,0,0.3-0.6)` 软阴影，且不超过 3 个层级。
3. **禁止圆角 < 4px 或 > 16px**：4-12 是统一区间；表单 / 按钮 / 卡片 / 弹窗在此区间内分配。
4. **禁止使用 ≥ 3 种主色**：全站只有一个强调色（青蓝 #00B8D9 系列）+ 4 个语义色（success/warning/error/info），不允许引入紫 / 粉 / 橙作为装饰。
5. **禁止 glassmorphism / neumorphism / 拟物**：任何模糊背景、浮雕、3D 投影、纹理填充都不允许。表格背景必须 solid color。

---

## Design Tokens

### 色板 —— 暗色（默认）

```yaml
# Primary（青蓝，技术工具感 + 不与 success 蓝色撞色）
primary:            "#00B8D9"     # 主强调色（按钮 / 链接 / focus ring）
primary-hover:      "#33C8E1"
primary-active:     "#0095AF"
primary-foreground: "#001A1F"     # primary 上的文字

# Neutral（10 阶 + 950）—— 主基调
neutral-0:    "#000000"
neutral-50:   "#0A0A0B"
neutral-100:  "#0F1011"           # bg
neutral-200:  "#16181C"           # bg-elevated
neutral-300:  "#1D2025"           # surface
neutral-400:  "#272A30"           # border-subtle
neutral-500:  "#3A3D44"           # border
neutral-600:  "#5C606A"           # border-strong / text-disabled
neutral-700:  "#8A8E98"           # text-tertiary
neutral-800:  "#B4B8C0"           # text-secondary
neutral-900:  "#E4E6EB"           # text-primary
neutral-950:  "#FFFFFF"

# 语义色（每色 + foreground）
success:             "#22C55E"
success-foreground:  "#04210E"
success-bg:          "#0A2418"    # 浅底用于 banner

warning:             "#F59E0B"
warning-foreground:  "#1F1300"
warning-bg:          "#2A1E08"

error:               "#EF4444"
error-foreground:    "#240606"
error-bg:            "#2A1212"

info:                "#3B82F6"
info-foreground:     "#050F24"
info-bg:             "#0E1A2E"

# 角色色（暗色）
bg:                  "#0F1011"    # 整体背景
bg-elevated:         "#16181C"    # 主区背景
surface:             "#1D2025"    # 卡片 / 表格行底
surface-hover:       "#22262C"
border:              "#272A30"    # 默认 1px 边框
border-strong:       "#3A3D44"    # 强分隔 / 输入框 focus 前
text-primary:        "#E4E6EB"
text-secondary:      "#B4B8C0"
text-tertiary:       "#8A8E98"
text-disabled:       "#5C606A"

# 节点 / agent 状态色（探针特有）
status-online:       "#22C55E"    # 在线，绿
status-offline:      "#EF4444"    # 离线，红
status-degraded:     "#F59E0B"    # 异常（高延迟 / 部分丢包），橙
status-unknown:      "#8A8E98"    # 未知 / 未上报，灰
```

### 色板 —— 亮色

```yaml
primary:            "#0095AF"
primary-hover:      "#00B8D9"
primary-active:     "#007891"
primary-foreground: "#FFFFFF"

neutral-0:    "#FFFFFF"
neutral-50:   "#FAFAFA"
neutral-100:  "#F4F5F7"           # bg
neutral-200:  "#EBEDEF"           # bg-elevated
neutral-300:  "#E1E4E8"           # surface
neutral-400:  "#D0D4DA"           # border-subtle
neutral-500:  "#B4B8C0"           # border
neutral-600:  "#8A8E98"           # text-tertiary
neutral-700:  "#5C606A"           # text-secondary
neutral-800:  "#272A30"           # text-primary
neutral-900:  "#16181C"
neutral-950:  "#000000"

success:             "#16A34A"
success-foreground:  "#FFFFFF"
success-bg:          "#DCFCE7"

warning:             "#D97706"
warning-foreground:  "#FFFFFF"
warning-bg:          "#FEF3C7"

error:               "#DC2626"
error-foreground:    "#FFFFFF"
error-bg:            "#FEE2E2"

info:                "#2563EB"
info-foreground:     "#FFFFFF"
info-bg:             "#DBEAFE"

bg:                  "#F4F5F7"
bg-elevated:         "#FFFFFF"
surface:             "#FFFFFF"
surface-hover:       "#F4F5F7"
border:              "#E1E4E8"
border-strong:       "#D0D4DA"
text-primary:        "#16181C"
text-secondary:      "#5C606A"
text-tertiary:       "#8A8E98"
text-disabled:       "#B4B8C0"

status-online:       "#16A34A"
status-offline:      "#DC2626"
status-degraded:     "#D97706"
status-unknown:      "#8A8E98"
```

### 字体配对（Google Fonts）

**必须同时引入 4 套 Noto + Inter + JetBrains Mono，让浏览器按 unicode-range 自动落字。**

```yaml
font-display:
  family: "'Inter', 'Noto Sans SC', 'Noto Sans JP', 'Noto Sans KR', system-ui, sans-serif"
  weights: [500, 600, 700]
  use: "标题 / 数字大屏 / 导航品牌"

font-body:
  family: "'Inter', 'Noto Sans SC', 'Noto Sans JP', 'Noto Sans KR', system-ui, sans-serif"
  weights: [400, 500]
  use: "正文 / 表格 / 表单 / 按钮"

font-mono:
  family: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'SF Mono', Consolas, monospace"
  weights: [400, 500]
  use: "代码 / YAML / 日志 / 终端 / 数字（流量 / 延迟 / 时间戳，启用 tabular-nums）"

# 字号阶梯（px）
font-size:
  xs:   "11px"     # 辅助文字（表格 footer / tooltip）
  sm:   "12px"     # 表格内容 / 表单 label / metadata
  base: "14px"     # 正文默认
  lg:   "16px"     # 卡片标题 / 模块小标
  xl:   "18px"     # 页面副标
  2xl:  "24px"     # 页面主标
  3xl:  "32px"     # Dashboard 数字大屏
  4xl:  "48px"     # 错误页 / 静默页占位大字

# 行高阶梯
line-height:
  tight:   "1.2"   # 大屏数字 / 标题
  normal:  "1.5"   # 正文
  relaxed: "1.7"   # 长描述 / 帮助文字

# 字重
font-weight:
  normal:    400
  medium:    500
  semibold:  600
  bold:      700

# 字符特性
font-feature:
  tabular-nums: '"tnum" 1'    # 所有数字列表 / 流量数 / 延迟数必须开
  ligatures:    '"liga" 1'    # mono 字体的代码连字
```

### 间距阶梯（4px 基准）

```yaml
space-0:    "0px"
space-0.5:  "2px"     # icon 内边
space-1:    "4px"     # 紧凑 chip / badge 内边
space-1.5:  "6px"
space-2:    "8px"     # 表单 label-input 间距
space-3:    "12px"    # 按钮 padding-y
space-4:    "16px"    # 卡片内边 / 表格 cell
space-6:    "24px"    # 卡片之间 / 模块间距
space-8:    "32px"    # 区域分隔
space-12:   "48px"    # 主区域 padding
space-16:   "64px"    # 大屏顶部留白
space-20:   "80px"
space-24:   "96px"    # 登录页内容垂直居中辅助
```

### 圆角

```yaml
radius-none:  "0px"     # 无（紧贴边的元素）
radius-sm:    "4px"     # 标签 / chip / badge
radius-md:    "6px"     # 输入框 / 小按钮
radius-lg:    "8px"     # 大按钮 / 表格圆角 / 卡片
radius-xl:    "12px"    # 弹窗 / Drawer
radius-2xl:   "16px"    # 大型 illustration 容器
radius-full:  "9999px"  # 头像 / 圆形按钮 / 状态点
```

### 阴影（暗色场景下不叠黑阴影 → 用 alpha 软阴影）

```yaml
shadow-none:  "none"

# 暗色阴影（rgba 黑色低透明）
shadow-sm-dark:    "0 1px 2px 0 rgba(0,0,0,0.4)"
shadow-md-dark:    "0 4px 8px -2px rgba(0,0,0,0.5), 0 2px 4px -2px rgba(0,0,0,0.3)"
shadow-lg-dark:    "0 12px 24px -6px rgba(0,0,0,0.6), 0 4px 8px -4px rgba(0,0,0,0.4)"
shadow-xl-dark:    "0 24px 48px -12px rgba(0,0,0,0.7)"

# 亮色阴影
shadow-sm-light:   "0 1px 2px 0 rgba(16,18,28,0.06)"
shadow-md-light:   "0 4px 8px -2px rgba(16,18,28,0.08), 0 2px 4px -2px rgba(16,18,28,0.04)"
shadow-lg-light:   "0 12px 24px -6px rgba(16,18,28,0.12), 0 4px 8px -4px rgba(16,18,28,0.06)"
shadow-xl-light:   "0 24px 48px -12px rgba(16,18,28,0.18)"

# inset（用于 active 按钮 / 输入框）
shadow-inset:      "inset 0 1px 2px 0 rgba(0,0,0,0.2)"
```

### 动效

```yaml
duration-fast:        "150ms"    # hover / focus 变色
duration-normal:      "250ms"    # 弹窗 / drawer 打开
duration-slow:        "350ms"    # 复杂布局重排
duration-extra-slow:  "500ms"    # 流水线节点拖拽吸附

easing-out:           "cubic-bezier(0.2, 0.8, 0.2, 1)"     # 入场（最常用）
easing-in-out:        "cubic-bezier(0.4, 0, 0.2, 1)"       # 双向（如手风琴展开）
easing-spring:        "cubic-bezier(0.34, 1.56, 0.64, 1)"  # 流水线节点回弹（限定使用）
```

### 数据可视化色板（Chart palette，7 色 categorical）

**用于多探针 / 多订阅 / 多协议折线图、堆叠柱状图。在暗色和亮色下均通过 WCAG AA 对比。**

```yaml
chart-1: "#00B8D9"     # 青（主色，第一条线）
chart-2: "#7C5CFF"     # 紫
chart-3: "#22C55E"     # 绿
chart-4: "#F59E0B"     # 橙
chart-5: "#EC4899"     # 玫粉
chart-6: "#06B6D4"     # 浅青
chart-7: "#A3A3A3"     # 灰（兜底 / "其它"分类）

# 流量趋势区域填充用同色 alpha=0.15
chart-area-alpha: 0.15

# 图表网格 / 轴线
chart-grid:  "neutral-400"   # 暗色 1px 细线
chart-axis:  "neutral-600"
chart-label: "text-tertiary"
```

### Z-index 层级

```yaml
z-base:        0
z-dropdown:    1000
z-sticky:      1100
z-banner:      1200
z-overlay:     1300
z-modal:       1400
z-popover:     1500
z-toast:       1600
z-tooltip:     1700
z-command-palette: 1800   # Cmd+K 优先级最高
```

---

## 页面/视图列表

| # | 路由 | 名称 | 角色 | 备注 |
|---|------|------|------|------|
| 1 | `/_app/<random>/login` | 登录页 | 任意 | 静默模式前缀，含用户名+密码 |
| 2 | `/_app/<random>/login/2fa` | 2FA 验证页 | 任意 | TOTP 6 位输入 |
| 3 | `/_app/<random>/login/recovery` | 备份码救援页 | 任意 | 8 位 hex 输入 |
| 4 | `/dashboard` | 总览 Dashboard | admin/user | 首页，多看板汇总 |
| 5 | `/subscriptions` | 订阅管理列表 | admin/user | 表格 + 批量操作 |
| 6 | `/subscriptions/:id` | 订阅详情 | admin/user | 节点列表 + 元数据 + 挂载流水线 |
| 7 | `/subscriptions/:id/pipeline` | 算子流水线编辑器 ★ | admin/user | 拖拽 + YAML 双视图 + 调试 |
| 8 | `/nodes` | 节点管理列表 | admin/user | 全局节点视图，跨订阅 |
| 9 | `/nodes/:id` | 节点详情面板 | admin/user | 右侧抽屉，可复制 raw URI |
| 10 | `/rules` | 规则管理 | admin/user | custom_rules CRUD + 最终配置预览 |
| 11 | `/scripts` | 脚本管理 | admin/user | goja JS 编辑器 + 错误日志 |
| 12 | `/agents` | agent 列表 | admin/user | 含 Nezha 兼容标识 |
| 13 | `/agents/:id` | agent 详情 | admin/user | CPU/MEM/Disk/NetIO 实时图 |
| 14 | `/traffic` | 流量统计 | admin/user | 多探针 + 多订阅汇总图 |
| 15 | `/notifications` | 通知中心 | admin/user | 10 渠道配置 + 事件 opt-in 矩阵 |
| 16 | `/notifications/telegram` | Telegram Bot 设置 | admin/user | inline keyboard 命令配置 |
| 17 | `/settings` | 系统设置 | admin | 静默模式 / OTA / 备份 / i18n |
| 18 | `/users` | 用户管理 | admin | user CRUD + 重置密码 |
| 19 | `/profile` | 个人资料 | admin/user | 改密 / 改用户名 / 删除账号 |
| 20 | `/profile/2fa` | 2FA 设置 | admin/user | 启用 / 关闭 / 重生备份码 |
| 21 | `/audit` | 审计日志 | admin | 表格 + 过滤 + 导出 |
| 22 | `/404` | 404 / 静默占位页 | 任意 | 伪装为 nginx 默认页 |

**总计：22 个视图（含 5 个登录前 / 5 个登录后子页 + 12 个主导航页）**

---

## 信息架构

### 顶层导航树（侧栏菜单）

```
拾光VPS
│
├── [User 视图] (M-USER-1 普通用户)
│   ├── 总览                    /dashboard
│   ├── 订阅管理                /subscriptions
│   │   └── 流水线编辑器 ★      /subscriptions/:id/pipeline
│   ├── 节点管理                /nodes
│   ├── 规则管理                /rules
│   ├── 脚本管理                /scripts
│   ├── 探针 Agent              /agents
│   ├── 流量统计                /traffic
│   ├── 通知中心                /notifications
│   │   └── Telegram Bot       /notifications/telegram
│   └── [底部] 个人资料         /profile
│       └── 2FA 设置            /profile/2fa
│
└── [Admin 视图] (M-USER-1 管理员，多出以下菜单)
    ├── 总览                    /dashboard         (全系统范围)
    ├── 订阅管理                /subscriptions     (可见所有用户的订阅)
    ├── ... (同 User 视图)
    ├── ── ── ── ── ── ── ──    (分组分隔线)
    ├── 用户管理                /users             ★ admin 专属
    ├── 系统设置                /settings          ★ admin 专属
    ├── 审计日志                /audit             ★ admin 专属
    └── [底部] 个人资料 + OTA 入口
```

### 顶栏（AppShell Top Bar）固定元素

```
左侧：               中间：                              右侧：
品牌 logo + 名称     全局命令面板 Cmd+K 快捷入口          通知 bell  |  主题切换  |  语言切换  |  头像下拉
拾光VPS              [搜索框：搜节点 / 订阅 / agent]      (Toast)     (亮/暗/系统)  (zh/en/ja/ko)  (个人/退出)
```

### 角色差异（admin 与 user 的 UI 差异点）

| 区域 | admin 看到 | user 看到 |
|------|-----------|-----------|
| 侧栏菜单 | 多出 3 项：用户管理 / 系统设置 / 审计日志 | 不显示 |
| 总览 Dashboard | "全系统"维度统计 + 顶部用户切换器 | 仅本人维度 |
| 订阅列表 | 显示 owner 列 + 可过滤所有用户 | 只显示本人订阅 |
| Agent 列表 | 显示 owner 列 | 只显示本人 agent |
| 通知中心 | 顶部多 "系统级通道（下发给所有用户）" tab | 只看个人通道 |
| 设置页 | 完整 | 仅显示"个人偏好"小节 |

---

## 组件设计

### 1. AppShell（顶栏 + 侧栏 + 主区）

- **用途**：所有登录后页面的外壳。
- **结构**：
  - 顶部固定 56px 高度 TopBar
  - 左侧 240px 宽 Sidebar（可折叠到 64px 仅 icon）
  - 主区域 padding 24px，最大宽度无限制（表格密集）
  - 右侧可弹出 Drawer（节点详情等），固定 480px 宽
- **状态**：
  - `sidebar-collapsed` / `sidebar-expanded`（持久化到 localStorage）
  - `drawer-open` / `drawer-closed`

### 2. Sidebar（含角色差异化菜单）

- **元素**：品牌 logo (40px)、菜单组（分组带分隔线）、菜单项（icon + label + 可选 badge）、底部用户名头像 + 折叠按钮
- **状态**：
  - 正常 / hover（surface-hover）/ active（左侧 2px primary 竖条 + 文字 primary）
  - admin 专属菜单组上方有 `border-top + label "管理"`
  - collapsed 状态下仅显示 icon，hover 显示 tooltip
- **菜单项 icon**：用 lucide-react 16px

### 3. DataTable（节点 / 订阅 / agent / 审计等通用）

- **用途**：项目核心组件，70% 的页面都是表格。
- **元素**：
  - 顶部工具栏：搜索框（占左）/ 过滤 chips / 视图切换 / 列设置 / 导出 / 批量操作按钮
  - 表头：固定（粘性）、可排序（点击列名）、可拖拽调整列宽
  - 表身：行 hover 高亮（surface-hover）、行选中（左侧 2px primary）
  - 行 padding：纵 12px / 横 16px
  - 字体：`font-body 14px`，数字列必须 `font-mono + tabular-nums`
  - 行分隔：1px border-subtle
  - 空行高度：56px
  - 底部：分页（页码 + 跳转 + 每页数量选择）+ 总数
- **状态**：
  - normal / loading（顶部 2px primary progress + 行替换为 skeleton）
  - error（顶部 banner + retry 按钮）
  - empty（中央 illustration + 主提示 + 副提示 + CTA 按钮）
  - selected（行选中态 + 顶部出现批量操作工具栏）
- **响应式**：手机端整张表变卡片列表（每行 → 一张卡片，列变成 key-value）

### 4. StatusBadge

- **用途**：表达 online / offline / degraded / unknown
- **元素**：8px 圆点 + 8px 间距 + label 文字（12px sm）
- **变体**：
  - `online`：绿点 + "在线"
  - `offline`：红点 + "离线"
  - `degraded`：橙点 + "异常"
  - `unknown`：灰点 + "未知"
- **动效**：`online` 状态下圆点有 1.5s pulse 动画（仅在 Dashboard 等强调位置）

### 5. TrafficChart（折线 + 区域填充）

- **用途**：流量趋势 / 延迟趋势 / agent CPU 趋势
- **元素**：
  - 图表区（recharts 实现）
  - 顶部：标题 + 时间范围切换（1h / 24h / 7d / 30d）+ 导出 PNG
  - 折线：2px 粗，使用 chart-1 ~ chart-7
  - 区域填充：alpha 0.15
  - tooltip：黑底白字 + tabular-nums 数字 + 多 series 横向并列
  - 网格：仅水平虚线（chart-grid 1px dashed）
  - 轴：仅显示 5-6 个刻度，标签 font-mono 11px
- **状态**：loading（skeleton 占位）/ error（提示重试）/ empty（中央"暂无数据，等待 agent 上报"）

### 6. PipelineCanvas ★（算子流水线编辑器，差异化核心）

- **用途**：M-PIPE 模块的核心组件，sub-store 风格但带 GUI 拖拽。
- **结构**（三栏布局）：
  ```
  [240 算子库] [中间 自适应 画布] [320 参数面板]
                                    │
                                    └── 顶部 tab 可切到 YAML 视图
  ```
- **左栏 算子库**：6 种算子卡片可拖出（filter / map / sort / dedupe / regex-rename / output），每卡 80x60，含 icon + 中文名 + 英文 type
- **中栏 画布**：
  - 垂直流式布局（从上到下顺序执行）
  - 算子卡片之间有 24px 连接线（向下箭头 + 当前节点数 badge）
  - 卡片宽度撑满（约 480px），高度 80px
  - 卡片可拖拽重排（@dnd-kit）
  - 卡片右上角 ⋯ 菜单：复制 / 删除 / 启用-禁用
  - 卡片选中时左侧 2px primary 竖条
  - 顶部固定一个不可移除的 "原始节点列表"输入节点
- **右栏 参数面板**：
  - 当前选中算子的参数表单（react-hook-form）
  - 例如 filter：表达式输入框 + 表达式手册链接
  - 底部"调试预览"按钮 → 触发"运行到此算子"展示 diff
- **YAML 视图**（顶部 tab 切换）：
  - 左半 monaco editor（YAML 编辑）/ 右半 当前 GUI 流水线的 YAML 渲染
  - 修改 YAML 后 GUI 自动同步（带 debounce 300ms）
  - GUI 修改后 YAML 自动重新渲染
- **底部调试栏**（slide-up 抽屉）：
  - 展示每个算子前后 节点数 + diff（新增 / 删除 / 修改的字段红绿高亮）
  - 行内 mono 字体显示节点名、JSON diff
- **状态**：dirty（标题旁显示橙色 ●）/ saving / saved / error（YAML schema 报错红条）

### 7. OperatorCard（流水线中的算子卡片）

- **用途**：PipelineCanvas 中的单个节点
- **元素**：左侧 icon (24px) + 中间标题 + 类型 chip + 启用开关 + 右侧拖拽柄
- **状态**：normal / hover（surface-hover）/ selected（左侧 primary 竖条）/ disabled（透明度 0.4 + 灰背景）/ error（左侧 error 竖条 + 警告 icon）

### 8. YamlDiffViewer（流水线 YAML 双向同步 + 规则预览）

- **用途**：
  - 流水线 YAML 与 GUI 双向同步
  - 规则管理页查看注入前/注入后 Clash 配置 diff
- **元素**：
  - 双栏对比（左旧 / 右新）
  - 行号 + 行级 +/- diff 高亮（success-bg / error-bg）
  - 顶部：复制 / 下载 / 全屏 按钮
  - 字体 font-mono 12px

### 9. ProtocolBadge

- **用途**：节点协议标签
- **变体**（12 种协议 + raw）：
  - vmess / vless / ss / ssr / trojan / hysteria / hysteria2 / tuic / wireguard / anytls / socks5 / naive / raw
- **元素**：3px 圆角 + 4px padding + 11px font-mono 小写
- **配色**：每个协议固定一个 chart-1 ~ chart-7 颜色（不打破单一强调色原则——badge 是"数据"不是"装饰"）

### 10. NotificationChannelCard（10 渠道统一卡片）

- **用途**：通知中心展示每个渠道的配置入口
- **元素**：
  - 左侧 24px 渠道 logo（Telegram / Discord / Slack / Email / Bark / Gotify / Webhook / Server酱 / PushDeer / IFTTT）
  - 中间：渠道名 + 配置摘要（如"Bot: @xxx"）+ 上次发送时间
  - 右侧：开关 + 测试按钮 + ⋯ 编辑/删除
- **状态**：configured / not-configured（按钮变"配置"）/ disabled / error（"上次发送失败"红条）

### 11. TerminalOutput（脚本日志 / TCPing / OTA 实时输出）

- **用途**：实时流式输出（WebSocket / SSE 流）
- **元素**：
  - 黑色背景（暗色：neutral-50 / 亮色：neutral-800）
  - font-mono 12px，行高 1.5
  - ANSI 色彩支持
  - 自动滚动到底部（顶部有"暂停滚动"按钮）
  - 顶部：清屏 / 复制全部 / 下载日志 / 时间戳开关
- **状态**：streaming / paused / finished / error

### 12. CommandPalette（Cmd+K 快捷操作）

- **用途**：技术用户友好——所有页面跳转 / 主要动作的快捷入口
- **触发**：Cmd+K / Ctrl+K
- **元素**：
  - 居中弹层，宽 640px，最大高 480px
  - 顶部搜索框（自动 focus）
  - 下方分组结果：
    - "页面"（dashboard / subscriptions / ...）
    - "动作"（新建订阅 / 触发同步 / 重启 agent / 切换语言 ...）
    - "节点 / 订阅 / agent"（按搜索词匹配）
    - "最近访问"（cookie 持久化）
  - 每项右侧显示快捷键（如 ⌘1 跳 Dashboard）
- **键盘**：↑↓ 选择，Enter 执行，Esc 关闭
- **状态**：loading（搜索动作时顶部 1px progress）/ empty（"无匹配项"）

### 13. ThemeToggle（亮 / 暗 / 跟随系统）

- **用途**：顶栏右侧 icon 按钮
- **元素**：当前主题 icon（sun / moon / monitor）+ 点击展开三选一菜单
- **持久化**：localStorage `theme = light | dark | system`
- **响应**：跟随系统时监听 `prefers-color-scheme` 自动切换

### 14. LangSwitch（4 语言切换）

- **用途**：顶栏右侧 icon + 当前语言代码（"中" / "EN" / "JA" / "KO"）
- **菜单**：4 选项 + 每项右侧母语名（中文 / English / 日本語 / 한국어）
- **持久化**：登录后写入 user 表 `preferred_locale` + cookie

### 15. TwoFactorInput（6 位 OTP 单格独立）

- **用途**：登录 2FA 验证 / 启用 2FA 时验证
- **元素**：6 个独立方框，每格 48x56px，font-mono 24px 居中
- **行为**：自动跳格、支持粘贴拆分、删除回退、Enter 提交
- **状态**：normal / focused（focus 框 primary 边框 2px）/ error（全部红色 + shake 动画 200ms）/ verifying（progress bar）

### 其它复用基础组件（基于 Radix UI + 自定义样式）

- Button（primary / secondary / ghost / destructive 四变体；sm / md / lg 三尺寸）
- Input / Textarea（normal / focus / error / disabled）
- Select / Combobox / MultiSelect
- Checkbox / Switch / RadioGroup
- Tabs / Accordion / Tooltip / Popover / DropdownMenu
- Modal / Drawer / Sheet
- Toast / Banner / Alert
- Skeleton / Spinner / Progress
- Breadcrumb / Pagination

---

## 页面布局

> 五个关键页面 ASCII 草图。比例不严格，意在让前端开发理解"哪里放什么"。

### 1. 总览 Dashboard

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│  拾光VPS    [Cmd+K 搜索...]                       🔔 ☀ 中  👤 admin ▼            │
├────────────┬─────────────────────────────────────────────────────────────────────┤
│            │                                                                     │
│ ◆ 总览     │  欢迎回来，admin                              [+ 新建订阅]          │
│   订阅管理 │                                                                     │
│   节点管理 │  ┌───────────────┬───────────────┬───────────────┬───────────────┐ │
│   规则管理 │  │ 在线 Agent    │ 节点总数      │ 本月流量      │ 待处理告警    │ │
│   脚本管理 │  │     12 / 14   │     287       │   1.4 TB      │     2         │ │
│   探针    │  │ ↑ 2 离线      │ ✓ 全部可达    │ 配额 65% 用尽 │ ⚠ 2 节点离线 │ │
│   流量    │  └───────────────┴───────────────┴───────────────┴───────────────┘ │
│   通知    │                                                                     │
│            │  ┌───────────────────────────────────┬─────────────────────────┐   │
│ ── 管理 ── │  │  流量趋势 (近 30 天)              │  Agent 在线状态        │   │
│   用户管理 │  │  ╭────────────────────────────╮   │  ╭──────────────────╮  │   │
│   系统设置 │  │  │     /\        /\           │   │  │ vps-hk-01  ●在线 │  │   │
│   审计日志 │  │  │    /  \      /  \    /\    │   │  │ vps-jp-02  ●在线 │  │   │
│            │  │  │ __/    \____/    \__/  \__ │   │  │ vps-us-03  ●离线 │  │   │
│            │  │  ╰────────────────────────────╯   │  │ vps-sg-04  ●异常 │  │   │
│            │  │  ─── total  ─── HK  ─── JP        │  │ ... 共 12 个      │  │   │
│            │  └───────────────────────────────────┴─────────────────────────┘   │
│            │                                                                     │
│            │  ┌───────────────────────────────────┬─────────────────────────┐   │
│            │  │  最近事件 (Audit Log)             │  订阅健康概览          │   │
│            │  │  14:32  订阅 "机场A" 同步成功     │  ✓ 5 个订阅正常        │   │
│            │  │  14:15  Agent vps-us-03 离线      │  ⚠ 1 个订阅同步失败    │   │
│            │  │  13:55  user@team 登录            │  ─ 共 6 个订阅          │   │
│            │  │  ...                              │  [查看全部 →]          │   │
│            │  └───────────────────────────────────┴─────────────────────────┘   │
│ [折叠 ◀]   │                                                                     │
└────────────┴─────────────────────────────────────────────────────────────────────┘
```

### 2. 订阅详情

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│  拾光VPS   ...                                                                   │
├────────────┬─────────────────────────────────────────────────────────────────────┤
│ Sidebar    │  ← 订阅管理 / 机场A                                                 │
│            │                                                                     │
│            │  ┌────────────────────────────────────────────────────────────────┐ │
│            │  │  机场A                                          [刷新 ⟳]      │ │
│            │  │  https://airport.example.com/sub/abc...    [复制] [编辑] [⋯]  │ │
│            │  │                                                                │ │
│            │  │  最近更新 5 分钟前 ✓  节点 56  流量 220G / 500G  到期 2026-12 │ │
│            │  │  标签 [HK] [游戏专线]   流水线 ⚙ 已挂载 4 算子 [编辑流水线 →] │ │
│            │  └────────────────────────────────────────────────────────────────┘ │
│            │                                                                     │
│            │  [节点列表 (56)]  [元数据]  [同步历史]  [流水线挂载]                │
│            │  ─────────────────────────────────────────────────────────────────  │
│            │                                                                     │
│            │  [搜索 tag:hk 或节点名...]   [批量 TCPing] [筛选 ▽] [列设置 ⚙]    │
│            │  ┌─┬──────────────┬────┬─────────────┬───────┬─────────┬─────────┐ │
│            │  │☐│名称          │协议│Server:Port  │延迟ms │标签     │动作    │ │
│            │  ├─┼──────────────┼────┼─────────────┼───────┼─────────┼─────────┤ │
│            │  │☐│HK-Premium-01 │vless│hk1.xxx:443 │  42ms │ [HK]    │ ⋯      │ │
│            │  │☐│HK-Game-02    │ss   │hk2.xxx:8388│  38ms │ [HK][游]│ ⋯      │ │
│            │  │☐│JP-Tokyo-01   │vmess│jp1.xxx:443 │ 128ms │ [JP]    │ ⋯      │ │
│            │  │☐│US-Speed-01   │trojan│us1.xx:443 │ 230ms │ [US]    │ ⋯      │ │
│            │  │ │... 共 56 行                                                  │ │
│            │  └─┴──────────────┴────┴─────────────┴───────┴─────────┴─────────┘ │
│            │       1 - 20 of 56     ◀ 1 2 3 ▶     20 / 页 ▽                    │
└────────────┴─────────────────────────────────────────────────────────────────────┘
```

### 3. 算子流水线编辑器 ★（差异化核心）

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│  拾光VPS   ...                                                                   │
├────────────┬─────────────────────────────────────────────────────────────────────┤
│ Sidebar    │  ← 订阅 / 机场A / 流水线                                            │
│            │                                                                     │
│            │  机场A 流水线  ●未保存       [GUI视图][YAML视图]  [导入][导出][保存]│
│            │  ════════════════════════════════════════════════════════════════   │
│            │                                                                     │
│            │  ┌─────────┬──────────────────────────────────┬─────────────────┐  │
│            │  │ 算子库  │  画布                            │ 参数面板        │  │
│            │  │         │                                  │ (filter)        │  │
│            │  │ [拖拽 →]│  ┌──────────────────────────┐    │                 │  │
│            │  │         │  │ ◉ 原始节点列表 (56)      │    │ 名称            │  │
│            │  │ ▢ filter│  └─────────────┬────────────┘    │ [按地域过滤]    │  │
│            │  │ ▣ map   │                ↓ 56 → 38         │                 │  │
│            │  │ ⇅ sort  │  ┌──────────────────────────┐    │ 表达式          │  │
│            │  │ ⊟ dedupe│  │ ▢ filter • 按地域过滤    │■◉│ ┌─────────────┐ │  │
│            │  │ ✎ regex-│  │   过滤 region in [hk,jp] │    │ │ region in   │ │  │
│            │  │   rename│  └─────────────┬────────────┘    │ │ ['hk','jp'] │ │  │
│            │  │ ◈ output│                ↓ 38 → 38         │ └─────────────┘ │  │
│            │  │         │  ┌──────────────────────────┐    │  [表达式手册→]  │  │
│            │  │ [自定义 │  │ ✎ regex-rename           │    │                 │  │
│            │  │  脚本→] │  │   pattern: /^(.+)-(\d+)$/│    │ 启用 ⬤━━━      │  │
│            │  │         │  └─────────────┬────────────┘    │                 │  │
│            │  │         │                ↓ 38 → 38         │ ── ── ── ── ──  │  │
│            │  │         │  ┌──────────────────────────┐    │                 │  │
│            │  │         │  │ ⇅ sort • by latency      │    │ [运行到此 ▶]    │  │
│            │  │         │  │   key: latency, asc      │    │ [删除算子 🗑]   │  │
│            │  │         │  └─────────────┬────────────┘    │                 │  │
│            │  │         │                ↓ 38 → 38         │                 │  │
│            │  │         │  ┌──────────────────────────┐    │                 │  │
│            │  │         │  │ ◈ output • Clash         │    │                 │  │
│            │  │         │  └──────────────────────────┘    │                 │  │
│            │  │         │                                  │                 │  │
│            │  │         │   [+ 拖拽算子或点击添加]         │                 │  │
│            │  └─────────┴──────────────────────────────────┴─────────────────┘  │
│            │                                                                     │
│            │  ▼ 调试预览  执行成功 287ms              [清空 ⌫] [关闭 ▼]         │
│            │  ════════════════════════════════════════════════════════════════   │
│            │  filter (56 → 38)  +0 -18 ~0                                        │
│            │   - US-Speed-01    us1.xxx:443     被移除                          │
│            │   - DE-Frankfurt-2 de.xxx:443      被移除                          │
│            │   ...                                                              │
│            │  regex-rename (38 → 38)  +0 -0 ~38                                  │
│            │   ~ HK-Premium-01  → HK-Premium 01 (- 变为空格)                    │
│            │   ...                                                              │
└────────────┴─────────────────────────────────────────────────────────────────────┘

YAML 视图（点顶部 [YAML视图] tab 切换）：
┌──────────────────────────────────────────────────────────────────────────────────┐
│   YAML 编辑器 (Monaco)             |   GUI 渲染预览（只读 mirror）                │
│                                    |                                              │
│   apiVersion: shiguang/v1          |   ◉ 原始节点列表 (56)                       │
│   kind: Pipeline                   |        ↓                                     │
│   metadata:                        |   ▢ filter • by region                      │
│     name: 机场A 流水线              |        ↓                                     │
│   spec:                            |   ✎ regex-rename                            │
│     operators:                     |        ↓                                     │
│       - type: filter               |   ⇅ sort by latency                         │
│         params:                    |        ↓                                     │
│           expr: "region in [...]"  |   ◈ output                                  │
│       - type: regex-rename         |                                              │
│         ...                        |                                              │
│                                    |   [一键同步到 GUI ▶]                        │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### 4. 通知中心

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│  拾光VPS   ...                                                                   │
├────────────┬─────────────────────────────────────────────────────────────────────┤
│ Sidebar    │  通知中心                                                           │
│            │  ════════════════════════════════════════════════════════════════   │
│            │                                                                     │
│            │  [我的通道] [事件订阅] [系统级通道 (admin)] [模板管理]              │
│            │  ─────────────────────────────────────────────────────────────────  │
│            │                                                                     │
│            │  通知渠道 (10)                                       [+ 添加渠道]   │
│            │                                                                     │
│            │  ┌──────────────────────────────────────────────────────────────┐  │
│            │  │ ✈ Telegram     Bot: @MyBot   chat_id: 12345                  │  │
│            │  │   ✓ 已配置  上次发送 2 分钟前  ⬤━━━  [测试] [Bot 设置 →] ⋯ │  │
│            │  └──────────────────────────────────────────────────────────────┘  │
│            │  ┌──────────────────────────────────────────────────────────────┐  │
│            │  │ # Discord      Webhook: https://disc.../...                  │  │
│            │  │   ✓ 已配置  上次发送 1 小时前  ⬤━━━  [测试] ⋯              │  │
│            │  └──────────────────────────────────────────────────────────────┘  │
│            │  ┌──────────────────────────────────────────────────────────────┐  │
│            │  │ 📧 Email       SMTP: smtp.gmail.com:587                       │  │
│            │  │   ⚠ 上次发送失败  ━━━━  [测试] [查看错误 →] ⋯               │  │
│            │  └──────────────────────────────────────────────────────────────┘  │
│            │  ┌──────────────────────────────────────────────────────────────┐  │
│            │  │ 🔔 Bark        device_key: ...                                │  │
│            │  │ 📲 Gotify      未配置                                [配置 →] │  │
│            │  │ 🔗 Webhook     未配置                                [配置 →] │  │
│            │  │ ✉ Server酱    未配置                                [配置 →] │  │
│            │  │ 📌 PushDeer    未配置                                [配置 →] │  │
│            │  │ ⚡ IFTTT       未配置                                [配置 →] │  │
│            │  │ 💬 Slack       未配置                                [配置 →] │  │
│            │  └──────────────────────────────────────────────────────────────┘  │
│            │                                                                     │
│            │  ── 事件订阅矩阵 (tab 切换后)：                                     │
│            │  ┌──────────────────┬─────┬─────┬─────┬──────┬─────┬─────┬─────┐ │
│            │  │ 事件 \ 渠道     │ TG  │ DC  │ 邮件│ Bark │ ...                │ │
│            │  ├──────────────────┼─────┼─────┼─────┼──────┼─────┼─────┼─────┤ │
│            │  │ 节点离线        │ ✓   │ ✓   │ ☐   │ ✓    │ ...                │ │
│            │  │ 流量告警 (>80%) │ ✓   │ ☐   │ ✓   │ ✓    │                    │ │
│            │  │ 订阅同步失败    │ ✓   │ ✓   │ ✓   │ ☐    │                    │ │
│            │  │ 备份完成        │ ☐   │ ☐   │ ☐   │ ☐    │                    │ │
│            │  │ 登录异常        │ ✓   │ ☐   │ ✓   │ ☐    │                    │ │
│            │  │ OTA 升级        │ ✓   │ ☐   │ ☐   │ ☐    │                    │ │
│            │  │ 脚本告警        │ ☐   │ ☐   │ ☐   │ ☐    │                    │ │
│            │  └──────────────────┴─────┴─────┴─────┴──────┴─────┴─────┴─────┘ │
└────────────┴─────────────────────────────────────────────────────────────────────┘
```

### 5. 节点管理列表

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│  拾光VPS   ...                                                                   │
├────────────┬─────────────────────────────────────────────────────────────────────┤
│ Sidebar    │  节点管理                                          总计 287 个节点  │
│            │  ════════════════════════════════════════════════════════════════   │
│            │                                                                     │
│            │  [搜索 tag:hk vless ...]   [全部协议 ▽] [全部订阅 ▽] [全部标签 ▽]  │
│            │                                                                     │
│            │  [批量 TCPing] [批量打标签] [批量删除]      [列设置 ⚙] [导出 ⤓]   │
│            │  ─────────────────────────────────────────────────────────────────  │
│            │                                                                     │
│            │  ┌─┬─────────────────┬──────┬───────────────┬───────┬────────────┐ │
│            │  │☐│名称             │协议  │Server:Port    │延迟 ms│标签       │ │
│            │  ├─┼─────────────────┼──────┼───────────────┼───────┼────────────┤ │
│            │  │☐│HK-Premium-01    │vless │hk1.xxx:443    │  42   │[HK][游戏] │ │
│            │  │☐│HK-Game-02       │ ss   │hk2.xxx:8388   │  38   │[HK][游戏] │ │
│            │  │☐│JP-Tokyo-01      │vmess │jp1.xxx:443    │ 128   │[JP]       │ │
│            │  │☐│JP-Tokyo-Game    │vmess │jp2.xxx:443    │ 145   │[JP][游戏] │ │
│            │  │☐│US-Speed-01      │trojan│us1.xxx:443    │ 230   │[US]       │ │
│            │  │☐│SG-Premium-01    │vless │sg1.xxx:443    │  85   │[SG]       │ │
│            │  │ │... 共 287 行                                                  │ │
│            │  └─┴─────────────────┴──────┴───────────────┴───────┴────────────┘ │
│            │       1 - 50 of 287     ◀ 1 2 3 ... 6 ▶     50 / 页 ▽            │
│            │                                                                     │
│            │  ┌─────────────────────────────────────────────────────────────┐  │
│            │  │ ★ 点击行打开右侧详情抽屉                                    │  │
│            │  └─────────────────────────────────────────────────────────────┘  │
│            │                                                                     │
│            │  ⇒ 点击 HK-Premium-01 行 → 右侧 480px Drawer 滑入：                 │
│            │     节点详情 / Server / Port / UUID / Cipher / 加密 / SNI / WS...  │
│            │     [复制 Raw URI]  [测试连接]  [编辑]  [删除]                     │
└────────────┴─────────────────────────────────────────────────────────────────────┘
```

---

## 交互流程

### 流程 1：首次部署 → 拿密码 → 登录 → 启用 2FA → 创建第一个订阅

```
[部署主机]
   ↓ docker run / 一键脚本
[日志输出]
   "Admin password: Aj7$kK9pXm2Q"
   "Login URL: http://host:8080/_app/a3f8b9c2.../"
   ↓ 用户复制 URL
[浏览器访问 URL]
   ↓
[登录页 /_app/.../login]
   - 用户名 admin
   - 密码 Aj7$kK9pXm2Q
   - 提交
   ↓ 登录成功（未开 2FA）
[Banner 横幅提示]
   "你的账户未启用 2FA，强烈建议立即启用 [立即启用 →]"
   ↓ 点击
[/profile/2fa 启用流程]
   1. 显示 QR Code（otpauth://...）+ 文字 secret 备用
   2. 在 Authenticator 扫码
   3. 输入 6 位 TOTP 验证
   4. 显示 8 个备份码 + 强制勾选 "我已保存"
   5. 完成
   ↓
[Toast "2FA 已启用"]  + 跳回 /dashboard
   ↓
[空 Dashboard 引导]
   - 中央 illustration + "看起来你还没有订阅"
   - [+ 创建第一个订阅] CTA 主按钮
   ↓ 点击
[订阅创建向导 Modal] (3 步)
   Step 1: 选择来源 ─ ◉ URL  ○ YAML 文件  ○ 手动添加
   Step 2: 填写 URL + 自定义 User-Agent (可选)
   Step 3: 选择标签 + 更新周期
   ↓ 提交
[Loading "正在拉取节点..." + 进度条]
   ↓ 成功
[跳转 /subscriptions/:id 订阅详情]
   - 节点列表已渲染
   - 顶部 Toast "成功导入 56 个节点"
```

### 流程 2：导入订阅 → 系统拉取 → 解析失败/成功 → 触发通知

```
[订阅自动周期触发 (6h)] 或 [用户点击刷新]
   ↓
[hub 后台拉取 URL]
   ┌── HTTP 200 ──→ [YAML/订阅解析]
   │                   ┌── 全部成功 ──→ [更新节点表]
   │                   │                   ↓
   │                   │                [运行已挂载流水线]
   │                   │                   ↓
   │                   │                [写入最终节点]
   │                   │                   ↓
   │                   │                [前端 SSE 推送 → UI 实时刷新订阅状态]
   │                   │                   ↓
   │                   │                [✓ 无通知触发]
   │                   │
   │                   └── 部分失败 ──→ [入库可解析的部分]
   │                                      ↓
   │                                   [warning 计入 sync_history]
   │                                      ↓
   │                                   [UI 顶部 banner 提示 + 详情链接]
   │
   └── HTTP 5xx / 超时 / 解析全失败 ──→
       [写入 sync_history 失败]
          ↓
       [触发 NotificationEvent "订阅同步失败"]
          ↓
       [按用户 opt-in 矩阵分发到 Telegram / Email / ...]
          ↓
       [每个渠道 NotificationChannel.Send()]
          ↓
       [Telegram 收到："⚠ 订阅 '机场A' 同步失败 (HTTP 503)"
        + inline keyboard [立即重试] [查看详情]]
          ↓ 用户在手机点 [立即重试]
       [Bot 推送回 hub → 立即重新触发同步]
          ↓
       [Bot 回复 "已触发重试..."]
```

### 流程 3：拖拽编辑流水线 → 实时预览 diff → 导出 YAML → 重导入（差异化核心）

```
[用户进入 /subscriptions/:id/pipeline]
   ↓ 初次进入（流水线为空）
[画布展示：◉ 原始节点列表 (56) + 空白引导 "拖入算子或从模板选择"]
   ↓ 从左侧拖入 filter 算子
[算子卡片落入画布，自动连线 + 弹出右侧参数面板]
   ↓ 用户填写 expr: "region in ['hk','jp']"
[失焦自动保存到 client state，标题旁出现 ●未保存]
   ↓ 点击参数面板 [运行到此 ▶]
[底部调试栏滑出]
   - filter (56 → 38)  +0 -18 ~0
   - 列出被移除的 18 个节点（红色 strikethrough）
   ↓ 拖入 sort 算子 → 设 key=latency, asc
[新算子卡片落入下方，调试栏自动追加该算子结果]
   ↓ 拖入 output → Clash 格式
[画布完整：input → filter → sort → output]
   ↓ 用户切换顶部 tab [YAML视图]
[左侧 Monaco 编辑器渲染当前 YAML，右侧 GUI mirror 只读]
   ↓ 用户手动改 YAML（如改 sort 的 key 为 name）
[debounce 300ms 后 GUI mirror 同步更新]
   ↓ 用户点击 [保存]
[POST /api/pipelines/:id → 200]
   - 标题 ● 消失
   - Toast "流水线已保存"
   - 自动跑一次完整 pipeline → 更新订阅最终节点
   ↓ 用户点击 [导出 YAML ⤓]
[浏览器下载 pipeline-机场A-20260520.yaml]
   ↓ 用户 commit 到 git
   ↓ 用户在另一台部署的拾光VPS 上 [导入 YAML ⤴]
[文件选择 → 上传 → 后端 schema 校验]
   ┌── 通过 ──→ [GUI 渲染完整流水线 + Toast "成功导入"]
   └── 失败 ──→ [Banner "schema 校验失败：line 12 unknown operator type 'group-by'"]
```

### 流程 4：agent 部署 → 上报心跳 → 流量聚合 → 月度计费周期重置

```
[admin 在 /agents 页面点击 + 添加 Agent]
   ↓
[Modal: 选择系统架构 + 命名 + 标签]
   ↓ 提交
[生成 Token 与一键安装命令]
   "curl -fsSL https://hub/install.sh | TOKEN=xxx bash"
   ↓ 用户复制到 VPS 执行
[Agent 二进制下载 + 注册 systemd 服务]
   ↓
[Agent WS 连接 wss://hub/_app/.../ws/agent + Token]
   ↓ 鉴权通过
[hub 推送配置：心跳频率 30s]
   ↓
[Agent 每 30s 上报 agent_record]
   - cpu / mem / disk / netio / load / uptime
   ↓
[UI /agents 列表行实时刷新（SSE）]
   - 状态点变绿
   - 最近心跳时间倒计时
   ↓ 用户进入 /agents/:id
[详情页：CPU/MEM/NetIO 实时折线图（最近 1h，2s 一刷）]
   ↓
[每日 00:00 hub 跑日聚合任务]
   - agent_records (前一天) → traffic_records (一行/agent)
   - agent_records 超 7 天的自动清理
   ↓
[/traffic 页：折线图按"日"维度展示]
   ↓ 用户切到"月"
[显示本月累计 + 当前进度条 + 配额 500G]
   - "本月已用 220G / 500G  44%"
   - 进度条接近 80% 时变橙色
   ↓ 累计到达 80% (400G)
[触发 NotificationEvent "流量告警 80%"]
   - 5 分钟去抖窗口内只发一次
   ↓
[Telegram / Email / Bark 收到 "⚠ 本月流量已用 80%"]
   ↓
[计费周期日（默认每月 1 号 00:00）]
   - cron 触发重置
   - "本月已用" → 0
   - "上月已用" → 220G
   - 不影响 traffic_records 历史
```

### 流程 5：配置 Telegram Bot → 添加 inline keyboard 命令 → 在手机上发 /nodes 触发响应

```
[/notifications/telegram 入口]
   ↓
[配置 Telegram Bot 表单]
   - Bot Token (从 @BotFather 获取)
   - Chat ID (个人 / 群组)
   - [测试发送 →]
   ↓ 测试通过
[启用 inline keyboard 命令开关 ⬤━━━]
   - 提供 5 个内置命令复选框：
     ☑ /nodes        查询节点列表 (按延迟排序前 10)
     ☑ /refresh      手动触发订阅同步
     ☑ /agent_restart 重启指定 agent
     ☑ /traffic      查看本月流量
     ☑ /silent       开/关静默模式
   - [保存]
   ↓
[hub 注册 webhook 到 https://api.telegram.org/.../setWebhook]
   ↓
[用户手机 Telegram 与 Bot 私聊]
   - 用户发送 /nodes
   ↓
[Telegram 回调到 hub /api/telegram/webhook]
   - 验证来源（Bot Token）+ 验证 chat_id 在白名单
   ↓ 通过
[hub 执行命令 → 查询数据库 → 渲染消息模板]
   ↓
[Bot 回复消息：
   "📋 节点列表 (TOP 10 by latency)
    1. HK-Premium-01    42ms  ✓
    2. HK-Game-02       38ms  ✓
    3. JP-Tokyo-01     128ms  ✓
    ...
   [刷新 ↻] [查看全部 →] [测速所有 ⚡]"]
   ↓ 用户点 [测速所有 ⚡]
[Telegram 回调 → hub 触发批量 TCPing 任务（异步）]
   ↓ Bot 立即回复 "测速中... 预计 5s"
   ↓ 5s 后 Bot 编辑原消息为最新结果
```

---

## 响应式策略

### 断点

```yaml
breakpoint-sm:  "640px"     # 大手机
breakpoint-md:  "768px"     # 平板竖屏
breakpoint-lg:  "1024px"    # 平板横屏 / 小笔记本
breakpoint-xl:  "1280px"    # 桌面默认
breakpoint-2xl: "1536px"    # 大桌面
```

### 桌面（≥ 1280px）

- 完整三栏：240 Sidebar + 主区 + 480 右侧 Drawer（按需弹出）
- 表格全字段展示
- 流水线编辑器三栏可用
- 命令面板 Cmd+K 全功能

### 笔记本 / 平板横屏（1024-1279px）

- Sidebar 默认折叠到 64px icon-only
- 主区域 padding 减少到 16px
- 右侧 Drawer 改为覆盖式（非推开式）
- 表格部分次要列自动隐藏（用列设置可恢复）

### 平板竖屏（768-1023px）

- Sidebar 默认关闭，顶部出现 hamburger 按钮
- 表格保留核心 3-4 列，长行可展开查看更多
- 流水线编辑器进入"只读 + GUI tab"模式，**不支持 YAML 编辑器**（屏幕不够）

### 手机（< 768px）

- 顶栏简化：左 hamburger + 中 logo + 右头像
- **底部 TabBar 替代 Sidebar**：5 个最常用入口（总览 / 订阅 / 节点 / 通知 / 我的）
- 其它入口通过"我的"页面展开二级菜单
- **大表格变卡片列表**：
  - 每行 = 一张卡片
  - 卡片：第一行节点名 + StatusBadge；第二行 server:port + 协议 + 延迟；右下 ⋯ 菜单
- **流水线编辑器进入只读模式**：只允许查看流程图与调试预览，编辑动作灰禁并提示"请在桌面端编辑"
- 命令面板 Cmd+K 不显示（无键盘）；改用顶部搜索按钮触发
- 表单全宽（无左右栏布局）
- Modal 改为全屏 Sheet（从底部滑入）

### 暗色 / 亮色切换

- 跟随系统 `prefers-color-scheme`，用户可在顶栏 ThemeToggle 强制切换
- 切换无动画过渡（避免 flash），瞬时切换
- 全部 token 使用 CSS custom property 由根节点切换 `[data-theme="dark"]` / `[data-theme="light"]`

---

## 附：可访问性要点

- 所有交互元素 focus 状态有 2px primary outline（offset 2px）
- 颜色对比度满足 WCAG AA（小字 4.5:1 / 大字 3:1）
- 所有 icon-only 按钮必须有 aria-label
- TwoFactorInput 支持屏幕阅读器逐位朗读
- 表格列头支持 aria-sort
- 通知 Toast 使用 role="status" + aria-live="polite"
- 关闭键盘陷阱：所有 Modal/Drawer Esc 关闭、tab 循环聚焦
- 流水线编辑器虽以鼠标为主，仍提供 tab 焦点遍历 + Enter 进入算子参数面板

## 附：i18n 文案落位约定

- 所有标题、按钮、表格列头、tooltip、空状态、错误信息走 `t('namespace:key')`
- 时间渲染统一用 `Intl.DateTimeFormat(locale)`，禁止 dayjs 固定格式串
- 数字渲染统一用 `Intl.NumberFormat(locale)`，自动按 locale 处理千分位
- 字体兜底链确保 4 语言无方框字符
- 文案长度估算（同一 key 在 4 语言下的字符数）由 UI 自适应：按钮 min-width 不固定、表格列宽 auto

---

文档结束。下一步：交付给 Tech Lead / 架构师对齐，并由前端开发实现 Design Tokens 与组件库。

## 开发提示

> 本小节由 design-tokens-bootstrap 自动追加，供后续开发 Agent 参考。

### 风格关键词（开发时必须遵守的视觉语言）

1. `generous whitespace` —— 大量留白，间距优先于装饰
2. `functional typography` —— 字体即层级，禁止用色彩区分层级
3. `1px hairline border` —— 用 1px 细线分隔区域，禁止粗边框
4. `monochrome with single accent` —— 90% 中性灰 + 一个强调色（青蓝）
5. `grid-based layout` —— 8 列 / 12 列网格对齐，禁止飘字符位
6. `subtle elevation` —— 阴影只用于 popover / modal，不用于卡片
7. `numeric-first` —— 数字（流量 / 延迟 / 时间戳）用 tabular-nums 等宽对齐
8. `tonal hierarchy` —— 同色相不同明度表达层级（bg / surface / elevated）
9. `data-ink ratio first` —— 图表去除多余轴线刻度，墨水都给数据
10. `keyboard-first` —— 所有核心动作都有快捷键，鼠标是辅助

### Anti-patterns（绝对禁止）

1. **禁止 gradient / 渐变装饰**：仅图表数据填充允许 alpha 渐变，按钮 / 卡片 / banner 不允许任何渐变。
2. **禁止彩色硬阴影 + 阴影 ≥ 4 层**：暗色下黑阴影叠黑无效；阴影只用 `rgba(0,0,0,0.3-0.6)` 软阴影，且不超过 3 个层级。
3. **禁止圆角 < 4px 或 > 16px**：4-12 是统一区间；表单 / 按钮 / 卡片 / 弹窗在此区间内分配。
4. **禁止使用 ≥ 3 种主色**：全站只有一个强调色（青蓝 #00B8D9 系列）+ 4 个语义色（success/warning/error/info），不允许引入紫 / 粉 / 橙作为装饰。
5. **禁止 glassmorphism / neumorphism / 拟物**：任何模糊背景、浮雕、3D 投影、纹理填充都不允许。表格背景必须 solid color。
