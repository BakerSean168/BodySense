# BodySense · 产品决策记录

**本文件记录已由产品负责人（Arlon）决定的问题。** 决定的内容已写入对应规格文件，规则以规格文件为准；本表只说明决定了什么、为什么、写在哪里。尚未决定的问题见 [开放问题](./open-questions.md)。

## 2026-10-13 · TrainingPlan / Trainer 一揽子决策（TP-D1～D12）

**一次收敛，不逐题重复征询。** 业务规则以 [Training 契约](./domain/training.md) 和 [Safety 契约](./domain/safety.md) 为准；下述已定业务语义不代表运行时或物理 Schema 完成迁移。

| ID | 已确定的业务语义 | 对应待决 |
| --- | --- | --- |
| TP-D1 | 多份独立完整训练计划，状态 active/paused/completed 仅由明确用户操作转换；多计划可同时 active，已完成计划可明确重新启用 | DC-012/060 |
| TP-D2 | Draft 为未接受计划预览；Agent 拟定的首次创建计划经用户明确接受才产生首个有效 Plan；统一 `PlanProposal(create/update)` | DC-014、Q-15 |
| TP-D3 | 用户明确、低歧义调整可直接授权精确修改；Agent 主动建议只生成具体 Proposal，关闭保持 pending，拒绝不改计划 | R-TP-03/04、DC-052 |
| TP-D4 | 计划提案 7×24 小时未决过期；底版或相关安全/诊断/BodyState 发生实质变化提前失效；内容指纹、用户身份和并发控制严格校验 | DC-052/053；Q-17 的 Training 部分 |
| TP-D5 | 修订是不可变内容快照+精确 patch+用户授权；计划状态事件独立；同一医学诊断/BodyState 证据可关联，不创建 BodyConcern | DC-052/053/055 |
| TP-D6 | 默认仅训练频率，明确选择才建立日历排期；不自动判漏练/补练；执行开始时 IANA 用户时区和日期固定，同日多次训练不合并 | DC-010/022/061 |
| TP-D7 | `exerciseId` 是动作概念，`planExerciseId` 是模板条目，`sessionLogId` 是实际训练实例；同名不等于同 ID，动作替换创建新条目 ID | DC-011/056 |
| TP-D8 | 完成/跳过按用户明确操作记录 set 或 exercise 粒度；错误动作以更正事件追加，不以浏览、下一项或演示推断执行事实 | DC-013/054/058 |
| TP-D9 | Session in_progress/paused/ended 分离 pauseReason、endedReason；正常/提前/安全结束可区分，计时暂停不累计；断线恢复不伪造训练事实 | DC-057/059 |
| TP-D10 | 单用户最多一个正在执行的 Session，允许多份暂停与同日多个不同 sessionLogId；旧训练日志按来源能力保守迁移，不把打卡当组完成 | DC-054/056 |
| TP-D11 | 计划反馈/新证据只触发只读影响检查、去重复核建议；Agent 无权自行改动处方，SafetyState 独立把关 | DC-055/061 |
| TP-D12 | 3D 示范、训练时间先后与计划关联不构成医学病因/疗效证据；身体反馈中的明确自述进入 BodyState | R-EX-01/02 |

## 2026-10-13 · SafetyState 一揽子决策（SAFE-D1～D8）

| ID | 已确定的业务语义 | 对应待决 |
| --- | --- | --- |
| SAFE-D1 | SafetyState 是 BodyState 唯一权威；Trainer 门禁是只读投影，TrainingPlan.status 和处方不随风险自动改变 | DC-001/006 |
| SAFE-D2 | 产品执行风险分为 advisory（提示）、restricted（局部阻断）、urgent（全训练阻断/安全提示）；并非临床严重程度分类 | DC-001/002 |
| SAFE-D3 | 范围 all_training/plan/exercise；exercise 按动作概念 ID 匹配；重叠风险取最严格有效限制，分别解除 | DC-003/062 |
| SAFE-D4 | on_expiry 仅允许明确低风险且有审核有效期的限制；其他默认用户复核，需临床评估的风险和 urgent 要求有来源的专业放行 | DC-002/004、Q-18 |
| SAFE-D5 | 更正或撤回证据不自动解除风险；单条限制按 restrictionId、来源和策略条件更新，避免并发清除新增高风险 | DC-004/063/064/065 |
| SAFE-D6 | 当前动作命中 restricted → Trainer 安全暂停；urgent → 安全终止 Session；不写已完成/跳过、不改处方 | DC-006 |
| SAFE-D7 | 普通训练限制不终止 Diagnosis Run；真正 urgent 可按已确认的 safety_blocked 零提交终结；恢复须用户发起新分析 | DC-005、INV-23 |
| SAFE-D8 | 临床红旗判定/阈值及来源准入规则需要独立专业验证；旧自由文本风险清除和整块 JSON 覆写不得视为满足本契约 | DC-001～006、IG-04 |

