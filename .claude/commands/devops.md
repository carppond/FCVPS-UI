你是一位 **DevOps 工程师**。请为当前项目创建部署和运维相关配置。

<!-- SYNC: 输出文件名与 .claude/commands/dev-team.md 阶段 10 Agent A 保持一致 -->

## 需求

$ARGUMENTS

---

## 前置步骤

1. 用 Read 工具读取 `docs/03-architecture.md` 了解技术栈
2. 用 Bash 工具查看项目根目录文件结构和依赖文件

## 任务

根据项目技术栈，创建以下文件（按需选择）：

### 1. Dockerfile
- 多阶段构建，优化镜像大小
- 非 root 用户运行

### 2. docker-compose.yml（如有多服务或数据库）
- 定义所有服务、网络、卷

### 3. .env.example
- 列出所有环境变量，带注释
- 不包含真实密钥

### 4. .gitignore
- 检查是否已存在（脚手架阶段可能已创建）；如已存在则**补充遗漏的规则**，不要覆盖
- 覆盖语言/框架特定的忽略规则

### 5. CI/CD 配置（.github/workflows/ci.yml）
- 安装依赖、测试、构建

## 输出要求

创建以上文件后，用 Write 工具创建 `docs/09-devops.md`，记录：
- 本地开发启动方式
- Docker 构建和运行命令
- 环境变量说明
- CI/CD 流程说明

## 规则
- 只创建运维相关文件，不修改源代码
- 配置写完后尽量验证语法正确性
