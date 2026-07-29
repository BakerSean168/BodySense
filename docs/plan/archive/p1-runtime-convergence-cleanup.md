# P1：运行时收敛清理（终态化的未竟清理 + 契约漂移 + 真值瑕疵）
> ✅ **已完成并归档**（2026-07-29）。实施落地见对应代码与测试；本文件移入 archive 仅作历史记录。


> 文档状态：待实施（源自 [architecture-review-2026-07-26.md](./architecture-review-2026-07-26.md) P1 + P5 + 真值边界瑕疵）
> 创建日期：2026-07-26
> 关联：[final-agent-runtime-architecture.md](./final-agent-runtime-architecture.md) §6.2/§10、ADR 0002
> 真值来源：当前代码。文中 `file:line` 为撰写时锚点，实施前以最新代码为准。
> 优先级：🔴 第 0 阶段 —— 纯收敛/减法，风险最低，越早越省认知负担

---

## 0. 一句话定位

代码已经跑到了终态文档前面：`final-agent-runtime-architecture.md` §10 明令删除的 `consultation_graph`、旧 chat 路径至今**仍在仓库里并存**，`AgentOrchestrator` 只服务这条**未挂载的死路径**。本方案完成这份终态文档**没执行完的清理**，并顺手修掉两处小瑕疵：StreamChannel 契约漂移（P5）、checkpointer 静默内存降级（真值边界）。全部是减法或加固，无新功能、无下游依赖。

---

## 1. 现状盘点（两套运行时并存的死代码）

### 1.1 Python 侧：两套问诊图，一套是死代码

| 路径 | 文件 | 是否挂载 | checkpointer | HITL |
|---|---|---|---|---|
| **新（生效）** | `runtime/consultation_thread.py`（726 行） | ✅ `/runtime` 路由已进 `main.py:29-33` | ✅ `:529` 带 checkpointer 编译 | 原生 `interrupt()` + `Command(resume=...)` |
| **旧（死代码）** | `services/consultation_graph.py`（489 行） | ❌ chat 路由**未 include 进 main.py** | ❌ `:455` 裸 `compile()` | 无 resume 机制 |

- `AgentOrchestrator`（`services/agent/orchestrator.py`）**只服务旧死路径**；新路径把 tool loop 拆成 `llm_turn`↔`execute_tool` 图节点，不经 orchestrator。
- `chat_service.py`（`ChatService.stream_chat`）也只在死路径上，其"observe-only 治理"（`:86-96`）随死路径一起失效。
- ⚠️ **陷阱**：`t0-hitl-agent-runtime-plan.md` §1.2 的锚点 `orchestrator.py:363` 指向的正是这条死路径 —— 该文档的"现状"描述已过时（见 [索引](./README.md) 的校正说明）。

### 1.2 Go 侧：死 DTO 残留

- `dto/conversation.go:5 SendMessageRequest` + `:27 ContextDTO` 仍在源码，但**无任何路由绑定**（grep `dto.SendMessageRequest` 在 `internal`/`cmd` 下 0 匹配）。属 ADR 0002 应删而未删的残留。

### 1.3 契约漂移（P5）

- `StreamChannel` 枚举：TS（`stream-events.ts:1-12`，权威）有 `run`/`title`，但 **Python（`stream_event.py:9-19`）与 JSON Schema 都缺这两个**。后端**确实在发** `run.started`（`runtime.go:138`）、`title.generated`（`:210`）。fixture 恰好没这两类样本，parity 测试没抓到。
- 前端本地重定义了契约里已有的 `InteractionRequired/Answered` 事件（`consultation.ts:309-331` vs `stream-events.ts:194-204`）。

### 1.4 真值边界瑕疵

- `runtime/checkpointing.py:60-68`：Postgres 初始化失败时**静默降级** `InMemorySaver()`。生产未配 DB 时运行时真值不落库且无告警 —— 违背 ADR 0002 "Python 通过 checkpoint 拥有持久运行时真值"。

---

## 2. 设计原则（优雅解法：删而非留，一处真源）

`final-agent-runtime-architecture.md` §9.4 已写明纪律：**"若一个 Module 只是因为所有权分裂而把一种浅表示翻译成另一种，删掉它而非保留"**。本方案就是执行这条纪律。

1. **死代码零保留**：未挂载的旧问诊图、orchestrator、死 DTO 直接删，不留"以防万一"。
2. **契约单一真源**：漂移的枚举以 TS 为准补齐 Python/Schema，并补 fixture 让 parity 网能抓住（正是 [T0-3](./t0-cross-language-contract-testing-plan.md) Phase B 设计目标）。
3. **真值不静默**：checkpointer 不可用应**显式失败或显式告警**，绝不静默退化成内存态。

---

## 3. 实施方案

### Phase A：Python 死路径删除（0.5 天，纯减法）

> ⚠️ **实施时核查修正（2026-07-26）**：`agent/orchestrator.py` **不能整文件删**。核查发现生效路径 `runtime/consultation_thread.py:23` 共享了它的两个纯函数 `build_fallback_reply`、`emit_citation_events`（及私有 `_markdown_to_text`）。这三者与"orchestrator"无关，是"无 LLM 兜底回复 + 引用事件构造"。正确做法是**先把存活函数迁到语义正确的新模块 `agent/reply_fallback.py`，再删 orchestrator.py**。死代码是 `AgentOrchestrator` 类、`TextBuffer`、`AgentTurnInput/Result`、orchestrator 私有的 `_health_features_from_symptom`/`_chunk_text`（生效路径有自己的同名实现）。

