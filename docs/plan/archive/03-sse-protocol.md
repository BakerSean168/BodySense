# 03 — 客户端 SSE 事件协议

## 概述

Go API 通过 `POST /api/v1/chat/send` 返回 SSE 流。Go 是协议的唯一定义者和生产者。Python AI 服务返回的结构化 JSON 事件由 Go 映射为客户端 SSE 事件。

---

## 连接规范

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

SSE 格式：

```
event: <event_name>
data: <json_payload>

```

（每个事件后有两个换行符）

---

## 事件类型

### 1. `conversation.created`

新会话创建后立即发送。前端收到后执行 `router.replace('/consultation/{conversationId}')`。

```
event: conversation.created
data: {"conversationId":"conv_0197f3c2-...","replacesDraftId":"draft_abc"}
```

| 字段 | 说明 |
|------|------|
| `conversationId` | 服务端生成的正式会话 ID |
| `replacesDraftId` | 对应请求中的 `clientDraftId`，前端用此匹配本地草稿 |

---

### 2. `message.persisted`

用户消息已落库。前端收到后用正式 `messageId` 替换临时 `clientMessageId`。

```
event: message.persisted
data: {"clientMessageId":"tmp_msg_abc","messageId":"msg_0197f3c3-...","role":"user"}
```

---

### 3. `message.created`

Assistant 消息开始生成。前端收到后显示 typing indicator。

```
event: message.created
data: {"messageId":"msg_0197f3c4-...","role":"assistant","status":"streaming","turnId":"turn_0197..."}
```

---

### 4. `text.delta`

文本流式增量。

```
event: text.delta
data: {"messageId":"msg_0197f3c4-...","delta":"我来帮你"}
```

```
event: text.delta
data: {"messageId":"msg_0197f3c4-...","delta":"分析一下腰部问题"}
```

前端拼接 delta 显示流式文本。

---

### 5. `tool.call`

AI 调用工具的通知。前端可选择显示"正在检索知识..."等状态。

```
event: tool.call
data: {"messageId":"msg_0197f3c4-...","tool":"search_knowledge","args":{"query":"腰部不适原因"}}
```

---

### 6. `tool.result`

工具执行结果。

```
event: tool.result
data: {"messageId":"msg_0197f3c4-...","tool":"search_knowledge","result":{"found":3,"items":[...]}}
```

---

### 7. `extracted_info`

结构化信息提取事件。前端实时更新右侧面板。

```
event: extracted_info
data: {"messageId":"msg_0197f3c4-...","info":{"body_part":"腰部","symptom_type":"不适","severity":"unknown"}}
```

**说明：** 每次事件携带一条新提取或更新的信息项。前端追加/更新到 InfoPanel。

---

### 8. `phase_change`

阶段转换提议。前端更新 UI 状态（如启用"生成分析"按钮）。

```
event: phase_change
data: {"messageId":"msg_0197f3c4-...","from":"collecting","to":"ready_for_analysis","reason":"已收集足够症状信息"}
```

**注意：** Go 会校验转换合法性后再持久化。即使 Go 拒绝了转换，前端仍会收到此事件（附带 `rejected: true` 可选字段）。

---

### 9. `citation`

知识库引用。

```
event: citation
data: {"messageId":"msg_0197f3c4-...","citation":{"title":"腰痛康复指南","snippet":"...","category":"exercise"}}
```

---

### 10. `red_flag`

安全警告（红旗症状）。

```
event: red_flag
data: {"messageId":"msg_0197f3c4-...","flag":{"type":"red_flag","message":"建议尽快就医检查","severity":"high"}}
```

---

### 11. `message.completed`

Assistant 消息生成完成。

```
event: message.completed
data: {"messageId":"msg_0197f3c4-...","status":"completed","finishReason":"stop","usage":{"inputTokens":1234,"outputTokens":567}}
```

前端：移除 typing indicator，显示完整消息，保存 metadata。

---

### 12. `message.failed`

生成失败。

```
event: message.failed
data: {"messageId":"msg_0197f3c4-...","status":"failed","error":{"code":"MODEL_TIMEOUT","message":"模型响应超时"}}
```

