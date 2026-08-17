# BodySense 上下文工程架构

> **Historical context-engineering reference.** Retired stream events and session health projections are not current contracts. See [Current Longitudinal System](./current-longitudinal-system.md).


> **⚠️ HISTORICAL — 本方案已被 ADR 0002 完全取代。**
>
> 本文档描述的 Go 侧 `ContextBuilder` + `ChatHandler` 上下文组装方案已被删除。当前实现中 Python 通过 LangGraph checkpoint 拥有 Agent Thread 运行时真值，Go 仅负责 Runtime Event Log 和 projection 持久化。详见：
> - [`docs/adr/0002-agent-runtime-ownership.md`](../adr/0002-agent-runtime-ownership.md)
> - [`docs/plan/active/final-agent-runtime-architecture.md`](../plan/active/final-agent-runtime-architecture.md)
>
> 以下正文保留作为设计演进历史参考。

**文档版本**：v1.0
**更新日期**：2026-06-29
**状态**：已归档（superseded by ADR 0002）  
**关联文档**：[技术方案](./technical-approach.md)、[会话管理统一重设计](../plan/archive/unified-session-redesign.md)、[Go-Python 契约](../plan/archive/04-python-contract.md)、[SSE 协议](../plan/archive/03-sse-protocol.md)

---

## Implementation Status

**当前状态**：部分实现（~50%）

| 模块 | 状态 | 说明 |
|---|---|---|
| ContextBuilder（Go 侧上下文组装） | 部分实现 | 上下文组装逻辑仍在 ChatHandler 内联，未提取为独立模块。Phase 01a 未完成。 |
| ask_user 工具契约 | ✅ 已完成 | Python ask_user 工具定义、ToolResult(interrupted)、interaction_id 流转。Phase 03a 完成。 |
| agent_interactions 持久化 + Resume API | ✅ 已完成 | Go 侧 agent_interactions 表、resume 端点、pending/answered 状态机。Phase 03b 完成。 |
| AskUserCard UI | 未实现 | 前端 AskUserCard 组件、StreamEventReducer pendingInteractions 扩展。Phase 03c 未完成。 |
| Go-to-Python Resume 直连 | 未实现 | 当前需前端发新消息触发 resume。 |

**相关 Phase**：01a, 03a, 03b, 03c → 归档于 `docs/plan/archive/implementation/`

---

## 1. 背景

BodySense 的咨询工作台不是普通聊天应用，而是一个有业务状态的问诊 Agent。系统需要在多轮对话中持续维护：

- 用户当前主诉、症状细节、体态问题和已确认信息
- 缺失字段，例如疼痛程度、持续时间、诱因、是否肿胀、是否外伤
- 红旗风险和安全提示
- 当前问诊阶段、诊断候选、康复方案
- RAG 检索结果、知识引用和知识缺口
- 前端右侧结构化面板、人体高亮和用户手动修正状态

这些上下文不能只依赖 `messages` 拼接，也不能完全交给模型或框架自动记忆。框架能提供状态机、短期记忆、checkpoint、tool orchestration，但不知道 BodySense 业务里什么信息更重要，什么字段缺失，什么状态应该被用户确认，什么历史应该被过滤。

因此，本项目的上下文工程目标是：

> 用明确的业务 Context Layer 管理问诊上下文，用 LangGraph 管理 Python 侧推理流程，而不是让 LangGraph 成为业务数据的第二套真相源。

---

## 2. 核心决策

### 2.1 继续使用 LangGraph，但限制其职责

BodySense 继续使用 LangGraph 作为 Python AI Service 内部的 Agent workflow 框架。

LangGraph 负责：

- 节点化问诊流程
- 症状抽取、红旗检测、意图判断、知识检索、回答生成的编排
- 本轮 graph state 的读写
- tool call loop 和结构化事件发射
- 开发期调试、time travel、临时 checkpoint 等可选能力

LangGraph 不负责：

- 作为正式会话数据库
- 替代 `messages` 表
- 替代 `consultation_sessions`
- 保存最终业务状态
- 直接决定客户端 SSE 协议
- 直接写入 PostgreSQL

### 2.2 Go API 是业务上下文权威编排器

Go API 仍然是业务上下文的权威编排器。

