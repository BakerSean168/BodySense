# Stream Event Contract Runtime 架构设计

> **ADR 0002 交叉引用**：StreamEvent 契约设计、channel 分类和 replay 概念仍然有效。ADR 0002 确认 "Public SSE/Web stream events are derived from runtime events and remain the only frontend contract"。但事件的产生点已从 `ChatHandler` 转移到 consultation thread runtime（Go `consultation/runtime.go` → Python Agent Runtime → Go Runtime Event Log → Stream Contract）。文档中对 `ChatHandler` 的引用为历史参考。

**文档版本**：v1.0
**更新日期**：2026-06-29
**状态**：设计稿（事件契约仍有效，产出点已迁移）
**适用范围**：咨询 SSE、工具事件、上下文事件、Job 进度、前端消息渲染

---

## Implementation Status

**当前状态**：部分实现（~30%）

| 模块 | 状态 | 说明 |
|---|---|---|
| Go StreamRuntime 模块 | ✅ 已完成 | StreamEvent 验证、ID enrichment、sequence 分配、SSE 写入。Phase 01b 完成（审查误报，模块已存在）。 |
| SSEWriter | ✅ 已完成 | 独立 SSE 写入器。 |
| StreamEvent v1 契约 | ✅ 已完成 | packages/contracts 定义，JSON schema 存在。 |
| interaction_id 扩展 | ✅ 已完成 | StreamEvent ID 中添加 interaction_id。 |
| StreamEventReducer | 部分实现 | 前端 reducer 基础结构存在，pendingInteractions 扩展未完成。Phase 01c 部分完成。 |
| 事件命名规范化 | 部分实现 | 部分事件已按规范命名，部分仍为旧格式。 |
| 事件持久化 / 回放 | 未实现 | replayable event persistence。 |
| 完整 channel 分类 | 未实现 | message/state/tool/job/debug 等 channel 尚未完全分离。 |
| 前端事件处理策略 | 未实现 | strict vs. optional vs. idempotent 状态事件策略。 |

**相关 Phase**：01b, 01c → 归档于 `docs/plan/archive/implementation/`

---

## 1. 背景

BodySense 已经通过 SSE 支撑咨询工作台的流式体验，并且已有共享包：

```txt
packages/contracts/src/stream-events.ts
packages/contracts/schemas/stream-event.v1.schema.json
```

现有事件包括：

```txt
message.text.delta
message.completed
tool.call
tool.result
state.extracted_info.upsert
state.phase.changed
source.citation.added
source.knowledge_gap
safety.red_flag.detected
usage.reported
stream.done
stream.error
```

随着上下文工程化、工具调用工程化、Job Runtime 加入，事件会继续增长：

```txt
state.consultation.patch
debug.context_trace
tool.interrupted
state.interaction.required
job.progress
job.completed
```

如果事件只是“能发 JSON”，很快会出现：

- Python、Go、TypeScript 三处类型不一致。
- 前端处理未知事件时行为不可控。
- Go 转发 Python 事件时无法校验。
- SSE 重连后缺少事件回放语义。
- 测试只能靠端到端，不容易定位是哪层事件错。

本设计目标是把 Stream Event 升级为正式的 **Event Contract Runtime**。

---

## 2. 设计目标

1. **单一事件契约**
   所有客户端可见事件以 `@bodysense/contracts` 为准。

2. **跨语言一致**
   TypeScript、Go、Python 使用同一语义模型，避免手写漂移。

3. **事件可校验**
   Go 在转发 Python 事件前能校验 shape、version、channel、type。

4. **事件可回放**
   对关键 run/job 支持基于 `seq` 的事件恢复。

5. **事件可测试**
   每种事件有 fixture，Python/Go/前端共享测试样例。

6. **兼容演进**
   新事件有版本策略，旧前端遇到未知事件不会崩溃。

---

## 3. 事件分层

```txt
Internal Provider Event
  - provider 私有 streaming chunk
  - 只存在 Python provider adapter 内部

Agent / Job Internal Event
  - Python AgentEvent
  - Go JobEvent
  - 面向内部 Module

Public StreamEvent v1
  - 客户端可见契约
  - Go 对外 SSE 只发送这一层
```

