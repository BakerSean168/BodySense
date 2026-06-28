# 会话管理系统统一重设计方案

> 状态：已完成 (根据实际开发做出部分设计微调) | 创建时间：2026-06-27 | 更新时间：2026-06-28
> 整合自：会话管理重设计 + 历史记录区重设计 + 多供应商 AI Router

---

## 一、背景与目标

当前系统存在三类问题，本方案统一解决：

| 领域 | 问题 |
|------|------|
| **会话管理** | 消息以 JSONB 数组存储，存在读-改-写竞态；前端生成正式 session ID；无幂等保护 |
| **历史记录 UI** | 侧边栏功能简陋，无置顶/分享/重命名；卡片样式陈旧 |
| **AI 服务** | 单供应商锁定；无自动降级；JSON 解析代码重复三份 |

### 统一目标

1. 通用会话层（conversations/messages/runs）+ 领域扩展层分离
2. 侧边栏三区布局 + 置顶 + 分享 + AI 标题 + shadcn/ui
3. 配置驱动的多 Provider AI 路由 + 自动熔断降级
4. Go 为唯一编排器，Python 为无状态 LLM 执行引擎

---

## 二、核心设计决策

| # | 决策项 | 选择 | 来源 |
|---|--------|------|------|
| 1 | 主线方案 | 会话管理重设计为骨架，其余合并 | 统一 |
| 2 | 置顶字段归属 | `conversations` 表（通用层） | 原方案 1 → 上提 |
| 3 | 标题生成 | `conversations.title` + `title_status`，SSE 推送 | 方案 3 |
| 4 | 分享功能 | 通用层 `conversation_shares` 表，JSONB 扩展 | 原方案 1 → 泛化 |
| 5 | API 命名 | 复数 RESTful（`/api/v1/conversations`） | 方案 3 |
| 6 | UI 组件库 | 引入 shadcn/ui | 方案 1 |
| 7 | 模型选择 | Go 传 `use_case`，Python 路由层决定 model | 方案 2 |
| 8 | Python 改造 | 协议改造 + 内部实现合并为一个阶段 | 统一 |
| 9 | 分支策略 | 直接在 dev 上开发 | 用户决定 |
| 10 | 旧代码迁移 | 干净切换，不保留旧表 | 用户决定 |

---

## 三、架构总览

```
┌─────────────┐     SSE (Go-defined protocol)     ┌──────────────────┐
│   Frontend   │ ◄──────────────────────────────── │   Go API (Gin)   │
│  React +     │     POST /api/v1/chat/send        │                  │
│ assistant-ui │ ────────────────────────────────► │  ┌────────────┐  │
│ + shadcn/ui  │                                    │  │Conversations│  │
└─────────────┘                                    │  │Messages     │  │
                                                    │  │Runs         │  │
                                                    │  │Shares       │  │
                                                    │  └──────┬─────┘  │
                                                    └─────────┼────────┘
                                                              │
                                                structured    │  PostgreSQL
                                                NDJSON events │
                                                              │
                                                    ┌─────────▼────────┐
                                                    │  Python AI Service│
                                                    │  (FastAPI)        │
                                                    │                   │
                                                    │  ┌─ AIService ──┐ │
                                                    │  │ ModelRouter  │ │
                                                    │  │ CircuitBreak │ │
                                                    │  │ Providers    │ │
                                                    │  └──────────────┘ │
                                                    │                   │
                                                    │  - LLM 调用       │
                                                    │  - 工具执行       │
                                                    │  - Agent 循环    │
                                                    └──────────────────┘
```

### 职责划分

| 层 | 职责 |
|---|------|
| **Frontend** | UI 渲染、乐观更新、临时 ID、SSE 消费、路由管理、shadcn/ui 组件 |
| **Go API** | 认证、会话/消息 CRUD、SSE 协议、幂等校验、阶段校验、分享/置顶管理 |
| **Python AI** | 多 Provider 路由、LLM 调用、工具执行、Agent 循环、流式文本生成 |
| **PostgreSQL** | 唯一数据源 |

