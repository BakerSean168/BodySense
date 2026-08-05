# T0 亮点方案（一）：人在回路（HITL）的持久化 Agent 运行时
> ✅ **已完成并归档**（2026-07-29）。实施落地见对应代码与测试；本文件移入 archive 仅作历史记录。


> 文档状态：调研 + 增强方案（待评审）
> 创建日期：2026-07-10
> 关联：`docs/project-review-2026-07-10.md` §1 T0-1、§4-3
> 真值来源：当前代码。文中 `file:line` 为撰写时锚点，实施前以最新代码为准。

> ⚠️ **锚点校正（2026-07-26，见 [architecture-review-2026-07-26.md](./architecture-review-2026-07-26.md) P1）**：
> 本文第 1 节的 Python 侧锚点 `orchestrator.py:363 _handle_ask_user` 指向的是**未挂载的死路径**（`services/consultation_graph.py` + `AgentOrchestrator`，未在 `main.py` include）。**当前生效的 HITL 实现在 `/runtime` 路径**：`apps/ai-service/src/runtime/consultation_thread.py`，用 LangGraph 原生 `interrupt()`（:432）+ `Command(resume=...)`（:687）+ Postgres checkpointer（:529）实现，比本文描述的旧路径更完善。
> Go 侧锚点（`runtime.go:858 handleInteractionRequired` 等）仍然有效。
> **实施本方案的 Phase A/B/C 前，必须先完成 P1 死路径清理**（否则会在错误的文件上加功能）。Phase A（超时回收）、Phase C（可观测性）与 Go 侧解耦，可独立推进；Phase B（多字段中断）需落在 `consultation_thread.py` 的 `ask_user` 节点而非 `orchestrator.py`。

---

## 0. 一句话定位

Agent 在一次生成中**主动中断向用户提问**（`ask_user` 工具），把待答问题**持久化**为 `agent_interaction`，通过 SSE 发出 `run.interrupted` 并**原子地**把助手消息落库为 `aborted`；用户在**另一个独立 HTTP 请求**里回答后，运行时**跨请求恢复**并续跑。这不是"一次性跑完"的玩具 Agent，而是理解 Agent 运行时**状态管理**的体现。

---

## 1. 现状架构剖析（先看清楚已有什么）

### 1.1 端到端时序（三方协作）

```text
用户 ──POST /consultation-runs────────────► Go Runtime.StartRun (runtime.go:82)
                                              │ 幂等检查 CheckIdempotency (runtime.go:89)
                                              │ createTurnEnvelope: session+run+2 messages
                                              │ SSE: run.started / message.created
                                              ▼
                                         executeRunFlow ──HTTP/NDJSON──► Python AI 服务
                                                                          │ consultation_graph → AgentOrchestrator
                                                                          │ LLM 决定调用 ask_user 工具
                                                                          │ executor 返回 status=interrupted (orchestrator.py:375)
                                                                          ▼
                                              emit {type: state.interaction.required, question}
                                              ◄──────────────────────────┘
   Go 收到 interaction.required
   → handleInteractionRequired (runtime.go:858)
      ① CreatePendingInteraction 持久化 agent_interaction (waiting)
      ② SSE: interaction.required（带 interaction_id）
      ③ SSE: run.interrupted {status: waiting_user}
      ④ 原子 UpdateMessageCompletedWithStatus(parts, "aborted") (runtime.go:921)
      ⑤ SSE: stream.done + clearActiveRun
   ◄─── 前端渲染问题卡片，等待用户输入 ───

用户回答 ──POST /consultations/:id/interrupts/:iid/answers──► Go Runtime.ResumeInteraction (runtime.go:327)
                                              │ 幂等检查（同 request_id 直接回放 replayCompletedRun）
                                              │ interactionService.ResumeInteraction：CAS 校验（answered/conflict/closed）
                                              │ createTurnEnvelope（把答案作为一条 user 消息，metadata.is_interaction_answer）
                                              │ SSE: state.interaction.answered / run.resumed
                                              ▼
                                         executeRunFlow（带上答案上下文续跑）→ 正常完成
```

### 1.2 关键代码锚点

