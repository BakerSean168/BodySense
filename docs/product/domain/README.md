# BodySense · 领域模型总览

此目录是 [产品规格](../README.md) 的领域契约权威来源，定义目标逻辑模型，不意味着代码已经实现。

| 领域 | 权威文档 |
| --- | --- |
| BodyState、BodyFact、BodyObservation、BodyHypothesis | [bodystate.md](./bodystate.md) |
| Diagnosis、ConditionEntry、分析与提案审计 | [diagnosis.md](./diagnosis.md) |
| TrainingPlan、Trainer、TrainingSessionLog | [training.md](./training.md) |
| SafetyState、SafetyRestriction | [safety.md](./safety.md) |
| Person、Profile、资产与 Chat/Conversation | [person-chat.md](./person-chat.md) |

业务规则由 [requirements.md](../requirements.md) 定义，流程见 [agent-workflows.md](../agent-workflows.md)，视觉与交互见 [interface.md](../interface.md)，验收见 [acceptance.md](../acceptance.md)。未决定的语义保留在 [开放问题](../open-questions.md) 和 [领域决策登记](../domain-decisions.md)，不能当作已批准规则。

## 1. 聚合关系

~~~text
User
├─ Profile                    基础资料：身份与偏好（在 Person 中展示）
├─ BodyState (1)              当前身体事实及真实病程
│  ├─ BodyFact[]              用户自述、行为和生活习惯
│  ├─ BodyObservation[]       测量、自测和有来源的观察
│  ├─ BodyHypothesis[]        用户本人提出的怀疑（source = user）；AI 假设不进入 BodyState
│  ├─ ConcernGroups           由相关信息形成的组织范围
│  ├─ SafetyState             权威风险状态与执行限制（Trainer 据此把关）
│  └─ BodyStateRevision[]
├─ Diagnosis (1)              用户确认过的病情综合诊断（含用户选择保留的疑似病情）
│  ├─ ConditionEntry[]        稳定身份，支持多项局部更新
│  ├─ SafetySummary
│  ├─ ConditionDirectEdit[]   用户直接更正 / 撤回 / 标记缓解的决定审计（§3.5）
│  └─ DiagnosisRevision[]
├─ ConditionAnalysisRun[]     正式分析执行审计（审计事件只追加；运行状态为投影），含 AI 假设（创建时快照 + 追加事件）与证据引用
│  ├─ ConditionProposal[]     本次分析提出的条目新增/变更
│  └─ ConditionProposalDecision[]  用户对每个提案的决定（审计）
└─ TrainingPlan (0..N)
   ├─ CurrentPlanContent
   ├─ TrainingPlanRevision[]
   ├─ PlanChangeProposal[]
   └─ TrainingSessionLog[]    实际执行的训练记录（区别于计划中的 PlanSessionTemplate）
~~~