---

## 四、数据库设计

### 4.1 通用层表

#### `conversations`

```sql
CREATE TABLE conversations (
  id              UUID PRIMARY KEY DEFAULT uuidv7(),
  user_id         UUID NOT NULL,

  -- 基本信息
  title           TEXT,
  title_status    VARCHAR(20) NOT NULL DEFAULT 'pending',
  -- pending / generating / generated

  status          VARCHAR(20) NOT NULL DEFAULT 'active',
  -- active / archived / deleted

  -- 置顶（通用功能）
  pinned          BOOLEAN NOT NULL DEFAULT FALSE,
  pinned_at       TIMESTAMPTZ,

  -- 模型配置
  default_model   TEXT,
  system_prompt_version TEXT,

  -- Provider 优化字段
  provider                    TEXT,
  provider_conversation_id    TEXT,
  provider_last_response_id   TEXT,

  -- 活跃流状态（断线恢复）
  active_run_id     UUID,
  active_stream_id  TEXT,

  -- 会话摘要（长会话压缩上下文）
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

CREATE INDEX idx_conversations_pinned
  ON conversations (user_id, pinned, pinned_at DESC)
  WHERE deleted_at IS NULL AND pinned = TRUE;
```

#### `messages`

```sql
CREATE TABLE messages (
  id                UUID PRIMARY KEY DEFAULT uuidv7(),
  conversation_id   UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  turn_id           UUID NOT NULL,

  parent_message_id UUID,
  role    VARCHAR(20) NOT NULL,    -- user / assistant / system / tool
  status  VARCHAR(20) NOT NULL DEFAULT 'completed',
  -- submitted / streaming / completed / failed / aborted

  seq     INT NOT NULL,

  -- 内容（多模态）
  parts   JSONB NOT NULL DEFAULT '[]',
  content_text TEXT,               -- 全文检索用纯文本副本

  -- Provider 信息
  model                 TEXT,
  provider              TEXT,
  provider_message_id   TEXT,
  provider_response_id  TEXT,

  -- Token 统计
  input_tokens    INT,
  output_tokens   INT,
  total_tokens    INT,

  -- 错误信息
  error     JSONB,
  metadata  JSONB NOT NULL DEFAULT '{}',

  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

  UNIQUE (conversation_id, seq)
);

CREATE INDEX idx_messages_conversation_seq ON messages (conversation_id, seq);
CREATE INDEX idx_messages_turn ON messages (turn_id);
```

#### `runs`

```sql
CREATE TABLE runs (
  id                UUID PRIMARY KEY DEFAULT uuidv7(),
  conversation_id   UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  turn_id           UUID NOT NULL,

  request_id  TEXT NOT NULL,
  user_id     UUID NOT NULL,

  status  VARCHAR(20) NOT NULL DEFAULT 'running',
  -- running / completed / failed / cancelled

  model     TEXT NOT NULL,
  provider  TEXT,
  provider_response_id TEXT,

  started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at  TIMESTAMPTZ,

  error     JSONB,
  usage     JSONB,
  metadata  JSONB NOT NULL DEFAULT '{}',

  UNIQUE (user_id, request_id)
);

CREATE INDEX idx_runs_conversation ON runs (conversation_id);
CREATE INDEX idx_runs_turn ON runs (turn_id);
```

#### `conversation_shares`

```sql
CREATE TABLE conversation_shares (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    share_token VARCHAR(32) UNIQUE NOT NULL,
    snapshot_messages JSONB NOT NULL,
    snapshot_metadata JSONB,           -- 领域扩展（诊断、治疗方案等）
    snapshot_title VARCHAR(200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_conversation_shares_token ON conversation_shares(share_token);
CREATE INDEX idx_conversation_shares_conversation ON conversation_shares(conversation_id);
```

### 4.2 领域扩展层表

#### `consultation_sessions`

