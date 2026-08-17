---
id: bodysense-fundamentals
title: BodySense 五语言进阶（Go / Python / JavaScript / TypeScript / React）
status: active
level: beginner
language: go, python, javascript, typescript, react
created_at: 2026-07-13
updated_at: 2026-08-15
---

# Goal

用 BodySense 这个真实项目当教材，把 **Go / Python / JavaScript / TypeScript / React** 五门语言的基础和工程心智打扎实：
不是背语法，而是能**读懂**项目里的核心代码、能**改**已有功能、最终能**独立实现**一个未完成的待办并合并 PR。

# Why

- 你手里就有一个 ~41k 行、三语言、结构清晰的真实工程，比任何教程都真实。
- 三端各自代表一类典型范式：Go（分层、并发、持久化）、Python（FastAPI、async、Pydantic）、JavaScript/TypeScript/React（流读取、类型契约、Reducer 与状态边界）。
- 学完能直接兑现价值：项目里有一批**明确、有限、可闭环**的待办（见 `docs/learning/05-practice-tasks.md`），学一个就能交付一个。

# Milestones

- **M1 · 读懂五语言基础**（阅读为主）
  - 读 `docs/learning/01-go-fundamentals.md` → 能说清 package main / 分层 / `:=` vs `=` / 多返回值+error / struct tag / 依赖注入。
  - 读 `docs/learning/02-python-fundamentals.md` → 能说清 async/await / Pydantic / 类型提示 / 异常链。
  - 读 `docs/learning/03-react-fundamentals.md` → 能说清 `UI=f(state)` / 各 Hook 用途 / Zustand vs TanStack Query（客户端态 vs 服务端态）。
  - 读 `docs/learning/06-javascript-typescript-streaming.md` → 能说清字节块、文本行、协议事件、运行时校验和 reducer 的分层。
  - 自测：每篇末尾的 self-check 全部能口头回答。
- **M2 · 读懂一条闭环**（阅读 + 画图）
  - 读 `docs/learning/04-closed-loop-features.md`，选"登录闭环"手画一遍时序图（前端→Go→DB）。
  - 目标：能指出每一步"为什么这么做"（如 `json:"-"` 防密码泄漏、JWT alg 校验防混淆攻击）。
- **M3 · 热身改一处**（动手，L0）
  - 完成新版 `P1 调试日志` 或 `P2 姿态校验`。目标：跑通“改代码→测试绿→复盘”的完整循环。
- **M4 · 建立安全边界**（动手，L1）
  - 完成 `P3 StreamEvent 运行时解析` + `P4 流分块测试`。目标：不再把类型断言误当作数据校验。
- **M5 · 修异步正确性**（动手，L2）
  - 完成 `P5 异步连接池` + `P6 CPU 阻塞下沉`。目标：区分同步 I/O 和 CPU 工作如何阻塞事件循环。
- **M6 · 打通流式恢复**（动手，L3）
  - 完成 `P7 统一事件校验` → `P8 事件续传与取消`。目标：跨 TS/React/Go 保持事件正确性。
- **M7 · 独立交付**（动手，L4）
  - 完成 `P9` 的一个纵向切片，从问题定义、契约、测试到用户验收独立闭环。

# Current Focus

**M1 · Diagnosis 学习检查点（BodyState 基础迁移已写入代码）** —— 2026-08-15 已把第一批 production refactor 推进到 Diagnosis 边界：长期 Conversation、BodyState revision、Fact/Observation、Consultation producer/context、安全 gate、0..N Diagnosis candidates、Diagnosis history 与 user candidate assessment 都已经落到代码。Treatment 尚未迁移。

现在恢复“边学边重构”的节奏，重点学习 Diagnosis execution layer，而不是继续自动往 Treatment 推进。现有 constructor DI / consumer-owned `AIExecutor(Protocol)` 会保留；下一步在这个正确的 BodyState 输入边界上学习并接入 PydanticAI，再学习 `RunContext/deps/tools` 和 targeted evidence-gap RAG。

当前学习顺序：`读懂 BodyState→Diagnosis 数据流 → Review typed Diagnosis models/status/0..N candidates → PydanticAI Agent → RunContext/deps → targeted RAG tool → evals/repair → 再进入 Treatment`。

# Exercises

练习任务的完整清单、难度分层、验收标准见 → `docs/learning/05-practice-tasks.md`

推荐顺序（与里程碑对应）：
- L0：`P1` 调试日志 · `P2` 姿态校验
- L1：`P3` 运行时事件解析 · `P4` 流分块测试
- L2：`P5` 异步连接池 · `P6` CPU 阻塞下沉
- L3：`P7` 统一事件校验 · `P8` 续传与取消
- L4：`P9` 独立纵向切片

# Session Log

## 2026-07-13

- 创建学习方案与 practice map，状态置为 `active`。
- 已产出配套教材：`docs/learning/01~04`（Go/Python/React/闭环，含真实代码逐行注释）+ `05`（练习任务清单）。
- 当前焦点定为 M1（读懂三端基础），起点为 Go 基础文档。

