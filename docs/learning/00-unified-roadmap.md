# BodySense 统一学习路线

> 更新时间：2026-08-22
>
> 这份文档是当前唯一的**学习主线**。`01~06` 是按主题查阅的教材；`05-practice-tasks.md` 是可选练习题池，不再承担主线排期。

## 目标

继续直接在已经重构后的 BodySense 主仓库中学习，而不是维护一份长期分叉的 learning snapshot。

学习方式固定为：

```text
理解当前真实代码
  -> 明确一个小边界
  -> 自己实现/修改
  -> focused tests
  -> wider validation
  -> 复盘并沉淀知识
  -> 继续推动 production 代码
```

最终完成标准不是“把教程读完”，而是能够在 BodySense 中独立完成一个跨前端、Go API、Python AI Service 的纵向改动，并知道哪些状态和契约不能破坏。

## 单一编号体系

从现在开始只使用 `L0 ~ L5` 六个阶段，不再同时维护旧的 `M1~M7`、`P1~P9`、`DMR-*` 作为学习进度编号。

旧编号仍保留在历史 commit、ADR、测试名和文档中，作为项目演进证据；但当前学习进度只看本文件与 `.practice-map/maps/bodysense-fundamentals.md`。

## L0 · 工程基础与真实项目心智模型 — 已完成主要部分

目标：能读懂项目三服务边界、Go 分层、Python 类型边界和前端基本状态流。

已经实际学习/验证：

- Go `handler -> service -> repository` 分层。
- constructor DI、composition root、DIP。
- Python `Protocol` 与 structural typing，并能用 Go interface 类比。
- Pydantic model / dataclass / `default_factory` / `slots=True`。
- Python 对象引用、`dict` 与对象属性访问、`append`/`extend`、comprehension。
- Ruff 与 Pyright/Pylance 的职责区别。

参考教材：

- `01-go-fundamentals.md`
- `02-python-fundamentals.md`
- `03-react-fundamentals.md`
- `06-javascript-typescript-streaming.md`

L0 不再要求按顺序重新读完；遇到代码断点时回查即可。

## L1 · Diagnosis Production Agent — 已完成（100%）

目标已经从最初的“把普通候选生成迁到 typed PydanticAI Agent”扩展并完成为：

> 从 **Diagnosis 怎么调用一个模型**，一路掌握到 **怎样证明一套 Agent Configuration 有资格上线，以及上线后怎样证明某个具体 case 有资格由它自动处理**。

已经实际学习、实施并验证：

### L1.1 · Typed PydanticAI execution boundary — 已完成

- Longitudinal BodyState 是 Diagnosis 的主要 durable truth source；每次运行 pin 精确 BodyState revision。
- `DiagnosisCandidateDraft` / `DiagnosisAgentOutput` typed models。
- `Agent[DiagnosisDependencies, DiagnosisAgentOutput]`、`RunContext[DiagnosisDependencies]`。
- application constructor DI 与 PydanticAI run deps 分层。
- `EvidenceSearcher(Protocol)` 与 consumer-owned capability boundary。
- ToolCall / ToolReturn / structured-output tool 的区别。
- `capture_run_messages()` focused testing。
- run-scoped evidence trail 与稳定 `evidence_id` 去重。
- Go 继续拥有 durable analysis/candidate IDs；Python 只提出语义结果。

### L1.2 · Production model routing / LiteLLM seam — 已完成

当前真值：

```text
Business / Agent Configuration
  -> BodySense logical model
  -> internal LiteLLM gateway
  -> physical provider / retry / fallback
```

关键结论：

- BodySense 业务代码选择 logical route / immutable AgentConfiguration。
- LiteLLM 独占 provider normalization、physical routing、retry、cooldown 与 infrastructure fallback。
- application-owned `ModelRouter` / provider clients / PydanticAI `FallbackModel` 已退休。
- infrastructure fallback 与业务 fallback / abstain / escalate 明确分层。

### L1.3 · Immutable Agent Configuration + Qualification — 已完成

- Qualification / promotion 单位是完整 immutable Agent Configuration，而不是单独 foundation model。
- Configuration identity 覆盖 model policy、prompt、schema、tool/evidence/governance/decision policy 与行为相关 generation settings。
- Capability policy 使用 `REQUIRED / PREFERRED / OPTIONAL`，并允许 Eval evidence 反向修正最初假设。
- `diagnosis_qualification_v1` 覆盖 development / holdout / regression / challenge 与 critical-safety slices。
- deterministic evaluators 覆盖 response contract、status、candidate policy、side effects、configuration provenance 与 tool trace。
- Champion general qualification：**7/7**。
- paired non-inferiority 使用同一 dataset fingerprint、预声明 0.02 margin，并阻断 critical regression。
- 学会用 factorial / interaction thinking 区分 Model、Prompt 与组合 interaction effect，避免错误归因。