```sql
CREATE TABLE consultation_sessions (
  conversation_id UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,

  phase VARCHAR(30) NOT NULL DEFAULT 'collecting',
  -- collecting / ready_for_analysis / analysis_ready
  -- diagnosis_confirmed / plan_ready / completed

  extracted_info JSONB NOT NULL DEFAULT '[]',
  diagnosis JSONB,
  treatment_plan JSONB,

  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at    TIMESTAMPTZ
);
```

### 4.3 阶段状态机

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

转换只能前进，不能回退。AI 提议，Go 校验。

### 4.4 ER 关系图

```
conversations (通用)
  │
  ├── messages (通用)              conversation_id FK
  ├── runs (通用)                  conversation_id FK
  ├── conversation_shares (通用)   conversation_id FK
  │
  └── consultation_sessions (领域) conversation_id FK
```

---

## 五、API 设计

### 5.1 通用会话 API

| 方法 | 端点 | 说明 |
|------|------|------|
| `POST` | `/api/v1/chat/send` | 发送消息（SSE 响应） |
| `GET` | `/api/v1/conversations` | 会话列表（含 pinned、pinned_at） |
| `GET` | `/api/v1/conversations/:id` | 会话详情 + 消息 |
| `DELETE` | `/api/v1/conversations/:id` | 软删除 |
| `PATCH` | `/api/v1/conversations/:id` | 更新（归档等） |
| `GET` | `/api/v1/conversations/:id/runs` | 会话的 runs |
| `PATCH` | `/api/v1/conversations/:id/pin` | 切换置顶状态 |
| `POST` | `/api/v1/conversations/:id/title` | 触发 AI 标题生成 |
| `PUT` | `/api/v1/conversations/:id/title` | 用户重命名标题 |
| `POST` | `/api/v1/conversations/:id/share` | 生成分享链接 |
| `DELETE` | `/api/v1/conversations/:id/share` | 取消分享 |
| `GET` | `/api/v1/conversations/share/:token` | 获取分享内容（无需认证） |

### 5.2 咨询领域 API

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/v1/consultations/:id` | 咨询详情（含领域数据） |
| `PUT` | `/api/v1/consultations/:id/extracted-info` | 更新提取信息 |
| `POST` | `/api/v1/consultations/:id/diagnosis` | 生成诊断 |
| `PUT` | `/api/v1/consultations/:id/confirm` | 确认诊断 |
| `POST` | `/api/v1/consultations/:id/treatment` | 生成治疗方案 |

### 5.3 发送消息请求格式

```json
{
  "conversationId": "conv_0197..." | null,
  "clientDraftId": "draft_abc",
  "clientMessageId": "tmp_msg_abc",
  "requestId": "req_abc",
  "message": {
    "role": "user",
    "parts": [{"type": "text", "text": "我最近腰有点不舒服"}]
  },
  "context": {
    "entry": "consultation",
    "profileId": "profile_xxx"
  }
}
```

注意：不传 `model`，由 Python 路由层根据 `use_case` 自动选择。

### 5.4 幂等行为

```
requestId 已存在且 run status = running  → 409 Conflict
requestId 已存在且 run status = completed → 返回已完成消息
requestId 不存在 → 正常创建 run
```

### 5.5 错误响应格式

```json
{
  "error": {
    "code": "CONVERSATION_NOT_FOUND",
    "message": "会话不存在或无权访问"
  }
}
```

---

## 六、SSE 协议

Go 是协议的唯一定义者和生产者。

### 事件类型

| 事件 | 触发时机 | 关键字段 |
|------|----------|----------|
| `conversation.created` | 新会话创建 | `conversationId`, `replacesDraftId` |
| `message.persisted` | 用户消息落库 | `clientMessageId`, `messageId` |
| `message.created` | Assistant 开始生成 | `messageId`, `role`, `status`, `turnId` |
| `text.delta` | 文本流式增量 | `messageId`, `delta` |
| `tool.call` | AI 调用工具 | `messageId`, `tool`, `args` |
| `tool.result` | 工具执行结果 | `messageId`, `tool`, `result` |
| `extracted_info` | 结构化信息提取 | `messageId`, `info` |
| `phase_change` | 阶段转换提议 | `messageId`, `from`, `to`, `reason` |
| `citation` | 知识库引用 | `messageId`, `citation` |
| `red_flag` | 安全警告 | `messageId`, `flag` |
| `message.completed` | 生成完成 | `messageId`, `usage` |
| `message.failed` | 生成失败 | `messageId`, `error` |
| `title.generated` | 标题生成完成 | `conversationId`, `title` |
| `done` | 流结束 | `{}` |

---

## 七、Python AI 服务

### 7.1 多 Provider 架构

```
业务代码（consultation_graph / diagnosis_service / ...）
        │
        ▼
    AIService  ←── 统一入口
        │
        ▼
    ModelRouter  ←── use_case + capabilities + circuit_breaker
        │
        ▼
    AiProvider 适配器（OpenAICompatibleProvider）
        │
        ▼
    MiMo / OpenRouter / Qwen / DeepSeek ...
