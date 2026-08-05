# BodySense 全面项目审查报告

> 审查日期：2026-07-10
> 审查范围：AI 服务（Python）、后端（Go）、前端（React）、工程化/DevOps、产品完整性、求职亮点
> 审查方式：逐文件阅读实际代码与配置（非仅看文件名）
> 定位：AI 应用开发学习项目 + 个人体态健康工具 + 求职作品集

---

## 0. 总体评价（TL;DR）

**这是一个远超"练手项目"平均水准的作品。** 它不是一个 CRUD demo，而是一套带有**事件溯源、人在回路（HITL）Agent 运行时、AI 输出治理、跨语言契约测试、自动化增量部署**的准生产级全栈系统。规模约 4 万行代码（Go ~1.47 万 / TS ~1.53 万 / Python ~1.06 万），三语言 Nx monorepo，功能覆盖了 PRD 的绝大部分。

**一句话总结**：架构与工程化的"骨架"非常成熟（这正是最难、最能打动面试官的部分），但有几处**运行时正确性/性能的"内脏"问题**会在被追问细节或做压测时暴露短板。把这几处补齐，它就是一份能直接进面试终面的作品。

### 评分卡（1–5，5 为优秀）

| 维度 | 评分 | 一句话 |
|---|:---:|---|
| 架构设计 | ★★★★★ | 事件溯源+投影+HITL+契约，思路清晰，远超同级 |
| AI 工程 | ★★★★☆ | Agent/治理/评估体系完整；被"默认哈希 embedding + 阻塞 DB"拖后腿 |
| 后端工程 | ★★★★☆ | 分层干净、安全扎实、幂等/可恢复；缺优雅关闭与可观测性 |
| 前端工程 | ★★★★☆ | 流式状态机设计优秀、性能有意识；巨型组件+残留调试代码 |
| DevOps | ★★★★☆ | 增量部署+缓存+契约测试出色；缺回滚、容器 root、CI 未用 affected |
| 可观测性 | ★★☆☆☆ | 全栈缺结构化日志/trace/metrics/LLM 成本计量（最大短板） |
| 测试 | ★★★★☆ | 三语言单测+契约测试+评估 harness；缺集成/E2E/覆盖率门禁 |
| 文档 | ★★★★★ | ADR、架构文档、术语表、计划归档，异常完整 |

---

## 1. 项目亮点（求职视角）—— 先讲好的

这些是**已经存在**、可以直接写进简历并在面试深挖的技术资产。建议按"能讲多深"排序主打前 3 个。

### 🏆 T0 级亮点（罕见，强烈建议作为简历主线）

1. **人在回路（HITL）的持久化 Agent 运行时**
   - Agent 通过 `ask_user` 工具**中断**一次生成，把待回答问题持久化为 `agent_interaction`，SSE 发出 `run.interrupted` 并原子地把助手消息落库为 `aborted`；用户回答后通过独立 HTTP 请求 `ResumeInteraction` **跨请求恢复**运行。
   - 代码：`apps/ai-service/src/services/agent/orchestrator.py:363`（中断）、`apps/api/internal/consultation/runtime.go:858`（`handleInteractionRequired`）、`runtime.go:327`（恢复）。
   - **为什么是亮点**：绝大多数候选人做的 Agent 是"一次性跑完"。跨 HTTP 请求的可中断/可恢复 Agent，是真正理解 Agent 运行时状态管理的体现。

2. **事件溯源 + 投影 + 流回放的流式架构**
   - 每条对外 SSE 事件都以 `runtime_event`（append-only）持久化（`runtime.go:1262 recordPublicEvent`），使得**幂等重放**成为可能：重复的 `request_id` 直接从事件日志回放完整流（`runtime.go:1010 replayCompletedRun`）。
   - 读模型走 `thread_projection`（投影），与写模型分离。
   - **为什么是亮点**：这是分布式系统/事件驱动架构的核心思想，能讲清楚 CQRS、幂等、回放，面试含金量极高。

