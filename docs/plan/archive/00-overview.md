# AI 会话管理重设计方案 — 总览

## 背景

当前项目的会话管理存在以下问题：

- 消息以 JSONB 数组存储在 `consultation_sessions` 表的单行中，无法对单条消息做原子更新
- `AppendMessage` 存在读-改-写竞态条件（无锁）
- Go handler 被动解析 Python 返回的 SSE 字符串来重建状态，耦合度高
- 前端生成正式 session ID（`CreateSessionWithID`），架构上不合理
- 无幂等保护，重复请求会创建重复消息/会话
- 无 `turnId`/`requestId` 等概念，难以支撑重生成、分支、审计等功能

由于当前无生产数据，决定**完全抛弃现有实现，从零设计**。

---

## 核心设计决策

| # | 决策项 | 选择 | 理由 |
|---|--------|------|------|
| 1 | 消息存储 | 独立 `messages` 表 | 支持单条消息原子更新、并发安全、状态追踪 |
| 2 | 领域模型 | 通用表 + 咨询扩展表 | 通用层（conversations/messages/runs）复用，consultation_sessions 为领域扩展 |
| 3 | Go handler 角色 | 权威编排器 | Go 唯一写 DB、唯一拥有客户端 SSE 协议；Python 为无状态 LLM 服务 |
| 4 | 阶段管理 | 混合模式 | phase 枚举保留为契约，AI 提议转换，Go 校验合法性 |
| 5 | Python 服务职责 | Agent 循环内部处理 | Python 处理工具调用、知识检索、症状提取，返回结构化事件流 |
| 6 | 上下文串联 | 包含 response_id | Python 返回 `response_id`，Go 存储并在下一轮回传，支持 provider 侧上下文优化 |
| 7 | 幂等性 | runs 表级别幂等 | `unique(user_id, request_id)` 约束，重复请求返回已有 run |
| 8 | Turn vs Run | 两者都有 | turn_id 分组一轮对话，run_id 追踪一次执行；MVP 阶段 1:1 |
| 9 | 会话创建 | 懒创建 | 点击"新咨询"不入库，首条消息才创建 conversation |
| 10 | 消息落库 | user 立即落库，assistant 先 placeholder | 用户消息收到即存；assistant 先创建 streaming 状态行，完成后更新 |
| 11 | ID 生成 | 服务端 UUIDv7 | 所有正式 ID 由服务端生成，前端仅用临时 ID |
| 12 | SSE 协议 | Go 定义完整客户端协议 | Go 发 conversation.created / text.delta / done 等事件，Python 返回结构化 JSON 行 |
| 13 | 消息格式 | parts JSONB | 支持文本、图片、工具调用、引用等多模态内容 |
| 14 | 处理策略 | 失败保留 failed message | 不静默删除，前端显示重试入口 |

---

## 架构总览

```
┌─────────────┐     SSE (Go-defined protocol)     ┌──────────────┐
│   Frontend   │ ◄──────────────────────────────── │   Go API     │
│  (React +    │     POST /api/v1/chat/send        │  (Gin)       │
│ assistant-ui)│ ────────────────────────────────► │              │
└─────────────┘                                    │  ┌─────────┐ │
                                                    │  │ Runs    │ │
                                                    │  │ Messages│ │
                                                    │  │ Convos  │ │
                                                    │  └────┬────┘ │
                                                    └───────┼──────┘
                                                            │
                                              structured    │  PostgreSQL
                                              JSON events   │
                                                            │
                                                    ┌───────▼──────┐
                                                    │  Python AI   │
                                                    │  Service     │
                                                    │  (FastAPI)   │
                                                    │              │
                                                    │  - LLM call  │
                                                    │  - Tools     │
                                                    │  - Agent loop│
                                                    └──────────────┘
```

### 职责划分

| 层 | 职责 |
|---|------|
| **Frontend** | UI 渲染、乐观更新、临时 ID 生成、SSE 消费、路由管理 |
| **Go API** | 认证、会话生命周期管理、消息持久化、SSE 协议生成、幂等校验、阶段校验 |
| **Python AI** | LLM 调用、工具执行（知识检索、症状提取）、Agent 循环、流式文本生成 |
| **PostgreSQL** | 唯一数据源（source of truth） |

---

## 关键工程规则

1. 新咨询点击不入库，首条消息才创建 conversation
2. 正式 ID 只由服务端生成（UUIDv7）
3. 前端的 clientDraftId、clientMessageId 仅为临时 ID
4. 每次发送必须带 requestId，用于幂等
5. user message 收到请求后立即落库
6. assistant message 开始生成前先创建 streaming placeholder
7. SSE 第一批事件返回 conversationId 和正式 messageId
8. 前端收到 conversation.created 后 router.replace 到正式 URL
9. assistant 生成完成后更新 message.status = completed
10. 失败时保留 failed message，不静默删除
11. 会话标题异步生成，不阻塞主回复
12. 自己的 DB 是 source of truth，provider 的 response_id 仅为优化字段

---

## 文档索引

| 文件 | 内容 |
|------|------|
| [01-schema-design.md](./01-schema-design.md) | 数据库表结构设计 |
| [02-api-design.md](./02-api-design.md) | API 端点与请求/响应格式 |
| [03-sse-protocol.md](./03-sse-protocol.md) | 客户端 SSE 事件协议 |
| [04-python-contract.md](./04-python-contract.md) | Python AI 服务接口契约 |
| [05-frontend.md](./05-frontend.md) | 前端改造方案 |
| [06-implementation-roadmap.md](./06-implementation-roadmap.md) | 实施路线图 |