```

### 7.2 YAML 配置（`config/models.yaml`）

```yaml
providers:
  mimo:
    type: openai-compatible
    base_url: ${MIMO_BASE_URL}
    api_key: ${MIMO_API_KEY}
    models:
      - id: mimo-v2.5-pro
        capabilities: [stream, tools, json_mode, long_context, reasoning]

  openrouter:
    type: openai-compatible
    base_url: https://openrouter.ai/api/v1
    api_key: ${OPENROUTER_API_KEY}
    models:
      - id: deepseek/deepseek-chat
        capabilities: [stream, tools, json_mode]

routes:
  consultation.reply:
    defaults: { temperature: 0.7, max_tokens: 2048 }
    candidates:
      - { provider: mimo, model: mimo-v2.5-pro }
      - { provider: openrouter, model: deepseek/deepseek-chat }

  llm.json:
    defaults: { temperature: 0.3, max_tokens: 2048, response_format: json_object }
    candidates:
      - { provider: mimo, model: mimo-v2.5-pro }
      - { provider: openrouter, model: deepseek/deepseek-chat }

  llm.text:
    defaults: { temperature: 0.3, max_tokens: 2048 }
    candidates:
      - { provider: mimo, model: mimo-v2.5-pro }
      - { provider: openrouter, model: deepseek/deepseek-chat }
```

### 7.3 路由与熔断

- 路由策略：有序 fallback，按 candidates 顺序尝试
- 熔断触发：HTTP 429
- 熔断冷却：60 秒（可配置）
- 恢复方式：冷却期过后自动恢复
- 不熔断的错误：400、401（配置问题，重试无意义）

### 7.4 Go → Python 请求格式

```json
{
  "messages": [...],
  "context": {
    "user_id": "user_xxx",
    "session_id": "conv_xxx",
    "profile": {...},
    "extracted_info": [...],
    "phase": "collecting",
    "previous_response_id": "resp_xxx"
  },
  "use_case": "consultation.reply",
  "tools": [...],
  "stream": true
}
```

### 7.5 Python → Go 响应格式（NDJSON）

```jsonl
{"type":"text_delta","delta":"我来帮你"}
{"type":"tool_call","id":"call_001","tool":"search_knowledge","args":{...}}
{"type":"tool_result","id":"call_001","tool":"search_knowledge","result":{...}}
{"type":"extracted_info","info":{"body_part":"腰部","symptom_type":"不适"}}
{"type":"phase_change","phase":"ready_for_analysis","reason":"已收集足够信息"}
{"type":"citation","citation":{"title":"腰痛康复指南","snippet":"..."}}
{"type":"done","response_id":"resp_xxx","usage":{"input_tokens":1234,"output_tokens":567}}
```

### 7.6 类型定义

```python
@dataclass
class AiRequest:
    use_case: str                          # "consultation.reply" | "llm.json" | "llm.text"
    messages: list[ChatMessage]
    tools: list[ToolDefinition] | None = None
    stream: bool = False
    response_format: str | None = None
    temperature: float | None = None
    max_tokens: int | None = None
    metadata: dict[str, Any] | None = None