Go 负责：

- 用户认证和权限校验
- `conversations` / `messages` / `runs` 生命周期
- `consultation_sessions` 领域状态
- 每轮请求前的 ContextBuilder
- 客户端 SSE 协议生产
- Python 返回事件的校验、映射和落库
- 幂等、失败、取消、断连后的状态处理

### 2.3 PostgreSQL 是唯一业务真相源

PostgreSQL 是正式业务状态的唯一 source of truth。

核心持久化位置：

| 数据 | 真相源 |
|------|--------|
| 对话元信息 | `conversations` |
| 单条消息 | `messages` |
| 执行记录 | `runs` |
| 问诊领域状态 | `consultation_sessions` |
| 用户长期画像 | `user_profiles` |
| 知识库 | `knowledge_entries` / pgvector |

LangGraph checkpoint 可以在开发或单次长任务中辅助恢复，但不能成为正式问诊状态的来源。

---

## 3. 总体架构

```text
Frontend
  - 渲染对话流
  - 渲染结构化问诊事件
  - 渲染人体高亮
  - 用户确认/编辑/回答结构化问题
        |
        | POST /api/v1/chat/send
        v
Go API
  - 创建 turn/run/message
  - ContextBuilder 组装上下文
  - 调用 Python NDJSON stream
  - 映射为客户端 SSE
  - 校验并落库 state patch
        |
        | POST /api/chat/stream
        v
Python AI Service
  - LangGraph consultation_graph
  - extract / merge / red_flag / decide / retrieve / generate
  - 返回结构化 NDJSON events
        |
        v
PostgreSQL + pgvector
  - 会话、消息、问诊状态、用户画像、知识库
```

设计原则：

```text
Go 管上下文真相
Python 管推理流程
LangGraph 管本轮 workflow
PostgreSQL 管正式持久化
Frontend 管交互呈现和用户确认
```

---

## 4. 上下文分层模型

每轮模型上下文由 6 层组成：

```text
模型上下文 =
  1. System / Developer Prompt
  2. Recent Messages
  3. Conversation Summary
  4. Consultation State
  5. User Profile / Long-term User Context
  6. Retrieved Knowledge / Tool Results
```

### 4.1 System / Developer Prompt

由 Python prompt 模板维护，包含：

- BodySense 角色定位
- 问诊边界和安全声明
- 红旗症状处理原则
- 知识库使用原则
- 结构化追问原则
- 右侧面板状态优先级规则

重要约束：

> 当历史消息与当前结构化问诊状态冲突时，以当前结构化问诊状态为准。

### 4.2 Recent Messages

Go ContextBuilder 从 `messages` 表读取最近消息。

必须过滤：

- 当前 turn 的 user message，避免重复传入
- `streaming` assistant placeholder
- `failed` message
- `aborted` message
- 纯 UI event
- 过长 tool result
- 半截 SSE 内容

建议默认策略：

```text
最近 8-10 轮：保留原文
更早历史：使用 conversation.summary
```

### 4.3 Conversation Summary

`conversations.summary` 用于长会话压缩。

summary 应保留：

- 已经确认的主诉
- 关键症状演变
- 用户否认过的重要风险项
- 已完成的自测结论
- 已解释过的知识点

summary 不应保留：

- 过时且已被用户修正的信息
- 失败工具调用的原始输出
- 长篇知识库 chunk
- 临时 UI 状态

### 4.4 Consultation State

这是问诊 Agent 最重要的业务上下文层。它不能只是一组聊天记录，也不能只是一组松散的 `extracted_info`。

建议结构：

```json
{
  "stage": "collecting",
  "symptoms": [
    {
      "id": "sym_01",
      "body_part": "knee",
      "symptom_type": "下坠感",
      "duration": "一周",
      "trigger": "走路明显",
      "severity": null,
      "swelling": null,
      "trauma": null,
      "relief": null,
      "confidence": 0.82,
      "source_turn_id": "turn_...",
      "confirmed": false,
      "updated_by": "ai"
    }
  ],
  "missing_slots": ["severity", "swelling", "trauma"],
  "red_flags": [],
  "active_question": null,
  "last_state_revision": 7
}
```

### 4.5 User Profile

