# 体悟 BodySense

> **Agent 驱动的个人身体状态应用** —— 长期记录身体变化、维护一份当前综合诊断、管理多份独立训练计划。

<p align="left">
  <strong>语言：</strong> <a href="../../README.md">英文</a> · 简体中文
</p>

<p align="left">
  <a href="https://body.bakersean.top"><strong>在线产品</strong></a> ·
  <a href="https://bakersean168.github.io/BodySense/"><strong>项目介绍</strong></a> ·
  <a href="../product/README.md"><strong>产品规格</strong></a> ·
  <a href="../architecture/README.md"><strong>系统架构</strong></a>
</p>

BodySense 将用户的身体信息保存为有来源、具有时间语义的长期状态。用户清楚描述本人的情况时自动记录；主体、时间或部位有关键歧义时，通过交互工具追问。用户可以按需请求分析单个或多个病情，形成一份当前综合 Diagnosis；可独立管理多个训练计划。正式 Diagnosis 的发布需要用户请求或授权，训练内容变更必须来自明确请求或已接受的 Agent 建议。

界面以持续存在的 3D 人体为底层桌面，Chat、Person、Diagnosis、TrainingPlan 和 Trainer 是可移动、可收起的浮动窗口。用户日常只需查看当前信息，不必管理技术版本。

> [!IMPORTANT]
> BodySense 用于个人健康信息整理与有证据边界的辅助解释，并非医疗器械，不能替代专业医疗诊断和治疗。

## 核心业务

- **BodyState**：一个用户一份，明确的本人身体陈述自动保存，真实病程与记录更正分开处理。
- **Diagnosis**：一个用户一份当前综合结果，内部病情可以单独更新，同次变更构成一次整体修订。
- **TrainingPlan**：允许多个独立计划，只有明确的用户请求或接受建议才能更改训练内容。
- **Agent**：自然交流、针对性证据采集、澄清、诊断分析与训练影响检查。
- **3D 工作空间**：始终存在的人体 Canvas、上下文工具和浮动业务窗口，支持桌面与移动端。

详情见 [产品业务规格](../product/README.md)。

## 工程价值

BodySense 重点解决的不是“如何调用一个模型”，而是 **AI 输出如何成为可持久、可追踪、可复审的产品状态**。

| 工程问题 | BodySense 的处理方式 |
| --- | --- |
| 长时间 AI 交互 | Run / Turn / checkpoint + 持久事件日志 |
| 人在回路 | 结构化 `ask_user` interrupt / resume |
| AI 与业务状态 | Go 持有持久业务真值，AI 运行时只产生经过校验的提案与事件 |
| 检索 | 基于 pgvector 的证据检索与来源追踪 |
| 模型边界 | 独立 LiteLLM 网关，应用代码不直接持有物理供应商路由 |
| 诊断治理 | 不可变配置身份、回放评估与硬安全门禁 |
| 生产交付 | 一致的不可变发布产物与 revision 校验发布 |

## 系统架构

```text
React Web
   │  HTTP + SSE
   ▼
Go API / 领域真值
   │
   ├── PostgreSQL + pgvector
   ├── Redis
   └── Python AI Service
          │
          ├── FastAPI
          ├── LangGraph 持久运行时
          ├── PydanticAI 类型化 Agent
          └── LiteLLM 模型网关
```

仓库采用 Nx + pnpm monorepo：

```text
apps/
├── web/          React 19 + TypeScript + Vite
├── api/          Go + Gin + GORM
└── ai-service/   Python + FastAPI + LangGraph + PydanticAI

docker/           本地 / 生产编排
scripts/          校验与发布工具
docs/             产品规格、ADR、架构与计划
tools/            项目专用工程工具
```

当前运行时与部署边界见 [`docs/architecture/deployment-architecture.md`](../architecture/deployment-architecture.md)。

## 技术栈

- **前端：** React 19、TypeScript 6、Vite 8、Tailwind CSS 4、shadcn/ui
- **后端：** Go 1.26、Gin、GORM
- **AI：** Python 3.13、FastAPI、LangGraph、PydanticAI
- **数据：** PostgreSQL + pgvector、Redis
- **模型网关：** LiteLLM
- **工作区：** Nx 23、pnpm 11
- **质量体系：** Vitest、Go test、Pytest、Playwright、Ruff/Pyright、ESLint
- **交付：** GitHub Actions、Docker/Compose、Alibaba Cloud ACR + ECS

## 快速开始

### 环境要求

