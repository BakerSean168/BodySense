# 难点：统一 `POST /consultation-runs` 端到端流程

## 问题背景

在 v1 版本中，前端发一条消息需要**两步 HTTP 请求**：

1. `POST /api/v1/consultations` — 创建会话，拿回 `conversation_id`
2. `POST /api/v1/consultations/<id>/messages` — 发送消息，触发 SSE 流

这导致了几个问题：
- 两步之间存在时间窗口：用户看到的 URL 会短暂跳动（从 `/consultation` 到 `/consultation/<uuid>`）
- 如果第二步失败，会留下一个空会话记录
- 前端需要维护 `justCreatedRef` 防重挂载守卫，时序脆弱

v2 将两步合并为一个统一端点：**`POST /api/v1/consultation-runs`**。

---

## 整体时序

```
用户输入 → 点击发送
    │
    ▼
[1] adapter.run() 构造请求体
    │   conversationId: null（新会话）或 "<UUID>"（已有会话）
    │
    ▼
[2] POST /api/v1/consultation-runs ──→ 后端处理
    │       ├── 2a. 幂等性检查（request_id）
    │       ├── 2b. 创建或复用会话（conversationId === nil 时）
    │       ├── 2c. 创建 turn envelope（run + user msg + assistant msg）
    │       ├── 2d. 发射 conversation.created SSE 事件（新会话时）
    │       ├── 2e. 发射 message.persisted + message.created
    │       ├── 2f. 调用 AI 服务，流式转发事件
    │       ├── 2g. 持久化完成的 turn
    │       ├── 2h. 生成标题（新会话时，阻塞直到完成）
    │       └── 2i. 发射 stream.done
    │
    ▼
[3] 前端逐帧渲染流式回复
    │
    ▼
[4] 收到 stream.done → 刷新缓存 → UI 更新
```

---

## 第 1 步：前端构造请求

**文件**：`apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`

`useAssistantChatRuntime` 创建一个 `ChatModelAdapter`，其 `run()` 方法是 assistant-ui 框架在用户发送消息时调用的入口：

```ts
const adapter: ChatModelAdapter = {
  async *run({ messages }): AsyncGenerator<ChatModelRunResult> {
    const lastMessage = messages[messages.length - 1];
    const content = lastMessage.content
      .filter((p): p is { type: 'text'; text: string } => p.type === 'text')
      .map((p) => p.text)
      .join('');

    setIsStreaming(true);

    yield* streamConsultationRun(() =>
      consultationApi.startConsultationRun({
        // ★ 关键：新会话传 null，已有会话传 UUID
        conversationId: conversationId === 'new' ? null : conversationId,
        clientMessageId: `tmp_${crypto.randomUUID()}`,
        requestId: crypto.randomUUID(),
        message: {
          role: 'user',
          parts: [{ type: 'text', text: content }],
        },
      }),
    );
  },
};
```

**要点**：
- `conversationId` 来自 `useParams()`，新会话时为 `'new'` 字符串
- `clientMessageId` 是客户端临时 ID，用于乐观 UI 更新
- `requestId` 是 UUID，用于后端幂等性检查
- **不需要先调用 `createConsultation()`**，后端会在一个请求内完成会话创建 + 消息发送

**对比 v1 的变化**：

```diff
- // v1: 两步
- if (conversationId === 'new') {
-   const session = await consultationApi.createConsultation();
-   activeId = session.conversation_id;
-   optionsRef.current.onConversationCreated?.(activeId);
- }
- yield* streamConsultationRun(() =>
-   consultationApi.sendConsultationMessage(activeId, { ... }),
- );

+ // v2: 一步
+ yield* streamConsultationRun(() =>
+   consultationApi.startConsultationRun({
+     conversationId: conversationId === 'new' ? null : conversationId,
+     ...
+   }),
+ );
```

---

## 第 2 步：后端统一处理

**文件**：`apps/api/internal/consultation/runtime.go` → `StartRun()`

### 2a. 幂等性检查

