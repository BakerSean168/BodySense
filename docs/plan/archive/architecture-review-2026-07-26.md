# 架构审查与收敛路线图（2026-07-26）
> ✅ **已完成并归档**（2026-07-29）。实施落地见对应代码与测试；本文件移入 archive 仅作历史记录。


> 文档状态：审查结论 + 优雅解决方案 + 统一路线图（待评审）
> 创建日期：2026-07-26
> 关联：[ADR 0002](../adr/0002-agent-runtime-ownership.md)、[final-agent-runtime-architecture.md](./final-agent-runtime-architecture.md)、`docs/project-review-2026-07-10.md`
> 真值来源：当前代码。本文所有 `file:line` 为撰写时锚点，实施前以最新代码为准。

---

## 0. 一句话结论

BodySense 的**工程底座强于业务闭环**：Agent 运行时（LangGraph checkpoint + Go Runtime Event Log + projection + HITL 中断恢复）已真实落地且闭环，质量超出同阶段项目；但代码已走到 [ADR 0002](../adr/0002-agent-runtime-ownership.md) 终态，**七份架构文档描述的仍是上一代（ADR 0001）设计**，导致文档与代码整体错位；同时**业务主线（评估 → 问诊 → 计划 → 跟踪）在"最后一公里"断裂**——后端算好的能力前端不消费，最新做的体态分析是数据孤岛。

**核心矛盾不在架构选型，而在收敛**：两代设计并存、两套运行时并存、后端能力与前端消费脱节。因此本轮的主基调是**收敛与对接，而非新建**。

---

## 1. 审查方法与可信度

本审查基于对以下四个面的交叉核实（静态代码 + 文档），凡结论均带证据；**已验证事实**与**静态推断风险**分开标注：

- 文档层：`docs/architecture/` 全部文档、两份 ADR、`docs/plan/active/` 六份计划。
- Go 运行时：`apps/api/internal/{consultation,service,stream,model,handler,dto}`、`migrations/`。
- Python 运行时：`apps/ai-service/src/{runtime,services,ai,api}`。
- 契约与业务主线：`packages/contracts/`、`apps/web/src/features/`。

未执行 `go build` / `pytest` / `nx` 实跑，竞态与事务窗口均为静态推断，已在正文标注"未验证"。

---

## 2. 架构现状画像（真实态，非文档理想态）

```txt
生效路径（已落地、闭环）：
  Web (assistant-ui + 手写 SSE reducer)
    → Go consultation.Runtime  (StartRun/ResumeInteraction)
        → runtime_events (append-only, seq 单调, 可回放)
        → thread_projections (读模型)
        → Python /runtime/*  (consultation_thread.py)
            → LangGraph + Postgres checkpointer (interrupt/resume 原生)
            → ToolRegistry / ToolExecutor  (ask_user / search_knowledge / extract_symptom_info)

死代码 / 未收敛（存在但不生效）：
  Python services/consultation_graph.py + chat_service.py + AgentOrchestrator  ← 未挂载
  Go dto.SendMessageRequest + ContextDTO                                       ← 无路由绑定
  契约里的 job.* 事件                                                          ← 无前端消费者
  Go workflow/health_journey.go (完整实现 + 路由)                              ← 前端零调用
```

---

## 3. Top 5 架构问题（按严重度）

### 🔴 P1 — Python 侧两套并行问诊运行时，旧的一套是死代码但仍在维护

**问题**：`services/consultation_graph.py`（489 行，旧）与 `runtime/consultation_thread.py`（726 行，新）是两套几乎重复的 LangGraph 问诊实现。只有 `/runtime` 路由挂进 `main.py:29-33`；chat 路由**未 include**。旧路径无 checkpointer（`consultation_graph.py:455` 裸 `compile()`）、无 resume；`AgentOrchestrator` 只服务这条死路径。

**影响半径**：新人 onboarding + 任何"改问诊逻辑"的改动都要面对两套实现、极易改错文件；`t0-hitl-agent-runtime-plan.md` 引用的 `orchestrator.py:363` 锚点指向的正是死路径。

**根因**：[final-agent-runtime-architecture.md](./final-agent-runtime-architecture.md) §6.2 明令"删除 consultation_graph、无兼容模式"，但清理这一步没执行完。**这不是新问题，是既有终态文档未完成的收尾。**