3. **跨语言契约测试（Go / Python / TS 三方一致）**
   - 共享 `packages/contracts/schemas/stream-event.v1.schema.json` + `fixtures/stream-events.v1.json`，Go 和 Python 各自加载**同一份 fixture** 做一致性测试。
   - 证据：`apps/ai-service/tests/unit/test_stream_event.py:36`（`test_stream_event_fixture_parity`）、`apps/api/internal/dto/stream_event_test.go`。
   - **为什么是亮点**：多语言微服务最容易出的问题就是契约漂移。用 fixture 做三方 conformance test，是资深工程师才会做的事。

### 🥇 T1 级亮点（扎实，值得写）

4. **AI 输出治理（AI Safety / Governance）**
   - 可插拔策略的 `AIOutputGuard`（空输出/schema/red-flag/faithfulness），按严重度产出 accepted/degraded/rejected（`governance/output_guard.py`）。
   - RAG **忠实度校验**：治疗动作必须能在知识库检索结果中找到出处，否则标记 ungrounded（`faithfulness_checker.py`）。
   - **高召回**红旗症状检测：健康场景刻意偏向"宁可误报不可漏报"，并把设计理由写进 docstring（`red_flag_detector.py:2`）。
   - 治理结论持久化为 `ai_output_review`（`runtime.go:966`）。
   - **为什么是亮点**：健康 AI 的"安全护栏"是 2025-2026 最热的工程方向之一，你有完整的一套。

5. **多 Provider LLM 路由 + 熔断 + 降级**
   - 配置驱动路由（`ai/config.py` YAML + env 插值），按 use_case 选候选，指数退避熔断（`ai/router.py`），失败自动 fallback（`ai/service.py`），无云端模型时还有本地 curated 知识库**降级回复**（`orchestrator.py:415 build_fallback_reply`）。

6. **可评估的 AI 质量体系（Eval Harness）**
   - YAML 驱动的三维评估（工作流意图/路由、red-flag 安全召回、faithfulness 落地性），产出 JSON+Markdown 报告，**带 CI 退出码可做门禁**（`src/evals/consultation_eval_runner.py`）。
   - **为什么是亮点**：能说出"我如何度量并防止 AI 质量回归"，直接区分于只会调 prompt 的候选人。

7. **可恢复的持久化任务运行时（JobRuntime）**
   - 状态机（合法转移校验）+ 幂等 + `ListRecoverable`（pending + stale running）让 OCR 等后台任务在崩溃后可恢复，而非 fire-and-forget goroutine（`service/job_runtime.go`）。

8. **RAG 知识流水线（视频 → 知识单元）**
   - 视频 → ASR（多引擎：whisper.cpp/funasr/API/MiMo-omni）→ LLM 切分与策展 → 归一化 `units/clips` → pgvector 检索 + **意图感知重排**（`rag/knowledge_library.py:441 _intent_boost`）。

9. **安全实现扎实**
   - JWT HS256 且**显式校验签名算法**防 `alg:none`/算法混淆（`auth/jwt.go:81`）；无密钥 fail-fast；refresh token 为不透明随机值（可撤销）；越权防护在 service 层用 `verifyOwnership` 统一按 `user_id` 校验（`consultation_service.go:61`）；版本化迁移（golang-migrate，非 AutoMigrate）。

10. **成熟的自动化部署**
    - 路径级变更检测做**增量构建**、Docker 层缓存（GHA cache）、不可变镜像 tag、密钥卫生（GH Secrets + 服务器 `umask 077`）、部署后健康检查重试（`.github/workflows/deploy-do.yml`）。

---

## 2. 分技术栈审查（按严重程度分级）

> 🔴 严重（正确性/安全/生产阻断） · 🟡 中等（性能/健壮性/可维护性） · 🟢 建议（打磨）

### 2.1 AI 服务（Python）

#### 🔴 A-1　异步方法里混入同步阻塞的数据库调用 + 单一共享连接（无连接池）
- 位置：`apps/ai-service/src/rag/knowledge_library.py:114`（`_get_connection` 单连接）、`:275 search`、`:120 ingest`、`:427 stats` 均为 `async def` 却调用同步 `psycopg.connect()` / `cur.execute()`。
- 影响：
  1. 每次知识检索的 DB 往返会**阻塞整个 asyncio 事件循环**，在 FastAPI 下会卡住所有并发请求；
  2. 单个 `self._connection` 被所有协程共享，**psycopg 连接非并发安全**，高并发下会交叉污染/报错。