用户画像来自 `user_profiles`，属于长期上下文。

典型字段：

- 年龄、性别、身高、体重
- 职业和久坐情况
- 运动习惯
- 既往伤病
- 上传材料 OCR 摘要
- 长期训练偏好

用户画像不应混入当前 session 的短期问诊状态。

### 4.6 Retrieved Knowledge

知识上下文来自 pgvector / RAG。

原则：

- 默认由 Python LangGraph 的 `search_knowledge` tool 按需检索
- 不把大量 RAG chunk 长期放进消息历史
- 长工具结果只存数据库或日志，本轮 prompt 只放摘要
- 每个关键建议应尽量带 citation
- 无匹配时发出知识缺口事件，而不是伪造依据

---

## 5. ContextBuilder

Go 侧需要抽出独立 `ContextBuilder`，替代 handler 中散落的上下文拼装逻辑。

### 5.1 输入

```text
user_id
conversation_id
turn_id
current_user_message
request_context
```

### 5.2 输出

```go
type ConsultationContextBundle struct {
    SessionID           string
    TurnID              string
    UserID              string
    UserMessage         string
    RecentMessages      []ChatMessage
    ConversationSummary string
    ConsultationState   ConsultationState
    UserProfile         any
    ContextTrace        ContextTrace
}
```

### 5.3 职责

ContextBuilder 负责：

- 加载 profile
- 加载 consultation session
- 加载 recent messages
- 执行 message filtering
- 注入 conversation summary
- 计算 token 预算估计
- 生成 context trace
- 构造 Go -> Python 请求 payload

### 5.4 过滤规则

```text
include:
  - previous turn
  - status = completed
  - role in user / assistant
  - content_text 不为空

exclude:
  - current turn
  - status = streaming / failed / aborted
  - tool-only message
  - oversized tool result
  - canceled response
```

---

## 6. Go -> Python Context Contract

建议将 Python 请求从扁平字段升级为显式上下文契约。

### 6.1 请求结构

```json
{
  "session_id": "conv_...",
  "turn_id": "turn_...",
  "user_id": "user_...",
  "user_message": "走路时明显",
  "use_case": "consultation.reply",
  "context": {
    "recent_messages": [
      {"role": "user", "content": "我的膝盖有下坠感"},
      {"role": "assistant", "content": "这种感觉大概持续多久了？"}
    ],
    "conversation_summary": "",
    "consultation_state": {
      "stage": "collecting",
      "symptoms": [
        {
          "body_part": "knee",
          "symptom_type": "下坠感",
          "duration": "一周",
          "trigger": null
        }
      ],
      "missing_slots": ["severity", "swelling", "trauma"],
      "red_flags": []
    },
    "user_profile": {},
    "context_trace": {
      "included_message_count": 2,
      "included_summary": false,
      "filtered_current_turn": true,
      "filtered_failed_messages": 0,
      "token_estimate": 1800
    }
  }
}
```

### 6.2 兼容策略

当前 Python 入口已支持：

```text
messages
extracted_info
phase
profile
rag_results
```

升级时可以先在 Python 侧同时接受旧字段和新 `context` 字段。MVP 活跃期不要求长期双轨兼容，迁移完成后删除旧字段。

---

## 7. Python LangGraph State

LangGraph state 应表达“本轮推理工作内存”，而不是数据库 schema 的一比一复制。

建议结构：

```python
class ConsultationGraphState(TypedDict, total=False):
    session_id: str
    turn_id: str
    user_id: str
    user_message: str

    recent_messages: list[dict[str, Any]]
    conversation_summary: str
    consultation_state: dict[str, Any]
    user_profile: dict[str, Any]

    extracted_patch: dict[str, Any]
    missing_slots: list[str]
    red_flags: list[dict[str, Any]]
    retrieved_knowledge: list[dict[str, Any]]

    next_action: str
    ui_events: list[dict[str, Any]]
    response_text: str
```

### 7.1 推荐图拓扑

```text
START
  -> normalize_context
  -> extract_symptom_patch
  -> merge_consultation_state
  -> red_flag_check
  -> decide_next_action
      -> ask_followup
      -> search_knowledge
      -> generate_diagnosis
      -> generate_treatment
      -> safety_response
  -> emit_state_patch
  -> generate_response
  -> END
```