**优雅解法**：见 [p1-runtime-convergence-cleanup.md](./p1-runtime-convergence-cleanup.md)。纯减法，无下游依赖，**最先做**。

---

### 🔴 P2 — AI 输出治理是旁路/观察态，高风险输出（诊断、训练计划）根本不过 Guard

**问题**：`AIOutputGuard`（`governance/output_guard.py`）实现完整（schema/red_flag/faithfulness 三策略），但**没有一处把它当强制关卡**：

- 诊断/训练计划完全不调用它（`diagnosis_service.py:71-188` 只做 Pydantic 校验）。
- 唯一跑 faithfulness 治理的 `validate_treatment`（`output_guard.py:61`）**零调用者**，是死代码。
- chat 文本是 "observe-only, non-blocking"（`chat_service.py:86-96`），且在死路径上。
- 生效的 `/runtime` 路径下 `grep governance` **零命中**。
- `posture_analyzer.py:158` 是唯一真调用者，但只用于降 confidence，不拦截。

**影响半径**：这是**健康辅助**产品，诊断与训练计划是最高风险输出。"先治理再落库"的架构承诺在代码层不成立——幻觉诊断或不安全训练动作可直接下发给用户。

**根因**：Guard 建好了但接入点选在旧路径 / 低风险路径；重构到 `/runtime` 时没把 Guard 一起迁过去。

**优雅解法**：见 [p2-output-governance-gate.md](./p2-output-governance-gate.md)。把 Guard 收敛为 `/runtime` 图节点前的**统一 seam**，而非散点调用。**安全属性，优先级仅次于 P1。**

---

### 🟠 P3 — 业务主线的"下一步"后端已实现，前端却完全不消费，靠硬编码猜

**问题**：`HealthJourney` 状态机在 Go 侧**完整实现且非 0%**（与 README 标注相反）——`workflow/health_journey.go:39-167` 从五张表派生 stage，10 stage + 15 action，有单测，`GET /api/v1/journey` 已挂载（`main.go:227`）。但**全 web 代码搜 `journey` = 0 命中**。`DashboardPage.tsx:30-59` 是写死的 4 张卡片，唯一"下一步"是 `profile===null` 就跳 onboarding（`:24-28`）。

**影响半径**：这是"Agent 原生"承诺的核心——用户不该自己找路，系统该给 `available_actions`。现在后端算好的引导是无消费者的死代码，产品体验退回成静态菜单。

**根因**：前后端并行开发，journey 的 DTO 没进 `packages/contracts`，前端缺对接锚点。

**优雅解法**：见 [p3-health-journey-activation.md](./p3-health-journey-activation.md)。后端已就绪，是**高杠杆低成本对接**。

---

### 🟠 P4 — 体态照片分析是数据孤岛，未接入评估/问诊主线

**问题**：posture 三端链路技术上打通、数据契约三端对齐（`posture.py:1-7` 声明单一真源），但：

- 结果 `user_uploads.analysis_result` **未被 consultation 读取**（`consultation/` 0 命中），里面的 `posture_findings` 只是空数组占位。
- **也未被 assessment 使用**——`assessment_service.go:72-80` 发给 AI 的是原始照片 base64，自己重做一遍多模态，两条链路各算各的。
- 前端用 `setInterval` 轮询 `analysis_status`（`UploadStep.tsx:33-41`），契约里的 `job.*` 事件（`stream-events.ts:206-228`）**无前端消费者**。

**影响半径**：体态纠正是产品名字本身（体悟 / BodySense）。最贵的多模态 AI 能力用完即弃，既没喂给 Agent 诊断，也没减少 assessment 的重复调用。

**根因**：posture 是最新纵向功能（PR #39），作为独立 MVP 落地，尚未做与主线的横向集成。

**优雅解法**：这一半工作**已经是** [posture-photo-analysis-plan.md](./posture-photo-analysis-plan.md) 的 Phase 3-B1（`get_posture_analysis` Agent 工具 + assessment 复用）。本轮**不新建计划，只把它从"体态功能纵向增强"提级为"主线集成"并前置**。详见 §5 路线图与 §6 计划整理。

