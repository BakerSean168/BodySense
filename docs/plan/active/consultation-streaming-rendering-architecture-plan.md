# Consultation Streaming Rendering Architecture Plan

## 背景

当前咨询聊天页的流式渲染已经通过绕过 `assistant-ui` 的 `useSmooth` 闪烁问题获得了可用修复，但这次修复仍然存在架构层面的不稳定点：

- 流式文本存在双真值：`assistant-ui runtime message` 与 `StreamingTextContext`
- 工具调用、知识引用、红旗、知识缺口等结构化事件仍然部分依赖 `thread.messages` 反推
- `tool.call` 事件缺少幂等保护，重复到达时会导致 React `duplicate key` 报错
- 流式态与完成态的边界不清晰，后续继续叠加 `ask_user`、中断恢复、取消生成等能力时会越来越脆

本方案的目标不是继续局部修补，而是将“流式中的临时态”和“完成后的已提交消息”从架构层面分离，形成可维护、可验证、可扩展的长期方案。

## 问题陈述

### 当前现象

1. 流式文本每个 token 更新频繁，直接走 `assistant-ui` message part 更新时会触发平滑动画重置，造成闪烁。
2. 结构化事件与文本事件的生命周期不同，但当前实现仍试图让它们共享同一条渲染管线。
3. 相同 `tool_call_id` 的重复事件会被重复 append 到数组，导致：
   - 控制台 `duplicate key` 报错
   - 工具调用 UI 出现重复项
   - 中断恢复和工具结果匹配变得不可靠
4. `thread.messages` 同时承担“已提交消息存储”和“流式业务状态来源”两种职责，语义不够清晰。

### 根因

根因不是某一个组件写法不对，而是领域模型与 UI 模型没有分层：

- “当前 assistant turn 的流式状态”本质上是一个临时态聚合对象
- “历史消息”本质上是一个完成后可持久化的消息列表

当前实现把两者混在一起，导致所有问题都要靠局部 workaround 解决。

## 目标

### 主要目标

1. 流式阶段只保留一个前端真值源。
2. 所有结构化事件按业务主键幂等处理。
3. 流式态与完成态边界清晰，切换时无闪烁、无重复、无状态丢失。
4. 保留 `assistant-ui` 在线程容器、输入框、历史消息渲染方面的价值，不做无必要替换。
5. 为后续的 `ask_user` 中断恢复、工具 UI、取消生成、重放事件等能力预留清晰边界。

### 非目标

1. 本次不重写整个聊天页，不移除 `assistant-ui`。
2. 本次不修改所有既有聊天域模型，只聚焦咨询页流式链路。
3. 本次不追求将所有历史消息都迁移为全新的消息存储结构。

## 设计原则

1. 单一真值源：同一时刻，同一类状态只能有一个 authoritative source。
2. 临时态与提交态分离：streaming state 不伪装成 committed message。
3. 事件幂等：前端 reducer 和后端事件发射都必须对重复事件安全。
4. 渲染模型前置：先定义 UI 需要什么状态，再定义 reducer 和事件聚合方式。
5. 渐进迁移：优先替换最脆弱的边界，避免一次性重写。

## 目标架构

### 总体思路

引入一个“当前 turn 流式投影层”，将本轮 assistant 回复在完成前的所有状态统一聚合为 `ActiveTurnState`。UI 在流式阶段直接渲染该状态；只有当 turn 完成后，才将最终生成的消息内容提交给 `assistant-ui runtime` 作为历史消息的一部分。

### 分层模型

```text
SSE stream
  -> ConsultationStreamReducer
  -> ActiveTurnState
  -> ActiveTurnViewModel
  -> Streaming UI

turn completed
  -> FinalMessageParts
  -> assistant-ui runtime / persisted history
  -> historical message rendering
```

### 核心思想

- `ActiveTurnState` 是流式阶段唯一真值
- `thread.messages` 只表示已提交、可回放、可持久化的历史消息
- 流式中的 tool calls、citations、knowledge gaps、red flags、pending interaction 不再从 `thread.messages` 反推
- 最终切换到完成态时，使用同一份 reducer state 生成最终 `parts`

## 数据模型设计

### ActiveTurnState

建议引入如下状态模型：

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

### View Model 约定

