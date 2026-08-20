# BodySense 统一学习路线

> 更新时间：2026-08-18
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

## L1 · Diagnosis Typed Agent — 当前阶段，约 80% 完成

目标：把 Diagnosis 的普通候选生成路径迁到 typed PydanticAI Agent，同时保留 BodySense 已有业务契约。

已经完成：

- Longitudinal BodyState 成为 Diagnosis 的主要输入真相源。
- 每次 Diagnosis pin 精确 BodyState revision。
- `DiagnosisCandidateDraft` / `DiagnosisAgentOutput` typed models。
- `completed` 至少 1 个 candidate；insufficient/safety-blocked 可为 0；候选数量不再上限 3。
- Go 继续拥有 durable analysis/candidate IDs。
- application constructor DI 与 PydanticAI run deps 分层。
- `Agent[DiagnosisDependencies, DiagnosisAgentOutput]`。
- `RunContext[DiagnosisDependencies]`。
- `EvidenceSearcher(Protocol)`。
- `@agent.tool search_evidence`。
- ToolCall / ToolReturn / output tool 的区别。
- `capture_run_messages()` 的 focused 测试。
- run-scoped `retrieved_evidence` evidence trail。
- 按 `evidence_id` 去重。
- 以上 Diagnosis Agent/model 学习与回归测试当前为 14 passed。

下一步只剩三块：

### L1.1 · Production model routing seam — 已完成

当前真值：

```text
普通 LLM 调用
AIService -> BodySense logical route -> LiteLLM gateway -> provider / retry / fallback

Typed PydanticAI Agent
Agent -> OpenAIProvider(internal LiteLLM endpoint) -> logical model group
      -> LiteLLM -> provider / retry / fallback
```

关键结论：

- Python application 不再构建 physical provider clients 或 fallback chain。
- `models.yaml` / `ModelRouter` / PydanticAI `FallbackModel` 已退休。
- Mimo/OpenRouter LLM credentials 只进入 LiteLLM gateway；ai-service 只拿内部 gateway credential。
- business code 选择 logical route / immutable AgentConfiguration；LiteLLM 独占 provider selection 与 fallback。

### L1.2 · DiagnosisService cutover — 已完成

只切换“普通候选生成”分支，继续保护：

- safety gate；
- BodyState revision；
- Go-owned IDs；
- Diagnosis history persistence；
- HTTP `diagnoses` compatibility；
- governance；
- Consultation LangGraph runtime；
- Treatment legacy path。

### L1.3 · Evals / repair / regression

建立少量 golden cases 与错误修复闭环，确认 typed Agent 的输出质量和失败语义，再结束 Diagnosis 阶段。

预计剩余有效学习 + 编码时间：**4~6 小时**。

## L2 · Treatment Typed Vertical Slice

目标：在 Diagnosis 边界稳定后，把 Treatment 从 legacy 路径迁成基于明确 Diagnosis/BodyState revision 的下一纵向切片。

重点学习：

- 输入/输出 ownership。
- Diagnosis candidate assessment 如何影响 Treatment。
- structured output 与 runtime validation。
- safety constraint 与拒绝/降级语义。
- durable Treatment identity、history 和 outcome ownership。
- Go 与 Python 的边界如何保持单一 truth owner。

第一版只完成一条最窄可验证主流程，不扩张到全部治疗/训练功能。

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

从 2026-08-18 当前 checkpoint 计算：

| 阶段 | 预计有效时间 |
|---|---:|
| L1 Diagnosis 收尾 | 4~6 h |
| L2 Treatment slice | 5~8 h |
| L3 Streaming + React/TS/Go | 6~10 h |
| L4 Async/RAG | 4~6 h |
| L5 独立交付 | 5~8 h |
| **合计** | **24~38 h** |

这个时间指真正“读代码 + 思考 + 自己写 + 测试 + 复盘”的有效时间，不含长时间等待模型、依赖下载或 CI。

如果目标不是覆盖全部五语言知识，而是尽快达到“能独立继续推进 BodySense”的实战水平，优先完成 **L1 -> L2 -> L5**，再补 L3/L4，约 **14~22 小时**即可形成一个更实用的第一阶段闭环。

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
