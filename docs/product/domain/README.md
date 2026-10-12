# BodySense · 领域模型总览

此目录是 [产品规格](../README.md) 的领域契约权威来源，定义目标逻辑模型，不意味着代码已经实现。

| 领域 | 权威文档 |
| --- | --- |
| BodyState、BodyFact、SymptomEpisode、BodyObservation、BodyHypothesis | [bodystate.md](./bodystate.md) |
| Diagnosis、ConditionEntry、分析与提案审计 | [diagnosis.md](./diagnosis.md) |
| TrainingPlan、PlanProposal、PlanSessionTemplate、Trainer、TrainingSessionLog | [training.md](./training.md) |
| SafetyState、SafetyRestriction、TrainerGate | [safety.md](./safety.md) |
| Person、Profile、资产与 Chat/Conversation | [person-chat.md](./person-chat.md) |

业务规则由 [requirements.md](../requirements.md) 定义，流程见 [agent-workflows.md](../agent-workflows.md)，视觉与交互见 [interface.md](../interface.md)，验收见 [acceptance.md](../acceptance.md)。未决定的语义保留在 [开放问题](../open-questions.md) 和 [领域决策登记](../domain-decisions.md)，不能当作已批准规则。

## 1. 聚合关系

~~~text
User
├─ Profile                    基础资料：身份与偏好（在 Person 中展示）
├─ BodyState (1)              当前身体事实及真实病程
│  ├─ BodyFact[]              用户自述、行为和生活习惯（含明确训练反馈）
│  ├─ SymptomEpisode[]        有事实依据的症状经历、关联与推导状态
│  ├─ BodyObservation[]       测量、自测和有来源的观察
│  ├─ BodyHypothesis[]        用户本人提出的怀疑（source = user）；AI 假设不进入 BodyState
│  ├─ ConcernGroups           由相关信息形成的派生组织范围（无独立权威）
│  ├─ SafetyState             权威风险状态与执行限制（Trainer 据此把关）
│  └─ BodyStateRevision[]
├─ Diagnosis (1)              当前采纳的具名医学诊断集合及停用历史
│  ├─ ConditionEntry[]        具名医学诊断概念，adopted/inactive，支持局部修订
│  ├─ DerivedSafetySummary     从 SafetyState 只读投影，非第二权威
│  ├─ ConditionDirectEdit[]   用户更正非实质元数据/撤回诊断的决定审计（§3.5）
│  └─ DiagnosisRevision[]
├─ ConditionAnalysisRun[]     正式分析执行审计（审计事件只追加；运行状态为投影），含 AI 假设（创建时快照 + 追加事件）与证据引用
│  ├─ ConditionProposal[]     本次分析提出的条目新增/变更
│  └─ ConditionProposalDecision[]  用户对每个提案的决定（审计）
└─ TrainingPlan (0..N)
   ├─ CurrentPlanContent     课模板、频率及按需排期；关联 ConditionEntry 与 BodyState 稳定证据
   ├─ TrainingPlanRevision[]
   ├─ PlanProposal[]        创建/更新的准确授权提案，Draft 非已生效 Plan
   └─ TrainingSessionLog[]    实际执行的训练记录（区别于计划中的 PlanSessionTemplate）
~~~

- 一项具名医学诊断可以引用多条 Fact/Observation/Episode 等证据；一条事实也可以被多个医学诊断引用，证据关联本身不建立因果。SymptomEpisode 只组织有来源的症状经历；DerivedConcernGroup 是派生显示，不作为 Diagnosis 的必需身份，也不创建 BodyConcern。
- Conversation 记录对话和工具活动；当前身体事实的权威来源是 BodyState（G-02）。
- Profile 保存相对稳定的身份信息；测量值、生活习惯和伤病状态属于具有时间语义的 BodyState。

## 5. 一致性、审计与权限

- Go 业务服务持有持久化状态和授权判定；Python Agent 提供结构化输入、推理与工具调用；Web 显示当前状态并提交用户动作。
- 每项写操作绑定 userId、sourceId、idempotencyKey、expectedRevision（适用时）与业务权限。
- 工具执行需要报告成功/失败，不允许前端在事务未提交时宣称已保存。
- 异步事件重复、乱序或网络重试不造成重复数据或重复修订；并发提交使用乐观锁或等效机制避免丢失修改。
- 训练提案的授权必须对应**准确的提案身份及内容**，不能重用对其他计划的授权；病情确认记录同样绑定准确的提案身份和内容。
- 病程与业务修订可检索，但不把内部编号或版本管理列为普通页面的默认入口。

