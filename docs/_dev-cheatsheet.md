# 开发速查卡（自动生成，勿手动修改）

来源：docs/02-ui-design.md（Minimalism / Swiss，暗色优先）
基准 commit：691ab62

## Token 文件位置
- Tailwind 配置：web/tailwind.config.ts
- CSS 变量声明：web/src/styles/globals.css（@theme block）
- 字体引入：web/index.html（Google Fonts link）
- i18n 入口：web/src/lib/i18n.ts

## 可用颜色 class

### 主色
- bg-primary / text-primary / border-primary
- bg-primary-hover / bg-primary-active
- text-primary-foreground

### 中性色（neutral 阶梯）
- bg-neutral-0 / bg-neutral-50 / bg-neutral-100 / bg-neutral-200 / bg-neutral-300
- bg-neutral-400 / bg-neutral-500 / bg-neutral-600 / bg-neutral-700 / bg-neutral-800
- bg-neutral-900 / bg-neutral-950
- text-neutral-* 同理

### 语义角色色
- bg-bg（页面背景）/ bg-bg-elevated（主区背景）/ bg-surface（卡片背景）/ bg-surface-hover
- text-text-primary / text-text-secondary / text-text-tertiary / text-text-disabled
- border-border / border-border-strong

### 状态色（节点 / agent）
- text-status-online（在线）/ text-status-offline（离线）/ text-status-unknown（未知）/ text-status-degraded（降级）
- bg-success / bg-warning / bg-error / bg-info
- text-success / text-warning / text-error / text-info
- bg-success-bg / bg-warning-bg / bg-error-bg / bg-info-bg

### 数据可视化（多系列图表用，不能用主色调色板做图表区分）
- chart-1: #00B8D9（青，第一条线）
- chart-2: #7C5CFF（紫）
- chart-3: #22C55E（绿）
- chart-4: #F59E0B（橙）
- chart-5: #EC4899（玫粉）
- chart-6: #06B6D4（浅青）
- chart-7: #A3A3A3（灰，兜底 / "其它"分类）
- 区域填充用同色 alpha=0.15

## 可用间距
p-0 / p-0.5(2px) / p-1(4px) / p-1.5(6px) / p-2(8px) / p-3(12px) / p-4(16px) / p-6(24px) / p-8(32px) / p-12(48px) / p-16(64px) / p-20(80px) / p-24(96px)
（m-*、gap-*、space-*、w-*、h-* 同阶梯）

## 可用字号
text-xs(11px) / text-sm(12px) / text-base(14px) / text-lg(16px) / text-xl(18px) / text-2xl(24px) / text-3xl(32px) / text-4xl(48px)

## 可用行高
leading-tight(1.2) / leading-normal(1.5) / leading-relaxed(1.7)

## 可用圆角
rounded-none / rounded-sm(4px) / rounded-md(6px) / rounded-lg(8px) / rounded-xl(12px) / rounded-2xl(16px) / rounded-full(9999px)

## 可用阴影
shadow-none / shadow-sm / shadow-md / shadow-lg / shadow-xl
（暗色场景下 shadow 自动调整为软阴影，不要在暗色背景上叠黑色硬阴影）

## 可用动效
duration-fast(150ms) / duration-normal(250ms) / duration-slow(350ms) / duration-extra-slow(500ms)
缓动：ease-out（进场）/ ease-in-out（双向）

## 字体
- 标题：font-display（Inter, Noto Sans SC, Noto Sans JP, Noto Sans KR, system-ui）
- 正文：font-body（Inter, Noto Sans SC, Noto Sans JP, Noto Sans KR, system-ui）
- 代码 / 日志：font-mono（JetBrains Mono, Fira Code, Cascadia Code, SF Mono, Consolas）
- CJK 兜底自动生效，无需额外 class；数字列加 .tabular-nums 开启等宽

## 风格关键词
1. `generous whitespace` —— 大量留白，间距优先于装饰
2. `functional typography` —— 字体即层级，禁止用色彩区分层级
3. `1px hairline border` —— 用 1px 细线分隔区域，禁止粗边框
4. `monochrome with single accent` —— 90% 中性灰 + 一个强调色（青蓝）
5. `grid-based layout` —— 8 列 / 12 列网格对齐，禁止飘字符位
6. `subtle elevation` —— 阴影只用于 popover / modal，不用于卡片
7. `numeric-first` —— 数字用 tabular-nums 等宽对齐
8. `tonal hierarchy` —— 同色相不同明度表达层级（bg / surface / elevated）
9. `data-ink ratio first` —— 图表去除多余轴线刻度，墨水都给数据
10. `keyboard-first` —— 所有核心动作都有快捷键，鼠标是辅助

## Anti-patterns（绝对禁止）
1. 禁止 gradient / 渐变装饰：按钮 / 卡片 / banner 不允许任何渐变
2. 禁止彩色硬阴影 + 阴影 ≥ 4 层：只用 rgba 软阴影，不超过 3 层级
3. 禁止圆角 < 4px 或 > 16px：统一区间 4-16px
4. 禁止使用 ≥ 3 种主色：只有一个强调色（青蓝）+ 4 个语义色
5. 禁止 glassmorphism / neumorphism / 拟物：表格背景必须 solid color

## 编码硬约束
- 禁止裸 hex / rgb / hsl 字面量
- 禁止"任意像素"（如 13px / 17px），所有数值必须用 token 阶梯
- 禁止硬编码用户可见文案，必须走 i18n `t('namespace.key')`
- 所有数据驱动组件必须有四态：正常 / 加载（Skeleton）/ 空（EmptyState）/ 错误（ErrorState）
- 暗色优先，亮色作为辅助主题