原则：

- LLM provider 私有事件不能直接出现在前端。
- Python graph event 不能绕过 Go 直接成为前端真值。
- Go 对外发送的事件必须符合 `StreamEvent v1`。

---

## 4. 标准事件结构

```ts
export interface StreamEventBase<
  TChannel extends StreamChannel,
  TType extends string,
  TPayload,
> {
  version: 'v1';
  channel: TChannel;
  type: TType;
  seq: number;
  ids: StreamEventIds;
  payload: TPayload;
  timestamp: string;
}
```

事件字段语义：

| 字段 | 说明 |
|---|---|
| `version` | 契约版本，第一版固定 `v1` |
| `channel` | 粗粒度事件域，例如 `message`、`state`、`tool` |
| `type` | 稳定事件名 |
| `seq` | 当前 stream 内单调递增序号 |
| `ids` | conversation、message、run、tool_call、job 等关联 id |
| `payload` | 事件载荷 |
| `timestamp` | Go 发出或 Python 生成时间 |

---

## 5. Channel 分类

建议扩展为：

```ts
export type StreamChannel =
  | 'conversation'
  | 'message'
  | 'state'
  | 'tool'
  | 'source'
  | 'safety'
  | 'usage'
  | 'job'
  | 'debug'
  | 'stream';
```

约束：

- `message`：只表示聊天消息生命周期。
- `state`：表示业务状态变化。
- `tool`：表示工具调用和工具结果。
- `job`：表示后台任务状态。
- `debug`：开发期可见，生产可只写日志。
- `stream`：流生命周期和错误。

---

## 6. Runtime Module 设计

### 6.1 TypeScript Contract Package

**位置**：`packages/contracts/src/stream-events.ts`

职责：

- 定义所有事件类型。
- 导出事件 union。
- 导出 type guard。
- 导出事件常量。

建议结构：

```txt
packages/contracts/src/
  stream-events.ts
  stream-event-types.ts
  stream-event-guards.ts
  stream-event-fixtures.ts
```

### 6.2 JSON Schema

**位置**：`packages/contracts/schemas/stream-event.v1.schema.json`

职责：

- 给 Go / Python runtime 校验使用。
- CI 校验 fixtures。
- 未来可生成 OpenAPI 补充文档。

### 6.3 Go Event Runtime

**位置建议**：`apps/api/internal/stream/`

```txt
apps/api/internal/stream/
  event.go
  writer.go
  validator.go
  mapper.go
  replay.go
```

职责：

- 构建 `StreamEvent`。
- 校验 Python 传入事件。
- 分配或重写 `seq`。
- 持久化可回放事件。
- 写 SSE。

### 6.4 Python Event Runtime

**位置建议**：`apps/ai-service/src/models/stream_event.py`

职责：

- 构建 Python 内部 `StreamEvent`。
- 将 graph / tool / job 内部事件映射为 public-like event。
- 在测试中用 schema 校验输出。

注意：Python 可以产出符合 public shape 的事件，但 Go 仍是对外事件最终出口。

### 6.5 Frontend Event Processor

**位置建议**：`apps/web/src/features/consultation/hooks/useSSEProcessor.ts`

职责：

- 解析 SSE。
- 根据 `event.type` 分发。
- 对未知事件降级处理。
- 维护 message parts、state patch、job progress。

---

## 7. Go 转发规则

Go 从 Python 收到事件后：

```txt
Python NDJSON event
  -> decode
  -> validate known event shape
  -> enrich ids
  -> assign outbound seq
  -> persist if replayable
  -> write SSE
```

Go 可以拒绝：

- `version` 不支持。
- `channel/type` 未注册。
- `payload` shape 不合法。
- `ids.conversation_id` 与当前 conversation 不一致。

Go 可以补全：

- `conversation_id`
- `message_id`
- `run_id`
- `timestamp`
- `seq`

---

## 8. 可回放事件

不是所有事件都必须落库。

建议分类：

