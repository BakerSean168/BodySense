# 05 · 练习任务清单（用真实待办练手）

> 目标读者：正在学 Go / Python / React 基础的你。
> 这些任务**全部来自本项目真实的、尚未实现的待办**（`docs/plan/active/*` 与 `docs/project-review-2026-07-10.md`），
> 不是玩具题。做完一个，就真的往项目里合并一个 PR。
>
> 每个任务包含：**练什么技能 · 难度 · 目标 · 涉及文件（含撰写时锚点）· 约束 · 什么算做好了（验收）· 教练提示入口**。
> `file:line` 为撰写时锚点，动手前请以最新代码为准。
>
> 配合使用：本清单是"要做什么"，`.practice-map/`（见第 06 步/学习方案）是"按什么顺序做、做到哪了"。

---

## 难度分层总览

| 层级 | 定位 | 适合阶段 | 任务 |
|---|---|---|---|
| 🟢 L0 热身 | 单文件、有明确正确答案、能立刻验证 | 刚学完基础语法 | P1 日志开关、P2 类型标注补全 |
| 🟡 L1 缺陷修复 | 改动 1–2 个文件，需理解运行时行为 | 能读懂闭环代码后 | P3 异步连接池、P4 真实 embedding、P5 CPU 阻塞下沉 |
| 🟠 L2 功能扩展 | 跨 2–3 文件、复用已有骨架 | 能独立读一条闭环链路 | P6 体态诊断 Agent 工具、P7 姿态估计几何量化 |
| 🔴 L3 跨端契约 | 改动 Go+Python+TS 三端 + 契约测试 | 熟悉三端协作后 | P8 诊断台多模态输入、P9 契约测试扩展 |

建议顺序：**P1 → P2 → P3 → P5 → P4 → P6 → P7 → P9 → P8**。先热身建立信心，再修缺陷理解运行时，最后做跨端大件。

---

## 🟢 L0 — 热身（当天可完成）

### P1 · 全栈 debug 日志统一开关（React / TS）
- **练什么**：`import.meta.env.DEV`、副作用与纯函数边界、模块级工具函数封装。
- **难度**：★☆☆☆☆
- **目标**：把散落在前端的 `console.debug/warn` 收敛到一个可开关的 `debug(...)`，发布构建自动剥离；顺便恢复 reducer 的"纯度"。
- **涉及文件**（对应缺陷 W-1）：
  - `apps/web/src/features/consultation/hooks/useSSEProcessor.ts:147,181-195`
  - `apps/web/src/features/consultation/runtime/activeTurnReducer.ts:126,336,440`（**纯 reducer 里打日志是副作用，要移除**）
  - `apps/web/src/features/consultation/pages/ConsultationPage.tsx:329-363,711`
  - `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts:103,139,157`
- **约束**：不改变任何业务逻辑；reducer 内不得再有 I/O / 日志；新增 `debug` 工具放到 `apps/web/src/lib/` 或既有 utils。
- **什么算做好了**：
  1. 生产构建（`vite build`）产物里 grep 不到这些 `console.debug`；
  2. `activeTurnReducer.ts` 对同一输入永远返回同一输出（可加一个纯度单测）；
  3. 开发时设个环境变量仍能看到日志。
- **教练提示入口**：不确定"怎么让 reducer 变纯"时，向我要 `Hint`（会先给方向，再给小例子，最后才给代码）。

### P2 · 补全 Python 类型标注 + Pydantic 校验（Python）
- **练什么**：类型提示、`Literal`、`X | None`、Pydantic `Field(...)` 约束。
- **难度**：★☆☆☆☆
- **目标**：挑一个现有的分析结果模型，把宽松字段收紧为带约束的类型（如角度用 `float` + 取值范围、`view` 用 `Literal["front","side","back"]`）。
- **涉及文件**：`apps/ai-service/src/models/posture.py`（对照 `03/02` 文档里讲过的 Pydantic 用法）。
- **约束**：不破坏现有序列化输出的 JSON 形状（字段名不变）；`mypy`（若配置）与 `pytest` 全绿。
- **什么算做好了**：给一个越界值（如角度 999）时 Pydantic 直接抛校验错误，而不是静默通过。
- **教练提示入口**：想先自己写、只要我 review，就说 `Review`。

---

## 🟡 L1 — 缺陷修复（0.5–1 天/个，投入产出比最高）

### P3 · 知识库改异步连接池（Python）⭐最关键
- **练什么**：`async/await` 真义、事件循环阻塞、连接池、上下文管理器 `async with`。
- **难度**：★★☆☆☆
- **目标**：把 `async def` 里混用的**同步单连接 psycopg** 换成 `AsyncConnectionPool`，消除对 asyncio 事件循环的阻塞。
- **涉及文件**（对应缺陷 A-1，也是体态分析 Phase 2 的"前置修复"）：
  - `apps/ai-service/src/rag/knowledge_library.py:114`（`_get_connection` 单连接）
  - 同文件 `:120 ingest`、`:275 search`、`:427 stats`（都是 `async def` 却调用同步 `psycopg.connect()` / `cur.execute()`）