```go
existing, found, err := r.runService.CheckIdempotency(ctx, uid, req.RequestID)
if found {
    if existing.Status == "running" || existing.Status == "waiting_user" {
        return httpErr(http.StatusConflict, "RUN_IN_PROGRESS", "...")
    }
    r.replayCompletedRun(ctx, w, existing)  // 重放已完成的 run
    return nil
}
```

`requestId`（UUID）保证了即使网络重试导致同一请求到达后端多次，也只处理一次。

### 2b. 创建或复用会话

```go
var conversationID uuid.UUID
var isNewConversation bool

if req.ConversationID == nil || *req.ConversationID == "" {
    // 新会话：后端创建 Conversation + ConsultationSession
    session, err := r.consultationService.CreateSession(ctx, uid)
    conversationID = session.ConversationID
    isNewConversation = true
} else {
    // 已有会话：解析 UUID
    conversationID, _ = uuid.Parse(*req.ConversationID)
}
```

**数据模型**：

```
┌─────────────────────────┐       ┌──────────────────────────────┐
│     conversations       │       │    consultation_sessions      │
├─────────────────────────┤       ├──────────────────────────────┤
│ id          UUID (PK)   │──1:1──│ conversation_id UUID (PK/FK) │
│ user_id     UUID        │       │ phase         VARCHAR(30)    │
│ title       VARCHAR     │       │ extracted_info JSONB          │
│ title_status VARCHAR    │       │ diagnosis     JSONB           │
│ status      VARCHAR     │       │ treatment_plan JSONB          │
│ last_message_at TIMESTMP│       │ pending_interactions JSONB    │
│ active_run_id UUID      │       └──────────────────────────────┘
│ created_at  TIMESTMP    │
│ updated_at  TIMESTMP    │
└─────────────────────────┘
```

### 2c. 创建 Turn Envelope

`createTurnEnvelope()` 是一个原子操作，一次性创建本次对话轮次所需的所有数据库记录：

```go
func (r *Runtime) createTurnEnvelope(...) (turnID, run, userMsg, assistantMsg, baseIDs, *HTTPError) {
    // 1. 获取消息序列号
    userSeq, _ := r.messageService.GetNextSeq(ctx, conversationID)
    assistantSeq := userSeq + 1
    turnID := uuid.New()

    // 2. 创建 Run（带数据库级幂等性）
    run, existed, _ := r.runService.CreateRunWithIdempotency(
        ctx, conversationID, turnID, requestID, uid, "consultation-thread",
    )
    if existed {
        return ..., httpErr(http.StatusConflict, "RUN_IN_PROGRESS", "...")
    }

    // 3. 创建 User Message（status: "completed"）
    userMsg, _ = r.messageService.CreateMessage(
        ctx, conversationID, turnID, "user", userParts, userSeq, "completed", ...,
    )

    // 4. 创建 Assistant Message（status: "streaming"，parts 为空数组）
    assistantMsg, _ = r.messageService.CreateMessage(
        ctx, conversationID, turnID, "assistant", "[]", assistantSeq, "streaming", ...,
    )

    // 5. 设置 active_run_id
    r.conversationService.UpdateActiveRunID(ctx, conversationID, uid, &run.ID, ...)

    return turnID, run, userMsg, assistantMsg, baseIDs, nil
}
```

**数据库级幂等性**（`CreateRunWithIdempotency`）：

```go
// 使用 GORM 的 clause.OnConflict 实现原子 INSERT ... ON CONFLICT DO NOTHING
result := tx.Clauses(clause.OnConflict{
    Columns:   []clause.Column{{Name: "user_id"}, {Name: "request_id"}},
    DoNothing: true,
}).Create(run)

if result.RowsAffected > 0 {
    return nil  // 新插入成功
}
// 已存在，查询并返回现有记录
```

这比应用层的"先查后插"模式更安全——在并发场景下不会出现竞态条件。

### 2d. 发射 `conversation.created`（新会话时）