- 叠加放大：AI 服务容器是 `uvicorn` **单进程无 workers**（`apps/ai-service/Dockerfile:37`），并发能力本就受限，此问题会让"看起来能跑"在压测下立刻崩。
- 建议：改用 `psycopg_pool.AsyncConnectionPool` + `async with pool.connection()`，或最低限度用 `await asyncio.to_thread(...)` 包裹同步 DB 调用。这是**投入产出比最高的一个修复**。

#### 🔴 A-2　生产环境默认使用"哈希伪 embedding"，RAG 语义检索名不副实
- 位置：`.env.production:52` `EMBEDDING_PROVIDER=hashing`；实现见 `rag/embedding.py:21,49`（`_hash_embedding` 用字符/词 n-gram 哈希造向量）。
- 影响：生产的向量检索实际是**哈希/关键词级**而非语义级，"RAG"卖点在真实环境里被削弱；`_intent_boost` 的关键词加权只是部分补偿。
- 附带：维度不一致——`.env.production:53` 为 384，`embedding.py:17` 默认 1536，dev compose 也是 1536。若 pgvector 列维度固定，混用会导致检索报错或静默错误。
- 建议：生产切换到真实 embedding（如 `bge-m3` / `text-embedding-3-small` / 硅基流动等 OpenAI 兼容），统一维度并写入 `models`/迁移；`hashing` 仅保留为离线测试兜底（这是个**很好的测试设计**，但不该是生产默认）。

#### 🟡 A-3　流式 fallback 可能在"已输出部分内容后"切换 Provider 导致重复流
- 位置：`ai/service.py:71 generate_stream`。docstring 说"仅在首个 chunk 前 fallback"，但代码未跟踪"是否已 yield 过内容"——若 Provider 流到一半抛错，`except` 会继续下一个候选，从头重发，客户端收到重复/错乱输出。
- 建议：加 `first_chunk_sent` 标志，一旦已产出内容就不再 fallback，直接向上抛错由上层处理。

#### 🟡 A-4　CPU/阻塞调用在 async 路径中（本地 embedding / sentence-transformers）
- 位置：`rag/embedding.py:108,134` `model.encode(...)` 是同步 CPU 阻塞，却在 `async def generate` 内直接调用。
- 建议：`await asyncio.to_thread(self._local_model.encode, texts)`。

#### 🟡 A-5　缺 LLM 可观测性与用量计量落地
- 现状：Provider 拿到了 `TokenUsage`（`providers/openai_compatible.py:72`）并透传，但**全栈没有 tracing / token 成本 metrics / 结构化 LLM 日志**（无 LangSmith/Langfuse/OpenTelemetry —— 已确认 `src/` 无相关引用）。
- 影响：无法回答"每次问诊花多少 token/钱""哪个环节慢""prompt 回归"。
- 建议：接入 Langfuse 或 OTel + 自建 token/cost 表。**这同时是最值钱的新增亮点**（见 §4）。

#### 🟢 A-6　疑似死代码
- `src/api/routes/chat.py` 与 `services/chat_service.py` 未在 `main.py` 注册（活跃路径是 `runtime.py` → `consultation_graph`）。确认后删除，避免误导读者。

#### 🟢 A-7　其他打磨点
- `knowledge_library.py:107` DB 密码硬编码兜底 `bodysense123`（有 warning，但生产应 fail-fast）。
- `_intent_boost` 中 `"头前移"` 等硬编码词（`knowledge_library.py:464`）应数据化。
- `generate_with_retry` 对**所有**异常重试（含不可重试的鉴权/维度错误），且主检索路径其实没用它。
- 上下文管理是朴素截断（`consultation_graph.py:143` 最近 20 条），无摘要压缩（见 §4 可做成亮点）。
- `orchestrator.py:138` 硬编码 `temperature=0.7/max_tokens=2048`，绕过了 `RouteDefaults` 配置。

### 2.2 后端（Go）

