# Consultation Streaming Rendering — 实施流程

> 基于 [consultation-streaming-rendering-architecture-plan.md](./consultation-streaming-rendering-architecture-plan.md) 生成，结合当前代码现状派生出的具体执行路径。

## 完成状态（2026-07-01）

| Phase | 状态 | 关键产出 |
|-------|------|----------|
| Phase 1 幂等止血 | ✅ 完成 | `streamEventReducer` lastSeq 守卫 + tool.call upsert + tool.result 占位补建；`ToolCallCard` useMemo 去重防线 |
| Phase 2 ActiveTurnState | ✅ 完成 | `activeTurnReducer` + `activeTurnSelectors` + `ActiveTurnContext`双 context + `StreamingAssistantTurn` + `StreamingTurnToolCalls` |
| Phase 3 UI 数据源切换 | ✅ 完成 | `AssistantChatPanel` 移除线程消息派生 + `useAssistantChatRuntime` 最终 yield 简化 + `StreamingTextContext` 删除 |
| Phase 4 后端幂等收口 | ✅ 完成 | `openai_compatible.py` finish_reason 守卫 + 去重；`orchestrator.py` tool_call_id 去重 |
| 收尾 | ✅ 完成 | `streamEventReducer.ts` + 测试删除；全部 96 前端 + 187 Python 测试通过 |

---

## 总体路线

4 个 Phase 按依赖顺序推进，每个 Phase 内有独立可验证的提交切片。

```
Phase 1 (止血) ──→ Phase 2 (ActiveTurnState) ──→ Phase 3 (UI切换) ──→ Phase 4 (后端收口)
     2-3天               3-4天                      2-3天                 1-2天
```

跨 Phase 的收尾工作（删除旧代码、文档同步、回归验收）在 Phase 3 完成后集中执行。

---

## Phase 1: 幂等止血与最小稳定化

**现状**：`streamEventReducer.ts` 对 `tool.call` 是 append 而非 upsert，无 `lastSeq` 字段，`tool.result` 匹配逻辑可接受但无占位补建。`ToolCallCard.tsx` 无去重保护。测试覆盖缺 tool 事件。

**目标**：消除重复 key 和控制台报错，让 tool call UI 不重复。

### 执行步骤

#### 1.1 Reducer 幂等改造

**文件**：`apps/web/src/features/consultation/runtime/streamEventReducer.ts`

在 `ConsultationStreamState` 中增加 `lastSeq: number`，初始值 `-1`。

在 `reduceStreamEvent()` 函数体最顶部加入 seq 守卫：

```
if (event.seq !== undefined && event.seq <= state.lastSeq) {
  return { state, effects: [] };
}
```

在每个 case 分支末尾将 `event.seq` 写入 `lastSeq`（仅当 `event.seq !== undefined`）。

改造 `tool.call` 处理：从 `[...state.toolCalls, newToolCall]` 改为按 `tool_call_id` 查找，存在则返回原状态（字段相同时）或原地替换，不存在则追加。

改造 `tool.result` 处理：优先按 `tool_call_id` 精确匹配，无匹配时按 `tool_name` 兜底匹配第一个 running 态同类型工具；以上都不命中时，用 `tool_call_id` 补建一个 placeholder 工具项并标记为 completed。

为 `source.knowledge_gap` 增加按 `query` 去重（当前是裸 append）。

新增三个内部 helper：
- `upsertToolCall(existing: ToolCallInfo[], incoming: ToolCallInfo): ToolCallInfo[]`
- `completeToolCall(existing: ToolCallInfo[], toolCallId: string, toolName: string, result: unknown): ToolCallInfo[]`
- `dedupKnowledgeGap(existing: KnowledgeGap[], incoming: KnowledgeGap): KnowledgeGap[]`

#### 1.2 ToolCallCard 防线加固

**文件**：`apps/web/src/features/consultation/components/ToolCallCard.tsx`

在渲染前对 `toolCalls` 做 `useMemo` 去重——按 `tool_call_id` 去重，保留后出现的（latest wins）。排序：running 在前，completed 在后，同状态保持原始顺序。`key` 使用 `tool_call_id` 而非 index。

#### 1.3 useAssistantChatRuntime 类型适配

**文件**：`apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`

若 reducer state 新增了 `lastSeq` 字段，同步更新该 hook 中对 reducer state 的类型引用（如有）。

#### 1.4 测试补齐

**更新** `apps/web/src/features/consultation/runtime/streamEventReducer.test.ts`，新增 6 个用例：
- `ignores event with seq older than lastSeq`
- `upserts duplicate tool.call by tool_call_id`
- `marks existing tool call completed on tool.result`
- `creates placeholder tool call when tool.result arrives first`
- `deduplicates knowledge gaps by query`
- `keeps citation deduplication stable after repeated events`