```go
if isNewConversation {
    _ = sw.SendNew(ctx, "conversation", "conversation.created",
        dto.StreamEventIDs{ConversationID: conversationID.String(), ...},
        map[string]any{
            "id":             conversationID.String(),
            "title":          "",
            "title_status":   "pending",
            "status":         "active",
            "last_message_at": userMsg.CreatedAt.UTC().Format(time.RFC3339),
            "created_at":     userMsg.CreatedAt.UTC().Format(time.RFC3339),
        },
        "",
    )
}
```

### 2e. 发射 `message.persisted` + `message.created`

```go
r.emitUserTurnStarted(ctx, sw, req.ClientMessageID, userMsg, assistantMsg)
```

发射两个事件：
- `message.persisted` — 用户消息已入库，payload 包含 `client_message_id`（用于前端替换临时 ID）
- `message.created` — 助手消息已创建，status 为 `"streaming"`

### 2f. 调用 AI 服务并流式转发

```go
result, stopped := r.executeRunFlow(ctx, sw, uid, conversationID, turn, run, assistantMsg, baseIDs, userText, session)
```

`executeRunFlow()` 内部：

1. 加载用户 Profile JSON
2. 调用 `r.aiClient.StartConsultationTurn()` → 返回 `<-chan dto.StreamEvent`
3. 进入事件循环 `streamAIEvents()`，逐个处理事件：

```go
for {
    select {
    case <-state.RequestDone:  // 客户端断开连接
        // 标记 run 为 failed，清理 active_run_id
        return result, true

    case event, ok := <-events:
        if !ok { return result, false }  // AI 服务关闭了 channel

        switch event.Type {
        case "message.text.delta":    // 文本增量 → 转发给前端
        case "tool.call":             // 工具调用 → 记录 + 转发
        case "tool.result":           // 工具结果 → 记录 + 转发
        case "state.extracted_info.upsert":  // 提取信息更新 → 转发
        case "state.phase.changed":   // 阶段变更 → 更新数据库 + 转发
        case "state.interaction.required":   // 需要用户输入 → 中断
        case "usage.reported":        // 用量统计 → 记录
        case "stream.done":           // AI 完成 → 记录
        case "stream.error":          // AI 错误 → 转发
        }
    }
}
```

### 2g. 持久化完成的 Turn

```go
r.finishTurn(ctx, sw, uid, conversationID, run, assistantMsg, turn, result, baseIDs)
```

`finishTurn()` 内部：

```go
func (r *Runtime) finishTurn(...) {
    // 1. 更新 assistant message：parts = 累积的所有 parts，status = "completed"
    r.persistCompletedTurn(ctx, uid, conversationID, run, assistantMsg, result)

    // 2. 发射 message.completed SSE 事件
    _ = sw.SendNew(ctx, "message", "message.completed", ...)
}
```

`persistCompletedTurn()` 执行三件事：

```go
// 1. 更新消息的 parts 和 status
r.messageService.UpdateMessageCompleted(ctx, assistantMsg.ID, conversationID, finalPartsJSON, nil, nil)

// 2. 更新会话的 last_message_at（使会话出现在侧边栏列表中）
r.conversationService.UpdateLastMessageAt(ctx, conversationID, uid)

// 3. 标记 run 为 completed
r.runService.CompleteRun(ctx, run.ID, uid, result.Usage, result.ProviderResponseID)
```

### 2h. 生成标题（新会话时，阻塞直到完成）

```go
if isNewConversation {
    r.generateTitleAndNotify(ctx, conversationID, uid, sw, baseIDs)
}
```

**与 v1 的关键区别**：v2 中标题生成是**同步阻塞**的，在 `stream.done` 之前完成。这意味着：
- 前端收到 `stream.done` 时，标题已经生成完毕
- `title.generated` SSE 事件在 `stream.done` 之前发射
- 前端可以立即更新侧边栏标题，无需等待刷新

