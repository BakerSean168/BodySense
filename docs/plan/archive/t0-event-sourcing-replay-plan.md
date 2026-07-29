# T0 亮点方案（二）：事件溯源 + 投影 + 流回放的流式架构
> ✅ **已完成并归档**（2026-07-29）。实施落地见对应代码与测试；本文件移入 archive 仅作历史记录。


> 文档状态：调研 + 增强方案（待评审）
> 创建日期：2026-07-10
> 关联：`docs/project-review-2026-07-10.md` §1 T0-2、§2.2 G-1、§2.3 W-4、§4-3
> 真值来源：当前代码。文中 `file:line` 为撰写时锚点，实施前以最新代码为准。

> ✅ **现状校正（2026-07-26，见 [architecture-review-2026-07-26.md](./architecture-review-2026-07-26.md)）**：
> 本文第 1 节描述的事件日志(`runtime_events` append-only + 单调 `seq`)、`recordPublicEvent` 落库、`replayCompletedRun` 幂等回放、`thread_projection` 投影**均已在 Go 侧实现并运行**，Go 侧锚点有效。这与 `docs/architecture/stream-event-contract-runtime.md` 标注的「~30% 未实现」冲突——**以本文和当前代码为准，架构文档标注滞后**。
> 因此本文的 Phase A/B/C/D 全部是**在已落地能力上的增强**（增量回放端点、前端断线续传、写放大优化），非从零新建。Phase B 前端续传是审查点名的高性价比增强。

---

## 0. 一句话定位

每条对外 SSE 事件都以 `runtime_event`（append-only、单调 `seq`）持久化（`runtime.go:1262 recordPublicEvent`），因此**幂等重放**成立：重复的 `request_id` 直接从事件日志**完整回放**整条流（`runtime.go:1010 replayCompletedRun`）。读模型走 `thread_projection`（投影），与写模型分离——这就是 CQRS + Event Sourcing 的教科书落地。

---

## 1. 现状架构剖析

### 1.1 写路径：每个对外事件都进日志

```text
executeRunFlow 中任何一次对外事件
   ├─ sendNewEvent (runtime.go:1225)   // 新建带 seq 的事件
   │     sw.NewEvent → WriteEvent(SSE) → recordPublicEvent
   └─ sendEvent (runtime.go:1247)      // 透传已有事件（如来自 Python 的 NDJSON）
         sw.EnrichEvent → WriteEvent(SSE) → recordPublicEvent

recordPublicEvent (runtime.go:1262)
   → runtimeEventService.RecordPublicEvent(conversationID, runID, turnID, event)
   → 落 runtime_events(seq, channel, type, ids jsonb, payload jsonb, ...) append-only
```

**关键点**：SSE 与持久化在同一处发生（`sendNewEvent`/`sendEvent` 内联调用 `recordPublicEvent`），保证"用户看到的"与"日志里的"逐字节一致——这正是可回放的前提。

### 1.2 回放路径：重复 request_id → 完整重播

```text
StartRun / ResumeInteraction 开头都先 CheckIdempotency (runtime.go:89 / :335)
   found && status ∈ {running, waiting_user}  → 409 RUN_IN_PROGRESS
   found && 已终态                              → replayCompletedRun (runtime.go:1010)

replayCompletedRun:
   ListAllRunEvents(conversationID, runID)     // 按 seq 取全量
   for each stored event: sw.WriteEvent(重建 StreamEvent{version,seq,channel,type,ids,payload})
   末尾补 stream.done(seq=maxSeq+1)
```

即：**同一个 request_id 再打一次，客户端会拿到与第一次逐事件相同的流**（含最终文本、工具调用、引用、governance 结论），而不会触发二次 LLM 调用或二次副作用。

### 1.3 读路径：投影与写模型分离

- 写模型：`runtime_events`（append-only 事实）+ `messages`/`runs`/`agent_interactions`（聚合当前态）。
- 读模型：`thread_projection`（`refreshThreadProjection` `runtime.go:1113`）——把一个会话的消息、工具调用、中断历史、`health_features` 投影成前端一次拉取即用的结构。
- 分离收益：读侧结构可自由演化（如新增 `health_features` 投影列，迁移 000029），无需改写事件日志。

### 1.4 关键代码锚点

| 环节 | 位置 |
|---|---|
| 事件持久化入口 | `runtime.go:1262 recordPublicEvent` |
| 两类发送封装 | `runtime.go:1225 sendNewEvent` / `runtime.go:1247 sendEvent` |
| 幂等检查 | `runtime.go:89` / `runtime.go:335` `CheckIdempotency` |
| 完整回放 | `runtime.go:1010 replayCompletedRun`（`ListAllRunEvents` + 逐事件 `WriteEvent` + 补 `stream.done`） |
| 读模型投影 | `runtime.go:1113 refreshThreadProjection` → `thread_projection*` |
| 事件契约 | `packages/contracts/schemas/stream-event.v1.schema.json`（`version/seq/channel/type/ids/payload`） |

---

## 2. 现存不足与风险

