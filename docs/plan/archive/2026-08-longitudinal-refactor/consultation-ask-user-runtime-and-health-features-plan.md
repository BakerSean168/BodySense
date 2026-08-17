# Consultation AskUser Runtime And Health Features Plan

> Scope boundary update — 2026-08-15  
> 本计划继续负责 Consultation collection / ask_user HITL / 现有 `health_features` 工作台的近期改进。长期业务模型已切换到 [Longitudinal BodyState](../../architecture/longitudinal-body-state-domain.md)：`health_features` 现在应视为 BodyState 的迁移前身 / projection adapter，而不是最终领域 schema。Diagnosis、Treatment 与长期状态迁移统一由 [`longitudinal-body-state-migration-plan.md`](./longitudinal-body-state-migration-plan.md) 管理。实施时继续保护现有 consultation-runs / SSE / projection / interrupt-resume 链，不要把 runtime 重写与 BodyState 迁移混成一次不可验证的大改。

## 背景

当前咨询链路里，`ask_user` 已经不只是一个前端渲染问题，而是暴露了 3 个相互关联的设计缺口：

1. `ask_user` 触发过于激进，很多场景其实不需要阻塞式追问。
2. `ask_user` 虽然已经具备部分 runtime 事件和 resume 机制，但仍未被完整建模为一个端到端的中断流程。
3. 右侧结构化信息面板仍以 `extracted_info` / 症状 schema 为中心，无法稳定沉淀体态观察、否定回答、追问回答等健康特征。

这三个问题不能分开零修。若只修前端，交互会继续突兀；若只改 prompt，runtime 仍会混乱；若只扩 schema，又会缺少稳定事件来源。

本计划目标是把这三点统一成一条清晰链路：

```txt
模型决策 ask_user 是否必要
  -> runtime 以 interrupt 建模 ask_user
  -> 前端以追问卡片呈现 ask_user
  -> ask_user answer 进入统一历史与特征提取
  -> 健康特征面板沉淀 posture / symptom / negative finding / user answer
```

## 目标

### 主要目标

1. 降低不必要的阻塞式 `ask_user` 调用，只在“没有该信息就无法可靠继续”时中断。
2. 将 `ask_user` 视为完整 runtime interrupt，而不是普通消息或普通 tool call。
3. 让 `ask_user` 的 question / answer 成为统一会话历史和结构化健康特征的一部分。
4. 将右侧面板从单一“症状提取”扩展为更通用的“健康特征沉淀”。

### 非目标

1. 本次不引入新的复杂 HITL 类型（如 confirm_action、request_upload）。
2. 本次不做完整的多问题并行问诊图谱。
3. 本次不做历史数据迁移兼容层，按 MVP 直接升级当前 schema。

## 问题拆解

### 问题一：`ask_user` 调用策略过于激进

当前现象：

- 用户只是表达“疑似头前移”这类可先回答的观察，模型仍直接中断追问。
- 追问缺少 `why this matters` 的上下文，用户感知是“回答被打断”。

根因：

- `ask_user` 的工具语义定义过宽。
- orchestration / prompt 中缺少“何时必须阻塞、何时应该先回答再补问”的硬约束。

目标状态：

- `ask_user` 只用于关键缺失信息。
- 非关键补充信息优先走“先回答 + 文本追加追问”。
- 每个 `ask_user` 都必须携带简短 context，解释为什么问。

### 问题二：`ask_user` 的 runtime 模型还不完整

当前现象：

- assistant 空壳消息、tool call、interaction card 之间边界不稳定。
- answered interaction 只体现在局部 UI，不一定进入统一事件/历史模型。
- page refresh / replay / projection 仍然以“消息”和“事件”双轨拼接。

根因：

- `ask_user` 还没有被定义成一等 runtime 生命周期对象。
- 工具调用、interaction、message、projection 之间的职责边界尚未完全收紧。

目标状态：

```txt
pending -> answered -> resumed -> completed
```

- 前后端对该生命周期使用同一套字段和状态转移。
- interaction question / answer / tool_call_id / run_id / message_id 均可回放。
- answered interaction 能稳定显示在时间线和 projection 中。

### 问题三：健康特征 schema 太窄

当前现象：