```go
func (r *Runtime) generateTitleAndNotify(...) {
    title, err := r.conversationService.GenerateTitleSync(ctx, conversationID, userID)
    if err != nil {
        log.Printf("title generation failed: %v", err)
        return
    }
    _ = sw.SendNew(ctx, "title", "title.generated", baseIDs,
        map[string]any{"title": title}, "")
}
```

### 2i. 发射 `stream.done`

```go
_ = sw.SendNew(ctx, "stream", "stream.done", baseIDs, map[string]any{}, "")
```

---

## 第 3 步：前端 SSE 事件处理链

**文件**：`apps/web/src/features/consultation/hooks/useSSEProcessor.ts` → `consumeSSEStream()`

```
Response body (SSE text stream)
    │
    ▼
consumeSSEStream() ── 逐行解析 event:/data: 帧
    │
    ▼
dispatch(event) ── 调用 reduceActiveTurnEvent()
    │
    ▼
reduceActiveTurnEvent() ── 纯 reducer，返回 { state, effects }
    │
    ▼
applyEffects(effects) ── 触发回调（onConversationCreated, onPhaseChange 等）
    │
    ▼
enqueueResult(snapshot) ── 推入队列，等待 yield 给 assistant-ui
    │
    ▼
assistant-ui runtime ── 渲染 UI
```

### ActiveTurnState 核心字段

```ts
interface ActiveTurnState {
  conversationId: string | null;    // 从 conversation.created 事件获取
  runId: string | null;             // 从 conversation.created 事件获取
  status: 'idle' | 'streaming' | 'interrupted' | 'completed' | 'failed';
  text: string;                     // 累积的 Markdown 文本
  toolCallsById: Record<string, ToolCallInfo>;
  citationsByKey: Record<string, Citation>;
  redFlag: RedFlagEvent | null;
  pendingInteraction: PendingInteraction | null;
  finalParts: ThreadAssistantMessagePart[];  // 完成后的最终消息 parts
}
```

### 关键事件处理

| SSE 事件 | Reducer 行为 | 副作用 |
|----------|-------------|--------|
| `conversation.created` | 设置 `conversationId`、`runId`，status → `streaming` | `onConversationCreated` → 静默跳转 + 侧边栏乐观插入 |
| `message.persisted` | — | `onMessagePersisted` → 替换临时 ID |
| `message.created` | — | — |
| `message.text.delta` | 累积 `text` | `enqueueResult` → 实时渲染 |
| `tool.call` | 记录到 `toolCallsById` | — |
| `tool.result` | 更新 `toolCallsById` | — |
| `state.extracted_info.upsert` | — | `onExtractedInfoUpdate` → 右侧面板更新 |
| `state.phase.changed` | — | `onPhaseChange` → 阶段标签更新 |
| `state.interaction.required` | 设置 `pendingInteraction`，status → `interrupted` | `onInteractionRequired` → 显示交互表单 |
| `message.completed` | 构建 `finalParts`，status → `completed` | `enqueueResult` → 最终渲染 |
| `message.failed` | status → `failed` | — |
| `title.generated` | — | `onTitleGenerated` → 更新侧边栏标题 |
| `stream.done` | — | — |

---

## 第 4 步：静默 URL 跳转

### 4a. `onConversationCreated` 回调

**文件**：`apps/web/src/features/consultation/pages/ConsultationPage.tsx`

```tsx
<AssistantChatPanel
  key={chatSessionKey}
  conversationId={id || 'new'}
  onConversationCreated={(newId) => {
    // ★ 同步设置 ref — SSE 回调在同一事件批次中需要这个 ID
    justCreatedRef.current = newId;
    activeConversationIdRef.current = newId;

    // 乐观插入：立即在侧边栏显示新会话
    queryClient.setQueryData<ConversationListResponse>(
      consultationKeys.conversations(),
      (old) => {
        if (!old) return old;
        const now = new Date().toISOString();
        return {
          ...old,
          conversations: [
            {
              id: newId,
              title: '',
              title_status: 'pending' as const,
              status: 'active' as const,
              // ... 其他字段
            },
            ...old.conversations.filter((c) => c.id !== newId),
          ],
        };
      },
    );

    // 静默跳转：replace: true 不会产生新的浏览器历史记录
    navigate(`/consultation/${newId}`, { replace: true });
  }}
  // ...
/>
```

