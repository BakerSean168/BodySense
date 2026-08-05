# Consultation Streaming Architecture — Code Review

> 审查范围：Phase 1-4 全部改动（前台 reducer/context/组件 + 后端 provider/orchestrator）
> 审查日期：2026-07-01
> 修复日期：2026-07-01（同日修复全部发现项）

---

## 修复状态

| 编号 | 严重度 | 状态 |
|------|--------|------|
| C1 | 严重 | ✅ 已修复 — `dispatchEvent` JSDoc 标注 effects 不转发 |
| H1 | 高 | ✅ 已修复 — `lastSeq` 更新加 `event.seq !== undefined` 守卫 |
| H2 | 高 | ✅ 已修复 — 改用 `ThreadAssistantMessagePart`，`buildFinalMessageParts` 产出兼容类型，hook 最终 yield `finalParts` |
| H3 | 高 | ✅ 已修复 — 名称兜底匹配路径加 `console.warn` 日志 |
| M1 | 中 | ✅ 已修复 — `useRef` 稳定 actions 对象引用 |
| M2 | 中 | ✅ 已修复 — `crypto.randomUUID().slice(0,8)` 替代计数器后缀 |
| M3 | 中 | ✅ 已修复 — `onResume` 回调提升到 `ChatContent`，API 调用由父组件负责 |
| L1 | 低 | ✅ H2 修复后自动解决 — `finalParts` 被 hook 消费 |
| L2 | 低 | ✅ 已修复 — 新增 9 个测试（seq guard, tool.result completion, stream.error, message.failed, interaction.answered, knowledge_gap dedup, phase_changed）|
| L3 | 低 | ✅ 已修复 — 添加 `dismissRedFlag` action，流式中可关闭红旗 |
| L4 | 低 | ✅ 已修复 — 抽取 `ToolCallItem` 公共组件 |

## 总体评价

架构分离方向正确，`ActiveTurnState` 作为流式阶段唯一真值源的设计落地整洁。前端 Record-map 天然 upsert 语义消除了旧数组方案的重复 key 问题。后端两层去重（provider + orchestrator）形成了纵深防线。所有发现项已修复。

最终验证：前端 105 测试 + typecheck 零错误，Python 42 测试 + ruff 零问题。

## 严重（CRITICAL）

### C1. `ActiveTurnContext.turnReducer` 丢弃 effects

**文件**：`ActiveTurnContext.tsx` 第 52-61 行

```ts
function turnReducer(state: ActiveTurnState, action: TurnAction): ActiveTurnState {
  switch (action.type) {
    case 'DISPATCH_EVENT':
      return reduceActiveTurnEvent(state, action.event).state;  // .effects 被丢弃
    ...
  }
}
```

`reduceActiveTurnEvent` 返回 `{ state, effects }`，但 `turnReducer` 只取 `.state`，effects 全部丢失。当前主流程不经过此路径（事件通过 `useAssistantChatRuntime` 内部的 `dispatch()` 函数处理，effects 在那里被 `applyEffects` 消费），上下文仅通过 `HYDRATE_TURN` 被同步。但如果未来任何组件直接调用 `useActiveTurnActions().dispatchEvent()`，phase_changed、citation_added 等父级回调将不会触发。

**建议**：至少在 `dispatchEvent` 的 JSDoc 上标注 "effects are not forwarded through context; use HYDRATE_TURN for full reducer results"，或在上下文层提供一个 effects 回调注册机制。

---

## 高（HIGH）

### H1. `lastSeq` 可能被设为 `undefined`

**文件**：`activeTurnReducer.ts` 第 395-397 行

```ts
if (processed) {
  next = { ...next, lastSeq: event.seq };
}
```

当 `event.seq === undefined` 且事件被处理（`processed = true`）时，`lastSeq` 被设为 `undefined`。这会丢失此前已追踪的 seq 值。虽然顶部守卫 `event.seq <= current.lastSeq` 中 `number <= undefined` 为 `false`（不会误拦截），但信息丢失意味着后续有 seq 的事件无法与之前的状态做幂等比较。