#### 🟡 G-1　每条 SSE 事件（含每个文字 delta）同步写库
- 位置：`internal/consultation/runtime.go:1284 recordPublicEvent`，被 `sendEvent`/`sendNewEvent` 在流式循环内逐事件调用。
- 影响：一条长回复有数百个 `message.text.delta`，即数百次**内联同步 DB 插入**，增加每个 chunk 的延迟并放大 DB 负载。
- 建议：对高频 `text.delta` 做**批量/异步落库**（累积缓冲，或只持久化里程碑事件 + 完成时落最终文本），保留低频事件同步。这是事件溯源落地的常见优化点，能讲成"我如何在可回放与写放大之间权衡"。

#### 🟡 G-2　无优雅关闭 + 后台 worker 用 `context.Background()`
- 位置：`cmd/server/main.go:260 r.Run()`（阻塞、无信号处理）、`:72 StartOCRWorker(context.Background(), ...)`。
- 影响：`SIGTERM`（`docker stop`/滚动更新）时，进行中的 SSE 与 OCR worker 被硬杀，无法排空。
- 建议：改用 `http.Server` + 监听 `SIGINT/SIGTERM` → `srv.Shutdown(ctx)`，worker 用可取消的根 context，随关闭一起收敛。

#### 🟡 G-3　GORM 生产日志级别为 Info（全量 SQL + 参数）
- 位置：`internal/database/database.go:53 logger.Default.LogMode(logger.Info)`。
- 影响：生产会打印所有 SQL 及参数，性能开销 + 潜在 PII 泄漏到日志。
- 建议：按 env 配置，生产用 `Warn`/`Silent`。

#### 🟡 G-4　缺一批"生产必备"中间件
- 现状：`gin.Default()` 自带 Logger+Recovery、有健康检查（`main.go:150` 查 DB+Redis）、SSE 有 5min 超时与 10000 事件上限（`runtime.go:21-22`，很好）。
- 缺：**限流**、**请求级超时中间件**、**请求 ID / trace 透传**、**结构化日志**（当前是 `log.Printf`）、**metrics（Prometheus）**。
- 建议：至少加 request-ID 中间件 + 结构化日志（zap/slog）+ 基础限流；metrics 可作为亮点（见 §4）。

#### 🟢 G-5　打磨点
- `main.go:35 _ = db`：用了包级全局 `database.DB`，全局状态是轻微反模式（可接受）。
- Job 状态词汇 `completed` 与 `succeeded` 并存（`job_runtime.go:17-26`），略不统一。
- `verifyOwnership` 依赖每个 service 方法自觉调用；可加**纵深防御**：repo 更新语句也 join `user_id`。
- `ConsultationHandler.GetConsultation` 把所有错误映射为 500（`consultation_handler.go:68`），可区分 not-found/forbidden。

### 2.3 前端（React）

#### 🟡 W-1　生产代码残留大量调试日志，且破坏 reducer 纯度
- 位置：`hooks/useSSEProcessor.ts:147,181-195`、`runtime/activeTurnReducer.ts:126,336,440`（**纯 reducer 里 `console.debug/warn` 是副作用**）、`pages/ConsultationPage.tsx:329-363,711`、`hooks/useAssistantChatRuntime.ts:103,139,157`。
- 影响：噪音、泄漏内部结构、影响性能；纯 reducer 含副作用违背设计约定。
- 建议：统一收敛到一个 `debug(...)` 开关（`import.meta.env.DEV` 或 `debug` 库），发布构建剥离。

#### 🟡 W-2　`enqueueResult` 每个 chunk 对全量内容做 `JSON.stringify` 去重（O(n²)）
- 位置：`hooks/useAssistantChatRuntime.ts:79 const signature = JSON.stringify(result.content)`。
- 影响：文本越长，每个 delta 的 stringify 成本越高，长回复下累计 O(n²)。
- 建议：用增量长度/最后 delta 作为签名，或直接比较文本长度是否增长。

#### 🟡 W-3　`ConsultationPage` 巨型组件（1038 行）+ 一处死代码
- 位置：`pages/ConsultationPage.tsx`：路由 + 10+ handler + 2 mutation + 缓存管理 + 布局 + 移动端抽屉 + 一堆 helper 函数堆在一个文件。
- 死代码：`:878-884` `citations` prop 传入恒为 `undefined` 的三元表达式，明显是未完成/遗留。
- 建议：抽出 `useConversationActions` / `useConsultationMutations` hooks 与布局子组件；删死代码。