### 4b. 防重挂载守卫

**问题**：`AssistantChatPanel` 使用 `key={chatSessionKey}` 渲染。当 URL 变化时，如果 `chatSessionKey` 随之变化，React 会卸载旧组件、挂载新组件，**正在进行的 SSE 流会被中断**。

**解决方案**：`justCreatedRef` 做"刚刚创建"标记：

```ts
const justCreatedRef = useRef<string | null>(null);

useEffect(() => {
  if (!id || id === 'new') {
    setChatSessionKey('new');
    setIsPageLoading(false);
    activeConversationIdRef.current = null;
    return;
  }

  // ★ 关键：如果是刚刚创建的会话，跳过 key 更新
  if (justCreatedRef.current === id) {
    justCreatedRef.current = null;  // 清除标记，只生效一次
    return;                          // 不更新 chatSessionKey，组件不重挂载
  }

  // 正常情况：切换到已有会话
  activeConversationIdRef.current = null;
  setChatSessionKey(`conversation:${id}`);
  setIsPageLoading(true);
}, [id]);
```

### 4c. `activeConversationIdRef` 的作用

```ts
const activeConversationIdRef = useRef<string | null>(null);
```

SSE 回调（如 `handleTitleGenerated`、`handleStreamFinished`）在同一个事件批次中触发时，React 还没有 re-render，`routeConversationId`（来自 `useParams()`）仍然是旧值。`activeConversationIdRef` 提供了同步的、最新的会话 ID：

```ts
const handleTitleGenerated = useCallback((title: string) => {
  // 优先使用 ref 中的 ID（同步更新），fallback 到路由 ID
  const convId = activeConversationIdRef.current ?? routeConversationId;
  if (!convId) return;

  queryClient.setQueryData(/* ... */);
}, [routeConversationId, queryClient]);
```

---

## 第 5 步：缓存更新

### 5a. `handleStreamFinished` — 流结束后刷新

```ts
const handleStreamFinished = useCallback(() => {
  const convId = activeConversationIdRef.current ?? routeConversationId;
  if (!convId) return;

  // 刷新 session 缓存（extracted_info, phase, diagnosis 等）
  queryClient.invalidateQueries({
    queryKey: consultationKeys.session(convId),
  });

  // 刷新 conversation 缓存（messages）
  queryClient.invalidateQueries({
    queryKey: consultationKeys.conversation(convId),
  });
}, [routeConversationId, queryClient]);
```

### 5b. `handleTitleGenerated` — 标题实时更新

```ts
const handleTitleGenerated = useCallback((title: string) => {
  const convId = activeConversationIdRef.current ?? routeConversationId;
  if (!convId) return;

  // 更新单个会话详情缓存
  queryClient.setQueryData(
    consultationKeys.conversation(convId),
    (old) => old ? { ...old, conversation: { ...old.conversation, title } } : old,
  );

  // 更新会话列表缓存（侧边栏）
  queryClient.setQueryData<ConversationListResponse>(
    consultationKeys.conversations(),
    (old) => old ? {
      ...old,
      conversations: old.conversations.map((c) =>
        c.id === convId ? { ...c, title } : c,
      ),
    } : old,
  );
}, [routeConversationId, queryClient]);
```

### 5c. `handlePhaseChange` — 带去重的阶段更新

```ts
const handlePhaseChange = useCallback(async (newPhase: ConsultationPhase) => {
  const convId = activeConversationIdRef.current ?? routeConversationId;
  if (!convId) return;

  // ★ 去重：跳过相同阶段的重复事件
  const current = queryClient.getQueryData<ConsultationSession>(
    consultationKeys.session(convId),
  );
  if (current?.phase === newPhase) return;

  await queryClient.cancelQueries({ queryKey: consultationKeys.session(convId) });
  queryClient.setQueryData<ConsultationSession>(
    consultationKeys.session(convId),
    (old) => (old ? { ...old, phase: newPhase } : old),
  );
}, [routeConversationId, queryClient]);
```