UI 不直接消费原始 reducer 字段，而是消费 selector 输出：

```ts
interface ActiveTurnViewModel {
  streamingMarkdown: string;
  toolCalls: ToolCallVM[];
  citations: CitationVM[];
  knowledgeGaps: KnowledgeGapVM[];
  redFlag: RedFlagEvent | null;
  pendingInteraction: PendingInteraction | null;
  isRunning: boolean;
  isInterrupted: boolean;
  hasVisibleContent: boolean;
}
```

这样可以避免组件内部重复做排序、去重和过滤逻辑。

## 事件处理设计

### 事件幂等规则

所有 SSE 事件在 reducer 层统一做幂等保护。

#### 通用规则

1. 如果事件带 `seq`，当 `seq <= lastSeq` 时直接忽略。
2. 如果同一个业务主键再次到达，执行 upsert，而不是 append。
3. 未知事件类型保持 no-op，不污染状态。

#### 分事件规则

`message.text.delta`
- 仅在 `seq` 新增时追加 `delta`
- 更新 `text`

`tool.call`
- 主键：`tool_call_id`
- 行为：upsert 为 `running`
- 如果已存在且字段相同，返回原状态

`tool.result`
- 主键：`tool_call_id`
- 行为：将对应工具状态标记为 `completed`
- 如果缺少 `tool.call` 前置事件，则允许按 `tool_call_id` 补建一个占位项

`source.citation.added`
- 主键：优先 `title`，必要时扩展为 `title + source_title`
- 行为：upsert

`source.knowledge_gap`
- 主键：`query`
- 行为：upsert

`state.extracted_info.upsert`
- 主键：`body_part`
- 行为：upsert merge

`state.interaction.required`
- 主键：`interaction_id`，兜底 `tool_call_id`
- 行为：upsert pending interaction，并将状态置为 `interrupted`

`message.completed`
- 将 `status` 标记为 `completed`
- 根据当前聚合状态生成 `finalParts`

`stream.error`
- 将 `status` 标记为 `failed`
- 保留当前已知状态，便于 UI 提示和调试

## 组件边界设计

### 保留 assistant-ui 的部分

继续使用 `assistant-ui` 处理：

- Thread 容器
- Composer / send 交互
- 已完成历史消息渲染
- 线程级 message 排列和基础可访问性

### 从 assistant-ui 解耦的部分

从 runtime message 派生中移出：

- 当前流式文本显示
- 工具调用进度卡片
- citation 列表
- knowledge gap 提示
- red flag 展示
- `ask_user` 中断卡片

这些 UI 统一改为读取 `ActiveTurnViewModel`。

### 渲染策略

#### 历史消息

历史消息使用 `assistant-ui` 的 `MessagePrimitive.Parts` 渲染，避免继续依赖旧式 `MessagePrimitive.Content` 入口。

#### 流式消息

当 `ActiveTurnState.status === 'streaming' | 'interrupted'` 时：

- 直接渲染 `ActiveTurnViewModel.streamingMarkdown`
- 同屏附加渲染当前 turn 的 tool calls / citations / knowledge gaps / red flags / interaction

#### 完成切换

当 reducer 收到完成事件后：

1. 生成最终 `finalParts`
2. 提交给 runtime / 本地消息状态
3. 清空 active turn
4. 页面开始将这条消息视为历史消息渲染

切换过程不再依赖“library message 是否有内容”这种隐式条件。

## 模块拆分建议

### 新增模块

建议新增以下文件：

- `apps/web/src/features/consultation/runtime/activeTurnReducer.ts`
- `apps/web/src/features/consultation/runtime/activeTurnSelectors.ts`
- `apps/web/src/features/consultation/context/ActiveTurnContext.tsx`
- `apps/web/src/features/consultation/components/StreamingAssistantTurn.tsx`
- `apps/web/src/features/consultation/components/StreamingTurnToolCalls.tsx`

### 既有模块调整

`useAssistantChatRuntime.ts`
- 不再只维护文本 context 更新
- 改为将完整 SSE 事件 dispatch 到 active turn reducer
- 完成后提交 final message

`AssistantChatPanel.tsx`
- 删除 citation / red flag / gap 从 `thread.messages` 反推的 effect
- 渲染历史消息和 active turn 两层内容