#### 🟡 W-4　缺 Error Boundary；SSE 断线无重连/续传（尽管后端已具备回放能力）
- 现状：手写 SSE 解析（`useSSEProcessor.ts`）在网络异常时只 `onError`，不重连；后端明明有 `runtime_event` 回放能力却未被前端利用做"断线重连回放"。
- 建议：加全局/路由级 `ErrorBoundary`；实现基于事件 `seq` 的断线重连续传（**这能把已有的后端回放能力兑现成前端亮点**，见 §4）。

#### 🟢 W-5　打磨点
- 手写 SSE 解析不完全符合规范：`startsWith('data: ')` 要求恰好一个空格、不支持多行 `data:`（受控后端下 OK，但不健壮）。
- `handleDeleteAll`（`:200`）对每个会话发一个 DELETE（请求风暴），建议批量端点。
- **教学级注释密度极高**（如逐行解释 `slice(7)`/`pop()`）：对"学习项目"是加分，但作品集/生产视角下可能显得初级——建议保留一份带注释的"学习版"，另出精简版给面试官看。

#### 前端性能：一个需要澄清的"好设计"
你的 `docs/plan/active/consultation-render-performance-*` 说明你已重视此问题，且**架构上做得不错**：服务端状态走 TanStack Query；SSE 的结构化事件（extracted_info/health_features/phase）通过 `setQueryData` 推入查询缓存供 InfoPanel 读取；**高频文字流被隔离在 `AssistantChatPanel`/assistant-ui 内部**，不会重渲染右侧信息面板（`ConsultationPage.tsx:390-461`）。这是个正确的隔离。建议补一句可验证的话术："我用 React Profiler 验证过 text delta 不会触发信息面板重渲染"，并检查聊天面板子树内（tool call 卡片等订阅 `useActiveTurnState` 的组件）是否仍随每个 delta 重渲染——`ActiveTurnStateContext` 无选择器，理论上会（`context/ActiveTurnContext.tsx:132`）。

### 2.4 工程化 / DevOps

#### 🟡 D-1　容器以 root 运行
- 位置：`apps/api/Dockerfile`、`apps/ai-service/Dockerfile` 均无 `USER` 指令。
- 建议：加非 root 用户（`adduser`），符合最小权限。

#### 🟡 D-2　CI 未用 Nx affected / 无 Nx Cloud 远程缓存 / 无集成测试服务
- 位置：`.github/workflows/ci.yml`：三个 job 每次全量跑，未用 `nx affected`；未接 Nx Cloud 分布式缓存；CI 无 postgres/redis service container，故集成测试跑不起来。
- 影响：Nx 是你的招牌技术之一，却没发挥其**增量 + 缓存**核心价值；测试只覆盖单元。
- 建议：CI 改用 `nx affected -t lint test build`；加 `services: postgres/redis` 跑集成测试；接 Nx Cloud（免费额度）当亮点。

#### 🟡 D-3　前端构建非可复现
- 位置：`docker/Dockerfile.web:21,24` 与 `apps/web/Dockerfile:14,17`：`pnpm install --no-frozen-lockfile` + `pnpm add ... --no-save || true` 补 Linux 原生 binding。
- 根因：lockfile 在 Windows 生成，缺 Linux 平台二进制。
- 建议：用 pnpm 的 `supportedArchitectures`（`.npmrc`）预置多平台 binding，恢复 `--frozen-lockfile`，保证可复现构建。

#### 🟡 D-4　部署无回滚、非零停机、与 Watchtower 双机制并存
- 位置：`.github/workflows/deploy-do.yml:236` SSH `docker compose up -d`（重建即短暂停机；健康检查失败时新容器已在跑，无自动回滚）；同时 `.env.production:28 WATCHTOWER_POLL_INTERVAL=300` 说明 Watchtower 也在轮询 `prod-latest` 自动更新——两套部署机制可能竞争。
- 建议：二选一（推荐保留 CI 推送式，去掉 Watchtower 或让其只看非关键服务）；健康检查失败时 `docker compose rollback` 或重新拉上一个不可变 tag。

