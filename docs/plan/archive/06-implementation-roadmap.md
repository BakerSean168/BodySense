# 06 — 实施路线图

## 原则

- **自底向上**：先建数据层，再建服务层，最后改前端
- **可验证**：每个阶段完成后都有可运行的端到端流程
- **不破坏现有功能**：在新分支上开发，旧代码保留直到新流程验证通过

---

## 阶段 1：数据层（Go API）

**目标：** 新的数据库 schema 和 repository 层就绪

### 1.1 数据库迁移

- [ ] 创建 `conversations` 表
- [ ] 创建 `messages` 表
- [ ] 创建 `runs` 表
- [ ] 创建 `consultation_sessions` 表
- [ ] 编写 migration 文件（golang-migrate 或 GORM AutoMigrate）

### 1.2 Model 层

- [ ] `model/conversation.go` — Conversation struct
- [ ] `model/message.go` — Message struct（含 parts JSONB）
- [ ] `model/run.go` — Run struct
- [ ] `model/consultation_session.go` — 重写，引用 conversation_id
- [ ] UUIDv7 生成工具函数

### 1.3 Repository 层

- [ ] `repository/conversation_repository.go`
  - Create, GetByID, ListByUserID, Update, SoftDelete
- [ ] `repository/message_repository.go`
  - Create, GetByID, ListByConversationID, UpdateStatus, UpdateParts
  - GetNextSeq（获取会话下一条消息的序号）
- [ ] `repository/run_repository.go`
  - Create, GetByID, GetByRequestID, ListByConversationID, UpdateStatus
  - CheckIdempotency（幂等检查）
- [ ] `repository/consultation_repository.go` — 重写
  - Create（关联 conversation_id）, GetByConversationID
  - UpdatePhase, UpdateExtractedInfo, UpdateDiagnosis, UpdateTreatmentPlan

### 1.4 Service 层

- [ ] `service/conversation_service.go`
  - CreateConversation, GetConversation, ListConversations, DeleteConversation
- [ ] `service/message_service.go`
  - CreateMessage, GetMessages, UpdateMessageStatus
- [ ] `service/run_service.go`
  - CreateRun, CheckIdempotency, CompleteRun, FailRun
- [ ] `service/consultation_service.go` — 重写
  - CreateConsultation, GetConsultation
  - UpdatePhase（含 ShouldAdvancePhase 校验）
  - UpdateExtractedInfo, UpdateDiagnosis, UpdateTreatmentPlan

**验证点：** 单元测试通过，repository 层可以正确 CRUD

---

## 阶段 2：Go API Handler 层

**目标：** 新的 API 端点就绪，SSE 协议实现

### 2.1 通用会话 Handler

- [ ] `handler/chat_handler.go`
  - `SendMessage` — 核心端点
    - 幂等检查
    - 事务创建 conversation + user message + assistant placeholder + run
    - 调用 Python AI 服务
    - SSE 流式响应
    - 流结束后持久化
  - `handleFirstMessage` — 首条消息的特殊处理（创建 conversation）
  - `handleContinueMessage` — 后续消息处理

- [ ] `handler/conversation_handler.go`
  - ListConversations, GetConversation, DeleteConversation, GenerateTitle

### 2.2 咨询领域 Handler

- [ ] `handler/consultation_handler.go` — 重写
  - GetConsultation, UpdateExtractedInfo, ConfirmDiagnosis
- [ ] `handler/diagnosis_handler.go` — 重写
  - AnalyzeDiagnosis, GenerateTreatment
- [ ] `handler/knowledge_helper.go` — 保留，微调接口

### 2.3 SSE 协议层

- [ ] `handler/sse_writer.go`
  - 封装 SSE 事件写入
  - 类型安全的事件构造器

```go
type SSEWriter struct {
  w     http.ResponseWriter
  flush func()
}

func (s *SSEWriter) ConversationCreated(convID, draftID string) error
func (s *SSEWriter) MessagePersisted(clientMsgID, msgID, role string) error
func (s *SSEWriter) MessageCreated(msgID, role, turnID string) error
func (s *SSEWriter) TextDelta(msgID, delta string) error
func (s *SSEWriter) ExtractedInfo(msgID string, info any) error
func (s *SSEWriter) PhaseChange(msgID, from, to, reason string, rejected bool) error
func (s *SSEWriter) Citation(msgID string, citation any) error
func (s *SSEWriter) RedFlag(msgID string, flag any) error
func (s *SSEWriter) MessageCompleted(msgID string, usage any) error
func (s *SSEWriter) MessageFailed(msgID string, err any) error
func (s *SSEWriter) TitleGenerated(convID, title string) error
func (s *SSEWriter) Done() error
```

### 2.4 Python AI 客户端

- [ ] `service/ai_client.go`
  - `ChatStream` — 调用 Python `/api/chat/stream`，返回结构化事件 channel
  - `AnalyzeDiagnosis` — 调用 `/api/diagnosis/analyze`
  - `GenerateTreatment` — 调用 `/api/diagnosis/treatment`
  - `SearchKnowledge` — 调用 `/api/knowledge/search`