**新增** `apps/web/src/features/consultation/components/__tests__/ToolCallCard.test.tsx`，覆盖：
- 相同 `tool_call_id` 出现两次时只渲染一次
- running 状态显示转圈，completed 状态显示勾选
- `ask_user` 工具不显示在工具卡片中
- 同 summary 文本但不同 id 的工具项可并存

### 验证命令

```
pnpm nx run web:test -- --runInBand streamEventReducer
pnpm nx run web:test -- --runInBand ToolCallCard
pnpm nx run web:typecheck
```

### 提交切片

1. `fix(web): make consultation tool-call reducer idempotent` — reducer 改造 + helper
2. `test(web): cover duplicate tool events in consultation chat` — 测试补齐

---

## Phase 2: 建立 ActiveTurnState 基础设施

**现状**：`StreamingTextContext` 只管理纯文本，tool calls / citations / interaction 等仍散落在 `AssistantChatPanel` 本地 state 和 `thread.messages` 派生逻辑中。`streamEventReducer` 已有完整状态形状，但缺少 `lastSeq`、`finalParts` 生成和完成后的重置逻辑。

**目标**：用完整 `ActiveTurnState` 替代单一文本 context，统一流式阶段的全部 UI 数据源。

### 执行步骤

#### 2.1 新建 activeTurnReducer

**文件**：`apps/web/src/features/consultation/runtime/activeTurnReducer.ts`（新建）

定义类型：

```ts
type StreamStatus = 'idle' | 'streaming' | 'interrupted' | 'completed' | 'failed';

interface ActiveTurnState {
  runId: string | null;
  conversationId: string | null;
  assistantMessageId: string | null;
  status: StreamStatus;
  text: string;
  toolCallsById: Record<string, ToolCallVM>;
  citationsByKey: Record<string, CitationVM>;
  knowledgeGapsByKey: Record<string, KnowledgeGapVM>;
  redFlag: RedFlagEvent | null;
  pendingInteraction: PendingInteraction | null;
  extractedInfoByBodyPart: Record<string, ExtractedInfo>;
  finalParts: MessagePartVM[];
  lastSeq: number;
  error?: string;
}
```

实现 `reduceActiveTurnEvent(state, event): { state, effects }`。整体逻辑基于现有 `streamEventReducer` 演进，但将所有数组结构改为 `Record<id, T>`（Map 语义），天然支持 upsert。完成事件（`message.completed` / `stream.done`）时调用 `buildFinalParts(state)` 生成 `finalParts`。

核心 helper：
- `appendStreamingText(state, delta)`
- `upsertToolCallVM(state, toolCall)` — 按 `tool_call_id` upsert 到 map
- `upsertCitationVM(state, citation)` — 按主键 upsert
- `upsertKnowledgeGapVM(state, gap)` — 按 `query` upsert
- `upsertExtractedInfoVM(state, info)` — 按 `body_part` merge
- `buildFinalMessageParts(state): MessagePartVM[]` — 将 text + structured events 扁平化为 parts 数组

导出 `INITIAL_ACTIVE_TURN_STATE` 常量和 `resetActiveTurnState()` 工厂函数。

#### 2.2 新建 activeTurnSelectors

**文件**：`apps/web/src/features/consultation/runtime/activeTurnSelectors.ts`（新建）

```ts
selectActiveTurnViewModel(state: ActiveTurnState): ActiveTurnViewModel
selectVisibleToolCalls(state: ActiveTurnState): ToolCallVM[]  // 过滤 ask_user，排序
selectStreamingMarkdown(state: ActiveTurnState): string
selectIsInterrupted(state: ActiveTurnState): boolean
selectHasVisibleContent(state: ActiveTurnState): boolean
```

ViewModel 层负责排序、过滤 `ask_user` 工具、判断可见性。

#### 2.3 新建 ActiveTurnContext

**文件**：`apps/web/src/features/consultation/context/ActiveTurnContext.tsx`（新建）

双 context 结构（与现有 `StreamingTextContext` 模式一致，避免全树 re-render）：
- `ActiveTurnStateContext` — 暴露当前 `ActiveTurnState`
- `ActiveTurnActionsContext` — 暴露 `dispatchEvent`、`resetTurn`、`hydrateTurn`

内部使用 `useReducer(reduceActiveTurnEvent, INITIAL_ACTIVE_TURN_STATE)` 驱动。

#### 2.4 新建流式 UI 组件

**文件**：`apps/web/src/features/consultation/components/StreamingAssistantTurn.tsx`（新建）