#### 🟢 D-5　打磨点
- `apps/ai-service/Dockerfile:37` uvicorn 单进程 → 生产用 `--workers` 或 gunicorn+uvicorn worker（与 A-1 一起解决并发）。
- `docker-compose.yml:13` postgres 卷挂在 `/var/lib/postgresql` 而非标准 `/var/lib/postgresql/data`，确认数据是否真的持久化。
- `apps/web/Dockerfile` 是 Vite **dev server**（`npx vite`），仅供 dev profile；命名易与生产 `docker/Dockerfile.web` 混淆，建议改名 `Dockerfile.dev` 以正视听。

#### ✅ 密钥卫生：做得好
`.env.production` **刻意只放非敏感配置**，密钥走 `.env.production.local`（被 `.gitignore` 的 `.env.*.local` 覆盖），文件内明确文档化了分层加载与部署清单。未发现真实密钥被提交。`.mcp.json`、`*.pem`、`*.key` 也在 ignore 列表。**这是加分项，不是风险。**

---

## 3. 功能完整性（对照 PRD）

**已实现（覆盖度惊人）**：账号鉴权、分步档案 onboarding、上传 + OCR（JobRuntime 可恢复）、问诊工作台（对话 + 右侧信息面板 + 人体可视化 + tool call 可视化 + HITL 追问 + 引用来源 + 红旗警示）、可能性诊断、治疗方案、评估报告、训练计划 + 打卡 + 进度 + 阶段复评、会话历史、分享、健康旅程聚合、知识入库（视频→ASR→单元→片段）。**医疗免责声明已在多处展示**（`StreamingAssistantTurn.tsx`、`InfoPanel.tsx`、`AssessmentDetailPage.tsx`、`DiagnosisPanel.tsx`、`RedFlagBanner.tsx`），符合 PRD §7。

**PRD 中标注为"后续/可选"、当前未实现（可接受的 MVP 缺口）**：
- 症状术语点击弹出定义（PRD 3.2.2 高亮链接）——前端未见专门交互实现。
- 成就徽章 / 训练提醒（PRD 3.3.4）。
- 体态照片时间线对比（PRD 3.3.3）。
- 体态照片的 AI 视觉分析——PRD 明确"暂不做"，但 **README 首句写着"通过 AI 视觉分析帮助用户评估"**，与 PRD/现状不符，建议修正 README 措辞，避免面试官对照代码时觉得"过度承诺"。
- 国际化（当前仅中文）。

---

## 4. 可挖掘 / 新增的亮点（Roadmap）

这些**不必现在就做**，但每一条都能显著提升作品集的技术密度。按"性价比"排序：

1. **LLM 可观测性与成本看板**（最推荐）
   接入 Langfuse 或 OpenTelemetry，追踪每次问诊的 trace、token、时延、成本、工具调用链；做一个简单的"成本/时延"面板。**同时补上了当前最大短板（可观测性）并成为强亮点。** 话术："我为 AI 应用建了端到端 trace 与成本计量。"

2. **把评估体系升级为 LLM-as-Judge + 回归看板**
   现有 harness 只测确定性组件（规则/关键词/子串）。加一层用 LLM 评审生成质量（诊断合理性、方案落地性、忠实度），把分数入库并在 CI 出趋势。话术："我用 LLM 评审 + 黄金集防止 AI 质量回归。"

3. **兑现"断线重连回放"**（已有后端能力，缺前端临门一脚）
   你已经有 `runtime_event` 事件日志和 `replayCompletedRun`。前端基于 `seq` 做断线续传即可讲成"弹性流式：网络抖动不丢生成进度"。**低成本、高说服力。**

4. **真实 embedding + 检索质量评测**
   切换生产为真实语义向量（解决 A-2），并做一个小的检索评测集（命中率/MRR），用数据证明"换了 embedding 后召回提升 X%"。

5. **上下文工程：滑动窗口 + 摘要压缩**
   现在是朴素截断（`consultation_graph.py:143`）。做成"近 N 轮原文 + 更早历史 LLM 摘要"的分层记忆，讲"上下文窗口与 token 预算管理"。