### L1.4 · EvidenceGap / Evidence Acquisition Runtime — 已完成

- `EvidenceGap` 从自由文本升级为 typed decision-relevant uncertainty。
- 区分 `user_information` / `external_knowledge` / `conflict` / `safety`。
- `LLM = proposer/reasoner`，`Runtime = recorder/verifier`，`Policy = authority`。
- `user_fact` 不能被通用 RAG 补成用户事实。
- targeted acquisition：每次搜索必须绑定明确 Gap。
- `EvidenceBudget` 是资源上限，不是“证据已经充分”的证明。
- typed `EvidenceAttempt` 记录真实 acquisition 与 stopping reason。
- `retrieved evidence != admissible evidence != gap resolved`。
- EvidenceGap policy suite：**5/5**。
- critical unresolved gap 即使被模型遗漏，也由 runtime 合并回最终 information gaps。

### L1.5 · SafetyEnvelope / deterministic DecisionAuthority — 已完成

- SafetyEnvelope 表示某个 Configuration 已经被证明可以 normal AUTO 的运行边界。
- DecisionAuthority 表示超出/进入边界后系统**真正允许执行什么动作**。
- Go 持有 deterministic deny-overrides policy；LLM confidence / Judge score 不是最终 authority。
- 主要 outcome：`allow-normal` / `allow-degraded` / `abstain` / `escalate` / `block`。
- unknown policy revision、unknown enum、矛盾 safety facts、malformed required facts 都 fail closed。
- `Reasoning Result` 与 `Authorized Business Result` 分离；policy 能真正抑制普通 candidate delivery。

### L1.6 · Durable Diagnosis Domain Model — 已完成

- Ephemeral Agent execution state 与 durable domain artifact 分离。
- `Durable != Aggregate Root`。
- `DiagnosisAnalysis` 是 pin 在精确 BodyState revision 上的 immutable historical artifact。
- 新 BodyState 产生新 Analysis，不回写旧 Analysis。
- `DiagnosisCandidate` = 某一次 Analysis 当时提出的候选。
- `BodyStateHypothesis` = 可跨 revision 演化的 longitudinal explanatory entity。
- Evidence 参与决策时保留 source/version/snapshot provenance，但不会自动升级为 User Fact。
- 影响历史 Decision 的 EvidenceGap / EvidenceAttempt 作为 durable audit facts 保留。
- 核心心智：`Past knowledge != current knowledge`，`Historical truth != current applicability`。

### L1.7 · DecisionTrace / Provenance / Replay — 已完成

- Observability Trace 回答“技术上发生了什么”；DecisionTrace 回答“业务上为什么允许/阻断”。
- Configuration Provenance = approved/intended；Execution Provenance = actually observed。
- 同一 `BodyState + Configuration` 仍可能因为实际 Evidence/provider/tool observations 不同而形成不同 Analysis。
- Historical Replay 使用冻结历史输入/版本解释当时行为。
- Counterfactual Replay 用同一个 frozen case 比较新 Configuration。
- Current Re-analysis 使用当前 BodyState 产生新的业务 Analysis，不属于 Replay。
- Replay 重点验证 Behavioral Contract / deterministic invariants，而不是逐 token 一致。

### L1.8 · Behavioral Contract / production failure attribution — 已完成

- Agent 可靠性不是要求所有 semantic output 固定，而是把随机性限制在稳定业务 contract 内。
- 区分 hard invariants、bounded semantic behavior、presentation variation。
- `Correct answer != valid execution`：即使最终猜对，非法 evidence path 仍属于失败。
- 排障沿 `Input -> Reasoning -> Evidence -> Runtime Facts -> DecisionAuthority -> Persistence -> Delivery` 寻找**第一个 contract violation**。
- 不再把所有 production failure 默认归因给模型或 Prompt。

### L1 完成标志

Diagnosis Agent Platform 的治理计划已经完成并归档；当前代码已经具备 immutable configuration、Pydantic Evals qualification、typed EvidenceGap/acquisition、Go SafetyEnvelope/DecisionAuthority、DecisionTrace/provenance、historical/counterfactual replay、Shadow/Canary/Promotion 与统一 LiteLLM routing。

本阶段知识已经同步整理到 Thought Forest，包括统一入口 `BodySense Diagnosis Agent Architecture` 以及 DecisionAuthority、Behavioral Contract、Evidence Admissibility、Durable Domain Model、Failure Attribution 等原子笔记。

**当前学习主线正式进入 L2。**

## L2 · Treatment Typed Vertical Slice — 当前下一阶段

当前仓库已经存在 production-shaped Treatment Agent 基础：typed `TreatmentAgentOutput`、immutable Treatment configurations、EvidenceGap challenger、qualification/evidence/promotion evals 与统一 LiteLLM gateway。因此这一阶段**不再从“把 legacy Treatment 迁到 PydanticAI”开始**，而是以现有实现为教材继续学习、审查和补齐 Treatment 的领域/权限边界。

