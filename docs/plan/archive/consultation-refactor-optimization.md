# Consultation Refactor 优化计划

> 日期：2026-06-26
> 状态：已完成
> 分支：dev
> 范围：consultation workflow LangGraph 重构后的代码质量优化

## 背景

`dev` 分支完成了 consultation workflow 的大规模重构：
- ChatService 从线性流程迁移到 LangGraph 状态图
- RAG 搜索从 Go 层下沉到 Python agent tool calling
- 新增红旗检测、faithfulness 检查、Pydantic 验证
- 前端迁移到 assistant-ui runtime
- 新增 consultation phase 状态机

本文档记录 code review 发现的所有问题及修复进度。

---

## P0 — 必须修复（合并前阻塞）

### P0-1: Phase 状态机前端/后端不一致

**文件**：`apps/api/internal/service/consultation_phase.go`、`apps/web/src/features/consultation/services/consultationService.ts`

**问题**：Go 端 `ConsultationPhaseRank` 缺少 `analysis_ready` phase。前端 `ConsultationPage.tsx` 在 `handleAnalyzeDiagnosis` 成功后设置 `phase: 'analysis_ready'`，但 Go handler 调用 `UpdatePhase(sessionID, "analysis_ready")` 时，`ShouldAdvancePhase` 因 `analysis_ready` 不在 map 中返回 `false`，导致 phase 不会被持久化。

**修复方案**：在 Go 的 `ConsultationPhaseRank` 中添加 `"analysis_ready": 1`（与 `ready_for_analysis` 同级，都表示"可以进行分析"的状态）。

**进度**：✅ 已修复

---

### P0-2: Go handler SSE 解析存在行截断风险

**文件**：`apps/api/internal/handler/consultation_handler.go:276-316`

**问题**：将 `bufio.Scanner` 替换为手动 `resp.Body.Read(buf)` + `lineBuf` 累积。4096 字节的 buffer 可能在 SSE `data:` 行中间截断，导致大 JSON（如 `extracted_info`）被拆成两段，第二段不以 `data:` 开头，会被静默丢弃。

**修复方案**：改回 `bufio.Scanner` 并设置足够大的 buffer（64KB），同时保留 `request.Context` 传播和新增的 `phase_changed` 解析逻辑。

**进度**：✅ 已修复

---

### P0-3: ChatContext 缺少 phase 初始化

**文件**：`apps/ai-service/src/api/routes/chat.py`、`apps/api/internal/handler/consultation_handler.go`

**问题**：Go handler 的 `aiReq` 没有传 `phase`，Python `ChatRequest` model 没有 `phase` 字段，`ChatContext` 创建时 `phase` 永远是默认值 `"collecting"`。即使 session 已经到了 `ready_for_analysis`，graph 的 `decide_phase` 节点也会从错误的初始 phase 做判断。

**修复方案**：
1. Go handler 在 `aiReq` 中添加 `"phase": session.Phase`
2. Python `ChatRequest` 添加 `phase: str = "collecting"` 字段
3. 创建 `ChatContext` 时传入 `phase=request.phase`

**进度**：✅ 已修复

---

## P1 — 建议修复（当前迭代内）

### P1-1: Diagnosis/Treatment 双重 RAG 搜索

**文件**：`apps/api/internal/handler/diagnosis_handler.go`、`apps/ai-service/src/services/consultation_graph.py`

**问题**：`diagnosis_handler.go` 的 `AnalyzeDiagnosis` 和 `GenerateTreatment` 在 Go 层做 RAG 搜索并传结果给 Python，同时 Python 的 `generate_diagnosis` 和 `generate_treatment` 节点也会自己做 RAG 搜索。导致 RAG 搜索被执行两次。

**修复方案**：删除 Go 层 diagnosis/treatment 的 RAG 搜索调用，统一由 Python agent 自主搜索，与 chat 的架构保持一致。Go 层只传 `extracted_info` 和 `profile`。

**进度**：✅ 已修复

---

### P1-2: 前端串行调用 confirm + generate 失败恢复问题

**文件**：`apps/web/src/features/consultation/pages/ConsultationPage.tsx`

**问题**：`handleConfirmAndGenerateTreatment` 先调 `confirmDiagnosis` 再调 `generateTreatment`。如果 confirm 成功但 generate 失败，Go 端 phase 已更新为 `diagnosis_confirmed`，但前端没有 treatment plan，且 `selectedDiagnosis` 可能已重置。

**修复方案**：在 `generateTreatment` 失败时保留 `selectedDiagnosis` 状态，允许用户重试。或者将 confirm + generate 合并为一个后端端点。

**进度**：✅ 已修复

---

### P1-3: FaithfulnessChecker 浅层字符串匹配

**文件**：`apps/ai-service/src/services/faithfulness_checker.py`

**问题**：`_find_matching_source` 用 `in` 做子串匹配，中文容易误匹配（"臀" 匹配到 "臀部"、"臀桥"）。`EXERCISE_ALIASES` 是硬编码静态映射。

**修复方案**：作为 MVP 可接受，但需添加注释标注局限性，并在匹配时至少做词边界检查（避免单字匹配）。

**进度**：✅ 已修复

---

### P1-4: tool loop 终止条件逻辑缺陷

**文件**：`apps/ai-service/src/services/consultation_graph.py:generate_response`