## 6. 运行时不变量

| ID | 不变量 |
| --- | --- |
| INV-01 | 每用户各有一份权威 BodyState、Diagnosis；每份 TrainingPlan 有独立拥有者；Diagnosis 引用 BodyState 而不复制身体事实 |
| INV-02 | 用户自述不会被模型转化为临床确诊；Observation / Hypothesis 与 Fact 不混同；AI 假设不写入 BodyState |
| INV-03 | Correction 与 TemporalChange 是不同业务操作 |
| INV-04 | Diagnosis 局部 patch 不删除、不重建无关医学诊断条目；实质不同医学诊断使用不同 conditionId，同一概念的名称规范化可以保留 ID |
| INV-05 | 一次有效 Diagnosis 原子补丁最多产生一次整体新修订；无实际变更不创建空修订 |
| INV-06 | 具名医学诊断的创建、实质更新、采纳变化与诊断间关系均需要准确提案的用户接受；分析请求不是接受结论。用户直接元数据纠错/撤回走准确的轻量确认（§3.5） |
| INV-07 | 未经用户授权绝不改变 TrainingPlan 的当前有效内容 |
| INV-08 | 训练安全阻断与计划内容修订的权限独立：风险状态由 BodyState SafetyState 持有，Trainer 执行门禁，`TrainingPlan.status` 与内容不因此改变 |
| INV-09 | 工具、事件和网络重试具备幂等、错误提示和可恢复性 |
| INV-10 | 当前页面不暴露技术修订号、历史版本管理或后端运行编号 |
| INV-11 | ConditionEntry 的多来源声明、临床诊断原话、AI assessment 和用户 disposition 相互独立；用户采纳不改变 suspected/likely 或临床来源，旧 clinical_diagnosis 确定性等级退役 |
| INV-12 | ConditionAnalysisRun 中全部有效决定在一次原子事务中提交，提交前 Diagnosis 不变；以 safety_blocked 终结的 Run 不得提交 |
| INV-13 | 明确 reject 的语义相同提案，若无相关新证据 ID/版本不得重复提出；仅任意修订号变化不构成新证据 |
| INV-14 | ConditionEntry disposition 仅 adopted/inactive；停用原因（用户撤回、替代、不再适用等）独立追加审计；不会删除已采纳历史、BodyState 事实或制造症状已缓解结论 |
| INV-15 | 诊断临床来源必须保留医生原始不确定性和有准入资格的来源；用户转述与医疗文件不同，检验数值不能冒充医生具名诊断，较新文件不覆盖旧临床声明 |
| INV-16 | Run/Proposal 基础数据、提案内容与用户 Decision 及分析事件只追加、不可静默改写；当前 Run/Proposal state 为事件投影 |
| INV-17 | 新证据进入进行中的 Run 后，所有已决定提案必须复核后才能提交；受影响的提案重新提出并由用户重新决定，未受影响的不重复询问 |
| INV-18 | 对已有 ConditionEntry 的更新、停用或关系提案选 reject 不改变该条目；撤回已采纳诊断只能来自用户明确请求并经确认的直接编辑，或用户接受准确的 set_disposition 提案 |
| INV-19 | 直接编辑只由用户明确请求触发，限于不改变医学诊断实质的元数据纠错或明确撤回；保留谁、改什么、何时、为什么的独立授权审计，不允许 mark_resolved |
| INV-20 | 安全限制只按其 `liftPolicy` 解除：只有 `on_expiry` 到期自动解除；其他策略到期只触发复核、限制继续生效；每次解除记录与策略一致的依据；Agent 不能自行解除或放宽策略 |
| INV-21 | 提案 create/update/set_disposition/update_relation 的目标内容（包括状态目标、停用原因及关系）绑定内容指纹与用户 accept 决定；不得将对一项医学诊断的授权复用于另一内容 |
| INV-22 | 回放 Run/Proposal 基础记录、Decision 与有序事件应重建 Run outcome、hypotheses、证据、提案、待决/取代/过期状态及决定有效性；AI 运行时假设以 hypothesis_added 追加 |
| INV-23 | 以 `safety_blocked` 终结的 ConditionAnalysisRun 绝不产生 DiagnosisRevision：未决定提案 `expired`（`safety_blocked`），已作出的决定只留档、不生效；终结后该运行的补丁、提交与任何事件由 Go 后端拒绝；每个运行只有一个终结事件（先写入者胜出），重复的 `safety_blocked` 幂等 |