组合渲染：流式 markdown、工具调用进度、引用列表、knowledge gap、red flag、AskUserCard。所有数据从 `useActiveTurnViewModel()` selector 读取。

**文件**：`apps/web/src/features/consultation/components/StreamingTurnToolCalls.tsx`（新建）

从 `ToolCallCard` 中抽离出的纯展示组件，只接收 selector 归一化后的 view model。

#### 2.5 既有模块改造

**`useAssistantChatRuntime.ts`**：
- 将 `reduceStreamEvent` 替换为 `reduceActiveTurnEvent`
- `applyEffects` 中的回调改为 dispatch 到 active turn context 而非直接操作文本 context/local state
- 完成态仍走现有 final yield 路径，暂不改动提交机制

**`AssistantChatPanel.tsx`**：
- 用 `ActiveTurnProvider` 包裹聊天区域
- 将流式区渲染改为 `<StreamingAssistantTurn />`，从 active turn 读数据
- 暂时保留旧的 `StreamingTextContext` / `streamingTextRef` / citation 提取 effect，作为过渡期兼容（Phase 3 再删除）

**`StreamingTextContext.tsx`**：
- 标记 `@deprecated`，添加 JSDoc 说明迁移中

#### 2.6 测试补齐

**新增** `activeTurnReducer.test.ts`：
- aggregates text, tools, citations, and interaction into one state
- builds finalParts on message.completed
- resets state after completion
- keeps interrupted state on interaction.required
- preserves prior text when interaction event arrives
- ignores duplicate tool.call by tool_call_id
- tool.result creates placeholder when no prior tool.call

**新增** `activeTurnSelectors.test.ts`：
- selectVisibleToolCalls filters ask_user
- selectActiveTurnViewModel sorts and normalizes tool calls
- selectHasVisibleContent returns true for text-only and tool-only states

**新增** `StreamingAssistantTurn.test.tsx`：
- 流式 markdown 文本正确显示
- tool calls 随 state 更新
- interruption 时显示 AskUserCard
- 没有内容时显示加载态

### 验证命令

```
pnpm nx run web:test -- --runInBand activeTurnReducer
pnpm nx run web:test -- --runInBand activeTurnSelectors
pnpm nx run web:test -- --runInBand StreamingAssistantTurn
pnpm nx run web:typecheck
```

### 提交切片

1. `refactor(web): introduce consultation active turn reducer and selectors`
2. `feat(web): render consultation streaming turn from active turn state`

---

## Phase 3: UI 数据源切换与完成态提交流程

**现状**：`AssistantChatPanel` 用 `useEffect` 从 `thread.messages` 派生 citation / red flag / gap。`useAssistantChatRuntime` 的 applyEffects 中有 `onToolCallUpdate` 回调更新 local state。`StreamingTextContext` 仍在工作。

**目标**：聊天页不再从 `thread.messages` 派生任何流式业务状态，完成"流式态→历史态"的正式切换。

### 执行步骤

#### 3.1 AssistantChatPanel 重构

**文件**：`apps/web/src/features/consultation/components/AssistantChatPanel.tsx`

删除项：
- 从 `thread.messages` 提取 citation / red flag / gap 的 `useEffect`
- 依赖 `message.status === running && message.content empty` 的隐式流式判断
- `streamingTextRef` bridge 及相关代码
- `StreamingTextBridge` 组件
- 本地 `toolCalls`、`pendingInteraction` state（已迁移到 active turn）

新增/调整项：
- 流式区改为 `<StreamingAssistantTurn />`，当 `activeTurn.status === 'streaming' | 'interrupted'` 时在消息列表末尾渲染
- 历史消息统一切到 `MessagePrimitive.Parts`，为 assistant 和 user 消息分别定义稳定 render path
- 可选拆分：`HistoricalAssistantMessage`、`HistoricalUserMessage` 子组件

#### 3.2 完成态提交机制

**文件**：`apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`

在 `stream.done` / `message.completed` 到达后：
1. 从 reducer state 取出 `finalParts`（已由 reducer 在完成事件时生成）
2. 将 `finalParts` 一次性 yield 给 assistant-ui runtime
3. yield 完成后调用 `resetTurn()` 清空 active turn

保留现有的 `ChatModelAdapter.run()` 异步生成器模式，只调整 yield 时机和内容来源。

#### 3.3 ConsultationPage 梳理

**文件**：`apps/web/src/features/consultation/pages/ConsultationPage.tsx`

确认 `initialMessages` 仅代表历史消息，不再假设页面级 state 感知流式工具调用细节。仅保留会话级元数据回调（extracted info / phase / title），这些从 active turn effects 中触发。