### 7.2 节点职责

| 节点 | 输入 | 输出 |
|------|------|------|
| `normalize_context` | Go context bundle | 标准化 graph state |
| `extract_symptom_patch` | 当前消息 + state | 新增/修正的症状 patch |
| `merge_consultation_state` | patch + old state | 本轮候选 state |
| `red_flag_check` | symptoms + 当前消息 | red flag events |
| `decide_next_action` | missing slots + phase + intent | `ask_followup` / `retrieve` / `diagnose` / `treat` |
| `ask_followup` | missing slots | structured ask event |
| `search_knowledge` | query | citations / knowledge gap |
| `generate_response` | full state | 流式文本 |
| `emit_state_patch` | graph state diff | state patch event |

---

## 8. Structured Ask

自然语言追问容易重复、难以落库，也不利于前端 UI 状态同步。因此缺失字段应优先通过结构化事件询问。

### 8.1 事件类型

新增客户端 SSE 事件：

```text
ui.ask_user_options
```

### 8.2 示例

```json
{
  "type": "ui.ask_user_options",
  "payload": {
    "question_id": "q_pain_severity",
    "field": "symptoms[0].severity",
    "question": "走路时不适程度大概是？",
    "options": [
      {"label": "轻微", "value": "mild"},
      {"label": "明显", "value": "moderate"},
      {"label": "严重", "value": "severe"}
    ],
    "allow_free_text": true
  }
}
```

### 8.3 回写路径

```text
Frontend 用户点击选项
  -> Go API 接收结构化回答
  -> 更新 consultation_state
  -> 下一轮 ContextBuilder 注入最新 state
  -> Python LangGraph 继续推理
```

Structured ask 不是聊天装饰，而是让 Agent 主动获取缺失上下文的正式协议。

---

## 9. SSE 事件扩展

当前已有事件：

- `state.extracted_info.upsert`
- `state.phase.changed`
- `state.diagnosis.ready`
- `state.treatment.ready`
- `source.citation.added`
- `source.knowledge_gap`
- `safety.red_flag.detected`

建议新增：

| 事件 | channel | 用途 |
|------|---------|------|
| `state.consultation.patch` | `state` | Python 提议更新 consultation state |
| `ui.ask_user_options` | `ui` | 结构化追问 |
| `debug.context_trace` | `debug` | 开发期上下文可观测性 |

### 9.1 State Patch

```json
{
  "channel": "state",
  "type": "state.consultation.patch",
  "payload": {
    "patch": {
      "symptoms": [
        {
          "body_part": "knee",
          "trigger": "走路明显"
        }
      ],
      "missing_slots": ["severity", "swelling", "trauma"]
    },
    "base_revision": 6
  }
}
```

Go 接收后必须：

- 校验 patch schema
- 校验 revision
- 合并到正式 `consultation_sessions`
- 必要时拒绝或降级
- 再转发给前端

### 9.2 Context Trace

```json
{
  "channel": "debug",
  "type": "debug.context_trace",
  "payload": {
    "included_message_count": 8,
    "included_summary": true,
    "included_state_fields": ["symptoms", "missing_slots", "red_flags"],
    "filtered_current_turn": true,
    "filtered_failed_messages": 1,
    "retrieved_doc_count": 3,
    "token_estimate": 4200
  }
}
```

生产环境可以只写日志，不必发给客户端。

---

## 10. 状态合并规则

上下文工程的关键不是“保存更多信息”，而是可靠合并状态。

### 10.1 优先级

```text
用户手动确认/编辑 > 当前 structured ask 回答 > 最新用户消息抽取 > 旧 AI 抽取 > 历史 summary
```

### 10.2 合并原则

- 用户确认的信息不得被低置信度 AI 抽取覆盖
- 用户删除的信息必须从后续上下文中移除或标记为 inactive
- AI 可以提出 patch，但 Go 负责最终合并和落库
- 同一 body part 的症状应有稳定 ID，不应只按中文名称合并
- 每次合并应增加 `last_state_revision`
- 关键字段应记录 `source_turn_id` 和 `updated_by`

### 10.3 冲突处理

当历史消息与结构化 state 冲突：