- Node.js 24+
- pnpm 11+
- Go 1.26+
- Python 3.13+
- Docker 与 Compose

### 本地运行

```bash
git clone https://github.com/BakerSean168/BodySense.git
cd BodySense
pnpm install
pnpm dev
```

`pnpm dev` 是 direct-dev 的标准交互入口：它会幂等确认 Docker 基础设施健康，读取存在的 `.env.dev.local`，然后在宿主机前台启动 Web / API / AI，并保留热更新能力。

基础设施刻意比应用进程更长寿：

```bash
pnpm dev:infra:status  # PostgreSQL 18 + pgvector、Redis、LiteLLM
pnpm dev:infra:up      # 创建/启动并等待健康
pnpm dev:infra:logs    # 查看基础设施日志
pnpm dev:infra:down    # 显式拆除；持久化数据卷保留
```

默认 direct-dev 端口为 Web `20100`、API `20101`、AI `20102`、PostgreSQL `20110`、Redis `20111`、LiteLLM `20112`。基础设施容器采用 `restart: unless-stopped`，因此 Docker 或宿主机正常重启后会自动恢复，但 Web / API / AI 不会被强制常驻。

机器相关覆盖项和模型供应商凭据放到 Git 忽略的 `.env.dev.local`。`scripts/dev-env.sh` 已提供非敏感开发默认值，不再要求把 `.env.example` 复制成 `.env`。direct-dev 的 Web lane 默认启用 Vite Bundled Dev：工作站通过 Tailscale/SSH 访问 GCP 时，浏览器侧会从大量 native-ESM 模块请求收敛为少量 bundled development assets，同时保留 HMR。需要诊断 Vite/plugin 本身时，可在 `.env.dev.local` 设置 `BODYSENSE_VITE_BUNDLED_DEV=false` 临时回退 classic Vite；测试 lane 固定保持 classic。

如果不需要完整技术栈，可以单独启动某个运行面：

```bash
pnpm dev:web
pnpm dev:api
pnpm dev:ai
```

## 验证

仓库尽量让本地验证与 CI 使用同一套质量契约。

```bash
pnpm lint
pnpm typecheck
pnpm test
pnpm e2e
pnpm verify:release
```

`pnpm verify:release` 是仓库级综合发布门禁。CI 还会验证已发布数据库迁移历史和长期健康浏览器流程，全部通过后版本才具备生产发布资格。

### API 契约与 Postman

浏览器侧 REST 契约只有一个真值来源：[`packages/contracts/openapi/bodysense.v1.openapi.yaml`](../../packages/contracts/openapi/bodysense.v1.openapi.yaml)。Postman collection 由这份 OpenAPI 确定性生成，不再维护第二套手写接口规格：

```bash
pnpm postman:generate          # 重新生成 collection 与安全环境模板
pnpm postman:verify            # 新鲜度 + 95/95 OpenAPI 路由一致性
pnpm postman:validate-native   # 可选：使用已安装的 Postman CLI 做原生格式校验
```

环境变量、内部服务边界以及 Native Git / Postman Cloud 绑定方式见 [`postman/README.md`](../../postman/README.md)。

## 生产环境

当前生产应用：**[body.bakersean.top](https://body.bakersean.top)**。

生产应用与 GitHub Pages 项目介绍页有意保持职责分离：

- **GitHub Pages** 负责解释项目价值与工程故事。
- **Alibaba Cloud ECS** 运行真实产品。
- **Alibaba Cloud ACR** 保存生产 OCI 产物。
- **GitHub Actions** 负责 CI、版本发布与不可变镜像发布。

当前部署契约见 [`docs/architecture/deployment-architecture.md`](../architecture/deployment-architecture.md)。

## 文档

- [文档总索引](../README.md)：现行文档、主题位置与归档说明
- [产品文档导航](../product/README.md)
- [产品需求](../product/requirements.md)
- [领域契约](../product/domain/README.md)
- [Agent 工作流](../product/agent-workflows.md)
- [UI 与交互](../product/interface.md)
- [验收标准](../product/acceptance.md)
- [术语表与 Penpot 名称对照](../product/glossary.md)
- [系统架构](../architecture/README.md)
- [仓库协作规范](../../AGENT.md)

## 仓库状态与许可

BodySense 是一个 **公开源码仓库**，但当前 **没有附带开源许可证**。仅公开可见并不代表授予复制、修改或再分发代码的许可；除非未来明确加入许可证，否则版权仍由仓库所有者保留。