- 右侧仍是 `提取的症状信息`，更像 `symptom list`。
- `头前移` 这类体态观察无法自然表达。
- `无颈肩不适` 这类否定回答没有合适结构位。
- `ask_user` answer 没有稳定进入右侧面板。

根因：

- `ConsultationSession.ExtractedInfo` 仍使用 `body_part / symptom_type / duration / trigger` 的症状 schema。
- 该 schema 无法覆盖 posture finding、negative finding、supplemental answer。

目标状态（近期迁移层）：

- 右侧面板升级为更通用的健康特征面板，并作为未来 BodyState Workbench 的过渡 projection。
- 当前 `health_features` schema 暂时承载：
  - posture findings
  - symptoms / discomforts
  - negative findings
  - red flags
  - movement limitations
  - ask_user answers
- 后续不要继续无限扩张 `health_features` JSON；Fact / Observation / Hypothesis / Evidence 与时间语义的最终边界以 Longitudinal BodyState Domain Model 为准。

## 设计原则

1. 普通消息、工具调用、interaction、健康特征四者分层，不能互相冒充。
2. `ask_user` 是 runtime interrupt，不是普通 assistant 文本。
3. 用户选择是 resume input，不是新的 user chat message。
4. 结构化特征必须来自统一事件源，不能依赖 UI 推断。
5. schema 以“健康特征”而不是“症状”命名，避免天然遗漏体态问题。

## 总体方案

### Phase 1：收紧 `ask_user` 触发策略

目标：让模型只在必要时中断。

改动层：

- `apps/ai-service/src/services/agent/tools/ask_user.py`
- consultation prompt / agent workflow / next-action decision logic
- 相关 unit tests / golden cases

实施内容：

1. 明确定义 `ask_user` 触发门槛：
   - 必须追问：红旗、诊断关键缺失、部位不清、严重程度不清、持续时间不清、麻木/放射痛/外伤等高价值信息
   - 非必须追问：姿态观察、一般补充偏好、可选上下文
2. 在 prompt 中加入 hard rule：
   - 能先给初步建议时，不要立即 `ask_user`
   - 若调用 `ask_user`，必须提供 `context`
3. 为 posture 类 case 增加回归样例：
   - “我感觉头有点前移”应先回答基础自测，再追加可选追问

交付标准：

- posture / mild observation 类 case 不再默认阻塞。
- `ask_user` payload 默认包含 context 或 reason。

### Phase 2：补全 `ask_user` runtime 生命周期

目标：让 interrupt 成为一等 runtime 实体。

改动层：

- `apps/ai-service/src/runtime/consultation_thread.py`
- `apps/api/internal/consultation/runtime.go`
- `apps/api/internal/service/agent_interaction_service.go`
- `apps/api/internal/service/thread_projection_service.go`
- `apps/web/src/features/consultation/runtime/*`
- `apps/web/src/features/consultation/components/*`

实施内容：

1. 统一 interaction lifecycle：
   - `pending`
   - `answered`
   - `resumed`
   - `completed` 或 `cancelled`
2. interaction answered 事件补齐 answer payload，并明确何时清理 pending interaction。
3. thread projection 中显式持有 interaction timeline，而不只是 pending list。
4. 前端 active turn / historical message / projection replay 使用一致的 interaction view model。
5. answered interaction 应能在刷新后恢复显示“问题 + 回答”。

建议的数据演进：

```txt
pending_interactions     -> 当前阻塞中的 interaction
interaction_history      -> 当前会话中的已回答/已完成 interaction 摘要
```

交付标准：

- 刷新页面后，已回答的 `ask_user` 仍能在历史时间线中表达。
- `answer` 不再只存在于隐藏 user message metadata 或局部状态中。
- projection 回放时不再需要通过消息内容猜 interaction answer。

### Phase 3：升级结构化健康特征 schema

目标：让“头前移”“无颈肩不适”“追问回答”都能稳定沉淀。

改动层：

- `apps/api/internal/model/consultation_session.go`
- consultation DTO / handler / service
- thread projection read model
- AI service extracted info / tool result mapping
- web types + right panel UI

建议 schema：