6. **并发与压测报告**（解决 A-1 后）
   修好阻塞 DB + 加 workers 后，用 k6/locust 出一份"并发 N 路问诊"的压测报告放进 README。**"我做过压测并优化了 P95"是硬通货。**

7. **体态照片 AI 视觉分析**（产品层面最亮）
   用姿态估计（MediaPipe/关键点）或多模态模型分析侧面站姿照，量化头前移角、骨盆倾斜等 —— 这正好落在项目主题上，且能把 README 的"AI 视觉分析"兑现。

8. **一份架构 README + 架构图 + 30 秒 Demo 视频**
   你的 `docs/` 已非常完整，但面试官不会读 80 个文件。做一张总架构图（前端↔Go↔Python↔pgvector，标出 SSE/事件溯源/HITL/治理）+ 一段问诊 Demo 录屏，放在 README 顶部。**这是投入最小、回报最大的一步。**

---

## 5. 优先级行动清单

> 标注：`[修]` 修复缺陷 · `[亮]` 打造亮点 · 括号内为大致投入

### P0 — 立刻做（正确性/说服力，投入小回报大）
- `[修]` **A-1** 知识库改异步连接池 / `asyncio.to_thread`（0.5–1 天）——最关键。
- `[修]` **A-2** 生产切真实 embedding 并统一维度（0.5 天）。
- `[亮]` **§4-8** README 顶部加架构图 + Demo 视频 + 修正"AI 视觉分析"措辞（0.5 天）。
- `[修]` **W-1** 清理全栈 `console.debug` / 恢复 reducer 纯度（0.5 天）。

### P1 — 近期做（补齐生产短板）
- `[修]` **G-2** 优雅关闭 + 可取消 worker（0.5 天）。
- `[修]` **G-1** SSE 高频事件批量/异步落库（1 天）。
- `[修]` **D-1/D-5** 容器非 root + uvicorn 多 worker（0.5 天）。
- `[修]` **G-3/G-4** 生产 SQL 日志降级 + request-ID + 结构化日志（1 天）。
- `[修]` **A-3** 流式 fallback 首 chunk 后不再切换（0.5 天）。

### P2 — 作品集增值（挑 1–2 个深做）
- `[亮]` **§4-1** LLM 可观测性/成本看板（2–3 天）——**最推荐**。
- `[亮]` **§4-3** 断线重连回放（1–2 天）——复用已有后端能力。
- `[亮]` **§4-2** LLM-as-Judge 评估 + 回归看板（2–3 天）。
- `[修]` **D-2** CI 用 nx affected + 集成测试 service（1 天）。
- `[亮]` **§4-6** 并发压测报告（1 天，需先做 A-1）。

### P3 — 清理与打磨
- `[修]` **A-6** 删 `chat.py`/`chat_service.py` 死代码。
- `[修]` **W-3** 拆分 `ConsultationPage` + 删死代码。
- `[修]` **D-3** 恢复可复现构建（`supportedArchitectures`）。
- `[修]` **D-4** 部署回滚策略 / 去除 Watchtower 双机制。

---

## 6. 结语

从工程角度看，你已经具备**中高级工程师的架构直觉**：会做事件溯源、会做 HITL Agent、会做契约测试、会做治理与评估、会做自动化部署——这些是**别人想模仿都很难在练手项目里做出来**的东西。

当前与"一份无懈可击的作品集"之间的差距，**不在架构，而在少数几处运行时细节（阻塞 DB、生产 embedding、可观测性）**。这反而是好消息：这些是**明确、有限、可在一两周内闭环**的问题，而不是需要推倒重来的设计缺陷。

**建议的叙事主线**（面试怎么讲）：
> "我做了一个体态健康 AI 助手。技术上我重点解决了三个难题：① 让 Agent 能在问诊中**中断向用户提问、再跨请求恢复**（HITL 运行时）；② 用**事件溯源 + 版本化流事件契约**保证 Go/Python/前端三端的流式一致性与可回放；③ 给医疗场景加了**AI 输出治理与红旗安全护栏**，并用**可门禁的评估体系**防止质量回归。"

把 P0 做完、P2 挑一个深做，这份作品足以在 AI 应用工程师面试中作为**终面级的主项目**。

---

*报告完 · 如需针对任一条目展开为可执行的实施方案，可单独提出。*
