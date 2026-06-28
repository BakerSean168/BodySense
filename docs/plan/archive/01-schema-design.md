# 01 — 数据库表结构设计

## 设计原则

- 通用层（conversations / messages / runs）与领域扩展层（consultation_sessions）分离
- 所有 ID 使用 UUID，由服务端通过 `uuid_generate_v7()` 或应用层生成
- 时间字段统一使用 `timestamptz`
- 软删除：`deleted_at` 字段，非物理删除
- JSONB 用于灵活结构数据（parts, metadata, usage）

---

## 通用层表

### `conversations`

一整个聊天会话的容器。

```sql
CREATE TABLE conversations (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
  user_id         UUID NOT NULL,

  -- 基本信息
  title           TEXT,
  title_status    VARCHAR(20) NOT NULL DEFAULT 'pending',
  -- pending / generating / generated

  status          VARCHAR(20) NOT NULL DEFAULT 'active',
  -- active / archived / deleted

  -- 模型配置
  default_model   TEXT,
  system_prompt_version TEXT,

  -- Provider 优化字段（可选，非 source of truth）
  provider                    TEXT,
  provider_conversation_id    TEXT,
  provider_last_response_id   TEXT,

  -- 活跃流状态（用于断线恢复）
  active_run_id     UUID,
  active_stream_id  TEXT,

  -- 会话摘要（长会话压缩上下文用）
  summary     TEXT,

  -- 灵活扩展
  metadata    JSONB NOT NULL DEFAULT '{}',

  -- 时间
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_message_at TIMESTAMPTZ,
  deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_conversations_user_last
  ON conversations (user_id, last_message_at DESC)
  WHERE deleted_at IS NULL;
```

**字段说明：**

| 字段 | 用途 |
|------|------|
| `title_status` | 标题生成状态。`pending` = 尚未生成，`generating` = 正在异步生成，`generated` = 已生成 |
| `provider_last_response_id` | 存储 Python 返回的 `response_id`，下一轮回传实现 provider 侧上下文串联 |
| `active_run_id` / `active_stream_id` | 用于断线恢复：前端重连时可通过此字段找到正在执行的 run |
| `summary` | 长会话的上下文压缩摘要，避免每次发送全部历史消息 |

---

### `messages`

单条消息，一个会话中的消息按 `seq` 排序。

```sql
CREATE TABLE messages (
  id                UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
  conversation_id   UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  turn_id           UUID NOT NULL,

  -- 消息关系
  parent_message_id UUID,
  -- 用于消息编辑/分叉：编辑用户消息后，新消息指向原消息

  role    VARCHAR(20) NOT NULL,
  -- user / assistant / system / tool

  status  VARCHAR(20) NOT NULL DEFAULT 'completed',
  -- submitted / streaming / completed / failed / aborted

  seq     INT NOT NULL,
  -- 同一会话内的递增序号，用于排序

  -- 内容（多模态）
  parts   JSONB NOT NULL DEFAULT '[]',
  -- [{type:"text", text:"..."}, {type:"image", url:"..."}, {type:"tool-call", ...}]

  -- 全文检索用的纯文本副本
  content_text TEXT,

  -- Provider 信息
  model                 TEXT,
  provider              TEXT,
  provider_message_id   TEXT,
  provider_response_id  TEXT,

  -- Token 统计
  input_tokens    INT,
  output_tokens   INT,
  total_tokens    INT,

  -- 错误信息（status = failed 时）
  error     JSONB,
  -- {"code":"MODEL_TIMEOUT","message":"模型响应超时"}

  metadata  JSONB NOT NULL DEFAULT '{}',

  -- 时间
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

  -- 同一会话内序号唯一
  UNIQUE (conversation_id, seq)
);

CREATE INDEX idx_messages_conversation_seq
  ON messages (conversation_id, seq);

CREATE INDEX idx_messages_turn
  ON messages (turn_id);

CREATE INDEX idx_messages_conversation_role
  ON messages (conversation_id, role)
  WHERE status != 'aborted';
```

**parts 格式示例：**

```json
// 文本消息
[{"type": "text", "text": "我来帮你分析一下腰部问题"}]

// 包含引用的消息
[
  {"type": "text", "text": "根据资料，腰部不适常见原因有..."},
  {"type": "source", "title": "腰痛康复指南", "url": "...", "snippet": "..."}
]

// 工具调用
[
  {"type": "tool-call", "tool": "extract_symptoms", "args": {"body_part": "腰部"}},
  {"type": "tool-result", "tool": "extract_symptoms", "result": {...}}
]
```

---

### `runs`

一次模型调用 / agent 执行。幂等性的核心实体。