@dataclass
class AiResponse:
    text: str
    model: str
    provider: str
    usage: TokenUsage | None = None
    finish_reason: str | None = None
    tool_calls: list[ToolCall] | None = None

@dataclass
class AiStreamEvent:
    type: str          # "text_delta" | "tool_call_delta" | "tool_call_done" | "usage" | "done" | "error"
    text: str | None = None
    tool_call_id: str | None = None
    tool_name: str | None = None
    tool_arguments: dict | None = None
    usage: TokenUsage | None = None
    finish_reason: str | None = None
    error: str | None = None
```

### 7.7 目录结构

```
apps/ai-service/src/
├── ai/                              # 新增：多 Provider 系统
│   ├── __init__.py
│   ├── types.py
│   ├── errors.py
│   ├── service.py                   # AIService（唯一入口）
│   ├── router.py                    # ModelRouter + CircuitBreaker
│   ├── config.py                    # YAML 加载 + env 插值
│   └── providers/
│       ├── __init__.py
│       ├── base.py                  # AiProvider Protocol
│       └── openai_compatible.py
├── config/
│   └── models.yaml
├── services/
│   ├── llm_provider.py              # 删除（被 ai/ 替代）
│   ├── chat_service.py              # 迁移：使用 AIService
│   ├── consultation_graph.py        # 迁移：使用 AIService
│   ├── diagnosis_service.py         # 迁移 + 删除 _parse_json
│   └── ...
└── rag/                             # 暂不迁移 embedding
```

---

## 八、前端设计

### 8.1 路由

```
/consultation                       → 重定向到 /consultation/new
/consultation/new                   → 新咨询草稿页（不入库）
/consultation/:id                   → 咨询会话页
/consultation/share/:token          → 分享页面（无需登录）
```

### 8.2 shadcn/ui 组件

使用 `npx shadcn@latest init` 初始化，添加以下组件：

| 组件 | 用途 |
|------|------|
| `dropdown-menu` | 会话卡片更多菜单 |
| `dialog` | 删除确认、分享确认 |
| `sonner` | Toast 通知 |
| `button` | 统一按钮样式 |

主题：将现有 primary 色系（绿色调）映射到 shadcn CSS 变量。

### 8.3 侧边栏三区布局

```
┌─────────────────────────┐
│  [+ 开始新咨询]          │  ← 按钮区
│  [清空全部历史]          │
├─────────────────────────┤
│  📌 已置顶               │  ← 置顶区（仅有置顶时显示）
│  ┌───────────────────┐  │
│  │ 久坐颈椎酸痛咨询   │  │
│  └───────────────────┘  │
├─────────────────────────┤
│  全部会话               │  ← 聊天区
│  ┌───────────────────┐  │
│  │ 新咨询             │  │
│  │ 膝盖疼痛问诊       │  │
│  └───────────────────┘  │
└─────────────────────────┘
```

### 8.4 会话卡片

- 扁平化：无边框，`rounded-lg`（8px），`bg-gray-50`
- 选中态：`bg-primary-50`，`text-primary-900`
- 只显示标题，不显示日期/状态/body parts
- 桌面端 hover 显示操作按钮，移动端始终显示

### 8.5 下拉菜单项

| 操作 | 说明 |
|------|------|
| 重命名 | 触发标题内联编辑 |
| 复制链接 | 生成分享链接并复制 |
| 取消分享 | 仅在已分享时显示 |
| 删除 | 确认对话框后删除 |

### 8.6 分享页面

- 路由：`/consultation/share/:token`
- 无需登录
- 展示：对话消息 + 诊断摘要（聊天气泡样式与主应用一致）
- 布局：居中卡片式（`max-w-2xl`）
- 底部："由 BodySense 智能问诊生成" + 注册引导

### 8.7 SSE 事件处理

```typescript
const sseHandlers = {
  'conversation.created': ({ conversationId }) => {
    router.replace(`/consultation/${conversationId}`);
    setIsDraft(false);
  },
  'message.persisted': ({ clientMessageId, messageId }) => {
    updateMessageId(clientMessageId, messageId);
  },
  'text.delta': ({ messageId, delta }) => {
    appendTextDelta(messageId, delta);
  },
  'extracted_info': ({ info }) => {
    updateExtractedInfo(info);
  },
  'phase_change': ({ from, to }) => {
    updatePhase(to);
  },
  'title.generated': ({ conversationId, title }) => {
    updateSessionTitle(conversationId, title);
    refreshSessionList();
  },
  'message.completed': ({ messageId, usage }) => {
    updateMessageStatus(messageId, 'completed');
  },
  'done': () => {
    cleanup();
  },
};
```

### 8.8 文件结构变更

```
apps/web/src/features/consultation/
├── components/
│   ├── AssistantChatPanel.tsx       // 适配新协议
│   ├── SessionHistorySidebar.tsx   // 新增：三区布局侧边栏
│   ├── SessionCard.tsx             // 新增：扁平化会话卡片
│   ├── InfoPanel.tsx               // 保留
│   ├── DiagnosisPanel.tsx          // 保留
│   ├── SharePage.tsx               // 新增：分享页面
│   └── ui/                         // 新增：shadcn/ui 组件
│       ├── dropdown-menu.tsx
│       ├── dialog.tsx
│       ├── sonner.tsx
│       └── button.tsx
├── hooks/
│   ├── useAssistantChatRuntime.ts  // 重写
│   ├── useChatSSE.ts               // 重写
│   └── useSSEProcessor.ts          // 新增
├── pages/
│   └── ConsultationPage.tsx        // 改造
├── services/
│   └── consultationService.ts      // 重写
└── types/
    └── consultation.ts             // 重写