```text
以 consultation_state 为准
在 prompt 中明确告知模型忽略过时历史
必要时把冲突写入 context_trace
```

---

## 11. 长短期记忆边界

### 11.1 Session 级上下文

属于当前咨询会话：

- recent messages
- conversation summary
- consultation state
- diagnosis
- treatment plan
- red flags
- active question

真相源：

```text
conversations
messages
consultation_sessions
```

### 11.2 User 级长期上下文

跨咨询复用：

- 年龄、性别、身高、体重
- 职业习惯
- 运动习惯
- 既往伤病
- 长期偏好
- 历史问诊摘要

真相源：

```text
user_profiles
未来可扩展 user_memory
```

### 11.3 Knowledge 级上下文

跨用户复用：

- 体态知识
- 动作训练知识
- 视频片段
- 引用来源
- 知识质量标记

真相源：

```text
knowledge_entries
pgvector
curated source files
```

---

## 12. LangGraph Checkpointer 使用边界

LangGraph checkpointer 适合保存 thread-scoped graph state，但在 BodySense 中不应成为主存储。

### 12.1 可以使用

- 本地开发调试
- 图执行 time travel
- 人机中断 / resume 的临时恢复
- 单次复杂长 run 的容错
- 测试不同节点输出

### 12.2 不应使用

- 保存正式问诊状态
- 保存正式消息历史
- 保存用户长期画像
- 替代 Go 的幂等和 run 管理
- 替代 PostgreSQL 审计链路

### 12.3 原因

如果 Python checkpoint 同时保存业务状态，会出现：

- Go DB 和 Python checkpoint 双写
- 断连和重试时状态难以对齐
- 前端右侧面板和 Python 内部状态不一致
- 审计和回放复杂化
- Python 服务水平扩展成本增加

---

## 13. 落地路线

### Phase 1：抽 ContextBuilder

- 从 chat handler 中抽出 Go `ContextBuilder`
- 固化 message filtering 策略
- 生成 `ContextTrace`
- 添加单元测试覆盖 current turn / failed / streaming 过滤

### Phase 2：升级 Consultation State

- 将 `extracted_info` 从症状 list 升级为完整 `consultation_state`
- 引入 `symptoms`、`missing_slots`、`red_flags`、`active_question`
- 明确状态合并规则和 revision

### Phase 3：升级 Go -> Python 契约

- Python `ChatRequest` 新增 `context` bundle
- LangGraph 使用 `consultation_state` 而不是仅使用 `extracted_symptoms`
- 迁移完成后删除旧扁平字段

### Phase 4：增加结构化追问

- 新增 `ui.ask_user_options` 事件
- 前端渲染选项按钮或表单
- 用户回答直接回写 `consultation_state`

### Phase 5：增加 State Patch 和 Trace

- 新增 `state.consultation.patch`
- Go 校验 patch 后落库
- 增加 `debug.context_trace` 日志

### Phase 6：长会话摘要

- 使用 `conversations.summary`
- 最近消息 + summary + structured state 共同组成上下文
- 对 summary 过期和冲突做 trace

---

## 14. 成功标准

上下文工程落地后，应满足：

- 用户补充“走路明显”时，模型能关联上一轮“膝盖下坠感”
- 模型不会重复询问已经结构化收集过的字段
- 用户在右侧面板删除/修改症状后，下一轮模型以右侧状态为准
- failed / aborted / streaming message 不进入模型上下文
- 长会话中模型仍能保留核心问诊事实
- 红旗风险不会被 summary 或旧消息覆盖
- 每轮能通过 context trace 解释“模型看到了什么”
- Go DB、前端 UI、Python graph state 三者不会出现长期不一致

---

## 15. 结论

BodySense 的上下文工程不应理解为“给模型传更多历史”，而应理解为：

> 以 Go ContextBuilder 和 PostgreSQL 为业务真相源，以 LangGraph 为 Python 推理状态机，以结构化事件连接前端交互，形成可过滤、可追踪、可合并、可审计的问诊上下文系统。

最终目标不是让系统更像聊天机器人，而是让它成为：

```text
有状态
有记忆
有流程
有结构化问诊上下文
可被用户修正
可被工程排查
```

的体态健康咨询 Agent。
