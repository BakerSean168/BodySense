# 难点：新会话懒创建 + 静默 URL 跳转 + 流式渲染

## 问题背景

咨询工作台的新会话流程有一个核心矛盾：

- **用户期望**：输入内容、点击发送，立刻看到流式回复，URL 同步更新为真实会话地址
- **技术约束**：会话 ID 由后端数据库生成（UUID），前端必须先拿到真实 ID 才能发送消息、更新路由
- **难点**：如果在 URL 变更时重新挂载聊天组件（React 的 `key` 机制），正在进行的 SSE 流式传输会被中断

本文档详细拆解这个流程的实现方案。

---

## 整体时序

```
用户输入 → 点击发送
    │
    ▼
[1] adapter.run() 检测 conversationId === 'new'
    │
    ▼
[2] POST /api/v1/consultations ──→ 后端持久化 Conversation + ConsultationSession
    │                                    │
    │                                    ▼
    │                              生成 UUID，写入数据库
    │                                    │
    │                                    ▼
    │                              返回 201 { conversation_id: "<UUID>", ... }
    │
    ▼
[3] 前端拿到 session.conversation_id
    │
    ├─→ [3a] onConversationCreated(UUID) → 静默 navigate + 防重挂载守卫
    │
    ▼
[4] POST /api/v1/consultations/<UUID>/messages ──→ SSE 流式传输开始
    │
    ▼
[5] 前端逐帧渲染流式回复
```

---

## 第 1 步：adapter.run() 检测新会话

**文件**：`apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`

`useAssistantChatRuntime` 创建一个 `ChatModelAdapter`，其 `run()` 方法是 assistant-ui 框架在用户发送消息时调用的入口：

```ts
const adapter: ChatModelAdapter = {
  async *run({ messages }): AsyncGenerator<ChatModelRunResult> {
    // 提取用户输入的文本内容
    const lastMessage = messages[messages.length - 1];
    const content = lastMessage.content
      .filter((p): p is { type: 'text'; text: string } => p.type === 'text')
      .map((p) => p.text)
      .join('');

    // 生成客户端临时 ID（用于乐观 UI 和幂等性）
    const clientMessageId = `tmp_${crypto.randomUUID()}`;
    const requestId = crypto.randomUUID();
    setIsStreaming(true);

    let activeId = conversationId;

    // ★ 关键判断：如果是新会话，先创建
    if (conversationId === 'new') {
      try {
        const session = await consultationApi.createConsultation();
        activeId = session.conversation_id;  // 拿到后端生成的真实 UUID
        optionsRef.current.onConversationCreated?.(activeId);  // 触发导航回调
      } catch (err) {
        setIsStreaming(false);
        throw err;
      }
    }

    // 用真实 ID 发送消息，开始 SSE 流
    yield* streamConsultationRun(() =>
      consultationApi.sendConsultationMessage(activeId, {
        clientMessageId,
        requestId,
        message: { role: 'user', parts: [{ type: 'text', text: content }] },
      }),
    );
  },
};
```

**要点**：
- `conversationId === 'new'` 是前端占位符，表示"尚未有真实 ID"
- `createConsultation()` 是一个 `await` 调用，阻塞直到后端返回
- 拿到 `session.conversation_id` 后，先触发 `onConversationCreated` 回调，再发送消息
- 整个流程在同一个 `async *run()` 生成器内顺序执行

---

## 第 2 步：后端持久化并返回真实 ID

**前端调用**：

```ts
// apps/web/src/features/consultation/services/consultationService.ts
async createConsultation(): Promise<ConsultationSession> {
  return authFetch(`${API_BASE}/consultations`, {
    method: 'POST',
  }).then((res) => parseJson<ConsultationSession>(res));
}
```

- HTTP `POST /api/v1/consultations`，无请求体
- 返回类型为 `ConsultationSession`，关键字段是 `conversation_id`

**后端处理**（Go）：

```go
// apps/api/internal/service/consultation_service.go
func (s *ConsultationService) CreateSession(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error) {
    // 1. 先检查是否有可复用的空会话（last_message_at IS NULL）
    existingConv, err := s.conversationRepo.GetLastEmptyConversation(ctx, userID)
    if existingConv != nil {
        // 复用已有会话，避免频繁创建空记录
        session, _ := s.consultationRepo.GetByConversationID(ctx, existingConv.ID)
        return session, nil
    }

    // 2. 没有可复用的，创建全新记录
    conversation := &model.Conversation{
        ID:     uuid.New(),   // ★ 后端生成 UUID
        UserID: userID,
        Status: "active",
    }
    s.conversationRepo.Create(ctx, conversation)  // 写入 conversations 表

    session := &model.ConsultationSession{
        ConversationID: conversation.ID,  // 1:1 关系，共享同一个 UUID
        Phase:          "collecting",
        ExtractedInfo:  "[]",
    }
    s.consultationRepo.Create(ctx, session)  // 写入 consultation_sessions 表

    return session, nil  // 返回完整 session，包含 conversation_id
}
```