| 事件 | 是否回放 | 原因 |
|---|---|---|
| `message.text.delta` | 可选 | 可由最终 message parts 恢复 |
| `message.completed` | 是 | 消息生命周期 |
| `state.*` | 是 | 前端状态恢复 |
| `tool.call` / `tool.result` | 是 | 审计和调试 |
| `state.interaction.required` | 是 | 刷新后恢复交互 |
| `job.*` | 是 | 任务进度 |
| `usage.reported` | 是 | 成本统计 |
| `debug.*` | 否或日志 | 生产不必发客户端 |

可新增表：

```sql
CREATE TABLE stream_events (
    id              UUID PRIMARY KEY DEFAULT uuidv7(),
    conversation_id UUID REFERENCES conversations(id) ON DELETE CASCADE,
    run_id          UUID REFERENCES runs(id) ON DELETE SET NULL,
    job_id          UUID REFERENCES jobs(id) ON DELETE CASCADE,
    seq             INT NOT NULL,
    channel         VARCHAR(40) NOT NULL,
    type            VARCHAR(120) NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    ids             JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 9. 前端处理策略

### 9.1 严格事件

必须支持：

```txt
message.text.delta
message.completed
message.failed
stream.done
stream.error
```

这些事件处理失败会直接影响聊天体验。

### 9.2 可选事件

可降级：

```txt
source.citation.added
source.knowledge_gap
usage.reported
debug.context_trace
```

未知可选事件只记录，不打断 UI。

### 9.3 状态事件

必须幂等：

```txt
state.extracted_info.upsert
state.consultation.patch
state.phase.changed
state.interaction.required
```

前端不能假设事件只到一次。刷新恢复、SSE 重连、replay 都可能重复。

---

## 10. 事件命名规范

格式：

```txt
{domain}.{noun}.{verb}
```

或已有简化形式：

```txt
message.text.delta
stream.done
stream.error
```

推荐：

```txt
job.created
job.progress
job.completed
job.failed
tool.interrupted
state.interaction.required
state.consultation.patch
debug.context_trace
```

避免：

```txt
ask_user_required
newToolResult
done2
custom_event
```

---

## 11. 测试策略

### 11.1 Contract Fixtures

新增：

```txt
packages/contracts/fixtures/stream-events/
  message.text.delta.json
  tool.call.json
  state.interaction.required.json
  job.progress.json
```

### 11.2 Go 测试

```txt
apps/api/internal/stream/validator_test.go
apps/api/internal/stream/writer_test.go
apps/api/internal/consultation/runtime_test.go  // 替代旧 chat_handler_test.go
```

覆盖：

- Python 事件 shape 校验。
- Go 补全 ids。
- seq 单调递增。
- 未知事件降级或拒绝。

### 11.3 Python 测试

```txt
apps/ai-service/tests/unit/test_stream_event.py
```

覆盖：

- StreamEventFactory 生成合法事件。
- graph event 正确映射为 public event。
- tool interrupt 事件符合 schema。

### 11.4 前端测试

```txt
apps/web/src/features/consultation/hooks/useSSEProcessor.test.ts
```

覆盖：

- 每个事件调用正确 handler。
- 未知事件不抛异常。
- 重复 state event 幂等。

---

## 12. 分阶段落地

### Phase 1：固定事件命名和 schema

- 扩展 `@bodysense/contracts`。
- 增加 fixtures。
- 保持现有事件行为。

### Phase 2：Go Stream Runtime

- 从 consultation runtime 中抽出 writer / validator / mapper（原 ChatHandler 已删除）。
- Go 统一补 `seq` 和 ids。

### Phase 3：Python Event Runtime 对齐

- Python 输出前通过模型约束。
- graph event 到 public event 映射集中到一个 Module。

### Phase 4：前端 Processor 强化

- 事件分发从 map 扩展为 typed handlers。
- 增加 unknown event 策略。

### Phase 5：Replay

- 关键事件落库。
- 支持 `Last-Event-ID` 或 `?after_seq=` 恢复。

---

## 13. 成功标准

落地后应满足：

1. 新增事件必须先改 contracts。
2. Go、Python、前端对同一事件的理解一致。
3. Python 发错事件时 Go 能拦截。
4. 前端刷新后能恢复 pending interaction 和 job progress。
5. SSE 重连不会造成重复错误状态。
6. 事件 fixture 可以作为跨语言测试样本。