---

## 第 6 步：Interaction 中断与恢复

当 AI 需要用户输入时（如选择症状、确认信息），会发射 `state.interaction.required` 事件：

### 6a. 后端处理

```go
func (r *Runtime) handleInteractionRequired(...) bool {
    // 1. 创建 pending interaction 记录
    interaction, _ := r.interactionService.CreatePendingInteraction(
        ctx, run.ID, conversationID, toolCallID, question,
    )

    // 2. 原子更新 assistant message：parts + status="aborted"
    r.messageService.UpdateMessageCompletedWithStatus(
        ctx, assistantMsg.ID, conversationID, finalPartsJSON, "aborted",
    )

    // 3. 发射 stream.done，结束当前 SSE 流
    _ = sw.SendNew(ctx, "stream", "stream.done", ...)

    // 4. 清理 active_run_id
    r.clearActiveRun(ctx, conversationID, uid)
    return true  // 停止事件循环
}
```

### 6b. 前端恢复

用户回答交互问题时，调用 `resumeInteraction()`：

```ts
const resumeInteraction = useCallback(
  (threadRuntime: ThreadRuntime, interactionId: string, answer: unknown) => {
    setIsStreaming(true);
    threadRuntime.resumeRun({
      parentId: threadRuntime.getState().messages.at(-1)?.id ?? null,
      stream: async function* () {
        const runFn = streamConsultationRunRef.current;
        if (!runFn) {
          throw new Error('streamConsultationRun not initialized');
        }
        yield* runFn(
          () => consultationApi.resumeInteractionStream(conversationId, interactionId, {
            requestId: crypto.randomUUID(),
            answer,
          }),
        );
      },
    });
  },
  [conversationId],
);
```

后端 `ResumeInteraction()` 会：
1. 验证 interaction 存在且属于该会话
2. 标记 interaction 为已回答
3. 创建新的 turn envelope（run + messages）
4. 调用 `ResumeConsultationInterrupt` AI 端点
5. 流式转发事件
6. 持久化完成的 turn

---

## SSE 事件流完整序列

```
POST /api/v1/consultation-runs  (HTTP 200, Content-Type: text/event-stream)

event: conversation          ← 仅新会话时
data: {"type":"conversation.created","ids":{"conversation_id":"<UUID>","run_id":"<UUID>","turn_id":"<UUID>"},"payload":{...}}

event: message
data: {"type":"message.persisted","ids":{"message_id":"<UUID>","turn_id":"<UUID>"},"payload":{"client_message_id":"tmp_xxx","role":"user"}}

event: message
data: {"type":"message.created","ids":{"message_id":"<UUID>","turn_id":"<UUID>"},"payload":{"role":"assistant","status":"streaming"}}

event: message
data: {"type":"message.text.delta","ids":{"message_id":"<UUID>"},"payload":{"delta":"根据您描述的"}}

event: message
data: {"type":"message.text.delta","ids":{"message_id":"<UUID>"},"payload":{"delta":"症状..."}}

event: state
data: {"type":"state.extracted_info.upsert","ids":{},"payload":{"info":[...]}}

event: state
data: {"type":"state.phase.changed","ids":{},"payload":{"from":"collecting","to":"ready_for_analysis","reason":"..."}}

event: message
data: {"type":"message.completed","ids":{"message_id":"<UUID>","turn_id":"<UUID>"},"payload":{"status":"completed","finish_reason":"stop","usage":{...}}}

event: title                ← 仅新会话时，在 stream.done 之前
data: {"type":"title.generated","ids":{},"payload":{"title":"关于肩颈疼痛的咨询"}}

event: stream
data: {"type":"stream.done","ids":{},"payload":{}}
```