## 2026-07-29

- 同步最新 `origin/dev` 后重新核对学习资料与代码。
- 原 P6–P9 的体态工具、姿态估计、多模态和契约基础已经落地，不再列为待实现。
- 新增 JavaScript/TypeScript 流式链路教材，并把路线扩展为五语言。
- 当前仍处于 M1，但完成标准增加“能解释从网络字节到可信 React 状态”的完整分层。

## 2026-08-07

- 学习者尚未开始，从零启动。整理确认 M1 阅读顺序：**01 → 02 → 03 → 06**（06 依赖 03 的 React 心智，不是起点）。
- 修正 Next Step 回到 01，与 M1 一致。

## 2026-08-11

- **M1 第 1 步完成**：读 `01-go-fundamentals.md` §0–§8，理解分层（handler/service/repository）、变量与错误处理、struct tag、防账号枚举。
- 实战：给 `auth_handler.go` 的 `Login` 逐行写注释并两轮 Review。
  - 通过：LastLoginAt 指针语义、`c.Request.Context()` 为什么要传 (超时/横切数据)、防枚举 why。
  - 已修正：respondError 行不对物的注释、authService.Login typo。
  - 待巩固：手绘 Login 完整分层流（handler→service→repository）一次。

## 2026-08-14

- 学习主线切到真实的 Diagnosis vertical slice，不再要求先线性读完 Python 教程。
- 原学习路线里的 **TICKET 0 / Python Diagnosis contract freeze 已实施到代码**：`test_diagnosis_api_contract.py` 与 `test_diagnosis_service.py` 已建立 HTTP/service characterization tests，并加入大量逐行学习注释。
- Go 侧 `DMR-002` characterization/target test 也已存在；`ready_for_analysis` authoritative gate 仍作为后续 target contract 保持 `t.Skip`。
- 识别出新的学习断点：`DiagnosisService.__init__()` 当前直接 `AIService()`，service 同时承担“使用依赖”和“创建依赖”；测试因此依靠 monkeypatch 构造器。
- 已把 active refactor plan 调整为 **DMR-100 DI-first**：先学 constructor injection / dependency inversion / composition root，再进入 typed models 与 PydanticAI。
- 明确区分：应用层 Dependency Injection ≠ PydanticAI `deps_type` / `RunContext` runtime dependencies。
- **DMR-100 第一小步已实施（2026-08-14）**：`DiagnosisService` 改为 constructor injection；`get_diagnosis_service()` 成为当前 composition root，负责创建 `AIService()` 并注入；当时测试先从 patch `AIService` 构造器迁移为显式 Fake 注入，为下一步抽 Protocol 暴露真实使用面。
- DMR-100 的第二小步已完成：从 `DiagnosisService` 的真实使用面提取最小 `AIExecutor(Protocol)`，constructor 不再类型依赖 concrete `AIService`；测试 Fake 也已去掉 `AIService` 继承，仅凭 `generate()` 方法结构满足 Protocol。由此可以对照理解：DI 解决“谁创建/注入依赖”，DIP 解决“高层模块依赖抽象还是具体实现”，Protocol 则是 Python 表达该能力抽象的一种方式。
- HTTP contract tests 仍在 route/provider 边界 monkeypatch `get_diagnosis_service()`；这是合理的边界替换，与劫持 service 内部构造器不同。
- 当前远程 `@agent` 文件系统没有命令执行接口，执行容器也未挂载 `/home/ubuntu/projects/bodysense`，因此本次 focused pytest 尚未实际运行；不能把测试状态记为已验证。
- **DMR-101 第一小步已实施（2026-08-15）**：新增 `src/models/diagnosis.py`，把 `confidence/severity` 从无限制 `str` 收窄为 `StrEnum`，并建立 `DiagnosisCandidateDraft` / `DiagnosisAgentOutput`。`DiagnosisAgentOutput` 内部使用 `candidates`，刻意与当前 HTTP `diagnoses` 命名解耦；同时把 prompt 的“1-3 个候选”规则编码进 Pydantic runtime constraint。
- 新增 `tests/unit/test_diagnosis_models.py` 作为 target/domain tests：与旧 characterization tests 区分，验证“从现在开始允许哪些状态存在”。
- **DMR-101 第二小步已实施（2026-08-15）**：新增 `DiagnosisDependencies(dataclass, slots=True)`，把单次 Diagnosis run 的 `extracted_info / profile / conversation_summary / rag_context` 聚合成显式 typed context。学习重点是区分两类“依赖”：`DiagnosisService(ai=...)` 是对象级 constructor DI，生命周期较长；`DiagnosisDependencies(...)` 是每次 run 都重新组装的上下文数据。
- `DiagnosisDependencies` 暂不包含 `use_case`（属于 execution/model-routing policy）和 `rag_results`（当前属于 generation 后 citations/governance 数据），也暂不把 nested profile/symptom dict 扩展成更多新模型，保持本小步边界。DMR-101 仍尚未安装/接入 PydanticAI，也尚未把这些 models 接进 `DiagnosisService`。