| INV-24 | 文件/图片提取的 BodyObservation 只有在对应证据候选按既有管线准入后才可形成；OCR 未准入候选不能直接成为可信观察或 Diagnosis；来源明确的用户自报/自测可以作为该来源等级的 Observation，但不得冒充临床核实结果 |
| INV-25 | DerivedConcernGroup 是组织投影，不得成为第二份身体事实的独立权威，重分组不制造事实 |
| INV-26 | TrainingSessionLog 的 completed / skipped 必须来自明确操作或陈述；播放演示、导航下一项和退出不能批量制造完成记录 |
| INV-27 | TrainingSessionLog.date 按用户时区定义并关联执行时的 planRevisionId/templateId；服务器日期不能代替用户训练日 |
| INV-28 | Person 是组合视图，不能持有另一份 BodyState、Diagnosis 或 Conversation 的权威状态 |
| INV-29 | 关闭或收起 Chat 不自动接受/拒绝待决定业务操作；pending 交互有稳定身份、可恢复或明确失效 |
| INV-30 | BodyFact 的陈述记录状态 asserted/superseded/withdrawn 与真实身体变化、症状经历状态、历史时间独立；不能由时间经过自行宣称症状结束 |
| INV-31 | SymptomEpisode 的 ongoing/ended/unknown 由有效 Fact、关联和领域事件可重建；High 可自动关联，关键历史更正及 Episode 合并拆分需要具体用户授权，旧身份与引用可追溯 |
| INV-32 | 症状经历的实质冲突由 Agent 优先 Ask User；未完成交互属 Chat 状态，不是 Episode 状态；只有状态确实受冲突影响才保守降为 unknown，安全风险不等待关联完成 |
| INV-33 | 用户自述观察、自测/设备观察与文件候选准入各留来源等级；记录状态、文件准入与医学证据权威不混用；AI 假设不写入用户 BodyHypothesis |
| INV-34 | Person 的 Records/Habits 只是各领域真值的视图或入口；事实更正、陈述撤回与健康数据隐私删除不可混用，删除清除与衍生证据处理须另有明确契约（DC-033/035） |
| INV-35 | ConditionEntry 必须是具名医学疾病或综合征而非纯症状标题；无足够病名证据时保留 BodyState/Run，不编造正式 Diagnosis 条目 |
| INV-36 | BodyState 与 Diagnosis 按证据 ID/版本形成多对多关系，supporting/counter/context 区分，SymptomEpisode 和其 Fact 不得重复计权或暗示未经证实的因果 |
| INV-37 | Agent 评估仅 suspected/likely 两个定性等级；临床声明可以不带 AI assessment，必须原样表示“考虑”等医学不确定性 |
| INV-38 | 新证据/来源变化只能产生派生的诊断复核提示；正式内容更新由用户发起分析并确认，提示去重且不另建 DiagnosisReviewTask |
| INV-39 | 诊断关系与替代不由共享症状自动推导；新诊断采纳不隐式使旧条目 inactive，inactive 重新采纳须重新分析并确认 |
| INV-40 | 未接受的 Plan Draft/PlanProposal 不是有效 TrainingPlan；首次创建需要用户明确接受。Plan 只关联稳定 ConditionEntry/BodyState 证据，不创建 BodyConcern |
| INV-41 | 动作概念 exerciseId、模板条目 planExerciseId、训练实例 sessionLogId 分别稳定标识概念、处方位置和实际执行；同名/同日不合并执行事实 |
| INV-42 | Trainer 只在用户明确完成、跳过或更正时追加有效动作记录；跨午夜/时区按开始时用户时区快照记录训练日；未排期不自动生成漏练 |
| INV-43 | PlanProposal 的接受校验身份、内容指纹、底版、7 天期限与关联安全上下文；并发/新证据失效重新授权；安全阻断不等于计划内容修订 |
| INV-44 | SafetyState 的 advisory/restricted/urgent 门禁独立于计划状态，重叠限制以最严格有效限制为准；解除一条不能放行仍被另一条限制覆盖的动作 |
| INV-45 | urgent 风险可终止运行中的 Trainer 并按既有 Diagnosis safety_blocked 语义处理；restricted 命中执行暂停；风险事件绝不伪造完成/跳过或改写计划历史 |