- 一个病情可以关联多个 Concern，一个 Concern 可关联多种候选解释。Concern 是便于组织事实的派生分组，不是另一份身体真值，不要求用户创建或管理独立问题实体与报告。
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
| INV-01 | BodyState、Diagnosis 和每份 TrainingPlan 各有明确、唯一的业务拥有者 |
| INV-02 | 用户自述不会被模型转化为临床确诊；Observation / Hypothesis 与 Fact 不混同；AI 假设不写入 BodyState |
| INV-03 | Correction 与 TemporalChange 是不同业务操作 |
| INV-04 | Diagnosis 的局部补丁不会删除或改写无关病情 |
| INV-05 | 同一 Diagnosis 原子补丁最多产生一个整体新修订 |
| INV-06 | Diagnosis 条目的创建/实质更新需要可追溯的用户请求，以及用户对病名、原因和处理方案的确认决定（`likely` / `keep_as_suspected` / `include_clinical`，见 §3.2）；单纯请求分析不代表接受 Agent 结论。用户直接更正 / 撤回 / 标记缓解以对准确变更的确认代替病情确认工具（§3.5） |
| INV-07 | 未经用户授权绝不改变 TrainingPlan 的当前有效内容 |
| INV-08 | 训练安全阻断与计划内容修订的权限独立：风险状态由 BodyState SafetyState 持有，Trainer 执行门禁，`TrainingPlan.status` 与内容不因此改变 |
| INV-09 | 工具、事件和网络重试具备幂等、错误提示和可恢复性 |
| INV-10 | 当前页面不暴露技术修订号、历史版本管理或后端运行编号 |
| INV-11 | `ConditionEntry.source` 只由证据来源决定，用户确认不能把条目变为 `clinical_document`；`clinical_diagnosis` 只与临床来源同时出现 |
| INV-12 | 一次 ConditionAnalysisRun 的全部生效决定在一个原子补丁中提交；提交前 Diagnosis 不变，已作出的决定不丢失（以 `safety_blocked` 终结的运行不提交，见 INV-23） |
| INV-13 | 已被用户拒绝的提案指纹，无相关新证据 ID 时不得再次提出 |
| INV-14 | 撤回 / 不再适用只改变 `disposition`，条目与历史保留，`lifecycle` 与 BodyState 事实不被改写 |
| INV-15 | 只有明确记载临床医生诊断的已采纳文件证据才能支持 `source = clinical_document` + `conclusionStatus = clinical_diagnosis`；化验 / 影像等数值只作为 `agent_analysis` 提案的证据，不升级来源或结论 |
| INV-16 | 正式分析的审计事件、提案内容与用户决定只追加、不改写；运行与提案的当前状态是由事件推导的投影，可以更新 |
| INV-17 | 运行中加入新证据后，每个已决定提案都必须对其决定之后的全部新证据 ID 完成复核才能提交；受影响的决定失效并须重新提出、重新决定 |
| INV-18 | 对已有条目提案的 `rejected` 不改变该条目；`withdrawn_by_user` 只来自用户明确请求并经确认的直接编辑 |
| INV-19 | 直接编辑路径只能由用户的明确请求触发，Agent 不得用它做自己的修改；每次直接编辑留下用户决定审计（谁、什么、何时、原因），并产生一次 Diagnosis 修订 |
| INV-20 | 安全限制只按其 `liftPolicy` 解除：只有 `on_expiry` 到期自动解除；其他策略到期只触发复核、限制继续生效；每次解除记录与策略一致的依据；Agent 不能自行解除或放宽策略 |
| INV-21 | `changeKind = end` 的提案必须写明 `endTarget`（`mark_resolved` / `mark_no_longer_applicable`）并计入内容指纹；提交只应用 `accept_end` 决定所针对的那个 `endTarget`；`disposition = no_longer_applicable` 只来自被接受的 `mark_no_longer_applicable` 结束提案 |
| INV-22 | 正式分析的投影可完全回放：按顺序回放 ConditionAnalysisRun / ConditionProposal 基础记录、ConditionProposalDecision 与 ConditionAnalysisEvent，可重现当前每个运行与提案的状态（含 `outcome`、`settledAt`、`hypotheses`、过期/取代原因与决定是否生效）；证据不足与安全阻断各有终结事件；运行开始后形成的 AI 假设以 `hypothesis_added` 追加，不改写创建时快照 |
| INV-23 | 以 `safety_blocked` 终结的 ConditionAnalysisRun 绝不产生 DiagnosisRevision：未决定提案 `expired`（`safety_blocked`），已作出的决定只留档、不生效；终结后该运行的补丁、提交与任何事件由 Go 后端拒绝；每个运行只有一个终结事件（先写入者胜出），重复的 `safety_blocked` 幂等 |

