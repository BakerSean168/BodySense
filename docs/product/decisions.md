# BodySense · 产品决策记录

**本文件记录已由产品负责人（Arlon）决定的问题。** 决定的内容已写入对应规格文件，规则以规格文件为准；本表只说明决定了什么、为什么、写在哪里。尚未决定的问题见 [开放问题](./open-questions.md)。

## 2026-10-12 · BodyState 业务模型评审（BS-D1～D10）

本轮为**业务逻辑契约确认**，并不代表 Go/API、数据迁移、证据删除机制已经实现。BS-D7 来自用户对实时主动澄清的明确要求；BS-D8～D10 已确认其原则，其中具体的数据擦除范围与技术流程继续由 DC-033/035 决定。

| ID | 已确认的业务原则 | 影响范围 |
| --- | --- | --- |
| BS-D1 | 历史时间、真实身体变化与记录更正分离；BodyFact 不使用 active/resolved/historical 混合生命周期 | R-BS-04/07，domain/bodystate §2.2 |
| BS-D2 | BodyFact 保留 symptom、health_history、activity、lifestyle、training_feedback 五类，按类别分别定义细节和适用时间 | R-BS-08，domain/bodystate §2.3 |
| BS-D3 | 用户陈述是 BodyFact；同一段症状经历由 SymptomEpisode 组织，复发关联但不复制旧事实 | R-BS-07，domain/bodystate §2.4 |
| BS-D4 | 症状结束/复发以有依据的时间与语义为准；固定天数及沉默不构成自动结束或拆分 | R-BS-07，domain/bodystate §2.4 |
| BS-D5 | High 自动关联；Medium/Low 保留事实、按需澄清；Episode 关键更正、合并、拆分要求明确授权 | R-BS-07，INV-31 |
| BS-D6 | Episode 状态由有效 Fact、关联与领域事件推导，可缓存但不能脱离来源任意写状态 | domain/bodystate §2.4，INV-31 |
| BS-D7 | 存在关键冲突时 Agent 在实时 Chat 优先 Ask User 并给选项与自由说明；未完成且影响状态时 unknown 兜底，保留冲突依据 | R-BS-10，INV-32 |
| BS-D8 | Habits/Records 中明确的本人陈述按权威领域规则自动入库；Person 只是视图及编辑入口 | R-BS-08、R-PS-05，domain/person-chat |
| BS-D9 | Observation 分离自报、自测、设备与文件证据来源；OCR 候选须准入，自报不冒充临床测量，证据资格与陈述状态不同 | R-BS-09，INV-24/33 |
| BS-D10 | 事实纠错、撤回陈述与删除健康数据分离；必须真正处理来源与衍生数据，删除的具体范围和传播机制仍开放 | R-BS-10，INV-34；DC-033/035 |

## 2026-10-12 · 开放问题 Q-01～Q-14 评审