| 环节 | 位置 | 说明 |
|---|---|---|
| Agent 触发中断 | `apps/ai-service/src/services/agent/orchestrator.py:363` `_handle_ask_user` | 工具执行返回 `status == "interrupted"` 即 emit `state.interaction.required` 并 `return True`（结束本轮生成） |
| Go 落地中断 | `apps/api/internal/consultation/runtime.go:858` `handleInteractionRequired` | 持久化 interaction + 发 `run.interrupted` + **原子**把助手消息置 `aborted` |
| 原子落库 | `runtime.go:921` `UpdateMessageCompletedWithStatus` | 单条 SQL 同时写 parts 与 status，避免"崩在两步之间"的不一致 |
| 跨请求恢复 | `runtime.go:327` `ResumeInteraction` | 独立 HTTP 入口，重建 turn envelope 续跑 |
| 状态机 CAS | `interactionService.ResumeInteraction` → `ErrInteractionConflict` / `ErrInteractionClosed`（`runtime.go:375-382`） | 同一 interaction 被不同答案重复回答 → 冲突；已关闭 → 拒绝 |
| 幂等回放 | `runtime.go:335-353` | Resume 也走 `CheckIdempotency`，重复 `request_id` 直接 `replayCompletedRun` |

### 1.3 为什么这是"对的"设计

1. **中断即持久化**：问题不是存在内存里的 goroutine，而是 `agent_interaction` 行；进程重启也能恢复对话。
2. **助手消息落 `aborted` 而非 `completed`**：读模型能区分"这轮被中断"与"这轮正常结束"，前端可据此渲染"追问卡片"而非普通气泡。
3. **恢复是独立请求**：与 Start 同构（都走 `createTurnEnvelope` + `executeRunFlow`），复用同一套幂等与事件溯源，无特殊路径。
4. **答案作为一条 user 消息**：把"回答追问"归一为"用户又说了一句话"，下游 LLM 上下文构建无需特判。

---

## 2. 现存不足与风险（诚实盘点）

> 🔴 正确性 · 🟡 健壮性 · 🟢 打磨

- 🟡 **H-1 中断超时无回收**：`agent_interaction` 置 `waiting` 后若用户永不回答，无 TTL/清理，`active_run_id` 已清除但 interaction 悬挂。缺"过期自动关闭 + 可视化"。
- 🟡 **H-2 恢复期的并发竞态**：同一 interaction 若前端"双击/重试"并发两次 Resume，依赖 `ErrInteractionConflict` 的 CAS；但两个请求可能都通过幂等检查（不同 request_id）后在 `ResumeInteraction` 处竞争。需确认 CAS 是行级锁/`UPDATE ... WHERE status='waiting'` 的受影响行数判定。
- 🟡 **H-3 多问阻塞语义单一**：`ask_user` 每次仅一问（prompt 已约束）。缺"一次收集多字段的结构化表单"式中断（answer_type 已有雏形：text/select）。
- 🟢 **H-4 中断可观测性弱**：无"平均等待时长""追问被采纳率""中断后流失率"等指标（与全局可观测性短板一致，见审查 §2.1 A-5）。
- 🟢 **H-5 断线未续传**：中断前已 flush 的 text_delta，用户断线重连不会自动回放（与 T0-2 事件溯源方案联动，见该文 §3）。

---

## 3. 增强方案（分阶段，可执行）

### Phase A：中断生命周期健全（0.5–1 天）

**目标**：让"悬挂中断"可回收、可观测、可恢复。

1. **过期回收**：新增 `agent_interaction.expires_at`（默认 now()+24h）。复用 `JobRuntime`/后台 worker 思路，加一个轻量 sweeper：`ListExpiredInteractions` → 置 `expired` 并发 `state.interaction.expired` 事件（进事件日志）。
2. **恢复前置校验**：`ResumeInteraction` 在 CAS 之外，明确以 `UPDATE agent_interactions SET status='answered', answer=$1 WHERE id=$2 AND status='waiting'` 的 `RowsAffected==1` 作为唯一"抢到"判据，`==0` 则区分 already-answered vs expired 返回对应 4xx。
3. **验收**：并发两次 Resume 只有一个 200、另一个 409；超时 interaction 24h 后变 `expired`，前端卡片显示"该追问已过期，请重新发起"。