- 🟡 **E-1 写放大（审查 G-1）**：一条长回复有数百个 `message.text.delta`，每个都**内联同步**插一行 `runtime_events`（`recordPublicEvent` 在流循环里被逐事件调用）。增加每 chunk 时延、放大 DB 负载。
- 🟡 **E-2 断线不续传（审查 W-4）**：后端**已具备**按 `seq` 回放能力，但前端手写 SSE 解析（`useSSEProcessor.ts`）断线只 `onError`，不基于 `seq` 续传——**已有的后端能力没兑现成前端体验**。
- 🟢 **E-3 回放是"全量"而非"增量"**：`replayCompletedRun` 总是从 `seq=1` 重播。断线重连场景其实只需 `seq > lastSeq` 的增量。
- 🟢 **E-4 事件日志无保留策略**：`runtime_events` 只增不删，缺冷归档/TTL；长期体量增长。
- 🟢 **E-5 投影重建成本**：`refreshThreadProjection` 何时全量重投、何时增量，未见明确策略；大会话可能偏慢。

---

## 3. 增强方案（主打：把"回放能力"兑现成"断线重连续传"）

> 这是审查 §4-3 明确点名的**低成本、高说服力**增强：后端已有 `seq` 事件日志与 `replayCompletedRun`，只差前端临门一脚 + 后端一个增量端点。

### Phase A：增量回放端点（0.5 天）

1. 新增 `GET /conversations/:id/runs/:runId/events?after_seq=N`（已有 `ListRunEvents` 路由 `main.go:208`，扩展 `after_seq` 过滤即可）。
2. `replayCompletedRun` 抽出 `replayFrom(seq)`：断线重连时只回放 `seq > after_seq`，末尾补 `stream.done`。
3. **验收**：`after_seq=0` 等价全量；`after_seq=lastSeq` 只补尾部与 `stream.done`。

### Phase B：前端弹性流（1–1.5 天）

1. `useSSEProcessor.ts` 记录已消费的 `maxSeq`（事件本就带 `seq`）。
2. `onError`/网络中断 → 指数退避重连，重连时：
   - 若 run 仍 `running` → 重新订阅 SSE（后端 `StartRun` 幂等；或走事件端点 `after_seq=maxSeq` 增量拉取直到 `stream.done`）。
   - 若 run 已终态 → 直接 `GET events?after_seq=maxSeq` 补齐。
3. **去重**：以 `seq` 单调性丢弃重复/乱序事件（reducer 幂等）。
4. **验收**：问诊生成中途断网 10s 再恢复，**不丢生成进度**、无重复文本、最终态一致（用 React Profiler + 断网模拟验证）。

### Phase C：写放大优化（1 天，解决 G-1）

**目标**：在"可回放"与"写放大"之间取得工程权衡，并能把权衡讲清楚。

1. **高频事件批量落库**：`message.text.delta` 累积进缓冲，按"每 N 条或每 T 毫秒"批量 `INSERT`；低频里程碑事件（run.*/tool.*/state.*/safety.*）仍同步落库。
2. **回放语义保持**：批量落库不改变 `seq` 单调性与逐事件重建；只是把"实时逐行 insert"改为"分批 insert"。
3. **可选：里程碑 + 最终文本模式**（更激进）：只持久化里程碑事件 + 完成时的最终 parts，回放时用最终文本一次性重建 `message.text.delta`（牺牲"逐字回放"，换极低写放大）。二者按需二选一，**面试时讲清取舍**即为亮点。
4. **验收**：长回复（>300 delta）的 DB 写次数下降一个量级；回放结果与优化前逐事件等价（用 fixture 对比）。

### Phase D（可选）：日志保留与投影健壮性（0.5–1 天）

1. `runtime_events` 冷归档：已完成会话 N 天后转冷表/对象存储，热表保留最近窗口。
2. 投影重建幂等测试：从事件日志**从零重投**一个会话，断言与在线投影结果一致（Event Sourcing 的杀手锏演示：**投影可从事实重建**）。

---

## 4. 与其它 T0 亮点的联动

- **HITL（T0-1）**：中断/恢复事件（`run.interrupted`/`run.resumed`）本就在日志里，断线重连续传（Phase B）能让"追问中途断线"无缝恢复。
- **契约测试（T0-3）**：新增 `after_seq` 不改事件结构，无需动 schema；但若 Phase C 引入新的"最终文本重建"事件语义，需在 fixtures 里补一条并过三方 parity。

---

## 5. 面试叙事

> "我把问诊做成了**事件溯源**：每条对外 SSE 事件都以单调 `seq` append-only 落库，用户看到的与日志里的逐字节一致。于是**幂等回放**成立——重复的 `request_id` 直接从事件日志完整重播，不会二次调用 LLM。读侧走**投影**（CQRS），与写模型分离、可独立演化。基于这套事实日志，我做了**断线重连的弹性流**：网络抖动后按 `seq` 增量补齐，不丢生成进度、无重复。写侧我对**高频 text delta 做了批量落库**，在'可回放'与'写放大'之间做了显式权衡。"

命中考点：Event Sourcing、CQRS、幂等回放、弹性流式、写放大权衡、投影可重建。

---

## 6. 落地任务清单

```text
A1 feat(api): events 端点支持 after_seq 增量过滤 + replayFrom(seq)
B1 feat(web): useSSEProcessor 记录 maxSeq + 指数退避重连
B2 feat(web): 重连按 after_seq 增量补齐 + seq 去重（reducer 幂等）
C1 perf(api): text.delta 批量落库缓冲；里程碑事件仍同步
C2 test(api): 回放等价性 fixture（优化前后逐事件一致）
D1 feat(api): runtime_events 冷归档策略（可选）
D2 test(api): 从事件日志从零重建投影 == 在线投影
```

## 7. 风险与回滚

- Phase A/B 纯增量、不改事件结构，回滚=前端不启用重连即可。
- Phase C 需守住"回放等价性"红线（C2 测试是门禁）；若批量落库导致崩溃丢尾部事件，回退为同步落库。