**建议**：加上 `event.seq !== undefined` 守卫。

```ts
if (processed && event.seq !== undefined) {
  next = { ...next, lastSeq: event.seq };
}
```

### H2. 结构化事件在完成态丢失

**文件**：`useAssistantChatRuntime.ts` 第 199-203 行

```ts
// Final yield: submit the complete text to assistant-ui runtime.
if (reducerState.text) {
  yield {
    content: [{ type: 'text', text: reducerState.text }],
  };
}
```

最终提交只包含纯文本。citations、red flags、knowledge gaps 在流式阶段由 `StreamingAssistantTurn` 渲染，但 turn 完成后这些结构化数据不会出现在历史消息中。`buildFinalMessageParts()` 已正确生成了包含 source/data parts 的 `finalParts`，但类型与 `assistant-ui` 的 `ThreadAssistantMessagePart` 不兼容，导致 `finalParts` 被生成后从未被消费。

**影响**：用户滚动历史消息时看不到本轮 assistant 引用了哪些知识条目、是否命中了知识缺口、有无红旗警告。

**建议**（二选一）：

A. 修复类型兼容，将 `finalParts` 提交到 thread。需要补 `sourceType`、`id`、`mediaType` 等必填字段。
B. 如果短期内无法解决类型兼容，在 `buildFinalMessageParts` 的 JSDoc 和架构文档中标注"仅流式阶段可见，完成态切换后消失"，避免后续开发者疑惑。

### H3. `tool.result` 名称兜底匹配可能命中错误工具

**文件**：`activeTurnReducer.ts` 第 356-369 行

```ts
const fallbackEntry = Object.entries(current.toolCallsById).find(
  ([, tc]) => tc.tool === payload.tool && tc.status === 'running',
);
```

当 `tool.result` 的 `tool_call_id` 未被识别时，按 `tool_name + running` 兜底匹配第一个符合条件的工具。如果同轮有两个 `search_knowledge` 工具同时 running，`Object.entries` 返回插入顺序，result 可能被分配给错误的工具调用。

**建议**：至少按创建时间排序选最近的一个；更根本的方案是确保后端始终在 `tool.result` 事件中带上准确的 `tool_call_id`（当前 orchestrator 已保证这一点，此路径仅为防御性代码）。可加一条 warn 日志标记命中了 fallback 路径。

---

## 中（MEDIUM）

### M1. `ActiveTurnActionsContext` 值引用每帧重建

**文件**：`ActiveTurnContext.tsx` 第 83-84 行

```ts
const actionsRef = useRef<ActiveTurnActions>({ dispatchEvent, resetTurn, hydrateTurn });
actionsRef.current = { dispatchEvent, resetTurn, hydrateTurn };
```

三函数本身通过 `useCallback([], [])` 稳定，但包含它们的新对象 `{ ... }` 每帧创建，传给 Context.Provider 的 value 引用每次都变。这导致所有消费 `useActiveTurnActions()` 的组件在每次 state 更新时都 re-render（即使它们不需要 state）。

**当前影响**：`ActiveTurnBridge` 是唯一主动消费 actions 的组件，它是一个返回 `null` 的无 UI 组件，所以实际上没有可见的性能影响。但如果后续有组件同时消费 actions 和自身 local state，会出现不必要的 re-render。

**建议**：去掉 `actionsRef.current` 的每帧赋值，把初始对象直接放进 ref 不动。

```ts
const actions = useRef<ActiveTurnActions>({ dispatchEvent, resetTurn, hydrateTurn });
// 不再赋值 actions.current
```

因为三个函数稳定，对象内容永远不会变。

### M2. `tool.call` 无 ID 时的 fallback 生成方式脆弱