| INV-24 | BodyObservation 只有在可信证据准入后才可形成；OCR 未准入候选不能直接成为可信观察或 Diagnosis |
| INV-25 | DerivedConcernGroup 是组织投影，不得成为第二份身体事实的独立权威，重分组不制造事实 |
| INV-26 | TrainingSessionLog 的 completed / skipped 必须来自明确操作或陈述；播放演示、导航下一项和退出不能批量制造完成记录 |
| INV-27 | TrainingSessionLog.date 按用户时区定义并关联执行时的 planRevisionId/templateId；服务器日期不能代替用户训练日 |
| INV-28 | Person 是组合视图，不能持有另一份 BodyState、Diagnosis 或 Conversation 的权威状态 |
| INV-29 | 关闭或收起 Chat 不自动接受/拒绝待决定业务操作；pending 交互有稳定身份、可恢复或明确失效 |
## 7. 状态索引与领域权威

| 对象 | 已定义状态/维度 | 详细转换与规则 |
| --- | --- | --- |
| BodyFact | lifecycle: active / resolved / historical | [BodyState](./bodystate.md) |
| ConditionEntry | source / conclusionStatus / lifecycle / disposition 四个独立维度 | [Diagnosis](./diagnosis.md) |
| ConditionAnalysisRun | open / committed / closed_without_change 与 outcome | [Diagnosis](./diagnosis.md) |
| ConditionProposal | pending / decided / superseded / expired | [Diagnosis](./diagnosis.md) |
| TrainingPlan | active / paused / completed | [TrainingPlan](./training.md) |
| TrainingSessionLog | in_progress / paused / ended | [Trainer](./training.md) |
| PlanChangeProposal | pending / accepted / rejected / expired | [TrainingPlan](./training.md) |
| SafetyRestriction | 由有效期、作用范围和解除依据推导门禁；无独立 status 枚举 | [SafetyState](./safety.md) |
| 文档证据候选 | admissible / needs_review / rejected | [Person / Chat](./person-chat.md) |
| Chat 待回复交互 | pending；accept / reject / resume / expire / stale 等语义操作 | [Person / Chat](./person-chat.md) |

**完整领域契约须同时提供字段、状态、触发、授权、写入副作用、历史/回放及验收。** 已确定的规则保持唯一权威，未决字段和状态转移见 [domain-decisions.md](../domain-decisions.md)，不允许仅据 UI 名称补全领域语义。

## 8. 契约完整度与待决入口

本目录是**目标契约**，不代表功能已上线。下表用来识别仍不可直接落到数据库与 API 的字段，不用单一 “ready” 标签掩盖缺口。

| 领域 | 现已明确 | 尚待业务决策 | 已有实现差异 |
| --- | --- | --- | --- |
| BodyState | 用户事实与观察/假设的认识论边界、主要来源、时间与更正原则、修订不变量 | DC-019/020、DC-025～033（Observation 完整逻辑字段和 Hypothesis 生命周期仍未定） | IG-05 |
| Diagnosis | ConditionEntry 四维状态、Run/Proposal/Decision 事件回放与原子提交 | DC-021/024、DC-047～051、DC-066～070（复发、重新采纳与历史） | IG-05 与旧 Diagnosis 迁移 |
| TrainingPlan/Trainer | 计划/日志/提案分开持有状态、基本动作/组次、授权与安全门禁 | DC-010～014、DC-052～061（排期、修订、跳过及结束语义） | IG-01/02/03 |
| SafetyState | 三类 liftPolicy、生效范围、何时阻断与不可绕过原则 | DC-001～006、DC-062～065（风险等级、范围重叠、解除证据） | IG-04 |
| Person/Chat | Person 五标签是组合视图，Conversation 不是身体事实真值，交互身份可恢复 | DC-007～009、DC-015～017、DC-023、DC-034～046（身份、隐私、会话） | IG-06/07 |

**完整性判定**：每个对象最终都需要字段表（含来源、默认/可空、单位与精度）、状态枚举和非法组合、合法转换表（触发/授权/事务/审计）、数据迁移规则以及 UI 对照和可执行验收。当前遗漏项按决策登记，尚不能把这些文件视为完整可迁移的最终 Schema。
