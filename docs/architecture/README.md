# BodySense 架构与设计指南 (Architecture & Design Guide)

本目录为 BodySense 项目的核心架构与设计文档中心。项目采用 Monorepo 单仓管理，技术栈涵盖 React 前端、Go 业务网关及 Python AI 算法微服务。

---

## 1. 核心架构文档

这里包含了系统全局的技术方案与部署架构规范：

- 📑 **[技术方案大纲 (Technical Approach)](file:///d:/home/projects/BodySense/docs/architecture/technical-approach.md)**
  *详细记录了项目的核心技术选型（React 19, Go 1.26, Python 3.13, PostgreSQL 18）、Monorepo 项目结构设计、三端接口交互契约与安全机制。*
  
- 🐳 **[部署与运维架构 (Deployment Architecture)](file:///d:/home/projects/BodySense/docs/architecture/deployment-architecture.md)**
  *描述了基于 Docker Compose 的容器编排设计（Caddy 反向代理、Go API、FastAPI AI-Service、Redis 7、PostgreSQL 18），本地/生产端口定义以及 GitHub Actions CI/CD 滚动部署流水线。*

---

## 2. 历史方案与设计文档索引 (Archived Plans)

随着项目的迭代，所有的阶段性实施方案已成功上线，并归档至历史库中。你可以通过以下索引追溯各个系统模块的设计原委与演进历程：

### 核心底座与会话重设计
* **通用会话管理统一重构**：**[会话管理系统统一重设计方案](file:///d:/home/projects/BodySense/docs/plan/archive/unified-session-redesign.md)**
  *解决了最初版本的并发读-改-写竞态、缺乏幂等控制、及会话消息绑定具体领域的问题。引入了通用的 `conversations` / `messages` / `runs` / `shares` 抽象，并在 **PostgreSQL 18** 中直接启用了原生的递增 **`uuidv7()`** 作为主键提升 B-Tree 索引写入效率。*
  * *子方案：[Schema 设计](file:///d:/home/projects/BodySense/docs/plan/archive/01-schema-design.md) · [API 设计](file:///d:/home/projects/BodySense/docs/plan/archive/02-api-design.md) · [SSE 流式协议](file:///d:/home/projects/BodySense/docs/plan/archive/03-sse-protocol.md) · [Go-Python 契约](file:///d:/home/projects/BodySense/docs/plan/archive/04-python-contract.md) · [前端适配器](file:///d:/home/projects/BodySense/docs/plan/archive/05-frontend.md) · [实施路线图](file:///d:/home/projects/BodySense/docs/plan/archive/06-implementation-roadmap.md) · [方案总览](file:///d:/home/projects/BodySense/docs/plan/archive/00-overview.md)*

* **多 Provider AI 服务网关**：**[多供应商 AI 路由设计方案](file:///d:/home/projects/BodySense/docs/plan/archive/multi-provider-ai-router.md)**
  *设计了基于 YAML 配置（`models.yaml`）驱动的多供应商 AI 自动路由，支持基于有序候选队列的故障降级（fallback）与针对 HTTP 429 速率限制的指数级熔断退避（CircuitBreaker）机制。*

* **安全认证**：**[JWT 认证系统设计](file:///d:/home/projects/BodySense/docs/plan/archive/issue-2-auth-jwt.md)**
  *完成了基于 JWT Token 的用户注册、登录、登出、双 Token 双通道刷新及受保护路由中间件开发。*

### 咨询工作台与业务功能链路
* **端到端核心功能合集 (Issues 6-12)**：**[核心业务功能实施总案](file:///d:/home/projects/BodySense/docs/plan/archive/issues-6-12-implementation.md)**
  *定义并串联了问诊咨询（Chat）、用户画像提取（Info Panel）、人体部位热力可视化（Body Viz）、诊断报告生成（Assessment Report）、训练计划下发与每日打卡（Training Plan）、训练进度追踪与重估（Reassessment）的完整数据链。*

* **咨询工作台深度优化**：**[咨询工作台代码质量修复](file:///d:/home/projects/BodySense/docs/plan/archive/consultation-workbench-code-quality-fixes.md)** 与 **[咨询工作台核心分析](file:///d:/home/projects/BodySense/docs/plan/archive/consultation-workbench-ai-agent-core-analysis.md)**
  *重点排查并修复了咨询主页的多流并发加载、状态流失、乐观更新及图表渲染性能问题。*

### AI 算法与 RAG 知识库
* **自动化视频知识库入库**：**[头前移自动知识入库方案](file:///d:/home/projects/BodySense/docs/plan/archive/issue-13-forward-head-knowledge-pilot.md)**
  *打通了从本地 MP4 动作演示视频开始，经由 Whisper 语音转写 ASR、语义/规则切分、AI Curator 品控精修、动作片段切割（FFmpeg）直至自动导入向量库的完整 RAG 资产构建流水线。*
  * *子方案：[ASR 语音转写模型基准对比](file:///d:/home/projects/BodySense/docs/plan/archive/asr-base-vs-small-benchmark.md) · [肘外翻与肱骨前移实测](file:///d:/home/projects/BodySense/docs/plan/archive/asr-candidates-field-test-humerus-elbow.md) · [实测统计汇总](file:///d:/home/projects/BodySense/docs/plan/archive/asr-candidates-field-test-results.md) · [已入库库内资产统计](file:///d:/home/projects/BodySense/docs/plan/archive/issue-13-live-ingestion-status.md)*

* **RAG 向量检索底座**：**[RAG 知识检索基础设施方案](file:///d:/home/projects/BodySense/docs/plan/archive/issue-3-rag-infra.md)**
  *设计并实现了基于 PostgreSQL `pgvector` 的动作库向量存储与检索核心服务。*

* **文件上传与 OCR 模块**：**[图片/文件上传与体检报告 OCR 方案](file:///d:/home/projects/BodySense/docs/plan/archive/issue-5-file-upload-ocr.md)**
  *实现了基于 local ocr 与 PaddleOCR 的体检报告文本抽取，支撑健康诊断的基础数据画像输入。*