**文件**：`activeTurnReducer.ts` 第 319 行

```ts
const toolCallId = event.ids.tool_call_id || `tc_${Object.keys(current.toolCallsById).length}`;
```

用当前 map 的 key 数量作为后缀。如果未来支持从 map 中移除 tool call（例如取消），计数器可能重复。

**建议**：使用 `crypto.randomUUID().slice(0, 8)` 后缀，保证唯一性。

### M3. `StreamingAssistantTurn` 硬编码 API 调用

**文件**：`StreamingAssistantTurn.tsx` 第 46 行

```ts
const result = await consultationApi.resumeInteraction(
  conversationId,
  interaction.id,
  answer,
);
```

UI 组件直接依赖 API 服务层。这使得组件在不同场景（测试、storybook、复用）中难以替换交互恢复逻辑。

**建议**：将 `onResumeComplete` 改为 `onResume: (interactionId: string, answer: unknown) => Promise<{ answer_text?: string }>`，把 API 调用提升到父组件（`ChatContent`）。

---

## 低（LOW）

### L1. `buildFinalMessageParts` 产出未被消费

`finalParts` 在 `message.completed` 和 `stream.done` 时正确生成并存入 state，但 hook 最终 yield 只用 `reducerState.text`，没有任何代码读取 `finalParts`。这是一个"有产出无消费"的死数据路径。

**建议**：如果不打算短期内提交结构化 parts，可以从 `ActiveTurnState` 中移除 `finalParts` 字段以减少状态体积，同时删除 `buildFinalMessageParts` 及其测试。保留的话需要加注释说明未来的消费计划。

### L2. 缺少错误路径测试覆盖

当前 `activeTurnReducer.test.ts` 缺少以下场景：
- `stream.error` → status 变为 'failed' + error 信息
- `message.failed` → status 变为 'failed'
- `state.interaction.answered` → pendingInteraction status 更新
- `source.knowledge_gap` → dedup by query
- `state.phase.changed` → effect 发射
- `conversation.created` → conversationId + runId 写入

**建议**：补 4-5 个关键错误和状态转换的测试用例。

### L3. `StreamingAssistantTurn` 的 red flag 无法手动关闭

**文件**：`StreamingAssistantTurn.tsx` 第 129 行

```ts
<RedFlagBanner
  redFlags={vm.redFlag.flags}
  onAcknowledge={() => {}}  // no-op
/>
```

历史实现 `AssistantChatPanel` 中的 `ChatContent` 使用 `onAcknowledge={() => setRedFlags(null)}` 可以关闭红旗。流式版本不能关闭，但红旗会在 turn 完成后随 active turn 重置而消失。如果用户想在流式过程中关闭红旗，当前无法做到。

**建议**：添加一个 `dismissRedFlag` action 到 `ActiveTurnActions`，或标记为已知差异。

### L4. `StreamingTurnToolCalls` 与 `ToolCallCard` 代码重复

`StreamingTurnToolCalls.tsx` 和 `ToolCallCard.tsx` 有几乎相同的渲染逻辑（TOOL_LABELS、getToolSummary、spinner/checkmark 选择）。`ToolCallCard` 有 dedup 防线（useMemo），`StreamingTurnToolCalls` 没有（因为它依赖 selector 层已完成去重）。

**建议**：提取公共渲染子组件（`ToolCallItem`），两边复用，减少维护负担。

---

## 架构层面观察

### 做得好的

1. **Record-map 天然 upsert**：`toolCallsById`、`citationsByKey`、`knowledgeGapsByKey`、`extractedInfoByBodyPart` 全部使用 `Record<id, T>`，消除了旧数组方案的 indexOf/findIndex 查找和 splice 操作。代码简洁且不易出错。

2. **Selector 层**：UI 组件不消费原始 reducer state，统一通过 selectors 得到排序/过滤后的 view model。如果有新的排序需求，只需修改 selector，不影响组件。

