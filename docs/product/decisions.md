# BodySense · 产品决策记录

**本文件记录已由产品负责人（Arlon）决定的问题。** 决定的内容已写入对应规格文件，规则以规格文件为准；本表只说明决定了什么、为什么、写在哪里。尚未决定的问题见 [开放问题](./open-questions.md)。

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