`streamEventReducer.ts`
- 若保留，需明确职责
- 更推荐直接演进为 active turn reducer，而不是继续维护一个“半业务、半适配”的中间层

`ToolCallCard.tsx`
- 输入改为已经排序去重的 view model
- 不再直接消费原始 `toolCalls: ToolCallInfo[]`

## 后端配合改造

前端必须具备幂等能力，但后端也应减少重复事件。

### AI provider 层

`openai_compatible.py`

需要确认并修复以下风险：

- 在一个流式响应生命周期内，若 `finish_reason` 被观察到多次，不能重复发射相同 `tool_call_done`
- 发射完成后应清理或冻结 accumulator，避免同一工具调用被重复转换

### orchestrator 层

`orchestrator.py`

需要补充：

- `completed_tool_calls` 按 `tool_call_id` 去重
- 向前端写出 `tool_call` 事件前再次校验是否已写出
- 如果模型层出现重复 `tool_call_done`，orchestrator 也不能把重复直接扩散出去

### 契约层

若条件允许，建议在文档中明确：

- `tool.call` 对同一 `(run_id, tool_call_id)` 组合至多语义生效一次
- `tool.result` 可以重放，但消费者必须按主键覆盖

## 迁移计划

### Phase 1: 稳定性止血

目标：先消除重复 key 和流式状态分叉最严重的问题。

任务：

1. 将 `tool.call` 改为按 `tool_call_id` upsert
2. 为 `tool.result` 增加无前置 call 时的占位补建
3. 为 reducer 增加 `seq` 去重
4. 补单元测试覆盖重复 `tool.call`、重复 `tool.result`、乱序 `seq`

完成标准：

- 控制台不再出现 `duplicate key`
- 工具调用 UI 不再重复

### Phase 2: 建立 ActiveTurnState

目标：用完整 active turn 状态替代仅文本 context。

任务：

1. 新建 `ActiveTurnContext`
2. 将 streaming text、tool calls、pending interaction 等统一挂到 active turn
3. 让流式 UI 改读 active turn selectors

完成标准：

- 流式 UI 不再依赖 `thread.messages` 派生业务状态

### Phase 3: 提交态切换重构

目标：形成“流式态 -> 完成态”的正式切换机制。

任务：

1. 在 reducer 完成时生成 `finalParts`
2. runtime 在完成时一次性提交最终消息
3. 历史消息统一走 `MessagePrimitive.Parts`
4. 删除旧的 `StreamingTextContext`

完成标准：

- 流式中无闪烁
- 完成切换无闪屏、无双渲染、无丢状态

### Phase 4: 后端幂等收口

目标：让上游事件语义本身更稳定。

任务：

1. provider 层去重 `tool_call_done`
2. orchestrator 层去重 `tool_call`
3. 补端到端回归测试

完成标准：

- 即使前端移除额外防御，事件语义仍然稳定

## 测试与验证

### 单元测试

前端 reducer 测试至少覆盖：

1. 连续 text delta 聚合
2. 重复 `tool.call` 不新增重复项
3. `tool.result` 覆盖已有项
4. `tool.result` 无前置 `tool.call` 时补建占位
5. citation 去重
6. knowledge gap 去重
7. interaction required 中断状态切换
8. `message.completed` 生成 `finalParts`
9. 旧 `seq` 事件被忽略

### 组件测试

1. 流式阶段显示 markdown 文本
2. 流式阶段工具调用可更新为 completed
3. 中断阶段显示 `AskUserCard`
4. 完成后从 active turn 切换到历史消息显示

### 集成测试

建议增加一个基于 mock SSE 的集成测试，模拟：

1. 文本流 + 工具调用 + citation + 完成
2. 工具调用重复发射
3. `ask_user` 中断 + resume

### 手工验收

在本地页面验证以下场景：

1. 普通纯文本回复
2. 搜索知识库工具调用
3. 症状提取工具调用
4. `ask_user` 中断问题
5. 模拟重复工具事件
6. 弱网下长文本流式回复

## 风险与权衡

### 风险

1. ActiveTurnState 引入后，短期内状态层会比现在更明确，但代码行数会增加。
2. 若 runtime 提交最终消息的方式与 `assistant-ui` 当前本地 runtime 能力不完全契合，可能需要再做一层适配。
3. 若后端事件顺序在现实中并不稳定，前端 reducer 必须比现在更严格地处理乱序和缺失事件。