## 2026-08-15 · Longitudinal BodyState 领域模型定稿

- 经过复杂场景模拟，业务模型从“多 Consultation / Assessment / MedicalRecord 文档”收敛为“一用户一个长期健康工作区 + 一个 Longitudinal BodyState”。
- Conversation 只作为长期交互入口，不再作为 health truth；Consultation、Posture Analysis、Training 等都是 BodyState producer。
- Diagnosis / Treatment 变成 BodyState consumer，并必须关联明确 BodyState revision；历史分析不因后续状态变化被静默重写。
- 正式取消 MedicalRecord 作为核心 aggregate；历史追踪由 BodyState timeline + Diagnosis history + Treatment/outcome history 表达。
- 正式推翻 Diagnosis `1..3 candidates` 业务约束；复杂长期用户可按 Concern 产生实际需要数量的 candidates。
- 已生成/更新项目文档：ADR 0004、Longitudinal BodyState Domain Model、Longitudinal Health Loop、Longitudinal Body Health Feature Spec 和新的 active migration plan。
- 原 `diagnosis-medical-record-refactor-plan.md` 已归档；DMR-100/101 实现历史仍保留为学习与迁移证据。

## 2026-08-15 · BodyState → Diagnosis 第一批重构代码已写入

- Go 新增 Longitudinal BodyState persistence：`body_states / body_state_facts / body_state_observations / body_state_revisions`，以及 revisioned Fact correction、temporal update、Observation、SafetyState。
- 现有 `health_features` 右侧面板改成 BodyState 的兼容 projection：行项目携带 durable BodyState item ID，确认/修改/删除不再创建第二份 health truth。
- Consultation runtime 会把 extracted symptom、ask_user answer、用户 workbench edit 和 positive safety event 写入 BodyState；Python 下一轮上下文优先读取 Go-owned BodyState。
- 产品级 Conversation 行为收敛为长期复用：没有 conversation id 时复用该用户现有 consultation；前端 `/consultation` 会自动解析到已有长期会话。
- Diagnosis 新增 durable `diagnosis_analyses / diagnosis_candidates / diagnosis_candidate_assessments`，每次分析 pin 精确 BodyState revision，candidate ID 由 Go 分配。
- Python Diagnosis typed model 已正式删除 `max_length=3`；completed 支持实际需要数量的 candidates，`insufficient_information / safety_blocked` 允许 0 candidate。
- Web DiagnosisPanel 改成每个 candidate 独立 `confirmed / unsure / not_applicable`，不再“单选后立即生成 Treatment”；同时新增基础 Diagnosis history timeline。
- SafetyState 为 `requires_review` 时 Go 会直接持久化 `safety_blocked` DiagnosisAnalysis，不运行普通候选分析。
- Treatment 路径刻意保持 legacy，作为 Diagnosis 学习完成之后的下一阶段。
- 2026-08-15 已通过 `@agent-v2` 完成真实验证与 repair：Go 全量测试通过、Python 212 tests 通过、Diagnosis touched files Ruff/Pyright clean、Web 138 tests + typecheck + build 通过、contracts 通过，并在临时 PostgreSQL 数据库从 000001 完整迁移到 000033 成功。该 Diagnosis checkpoint 现可记为 locally green；仓库范围仍存在与本次无关的历史 Pyright 债务。

## 2026-08-15 · DX-001 Step A：PydanticAI 平行学习跑道

- 新增 `pydantic-ai-slim[openai]>=2.31.0`，避免安装暂时不用的全 provider 元包。
- 新增 `src/agents/diagnosis_agent.py`：`Agent(..., deps_type=DiagnosisDependencies, output_type=DiagnosisAgentOutput)`；动态 instructions 通过 `RunContext[DiagnosisDependencies].deps` 读取精确 BodyState revision/state/history/profile。
- 新增 `tests/unit/test_diagnosis_agent.py`，使用 PydanticAI `TestModel` 验证 `result.output` 直接得到 `DiagnosisAgentOutput`，无需 `json.loads -> model_validate`。
- 本步刻意不切换 production `DiagnosisService`：当前 `AIService` 还负责 Mimo/OpenRouter routing/fallback，下一步要先学习并决定 PydanticAI model/provider/fallback adapter 的 ownership，不能为接 Agent 偷偷丢掉现有容错契约。
- 验证：Diagnosis focused 17 passed；新增文件 Ruff clean / Pyright 0 errors；Python full 214 passed。

# Next Step

**DX-001 Step B：读懂并设计 production model routing seam。** 对照现有 `AIService -> ModelRouter -> OpenAICompatibleProvider` 与 PydanticAI `Model / OpenAIProvider / FallbackModel`，明确哪些能力应由 PydanticAI 接管、哪些配置语义需要保留。然后再把 `DiagnosisService` 的普通候选生成分支切到 typed Agent；继续保护 safety gate、BodyState revision、Go-owned IDs、HTTP `diagnoses` compatibility、governance 和 Treatment legacy path。
