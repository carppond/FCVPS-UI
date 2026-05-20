# ADR 0004: 订阅算子流水线设计（差异化 #1）

## Context

- 调研报告（docs/_research-competitors.md 第四节"差异化机会 2"）明确："订阅算子流水线 + YAML 双模式"是高价值差异化。
- sub-store 已有"Script Operator"算子模型，但其 UI 是堆叠表单，体验不直观；同时仅支持代码编辑，**对 GitOps 用户不友好**。
- Gatus 走纯 YAML 路线，无 GUI，普通用户上手陡。
- 妙妙屋目前只有 goja 脚本钩子，是"一次性脚本"，可扩展性低、用户学习成本高。
- 拾光VPS 的目标用户横跨"个人爱好者（更喜欢 GUI）"和"小机场主 / 小团队管理员（需要 git 化管理）"，需双模式覆盖。

## Decision

把订阅处理设计为**可视化算子流水线**（差异化 #1），架构要点：

1. **算子库（v1）**：filter / map / sort / dedupe / regex-rename / output 共 6 类，每类一组结构化参数。
2. **数据契约**：v1 所有算子输入输出都是 `[]Node`（CONTEXT.md Flagged #3 已锁定）。
3. **双模式编辑**：
   - GUI 模式：基于 @dnd-kit 实现拖拽编排 + 实时预览（左边算子库 / 中间画布 / 右边参数面板）。
   - YAML 模式：声明式描述同一流水线，schema `apiVersion: shiguang/v1`，平铺数组 + `type` 字段。两个模式实时双向同步（YAML 改 → 画布跟着变 / 反之亦然）。
4. **存储**：流水线作为订阅的关联实体（subscription_pipelines 表），运行时由 hub 加载 + 编排执行。
5. **执行性能**：100 节点全流水线执行 < 500ms（性能验收线，对应 PRD 6.1）。
6. **调试**：UI 提供"运行预览"按钮，显示每个算子前后的节点列表 diff，方便排错。

## Consequences

**正面**：
- 同时满足"普通用户 GUI 编排"与"进阶用户 git 化管理"两类受众，是 sub-store / Gatus 都未覆盖的市场区间。
- 算子库内置且类型受限（v1 仅 6 种），可类型校验、可单元测试，避免 goja 脚本"出了 bug 难定位"的痛点。
- YAML 导入导出意味着可以做"流水线模板市场"（v1.1+ 路线）。

**负面 / 待办**：
- GUI ↔ YAML 双向编辑器是中-高复杂度工程，前端工程量较大；预算 1 个 Sprint（W7）。
- 算子库扩展性需要预留：v2 可能引入 group/branch/conditional 等控制流算子，需要在抽象时就考虑（用 Operator 接口而非硬 enum）。
- goja 脚本扩展（妙妙屋遗留）继续保留，但 **优先推算子**；脚本作为"算子覆盖不了的场景"的逃生舱。

**替代方案为何放弃**：
- **只做 GUI（妙妙屋路线）**：放弃 GitOps 出口，丢掉小机场主用户群。
- **只做 YAML（Gatus 路线）**：放弃普通用户群，社区扩张困难。
- **直接复用 sub-store 算子模型**：JS 实现迁移成本高，且无 GUI；不如自研。

## Related

- ADR 0001: 技术栈（前端 @dnd-kit / yaml.v3）
- 调研：docs/_research-competitors.md 第四节"差异化机会 2" + 第二节"订阅算子流水线"行
- CONTEXT.md Flagged #2、#3