**数据模型**：

```go
// Conversation 和 ConsultationSession 是 1:1 关系
// ConsultationSession.ConversationID 既是主键也是外键
type Conversation struct {
    ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    UserID uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
    Status string    `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
    // ...
}

type ConsultationSession struct {
    ConversationID uuid.UUID      `gorm:"type:uuid;primaryKey" json:"conversation_id"`
    Phase          string         `gorm:"type:varchar(30);not null;default:'collecting'" json:"phase"`
    ExtractedInfo  datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"extracted_info"`
    // ...
}
```

**返回的 JSON**（HTTP 201）：

```json
{
  "conversation_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "phase": "collecting",
  "extracted_info": [],
  "diagnosis": null,
  "treatment_plan": null,
  "created_at": "2026-07-03T10:00:00Z",
  "updated_at": "2026-07-03T10:00:00Z"
}
```

**设计决策：空会话复用**

后端有一个 `GetLastEmptyConversation` 查询，会查找该用户最近创建的、尚未发送过消息的会话（`last_message_at IS NULL`）。如果存在，直接复用而不是创建新记录。这避免了用户反复点击"新建咨询"时产生大量空记录。

---

## 第 3 步：前端拿到 ID → 静默 URL 跳转

### 3a. onConversationCreated 回调

**文件**：`apps/web/src/features/consultation/pages/ConsultationPage.tsx`

```ts
<AssistantChatPanel
  key={chatSessionKey}
  conversationId={id || 'new'}
  onConversationCreated={async (newId) => {
    // ★ 防重挂载守卫：记录刚创建的 ID
    justCreatedRef.current = newId;

    // 刷新侧边栏会话列表
    queryClient.invalidateQueries({
      queryKey: consultationKeys.conversations(),
    });

    // 静默跳转：replace: true 不会产生新的浏览器历史记录
    navigate(`/consultation/${newId}`, { replace: true });
  }}
  // ...
/>
```

### 3b. 防重挂载守卫机制

这是整个方案中最关键的技巧。问题在于：

1. `AssistantChatPanel` 使用 `key={chatSessionKey}` 渲染
2. 当 URL 从 `/consultation` 变为 `/consultation/<UUID>` 时，`useParams` 的 `id` 会变化
3. 如果 `chatSessionKey` 随之变化，React 会卸载旧组件、挂载新组件
4. **正在进行的 SSE 流式传输会因为组件卸载而中断**

解决方案是用 `justCreatedRef` 做一个"刚刚创建"的标记：

```ts
const justCreatedRef = useRef<string | null>(null);

// URL 变化时的同步逻辑
useEffect(() => {
  if (!id || id === 'new') {
    setChatSessionKey('new');
    return;
  }

  // ★ 关键：如果是刚刚创建的会话，跳过 key 更新
  if (justCreatedRef.current === id) {
    justCreatedRef.current = null;  // 清除标记，只生效一次
    return;                          // 不更新 chatSessionKey，组件不重挂载
  }

  // 正常情况：切换到已有会话
  setChatSessionKey(`conversation:${id}`);
}, [id]);
```

**执行流程**：

```
1. 用户在 /consultation 页面发送消息
2. adapter.run() 调用 createConsultation()，拿到 UUID "abc-123"
3. onConversationCreated("abc-123") 被调用：
   - justCreatedRef.current = "abc-123"
   - navigate("/consultation/abc-123", { replace: true })
4. URL 变化触发 useEffect：
   - id 变为 "abc-123"
   - 检查 justCreatedRef.current === "abc-123" → true
   - 清除 ref，return（不更新 chatSessionKey）
   - ★ 组件不重挂载，SSE 流继续
5. adapter.run() 继续执行 sendConsultationMessage("abc-123", ...)
6. 流式回复正常渲染
```

**为什么用 `useRef` 而不是 `useState`**：

- `useRef` 的更新是同步的、不触发 re-render
- `navigate()` 和 `useEffect` 的触发时序是：navigate → React 路由更新 → 组件 re-render → useEffect 执行
- 需要在 useEffect 执行时能读到标记，`useRef` 保证了这一点
- 如果用 `useState`，设置值会触发额外的 re-render，时序不可控

---

## 第 4 步：SSE 流式消息发送

拿到真实 UUID 后，`adapter.run()` 立即调用：

```ts
yield* streamConsultationRun(() =>
  consultationApi.sendConsultationMessage(activeId, {
    clientMessageId,
    requestId,
    message: { role: 'user', parts: [{ type: 'text', text: content }] },
  }),
);
```

`sendConsultationMessage` 返回一个 `Response` 对象（fetch 的原始响应，尚未消费 body），传给 `streamConsultationRun()` 进行 SSE 流式解析。

---

## 第 5 步：流式渲染

**SSE 事件处理链**：

```
Response body (SSE text stream)
    │
    ▼
consumeSSEStream() ── 逐行解析 event:/data: 帧
    │
    ▼
reduceActiveTurnEvent() ── 纯 reducer，更新 ActiveTurnState
    │
    ▼
onActiveTurnUpdate() ── 推送到 ActiveTurnContext
    │
    ▼
StreamingAssistantTurn ── 从 Context 读取，渲染 UI
```

**ActiveTurnState 核心字段**：

```ts
interface ActiveTurnState {
  status: 'idle' | 'streaming' | 'interrupted' | 'completed' | 'failed';
  text: string;                          // 累积的 Markdown 文本
  toolCallsById: Record<string, ToolCallInfo>;  // 工具调用
  citationsByKey: Record<string, Citation>;     // 引用来源
  redFlag: RedFlagEvent | null;          // 红旗警告
  pendingInteraction: Interaction | null; // 中断等待用户输入
  finalParts: ThreadAssistantMessagePart[]; // 完成后的最终消息 parts
}
```

**渲染时机**：

- `status === 'streaming'` 且有 `text` → 实时渲染 Markdown（ReactMarkdown + remarkGfm）
- `status === 'streaming'` 且无 `text' → 显示加载动画（跳动的点）
- `status === 'completed'` → 构建 `finalParts`，交给 assistant-ui 的 `MessagePrimitive.Content` 渲染

---

## 关键设计决策总结

| 决策 | 原因 |
|------|------|
| 懒创建（发送时才创建会话） | 避免用户打开页面不输入就产生空记录 |
| 空会话复用 | 减少数据库垃圾记录 |
| `replace: true` 导航 | 不产生 `/consultation` → `/consultation/uuid` 的浏览器历史记录，用户点返回不会回到空页面 |
| `justCreatedRef` 防重挂载 | 保护正在进行的 SSE 流不被 React key 变化中断 |
| `useRef` 而非 `useState` | 同步更新、不触发 re-render、时序可控 |
| 客户端 `clientMessageId` + `requestId` | 乐观 UI 渲染 + 服务端幂等性保证 |
| `last_message_at IS NOT NULL` 过滤 | 侧边栏只显示有消息的会话，空会话不出现在列表中 |
| 标题异步生成（goroutine） | 不阻塞主消息流的返回，用户先看到回复再看到标题更新 |
| 标题前端实时更新链路已预留 | SSE 事件类型、adapter handler、缓存更新逻辑都已实现，后端只需发射 `title.generated` 事件即可激活 |

---

## 边界情况

### 网络失败

```ts
if (conversationId === 'new') {
  try {
    const session = await consultationApi.createConsultation();
    // ...
  } catch (err) {
    setIsStreaming(false);  // 重置流式状态
    throw err;              // 向 assistant-ui 框架抛出错误，UI 显示错误状态
  }
}
```

如果 `createConsultation()` 失败（网络错误、服务端 500），流式状态被重置，错误向上抛出，assistant-ui 框架会处理错误显示。

### 并发发送

assistant-ui 框架在 `run()` 执行期间会禁用发送按钮，用户无法并发发送。`requestId`（UUID）提供了额外的幂等性保证——即使请求因重试等原因到达后端多次，也只处理一次。

### 页面刷新

用户刷新页面时，URL 已经是 `/consultation/<UUID>`，`useConversationQuery` 会加载历史消息，`buildInterruptedTurnSeed()` 会恢复中断的流式状态。不会触发懒创建逻辑。

---

## 第 6 步：侧边栏会话列表刷新

新会话创建并完成首条消息后，侧边栏需要显示这条新会话。这个过程涉及三个问题：列表何时刷新、新会话如何出现在列表中、初始标题是什么。

### 6a. 列表刷新触发时机

在 `onConversationCreated` 回调中，通过 TanStack Query 的 `invalidateQueries` 触发刷新：

```ts
// ConsultationPage.tsx
onConversationCreated={async (newId) => {
  justCreatedRef.current = newId;

  // ★ 使会话列表缓存失效，触发重新请求
  queryClient.invalidateQueries({
    queryKey: consultationKeys.conversations(),
  });

  navigate(`/consultation/${newId}`, { replace: true });
}}
```

Query key 结构（`consultationQueryKeys.ts`）：

```ts
export const consultationKeys = {
  all: ['consultation'] as const,
  conversations: () => [...consultationKeys.all, 'conversations'] as const,
  // key = ['consultation', 'conversations']
};
```

`invalidateQueries` 不会立即发请求，而是在组件下次 mount 或 refetch 时重新执行 `queryFn`。由于侧边栏已经在页面上（`useConversationsQuery` 已订阅这个 key），TanStack Query 会立即在后台 refetch。

### 6b. 列表查询的 API 和过滤条件

```ts
// useConversationsQuery.ts
export function useConversationsQuery() {
  return useQuery({
    queryKey: consultationKeys.conversations(),
    queryFn: () => consultationApi.listConversations({ limit: 50 }),
    select: (data) => data.conversations,
  });
}
```

调用 `GET /api/v1/conversations?limit=50`。

后端查询条件（`conversation_repository.go`）：

```go
query := r.db.WithContext(ctx).
    Where("user_id = ? AND deleted_at IS NULL AND last_message_at IS NOT NULL", userID)
// ...
err := query.Order("last_message_at DESC").Limit(limit + 1).Find(&conversations).Error
```

**关键过滤**：`last_message_at IS NOT NULL` — 只返回有过消息的会话。新创建的会话在 `CreateSession` 时 `last_message_at` 为 null，不会出现在列表中。

**什么时候变为 non-null**？在 `Runtime.SendUserMessage()` 的 `persistCompletedTurn` 中：

```go
// runtime.go — stream 结束后持久化
r.conversationService.UpdateLastMessageAt(ctx, conversationID, uid)
```

这发生在 `stream.done` 发送**之前**。所以当 `onConversationCreated` 在前端触发时（收到 `stream.done` 之后），数据库中的 `last_message_at` 已经被设置，refetch 查询会包含这条新会话。

**排序**：`last_message_at DESC` — 最近活跃的排最上面。新会话因为刚发过消息，会出现在列表顶部。

### 6c. 侧边栏渲染

```tsx
// ConsultationPage.tsx
const { data: conversations = [] } = useConversationsQuery();

// 桌面端侧边栏
<SessionHistorySidebar conversations={conversations} ... />
```

```tsx
// SessionHistorySidebar.tsx — 分组渲染
const pinnedConversations = conversations.filter(c => c.pinned);
const unpinnedConversations = conversations.filter(c => !c.pinned);
// 先渲染置顶，再渲染普通列表，每个会话是一个 <SessionCard>
```

---

## 第 7 步：新会话的初始标题

### 7a. 创建时的标题状态

后端创建 `Conversation` 时：

```go
conversation := &model.Conversation{
    ID:          uuid.New(),
    UserID:      userID,
    Status:      "active",
    TitleStatus: "pending",  // 标题状态：待生成
    // Title 字段未设置，零值为 ""
}
```

前端 `SessionCard` 的显示逻辑：

```tsx
// SessionCard.tsx
const displayTitle = conversation.title || '新咨询';
```

**所以新会话在侧边栏中显示为"新咨询"**，直到标题生成完成。

### 7b. 标题生成的触发

`Runtime.SendUserMessage()` 在 stream 完全结束后，异步触发标题生成：

```go
// runtime.go — 第 196 行：stream.done 已发送
// 第 198 行：
r.maybeGenerateTitle(ctx, conversationID, uid)
```

```go
func (r *Runtime) maybeGenerateTitle(ctx context.Context, conversationID, userID uuid.UUID) {
    conv, _ := r.conversationService.GetConversationByID(ctx, conversationID, userID)
    // 仅在 title_status == "pending" 且 title == "" 时触发（即首条消息）
    if conv.TitleStatus == "pending" && conv.Title == "" {
        r.conversationService.GenerateTitle(ctx, conversationID, userID)
    }
}
```

### 7c. 标题生成的执行流程

```go
// conversation_service.go
func (s *ConversationService) GenerateTitle(ctx context.Context, id, userID uuid.UUID) error {
    // 1. 更新状态为 "generating"
    s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "generating")

    // 2. 启动异步 goroutine
    go s.generateTitleAsync(id, userID)
    return nil
}
```

`generateTitleAsync` 在独立 goroutine 中执行（30 秒超时）：

```go
func (s *ConversationService) generateTitleAsync(id, userID uuid.UUID) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // 1. 加载会话的所有消息
    messages, _ := s.messageRepo.ListByConversationID(ctx, id)

    // 2. 调用 AI 服务生成标题
    title, _ := s.aiClient.GenerateTitle(ctx, msgPayload)
    // POST {ai-service}/api/title/generate
    // 请求体：{ messages: [{ role, parts }] }

    if title == "" {
        title = "新对话"  // 兜底标题
    }

    // 3. 写入数据库
    s.conversationRepo.UpdateTitle(ctx, id, userID, title)
    s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "generated")
}
```

### 7d. ⚠️ 当前实现的缺陷：标题不会实时更新

**后端不会发射 `title.generated` SSE 事件。**

`generateTitleAsync` 运行在一个独立的 goroutine 中，使用 `context.Background()` 上下文。此时 SSE 流已经关闭（`stream.done` 在 `maybeGenerateTitle` 之前发送），没有任何机制将标题推送给前端。

前端虽然有完整的 `title.generated` 事件处理链路（SSE 解析器 → adapter → panel → page），但这条链路是**死代码**——后端从未发射过这个事件。

**当前的标题更新方式**：用户刷新页面，或切换到其他会话再切回来，TanStack Query 重新请求会话列表，标题从数据库读取后显示。

**前端已有的（未激活的）实时更新逻辑**：

```ts
// ConsultationPage.tsx — handleTitleGenerated 回调
const handleTitleGenerated = useCallback((title: string) => {
  // 更新单个会话详情缓存
  queryClient.setQueryData(
    consultationKeys.conversation(routeConversationId),
    (old) => old ? { ...old, conversation: { ...old.conversation, title } } : old,
  );
  // 更新会话列表缓存（侧边栏）
  queryClient.setQueryData(
    consultationKeys.conversations(),
    (old) => old ? {
      ...old,
      conversations: old.conversations.map((c) =>
        c.id === routeConversationId ? { ...c, title } : c,
      ),
    } : old,
  );
}, [routeConversationId, queryClient]);
```

这段代码通过 `queryClient.setQueryData` 直接更新 TanStack Query 缓存，无需重新请求 API。如果后端发射了 `title.generated` SSE 事件，前端会立即更新侧边栏标题，无需刷新页面。

---

## 完整时序图（含列表和标题）

```
用户输入 → 点击发送
    │
    ▼