```

---

## 九、移动端适配

- 移动端抽屉与桌面端侧边栏保持一致的三区布局
- 卡片上的置顶和更多按钮始终显示（不依赖 hover）
- 下拉菜单、确认对话框、内联编辑与桌面端一致

---

## 十、实施路线图

### 阶段 1：数据层（Go API）

**目标：** 新 schema + repository 层就绪

- [x] 数据库迁移：conversations、messages、runs、conversation_shares、consultation_sessions
- [x] Model 层：conversation.go、message.go、run.go、consultation_session.go、conversation_share.go
- [x] UUID 方案确定（升级：使用 PostgreSQL 18 原生 `uuidv7()` 进行数据库级的主键生成，完美融合有序主键与高性能 B-Tree 写入）
- [x] Repository 层：conversation、message、run、consultation、share
- [x] Service 层：conversation、message、run、consultation
- [x] 置顶/分享 Repository + Service

**验证：** 单元测试通过，CRUD 正常

### 阶段 2：Go API Handler 层

**目标：** 新 API 端点 + SSE 协议

- [x] `handler/chat_handler.go` — SendMessage（幂等 + 事务 + SSE）
- [x] `handler/conversation_handler.go` — CRUD + 置顶 + 分享 + 标题
- [x] `handler/consultation_handler.go` — 咨询领域操作
- [x] `handler/diagnosis_handler.go` — 诊断 + 治疗
- [x] `handler/sse_writer.go` — SSE 事件写入封装
- [x] `service/ai_client.go` — Python AI 服务客户端（NDJSON 解析）
- [x] 路由注册

**验证：** SSE 流正确返回，幂等检查工作，置顶/分享 API 正常

### 阶段 3：Python AI 服务改造

**目标：** 多 Provider 路由 + NDJSON 输出

- [x] 创建 `ai/` 目录结构
- [x] 实现 types.py、errors.py、config.py
- [x] 实现 providers/base.py、providers/openai_compatible.py
- [x] 实现 router.py（ModelRouter + CircuitBreaker 熔断器）
- [x] 实现 service.py（AIService 统一入口）
- [x] 编写 config/models.yaml 配置
- [x] 改造 `/api/chat/stream` 输出 NDJSON
- [x] 迁移 consultation_graph → AIService
- [x] 迁移 diagnosis_service → AIService + 删除 _parse_json
- [x] 迁移 assessment_service、reassessment_service → AIService
- [x] 迁移 video_pipeline (及其子模块 ai_splitter.py 和 ai_curator.py) → AIService
- [x] 删除 llm_provider.py

**验证：** Python 独立测试通过，Go 可正确解析事件流，429 熔断 + fallback 正常

### 阶段 4：前端改造

**目标：** 适配新协议 + 侧边栏重设计 + shadcn/ui

- [x] 引入 shadcn/ui 并初始化组件（调整：组件文件被提升至全局共享的 `apps/web/src/components/ui/`）
- [x] 类型定义重写
- [x] consultationService.ts 重写
- [x] useSSEProcessor.ts 新增
- [x] useChatSSE.ts 逻辑合并（调整：不单独建立文件，其逻辑直接并入 `useAssistantChatRuntime.ts` 与 `useSSEProcessor.ts` 中）
- [x] useAssistantChatRuntime.ts 重写 (对接 @assistant-ui/react 运行时)
- [x] SessionHistorySidebar + SessionCard 组件（侧边栏置顶/分享/重置等操作）
- [x] ConsultationPage 改造（懒创建与路由对接）
- [x] 分享页面 SharePage
- [x] 删除废弃组件

**验证：** 新建 → 对话 → 流式回复 → URL 自动变化；侧边栏置顶/分享/重命名正常

### 阶段 5：集成测试与打磨

- [x] 端到端测试（完整流程验证）
- [x] 幂等性测试（相同 RequestId 重复请求验证）
- [x] 错误处理（429 速率限制、熔断切换测试）
- [x] 断线恢复机制
- [x] 移动端适配与布局微调
- [x] 清理并下线旧代码/旧表

### 依赖关系

```
阶段 1 (数据层)
  │
  ├──► 阶段 2 (Go API) ──► 阶段 5 (集成测试)
  │         │
  │         ├──► 阶段 3 (Python AI) ──┘
  │         │
  │         └──► 阶段 4 (前端) ──┘
