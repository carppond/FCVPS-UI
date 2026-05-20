# ADR 0007: i18n 策略 —— react-i18next + 4 语言文案隔离

## Context

- 拾光VPS 的潜在用户群除中文圈外，还包括日韩 VPS 圈（日本节点 / 韩国节点用户密集）以及英文圈技术爱好者（GitHub 国际化开源生态）。
- 妙妙屋当前是单语言（中文）硬编码，国际化扩展时改造成本极高。
- 调研中的同类项目：Nezha / Komari 走多语言（zh-CN/en），Beszel 仅 en。日韩本地化在自托管面板类目里基本无人做（差异化空白）。
- 后端日志 / API 错误信息可以保留英文（开发者向），仅 UI 文案需要 i18n。

## Decision

采用 **react-i18next 作为 i18n 框架，v1 支持 4 语言：zh-CN（默认）/ en / ja / ko**。

1. **代码层面**：所有 UI 文案**严禁硬编码中文**，必须通过 `t('key')` 调用；CI 阶段加 lint 规则检查硬编码中文字符串。
2. **locale 文件结构**：`src/locales/<lang>/<namespace>.json`，命名空间按模块（user / subscription / pipeline / node / agent / notification / rule / script / common 共 9 个）。
3. **默认语言**：zh-CN；首次访问按 `navigator.language` 自动检测；用户登录后偏好持久化到 user 表。
4. **时间 / 数字格式**：用 Intl.DateTimeFormat / Intl.NumberFormat 按 locale 渲染，不使用 dayjs 的固定 format 串。
5. **后端**：API 错误码使用 enum 字符串（如 `ERR_SUB_NOT_FOUND`），前端按 i18n key 翻译；后端不直接返回本地化文本。
6. **文档**：README 提供中英双语；其他文档默认中文。

## Consequences

**正面**：
- 4 语言覆盖东亚 + 国际化基础盘；ja / ko 在自托管面板类目里是空白市场。
- react-i18next 是 React 生态事实标准，社区贡献者（如想加 vi / es）只需 PR 一份 JSON。
- 后端 API 错误码 enum 化是良好工程实践，与 i18n 分离让 API 协议稳定。

**负面 / 待办**：
- ja / ko 翻译质量需要母语者审校；初期可机翻 + 后续社区修正。
- 部分领域术语（如"节点 / 订阅 / 算子"）在日韩有专门用法，需要术语表对照（见 CONTEXT.md）。
- CI lint 规则对硬编码中文的检查有误伤可能（如代码注释、测试用例）；需精调白名单。

**替代方案为何放弃**：
- **只做中文（妙妙屋路线）**：放弃国际化扩张，社区贡献也只能在中文圈。
- **只做 zh-CN / en**：覆盖面不够；调研显示 ja / ko 空白市场存在。
- **走 LinguiJS / FormatJS**：生态不如 react-i18next 成熟，2026 年仍推荐 i18next。

## Related

- ADR 0001: 技术栈（React 19 生态）
- 调研：docs/_research-competitors.md 同类项目多语言现状（Nezha / Komari zh-CN+en）
