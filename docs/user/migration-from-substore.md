# 从 sub-store 迁移 (Migrating from Sub-Store)

拾光VPS 提供 sub-store 兼容路由，原客户端（mihomo / Clash Verge Rev 等）无需改任何配置即可完成迁移。

---

## 兼容原理

拾光VPS 暴露与 sub-store 相同格式的订阅下载路由：

```
GET /download/:name?token=<share_token>
```

- `:name`：对应拾光VPS 中订阅的名称（或 slug）
- `token`：访问令牌，在拾光VPS 创建订阅时生成
- 响应格式：Clash YAML（与 sub-store 输出格式兼容）

静默模式开启时，完整 URL 格式为：

```
https://your-hub.example/_app/<32hex>/download/:name?token=<share_token>
```

---

## 迁移步骤

### 步骤 1：在拾光VPS 创建订阅

1. 登录拾光VPS，导航到 **订阅** 页面
2. 点击 **添加订阅**，类型选择 **URL 订阅**
3. 在 URL 字段中填入你在 sub-store 中使用的原始订阅 URL（机场提供的链接）

<!-- screenshot: 添加订阅对话框，URL 类型，填入原始机场订阅链接 -->

4. 填写订阅名称（如 `my-airport`），名称将作为路由中的 `:name` 部分
5. 点击 **保存**，随后点击 **立即同步** 拉取节点
6. 同步成功后，在订阅详情中找到 **兼容链接** 字段，复制该 URL

<!-- screenshot: 订阅详情页，显示"sub-store 兼容链接"字段 -->

### 步骤 2：替换客户端配置

将 mihomo / Clash Verge Rev / Stash 等客户端中原来的 sub-store URL 替换为拾光VPS 生成的兼容 URL：

**原 sub-store URL（示例）**：
```
http://127.0.0.1:2999/download/my-airport?target=clash
```

**替换为拾光VPS URL（示例）**：
```
https://your-hub.example/_app/a3f8c2d1e4b7096f5a2c3e8d1f4a7b09/download/my-airport?token=abc123def456
```

> 将 `your-hub.example` 替换为你的拾光VPS 实际域名或 IP，`<32hex>` 和 `<token>` 从面板中复制。

### 步骤 3：在客户端更新订阅

在你的客户端中执行"更新订阅"操作，客户端会从拾光VPS 获取节点列表。

**客户端配置无需任何其他修改**——拾光VPS 输出的 Clash YAML 格式与 sub-store 完全兼容。

<!-- screenshot: Clash Verge Rev 中修改订阅 URL 的界面 -->

---

## 功能对照

### 拾光VPS 已支持的 sub-store 功能

| 功能 | sub-store | 拾光VPS |
|------|-----------|---------|
| 多协议 URI 解析（12 种） | 支持 | 支持 |
| ACL4SSR / subconverter 格式 | 支持 | 支持 |
| Clash YAML 输出 | 支持 | 支持 |
| 算子流水线（filter / sort / dedupe / rename / map） | Script Operator | 原生支持，可视化拖拽 |
| 自定义 UA | 支持 | 支持 |
| 订阅自动更新 | 支持 | 支持（默认 6h） |
| 订阅 token 保护 | 支持 | 支持 |
| YAML 导出 git 化管理 | 不支持 | 支持（流水线导出） |
| 探针监控 | 不支持 | 支持 |
| 多渠道通知 | 不支持 | 支持（10 渠道） |

### 拾光VPS v1 不支持的 sub-store 功能

| 功能 | 说明 |
|------|------|
| Surge / Loon / QX / sing-box 等输出格式 | v1 仅输出 Clash YAML；其他格式列入 P2 路线图 |
| sub-store 完整 API 兼容（v2 全部端点） | 仅兼容 `/download/:name` 路由，不支持 sub-store 管理 API |
| 多目标客户端格式转换 | v1 暂不支持 |
| Vercel / Cloudflare Pages 部署模式 | 拾光VPS 是自托管服务端，需要自己的服务器 |

---

## 常见问题

**Q: 迁移后节点数量与 sub-store 不一致怎么办？**

A: 检查以下几点：
1. 原机场订阅是否有 UA 限制，在订阅设置中自定义 User-Agent（如 `clash.meta`）
2. 查看 **订阅详情 → 同步日志**，看是否有解析错误
3. 拾光VPS 会自动过滤 Clash 不支持的节点（如 vless+reality），UI 会有 warning 提示

**Q: 使用了 sub-store 的 Script Operator 脚本，能直接迁移吗？**

A: sub-store 的 Script Operator 是 JS 脚本，拾光VPS 同样支持 JS 脚本钩子（goja 沙箱）。语法上基本兼容，但需要注意：
- 拾光VPS 脚本钩子入口函数名称不同，请参考 **脚本扩展** 文档
- 沙箱禁止网络请求和文件系统访问（同 sub-store 的沙箱限制）

**Q: 短链接能保持不变吗？**

A: 拾光VPS 的订阅 URL 格式与 sub-store 不同，无法保持完全一致的短链。拾光VPS 提供独立的短链系统（**设置 → 短链**），可以为订阅 URL 生成短链后分发。

**Q: sub-store 的 token 认证方式兼容吗？**

A: 拾光VPS 使用自己的 `share_token`，格式不同于 sub-store。迁移时需要从拾光VPS 获取新 token 填入客户端配置。