目标：把 Diagnosis 学到的 production Agent 心智模型迁移到一个真正会产生 intervention/action proposal 的纵向切片，并理解“提出干预建议”与“允许执行/保存干预”之间更严格的 authority boundary。

重点学习：

- Treatment 输入/输出 ownership：BodyState、DiagnosisAnalysis/Candidate Assessment、Treatment proposal 各由谁拥有。
- structured Treatment proposal 与 runtime validation。
- EvidenceGap / Evidence Admissibility 在 Treatment 中与 Diagnosis 的异同。
- safety constraint、contraindication、abstain / escalate / human review。
- proposal authority 与 action/execution authority 分离。
- durable Treatment / revision / intervention identity、history 和 outcome ownership。
- Go 与 Python 如何继续保持单一 durable truth owner。
- Treatment qualification / promotion evidence 如何证明配置可上线，而不是只证明“模型会生成方案”。

第一轮学习仍保持最窄 vertical slice：先读透当前真实实现和 contract，再选择一个最小缺口做修改、测试、review 与沉淀。

预计：**5~8 小时**。

## L3 · Consultation Streaming 与 React/TypeScript 状态边界

目标：借现在已经重构后的 Workbench/Consultation UI 学会真实前端和跨层流式协议，而不是单独做玩具 demo。

重点：

- `unknown -> runtime validation -> trusted StreamEvent`。
- TypeScript discriminated union。
- `ReadableStream` / `TextDecoder` / 分块。
- reducer 纯函数与 Active Turn state machine。
- TanStack Query server state 与本地 UI state 边界。
- AbortController / cancellation。
- replay / `after_seq` / 幂等恢复。
- Go runtime event persistence 与前端消费之间的契约。

旧 `P3/P4/P7/P8` 可以作为这一阶段的题目池，但实施前必须先根据当前 main 代码重新确认是否仍有真实缺口。

预计：**6~10 小时**。

## L4 · Python Async / RAG 工程化

目标：从“会写 async def”推进到理解事件循环、I/O/CPU 阻塞和生产 RAG 数据路径。

重点：

- async DB pool 生命周期。
- CPU-bound embedding 下沉到 thread/executor。
- targeted retrieval 与 preloaded context 的边界。
- normalized evidence contract。
- citation/provenance。
- RAG eval / faithfulness。

旧 `P5/P6` 只作为候选练习；先验证当前代码是否仍存在相同问题。

预计：**4~6 小时**。

## L5 · 独立纵向交付

目标：不依赖逐步提示，独立完成一个真实 BodySense 小功能。

完成标准：

1. 自己写 problem / non-goals。
2. 找到并保护现有 contracts。
3. 画出 React -> Go -> Python/DB 的数据流。
4. 先写 characterization/golden test。
5. 实现最窄 vertical slice。
6. 覆盖 failure / retry / cancel / safety 中与该功能相关的路径。
7. 本地 Docker 或相应集成路径验证。
8. 自己完成 review / repair / PR。

预计：**5~8 小时**。

## 剩余时间估算

从 **2026-08-22 / L1 已完成** 的 checkpoint 计算：

| 阶段 | 预计有效时间 |
|---|---:|
| L2 Treatment slice | 5~8 h |
| L3 Streaming + React/TS/Go | 6~10 h |
| L4 Async/RAG | 4~6 h |
| L5 独立交付 | 5~8 h |
| **合计** | **20~32 h** |

这个时间指真正“读代码 + 思考 + 自己写 + 测试 + 复盘”的有效时间，不含长时间等待模型、依赖下载或 CI。

如果目标是优先达到“能够独立继续推进 BodySense”的实战水平，当前最短路径变为：

```text
L2 Treatment -> L5 独立纵向交付
```

预计约 **10~16 小时**。L3/L4 再作为前端流式协议与 Python Async/RAG 的专项补强。

## 教材如何使用

- `01-go-fundamentals.md`：Go 语法/分层断点时查。
- `02-python-fundamentals.md`：Python 类型、async、Pydantic 断点时查。
- `03-react-fundamentals.md`：React state/effect/context 断点时查。
- `04-closed-loop-features.md`：需要重新理解端到端业务闭环时查。
- `05-practice-tasks.md`：练习题池，不代表当前项目一定仍有这些缺口。
- `06-javascript-typescript-streaming.md`：进入 L3 时作为主要参考。

## 每次学习 Session 的固定结束动作

每次只更新 `.practice-map/maps/bodysense-fundamentals.md`：

- 当前阶段与百分比；
- 本次真正学会的知识；
- 实际修改的代码；
- 实际跑过的命令和结果；
- 下一个最小任务。

只有课程结构本身发生变化时才修改本路线。这样可以避免学习计划和 Session Log 反复互相覆盖。