```sql
CREATE TABLE runs (
  id                UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
  conversation_id   UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  turn_id           UUID NOT NULL,

  -- 幂等键
  request_id  TEXT NOT NULL,
  user_id     UUID NOT NULL,

  status  VARCHAR(20) NOT NULL DEFAULT 'running',
  -- running / completed / failed / cancelled

  model     TEXT NOT NULL,
  provider  TEXT,

  -- Provider 优化
  provider_response_id TEXT,

  -- 执行信息
  started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at  TIMESTAMPTZ,

  error     JSONB,
  usage     JSONB,
  -- {"input_tokens":1234, "output_tokens":567, "total_tokens":1801}

  metadata  JSONB NOT NULL DEFAULT '{}',

  -- 幂等约束：同一用户的同一请求只能创建一次 run
  UNIQUE (user_id, request_id)
);

CREATE INDEX idx_runs_conversation
  ON runs (conversation_id);

CREATE INDEX idx_runs_turn
  ON runs (turn_id);
```

**幂等工作流程：**

```
请求进入
  │
  ├─ SELECT id, status FROM runs WHERE user_id = ? AND request_id = ?
  │
  ├─ 找到且 status = running  → 返回 409 Conflict 或 SSE "already_in_progress"
  ├─ 找到且 status = completed → 返回已完成的消息（不重新调用 LLM）
  └─ 未找到 → 创建新 run，继续正常流程
```

---

### `turns`（可选，MVP 可省略）

一轮对话的逻辑分组。MVP 阶段 turn_id 可以直接等于 run_id，此表可暂缓创建。

```sql
CREATE TABLE turns (
  id                UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
  conversation_id   UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,

  seq         INT NOT NULL,
  active_run_id UUID,
  -- 当前活跃的 run（支持重生成时一个 turn 有多个 run）

  metadata    JSONB NOT NULL DEFAULT '{}',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

  UNIQUE (conversation_id, seq)
);
```

---

## 领域扩展层表

### `consultation_sessions`

咨询会话的领域数据，通过 `conversation_id` 关联到通用 `conversations` 表。

```sql
CREATE TABLE consultation_sessions (
  conversation_id UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,

  phase VARCHAR(30) NOT NULL DEFAULT 'collecting',
  -- collecting / ready_for_analysis / analysis_ready
  -- diagnosis_confirmed / plan_ready / completed

  -- 结构化提取信息
  extracted_info JSONB NOT NULL DEFAULT '[]',
  -- [{body_part:"腰部", symptom_type:"不适", duration:"...", trigger:"...", severity:"..."}]

  -- 诊断结果
  diagnosis JSONB,
  -- {diagnoses: [{name, confidence, severity, basis, ...}], citations: [...]}

  -- 治疗方案
  treatment_plan JSONB,
  -- {goal, duration_weeks, correction_exercises: [...], daily_habits: [...], ...}

  -- 时间
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at    TIMESTAMPTZ
);

CREATE INDEX idx_consultation_phase
  ON consultation_sessions (phase)
  WHERE phase != 'completed';
```

---

## 阶段状态机

```
collecting ──► ready_for_analysis ──► analysis_ready
                                           │
                                           ▼
                                    diagnosis_confirmed
                                           │
                                           ▼
                                       plan_ready
                                           │
                                           ▼
                                       completed
```

**规则：**
- 转换只能前进，不能回退（rank 校验）
- AI 在 SSE 流中提议 `phase_change` 事件
- Go 校验 `ShouldAdvancePhase(current, proposed)` 后才持久化
- 非法转换被静默忽略并记录日志

```go
var phaseRank = map[string]int{
  "collecting":            0,
  "ready_for_analysis":    1,
  "analysis_ready":        2,
  "diagnosis_confirmed":   3,
  "plan_ready":            4,
  "completed":             5,
}

func ShouldAdvancePhase(current, next string) bool {
  return phaseRank[next] > phaseRank[current]
}
```

---

## ER 关系图

```
conversations (通用)
  │
  ├── messages (通用)         conversation_id FK
  ├── runs (通用)             conversation_id FK
  ├── turns (通用, 可选)      conversation_id FK
  │
  └── consultation_sessions (领域)  conversation_id FK
```

---

## 与现有实现的对比

| 维度 | 现有实现 | 新设计 |
|------|----------|--------|
| 消息存储 | JSONB 数组在 session 行 | 独立 messages 表，每条一行 |
| 幂等性 | 无 | runs 表 unique(user_id, request_id) |
| 竞态条件 | AppendMessage 读-改-写无锁 | 单条消息 INSERT，并发安全 |
| 阶段管理 | Go 代码在 SSE 解析后硬编码推进 | AI 提议 + Go 校验 |
| ID 生成 | 前端生成正式 UUID | 服务端 UUIDv7 |
| 领域隔离 | 咨询逻辑与会话逻辑混在一起 | 通用层 + 领域扩展层 |
| Provider 字段 | 无 | provider_last_response_id 支持上下文串联 |
| 消息状态 | 无 | submitted/streaming/completed/failed/aborted |