### 权衡

本方案选择“显式建模流式 turn”而不是继续局部修补，代价是增加一层状态模型；收益是把文本流、工具流和交互中断统一纳入同一个领域边界，后续扩展成本显著下降。

## 开放问题

1. 完成态消息最终是直接提交给 `assistant-ui runtime`，还是优先提交到页面本地消息状态后再作为 `initialMessages` 重建？
2. `assistant-ui` 当前 local runtime 是否提供稳定的“追加最终 assistant message”接口，还是需要在 adapter 层继续 yield 一次最终消息作为提交触发？
3. citation / knowledge gap / red flag 是否应最终落盘为 message parts，还是只作为临时 UI 元数据存在？
4. `ask_user` 恢复后是否仍视为同一个 active turn，还是视为新的 turn continuation？

## 推荐决策

在当前代码基础上，推荐采用以下决策：

1. 前端立即实施幂等 reducer 和 `ActiveTurnState`
2. 历史消息渲染逐步迁移到 `MessagePrimitive.Parts`
3. 后端在下一阶段补足 `tool_call_id` 去重
4. 将 `thread.messages` 限定为“完成态历史消息”，不再承担流式业务状态来源

## 实施结果预期

完成本方案后，咨询聊天页应具备以下性质：

1. 流式阶段无闪烁
2. 工具调用重复到达时无重复 UI、无控制台报错
3. citation / red flag / knowledge gap / interaction 全部从统一流式状态读取
4. 完成态切换路径明确且稳定
5. 后续新增流式能力时无需继续堆叠 workaround

## Implementation Checklist

本节将方案拆成可以直接执行的小步实现清单。建议每个小步都保持可独立验证，并优先按 Phase 顺序推进。

## Phase 1 Checklist: 幂等止血与最小稳定化

### 目标

在不引入完整 `ActiveTurnState` 之前，先消除重复 key、重复工具项和事件重放带来的最明显风险。

### 文件改动清单

#### `apps/web/src/features/consultation/runtime/streamEventReducer.ts`

- [ ] 为 reducer state 增加 `lastSeq` 字段
- [ ] 在 `reduceStreamEvent()` 顶部增加 `seq` 幂等判断
- [ ] 将 `tool.call` 从 append 改为 upsert
- [ ] 将 `tool.result` 改为按 `tool_call_id` 精确匹配优先、按 tool name 兜底匹配其次
- [ ] 当 `tool.result` 找不到已有项时，补建一个占位工具项再标记为 completed
- [ ] 为 `source.knowledge_gap` 增加按 `query` 去重
- [ ] 为 `source.citation.added` 保持或加强按主键去重
- [ ] 为 `state.interaction.required` 保持幂等，避免同一 interaction 重复覆盖造成多次 UI 抖动

建议新增/调整的内部 helper：

- [ ] `upsertToolCall(existing, incoming)`
- [ ] `completeToolCall(existing, toolCallId, toolName, result)`
- [ ] `dedupKnowledgeGap(existing, incoming)`

#### `apps/web/src/features/consultation/components/ToolCallCard.tsx`

- [ ] 在渲染前增加显式去重保护，作为 reducer 修复之外的 UI 防线
- [ ] 明确排序规则：`running` 在前，completed 在后，保持原始出现顺序
- [ ] 保证 `key` 只基于稳定主键，不再依赖数组位置

说明：

- 这里的去重只是第二道防线，不能替代 reducer 幂等

#### `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`

- [ ] 若 reducer state 新增 `lastSeq` 等字段，完成类型适配
- [ ] 保证 `onToolCallUpdate` 始终拿到的是 reducer 最新状态，而不是未归一化的 event append 结果

### 测试清单

#### 更新 `apps/web/src/features/consultation/runtime/streamEventReducer.test.ts`

新增测试用例：

- [ ] `ignores event with seq older than lastSeq`
- [ ] `upserts duplicate tool.call by tool_call_id`
- [ ] `marks existing tool call completed on tool.result`
- [ ] `creates placeholder tool call when tool.result arrives first`
- [ ] `deduplicates knowledge gaps by query`
- [ ] `keeps citation deduplication stable after repeated events`