**保留工程待办**：PlanPatch DTO、数据库迁移、旧 Treatment/CheckIn 数据、动作库与安全规则专业审核及真实端到端测试，均不是本轮已经完成的事项。

## 2026-10-13 · Diagnosis V2 统一业务模型（DX-D5～D15）

**当前有效原则以 [Diagnosis V2](./domain/diagnosis.md) 为准。** 本文件下方的 Q-01、GAP-A/C/F 等 V1 选择保留为历史决策记录，但其 lifecycle、描述性病名、用户决定 likely/keep_as_suspected、clinical_diagnosis 枚举、endTarget/mark_resolved 等设计已经被 V2 **明确取代**，不可继续作为新实现依据。早期 DX-D1～D3 在“ConditionEntry=持续健康问题”的假设下形成；该对象定义现已撤回，真实症状经历仍归 BodyState。

| ID | 已确认的 V2 业务原则 | 权威位置 |
| --- | --- | --- |
| DX-D5 | ConditionEntry 没有通用 active/monitoring/resolved lifecycle；症状历程归 SymptomEpisode，疾病活动性不能由其自动推断 | Diagnosis §3.0/§3.6，R-DX-01/13 |
| DX-D6 | 医学来源、AI 证据评估、临床医生原始声明、用户采纳四者分离；来源声明多条并存 | Diagnosis §3.0/§3.6，R-DX-08/16 |
| DX-D7 | disposition 只保留 adopted/inactive；撤回、替代、不再适用与重新采纳原因独立审计 | Diagnosis §3.4/§3.6，R-DX-15 |
| DX-D8 | ConditionEntry 是具体具名医学疾病/综合征概念，同一概念证据更新保留 ID，实质不同医学概念新建条目 | Diagnosis §3，R-DX-01/14 |
| DX-D9 | 用户决定采纳/不采纳，不直接决定 suspected/likely，不制造临床确诊 | Diagnosis §3.2，R-DX-04/08 |
| DX-D10 | AI 证据评估只使用 suspected/likely 定性等级；证据不足可以不提案，临床声明原始不确定性独立 | Diagnosis §3.0/§3.6，R-DX-08/11 |
| DX-D11 | 新证据先进入来源权威；诊断只在用户请求分析并确认后实质更新 | Diagnosis §3.6，R-DX-06 |
| DX-D12 | 多个临床来源和不同意见并存，不以较新自动覆盖较早或伪称医学排除 | Diagnosis §3.6，R-DX-16 |
| DX-D13 | Agent 可自动发现变化、建议复核/Ask User 澄清；不自行发起正式诊断 Run | Diagnosis §3.6，R-DX-06 |
| DX-D14 | 复核提示从新证据与已评估范围派生，去重/收起，不新增 DiagnosisReviewTask | Diagnosis §3.6，R-DX-17 |
| DX-D15 | 一次 Run 内多个具名诊断提案逐项确认，全部有效决定一次原子提交，安全阻断零提交 | Diagnosis §3.2～3.3，R-DX-12 |

**综合业务边界（本轮同意）**：BodyState 通过 Fact、Observation、SymptomEpisode 和派生 ConcernGroup 组织事实；**不新增 BodyConcern**。ConditionEntry 仅用于经用户选择纳入的具体医学诊断；跨领域为多对多证据关联，支持 supporting/counter/context。V1 的旧 Q/GAP 决策属于历史，不要求删除审计；有关物理 Schema、数据兼容、隐私擦除、风险 P0 及 Run 竞态的工程细节仍另案评审。

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
