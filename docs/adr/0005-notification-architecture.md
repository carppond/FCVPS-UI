# ADR 0005: 通知系统架构 —— NotificationChannel 接口 + 插件化（差异化 #2）

## Context

- 调研报告（docs/_research-competitors.md 第四节"差异化机会 3"）：通知渠道是性价比最高的"卷功能列表"项。
- 妙妙屋只接了 Telegram 单一渠道；探针类项目（Nezha / Komari / Beszel）普遍只有 3-5 个渠道、硬编码。
- 对标 Uptime Kuma 90+ 渠道是高门槛壁垒；本项目锁定 **10+ 主流渠道**（满足 95% 中文圈用户），分阶段覆盖。
- 业界最佳实践（调研报告"差异化机会 5"）：3X-UI 已证明 **Telegram Bot 双向交互**（inline keyboard 当低门槛管理界面）能显著降低运维门槛；本项目应抄。

## Decision

通知系统按"接口 + 适配器"模式设计：

1. **核心接口 `NotificationChannel`**：
   ```go
   type NotificationChannel interface {
       Name() string
       ConfigSchema() jsonschema.Schema
       Send(ctx context.Context, evt Event, cfg map[string]any) error
   }
   ```
2. **v1 渠道清单（10 种）**：Telegram / Discord / Slack / Email（SMTP）/ Bark / Gotify / Webhook（通用）/ Server酱 / PushDeer / IFTTT。
3. **事件类型 opt-in**：每个渠道实例可勾选要订阅的事件子集（节点离线 / 流量告警 / 订阅同步失败 / 备份完成 / 登录异常 / OTA 升级 / 自定义脚本告警 等）。
4. **Telegram Bot 双向交互**：单独支持 inline keyboard，命令包括：
   - `/nodes` 查节点列表 + 延迟
   - `/refresh <sub>` 刷新指定订阅
   - `/agent_restart <id>` 重启指定 agent
   - `/traffic` 查本月已用流量
   - `/silent on|off` 切换静默模式
5. **配置模板系统**：用户自定义消息模板（Go template 语法），不动代码就能改通知格式。

## Consequences

**正面**：
- 10 个渠道覆盖中文圈 95% 用户偏好（特别 Bark / Server酱 / PushDeer 是国内独有）。
- 接口化设计意味着社区贡献新渠道只需加一个 Go 文件 + JSON schema，未来扩到 20+ 不费力。
- Telegram Bot 双向交互让手机端用户不必打开 Web 即可运维，显著降低门槛（VPS 圈 90%+ 用户用 Telegram）。
- 模板系统让用户自定义 webhook 格式，无需改源码就能接企业内部 IM。

**负面 / 待办**：
- Email（SMTP）渠道需要用户自备邮件服务器或第三方 SMTP，配置复杂度比其他渠道高。
- IFTTT 自 2024 起免费额度收紧，作为兜底渠道存在但不主推。
- Telegram Bot 双向需要长连接（webhook 或 polling），轮询模式增加 hub 负载；默认 webhook 模式，但要求用户有公网 URL。

**替代方案为何放弃**：
- **复刻 Uptime Kuma 全部 90+ 渠道**：边际收益递减，60% 渠道用户数 < 100，工程投入不值。
- **硬编码 3-5 个渠道（Nezha / 妙妙屋当前路线）**：差异化丢失。
- **走 Apprise / Shoutrrr 第三方通知库**：依赖外部项目演进；自写接口控制力更强。

## Related

- ADR 0006: 静默模式（事件触发与未授权请求不混淆）
- 调研：docs/_research-competitors.md 第四节"差异化机会 3 + 5" + 第二节"通知渠道"行
