# 流水线常用模式 (Pipeline Cookbook)

本文档提供算子流水线的常用配方，每个示例都附 YAML 配置，可直接复制到拾光VPS 流水线编辑器的 YAML 面板中导入。

**YAML Schema**：`apiVersion: shiguang/v1`

---

## 流水线基础

算子按数组顺序依次执行，前一个算子的输出是下一个算子的输入：

```
原始节点 → op1 → op2 → ... → 最终节点
```

**如何使用 YAML**：
1. 导航到 **流水线** 页面，点击 **新建流水线**
2. 在编辑器顶部切换到 **YAML** 标签页
3. 粘贴下方的 YAML 配置，点击 **导入**
4. 切回 **画布** 视图可看到可视化结果

---

## 模式一：保留指定地区节点

**场景**：只保留香港和日本的节点。

使用算子 `filter`，按 `country_code` 字段过滤。

```yaml
apiVersion: shiguang/v1
name: 保留港日节点
ops:
  - type: filter
    params:
      mode: include
      field: country_code
      values:
        - HK
        - JP
```

**参数说明**：

| 参数 | 说明 | 可选值 |
|------|------|--------|
| `mode` | 过滤模式 | `include`（仅保留）/ `exclude`（排除） |
| `field` | 过滤字段 | `country_code` / `protocol` / `tag` / `name` |
| `values` | 匹配值列表 | 字符串数组 |

**其他常用变体**：

```yaml
# 排除大陆节点
ops:
  - type: filter
    params:
      mode: exclude
      field: country_code
      values:
        - CN

# 只保留 VLESS 和 Trojan 协议
ops:
  - type: filter
    params:
      mode: include
      field: protocol
      values:
        - vless
        - trojan
```

---

## 模式二：按延迟排序

**场景**：将节点按延迟从低到高排序，延迟低的节点排在前面。

使用算子 `sort`，按 `latency` 字段升序排列。

```yaml
apiVersion: shiguang/v1
name: 按延迟排序
ops:
  - type: sort
    params:
      field: latency
      direction: asc
      null_last: true   # 未测速节点排到最后
```

**参数说明**：

| 参数 | 说明 | 可选值 |
|------|------|--------|
| `field` | 排序字段 | `latency` / `name` / `protocol` / `country_code` |
| `direction` | 排序方向 | `asc`（升序）/ `desc`（降序） |
| `null_last` | 空值（未测速）节点排到最后 | `true` / `false` |

**其他常用变体**：

```yaml
# 按名称字母顺序排序
ops:
  - type: sort
    params:
      field: name
      direction: asc
```

---

## 模式三：重命名节点

**场景**：将节点名称统一格式化为 `[地区] 节点 N`，例如把杂乱的节点名改为 `[HK] 节点 1`、`[JP] 节点 2`。

使用算子 `regex-rename`，通过正则表达式匹配和替换节点名称。

```yaml
apiVersion: shiguang/v1
name: 重命名节点
ops:
  - type: regex-rename
    params:
      # 匹配含有国家代码的节点（不区分大小写）
      pattern: "(?i)(hong kong|hk|hongkong)"
      replacement: "[HK] 节点 {index}"
      index_scope: per_replacement   # 每个 replacement 独立计数

  - type: regex-rename
    params:
      pattern: "(?i)(japan|jp|日本)"
      replacement: "[JP] 节点 {index}"
      index_scope: per_replacement
```

**参数说明**：

| 参数 | 说明 |
|------|------|
| `pattern` | Go 正则表达式，匹配节点名称 |
| `replacement` | 替换后的名称，`{index}` 会被替换为序号（从 1 开始） |
| `index_scope` | 序号范围：`per_replacement`（每个规则独立计数）/ `global`（全局统一计数） |

**注意**：正则匹配失败不会 panic，未匹配的节点保持原名不变。

---

## 模式四：去除重复节点

**场景**：订阅来自多个渠道时，可能包含相同的节点（相同 server + port），使用 `dedupe` 去除重复。