[1] POST /api/v1/consultations ──→ 后端创建 Conversation (title="", title_status="pending")
    │                              + ConsultationSession (phase="collecting")
    ▼
[2] 拿到 conversation_id ──→ onConversationCreated()
    │                           ├─ justCreatedRef = id（防重挂载）
    │                           ├─ invalidateQueries（刷新侧边栏）
    │                           └─ navigate（静默跳转）
    ▼
[3] POST /api/v1/consultations/<id>/messages ──→ SSE 流开始
    │
    ▼
[4] 后端 LangGraph 执行：safety → classify → generate_response → decide_phase
    │
    ├─ message.text.delta ──→ 前端实时渲染文本
    ├─ tool.call/result    ──→ 前端显示工具调用
    ├─ extracted_info      ──→ 右侧面板更新
    │
    ▼
[5] 后端 persistCompletedTurn()
    ├─ 写入助手消息到数据库
    ├─ UpdateLastMessageAt()  ← ★ 此时 last_message_at 变为 non-null
    │
    ▼
[6] 后端发送 stream.done ──→ 前端收到
    │
    ├─ onConversationCreated 触发的 invalidateQueries 已在后台 refetch
    │   └─ GET /api/v1/conversations?limit=50
    │       └─ 新会话已满足 last_message_at IS NOT NULL 条件
    │           └─ 侧边栏显示新会话，标题为"新咨询"
    │
    ▼
[7] 后端 maybeGenerateTitle()（异步 goroutine）
    ├─ title_status → "generating"
    ├─ 调用 AI 服务 POST /api/title/generate
    ├─ 写入 title 到数据库
    └─ title_status → "generated"
    │
    ▼
[8] ⚠️ 标题已写入数据库，但前端不会实时感知
    └─ 用户刷新页面或切换会话后再切回，才能看到新标题
```
