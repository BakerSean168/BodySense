# 02 — API 端点设计

## 设计原则

- RESTful 风格，资源导向
- 所有端点需要认证（JWT / session token）
- 用户只能访问自己的资源（ownership check）
- 错误响应统一格式
- 分页使用 cursor-based 或 offset-based

---

## 通用会话 API

### 1. 发送消息（核心端点）

```
POST /api/v1/chat/send
```

**请求：**

```json
{
  "conversationId": "conv_0197f3c2-..." | null,
  "clientDraftId": "draft_abc",
  "clientMessageId": "tmp_msg_abc",
  "requestId": "req_abc",
  "message": {
    "role": "user",
    "parts": [
      {"type": "text", "text": "我最近腰有点不舒服"}
    ]
  },
  "context": {
    "entry": "consultation",
    "profileId": "profile_xxx"
  },
  "model": "qwen-max"
}
```

**字段说明：**

| 字段 | 必填 | 说明 |
|------|------|------|
| `conversationId` | 否 | 为空表示创建新会话 |
| `clientDraftId` | 否 | 前端草稿 ID，仅用于日志 |
| `clientMessageId` | 是 | 前端临时消息 ID，用于 SSE 事件中替换 |
| `requestId` | 是 | 幂等键，前端生成的 UUID |
| `message.parts` | 是 | 消息内容，支持多模态 |
| `context` | 否 | 领域上下文（咨询入口、用户档案等） |
| `model` | 否 | 指定模型，默认使用会话的 default_model |

**响应：** SSE 流（见 [03-sse-protocol.md](./03-sse-protocol.md)）

**幂等行为：**

```
requestId 已存在且 run status = running  → 409 Conflict
requestId 已存在且 run status = completed → 返回 200 + 已完成消息的 JSON
requestId 不存在 → 正常创建 run，返回 SSE 流
```

---

### 2. 获取会话列表

```
GET /api/v1/conversations?cursor=<cursor>&limit=20
```

**响应：**

```json
{
  "conversations": [
    {
      "id": "conv_0197...",
      "title": "腰部不适咨询",
      "status": "active",
      "lastMessageAt": "2026-06-27T10:30:00Z",
      "messageCount": 12,
      "metadata": {}
    }
  ],
  "nextCursor": "eyJpZCI6...",
  "hasMore": true
}
```

**过滤规则：**
- 只返回 `deleted_at IS NULL` 的会话
- 只返回有消息的会话（排除空壳）
- 按 `last_message_at DESC` 排序

---

### 3. 获取会话详情

```
GET /api/v1/conversations/:id
```

**响应：**

```json
{
  "id": "conv_0197...",
  "title": "腰部不适咨询",
  "status": "active",
  "model": "qwen-max",
  "messages": [
    {
      "id": "msg_0197...",
      "turnId": "turn_0197...",
      "role": "user",
      "status": "completed",
      "parts": [{"type": "text", "text": "..."}],
      "createdAt": "2026-06-27T10:00:00Z"
    },
    {
      "id": "msg_0198...",
      "turnId": "turn_0197...",
      "role": "assistant",
      "status": "completed",
      "parts": [{"type": "text", "text": "..."}],
      "inputTokens": 1234,
      "outputTokens": 567,
      "createdAt": "2026-06-27T10:00:05Z"
    }
  ],
  "metadata": {},
  "createdAt": "2026-06-27T10:00:00Z",
  "updatedAt": "2026-06-27T10:30:00Z"
}
```

---

### 4. 软删除会话

```
DELETE /api/v1/conversations/:id
```

**响应：** `204 No Content`

设置 `deleted_at = now()`，不物理删除。

---

### 5. 归档会话

```
PATCH /api/v1/conversations/:id
Content-Type: application/json

{"status": "archived"}
```

---

### 6. 获取会话的 runs

```
GET /api/v1/conversations/:id/runs
```

**响应：**