| ID | 决定 | 原因 | 写入位置 |
| --- | --- | --- | --- |
| Q-01 | 不新增 `refuted`。ConditionEntry 拆成两个维度：`lifecycle`（active / monitoring / resolved，真实病程）与 `disposition`（adopted / withdrawn_by_user / no_longer_applicable，解释是否仍被采纳）。撤回保留条目、历史及原因/时间；BodyState 症状事实不变 | 用户否定 AI 解释不等于医学上已排除；病程与采纳是两件事 | requirements R-DX-01、R-DX-04 · domain §3、§3.2、INV-14 |
| Q-02 | 用户对提案的决定独立留档（proposalId、内容指纹、引用证据 ID、决定、decidedAt）。「新证据」按证据 ID / 指纹判断，不按 BodyState 修订号；无相关新证据时不得重复提出同一指纹 | 修订号变化不代表出现了与该提案相关的新证据 | domain §3.1、INV-13 · acceptance DX-07c/e/f |
| Q-03 | 分开记录来源：临床文件 vs 用户转述的医生诊断。用户确认只表示「纳入我的个人病情管理」，不是重新判断医生的诊断；「临床文件记载」只由来源类型决定，不能通过点击确认获得 | 避免用户转述被当作有文件支持 | requirements R-DX-08 · domain §3、§3.2、INV-11 · acceptance BS-05、DX-09 |
| Q-04 | 用户本人的怀疑自动记录为 BodyHypothesis（source = user）；AI 假设只留在分析过程与提案审计中，不进入 BodyState | BodyState 只保存用户本人的事实与陈述 | requirements R-BS-06 · domain §2 · agent-workflows §3.1 · glossary |
| Q-05 | 顶层 `exerciseStages` 改为 `sessions: PlanSessionTemplate[]`，每个模板含 warmup / main / cooldown；与实际执行的 TrainingSessionLog 区分；单课计划只有一个模板 | 支持 A/B 日等多课计划，计划内容与执行记录不混用 | requirements R-TP-02 · domain §4 · acceptance TP-09 |
| Q-06 | 关闭界面不是拒绝，提案保持 pending 可恢复；明确「暂不修改」= rejected；超时或底版修订变化 = expired | 关闭窗口不是用户的决定 | requirements R-TP-04 · domain §4 · agent-workflows §4 · acceptance TP-04/b/c |
| Q-07 | 把归档训练规格中仍有效的 Trainer 规则并入（当前动作、组次、计时、暂停/恢复、跳过；训练记录按计划/日期/动作定位；不从点击推断完成）。Penpot Draft 状态不删除，含义作为开放问题待核对 | 恢复被归档丢失的有效规则 | requirements R-EX-03～04 · domain §4 · interface §5.1 · acceptance TP-10/11 · 开放问题 Q-15 |
| Q-08 | 上传资料归属 Person（Media / Reports）。不另立识别流程，沿用健康文件证据管线与 ADR 0013 的规则：原始文件 → 抽取 → 证据采纳 → BodyObservation / Diagnosis 分析；可靠结果可按规则自动采纳，存疑结果等待用户核实，未核实/低于阈值不算可信临床证据，自动采纳的数值不会自动形成诊断 | 已有经过设计的证据治理，不重复定义 | requirements R-PS-03 · domain §2.1 · interface §3 · acceptance BS-11 · architecture/health-document-evidence-pipeline.md（消费方改称 Diagnosis analysis） |
| Q-09 | 风险状态由 BodyState SafetyState 权威持有，Trainer 执行门禁；增加风险等级、范围（计划/动作）、有效期、解除条件；`TrainingPlan.status` 不变，界面单独显示「暂不宜执行」 | 安全阻断与计划内容/状态权限独立 | requirements R-SAFE-03 · domain §2、INV-08 · interface §5、§5.1 · acceptance TP-06/b/c |
| Q-10 | AGENT.md 真值顺序按问题类型区分：业务目标与规则 → docs/product；当前技术实现 → 代码与测试；旧代码不能否决尚未实现的业务规则 | 代码正在按新规格重构 | AGENT.md「真值顺序」 |
| Q-11 | 三篇架构文档补上 confirm_condition 步骤，链接到 agent-workflows 的授权/确认规则，不另行定义 | 规则只有一个归属 | architecture/longitudinal-health-loop.md、health-journey-workflow.md、longitudinal-body-state-domain.md |
| Q-12 | 不改写历史 ADR；旧模型工程文档开头加简短说明（描述重构前实现，指向 docs/product）；暂不新建 ADR | ADR 是历史决策记录 | architecture/current-longitudinal-system.md、technical-approach.md、treatment-*、diagnosis-*、agent-platform-role-governance.md（CONTEXT.md 已有同类说明） |
| Q-13 | 拆为 ConditionAnalysisRun（一次分析执行）、ConditionProposal（提出的条目变更）与 ConditionProposalDecision（用户决定，见 Q-02），不再使用单一 DiagnosisAnalysis；该名称保留给旧代码 | 避免与旧代码同名异义 | domain §3.1 · glossary §1、§3 |
| Q-14 | Person 保留五个标签页；身份放在窗口标题区 + Identity Popover；修正 design/README 的分类列表 | 与 Penpot 原型一致 | requirements R-PS-04 · interface §3 · design/README.md · glossary |

## 2026-10-12 · 评审中补充的缺口