建议继续保留并对齐的既有测试：

- [ ] `message.text.delta accumulates`
- [ ] `state.interaction.required sets pending interaction`

#### 建议新增 `apps/web/src/features/consultation/components/__tests__/ToolCallCard.test.tsx`

新增组件测试：

- [ ] 相同 `tool_call_id` 出现两次时只渲染一次
- [ ] running 状态显示转圈，completed 状态显示勾选
- [ ] `ask_user` 工具不显示在工具卡片中
- [ ] 同 summary 文本但不同 id 的工具项可并存

### 验证命令

- [ ] `pnpm nx run web:test -- --runInBand streamEventReducer`
- [ ] `pnpm nx run web:test -- --runInBand ToolCallCard`
- [ ] `pnpm nx run web:typecheck`

### 建议提交切片

1. `fix(web): make consultation tool-call reducer idempotent`
2. `test(web): cover duplicate tool events in consultation chat`

## Phase 2 Checklist: 建立 ActiveTurnState 基础设施

### 目标

用完整 active turn 状态替代单一 `StreamingTextContext`，把流式阶段的一切 UI 数据源统一起来。

### 新增文件清单

#### `apps/web/src/features/consultation/runtime/activeTurnReducer.ts`

- [ ] 定义 `ActiveTurnState`
- [ ] 定义 `ActiveTurnAction` 或复用 `StreamEvent`
- [ ] 实现 `reduceActiveTurnEvent()`
- [ ] 提供 `INITIAL_ACTIVE_TURN_STATE`
- [ ] 提供 `resetActiveTurnState()`
- [ ] 在 `message.completed` 时生成 `finalParts`

建议包含的 helper：

- [ ] `appendStreamingText()`
- [ ] `upsertToolCallVM()`
- [ ] `upsertCitationVM()`
- [ ] `upsertKnowledgeGapVM()`
- [ ] `upsertExtractedInfoVM()`
- [ ] `buildFinalMessageParts(state)`

#### `apps/web/src/features/consultation/runtime/activeTurnSelectors.ts`

- [ ] 实现 `selectActiveTurnViewModel(state)`
- [ ] 实现 `selectVisibleToolCalls(state)`
- [ ] 实现 `selectStreamingMarkdown(state)`
- [ ] 实现 `selectIsInterrupted(state)`
- [ ] 实现 `selectHasVisibleContent(state)`

#### `apps/web/src/features/consultation/context/ActiveTurnContext.tsx`

- [ ] 提供 `ActiveTurnStateContext`
- [ ] 提供 `ActiveTurnActionsContext`
- [ ] 暴露 `dispatchEvent`, `resetTurn`, `hydrateTurn` 等稳定 action
- [ ] 保持 state / actions 双 context 结构，避免全树重复 re-render

#### `apps/web/src/features/consultation/components/StreamingAssistantTurn.tsx`

- [ ] 渲染流式 markdown 文本
- [ ] 渲染工具调用列表
- [ ] 渲染知识库引用
- [ ] 渲染 knowledge gap
- [ ] 渲染 red flag
- [ ] 渲染 `AskUserCard`

#### `apps/web/src/features/consultation/components/StreamingTurnToolCalls.tsx`

- [ ] 从 `ToolCallCard` 中抽离纯“当前 turn 工具进度”显示
- [ ] 只接收 selector 归一化后的 view model

### 既有文件改动清单

#### `apps/web/src/features/consultation/context/StreamingTextContext.tsx`

- [ ] 标记为待删除或临时兼容层
- [ ] 如保留过渡期，文档中注明仅供迁移阶段使用

#### `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`

- [ ] 将 `onStreamingTextUpdate` 替换为 `onStreamEvent` 或 `dispatchActiveTurnEvent`
- [ ] 在 `applyEffects()` 中不再只更新文本，而是更新完整 active turn
- [ ] 继续保留完成态最终 yield，直到提交机制完全迁移完成

#### `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`

- [ ] 用 `ActiveTurnProvider` 包裹聊天区域
- [ ] 移除 `streamingTextRef` bridge
- [ ] 移除 `StreamingTextBridge`
- [ ] 将 `toolCalls`、`pendingInteraction` 等本地状态迁移到 active turn context

### 测试清单

