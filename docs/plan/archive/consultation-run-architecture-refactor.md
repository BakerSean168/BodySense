# 咨询工作台：Run 架构收敛记录

> 状态：已完成并归档
> 更新日期：2026-07-04
> 文档性质：实现收敛记录与验收证据
> 关联文档：
> - `CONTEXT.md`
> - `docs/adr/0002-agent-runtime-ownership.md`
> - `docs/knowledges/ai-agent-impl/unified-consultation-run-sse-pipeline.md`
> - `docs/knowledges/ai-agent-impl/old-lazy-session-creation-and-silent-navigation.md`

## 摘要

咨询工作台已经从旧的“先创建会话、再发送消息”流程，收敛到 **Conversation / AI Run / Runtime Event Log / Projection** 分层。

这个方向是优雅的。前端只提交一次 run intent 并消费 Stream Event；Go 负责业务持久化、公开事件 ledger 和 Web projection；Python 拥有 Agent Thread、checkpoint、interrupt/resume 和模型工具调用真值。

本轮实施补齐了两个运行时缺口：AI service 的 LangGraph Postgres checkpointer 改为连接池，Go SSE writer 统一重编号所有对外事件。新会话、标题、replay 和 HITL resume 均已通过本地 dev 端到端验证。

## 架构边界

最终边界按下面的真值关系理解：

```txt
Python Agent Thread = 运行时真值
Go Runtime Event Log = 公开运行 ledger
Go business tables = 持久业务真值
Go projections = Web 读模型
Web = projection consumer
```

这不是纯 Event Sourcing。当前 MVP 保留规范化业务表，同时把 `runtime_events` 用作 replay、debug、audit 和 active turn 恢复的公开事件日志。

`messages` 是 UI、搜索和历史展示的消息投影，不是 Python Agent runtime history。Go 不从文本消息重建 Agent 上下文。

## 当前主流程

新会话首轮发送现在走统一 run 入口：

```txt
Web POST /api/v1/consultation-runs
  -> Go 创建 Run envelope
  -> 事务内创建或复用 Conversation / Session
  -> 事务内创建 Run、user message、assistant placeholder
  -> 更新 active_run_id、active_stream_id、last_message_at
  -> SSE 发 run.started / conversation.created / message.*
  -> Python AI service 流式返回事件
  -> Go 持久化公开 Runtime Event Log
  -> Go 完成 assistant message、Run、projection
  -> 生成标题并发 title.generated
  -> SSE 发 stream.done
```

关键修正是：`conversation.created` 不再只表示“拿到了 ID”。它现在表示会话已经分配、首条用户消息已持久化，并且 `last_message_at` 已满足列表接口可见条件。

## 已完成收敛

### 1. 统一 Run 入口

前端主发送链路已经使用 `POST /api/v1/consultation-runs`。旧的两段式 `createConsultation()` 后再 `sendConsultationMessage()` 不再是主路径。

后端只保留 `consultation-runs` 作为公开发送入口。旧 `POST /consultations/:id/messages` 路由、handler、前端 service 方法和 `Runtime.SendUserMessage` 已删除。

### 2. 固化 Run 启动事务

Run 启动写入已集中到 `ConsultationRepository.CreateRunEnvelope`。

该事务负责创建或复用 Conversation、创建 ConsultationSession、创建 Run、创建 user message 与 assistant placeholder，并更新 `active_run_id`、`active_stream_id`、`last_message_at`。

`Runtime.createTurnEnvelope` 已改为委托 `ConsultationService.CreateRunEnvelope`。Runtime 不再散落管理这些持久化细节。

### 3. 统一 Stream Event 契约

公共事件契约以 `packages/contracts/src/stream-events.ts` 为准。

`conversation.created` 已收敛为扁平 payload。主键只从 `ids.conversation_id` 读取，payload 只承载展示字段：

```ts
{
  version: 1;
  channel: 'conversation';
  type: 'conversation.created';
  ids: {
    conversation_id: string;
    run_id: string;
    turn_id: string;
  };
  payload: {
    title: string;
    title_status: 'pending' | 'generated' | 'manual';
    status: 'active';
    last_message_at: string;
    created_at: string;
    replaces_draft_id?: string;
  };
}
```

Go payload、Web reducer 和 contracts 已对齐。`title.generated` 也进入同一套 Stream Event 处理链路。

### 4. 统一 SSE 输出序列

Go `StreamWriter` 现在对所有 live event 统一分配递增 `seq`。来自 Python 的 AI events 不再保留上游局部 seq。

这个修复保证了 live SSE、`runtime_events` 唯一序列和 replay 序列一致，避免 tool/title/run completion 事件因为 seq 冲突而没有入库。

### 5. 闭合标题更新链路

新会话首轮 run 会在 `stream.done` 前尝试同步生成标题，并通过同一条 SSE 发出 `title.generated`。

标题生成失败不阻塞主回复结束。前端收到 `title.generated` 后，同时更新 thread cache 和 conversation list cache，并同步 `title_status`。

resume 路径不会再经过旧发送入口；如果会话仍是 pending title，会走后台标题补偿逻辑。

### 6. 修正幂等重放

重复 `request_id` 不再把 completed / failed / interrupted run 伪装成 `stream.error`。

当前语义是：