---

### 🟡 P5 — 契约"单一真源"只覆盖 SSE 信封，且已出现未被测试捕获的枚举漂移

**问题**：

- `packages/contracts` **只导出 stream-events**（`index.ts:4`）；业务 REST DTO（AssessmentReport / TrainingPlan / PostureAnalysis）与状态枚举（`ConsultationPhase` / `AnalysisStatus`）在前端与 Go/Python 各写一份。`ConsultationPhase` 前端（`consultation.ts:147-153`）比 Go 权威（`consultation_phase.go:16-20`）多一个 `completed`。
- `StreamChannel` 枚举**已漂移**：TS（权威）有 `run`/`title`，Python（`stream_event.py:9-19`）与 JSON Schema 都缺这两个；而后端**确实在发** `run.started`（`runtime.go:138`）与 `title.generated`（`:210`）。fixture 恰好没这两类样本，parity 测试没抓到。
- 前端本地重定义了契约里已有的 `InteractionRequired/Answered` 事件（`consultation.ts:309-331` vs `stream-events.ts:194-204`）。

**影响半径**：Python Pydantic Literal 若严格校验 run/title 事件会失败；业务 DTO 三处维护，字段漂移已发生。

**优雅解法**：这**正是** [t0-cross-language-contract-testing-plan.md](./t0-cross-language-contract-testing-plan.md) Phase A/B 设计来抓的问题。本轮**不新建计划，只把"补齐 StreamChannel 漂移 + 加 run/title fixture"提为该计划的第 0 步先做**（低成本防线上错误），DTO 收敛随各 feature 迭代。

---

## 4. 真值边界专项结论（ADR 0002 成立到什么程度）

**结论：ADR 0002 在生效路径上基本成立，执行得比文档看起来干净。**

| 断言 | 结论 | 证据 |
|---|---|---|
| Python 经 checkpoint 拥有运行时真值 | ✅ 成立 | `consultation_thread.py:529` 带 checkpointer 编译；原生 `interrupt()`+`Command(resume=...)` 闭环 |
| Go 拥有 Runtime Event Log + projection | ✅ 成立 | `runtime_events`（`UNIQUE(run_id,seq)`）；`recordPublicEvent` 落库；`replayCompletedRun` 按 seq 回放 |
| Go 不再从文本 messages 重建 LLM 历史 | ✅ 成立 | 只传本轮输入 + 业务快照（`ai_client.go:55-70`） |
| 旧路径代码符号已删 | ✅ 成立 | `internal/context/`、`chat_handler.go`、`ContextBuilder` 均不存在；Web 合成 resume 已改 `resumeRun` intent |
| checkpointer 持久化 | ⚠️ 瑕疵 | Postgres 初始化失败**静默降级** `InMemorySaver`（`checkpointing.py:60-68`），生产未配 DB 时真值不落库且无告警 |
| 旧 DTO 清理 | ⚠️ 残留 | `dto.SendMessageRequest`+`ContextDTO` 死代码（无路由绑定，非活跃双写） |

**真值边界模糊风险（已验证代码存在，未验证是否触发）**：

- `health_features` 有两套启发式派生（stream 路径 `runtime.go:1334/1375` vs projection 路径 `thread_projection_service.go:243-357`）+ 用户可直接 `PUT` 写入，同概念真值分散。
- thread projection 为**读时全量重算 + upsert**，无版本 / 乐观锁，并发刷新存在后写覆盖竞态。
- 中断 / 失败路径跨多次 DB 调用无包裹事务，存在一致性窗口。

这三点归入 [p1-runtime-convergence-cleanup.md](./p1-runtime-convergence-cleanup.md) 的"边界收敛"部分。

---

## 5. 统一收敛路线图（重排后的优先级）

**原则**：先做**减法与对接**（低风险、高杠杆、兑现已有能力），再做**安全补强**，最后回到团队既定的深化计划。