前端：显示错误状态和重试按钮。消息已在 DB 中标记为 `failed`。

---

### 13. `title.generated`

会话标题异步生成完成。

```
event: title.generated
data: {"conversationId":"conv_0197...","title":"腰部不适咨询"}
```

---

### 14. `done`

流结束。所有业务数据已通过上述事件发送完毕。

```
event: done
data: {}
```

前端：关闭 EventSource 连接，清理流式状态。

---

## 完整时序示例

### 新会话首条消息

```
Client                          Go API                          Python AI
  │                               │                                │
  │── POST /chat/send ──────────►│                                │
  │   (conversationId=null)       │                                │
  │                               │── BEGIN TRANSACTION ──────────►│
  │                               │   create conversation          │
  │                               │   create user message          │
  │                               │   create assistant placeholder │
  │                               │   create run                   │
  │                               │── COMMIT ─────────────────────►│
  │                               │                                │
  │◄── conversation.created ──────│── POST /api/chat/stream ──────►│
  │    router.replace()           │                                │
  │                               │◄── {"type":"text_delta",...}───│
  │◄── message.persisted ─────────│                                │
  │    replace clientMessageId    │◄── {"type":"text_delta",...}───│
  │                               │                                │
  │◄── message.created ───────────│◄── {"type":"extracted_info"}───│
  │    show typing indicator      │                                │
  │                               │◄── {"type":"text_delta",...}───│
  │◄── text.delta ────────────────│                                │
  │◄── text.delta ────────────────│◄── {"type":"phase_change"}─────│
  │◄── extracted_info ────────────│                                │
  │◄── text.delta ────────────────│◄── {"type":"done",...}─────────│
  │◄── phase_change ──────────────│                                │
  │                               │── update assistant message ───►│
  │                               │── update conversation ────────►│
  │                               │── update consultation_sessions►│
  │◄── message.completed ─────────│                                │
  │◄── done ──────────────────────│                                │
  │                               │                                │
  │   (async)                     │── generate title ─────────────►│
  │◄── title.generated ───────────│                                │
```

### 已有会话继续对话

```
Client                          Go API
  │                               │
  │── POST /chat/send ──────────►│
  │   (conversationId=conv_xxx)   │
  │                               │── check idempotency (runs)
  │                               │── create user message
  │                               │── create assistant placeholder
  │                               │── create run
  │                               │── load history from DB
  │                               │── call Python AI
  │                               │
  │◄── message.persisted ─────────│
  │◄── message.created ───────────│
  │◄── text.delta ────────────────│
  │◄── ... ───────────────────────│
  │◄── message.completed ─────────│
  │◄── done ──────────────────────│
```

---

## 前端处理要点

### 临时 ID 替换流程

```typescript
// 发送时
const clientMessageId = `tmp_${crypto.randomUUID()}`;
optimisticMessages.add({ id: clientMessageId, role: 'user', ... });

// 收到 message.persisted 时
onMessagePersisted(event) {
  const msg = optimisticMessages.find(m => m.id === event.clientMessageId);
  msg.id = event.messageId;  // 替换为正式 ID
}

// 收到 conversation.created 时
onConversationCreated(event) {
  router.replace(`/consultation/${event.conversationId}`);
}
```

### 断线恢复

如果流中断，前端可调用：

```
GET /api/v1/conversations/:id/runs?status=running
```

如果有 running 状态的 run，说明生成仍在进行。前端可轮询等待 `message.completed` 事件，或等待未来的 SSE 恢复端点。

---

## 与现有 SSE 协议的对比

| 维度 | 现有实现 | 新协议 |
|------|----------|--------|
| 协议定义者 | Python AI 服务 | Go API |
| 会话创建通知 | 无（前端已知 ID） | `conversation.created` 事件 |
| 消息 ID 管理 | 无正式 ID | `message.persisted` 返回正式 ID |
| 流结束信号 | `done` 事件（Python 定义） | `done` 事件（Go 定义） |
| 错误处理 | 流中断，无明确错误事件 | `message.failed` 事件 |
| 工具调用 | 不暴露给前端 | `tool.call` / `tool.result` 事件 |
| 标题生成 | 无 | `title.generated` 异步事件 |