| ID | 决定 | 原因 | 写入位置 |
| --- | --- | --- | --- |
| GAP-A | 旧 `certainty` 拆为 `source`（clinical_document / user_reported_clinical / agent_analysis / user_suspicion）、`conclusionStatus`（likely / suspected / clinical_diagnosis）与 `confirmation`（likely / keep_as_suspected / include_clinical、toolCallId、时间、范围）。保留三选一确认（大概率是 / 可能是先保留 / 不是 + 继续补充）；「大概率是」= 用户接受记录一个大概率的可能性，不是临床诊断；来源升级路径可描述 | 一个字段混了来源、强度和用户决定三个维度 | requirements R-DX-04、R-DX-08 · domain §3、§3.2 · glossary §2 · acceptance DX-07、DX-10 |
| GAP-B | 一次正式分析可产生多个提案，各自独立决定；全部结算（或运行结束）后，所有生效决定在一个原子补丁中提交，只产生一次 Diagnosis 修订；定义超时、部分决定、继续补充、重新进入对话、底版变化的行为；提交前 Diagnosis 不变，已作出的决定不丢失 | 多病情确认的原子性此前未定义 | requirements R-DX-12 · domain §3.3、INV-12 · agent-workflows §3.2 · interface §4 · acceptance DX-03～03f |
| GAP-C | 三要素不强迫编造：名称可为描述性健康问题；原因可写「原因不明 / 证据不足」；处理方案可为观察、进一步检查、就医，不一定是训练处方 | 避免为凑齐三要素制造病名或病因 | requirements R-DX-11 · domain §3 · interface §4 · acceptance DX-11 |

## 2026-10-12 · PR #245 评审

原因列为摘要。

| ID | 决定 | 原因（摘要） | 写入位置 |
| --- | --- | --- | --- |
| Q-16 | 针对已有条目的提案，用户选「不是这个」= 只拒绝这次变更；原条目内容与 `disposition` 不变，决定照常留档。只有用户明确要求撤回（见 Q-20）才改变 `disposition` | 拒绝一次修改不等于否定原来已确认的病情 | requirements R-DX-04 · domain §3.2、§3.3、INV-18 · glossary §2 · acceptance DX-07d、DX-07g |
| Q-18（部分） | SafetyRestriction 增加 `liftPolicy`（`on_expiry` / `user_reverification` / `professional_clearance`）。只有 `on_expiry` 到期自动解除；其他策略到期只触发复核提示，限制继续生效，直到用户经复核重新确认症状，或记录专业人员放行及来源。警示 / 建议就医类默认 `professional_clearance`。Agent 不能自行解除限制。`riskLevel` 取值与其余风险的策略映射仍开放 | 到期不代表风险已消除 | requirements R-SAFE-04 · domain §2、INV-20 · agent-workflows §5 · interface §5 · glossary §1 · acceptance TP-06～06f |
| Q-20 | 用户明确要求更正、撤回或标记缓解某个条目时，不需要完整 AI 分析；走独立、可审计的直接编辑路径：轻量确认准确变更（如「确认撤回 X？」），记录为用户决定审计（谁、什么、何时、原因），产生一次 Diagnosis 修订。Agent 不得用此路径做自己的修改 | 用户对自己记录的明确指令不应被迫走分析流程，但仍需留痕 | requirements R-DX-13 · domain §1、§3、§3.5、INV-06、INV-19 · agent-workflows §1、§3、§3.5 · interface §4 · glossary §1、§2 · acceptance DX-14～14e |
| Q-21 | 运行中继续补充或出现新证据后，提交前必须按证据 ID 用新证据复核每个已决定提案：不受影响的决定照常生效；受影响的提案记为 superseded（`evidence_changed`），须重新提出并重新决定；受影响的决定绝不未经复核提交 | 已作出的决定可能依据已过时的证据 | requirements R-DX-12 · domain §3.1、§3.3、INV-17 · agent-workflows §3.2、§4 · glossary §2 · acceptance DX-03d、DX-03g、DX-03h |
| GAP-D | 临床文件证据 ≠ 临床诊断：只有明确记载临床医生诊断的已采纳文件才能支持 `clinical_document` + `clinical_diagnosis`；化验 / 影像数值只作为 `agent_analysis` 提案的证据，不升级来源或结论 | 避免把支持性数值当作医生诊断 | requirements R-DX-08 · domain §3、§3.2、INV-15 · glossary §2 · acceptance DX-10、DX-12 |
| GAP-E | ConditionAnalysisRun 的「不可变」指追加写入的审计事件（运行开始、证据加入、提案创建/取代/过期、决定、提交/关闭）；运行与提案的当前状态是由事件推导的投影，可以更新；提案内容与决定记录写入后不变 | 消除「不可变」与状态会变化之间的歧义 | domain §1、§3.1、INV-16 · glossary §1 · acceptance DX-13 |

