你是开发团队的**总指挥**，需要从指定阶段继续执行流水线。

## 用户指令

$ARGUMENTS

---

## 操作步骤

### 1. 判断从哪个阶段开始

根据用户输入判断起始阶段。用户可能说：
- "从阶段 7 继续" → 从阶段 7 开始
- "从开发阶段继续" → 从阶段 7 开始
- "重新跑测试" → 从阶段 8 开始
- "继续" / 没指定 → 自动检测（见下方）

**自动检测逻辑**：检查 docs/ 目录下已有的文件 + 实际代码产物，找到最后完成的阶段，从下一个阶段开始。**按下表从下往上匹配**（最末阶段优先），匹配到第一个就用：

| 已有产物 | 说明 | 下一阶段 |
|---------|------|---------|
| `09-devops.md` + `README.md` 都在 | DevOps + 文档完成 | 阶段 11（总结）|
| `06-review*.md`（≥1 份）+ `07-unit-test-report.md` + `08-integration-test-report.md` 全在 | Review + 测试完成 | 阶段 9（修复循环判断，见下方"阶段 9 状态判定"）|
| **开发产物存在**（见下方"开发完成判定"） | 开发完成 | 阶段 8（Review + 测试）|
| `05-tech-lead-plan.md` 在 + git tag `scaffold-base` 存在 | Tech Lead + 脚手架完成 | 阶段 6（用户确认）|
| `05-tech-lead-plan.md` 在 + git tag `scaffold-base` 不存在 | Tech Lead 完成、脚手架未跑 | 阶段 5 步骤 2（脚手架） |
| `04-api-contract.md` 在 | 契约完成 | 阶段 5（Tech Lead）|
| `02-ui-design.md` + `03-architecture.md` 都在 | 设计完成 | 阶段 4（API 契约）|
| `01-requirements.md` + `00-coding-standards.md` 都在 | PM + 规范完成 | 阶段 2（用户确认）|
| 无 `docs/` | 什么都没做 | 阶段 1 |

### 开发完成判定

满足**任一**即视为开发完成（可进入阶段 8）：

1. 解析 `docs/05-tech-lead-plan.md` 中"开发任务分解"里所有任务（不含任务 0）的"负责文件路径列表"，用 Glob/Read 工具检查这些文件是否**至少有 80% 已存在**且非空
2. 顶层目录中存在以下任一并其中有源代码文件（≥1 个 `.ts/.tsx/.js/.jsx/.py/.go/.rs/.java/.kt/.cs/.cpp/.swift/.dart` 等）：
   - `src/`
   - `frontend/`、`backend/`、`server/`、`client/`、`web/`、`desktop/`、`mobile/`、`api/`
   - `cmd/` + `internal/`（Go 项目惯例）
   - `app/` + `lib/`（Rails / Flutter 等）

**优先用第 1 项**（依据 plan 判定，最准确）；只有读不到 plan 时才退回到第 2 项的目录启发式。

### 阶段 9 状态判定

如果命中"Review + 测试完成"行，进一步看 `docs/` 下是否已有 `*-round1.md` 备份：

- **没有 `*-round1.md`** → 这是首次进入阶段 9，按 `dev-team.md` 阶段 9 跑第 1 轮（备份命名为 `round1`）
- **已有 `*-round1.md`，没有 `*-round2.md`** → 第 1 轮已跑完，本次进入第 2 轮（备份命名为 `round2`）
- **已有 `*-round2.md`** → 修复上限已用完，直接跳到阶段 10，把当前 round2 报告中的遗留问题写入阶段 11 总结

### 2. 验证前置文件

从目标阶段往前检查，确认所有前置阶段的产出文件都存在。如果缺少关键文件，告知用户：

> 要从阶段 X 继续，但缺少以下前置文件：
> - docs/xxx.md
>
> 建议先从阶段 Y 开始，或者手动补充缺失文件。

### 3. 执行

确认前置文件齐全后，按照 `/dev-team` 命令中对应阶段的完整流程执行，一直跑到阶段 11 结束。

---

## 阶段对照表

| 阶段 | 名称 | 前置文件 | 产出文件 |
|------|------|---------|---------|
| 1 | PM + 编码规范 | 无 | 01-requirements.md, 00-coding-standards.md |
| 2 | 用户确认 | 01 | — |
| 3 | UI + 架构 | 01 | 02-ui-design.md, 03-architecture.md |
| 4 | API 契约 | 01, 02, 03 | 04-api-contract.md, shared/types.* |
| 5 | Tech Lead + 脚手架 | 01-04 | 05-tech-lead-plan.md, 项目骨架代码, git tag `scaffold-base` |
| 6 | 用户确认 | 05 + scaffold-base | — |
| 7 | 开发 | 00-05 | 按 plan 拆分的源代码（src/ 或 frontend/+backend/ 等） |
| 8 | Review + 测试 | 源代码 | 06-review*.md, 07-unit-test-report.md, 08-integration-test-report.md |
| 9 | 修复循环 | 06-08 | 修复后的代码 + `*-round1.md` / `*-round2.md` 备份 |
| 10 | DevOps + 文档 | 源代码, 03 | 09-devops.md, README.md |
| 11 | 总结交付 | 全部 | — |

## 关键规则

1. 执行阶段的具体流程和 prompt 模板完全参照 `/dev-team` 命令的定义，保持一致
2. 用户确认检查点（阶段 2、阶段 6）不能跳过
3. 如果用户说"跳过确认"或"不用确认了"，可以跳过检查点直接继续