3. **双 context 模式**：state 和 actions 分体，使用 `useActiveTurnState()` 的组件才会在 token 级更新时 re-render，使用 `useActiveTurnActions()` 的组件不受影响（尽管 M1 指出了当前实现有小瑕疵）。

4. **后端纵深防线**：provider 层（`finish_reason_emitted` + `emitted_tool_call_ids`）+ orchestrator 层（`completed_tool_calls` dedup + `seen_tc_ids`）形成两层防护。即使 provider 层被绕过，orchestrator 层也能兜底。

5. **`processed` 标志控制 seq 更新**：未知事件不推进 `lastSeq`，确保幂等守卫不会被未知事件类型绕过。

### 需要关注的

1. **完成态的"信息损失"问题（H2）**：这是当前架构最显著的语义缺口。用户在流式中看到的 citations/red flags/gaps 在消息完成后消失，可能造成困惑——"刚才明明显示了参考知识，怎么看不到了"。

2. **双路径（hook dispatch vs context dispatch）**：同一个 `reduceActiveTurnEvent` 在两个地方被调用——hook 内部（正确消费 effects）和 context reducer（丢弃 effects）。这是 C1 的根源。如果长远来看 context 的 `dispatchEvent` 确实不需要，可以考虑删除它以消除歧义。

3. **测试偏向 happy path**：7 个 reducer 测试覆盖了核心聚合、完成、重置、中断和去重，但缺少错误处理和边界条件。考虑到 reducer 是纯函数，补充测试成本很低。

---

## 变更文件汇总

| 文件 | 变更类型 | 行数 |
|------|---------|------|
| `runtime/activeTurnReducer.ts` | 新建 | 432 |
| `runtime/activeTurnReducer.test.ts` | 新建 | 229 |
| `runtime/activeTurnSelectors.ts` | 新建 | 94 |
| `runtime/activeTurnSelectors.test.ts` | 新建 | 127 |
| `context/ActiveTurnContext.tsx` | 新建 | 111 |
| `components/StreamingAssistantTurn.tsx` | 新建 | 157 |
| `components/StreamingAssistantTurn.test.tsx` | 新建 | 126 |
| `components/StreamingTurnToolCalls.tsx` | 新建 | 67 |
| `components/ToolCallCard.tsx` | 修改 (+useMemo) | 92 |
| `components/ToolCallCard.test.tsx` | 新建 | 160 |
| `hooks/useAssistantChatRuntime.ts` | 重构 | 220 |
| `components/AssistantChatPanel.tsx` | 大幅简化 | 243 |
| `context/StreamingTextContext.tsx` | 删除 | - |
| `runtime/streamEventReducer.ts` | 删除 | - |
| `runtime/streamEventReducer.test.ts` | 删除 | - |
| `providers/openai_compatible.py` | 修改 (+dedup) | ~15 |
| `agent/orchestrator.py` | 修改 (+dedup) | ~10 |

---

## 建议修复优先级

| 优先级 | 编号 | 描述 | 预计工时 |
|--------|------|------|---------|
| P0 | H1 | lastSeq 可能被设为 undefined | 5 min |
| P0 | C1 | turnReducer 丢弃 effects（加文档/警告） | 15 min |
| P1 | H2 | 结构化事件完成态丢失（方案决策） | 2-4 h |
| P1 | M1 | Actions context 值每帧重建 | 5 min |
| P2 | M2 | tool.call fallback ID 改用 UUID | 5 min |
| P2 | L2 | 补错误路径测试 | 1 h |
| P3 | H3 | tool.result 兜底匹配加 warn 日志 | 10 min |
| P3 | M3 | StreamingAssistantTurn API 调用提升 | 30 min |
| P3 | L4 | ToolCallItem 公共组件抽取 | 1 h |
| P4 | L1 | finalParts 死代码处理 | 决策 |
| P4 | L3 | red flag 关闭按钮 | 30 min |