```go
type AIEvent struct {
    Type   string          `json:"type"`
    Delta  string          `json:"delta,omitempty"`
    Info   json.RawMessage `json:"info,omitempty"`
    Phase  string          `json:"phase,omitempty"`
    // ...
}

func (c *AIClient) ChatStream(ctx context.Context, req ChatRequest) (<-chan AIEvent, error)
```

### 2.5 路由注册

- [ ] 更新 `main.go` 或 router 文件，注册新端点
- [ ] 保留旧端点的兼容路由（可选，如有需要）

**验证点：**
- `POST /api/v1/chat/send` 返回正确的 SSE 流
- 幂等检查工作正常（重复 requestId 返回 409）
- 新会话正确创建 conversation + messages + runs
- 已有会话正确追加消息

---

## 阶段 3：Python AI 服务改造

**目标：** Python 返回结构化 JSON 行流，而非客户端 SSE

### 3.1 聊天流式接口

- [ ] 改造 `POST /api/chat/stream`
  - 输入：新的请求格式（messages + context + tools）
  - 输出：NDJSON 流（text_delta, tool_call, tool_result, phase_change, etc.）
- [ ] 改造 Agent 循环
  - 工具调用在内部完成，对外暴露 tool_call / tool_result 事件
  - 阶段转换由 AI 决策，返回 phase_change 事件
  - 症状提取返回 extracted_info 事件

### 3.2 诊断和治疗接口

- [ ] 适配新的请求格式
- [ ] 响应格式保持 JSON（非流式）

### 3.3 知识库搜索接口

- [ ] 保持现有接口，微调响应格式

**验证点：**
- Python 服务可以独立测试，返回正确的结构化事件
- Go 可以正确解析 Python 的事件流

---

## 阶段 4：前端改造

**目标：** 前端适配新协议和新 API

### 4.1 类型定义

- [ ] `types/consultation.ts` — 重写
  - 新的 ConsultationSession, ChatMessage, MessagePart 类型
  - SSE 事件类型定义

### 4.2 API Service

- [ ] `services/consultationService.ts` — 重写
  - 新端点、新请求/响应格式

### 4.3 SSE 处理

- [ ] `hooks/useSSEProcessor.ts` — 新增
  - SSE 行解析 + 事件分发
- [ ] `hooks/useChatSSE.ts` — 重写
  - 适配新事件格式

### 4.4 Chat Runtime

- [ ] `hooks/useAssistantChatRuntime.ts` — 重写
  - 适配新 SSE 协议
  - 临时 ID → 正式 ID 替换
  - conversation.created 路由替换

### 4.5 页面和组件

- [ ] `pages/ConsultationPage.tsx` — 改造
  - 懒创建流程（/consultation/new）
  - 新的数据结构
- [ ] `components/SessionSidebar.tsx` — 改造
  - 新的会话列表 API
- [ ] `components/AssistantChatPanel.tsx` — 适配
  - 新的消息格式（parts）
  - 失败消息 + 重试按钮
- [ ] 删除废弃组件：ChatInput.tsx, ChatMessage.tsx

**验证点：**
- 新建咨询 → 发送消息 → 看到流式回复 → URL 自动变为 /consultation/:id
- 侧边栏显示会话列表
- 打开已有会话 → 加载历史消息 → 继续对话
- 断网/刷新 → 消息不丢失
- 重复点击发送 → 不产生重复消息

---

## 阶段 5：集成测试与打磨

### 5.1 端到端测试

- [ ] 新会话完整流程（创建 → 对话 → 诊断 → 治疗）
- [ ] 已有会话继续对话
- [ ] 幂等性测试（重复请求）
- [ ] 错误处理（模型超时、网络中断）
- [ ] 并发安全（同一会话同时发两条消息）

### 5.2 体验打磨

- [ ] 会话标题异步生成
- [ ] 加载状态（骨架屏、typing indicator）
- [ ] 错误状态 UI（失败消息 + 重试）
- [ ] 侧边栏实时更新

### 5.3 清理

- [ ] 删除旧代码（JSONB messages、is_new 逻辑、CreateSessionWithID）
- [ ] 删除旧数据库表（如有迁移需要）
- [ ] 更新文档

---

## 依赖关系

```
阶段 1 (数据层)
  │
  ├──► 阶段 2 (Go API) ──► 阶段 5 (集成测试)
  │         │
  │         ├──► 阶段 3 (Python AI) ──┘
  │         │
  │         └──► 阶段 4 (前端) ──┘
  │
  └──► 阶段 3 可并行开始（不依赖 Go API）
```

**关键路径：** 阶段 1 → 阶段 2 → 阶段 4（前端需要 Go API 就绪才能联调）

**可并行：** 阶段 3（Python 改造）可以在阶段 2 进行中开始，因为 Python 的接口契约已经定义清楚。

---

## 预估工作量

| 阶段 | 预估 | 风险 |
|------|------|------|
| 1. 数据层 | 1-2 天 | 低 — 标准 CRUD |
| 2. Go API | 2-3 天 | 中 — SSE 协议 + 幂等 + 事务 |
| 3. Python AI | 1-2 天 | 中 — Agent 循环改造 |
| 4. 前端 | 2-3 天 | 中 — SSE 协议适配 + assistant-ui 集成 |
| 5. 集成测试 | 1-2 天 | 低 |
| **总计** | **7-12 天** | |