| 阶段 | 内容 | 对应问题 | 承接计划 | 性质 |
|---|---|---|---|---|
| **W0 收敛** | ①删 Python 死路径 ②补 StreamChannel 漂移+fixture ③checkpointer 内存降级改显式告警 ④删 Go 死 DTO | P1, P5, 真值瑕疵 | [final-runtime-convergence-plan](./p1-runtime-convergence-cleanup.md) + [t0-contract](./t0-cross-language-contract-testing-plan.md) 第0步 | 减法，风险最低 |
| **W1 主线对接** | ⑤前端接入 `/journey` 的 available_actions ⑥posture 结果注入 consultation + assessment 复用 | P3, P4 | [health-journey-frontend](./p3-health-journey-activation.md) + [posture](./posture-photo-analysis-plan.md) P3-B1 | 对接已有后端 |
| **W2 安全补强** | ⑦`/runtime` 诊断/训练节点前置 AIOutputGuard 强制关卡 | P2 | [governance-enforcement](./p2-output-governance-gate.md) | 安全属性 |
| **W3 深化** | 沿用团队既定：T0 事件溯源断线续传、契约全事件门禁、HITL 生命周期、posture P2/P3-B2、training schedule 未落地功能 | — | 各 T0 计划 + posture P2+ | 既定深化 |
| **随手** | 更新架构文档正文匹配 ADR 0002；修正三处滞后进度标注；业务 DTO 逐步收敛到 contracts | 文档漂移 | [README](./README.md) | 低优先级 |

**为什么这样排**：团队当前把注意力放在运行时底座深化（T0 + final-runtime），但底座已是全项目最强部分（`project-review-2026-07-10.md` 也评"差距不在架构，而在少数运行时细节"）。真正拉低产品完成度的是**主线断裂（P3/P4）**与**安全承诺未兑现（P2）**，而这三者都不需大重构——后端能力都在，缺的是最后一公里对接。W0→W1→W2 做完，产品才真正像一个 Agent 原生的体态助手，而非"底座漂亮但用户还在自己找路"的应用。

---

## 6. 文档 ↔ 代码漂移清单（供 §W3"随手"修正）

**声称待实现，实为半成品 / 旁路：**
- `confirm_action` 工具——文档列"待实现"，实际**完全不存在**（仅未用枚举 `REQUIRES_CONFIRMATION`）。
- AI 输出治理"先治理再落库"——文档承诺，代码是旁路（P2）。

**已实现但文档标注滞后（低估完成度）：**
- `runtime_events` + `replayCompletedRun`——`stream-event-contract-runtime.md` 标 ~30%「未实现」，实际已运行。
- HITL / AskUserCard——`agent-tool-calling-runtime.md` 标「未实现」，实际完整链路已上线。
- HealthJourney——README 标 **0%**，实际后端完整实现（前端未接入，P3）。

**命名 / 术语漂移：**
- 文档里 `AIRunRuntime` Module 不存在，实为 `consultation.Runtime`；`ToolRuntime` 无独立类型，职责在 `agent_tool_service.go`。
- `interaction` vs `interrupt` 命名跨文档未统一；`knowledge_entries`(旧) vs `knowledge_units`(新) 混用；job 状态 `completed` vs `succeeded` 并存；assessment `dimension_scores` 前端 5 项 vs spec 3 项。
- 七份架构文档正文仍描述已被 ADR 0002 否定的 `chat/send`+`ContextBuilder` 路线，仅顶部加 banner，正文未清理。

---

## 7. 交付物

本轮审查产出以下文档（均在 `docs/plan/active/`）：

1. 本文档 `architecture-review-2026-07-26.md` —— 审查结论 + 路线图（总纲）。
2. [p1-runtime-convergence-cleanup.md](./p1-runtime-convergence-cleanup.md) —— W0 死代码收敛 + 真值边界瑕疵（P1 + 真值瑕疵）。
3. [p2-output-governance-gate.md](./p2-output-governance-gate.md) —— W2 治理强制关卡（P2）。
4. [p3-health-journey-activation.md](./p3-health-journey-activation.md) —— W1 前端接入 journey（P3）。
5. [README.md](./README.md) —— active 计划总索引 + 依赖顺序 + 漂移校正。

P4 承接进 [posture-photo-analysis-plan.md](./posture-photo-analysis-plan.md)，P5 承接进 [t0-cross-language-contract-testing-plan.md](./t0-cross-language-contract-testing-plan.md)，不新建文档（见 §5）。
