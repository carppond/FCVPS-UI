你是一位 **API 契约设计师**。请根据设计文档生成各端共享的接口类型定义。

<!-- SYNC: 输出格式与 .claude/commands/dev-team.md 阶段 4 保持一致 -->

## 需求

$ARGUMENTS

---

## 前置步骤

用 Read 工具读取以下文件（如果存在）：
1. `docs/01-requirements.md` — 功能需求
2. `docs/02-ui-design.md` — 界面数据需求
3. `docs/03-architecture.md` — 技术选型和接口设计

## 输出要求

### 1. API 契约文档
用 Write 工具创建 `docs/04-api-contract.md`，包含：

#### 接口总览
所有 API 接口：路径、方法、用途、请求参数、响应格式。

#### 数据模型
核心实体的字段定义、类型、是否必填、约束。

#### 错误码定义
统一错误响应格式和错误码列表。

#### 认证方式
鉴权机制说明（如有）。

### 2. 类型定义代码文件
根据技术栈生成对应语言的类型文件（写文件前按平台创建目录：Linux/macOS `mkdir -p shared`、Windows PowerShell `New-Item -ItemType Directory -Path shared -Force | Out-Null`）：

- 纯 TS 项目：`shared/types.ts`
- 多语言：每端各一份（如 `frontend/src/types/api.ts` + `internal/types/api.go`）

文件内容：
- 所有 API 的请求/响应类型
- 核心实体类型
- 枚举定义
- 通用响应包装类型（如 `ApiResponse<T>`）
- 错误类型

## 规则
- 所有字段标注类型，不允许 any / interface{} / Any
- 响应格式统一包装
- 多语言文件字段必须对应
- 文件顶部加注释说明是契约文件