#### 新增 `apps/web/src/features/consultation/runtime/activeTurnReducer.test.ts`

新增测试：

- [ ] `aggregates text, tools, citations, and interaction into one state`
- [ ] `builds finalParts on message.completed`
- [ ] `resets state after completion`
- [ ] `keeps interrupted state on interaction.required`
- [ ] `preserves prior text when interaction event arrives`

#### 新增 `apps/web/src/features/consultation/runtime/activeTurnSelectors.test.ts`

新增测试：

- [ ] `selectVisibleToolCalls filters ask_user`
- [ ] `selectActiveTurnViewModel sorts and normalizes tool calls`
- [ ] `selectHasVisibleContent returns true for text-only and tool-only states`

#### 新增 `apps/web/src/features/consultation/components/__tests__/StreamingAssistantTurn.test.tsx`

新增组件测试：

- [ ] 流式 markdown 文本正确显示
- [ ] tool calls 会随 state 更新
- [ ] interruption 时显示 `AskUserCard`
- [ ] 没有内容时显示加载态

### 验证命令

- [ ] `pnpm nx run web:test -- --runInBand activeTurnReducer`
- [ ] `pnpm nx run web:test -- --runInBand activeTurnSelectors`
- [ ] `pnpm nx run web:test -- --runInBand StreamingAssistantTurn`
- [ ] `pnpm nx run web:typecheck`

### 建议提交切片

1. `refactor(web): introduce consultation active turn reducer`
2. `feat(web): render consultation streaming turn from active turn state`

## Phase 3 Checklist: UI 数据源切换与完成态提交流程

### 目标

让聊天页不再从 `thread.messages` 派生流式业务状态，并完成从流式态到历史消息的正式切换。

### 文件改动清单

#### `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`

- [ ] 删除 citation / red flag / knowledge gap 从 `thread.messages` 提取的 `useEffect`
- [ ] 删除依赖 `message.status === running && message.content empty` 的隐式流式判断
- [ ] 将当前流式区渲染改为单独插入 `StreamingAssistantTurn`
- [ ] 历史消息渲染部分统一切到 `MessagePrimitive.Parts`
- [ ] 为用户消息和 assistant 历史消息分别定义稳定 render path

建议拆分子组件：

- [ ] `HistoricalAssistantMessage`
- [ ] `HistoricalUserMessage`
- [ ] `StreamingAssistantTurn`

#### `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`

- [ ] 定义“完成态提交点”
- [ ] 在 reducer 收到 `message.completed` 或 `stream.done` 后生成最终 `content`
- [ ] 将最终 `content` 一次性交给 runtime
- [ ] 完成提交后触发 `resetTurn`

#### `apps/web/src/features/consultation/pages/ConsultationPage.tsx`

- [ ] 确认 `initialMessages` 仅代表历史消息
- [ ] 不再假设页面级状态要感知流式工具调用细节
- [ ] 仅保留会话级元数据更新回调，例如 extracted info / phase / title

#### `apps/web/src/features/consultation/index.ts`

- [ ] 导出新增的 active turn 相关模块（若需要给测试或页面使用）

### 测试清单

#### 更新 `apps/web/src/features/consultation/components/__tests__/AssistantChatPanel.test.tsx`

若当前不存在，建议新增：

- [ ] 流式中显示 `StreamingAssistantTurn`，完成后切换为历史消息
- [ ] citation / red flag / gap 不再依赖 `thread.messages`
- [ ] 完成切换后 active turn 被清空
- [ ] 历史 assistant message 使用 markdown parts 渲染

#### 建议新增集成测试 `apps/web/src/features/consultation/components/__tests__/AssistantChatPanel.integration.test.tsx`

模拟一轮完整交互：

- [ ] 用户发消息
- [ ] 收到 text delta
- [ ] 收到 tool.call
- [ ] 收到 tool.result
- [ ] 收到 message.completed
- [ ] UI 从流式态平滑切换到历史态

### 验证命令

- [ ] `pnpm nx run web:test -- --runInBand AssistantChatPanel`
- [ ] `pnpm nx run web:test -- --runInBand consultation`
- [ ] `pnpm nx run web:lint`
- [ ] `pnpm nx run web:typecheck`

### 建议提交切片