1. 新建 `services/agent/reply_fallback.py`，迁入 `build_fallback_reply`、`emit_citation_events`、`_markdown_to_text`（这三者不依赖任何死代码）。
2. 改 `runtime/consultation_thread.py:23` 的 import 指向 `reply_fallback`。
3. 删除 `services/consultation_graph.py`、`services/chat_service.py`、`services/agent/orchestrator.py` 及其专属测试（`tests/unit/test_consultation_graph.py`、`test_chat_service.py`）。将 `test_consultation_graph.py` 中仅测 `build_fallback_reply` 的用例迁到新的 `test_reply_fallback.py`。
4. 删除未挂载的 chat 路由文件（`api/routes/chat.py`）；`api/routes/__init__.py` 无 chat 引用（已核查），无需改。
5. 清理 `search_knowledge.py:10`、`extract_symptom_info.py:18` 注释里对 `consultation_graph` 的引用（仅注释，非真实依赖）。
6. 核对 `ask_user.py` 文件头过时注释（`:1-6` 自称"未包含在默认工具列表"，实则已注册暴露）——更正注释。
7. **验收**：`grep -r 'consultation_graph\|AgentOrchestrator\|ChatService' src/` 零命中；`uv run pytest` 全绿；`/runtime` 问诊端到端不受影响。

> ⚠️ 这是硬删除，确认后不可逆——建议独立 PR，reviewer 易审。

### Phase B：Go 死 DTO 删除（0.25 天，纯减法）

1. 删除 `dto/conversation.go` 中 `SendMessageRequest` + `ContextDTO`（确认无路由绑定后）。
2. **验收**：`go build ./...` + `go vet ./...` 通过；`grep SendMessageRequest` 仅剩历史文档。

### Phase C：StreamChannel 契约漂移修复（0.5 天，加固 P5）

1. 以 TS `stream-events.ts` 为权威，给 Python `StreamChannel`（`stream_event.py:9-19`）和 `stream-event.v1.schema.json` 补齐 `run`、`title`。
2. `packages/contracts/fixtures/stream-events.v1.json` 补 `run.started`、`title.generated` 两条黄金样本。
3. 前端删除本地重定义的 `InteractionRequired/Answered`，改从 `@bodysense/contracts` 导入（`consultation.ts:309-331`）。
4. **验收**：三方 parity 测试遍历新 fixture 全绿；故意在 Python 用旧枚举校验 run/title 事件应先红后绿（证明漂移被网住）。

### Phase D：checkpointer 降级加固（0.25 天，加固真值边界）

1. `runtime/checkpointing.py`：Postgres 初始化失败时——生产环境（`ENV=production`）**直接启动失败**（fail-fast）；开发环境降级内存但打**显式 WARNING** 日志 + 标记 `checkpointer_degraded=true`。
2. **验收**：模拟 DB 不可达，生产配置下服务拒绝启动；开发配置下有醒目告警而非静默。

---

## 4. 与其它计划的联动

- **[final-agent-runtime](./final-agent-runtime-architecture.md)**：本方案是其 §10 delete-list 的**实际执行**。完成后该终态文档的"待删清单"才算兑现，可据此更新其 Status。
- **[T0-1 HITL](./t0-hitl-agent-runtime-plan.md)**：Phase A 删除后，T0-1 的锚点须从 `orchestrator.py:363`（死路径）改指 `consultation_thread.py`（`:432 interrupt`）。见 [索引](./README.md) 校正说明。
- **[T0-3 契约测试](./t0-cross-language-contract-testing-plan.md)**：Phase C 正是 T0-3 Phase B "全事件覆盖" 的一个先行样本。
- **[P2 治理](./p2-output-governance-gate.md)**：Phase A 删掉的 `chat_service` observe-only 治理，其价值由 P2 在 `/runtime` 路径上以强制关卡形式重建——删旧不丢能力。

---

## 5. 落地任务清单

```text
--- Phase A（Python 减法） ---
A1 refactor(ai): 删除 consultation_graph / chat_service / orchestrator + 死 chat 路由
A2 fix(ai): 更正 ask_user 文件头过时注释
A3 test(ai): 确认 /runtime 问诊端到端回归全绿
--- Phase B（Go 减法） ---
B1 refactor(api): 删除 SendMessageRequest / ContextDTO 死 DTO
--- Phase C（契约漂移） ---
C1 fix(contracts): Python StreamChannel + JSON Schema 补 run/title
C2 test(contracts): fixture 补 run.started/title.generated + 三方 parity 遍历
C3 refactor(web): InteractionRequired/Answered 改从 contracts 导入
--- Phase D（真值加固） ---
D1 fix(ai): checkpointer 生产 fail-fast / 开发显式告警，消除静默内存降级
```

## 6. 风险与回滚

- Phase A/B 是硬删除，**不可逆**：务必独立小 PR、删前 grep 复核、CI 全绿再合。建议按 A→B→C→D 顺序，各自独立提交。
- Phase C/D 为加固，回滚=还原枚举/降级逻辑。
- 唯一实质风险点：Phase A 误删仍被引用的符号 → 由 `grep` 复核 + `pytest` + `/runtime` 回归三重兜底。
