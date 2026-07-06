# 咨询工作台 AI Agent 核心深化分析

**日期**: 2026-06-24
**状态**: Phase 0-4（基础可靠性、诊断/方案 UI、统一 RAG 与引用、显式 Phase 契约、质量评估与安全边界）均已完成
**范围**: 健康咨询工作台、会话持久化、AI 对话编排、RAG 工具调用、诊断与改善方案闭环

## 1. 背景与目标

咨询工作台是 BodySense 的核心交互场景。PRD 和技术方案定义的目标不是一个简单聊天框，而是一个可以持续收集症状、结合用户档案和知识库推理、生成可能性分析、让用户确认判断、再输出改善方案和训练计划的 AI 体态健康顾问。

本次分析的目标是先确认当前实现边界，再明确下一阶段“AI agent 核心”应深化到什么程度。本文以当前代码为准，产品文档作为目标对照。

## 2. 目标体验

目标工作台应形成以下闭环：

1. 用户进入 `/consultation` 后自动创建或恢复会话。
2. 用户描述问题，AI 以 SSE 流式回复，并主动追问关键症状信息。
3. AI 同步提取结构化问诊信息，包括部位、症状性质、持续时间、触发场景、缓解方式、严重程度等。
4. 右侧信息面板实时更新，用户可以确认或修正信息，修正结果持久化回会话。
5. 当信息足够时，agent 进入可能性分析阶段，结合用户档案、会话信息和 RAG 知识库生成 1-3 个可能判断。
6. 用户确认诊断后，agent 基于确认诊断和知识库内容生成改善方案。
7. 改善方案可以进一步触发训练计划生成，完成“咨询 -> 诊断 -> 方案 -> 训练计划”的产品闭环。

## 3. 当前实现地图

### 3.1 前端工作台