- `running` / `waiting_user`：返回 `409 RUN_IN_PROGRESS`。
- `completed` / `failed` / `interrupted`：从 `runtime_events` 按 `seq` 重放公开事件。
- replay 末尾追加 `stream.done`。

本地 E2E 已验证 replay 与 live stream 都包含完整事件集，且 `seq` 单调递增。

### 7. 修复 AI Checkpointer 并发

AI service 的 LangGraph Postgres checkpointer 不再使用 `AsyncPostgresSaver.from_conn_string()` 的单连接生命周期。

现在通过 `AsyncConnectionPool` 创建 `AsyncPostgresSaver`，避免并发 run 或前一个请求未完全释放时触发 psycopg `another command is already in progress`。

### 8. 明确 Projection 来源

`ThreadProjectionService` 已明确为混合 projection。

它主要从 `conversation`、`consultation_session`、`messages`、`pending_interactions` 构建 Web 读模型，再从 Runtime Event Log 派生 tool activity。它不宣称纯 Event Sourcing。

### 9. 保持中断恢复事件闭环

中断和恢复链路保留明确事件：

```txt
state.interaction.required
run.interrupted
state.interaction.answered
run.resumed
```

前端 reducer 已覆盖该生命周期。本地 E2E 也已验证真实 HITL 流程能从中断恢复到 completed。

## 验收状态

- [x] 新会话 `conversation.created` 发出前，`last_message_at` 已非空，列表接口可直接查到该会话。
- [x] Run 启动阶段的 Conversation、Session、Run、Message、active run、`last_message_at` 写入具备事务一致性。
- [x] `conversation.created` 和 `title.generated` 的 Go payload、Web 类型、contracts、文档一致。
- [x] 前端主路径只调用 `POST /api/v1/consultation-runs`。
- [x] 旧 `POST /consultations/:id/messages` 已删除，且无前端调用。
- [x] `title.generated` 能更新侧边栏标题和线程详情标题。
- [x] `ThreadProjectionService` 明确为混合 projection，不宣称纯 Event Sourcing。
- [x] 重复 completed `request_id` 能从 Runtime Event Log 重放。
- [x] 中断恢复保持 `run.interrupted`、`state.interaction.required`、`state.interaction.answered`、`run.resumed` 事件闭环。
- [x] 本地 dev 环境端到端验证完成。

## 端到端验证记录

本地 dev stack：

```txt
docker compose -f docker/docker-compose.yml --profile dev up -d postgres-dev redis-dev
docker compose -f docker/docker-compose.yml --profile dev up -d --build api ai-service
```

健康检查：

```txt
curl.exe -sS --max-time 10 http://127.0.0.1:8080/api/health
-> {"db":"ok","redis":"ok","service":"bodysense-api","status":"ok"}

curl.exe -sS --max-time 10 http://127.0.0.1:8100/health
-> {"status":"ok","service":"bodysense-ai"}
```

新会话首轮 run：

```txt
POST /api/v1/consultation-runs
conversation_id = 4eadc1e0-aa88-4583-bae0-2caa3592889f
run_id = ab3e4dfa-40aa-4082-a5df-7e1030dd1538
live events = 35
live seq monotonic = true
GET /api/v1/conversations contains conversation = true
GET /api/v1/consultations/:id/thread contains conversation = true
```

live event types：

```txt
run.started
conversation.created
message.persisted
message.created
tool.call
state.extracted_info.upsert
tool.result
message.text.delta
state.phase.changed
run.completed
message.completed
title.generated
stream.done
```

重复 `request_id` replay：

```txt
POST /api/v1/consultation-runs with same requestId
replay events = 35
replay seq monotonic = true
replay missing required event types = none
```

HITL 中断恢复：

```txt
conversation_id = f7886ff6-a507-427a-9cf3-c08f23767963
initial events:
run.started, conversation.created, message.persisted, message.created,
tool.call, state.interaction.required, run.interrupted, stream.done

resume events:
state.interaction.answered, run.resumed, message.created, tool.call,
tool.result, state.extracted_info.upsert, message.text.delta,
state.phase.changed, run.completed, message.completed, stream.done
```

## 自动化验证

已通过：

```txt
apps/api: go test ./...
apps/ai-service: uv run ruff check src/runtime/checkpointing.py
apps/ai-service: uv run pytest tests -q -o cache_dir=D:\home\projects\BodySense\.cache\pytest-ai
web consultation tests: vitest activeTurnReducer/threadMessageMapping/consultationService
web typecheck: tsc -p apps/web/tsconfig.json --noEmit
web lint: nx run @bodysense/web:lint
```

AI service pytest 需要把 `TMP`、`TEMP`、`UV_CACHE_DIR` 指到仓库内目录，避免 Windows 用户目录权限问题。

## 架构评价

该方案比旧实现更优雅，核心原因是阶段语义被拆清楚了：

- Conversation allocation：拿到真实会话 ID。
- Run envelope committed：首条消息、Run 和列表可见性已经持久化。
- Runtime events emitted：前端按公开事件渲染 active turn。
- Projection refreshed：Web 读模型与业务表同步。
- Title generated：标题作为独立事件更新 UI。

需要继续克制的是不要把 MVP 推成完整 Event Sourcing 平台。当前更好的边界是：业务表承载业务真值，Runtime Event Log 承载公开运行事件，projection 服务 Web 查询。

## 归档条件

本文档已满足归档条件，并已移入 `docs/plan/archive/`。