### Phase B：结构化多字段中断（1–2 天，作品集加分）

**目标**：从"一次一问"升级为"一次一张结构化小表单"，同时**不破坏**现有单问路径。

1. `ask_user` 工具 schema 扩展：`fields: [{key, label, answer_type: text|select|scale, options?, required}]`（向后兼容：单问 = 单 field）。
2. `state.interaction.required` 的 `question` payload 承载 `fields`；前端追问卡片按 field 类型渲染。
3. `interactionAnswerParts`（`runtime.go:1299`）扩展：把多字段答案归一为一段可读文本 + 结构化 metadata，供 `health_features.upsert` 落地。
4. **治理**：字段数上限（如 ≤3）防止 Agent 一次性轰炸用户；仍禁止在正文文本里罗列问题（复用现有 prompt 约束，`prompts/consultation.py`）。
5. **验收**：Agent 可一次收集"部位 + 性质 + 是否对称"三字段；答案正确写入 `thread_projection.health_features`。

### Phase C：中断可观测性（1 天，联动可观测性主线）

1. 落一张 `interaction_metrics`（或直接在事件日志上做投影）：记录 `created_at`/`answered_at`/`expired_at`/`answer_type`。
2. 指标：**平均等待时长**、**回答率**、**过期率**、**每会话追问次数分布**。
3. **验收**：能回答"追问是否提升了信息完整度"——对比"有追问 vs 无追问"会话的 `health_features` 字段填充率。

---

## 4. 与其它 T0 亮点的联动

- **事件溯源（T0-2）**：中断/恢复的每一步都已走 `recordPublicEvent`（`runtime.go:1262`），天然可回放。Phase C 的指标可直接**在事件日志上做投影**，无需新写路径。断线重连回放（T0-2 §3）能让"中断前的部分回答"在重连后无缝恢复。
- **契约测试（T0-3）**：`state.interaction.required` / `run.interrupted` / `run.resumed` 都是版本化 StreamEvent；Phase B 改 payload 结构时，**必须**同步 `packages/contracts` 的 schema+fixtures 并让三方 parity 测试通过（见 T0-3 文 §4 流程）。

---

## 5. 面试叙事（怎么讲）

> "我实现了一个**可中断、可跨请求恢复**的 HITL Agent 运行时。Agent 在信息不足时不会瞎猜，而是调用 `ask_user` 工具**中断本轮生成**，把问题**持久化**、把助手消息**原子地**标为 `aborted`，通过 SSE 通知前端渲染追问卡片；用户在**另一个 HTTP 请求**里回答后，运行时用**与首轮同构**的路径续跑。我用**幂等 + 状态机 CAS** 处理了重复回答与并发竞态，用**过期回收**处理了悬挂中断。整条链路的每一步都进了**事件日志**，所以它是可回放、可观测的。"

命中考点：Agent 运行时状态管理、跨请求会话恢复、幂等/CAS 并发控制、事件驱动、可观测性。

---

## 6. 落地任务清单

```text
A1 feat(api): agent_interaction.expires_at + 迁移
A2 feat(api): interaction sweeper（过期回收 + state.interaction.expired 事件）
A3 fix(api): ResumeInteraction 以 RowsAffected 作为抢占判据，区分 answered/expired
A4 feat(contracts): state.interaction.expired 事件 schema+fixtures（三方 parity）
--- Phase B ---
B1 feat(ai): ask_user 多字段 schema（向后兼容单问）
B2 feat(web): 结构化追问卡片渲染（text/select/scale）
B3 feat(api): interactionAnswerParts 多字段归一 + health_features 落地
B4 feat(contracts): interaction.required.question.fields schema+fixtures
--- Phase C ---
C1 feat(api): interaction 指标投影
C2 feat(web): 追问效果小面板（等待时长/回答率/字段填充率对比）
```

## 7. 风险与回滚

- 所有增强均以**新增列 / 新增事件类型 / 向后兼容 schema** 实现，对既有中断链路零侵入。
- 回滚：关闭 sweeper、隐藏多字段渲染即退回单问阻塞语义；事件类型新增不影响旧客户端（未知 type 忽略）。