1. `refactor(web): decouple consultation streaming ui from thread.messages`
2. `refactor(web): render consultation history with assistant-ui parts`

## Phase 4 Checklist: 后端事件幂等收口

### 目标

让重复事件不只是在前端被防御，而是在上游就尽量不产生。

### 文件改动清单

#### `apps/ai-service/src/ai/providers/openai_compatible.py`

- [ ] 明确 `tool_call_accumulators` 生命周期
- [ ] 在发射完 `tool_call_done` 后清理已发射条目，避免 finish reason 多次到达时重复发射
- [ ] 如底层 provider 可能重复给出相同 finish chunk，增加 `emitted_tool_call_ids` 集合防重
- [ ] 为异常 JSON 参数解析保留现有 warning 行为

建议新增内部状态：

- [ ] `emitted_tool_call_ids: set[str]`

#### `apps/ai-service/src/services/agent/orchestrator.py`

- [ ] `completed_tool_calls` 改为按 `tool_call_id` 去重聚合
- [ ] `_handle_tool_calls()` 前确认同一轮不会重复写出 `tool_call`
- [ ] 若上游重复给出相同 tool call，orchestrator 只消费一次

建议新增 helper：

- [ ] `_dedupe_completed_tool_calls(completed_tool_calls)`

#### 如需要补契约说明

- [ ] 在 `packages/contracts/src/stream-events.ts` 附近补充注释，说明 `tool_call_id` 的幂等语义

### 测试清单

#### 新增或更新 `apps/ai-service/tests/unit/test_chat_service.py`

- [ ] 同一 tool call 在重复 provider finish 下只映射一次 `tool.call`
- [ ] `tool.result` 仍正常发射

#### 新增或更新 `apps/ai-service/tests/unit/test_consultation_graph.py`

- [ ] 重复 `tool_call_done` 不会导致重复 writer 事件

#### 可选新增 `apps/ai-service/tests/unit/test_openai_compatible.py`

- [ ] provider 在重复 finish chunk 时不重复 yield `tool_call_done`

### 验证命令

- [ ] `cd apps/ai-service && uv run pytest tests/unit/test_chat_service.py`
- [ ] `cd apps/ai-service && uv run pytest tests/unit/test_consultation_graph.py`
- [ ] `cd apps/ai-service && uv run ruff check .`

### 建议提交切片

1. `fix(ai): dedupe tool call done events in provider stream`
2. `fix(ai): prevent duplicate consultation tool events in orchestrator`

## Cross-Cutting Checklist: 文档、回归与删除旧实现

### 删除/收尾项

- [ ] 删除 `StreamingTextContext.tsx`，前提是 active turn 方案已完全接管
- [ ] 删除 `AssistantChatPanel.tsx` 中与 `streamingTextRef` 相关的桥接逻辑
- [ ] 删除不再使用的 `toolCalls` 本地 state
- [ ] 删除不再使用的 `pendingInteraction` 本地 state，前提是已迁移到 active turn
- [ ] 删除从 `thread.messages` 派生 citation / red flag / gap 的旧 effect

### 文档同步

- [ ] 在本方案文档中标记已完成 Phase
- [ ] 若最终方案与本文档存在偏差，回填“最终设计决策”一节

### 最终回归清单

- [ ] 新咨询草稿第一轮消息
- [ ] 已存在会话继续发送消息
- [ ] 工具调用与知识引用同轮出现
- [ ] `ask_user` 中断并恢复
- [ ] 重复 `tool.call` 不再报错
- [ ] 弱网长文本无闪烁

## 推荐执行顺序

如果按最小风险推进，建议顺序如下：

1. 先完成 Phase 1，尽快止血重复事件问题
2. 再完成 Phase 2，建立 active turn 基础设施
3. 然后完成 Phase 3，切换 UI 真值源
4. 最后完成 Phase 4，收口后端幂等语义

## 推荐 Definition of Done

当以下条件全部成立时，可以认为本方案完成：

- [ ] 前端 reducer 与组件测试全部通过
- [ ] 后端重复工具事件测试全部通过
- [ ] `StreamingTextContext` 已删除
- [ ] 聊天页流式 UI 不再读取 `thread.messages` 作为业务状态源
- [ ] 历史消息渲染统一为 `MessagePrimitive.Parts`
- [ ] 手工验收场景全部通过