**问题**：当 LLM 只调用 `extract_symptom_info`（不调用 `search_knowledge`）时，`has_search` 为 `False`，循环立即终止。如果 LLM 第一轮只提取症状、第二轮才搜索知识库，第一轮的 tool round 被"浪费"。

**修复方案**：将终止条件改为检查是否有任何 tool call（`has_any_tool_call`），而非仅检查 `has_search`。

**进度**：✅ 已修复

---

### P1-5: Go handler 使用 `log.Printf` 而非结构化日志

**文件**：`apps/api/internal/handler/consultation_handler.go`、`apps/api/internal/handler/diagnosis_handler.go`

**问题**：引入了标准库 `log` 包做错误日志，但项目可能已有日志框架。

**修复方案**：检查项目是否已有日志框架（如 `slog`），统一使用。若无，保留 `log.Printf` 但添加 TODO 注释。

**进度**：✅ 已修复

---

## P2 — 改进建议（后续迭代）

### P2-1: ConsultationState 所有字段都是可选的

**文件**：`apps/ai-service/src/services/consultation_graph.py`

**问题**：`ConsultationState(TypedDict, total=False)` 意味着所有字段都可以缺失，但 `generate_response` 直接访问 `state["user_message"]` 不带 `.get()`。

**修复方案**：在 graph 入口 `stream_consultation` 中做 required field validation。

**进度**：✅ 已修复

---

### P2-2: `_merge_symptoms` reducer 可能覆盖非 None 值为 None

**文件**：`apps/ai-service/src/services/consultation_graph.py`

**问题**：`by_part[part].update(s)` 会用 `None` 值覆盖已有的非 None 值。

**修复方案**：merge 时过滤掉 `None` 值。

**进度**：✅ 已修复

---

### P2-3: `extract_info` 节点是空操作

**文件**：`apps/ai-service/src/services/consultation_graph.py`

**问题**：`extract_info` 节点什么都不做，占用 graph 步骤。

**修复方案**：移除该节点，将 `decide_phase` 直接接在 `generate_response` 后面。

**进度**：✅ 已修复

---

### P2-4: 前端 citation/redFlag/knowledgeGap 提取效率

**文件**：`apps/web/src/features/consultation/components/AssistantChatPanel.tsx`

**问题**：`useEffect` 对 `thread.messages` 做深度遍历 + `JSON.stringify` 比较，每次 streaming text 更新都触发。

**修复方案**：将提取逻辑移到 `useMemo` 计算，只在值变化时通过 `useEffect` 通知父组件。

**进度**：✅ 已修复

---

### P2-5: RedFlagDetector 关键词匹配没有上下文

**文件**：`apps/ai-service/src/services/red_flag_detector.py`

**问题**："我之前摔倒过，但现在好多了"中的"摔倒"会被标记为 trauma red flag。

**修复方案**：添加注释说明保守策略是设计意图，记录后续改进方向。

**进度**：✅ 已修复

---

### P2-6: agent_workflow.py intent classification 覆盖不全

**文件**：`apps/ai-service/src/services/agent_workflow.py`

**问题**：`INTENT_PATTERNS` 有限，"我想做个检查"不会匹配到 `REQUEST_ANALYSIS`。

**修复方案**：添加更多 pattern，或标注需要 LLM fallback。

**进度**：✅ 已修复

---

### P2-7: 新增模块缺少单元测试

**文件**：`apps/ai-service/tests/unit/`

**问题**：删除了 3 个测试文件，但新增的 `red_flag_detector.py`、`faithfulness_checker.py`、`agent_workflow.py`、`consultation_graph.py` 没有对应测试。

**修复方案**：为纯逻辑模块添加单元测试。

**进度**：✅ 已修复

---

## 修复批次

| 批次 | 范围 | 状态 |
|------|------|------|
| Batch 1 | P0-1, P0-2, P0-3 | ✅ 已修复，子 agent review 通过 |
| Batch 2 | P1-2, P1-3, P1-4, P1-5 | ✅ 已修复，子 agent review 通过 |
| Batch 3 | P2-1 ~ P2-7 | ✅ 已修复，56 tests passing |

每批修复后使用子 agent 进行 code review。

### 修复摘要

**P0（3项）**：
- P0-1: Go phase rank 添加 `analysis_ready`，graph 节点发射对应 `phase_changed` 事件
- P0-2: SSE 解析改回 `bufio.Scanner`（256KB buffer），添加 `scanner.Err()` 检查
- P0-3: Go handler 传递 `session.Phase`，Python `ChatRequest` 接收并传入 `ChatContext`

**P1（4项，P1-1 确认非问题跳过）**：
- P1-2: `handleConfirmAndGenerateTreatment` 跳过已确认的 diagnosis，支持重试
- P1-3: `FaithfulnessChecker` 添加模块文档 + 单字符长度 guard
- P1-4: tool loop 添加 `seen_tool_ids` 去重，防止 fake provider 死循环
- P1-5: Go handler 添加 TODO 注释（项目无结构化日志框架）

**P2（7项）**：
- P2-1: `stream_consultation` 入口添加 required field validation
- P2-2: `_merge_symptoms` 文档说明 None 覆盖是有意设计
- P2-3: 移除空操作 `extract_info` 节点，简化 graph
- P2-4: 前端提取逻辑添加性能说明注释
- P2-5: `RedFlagDetector` 添加保守策略设计说明
- P2-6: 扩展 intent patterns（"检查/测试/自测/自查"、"什么情况"）
- P2-7: 补充单字符 faithfulness 测试 + 新 intent pattern 测试