```

**关键路径：** 阶段 1 → 阶段 2 → 阶段 4
**可并行：** 阶段 3 可在阶段 2 进行中开始

### 预估工作量

| 阶段 | 预估 | 风险 |
|------|------|------|
| 1. 数据层 | 1-2 天 | 低 |
| 2. Go API | 2-3 天 | 中 — SSE + 幂等 + 事务 |
| 3. Python AI | 2-3 天 | 中 — 多 Provider + 协议改造 |
| 4. 前端 | 2-3 天 | 中 — SSE 适配 + shadcn/ui |
| 5. 集成测试 | 1-2 天 | 低 |
| **总计** | **8-13 天** | |

---

## 十一、风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| shadcn/ui 与现有 Tailwind 配置冲突 | 样式不一致 | 初始化时仔细映射 CSS 变量 |
| YAML 解析引入 pyyaml 依赖 | 新依赖 | PyYAML 是 Python 生态标准库 |
| 流式 fallback 时机 | 已输出后无法 fallback | 仅首个 chunk 前做 fallback |
| `response_format=json_object` 兼容性 | 部分模型不支持 | capabilities 声明中检查 json_mode |
| 测试迁移工作量 | mock 接口变化 | mock AIService 而非 provider 内部 |
| NDJSON vs SSE 协议混淆 | Go ↔ Python 接口错误 | 严格按契约文档实现 |

---

## 十二、未来演化

1. **Scored selection**：provider 数量 > 3 时加打分路由
2. **Quota tracking**：Redis 记录 token 用量和错误率
3. **Anthropic/Gemini provider**：实现新的 AiProvider
4. **Embedding 路由**：rag/embedding.py 纳入路由系统
5. **结构化输出 Schema**：`response_format: json_schema`
6. **Prompt 版本管理**：YAML/Jinja2 模板
7. **用户自选模型**：`conversations.default_model` 生效