使用算子 `dedupe`，按指定字段组合判断重复。

```yaml
apiVersion: shiguang/v1
name: 去重节点
ops:
  - type: dedupe
    params:
      # 按 server + port 组合判断重复，保留第一个出现的节点
      keys:
        - server
        - port
      keep: first   # first = 保留第一个，last = 保留最后一个
```

**参数说明**：

| 参数 | 说明 | 可选值 |
|------|------|--------|
| `keys` | 用于判断重复的字段列表 | `server` / `port` / `protocol` / `uuid` 等节点字段 |
| `keep` | 当有重复时保留哪个 | `first`（默认）/ `last` |

---

## 模式五：输出为 Clash YAML

**场景**：将处理后的节点以 Clash YAML 格式输出，供客户端使用。

使用算子 `output`，指定输出格式。

```yaml
apiVersion: shiguang/v1
name: Clash 输出
ops:
  - type: output
    params:
      format: clash
      # 可选：注入自定义 proxy-groups 和 rules
      include_groups: true
      include_rules: true
```

**参数说明**：

| 参数 | 说明 | 可选值 |
|------|------|--------|
| `format` | 输出格式 | `clash`（目前 v1 仅支持此格式） |
| `include_groups` | 是否在输出中包含 proxy-groups | `true` / `false` |
| `include_rules` | 是否包含规则（引用用户配置的 custom_rules） | `true` / `false` |

---

## 模式六：完整组合流水线

**场景**：综合过滤 → 去重 → 排序 → 重命名 → 输出的完整流水线，适合日常使用。

```yaml
apiVersion: shiguang/v1
name: 生产环境完整流水线
description: "过滤香港日本节点 → 去重 → 按延迟排序 → 统一命名 → Clash 输出"
ops:
  # 第一步：只保留香港和日本节点
  - type: filter
    params:
      mode: include
      field: country_code
      values:
        - HK
        - JP

  # 第二步：去除 server+port 相同的重复节点
  - type: dedupe
    params:
      keys:
        - server
        - port
      keep: first

  # 第三步：按延迟升序排序，未测速排最后
  - type: sort
    params:
      field: latency
      direction: asc
      null_last: true

  # 第四步：重命名为统一格式
  - type: regex-rename
    params:
      pattern: "(?i)(hong kong|hk|hongkong)"
      replacement: "[HK] {index}"
      index_scope: per_replacement

  - type: regex-rename
    params:
      pattern: "(?i)(japan|jp|日本)"
      replacement: "[JP] {index}"
      index_scope: per_replacement

  # 第五步：输出 Clash YAML
  - type: output
    params:
      format: clash
      include_groups: true
      include_rules: true
```

<!-- screenshot: 完整流水线的画布视图，5 个算子依次连接，右侧显示参数面板 -->

---

## 算子汇总

| 算子 | 用途 | 关键参数 |
|------|------|----------|
| `filter` | 按字段值过滤节点（保留或排除） | `mode`, `field`, `values` |
| `sort` | 按字段排序 | `field`, `direction`, `null_last` |
| `dedupe` | 去除重复节点 | `keys`, `keep` |
| `regex-rename` | 正则重命名 | `pattern`, `replacement`, `index_scope` |
| `map` | 批量修改节点字段值 | `field`, `value` 或 `template` |
| `output` | 指定输出格式 | `format` |

---

## 调试技巧

**使用预览面板**：

在流水线编辑器中点击 **运行预览**，可以看到每个算子执行前后的节点变化：
- 绿色行：新增节点
- 红色行：被过滤掉的节点
- 数字标注：节点总数变化

**YAML 导出到 git**：

点击右上角 **导出 YAML**，将流水线配置提交到 git 仓库，实现 GitOps 化管理：

```bash
# 示例：将流水线 YAML 纳入版本控制
git add pipelines/production.yaml
git commit -m "feat: 添加港日生产流水线"
```

**性能参考**：100 个节点 + 6 个算子在 2 核 4GB VPS 上运行时间 < 500ms。