- **约束**：`psycopg[binary,pool]` 依赖已装（`pyproject.toml`），无需新增；对外方法签名不变；池大小可配。
- **什么算做好了**：
  1. 所有 DB 调用走 `async with pool.connection()`；
  2. 并发多个检索请求时，不再互相阻塞（可用两个并发请求 + 计时粗验）；
  3. 现有 RAG 相关测试全绿。
- **降级方案**（若池改造太重）：最低限度用 `await asyncio.to_thread(...)` 包裹同步调用，也算达成"不阻塞事件循环"这一核心目标。
- **教练提示入口**：这题概念密度高，建议先要一次 `Explain`（讲清"为什么同步 DB 会卡住整个服务"），再动手。

### P4 · 生产切换真实 embedding 并统一维度（Python）
- **练什么**：provider 抽象、配置驱动、维度契约、OpenAI 兼容接口调用。
- **难度**：★★★☆☆
- **目标**：把生产默认的"哈希伪 embedding"换成真实语义向量，并统一向量维度。
- **涉及文件**（对应缺陷 A-2）：
  - `.env.production:52` `EMBEDDING_PROVIDER=hashing` → 改真实 provider
  - `apps/ai-service/src/rag/embedding.py:17,21,49`（`_hash_embedding` 保留为离线测试兜底，别删）
  - 维度不一致坑：`.env.production:53` 是 384，`embedding.py:17` 默认 1536 —— 必须统一，否则 pgvector 列维度对不上会报错/静默错。
- **约束**：`hashing` 作为**离线测试兜底**保留（这是好的测试设计）；维度变更要有对应迁移。
- **什么算做好了**：
  1. 生产配置指向真实 embedding（如 `bge-m3` / `text-embedding-3-small` / 硅基流动等 OpenAI 兼容）；
  2. 维度全链路一致（env、代码默认、pgvector 列、迁移）；
  3. **加分**：做一个小检索评测集（命中率 / MRR），用数据证明"换了之后召回提升 X%"。
- **教练提示入口**：想练"用数据说话"，让我帮你设计那个评测集（`Practice`）。

### P5 · 本地 embedding 的 CPU 阻塞下沉（Python）
- **练什么**：区分 I/O 阻塞与 CPU 阻塞、`asyncio.to_thread`、线程池。
- **难度**：★★☆☆☆
- **目标**：把 `async def` 里直接调用的同步 CPU 密集 `model.encode(...)` 挪到线程，避免卡事件循环。
- **涉及文件**（对应缺陷 A-4）：`apps/ai-service/src/rag/embedding.py:108,134`（`model.encode(...)` 同步 CPU 阻塞）。
- **约束**：改成 `await asyncio.to_thread(self._local_model.encode, texts)`；行为等价。
- **什么算做好了**：本地 embedding 生成期间，其他 async 请求不再被饿死。
- **教练提示入口**：做完 P3 再做 P5，你会更懂"阻塞事件循环"这个主题的两种形态（I/O vs CPU）。

---

## 🟠 L2 — 功能扩展（1–3 天/个，复用已有骨架）

### P6 · 诊断台"体态档案"Agent 工具（Python，低成本高价值）
- **练什么**：tool-calling 架构、Agent 工具注册、复用现有分析结果、不引入新范式。
- **难度**：★★★☆☆
- **目标**：让问诊 Agent 能**引用用户已有的体态分析结果**（Phase 3-B1）。用户上传三视角照片已能分析（Phase 1 已上线），现在让对话里的 Agent 能读到这份档案。
- **涉及文件 / 骨架**：
  - 复用现有 tool-calling 架构（见 `t0-hitl-agent-runtime-plan.md` 里描述的 `ask_user` 工具模式，同一套注册机制）
  - 分析结果来源：Phase 1 已写入的 `user_uploads.analysis_result`（见 `posture-photo-analysis-plan.md`）
- **约束**：**不改契约层**（这是它"低成本"的原因）；只新增一个只读工具。
- **什么算做好了**：
  1. Agent 在合适时机调用该工具，拿到结构化体态数据；
  2. 回答里能引用"你的档案显示头前移 X°"这类具体内容；
  3. 无档案时工具优雅返回"暂无数据"，Agent 不崩。
- **教练提示入口**：先读第 04 篇《闭环功能》里的"流式问诊闭环 + 工具调用"，再动手。