```ts
health_features: {
  posture_findings: Array<{
    key: string;
    label: string;
    status?: 'suspected' | 'reported' | 'confirmed';
    source: 'user_message' | 'ask_user' | 'assistant_inference';
    notes?: string;
  }>;
  discomforts: Array<{
    body_part: string;
    symptom_type?: string;
    severity?: string;
    duration?: string;
    trigger?: string;
    relief?: string;
    source: 'user_message' | 'ask_user';
  }>;
  negative_findings: Array<{
    key: string;
    label: string;
    value: string;
    source: 'ask_user' | 'user_message';
  }>;
  movement_limitations: Array<...>;
  red_flags: Array<...>;
  user_answers: Array<{
    interaction_id: string;
    question: string;
    answer_text: string;
    source: 'ask_user';
  }>;
}
```

兼容策略：

- 代码层仍可保留 `extracted_info` 字段一段时间，但前端主面板应转向 `health_features`。
- `extract_symptom_info` 工具的结果映射到 `discomforts`。
- `ask_user` answer 的解析结果映射到 `negative_findings` / `user_answers` / `posture_findings`。

交付标准：

- “头前移”能进入 posture findings。
- “无颈肩不适”能进入 negative findings。
- `ask_user` answer 能进入 user_answers。
- 右侧面板文案从“提取的症状信息”升级为“健康特征”。

## 实施顺序

按依赖关系，建议顺序如下：

1. Phase 1：先收紧 `ask_user` 触发策略  
   原因：如果继续过度触发，中断链路再优雅也会打断过多。

2. Phase 2：再补齐 runtime lifecycle  
   原因：健康特征沉淀需要可靠的 interaction question / answer 事件来源。

3. Phase 3：最后升级健康特征 schema  
   原因：schema 需要依赖前面明确下来的 interaction 和 extraction 语义。

## 影响文件清单

### AI Service

- `apps/ai-service/src/services/agent/tools/ask_user.py`
- `apps/ai-service/src/runtime/consultation_thread.py`
- `apps/ai-service/src/services/agent_workflow.py`
- consultation prompt / tests / golden cases

### API

- `apps/api/internal/consultation/runtime.go`
- `apps/api/internal/model/consultation_session.go`
- `apps/api/internal/model/thread_projection.go`
- `apps/api/internal/service/thread_projection_service.go`
- consultation DTO / handler / service
- migrations for schema expansion

### Web

- `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`
- `apps/web/src/features/consultation/components/StreamingAssistantTurn.tsx`
- `apps/web/src/features/consultation/components/AskUserCard.tsx`
- `apps/web/src/features/consultation/components/InfoPanel.tsx`
- `apps/web/src/features/consultation/runtime/*`
- `apps/web/src/features/consultation/types/consultation.ts`

## 风险

1. `ask_user` 调用策略收紧后，部分当前测试用例会改变预期输出。
2. interaction lifecycle 扩展会影响 projection / replay / refresh 行为，需要端到端回归。
3. schema 升级会触及 API、前端、AI 三端合同，必须同步修改，不能只改一侧。

## 验证策略

### AI / Prompt

- posture observation golden case
- red flag / critical missing info golden case
- ask_user payload context coverage

### API / Runtime

- interrupt -> answer -> resume -> complete 全链路测试
- refresh after answered interaction
- projection rebuild consistency

### Web

- active turn interaction rendering
- answered interaction replay rendering
- right panel health feature rendering
- regression: no empty assistant bubble

## 验收标准

- [ ] 非关键姿态观察问题不再默认触发阻塞式 `ask_user`
- [ ] `ask_user` question 带有用户可理解的 context
- [ ] interaction answer 能在 runtime / projection / timeline 中稳定回放
- [ ] 刷新页面后，追问问题和回答不会丢失
- [ ] 右侧面板能展示 posture findings、negative findings、user answers
- [ ] 不再出现纯 tool-call assistant 空气泡

## 建议实施批次

建议拆成 3 个连续批次提交：

1. `Phase A`: ask_user decision policy tightening
2. `Phase B`: interaction lifecycle + projection replay
3. `Phase C`: health_features schema + right panel migration

这样每批都能单独验证，不会把 prompt、runtime、schema 三类问题混成一次难以回滚的大改。