## 7. 状态索引与领域权威

| 对象 | 已定义状态/维度 | 详细转换与规则 |
| --- | --- | --- |
| BodyFact | assertionStatus: asserted / superseded / withdrawn；实际时间单独建模 | [BodyState](./bodystate.md) |
| SymptomEpisode | 由证据推导 unknown / ongoing / ended；身份 current / superseded | [BodyState](./bodystate.md) |
| BodyObservation / BodyHypothesis | 来源分级与各自的 assertionStatus；文件准入独立 | [BodyState](./bodystate.md) |
| ConditionEntry | 具名医学诊断身份 + assessment（AI suspected/likely，可空） + 多来源声明 + evidenceLinks + disposition（adopted/inactive）；没有通用 lifecycle | [Diagnosis](./diagnosis.md) |
| ConditionAnalysisRun | open / committed / closed_without_change 与 outcome | [Diagnosis](./diagnosis.md) |
| ConditionProposal | pending / decided / superseded / expired | [Diagnosis](./diagnosis.md) |
| TrainingPlan | active / paused / completed | [TrainingPlan](./training.md) |
| TrainingSessionLog | in_progress / paused / ended；pauseReason 与 endedReason 独立 | [Trainer](./training.md) |
| PlanProposal（create/update） | pending / accepted / rejected / expired；7 天未决定则过期 | [TrainingPlan](./training.md) |
| SafetyRestriction | advisory/restricted/urgent；由作用范围、有效期和解除依据推导门禁，无独立 status 枚举 | [SafetyState](./safety.md) |
| 文档证据候选 | admissible / needs_review / rejected | [Person / Chat](./person-chat.md) |
| Chat 待回复交互 | pending；accept / reject / resume / expire / stale 等语义操作 | [Person / Chat](./person-chat.md) |

**完整领域契约须同时提供字段、状态、触发、授权、写入副作用、历史/回放及验收。** 已确定的规则保持唯一权威，未决字段和状态转移见 [domain-decisions.md](../domain-decisions.md)，不允许仅据 UI 名称补全领域语义。

## 8. 契约完整度与待决入口

本目录是**目标契约**，不代表功能已上线。下表用来识别仍不可直接落到数据库与 API 的字段，不用单一 “ready” 标签掩盖缺口。

| 领域 | 现已明确 | 尚待业务决策 | 已有实现差异 |
| --- | --- | --- | --- |
| BodyState | BS-D1～D10 已确认：五类事实、症状 Episode、记录/病程独立、主动冲突澄清、来源分级、隐私删除边界 | 旧模型迁移及精确物理 Schema DC-025～030、DC-032；删除传播 DC-033/035；安全详见 Safety 领域 | IG-05；新增模型仍待实施 |
| Diagnosis | ConditionEntry 四维状态、Run/Proposal/Decision 事件回放与原子提交 | DC-021/024、DC-047～051、DC-066～070（复发、重新采纳与历史） | IG-05 与旧 Diagnosis 迁移 |
| TrainingPlan/Trainer | TP-D1～D12：计划、课模板、稳定动作身份、草案/建议、修订、排期、训练日、逐组真实执行、反馈和跨域授权已决定 | PlanPatch 物理 DTO、旧 Treatment/CheckIn 迁移、离线计时精度与动作库资源映射仍需工程方案 DC-052～061 | IG-01/02/03；新目标模型未实施 |
| SafetyState | SAFE-D1～D8：三级产品风险、全局/计划/动作门禁、最严格优先、解除依据、分析与执行安全终止、并发原则已决定 | 临床安全规则/阈值及专业审查、旧安全 JSON 迁移与后端条件更新仍需工程验证 DC-062～065 | IG-04；旧自由文本解除不能视为合规 |
| Person/Chat | Person 五标签是组合视图；Habits/Records 自动记录与权威边界已按 BS-D8 定稿；Conversation 不是身体事实真值，交互身份可恢复 | DC-009、DC-015～017、DC-023、DC-034～046（身份、隐私、会话） | IG-06/07 |

**完整性判定**：每个对象最终都需要字段表（含来源、默认/可空、单位与精度）、状态枚举和非法组合、合法转换表（触发/授权/事务/审计）、数据迁移规则以及 UI 对照和可执行验收。当前遗漏项按决策登记，业务规则已定不等于物理 Schema 可直接迁移；必须先完成映射和验证。