### P7 · 姿态估计 + 几何量化（Python，核心亮点）
- **练什么**：可选依赖管理、OpenCV/MediaPipe 关键点、几何计算（颅椎角/肩倾角/骨盆倾角）、"可复现的量化"。
- **难度**：★★★★☆
- **目标**：把 Phase 1 纯 VLM 的"定性描述"升级为**关键点 + 几何角度的可信量化**（Phase 2）。
- **涉及文件 / 决策**：
  - 需新增可选依赖：`mediapipe` 或 `ultralytics` + `opencv`（`pyproject.toml` 目前无）
  - 路线对比见 `posture-photo-analysis-plan.md` §2（方案 A 纯 VLM vs 方案 B 姿态估计）
  - **前置**：强烈建议先做 P3（异步连接池），多模态负载更重会放大 A-1 问题。
- **约束**：依赖设为**可选**（没装时回退到 Phase 1 的 VLM 路径，不能让服务起不来）；单张照片有深度缺失，正/侧面各测各的。
- **什么算做好了**：
  1. 侧面照能算出颅椎角、骨盆前倾角等**具体、可复现**的数值；
  2. 同一张图多次运行结果一致（不像 VLM 会飘）；
  3. 结果 JSON 里区分"几何测量值"与"VLM 解释性文字"。
- **教练提示入口**：这题大，建议开一个专门的 practice map 分里程碑推进。

---

## 🔴 L3 — 跨端契约（改动三端，最能体现工程能力）

### P8 · 诊断台多模态输入（Go + Python + TS + 契约测试）
- **练什么**：三语言契约一致性、消息 parts 结构扩展、多模态内容块透传、端到端联调。
- **难度**：★★★★★
- **目标**：让用户能在**问诊会话里直接传图**（Phase 3-B2），而不只是引导页上传。
- **当前为什么不支持**（`posture-photo-analysis-plan.md` §1.2 第 3 点）：
  - 输入框只有 `<textarea>`（`apps/web/.../AssistantChatPanel.tsx:163`）
  - 消息 parts 只含 `text`，Go `messagePartsToText`（`runtime.go:1289`）只取 text
  - AI 输入 `ConsultationUserInput{Text}` 是纯文本
  - 助手渲染还显式 `Image: () => null`
  - **好消息**：多模态底层已通 —— `ai/types.py:32` 的 `ChatMessage.content` 已支持 `list[dict]`（OpenAI vision 内容块），provider 直接透传
- **约束**：**必须同步三方契约测试**（见 P9 / `t0-cross-language-contract-testing-plan.md`）；改动 parts 结构后，Go/Python/TS 对同一 fixture 的解析都要过。
- **什么算做好了**：
  1. 前端能在对话里选图并发送；
  2. Go 正确把 image part 透传给 Python，不再只取 text；
  3. Python 把图拼进 `ChatMessage.content` 的 vision 内容块喂给 VLM；
  4. 三端契约 parity 测试全绿。
- **教练提示入口**：这是"毕业设计级"任务，务必先做 P9 摸清契约测试机制。

### P9 · 跨语言契约测试扩展（Go + Python + TS）
- **练什么**：单一真值（schema + fixture）、parity 测试、`json.Unmarshal` / `model_validate` 一致性。
- **难度**：★★★★☆
- **目标**：为新增的消息 part 类型（如 image part）扩展共享 schema/fixture，并让三端 parity 测试覆盖它。
- **涉及文件**（`t0-cross-language-contract-testing-plan.md`）：
  - `packages/contracts/schemas/stream-event.v1.schema.json`（JSON Schema draft 2020-12）
  - `packages/contracts/fixtures/stream-events.v1.json`（黄金样本，三端喂**同一份字节**）
  - Python：`apps/ai-service/tests/unit/test_stream_event.py:36`（`model_validate` 逐条解析）
  - Go：`apps/api/internal/dto/stream_event_test.go`（`json.Unmarshal` + round-trip）
  - TS：`packages/contracts/src/stream-events.ts`（类型层，编译期保证）
- **约束**：改 schema 必须同步更新 fixture 和三端测试，一处漏改就应让测试变红。
- **什么算做好了**：故意把 Go DTO 字段名改错时，parity 测试立刻失败（证明它真的在防契约漂移）。
- **教练提示入口**：这题是理解"资深工程师怎么防三端不一致"的最佳样本，配合第 04 篇一起看。

---

## 怎么用这份清单

1. **别贪多**：一次只开一个任务，做完合并再开下一个（对应 coach 的"短 rep 优于大项目"）。
2. **先说你想怎么学**：
   - 想自己写、只要点评 → 跟我说 `Review`；
   - 卡住了、但不想直接要答案 → 跟我说 `Hint`（我会分层给：方向 → 概念/文件 → 小步骤 → 伪代码 → 代码）；
   - 概念没懂 → 跟我说 `Explain`。
3. **进度落到 `.practice-map/`**：每个任务的里程碑、当前焦点、下一步都记在那里，跨会话也不丢。

> What you learned：真实待办 = 最好的练习题；难度分层让你先建立信心再啃硬骨头。
> Next rep：从 **P1（日志开关）** 开始——它单文件、可立即验证，最适合起步。