## 2026-10-12 · 结束提案与事件回放补充

原因列为摘要。

| ID | 决定 | 原因（摘要） | 写入位置 |
| --- | --- | --- | --- |
| GAP-F | `changeKind = end` 的提案必须在内容中写明 `endTarget`（`mark_resolved` → `lifecycle = resolved`；`mark_no_longer_applicable` → `disposition = no_longer_applicable`）与 `endReason`，`endTarget` 计入内容指纹。新增决定值 `accept_end`（只用于结束提案）；「不是这个 / 不同意」= `rejected`，条目不变；继续补充照常。提交只应用被批准的 `endTarget`。不再适用只来自被接受的结束提案；标记缓解还可经用户直接编辑（R-DX-13） | 结束提案原先没有明确的结束结果，也没有对应的「同意」决定值，用户批准的内容与提交结果可能不一致 | requirements R-DX-04 · domain §3、§3.1、§3.2、§3.3、INV-21 · agent-workflows §3、§3.2 · interface §4 · glossary §1、§2 · acceptance DX-07g～07j |
| GAP-G | 明确事件回放契约：ConditionAnalysisRun 基础记录（请求、`trigger`、范围、底版修订、输入证据 ID、`agentHypotheses` 快照）与 ConditionProposal 基础记录（内容含 `endTarget`、指纹、证据 / 反证 ID、取代关系）创建时写入且不变；运行与提案的状态字段全部由事件推导，并给出逐字段来源。新增终结事件 `insufficient_evidence`、`safety_blocked`（无提案结束运行），`decision_recorded` 带 `proposalId`；按顺序回放基础记录与事件必须重现当前状态 | 原契约未区分不变的基础数据与派生状态，且证据不足 / 安全阻断结束的运行没有对应事件，投影无法完全回放 | domain §3.1、INV-16、INV-22 · agent-workflows §3.2 · glossary §1 · acceptance DX-04、DX-13b |

## 2026-10-12 · 运行中安全阻断与 AI 假设审计

原因列为摘要。

| ID | 决定 | 原因（摘要） | 写入位置 |
| --- | --- | --- | --- |
| GAP-H | 只有需要停止分析的严重风险（警示 / 需尽快就医类）中断正式分析：立即展示医学安全提示，停止该运行后续确认；只影响 Trainer 的普通训练限制不中断。已有提案的运行也可以 `safety_blocked` 终结：未决定提案 `expired`（`safety_blocked`），已作出的决定留档但永不生效；不产生 DiagnosisRevision、无部分提交；风险处理后由用户重新发起分析，不自动恢复。Go 后端强制：拒绝已阻断运行的补丁 / 提交及终结后的任何事件；提交与阻断竞争时先写入的终结事件胜出；重复阻断幂等 | 原规则只允许在产生提案前阻断，确认中途出现的严重风险无处理路径；安全不能只依赖 Agent 提示 | requirements R-DX-10 · domain §3.1、§3.3、INV-12、INV-23 · agent-workflows §3、§3.2、§5 · interface §4 · glossary §2 · acceptance DX-04～04g、DX-13b |
| GAP-I | 运行开始后形成的 AI 假设以追加事件 `hypothesis_added`（内容、证据 ID、时间）记录；创建时的 `agentHypotheses` 快照保留为基础记录不变；投影 `hypotheses` = 快照 + 追加事件，可回放。AI 假设始终无权威，不写入 BodyState，也不直接进入 Diagnosis | 假设常在补充证据后才出现，只有创建时快照会让后来的假设无审计 | domain §1、§3.1、INV-22 · agent-workflows §3、§3.2 · glossary §1 · acceptance DX-13b、DX-13c |