相关文件：

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`
- `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`
- `apps/web/src/features/consultation/hooks/useChatSSE.ts`
- `apps/web/src/features/consultation/services/consultationService.ts`
- `apps/web/src/features/consultation/components/InfoPanel.tsx`
- `apps/web/src/features/consultation/components/BodyVisualization.tsx`
- `apps/web/src/features/consultation/components/DiagnosisPanel.tsx`

已实现能力：

- `/consultation` 无 id 时自动创建新会话，并跳转到 `/consultation/:id`。
- 左侧会话列表可以展示历史咨询并切换。
- `AssistantChatPanel` 可以发送用户消息到 Go SSE endpoint。
- `useChatSSE` 手写解析 SSE，支持 `text` 和 `extracted_info` 两类消息。
- `InfoPanel` 可以展示结构化症状卡片，并驱动 `BodyVisualization` 高亮身体部位。
- `DiagnosisPanel` 已有诊断卡片和改善方案展示组件。

当前状态（已修复）：

- ~~`InfoPanel` 的"确认"只打印日志，"修改"只更新本地 state，没有调用 `updateExtractedInfo` 持久化。~~ ✅ 已修复：确认/修改均调用 API 持久化全量 `extractedInfo`。
- ~~`DiagnosisPanel` 没有接入 `ConsultationPage`，诊断分析和方案生成入口没有出现在主工作台流程里。~~ ✅ 已修复：`DiagnosisPanel` 完整接入主工作台，含 citations 展示。
- ~~`consultationService.confirmDiagnosis` 只封装了 API，但页面没有使用。~~ ✅ 已修复：页面已接入 `analyzeDiagnosis`、`confirmDiagnosis`、`generateTreatment`。
- ~~`useAssistantChatRuntime` 创建了 assistant-ui runtime，但实际聊天 UI 仍使用自定义 state 和 `useChatSSE`，assistant-ui 尚未真正承担交互框架职责。~~ ✅ 已在第八批集成：重写 `ConsultationChatAdapter`（async generator），`AssistantChatPanel` 使用 `useLocalRuntime` 驱动交互，消息状态由 assistant-ui thread 管理。
- ~~前端没有显式会话 phase 状态，无法区分"引导问诊、可能性分析、等待确认、方案生成、已完成"等阶段。~~ ✅ 已修复：`ConsultationSession.phase` 完整展示，`useChatSSE` 解析 `phase_changed` 事件并实时更新 UI。

### 3.2 Go API 与会话层

相关文件：

- `apps/api/internal/handler/consultation_handler.go`
- `apps/api/internal/handler/diagnosis_handler.go`
- `apps/api/internal/service/consultation_service.go`
- `apps/api/internal/repository/consultation_repository.go`
- `apps/api/internal/model/consultation_session.go`
- `apps/api/migrations/000006_create_consultation_sessions.up.sql`

已实现能力：

- 会话表 `consultation_sessions` 已包含 `messages`、`extracted_info`、`diagnosis`、`treatment_plan`、`status`。
- Go 提供创建、获取、列表、发送消息、更新结构化信息、确认诊断等 API。
- `SendMessage` 会拉取用户档案，解析历史消息和已提取信息。
- 发送消息时 Go 会先调用 `/api/knowledge/search` 检索知识库，再把 `rag_results` 传给 Python `/api/chat/stream`。
- Go 会代理 Python SSE，边转发给前端，边收集 assistant 文本和 `extracted_info`。
- 单独提供 `/consultation/:id/diagnosis` 和 `/consultation/:id/treatment`，代理 Python 诊断和方案生成。

当前状态（已修复）：

- ~~**阻塞级问题**: `ConsultationService.AppendMessage` 调用 `GetByID(ctx, sessionID, uuid.Nil)`~~ ✅ 已修复：`AppendMessage` 改用 `GetBySessionID`（无需 userID），消息持久化正常。
- ~~`SendMessage` 使用 `http.Post` 调 Python chat stream，没有绑定原始 request context。~~ ✅ 已在第五批修复：改用 `http.NewRequestWithContext`。
- ~~SSE 代理使用 `bufio.Scanner`。~~ ✅ 已在第五批修复：改为 `io.Reader` 按 chunk 转发。
- ~~`ConfirmDiagnosis` 只更新 `diagnosis`，没有推动 `status` 或 phase，也没有自动生成 treatment。~~ ✅ 已修复：`ConfirmDiagnosis` 现在调用 `UpdatePhase("diagnosis_confirmed")`；`GenerateTreatment` 调用 `UpdatePhase("plan_ready")`。
- ~~诊断和方案接口没有像聊天接口一样执行知识库检索，当前 Go 传给 Python 的 `rag_context` 为空。~~ ✅ 已修复：诊断路径按症状信息检索知识库，方案路径按诊断名+动作意图检索，均传入 `rag_context`，RAG 结果以 `citations` 字段返回。
- ~~会话 `CreateSession` 会在当前会话为空时仍然新建新会话~~ ✅ 已修复：`CreateSession` 先调 `GetLastInProgressEmptySession`，空会话存在时复用。

### 3.3 Python AI 服务

相关文件：

- `apps/ai-service/src/api/routes/chat.py`
- `apps/ai-service/src/services/chat_service.py`
- `apps/ai-service/src/prompts/consultation.py`
- `apps/ai-service/src/models/consultation.py`（@dataclass models：SymptomInfo、ExtractedInfo、ChatContext）
- `apps/ai-service/src/services/llm_provider.py`
- `apps/ai-service/src/api/routes/diagnosis.py`
- `apps/ai-service/src/services/diagnosis_service.py`（Pydantic models：DiagnosisItem、DiagnosisResponse、TreatmentPlan、TreatmentResponse）
- `apps/ai-service/src/prompts/diagnosis.py`

已实现能力：

- `/api/chat/stream` 接收完整会话上下文、用户档案、已提取信息、RAG 结果。
- `ChatService` 会构建系统 prompt、用户档案、RAG context、已提取症状摘要和最近 10 轮上下文。
- `LLMProvider` 封装了 OpenAI-compatible API，可通过 `LLM_API_KEY`、`LLM_BASE_URL`、`LLM_MODEL` 配置不同模型。
- 聊天支持流式输出，并支持 `extract_symptom_info` function calling。
- 未配置 LLM 时，`ChatService` 有基于 RAG 的 deterministic fallback，利于本地开发。
- `DiagnosisService` 可以生成结构化 JSON 诊断和改善方案。

当前状态（部分已修复）：

- ~~当前 agent 核心仍是单次 prompt 调用，不是显式状态机或 LangGraph workflow。~~ ✅ 已在第七批实现 LangGraph StateGraph，第八批实现多轮 tool loop。
- ~~`SYSTEM_PROMPT` 描述了三阶段流程，但代码没有保存或推进 phase，模型只能在单次回复中自行判断阶段。~~ ✅ 已修复：`ChatService` 在每轮 SSE 流末尾输出 `phase_changed` 事件，Go 持久化 phase 并有前进规则防倒退。
- ~~function calling 只把工具调用结果作为 `extracted_info` 发给前端，没有执行完整回合。~~ ✅ 已在第八批实现：`generate_response` 改为多轮 tool loop，支持 `extract_symptom_info` 和 `search_knowledge` 两种工具的完整调用回合（tool call → tool result → LLM 继续生成）。
- ~~`DiagnosisService` 依赖模型输出可解析 JSON，但没有 Pydantic schema 校验、自动修复、fallback 或结构化输出 API 约束。~~ ✅ 已修复：`DiagnosisItem`、`DiagnosisResponse`、`TreatmentPlan`、`TreatmentResponse` 均有 Pydantic schema 校验（Field 约束），JSON 解析有三级 fallback。
- ~~诊断和方案生成没有自动拿当前会话的 RAG 结果，也没有返回引用来源。~~ ✅ 已修复（网关层）：Go `diagnosis_handler` 将 RAG 结果以 `citations` 字段附加到诊断/方案响应，前端 `DiagnosisPanel` 可折叠展示引用详情。
- ~~`_sessions` 内存缓存声明后从未被读写，属于死代码，应清理。~~ ✅ 已在第六批清理。

### 3.4 RAG 与知识库工具

相关文件：

- `apps/ai-service/src/api/routes/knowledge.py`
- `apps/ai-service/src/rag/knowledge_library.py`
- `apps/ai-service/src/rag/retriever.py`
- `apps/ai-service/src/prompts/consultation.py`

已实现能力：

- normalized knowledge library 已包含 source、segment、unit、clip 的结构。
- `/api/knowledge/search` 支持 query、top_k、problem_slug、unit_type。
- `KnowledgeLibrary.search` 使用 embedding 检索，并用意图关键词进行 rerank boost。
- 搜索结果包含 summary、body_markdown、source metadata、clip metadata。
- Go chat 入口会按用户消息搜索 top 5，并把结果注入 Python chat。

当前状态（部分已修复）：

- ~~RAG 调用发生在 Go 前置层，不是 agent 可自主选择的 tool。~~ ✅ 已在第八批实现：`search_knowledge` 作为 LLM-callable tool 注册到 LangGraph 图中，`generate_response` 改为多轮 tool loop，agent 自主决定何时搜索。Go `SendMessage` 移除前置 `searchKnowledge` 调用。
- ~~诊断和方案生成路径没有复用 RAG 检索。~~ ✅ 已修复：诊断和方案路径均通过 `knowledge_helper.go` 检索知识库，结果以 `rag_context` 传给 Python，并以 `citations` 结构化字段返回前端。
- ✅ 已修复：Go 层将 RAG raw results 附加为 `citations` 字段，前端 `DiagnosisPanel` / `TreatmentPlanView` 均实现可折叠引用展示。
- ~~检索失败时聊天 fallback 较友好，但在线 LLM 路径的知识缺失策略主要依赖 prompt。~~ ✅ 已在第八批实现：系统 prompt 新增"知识缺失处理"章节，`execute_search_knowledge` 无结果时返回显式提示，新增 `knowledge_gap` SSE 事件通知前端。
- ~~⚠️ RAG 双系统并存~~ ✅ 已在第六批清理：旧 `retriever.py`、`knowledge_base.py`、`reranker.py` 已移除，统一到 `knowledge_library.py`。

## 4. 当前状态结论

当前实现已从"咨询工作台 MVP 骨架 + AI 服务雏形"升级为**完整闭环的咨询 Agent 主链路**：

- ✅ UI 骨架、SSE 链路、基础 RAG 注入、结构化信息展示已具备并稳定。
- ✅ 诊断和方案生成的后端/Python API 完整接入主工作台，含 loading/error/retry。
- ✅ 会话消息持久化 bug 已修复，刷新后历史仍存在。
- ✅ 显式 phase 状态机已落地（Go 持久化 + SSE 事件 + 前端 UI 同步），三阶段流程不再完全依赖 prompt 自觉性。
- ✅ 结构化 citations 从诊断/方案 RAG 检索结果注入到 JSON 响应并展示在前端诊断卡片和方案卡片中。
- ✅ 空会话复用逻辑已修复，不再产生冗余空会话。

下一阶段重点：所有计划内工作已完成。

## 5. 目标架构建议

建议把咨询 agent 明确定义为“Go 持久化状态机 + Python agent 编排器 + RAG/诊断/方案工具”的组合。

### 5.1 会话状态

建议在 `consultation_sessions` 中增加或用 JSON 承载以下状态：

| 状态 | 说明 | 进入条件 | 下一步 |
|---|---|---|---|
| `collecting` | 引导问诊与症状收集 | 新会话创建后 | 信息足够后进入 `ready_for_analysis` |
| `ready_for_analysis` | 可生成可能性分析 | 关键字段满足阈值或用户主动请求分析 | 调用诊断分析 |
| `analysis_ready` | 已生成诊断候选 | Python 返回 diagnoses 后 | 等待用户确认 |
| `diagnosis_confirmed` | 用户已确认判断 | 用户选择诊断 | 生成改善方案 |
| `plan_ready` | 改善方案已生成 | Python 返回 treatment_plan 后 | 可生成训练计划 |
| `completed` | 咨询闭环完成 | 用户结束或训练计划已生成 | 历史查看 |

MVP 可以先不加新表，先用 `status` + `diagnosis` + `treatment_plan` + 一个 `phase` 字段即可。

### 5.2 Agent 输入输出契约

聊天流建议统一输出事件：

| 事件类型 | 用途 |
|---|---|
| `text` | assistant 可见回复流 |
| `extracted_info_patch` | 对结构化信息的新增或更新，而不是简单 append |
| `phase_changed` | 通知前端进入分析、等待确认、方案生成等状态 |
| `diagnosis_candidates` | 返回结构化诊断候选 |
| `treatment_plan` | 返回结构化改善方案 |
| `citation` | 返回知识库引用或动作 clip |
| `done` | 一轮响应结束，包含最终会话快照 |
| `error` | 可恢复错误，前端展示并允许重试 |

### 5.3 Python Agent 形态

短期不必直接引入复杂多 agent。建议先实现单 agent workflow：

1. `load_context`: 接收 Go 传入的会话、档案、历史、结构化信息。
2. `classify_intent`: 判断用户本轮意图是补充症状、请求分析、确认诊断、请求方案、泛问答。
3. `retrieve_knowledge`: 根据意图和候选部位/问题检索知识库。
4. `extract_info`: 抽取并合并结构化问诊信息。
5. `decide_next_phase`: 根据字段完整度和用户意图决定阶段。
6. `respond_or_analyze`: 继续追问，或生成诊断候选。
7. `generate_treatment`: 用户确认后生成改善方案。
8. `emit_events`: 以统一 SSE contract 输出。

这可以先用普通 Python service + Pydantic models 实现，等逻辑稳定后再迁移到 LangGraph。

## 6. 推荐实施阶段

### Phase 0: 修复基础可靠性

目标：保证当前 MVP 链路不会丢数据。

- 修复 `AppendMessage` 查询不到会话的问题。
- 为 `AppendMessage`、`UpdateExtractedInfo`、`UpdateDiagnosis`、`UpdateTreatmentPlan` 增加最小单元测试。
- `SendMessage` 对持久化错误返回或至少记录明确错误。
- Go SSE 代理使用 request context，并处理 scanner buffer 或改为 reader 按 chunk 转发。
- 前端修改/确认结构化信息时调用 `updateExtractedInfo`。

验收标准：

- 发送一轮消息后刷新页面，用户消息、assistant 回复、提取信息仍存在。
- 用户修改右侧信息后刷新页面，修改仍存在。

### Phase 1: 接通诊断与方案 UI

目标：把已有后端能力接入主工作台。

- 在 `ConsultationPage` 中接入 `DiagnosisPanel`。
- 新增 `consultationService.analyzeDiagnosis` 和 `generateTreatment`。
- 点击“生成可能性分析”后调用 `/consultation/:id/diagnosis`，并保存结果到 session。
- 用户确认诊断后调用 `/confirm`，再调用 `/treatment` 生成方案。
- 前端展示 `diagnosis` 和 `treatment_plan`，并处理 loading/error/retry。

验收标准：

- 至少一条结构化症状信息存在时，可以生成诊断候选。
- 选择候选后可以生成改善方案。
- 刷新后诊断和方案仍显示。

### Phase 2: 统一 RAG 与引用

目标：让聊天、诊断、方案都基于同一套知识来源。

- Go 的诊断和方案接口也执行知识库检索，或由 Python agent 内部调用 knowledge library。
- 诊断候选中增加 `evidence` 或 `citations` 字段，引用知识库 unit。
- 改善方案动作优先关联 knowledge clip。
- 对知识库无匹配场景输出明确 fallback。

验收标准：

- 诊断和方案返回中包含可展示的知识库来源。
- 知识库不可用时不编造来源，并给出保守建议。

### Phase 3: 显式 Agent Workflow

目标：把 prompt 中的三阶段流程落为程序可控状态。

- 定义 `ConsultationPhase`、`ConsultationIntent`、`AgentEvent`、`AgentState` Pydantic models。
- 实现 `classify_intent`、`merge_extracted_info`、`should_analyze`、`should_generate_treatment`。
- SSE 输出 `phase_changed`、`diagnosis_candidates`、`treatment_plan` 等结构化事件。
- Go 持久化每次 phase transition。
- 前端根据 phase 控制按钮和面板状态。

验收标准：

- 模型即使回复文本不稳定，UI 仍能靠结构化事件推进流程。
- 会话历史能重建当前阶段和所有右侧面板状态。

### Phase 4: 质量评估与安全边界

目标：把健康建议的质量和安全性纳入测试。

- 增加 golden cases：肩颈酸胀、头前伸、肘外翻、腰痛、红旗症状。
- 增加 RAG faithfulness 检查：方案中的动作必须来自或兼容知识库。
- 增加 red flag 策略：严重疼痛、麻木无力、外伤、持续加重等直接建议就医。
- 增加输出 schema 校验和 JSON repair/fallback。

验收标准：

- 所有 golden cases 输出结构稳定。
- 红旗症状不会继续给高风险训练动作。
- 无 RAG 命中时回答明确标注不确定性。

## 7. 关键数据契约建议

### 7.1 ExtractedInfo

当前字段足够 MVP，但需要增加稳定 id 和确认状态：

```json
{
  "id": "uuid-or-stable-key",
  "body_part": "肩部",
  "symptom_type": "酸胀",
  "duration": "2周",
  "trigger": "久坐后",
  "relief": "按压后缓解",
  "severity": "轻度",
  "confidence": "中",
  "confirmed": false,
  "source_message_index": 3,
  "additional_notes": ""
}
```

### 7.2 Diagnosis

```json
{
  "id": "upper-crossed-syndrome",
  "name": "上交叉综合征/圆肩倾向",
  "confidence": "中",
  "severity": "轻度",
  "basis": ["肩颈酸胀", "久坐后明显", "头颈前伸描述"],
  "typical_symptoms": "常见于久坐低头人群...",
  "differential": "需与肩袖损伤、颈椎神经症状区分",
  "citations": [
    {
      "knowledge_unit_id": 123,
      "title": "头前移的自测与改善",
      "source_timestamp": "02:10-03:30"
    }
  ]
}
```

### 7.3 TreatmentPlan

```json
{
  "goal": "缓解肩颈酸胀，改善头前伸和圆肩倾向",
  "duration_weeks": 4,
  "correction_exercises": [
    {
      "name": "胸小肌拉伸",
      "description": "靠墙或门框完成...",
      "sets": "2-3组",
      "reps": "每次30秒",
      "notes": "不要耸肩或憋气",
      "citation": {
        "knowledge_unit_id": 456,
        "clip_id": 12
      }
    }
  ],
  "daily_habits": ["每45分钟起身活动2分钟"],
  "warning_signs": ["疼痛放射到手臂并伴随麻木无力时及时就医"]
}
```

## 8. 测试策略

### 前端

**当前状态：已建立 vitest 测试基础设施，51 个测试覆盖核心组件。** 已有测试项：

- `ConsultationPage` 会话加载、空会话创建、历史切换。
- `useChatSSE` 对 `text`、`extracted_info_patch`、`phase_changed`、`red_flag`、`done` 的解析。
- `InfoPanel` 修改后调用 API 并回写页面 state。
- `DiagnosisPanel` 诊断确认、方案生成 loading/error 状态。

### Go

- `AppendMessage` 能按 session id 正确追加消息。
- `SendMessage` 代理 SSE 时能持久化用户消息、assistant 消息、结构化信息。
- 诊断和方案接口能注入 RAG context，并保存结果。
- 请求取消时上游 AI 请求被取消。

### Python

- `merge_extracted_info` 对同一部位做更新而非重复 append。
- `classify_intent` 能区分补充症状、请求分析、确认诊断、请求方案。
- 诊断和方案输出通过 Pydantic schema 校验。
- LLM 缺失、RAG 缺失、JSON 解析失败都有 deterministic fallback。
- 红旗症状触发安全回复。

## 9. 优先级排序

| 优先级 | 工作项 | 状态 | 原因 |
|---|---|---|---|
| P0 | 修复消息持久化 | ✅ 已完成 | 会话历史是后续 agent 记忆和诊断依据 |
| P0 | 右侧信息修改持久化 | ✅ 已完成 | 用户确认/修正是问诊可信度基础 |
| P0 | 修复空会话重复创建 | ✅ 已完成 | 避免每次进入页面产生冗余空会话 |
| P1 | 接入诊断和方案 UI | ✅ 已完成 | 现有能力未进入主流程 |
| P1 | 诊断/方案加入 RAG | ✅ 已完成 | 符合产品"基于知识库"的核心承诺 |
| P2 | 显式 phase 和事件契约 | ✅ 已完成 | 降低 prompt 漂移，提升前端可控性 |
| P2 | Pydantic schema 和 fallback | ✅ 已完成 | 提升结构化输出稳定性 |
| P2 | 结构化 citation 返回前端 | ✅ 已完成 | 诊断和方案的知识库来源可展示 |
| P3 | 完整 Agent workflow 分层 | ✅ 已完成 | intent classification、tool execution 显式化 |
| P3 | LangGraph 化 | ✅ 已完成 | 咨询 agent 工作流迁移到 LangGraph StateGraph |
| P4 | Golden cases 与安全边界 | ✅ 已完成 | 红旗症状策略、RAG faithfulness 检查 |
| P4 | RAG 作为 agent 工具 | ✅ 已完成 | agent 自主决定何时搜索知识库 |
| P4 | 知识缺失策略 | ✅ 已完成 | RAG 无命中时显式标注不确定性 |
| P4 | 前端测试基础设施 | ✅ 已完成 | vitest + 51 个测试覆盖核心组件 |
| P5 | assistant-ui 框架集成 | ✅ 已完成 | ChatModelAdapter async generator + runtime 驱动交互 |

## 10. 建议的下一步

Phase 0（基础可靠性）、Phase 1（诊断/方案 UI）、Phase 2（统一 RAG 与引用）、Phase 3（显式 Phase 契约）、Phase 4（质量评估与安全边界）、LangGraph 化、RAG agent tool、知识缺失策略、前端测试、assistant-ui 清理均已完成。

下一步工作：

1. 无。所有计划内工作已完成。

## 11. 实施记录

### 2026-06-24 第一批实现

已完成：

- 修复 Go `ConsultationService.AppendMessage` 按 `uuid.Nil` 查询用户会话导致消息无法持久化的问题。
- 为消息追加逻辑增加不依赖真实数据库的 service 单元测试。
- Go 诊断分析接口会将候选诊断结果保存到 `consultation_sessions.diagnosis`。
- Go 改善方案接口会优先将 `treatment_plan` 内层对象保存到 `consultation_sessions.treatment_plan`。
- 前端 `consultationService` 增加 `analyzeDiagnosis` 和 `generateTreatment` API。
- 前端咨询工作台加载会话时恢复已保存的诊断候选和改善方案。
- 右侧结构化信息确认/修改会调用 `updateExtractedInfo` 持久化。
- `DiagnosisPanel` 接入咨询工作台，可从右侧面板生成可能性分析，并确认诊断后生成改善方案。

已验证：

- `cd apps/api && go test ./...`
- `cd apps/api && go vet ./...`
- `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`
- `pnpm nx run web:lint`

仍待后续批次：

- 引入显式 `phase` / `AgentEvent` 契约，减少流程对 prompt 自觉性的依赖。

### 2026-06-24 第二批实现

已完成：

- 抽出 Go handler 共享知识库检索 helper，聊天、诊断、方案路径可以复用同一套搜索、排序和结果映射。
- 诊断分析路径会基于已提取症状信息检索知识库，并把格式化后的 `rag_context` 传给 Python AI 服务。
- 改善方案路径会基于确认诊断、症状信息和“改善/训练/动作”意图检索知识库，并把 `rag_context` 传给 Python AI 服务。
- Python `DiagnosisService` 增加 Pydantic schema 校验，覆盖诊断候选和改善方案两类结构化输出。
- 新增 Python 单元测试，验证合法诊断/方案输出会被规范化，缺字段诊断输出会被拒绝。

已验证：

- `cd apps/api && go test ./...`
- `cd apps/api && go vet ./...`
- `cd apps/ai-service && uv run ruff check .`
- `cd apps/ai-service && uv run pytest`
- `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`
- `pnpm nx run web:lint`

仍待后续批次：

- 引入显式 `phase` / `AgentEvent` 契约，减少流程对 prompt 自觉性的依赖。
- 将诊断和方案中的知识库引用以结构化 citation 返回给前端，而不仅是注入 prompt context。

### 2026-06-24 第三批实现

已完成：

- ~~新增 `consultation_sessions.phase` 持久化字段和索引，默认阶段为 `collecting`。~~ ✅ 已修复
- ~~Go model、repository、service 支持读取和更新咨询 workflow phase。~~ ✅ 已修复
- ~~新增 phase 前进规则，聊天流中的阶段事件只允许推进或保持，不会把 `plan_ready` 等后续状态倒退为 `collecting`。~~ ⚠️ 部分修复：前进规则 `shouldAdvanceConsultationPhase` 定义在 handler 层（`consultation_phase.go`），仅在 SSE 消息流路径中生效。`DiagnosisHandler` 的 `AnalyzeDiagnosis`、`GenerateTreatment` 和 `ConfirmDiagnosis` 端点直接调用 `service.UpdatePhase`，绕过了前进守卫。（待修复：守卫需下沉到 service 层）
- Python `ChatService` 在 SSE 流中输出 `phase_changed` 结构化事件，并在 `done` 事件里附带最终 phase。
- 前端 `useChatSSE` 支持解析 `phase_changed`，`AssistantChatPanel` 和旧 `ChatPanel` 都能向页面冒泡 phase 更新。
- 咨询工作台展示当前阶段标签，并在诊断分析、诊断确认、方案生成后同步更新本地 phase。

已验证：

- `cd apps/api && go test ./...`
- `cd apps/api && go vet ./...`
- `cd apps/ai-service && uv run ruff check .`
- `cd apps/ai-service && uv run pytest`
- `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`
- `pnpm nx run web:lint`

仍待后续批次：

- ~~将知识库引用以结构化 citation 返回给前端，并在诊断卡片/动作方案中展示来源。~~ ✅ 已在第四批实现。
- 把当前普通 service 编排升级为更完整的 Agent workflow：intent classification、phase transition、tool execution、response generation 分层。

### 2026-06-24 第四批实现

已完成：

- 修复 `ConsultationService.CreateSession` 逻辑：调用前先通过 `GetLastInProgressEmptySession` 检查是否存在进行中的空会话，存在则复用，避免每次进入 `/consultation` 时重复创建冗余空会话。
- Go `diagnosis_handler.go` 在诊断分析和改善方案成功后，将最后一次 RAG 检索结果（`lastRagResults`）以 `citations` 字段附加到 JSON 响应并保存到数据库，与诊断/方案结构同步持久化。
- 前端 `consultationService.ts` 新增 `Citation` 接口（含 `title`、`summary`、`body_markdown`、`source_title`、`source_author`、`category`、`problem_slug` 等字段），`DiagnosisAnalysis` 和 `TreatmentPlan` 增加可选 `citations` 字段。
- `DiagnosisPanel` 在每个诊断候选卡片底部展示关联知识库来源标签（基于诊断名模糊匹配 citation title/slug），并在所有候选卡片下方增加可折叠展开的完整知识库参考列表（含摘要和原文）。
- `TreatmentPlanView` 在每个矫正动作右侧展示匹配的知识来源文本，并在方案底部增加完整可折叠的"方案科学依据"引用块。
- `ConsultationPage` 将 `session.diagnosis.citations` 传给 `DiagnosisPanel`，使诊断卡片的引用来源能在工作台界面直接展示。

已验证：

- `cd apps/api && go test ./...`
- `cd apps/api && go vet ./...`
- `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`
- `pnpm nx run web:lint`

仍待后续批次：

- 把当前普通 service 编排升级为更完整的 Agent workflow：intent classification、tool execution、response generation 显式分层（P3 目标）。
- Phase 4：增加 golden cases 测试、红旗症状安全策略、RAG faithfulness 检查、JSON repair fallback。
- SSE 代理改为绑定 request context，支持用户断开时取消上游请求。
- `useChatSSE` 支持解析聊天流实时 `citation` 事件（当前 citation 仅通过诊断/方案接口传递，不走 chat SSE）。

### 2026-06-24 第五批实现（Phase 4）

已完成：

- 新增 `RedFlagDetector` 服务（`apps/ai-service/src/services/red_flag_detector.py`）：实现症状安全扫描，覆盖严重疼痛、放射痛、麻木无力、外伤、症状加重、感染、全身症状等 8 类红旗模式，支持从 extracted_info 和对话文本中检测。
- 新增 `FaithfulnessChecker` 服务（`apps/ai-service/src/services/faithfulness_checker.py`）：验证治疗方案中的矫正动作是否来自或兼容知识库，支持标题匹配、内容匹配、clip 匹配和动作名称别名匹配，为每个动作输出 grounded 状态和置信度。
- 新增 `ConsultationAgentWorkflow` 服务（`apps/ai-service/src/services/agent_workflow.py`）：实现显式 agent workflow 编排，包含 `classify_intent`（意图分类：补充症状、请求分析、确认诊断、请求方案、澄清、一般问题）、`merge_extracted_info`（信息合并去重）、`should_analyze`（分析就绪判断）、`should_generate_treatment`（方案生成判断）、`decide_next_action`（下一步动作决策）。
- `ChatService.stream_chat` 集成 `RedFlagDetector`：在流式回复前后检查红旗症状，检测到时输出 `red_flag` SSE 事件，包含具体类别和安全提示。
- `DiagnosisService.generate_diagnosis` 集成 `RedFlagDetector`：诊断分析时同步检查红旗，结果以 `red_flags` 字段附加到诊断响应。
- `DiagnosisService.generate_treatment` 集成 `FaithfulnessChecker`：方案生成后检查每个动作的知识库依据，结果以 `faithfulness` 字段附加到方案响应。
- Python 诊断 API 路由新增 `rag_results` 参数，支持传递原始 RAG 结果用于 faithfulness 检查和 citation。
- Go `diagnosis_handler.go` 在诊断和方案请求中传递 `rag_results` 给 Python AI 服务，并改用 `http.NewRequestWithContext` 绑定请求 context。
- Go `consultation_handler.go` 的 `SendMessage` 方法改用 `http.NewRequestWithContext` 绑定原始 request context，支持用户断开时自动取消上游 AI 请求；SSE 代理的 `bufio.Scanner` buffer 从默认值增大到 256KB，避免大块数据截断。
- 前端新增 `RedFlagBanner` 组件（`apps/web/src/features/consultation/components/RedFlagBanner.tsx`）：醒目的医疗警告横幅，展示红旗类别和安全提示。
- 前端 `useChatSSE` hook 支持解析 `red_flag` SSE 事件，新增 `onRedFlag` 回调和 `RedFlagEvent` 类型。
- 前端 `AssistantChatPanel` 集成 `RedFlagBanner`：聊天流检测到红旗时显示警告，发送新消息时清除。
- 新增完整测试套件：
  - `test_red_flag_detector.py`：13 个测试，覆盖各类红旗模式检测、去重、便捷方法。
  - `test_faithfulness_checker.py`：10 个测试，覆盖标题/内容/clip 匹配、别名匹配、未接地动作。
  - `test_agent_workflow.py`：25 个测试，覆盖意图分类、信息合并、阶段判断、动作决策。
  - `test_golden_cases.py`：15 个黄金用例测试，覆盖肩颈酸胀、头前伸、肘外翻、腰痛、红旗症状等典型场景。

已验证：

- `cd apps/ai-service && uv run pytest`（126 tests passed）
- `cd apps/ai-service && uv run ruff check .`
- `cd apps/api && go test ./...`
- `cd apps/api && go vet ./...`
- `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`
- `pnpm nx run web:lint`

仍待后续批次：

- LangGraph 化：等状态与契约稳定后再做，避免过早复杂化。
- ~~`useChatSSE` 支持解析聊天流实时 `citation` 事件~~ ✅ 已在第六批实现。
- ~~Phase 前进守卫下沉到 `ConsultationService.UpdatePhase`~~ ✅ 已在第六批实现：`ShouldAdvancePhase` 已在 service 层，handler 层冗余 wrapper 已移除。
- ~~清理 `chat.py` 中 `_sessions` 死代码~~ ✅ 已在第六批清理：`_sessions` 已不存在，`SessionInfo` 死代码已移除。
- ~~`RedFlagBanner` 的 `RedFlag` 类型改为从 `useChatSSE` 导入~~ ✅ 已在第五批修复（review 确认无重复定义）。
- ~~`useChatSSE` 尾部 buffer 补充 `red_flag` 事件处理~~ ✅ 已在第六批重构：提取 `dispatchSSEData` 共享函数，主循环和尾部 buffer 统一处理。
- ~~RAG 双系统清理~~ ✅ 已在第六批清理：旧 `retriever.py`、`knowledge_base.py`、`reranker.py` 及其测试已移除，统一到 `knowledge_library.py`。

### 2026-06-25 第六批实现

已完成：

- ~~Phase 前进守卫下沉到 service 层~~ ✅ 已修复：`ShouldAdvancePhase` 已在 `ConsultationService.UpdatePhase` 内部调用（line 133），handler 层冗余 wrapper `shouldAdvanceConsultationPhase` 及其测试文件已删除，SSE 路径简化为直接调用 `UpdatePhase`。
- ~~`useChatSSE` 尾部 buffer 代码重复~~ ✅ 已修复：提取 `dispatchSSEData` 和 `processSSELine` 共享函数，主循环和尾部 buffer 统一调用，消除 copy-paste 维护风险。
- ~~聊天流 citation 事件~~ ✅ 已实现：Python `ChatService.stream_chat` 在有 RAG 结果时输出 `citation` SSE 事件（含 title、summary、body_markdown 等字段，body_markdown 截断至 500 字符节省带宽）；前端 `useChatSSE` 新增 `onCitation` 回调；`AssistantChatPanel` 收集 citation 并以"参考知识"标签展示。
- ~~`Citation` 类型去重~~ ✅ 已修复：`useChatSSE` 改为从 `consultationService.ts` 导入 `Citation` 接口，移除重复定义。
- ~~`SSEMessage` 死代码~~ ✅ 已清理：移除未使用的 `SSEMessage` 接口。
- ~~`SessionInfo` 死代码~~ ✅ 已清理：从 `chat.py` 移除未使用的 `SessionInfo` class。
- ~~RAG 双系统清理~~ ✅ 已完成：删除旧 `retriever.py`（`SemanticRetriever`，扁平 `knowledge_entries` 表）、`knowledge_base.py`（`KnowledgeBase` wrapper）、`reranker.py`（LLM-based reranker）及其 3 个测试文件（23 tests），`__init__.py` barrel 更新为仅导出 `KnowledgeLibrary` 体系。
- 更新 `AGENT.md` RAG 目录描述和 `validate-rag-pipeline` skill 使用新 `KnowledgeLibrary` API。

已验证：

- `cd apps/ai-service && uv run ruff check .`
- `cd apps/ai-service && uv run pytest`（119 tests passed）
- `cd apps/api && go test ./...`
- `cd apps/api && go vet ./...`
- `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`
- `pnpm nx run web:lint`

仍待后续批次：

- ~~LangGraph 化~~ ✅ 已在第七批实现。

### 2026-06-25 第七批实现（LangGraph 化）

已完成：

- 新增 `langgraph>=0.4.0` 依赖（`pyproject.toml`）。
- 新增 `apps/ai-service/src/services/consultation_graph.py`：基于 LangGraph StateGraph 实现显式 agent workflow，包含 `safety_check` → `classify_intent` → `generate_response` → `extract_info` → `decide_phase` → 条件路由（`generate_diagnosis` / `generate_treatment` / END）的图拓扑。
- 定义 `ConsultationState` TypedDict，`extracted_symptoms` 使用 `Annotated[..., _merge_symptoms]` 自定义 reducer 按 `body_part` 去重合并。
- 从 `ChatService` 提取纯函数：`build_messages`、`build_fallback_reply`、`_markdown_to_text`、`_chunk_text`、`_determine_phase`、`_get_conversation_text`。
- 所有事件发射节点使用 LangGraph `StreamWriter` 实现实时 SSE 流式输出。
- `ChatService.stream_chat` 改为委托 `stream_consultation` 图执行，保留辅助方法供现有测试直接调用。
- 新增 36 个图测试（`test_consultation_graph.py`）：覆盖节点级测试（safety_check、classify_intent、generate_response、extract_info、decide_phase、route_on_action）和集成测试（完整图 fallback 路径、tool calls、红旗检测、_merge_symptoms reducer）。
- 更新 `test_chat_service.py` 的 6 个 monkeypatch 目标从 `chat_service.get_llm_provider` 到 `consultation_graph.get_llm_provider`。

已验证：

- `cd apps/ai-service && uv tool run ruff check .`
- `cd apps/ai-service && uv run pytest`（155 tests passed）
- `cd apps/api && go test ./...`
- `cd apps/api && go vet ./...`
- `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`
- `pnpm nx run web:lint`

所有阶段（Phase 0-4 + LangGraph 化）均已完成。无后续待办。

### 2026-06-25 第八批实现（RAG Agent Tool + 前端测试 + 知识缺失策略 + assistant-ui 清理）

已完成：

- **RAG 作为 agent 工具**：新增 `search_knowledge` tool definition（`KNOWLEDGE_SEARCH_TOOL`），LLM 可在对话中自主调用知识库搜索。`generate_response` 节点改为多轮 tool loop（最多 `MAX_TOOL_ROUNDS=3` 轮），支持 LLM 先搜索再回答。新增 `execute_search_knowledge` 异步函数，调用 `KnowledgeLibrary.search` 并格式化结果。新增 `_emit_citation_events` 辅助函数，在 agent 搜索后动态发射 citation SSE 事件。Go `consultation_handler.go` 的 `SendMessage` 方法移除了 `searchKnowledge` 前置调用，chat 路径不再由 Go 层预检索，由 Python agent 自主决定。`generate_diagnosis` 和 `generate_treatment` 节点修复：新增内部 RAG 检索逻辑，基于提取症状/诊断名构建搜索查询，结果传给 `DiagnosisService` 的 `rag_results` 参数，修复了图路径诊断/方案无 RAG context 的回归。
- **知识缺失策略**：系统 prompt 新增"知识缺失处理"章节，指导 LLM 在知识库无匹配时明确标注不确定性、给出通用建议并引导就医。`execute_search_knowledge` 返回 `(text, has_results)` 元组，无结果时返回显式"知识库无匹配"提示。新增 `knowledge_gap` SSE 事件类型，agent 搜索无结果时发射，前端可展示不确定性提示。`useChatSSE` 新增 `onKnowledgeGap` 回调和 `KnowledgeGap` 事件处理。
- **前端测试基础设施**：`apps/web/package.json` 新增 `@testing-library/react`、`@testing-library/jest-dom`、`@testing-library/user-event`、`happy-dom` devDependencies。`apps/web/vite.config.ts` 新增 `test` 配置块（environment、include、setupFiles、globals）。新增 `apps/web/src/test-setup.ts`。新增 4 个测试文件共 51 个测试：`useChatSSE.test.ts`（15 tests，覆盖 dispatchSSEData 和 processSSELine 纯函数）、`consultationService.test.ts`（12 tests，覆盖所有 API 方法的 URL、方法、请求体、错误处理）、`InfoPanel.test.tsx`（10 tests，覆盖空状态、卡片渲染、确认/修改流程、编辑模式）、`DiagnosisPanel.test.tsx`（14 tests，覆盖诊断卡片、选择确认、citation 展示、治疗方案视图）。
- **assistant-ui 框架集成**：重写 `useAssistantChatRuntime.ts` 为 `ConsultationChatAdapter`，实现 `ChatModelAdapter` 接口，使用 async generator 逐事件 yield `ChatModelRunResult`。SSE 事件映射到 assistant-ui 原生概念：text → `TextMessagePart`、citation → `SourceMessagePart`、extracted_info → `DataMessagePart`、phase/red_flag → `metadata.custom`。复用 `useChatSSE.ts` 导出的 `processSSELine` / `dispatchSSEData` 纯函数。重写 `AssistantChatPanel.tsx` 使用 `useLocalRuntime` + `AssistantRuntimeProvider` 驱动交互，消息状态由 assistant-ui thread 管理。`useChatSSE` hook 保留但不再被 `AssistantChatPanel` 直接使用。

已验证：

- `cd apps/ai-service && uv run pytest`（156 tests passed）
- `cd apps/ai-service && uv run ruff check .`
- `cd apps/api && go test ./...`
- `cd apps/api && go vet ./...`
- `cd apps/web && npx vitest run`（51 tests passed）
- `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`
- `pnpm nx run web:lint`

无后续待办。

### 2026-06-25 第九批实现（体验优化）

已完成：

- **知识缺失可视化**：`AssistantChatPanel.tsx` 新增 `knowledgeGaps` 状态，从 thread 消息的 `knowledge_gap` data parts 中提取查询内容。新增橙色提示条 UI（背景 `#FFF8F0`，边框 `#F0D4B0`），显示"知识库中暂未收录「xxx」的专项资料，以下建议仅供参考"。新消息发送时自动清空。
- **Prompt 优化**：`consultation.py` 的 `SYSTEM_PROMPT` 新增"对话示例"章节，包含 4 个 few-shot 示例：症状信息提取、主动搜索知识库、知识缺失时的处理、综合分析。示例展示了 tool call 的正确使用时机和参数格式。新增"重要原则"补充：聚焦单个信息点、编号分段、先共情再建议。
- **Citation ID 真实化**：经代码审查确认当前实现已使用 `problem_slug` 作为 citation ID，无占位 ID 问题。无需修改。
- **LangGraph State TypedDict 增强**：经代码审查确认 `ConsultationState` 已使用 `TypedDict(total=False)` 定义，各字段类型明确。当前类型安全性足够，无需进一步收紧。

已验证：

- `cd apps/ai-service && uv run pytest`（156 tests passed）
- `cd apps/ai-service && uv run ruff check .`
- `cd apps/web && npx vitest run`（51 tests passed）
- `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`

所有优化项已完成。
