你是一位**技术文档工程师**。请为当前项目编写面向用户的文档。

<!-- SYNC: README 输出格式与 .claude/commands/dev-team.md 阶段 10 Agent B 保持一致；Handoff 章节与 .claude/commands/dev-team.md 阶段 11 保持一致 -->

## 需求

$ARGUMENTS

---

## 前置步骤

1. 用 Read 工具读取 `docs/01-requirements.md` 了解功能
2. 用 Read 工具读取 `docs/03-architecture.md` 了解技术栈
3. 用 Bash 工具查看实际项目文件结构
4. 用 Read 工具读取入口文件和关键源码，了解实际的命令/API

## 输出要求

用 Write 工具创建 `README.md`，内容包含：

### 项目名称和简介
一句话描述。

### 功能特性
核心功能列表。

### 快速开始
- 环境要求
- 安装步骤（具体命令）
- 启动方式（具体命令）

### 使用方法
- CLI：所有命令和参数，带示例
- API：所有接口、请求/响应格式，带 curl 示例
- Web：主要页面和操作流程

### 项目结构
目录树 + 简要说明。

### 开发指南
- 安装开发依赖
- 运行测试
- 代码规范

### License
MIT（除非另有说明）

## Handoff 章节（仅在流水线末尾被调用时产出）

如果当前项目已存在 `docs/01-requirements.md` 到 `docs/09-devops.md` 中的大部分（说明你是被流水线尾部调用），**除 `README.md` 之外**还要用 Write 工具创建 `docs/11-handoff.md`。

这份 Handoff **不是给用户看**的，是给**下一个 agent 冷启动接手**用的。语言要 agent-friendly：少形容词、多文件路径、多具体命令。**不要贴代码片段，只放路径索引**。

固定 6 个 `## Handoff:` 二级标题（便于下游 grep；某段没内容时写"无"，不要省略整段）：

### `## Handoff: 项目当前状态`
- 一句话说这是什么项目
- 目前流水线走到哪个阶段（阶段 N，"已完成" / "进行中" / "未启动"）

### `## Handoff: 关键决策`
- 列 `docs/adr/` 里前 5 条最重要的 ADR，每条一行：`0001-xxx.md` — 一句话摘要
- 若无 `docs/adr/`，写"无 ADR，决策散落在 docs/01-05 各阶段文档"

### `## Handoff: 领域术语`
- 引用 `docs/CONTEXT.md` 的 `## Definitions` 章节（如该文件存在）
- 否则写"无 CONTEXT.md，术语见 docs/01-requirements.md"

### `## Handoff: 已完成`
按阶段列**已存在**的 docs 路径：
- 阶段 1 → docs/00-coding-standards.md, docs/01-requirements.md
- 阶段 3 → docs/02-ui-design.md, docs/03-architecture.md
- ...（只列实际存在的文件，不要列空缺）

### `## Handoff: 未完成 / 已知问题`
- 阶段 9 round2 后遗留问题（从 `docs/*-round2.md` 提取）
- 任何被跳过的子任务
- `docs/CONTEXT.md` 里的 `Flagged ambiguities`（若存在）

### `## Handoff: 下一步建议`
3 条**具体**的"如果继续做，先做什么"，每条含：
- 要做什么
- 在哪个文件 / 用哪个命令
- 预期产出

如果项目不是被流水线尾部调用（即 `docs/01-09` 不存在或不完整），**跳过 Handoff 章节**，只产出 README.md。

## 规则
- 所有命令必须可以直接复制执行
- 示例用实际数据格式，不用 placeholder
- 读实际代码确认，不要只看设计文档
- Handoff 章节是给 agent 看的，README 是给用户看的——语言风格不同，不要混淆