```json
{
  "runs": [
    {
      "id": "run_0197...",
      "turnId": "turn_0197...",
      "status": "completed",
      "model": "qwen-max",
      "startedAt": "2026-06-27T10:00:01Z",
      "completedAt": "2026-06-27T10:00:05Z",
      "usage": {"inputTokens": 1234, "outputTokens": 567}
    }
  ]
}
```

---

### 7. 异步生成会话标题

```
POST /api/v1/conversations/:id/title
```

触发异步标题生成。生成完成后通过 WebSocket 或轮询获取。

---

## 咨询领域 API

以下端点操作 `consultation_sessions` 扩展表。

### 8. 获取咨询详情

```
GET /api/v1/consultations/:id
```

**响应：** 在通用会话响应基础上，附加咨询领域数据：

```json
{
  "...": "通用会话字段",
  "consultation": {
    "phase": "collecting",
    "extractedInfo": [
      {"bodyPart": "腰部", "symptomType": "不适", "severity": "unknown"}
    ],
    "diagnosis": null,
    "treatmentPlan": null
  }
}
```

---

### 9. 更新提取信息

```
PUT /api/v1/consultations/:id/extracted-info
Content-Type: application/json

{
  "extracted_info": [
    {"body_part": "腰部", "symptom_type": "不适", "duration": "一周"}
  ]
}
```

**响应：** `200 OK`

---

### 10. 生成诊断分析

```
POST /api/v1/consultations/:id/diagnosis
```

触发诊断生成。Go handler 执行 RAG 检索，调用 Python AI 服务，保存结果。

**响应：**

```json
{
  "diagnosis": {
    "diagnoses": [
      {"name": "腰肌劳损", "confidence": 0.75, "severity": "moderate", "basis": "..."}
    ],
    "citations": [...]
  }
}
```

---

### 11. 确认诊断

```
PUT /api/v1/consultations/:id/confirm
Content-Type: application/json

{
  "diagnosis": {"name": "腰肌劳损", "confidence": 0.75, "severity": "moderate"}
}
```

将阶段推进为 `diagnosis_confirmed`。

---

### 12. 生成治疗方案

```
POST /api/v1/consultations/:id/treatment
```

需要阶段为 `diagnosis_confirmed`。触发治疗方案生成。

**响应：** 治疗方案 JSON。

---

## 错误响应格式

```json
{
  "error": {
    "code": "CONVERSATION_NOT_FOUND",
    "message": "会话不存在或无权访问",
    "details": {}
  }
}
```

**通用错误码：**

| HTTP Status | Code | 说明 |
|-------------|------|------|
| 400 | `INVALID_REQUEST` | 请求格式错误 |
| 401 | `UNAUTHORIZED` | 未认证 |
| 403 | `FORBIDDEN` | 无权访问 |
| 404 | `CONVERSATION_NOT_FOUND` | 会话不存在 |
| 409 | `RUN_IN_PROGRESS` | 幂等冲突，已有运行中的 run |
| 422 | `INVALID_PHASE_TRANSITION` | 阶段转换非法 |
| 500 | `INTERNAL_ERROR` | 服务端内部错误 |

---

## 端点总览

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/chat/send` | 发送消息（SSE 响应） |
| `GET` | `/api/v1/conversations` | 会话列表 |
| `GET` | `/api/v1/conversations/:id` | 会话详情 + 消息 |
| `DELETE` | `/api/v1/conversations/:id` | 软删除会话 |
| `PATCH` | `/api/v1/conversations/:id` | 更新会话（归档等） |
| `GET` | `/api/v1/conversations/:id/runs` | 会话的 runs 列表 |
| `POST` | `/api/v1/conversations/:id/title` | 异步生成标题 |
| `GET` | `/api/v1/consultations/:id` | 咨询详情（含领域数据） |
| `PUT` | `/api/v1/consultations/:id/extracted-info` | 更新提取信息 |
| `POST` | `/api/v1/consultations/:id/diagnosis` | 生成诊断 |
| `PUT` | `/api/v1/consultations/:id/confirm` | 确认诊断 |
| `POST` | `/api/v1/consultations/:id/treatment` | 生成治疗方案 |