#### 3.4 测试补齐

**更新/新增** `AssistantChatPanel.test.tsx`：
- 流式中显示 StreamingAssistantTurn，完成后切换为历史消息
- citation / red flag / gap 不再依赖 thread.messages
- 完成切换后 active turn 被清空
- 历史 assistant message 使用 markdown parts 渲染

**新增集成测试** `AssistantChatPanel.integration.test.tsx`：
- 模拟完整一轮交互：用户发消息 → text delta → tool.call → tool.result → message.completed → UI 从流式态平滑切换到历史态

### 验证命令

```
pnpm nx run web:test -- --runInBand AssistantChatPanel
pnpm nx run web:test -- --runInBand consultation
pnpm nx run web:lint
pnpm nx run web:typecheck
```

### 提交切片

1. `refactor(web): decouple consultation streaming ui from thread.messages`
2. `refactor(web): render consultation history with assistant-ui parts`

---

## Phase 4: 后端事件幂等收口

**现状**：`openai_compatible.py` 的 `tool_call_accumulators` 生命周期未明确，finish_reason 多次到达时可能重复发射 `tool_call_done`。`orchestrator.py` 的 `completed_tool_calls` 无去重。

**目标**：让重复事件在上游就尽量不产生。

### 执行步骤

#### 4.1 Provider 层去重

**文件**：`apps/ai-service/src/ai/providers/openai_compatible.py`

- 新增实例属性 `emitted_tool_call_ids: set[str]`
- 在发射 `tool_call_done` 前检查是否已发射过该 id，已发射则跳过
- 在流式响应结束后（或异常时）清理 `emitted_tool_call_ids`
- 如 finish_reason 被观察到多次，对同一 tool_call_id 只发射一次 `tool_call_done`

#### 4.2 Orchestrator 层去重

**文件**：`apps/ai-service/src/services/agent/orchestrator.py`

- 新增 helper `_dedupe_completed_tool_calls(completed_tool_calls)`，按 `tool_call_id` 去重聚合
- 在 `_handle_tool_calls()` 前调用去重
- 写出 `tool_call` 事件前校验本轮是否已写出

#### 4.3 测试补齐

**更新** `apps/ai-service/tests/unit/test_chat_service.py`：
- 同一 tool call 在重复 provider finish 下只映射一次 tool.call
- tool.result 仍正常发射

**更新** `apps/ai-service/tests/unit/test_consultation_graph.py`：
- 重复 tool_call_done 不会导致重复 writer 事件

### 验证命令

```
cd apps/ai-service && uv run pytest tests/unit/test_chat_service.py
cd apps/ai-service && uv run pytest tests/unit/test_consultation_graph.py
cd apps/ai-service && uv run ruff check .
```

### 提交切片

1. `fix(ai): dedupe tool call done events in provider stream`
2. `fix(ai): prevent duplicate consultation tool events in orchestrator`

---

## 收尾阶段

在 Phase 3 完成后，Phase 4 可与其并行执行。以下在所有 Phase 代码合入后统一执行。

### 删除旧代码

- 删除 `StreamingTextContext.tsx`
- 删除 `streamEventReducer.ts`（已被 `activeTurnReducer.ts` 替代）
- 删除 `AssistantChatPanel.tsx` 中的 `StreamingTextBridge`、`streamingTextRef`、旧的 `useEffect` 派生逻辑

### 文档同步

- 在本方案文档开头标记各 Phase 完成状态
- 若有与原始 plan 的偏差，在 plan 文档末尾"最终设计决策"一节回填

### 回归清单

逐项手工验证：

1. 新咨询草稿第一轮消息 — 正常流式显示
2. 已存在会话继续发送消息 — 历史消息保留，新消息流式显示
3. 工具调用与知识引用同轮出现 — 都在 active turn UI 中展示
4. `ask_user` 中断并恢复 — 卡片显示后回答可恢复流式
5. 重复 `tool.call` 不再报错 — 控制台无 duplicate key
6. 弱网长文本无闪烁 — token 级更新平滑

---

## 依赖与风险

| 风险 | 缓解 |
|------|------|
| `assistant-ui` local runtime 的"追加最终 assistant message"接口可能与预期不符 | Phase 3 前单独验证一次 yield，不阻塞前两个 Phase |
| 后端事件顺序在现实中不稳定（如 `tool.result` 先于 `tool.call` 到达） | Phase 1 已覆盖占位补建逻辑，后续发现新乱序模式可增量补齐 |
| 短期内代码行数增加 | 每个 Phase 独立可回滚，Phase 2 引入的 state 层在 Phase 3 验证稳定后再删旧代码 |