---

## 关键设计决策总结

| 决策 | 原因 |
|------|------|
| 统一端点 `POST /consultation-runs` | 消除两步请求的时间窗口和空会话问题 |
| `conversationId: null` 表示新会话 | 前端不需要先调用创建接口，后端原子处理 |
| 数据库级幂等性 `ON CONFLICT DO NOTHING` | 比应用层"先查后插"更安全，无竞态条件 |
| `justCreatedRef` 防重挂载 | 保护正在进行的 SSE 流不被 React key 变化中断 |
| `activeConversationIdRef` 同步 ID | SSE 回调在同一事件批次中需要最新 ID，但 React 还没有 re-render |
| `replace: true` 导航 | 不产生浏览器历史记录，用户点返回不会回到空页面 |
| 乐观侧边栏插入 | 用户立即看到新会话，无需等待 refetch |
| 标题同步生成（阻塞） | 前端收到 stream.done 时标题已就绪，实时更新侧边栏 |
| `handlePhaseChange` 去重 | 避免相同阶段的重复事件触发不必要的 cancelQueries + setQueryData |
| `handleStreamFinished` 双缓存刷新 | 确保 UI 反映最终持久化状态 |
| `UpdateMessageCompletedWithStatus` 原子更新 | 单条 UPDATE 语句设置 parts + status，避免崩溃导致不一致 |

---

## 边界情况

### 网络失败

```ts
// useAssistantChatRuntime.ts
try {
  const response = await startRequest();
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }
  // ... SSE 处理
} finally {
  setIsStreaming(false);
  optionsRef.current.onStreamFinished?.();
}
```

HTTP 错误（如 400/500）会直接抛出异常，assistant-ui 框架处理错误显示。SSE 流中断会导致 `consumeSSEStream` 的 `onError` 回调触发。

### 并发发送

assistant-ui 框架在 `run()` 执行期间会禁用发送按钮。`requestId`（UUID）提供了额外的幂等性保证。

### 页面刷新

用户刷新页面时，URL 已经是 `/consultation/<UUID>`，`useConversationQuery` 会加载历史消息，`buildInterruptedTurnSeed()` 会恢复中断的流式状态。不会触发统一端点的新会话逻辑。

### 重复请求（幂等性）

如果用户快速点击发送按钮（网络抖动导致重试），同一个 `requestId` 可能到达后端多次：

1. 第一次请求：创建 run，正常执行
2. 第二次请求：`CheckIdempotency` 发现 run 已存在
   - 如果 run 仍在执行（`status == "running"`）→ 返回 409 Conflict
   - 如果 run 已完成 → 重放已完成的结果（`replayCompletedRun`）

---

## 文件索引

| 文件 | 职责 |
|------|------|
| `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts` | 前端 adapter，构造请求 + SSE 处理 + assistant-ui 集成 |
| `apps/web/src/features/consultation/hooks/useSSEProcessor.ts` | SSE 文本流解析器 |
| `apps/web/src/features/consultation/runtime/activeTurnReducer.ts` | 纯 reducer，管理 ActiveTurnState |
| `apps/web/src/features/consultation/pages/ConsultationPage.tsx` | 页面组件，SSE 回调 + 缓存更新 + 静默跳转 |
| `apps/web/src/features/consultation/services/consultationService.ts` | API 调用封装 |
| `packages/contracts/src/stream-events.ts` | StreamEvent 类型定义（前后端共享） |
| `apps/api/internal/consultation/runtime.go` | 后端核心逻辑：StartRun, executeRunFlow, finishTurn |
| `apps/api/internal/dto/consultation.go` | 请求/响应 DTO 定义 |
| `apps/api/internal/handler/consultation_handler.go` | HTTP handler 路由 |
| `apps/api/internal/repository/run_repository.go` | RunRepository.CreateWithIdempotency |
| `apps/api/internal/repository/message_repository.go` | MessageRepository.UpdateMessageCompletedWithStatus |
