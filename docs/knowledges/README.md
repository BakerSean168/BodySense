# BodySense 开发知识库

本目录记录项目开发过程中的技术知识点和学习笔记。

## 目录结构

```
knowledges/
├── README.md                    ← 本文件
├── issue-1-dev-infra.md         ← Issue 1: 开发环境基础设施
├── issue-2-auth-jwt.md          ← Issue 2: 用户认证 + JWT
├── issue-3-rag-infra.md         ← Issue 3: 知识库 RAG 基础设施
├── issue-13-video-knowledge-pipeline.md ← Issue 13: 视频知识生产线与问答链路
├── issue-13-manual-video-ingestion-runbook.md ← Issue 13: 单视频手动入库操作手册
└── glossary.md                  ← 术语表
```

## 知识点索引

### Issue 1: 开发环境基础设施
- Docker Compose 多容器编排
- PostgreSQL + pgvector 向量数据库
- Redis 内存数据库
- Go 项目结构 (cmd/internal/pkg)
- 数据库迁移 (golang-migrate)
- Dockerfile 多阶段构建
- 环境变量管理

### Issue 2: 用户认证 + JWT
- bcrypt 密码加密
- JWT 双 Token 机制
- Refresh Token + Redis 存储
- Go 分层架构 (Handler/Service/Repository)
- React 状态管理 (Zustand)
- 路由守卫 (ProtectedRoute)
- CORS 跨域配置
- 自动 Token 刷新

### Issue 3: 知识库 RAG 基础设施
- pgvector 向量存储和索引
- Embedding 生成（OpenAI text-embedding-3-small）
- 语义检索（余弦相似度搜索）
- Reranker 重排序（LLM 相关性判断）
- Python RAG 管道架构
- Go 后端代理端点
- 端到端测试流程

### Issue 13: 视频知识生产线与问答链路
- 模块化架构：ASR 子系统 / 切分子系统 / AI 精修子系统
- 三种 ASR 提供商：whisper.cpp（本地）、FunASR（本地）、asr_api（MiMo/OpenAI）
- 两种切分策略：heuristic（关键词规则）、llm（LLM 语义切分）
- AI 精修：逐单元润色 + 质量评分 + 低分自动标记
- `knowledge_sources / segments / units / clips` 归一化知识库结构
- `generated_pack` 与 `curated_pack` 的双阶段知识生产模式
- 咨询页实时问答里的知识检索与 SSE 引用返回
- 诊断分析 / 改善方案链路中的 RAG 注入方式

### Issue 13 操作手册
- 从本地视频到数据库记录的手动执行步骤
- `.env`、数据库、迁移、ASR、embedding 的最小配置
- whisper.cpp / funasr_sensevoice / asr_api 三种 ASR 的选择与命令示例
- heuristic / llm 两种切分策略的配置
- AI 精修的启用方式与质量评分机制
- dry-run 检查、正式写库、SQL 校验、搜索验收
- 从一条 `knowledge_units` 反查原始转录证据的方法
- 全部 CLI 参数速查

### AI Agent 实施难点（`ai-agent-impl/`）

- `unified-consultation-run-sse-pipeline.md` — 统一 `POST /consultation-runs` 端到端流程：幂等性、原子会话创建、SSE 流式转发、静默 URL 跳转、缓存更新、Interaction 中断恢复
- `old-lazy-session-creation-and-silent-navigation.md` — v1 两步式流程（`createConsultation` + `sendConsultationMessage`）的历史实现

## 阅读建议

**了解当前视频到知识库的实现**：先读 `issue-13-video-knowledge-pipeline.md`，它覆盖了完整的系统架构、模块分工、数据模型、三种工作模式和生产策略。

**实际操作入库**：读 `issue-13-manual-video-ingestion-runbook.md`，照着命令一步步来，包含全部 CLI 参数速查和故障排除。

**了解第一代 RAG 基础设施**：`issue-3-rag-infra.md` 记录了早期扁平结构的 RAG 实现，已被 Issue #13 的归一化结构替代，仅供参考。

**了解咨询工作台 AI Agent 实现**：`ai-agent-impl/unified-consultation-run-sse-pipeline.md` 详细拆解了统一端点 `POST /consultation-runs` 的完整流程，包括幂等性、原子会话创建、SSE 流式转发、静默 URL 跳转、缓存更新策略。旧版两步式流程见 `old-lazy-session-creation-and-silent-navigation.md`。

### ASR 选型参考

- `asr-base-vs-small-benchmark.md`：whisper.cpp base vs small 模型对比
- `asr-candidates-field-test-results.md`：4 种 ASR 方案实测结论，推荐 FunASR SenseVoiceSmall GGUF 作为默认本地 ASR

---

*最后更新：2026-06-27*
