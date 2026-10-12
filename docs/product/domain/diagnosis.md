# BodySense · Diagnosis V2 领域契约

> **权威目标逻辑模型。** 本文依据 DX-D5～D15 的业务确认，修订原有 Diagnosis V1 的字段和操作语义。它不表示 Go/API/数据库或 Penpot 已经完成迁移。旧业务规则与本文冲突时，应同步迁移，而不是并行生效。

## 3. Diagnosis

**Diagnosis 是每位用户唯一的一份综合诊断聚合；ConditionEntry 是用户明确采纳的一项具体、具名的医学诊断概念。** 医学名称可以是疾病或具医学含义的综合征（例如坐骨神经痛），但不能把症状名称自动写成确定病因。AI 的疑似/较倾向判断不等于临床确诊，用户采纳也不提高医学确定程度。

- BodyState 权威保存用户的症状、活动、健康史、自测/检查观察及 SymptomEpisode；诊断证据引用它们，不复制身体事实。一个证据可以关联多个诊断，一个诊断可以引用多个证据；相关不等于因果证明。
- BodyHypothesis 只保存用户本人提出的猜测；AI 假设属于 ConditionAnalysisRun。DerivedConcernGroup 是 BodyState 的派生组织视图，不建立 BodyConcern 权威实体，不以 Concern 身份作为正式诊断的必需外键。
- 相同医学概念的合理名称规范化、证据更新可以保持同一 conditionId；不同医学诊断概念需要不同 conditionId，即使它们解释同一 Episode。不能仅凭名称、部位或共享证据自动合并身份；重要歧义先澄清。
- 诊断当前采纳状态、AI 判断、临床来源声明与症状病程是独立维度。**ConditionEntry 没有通用 lifecycle。** 症状结束、疾病活动性与管理阶段不能被一个 active/monitoring/resolved 枚举混同。

### 3.0 目标聚合与 ConditionEntry

~~~ts
type Diagnosis = {
  userId: string;                         // 每位用户一份
  currentRevision: number;                // 内部事务修订
  conditionEntries: ConditionEntry[];     // 包含 adopted 与 inactive 历史
  updatedAt: string;
};

type DiagnosisLevel = "suspected" | "likely"; // AI 证据评估，不是临床确诊或数值概率
type SourceKind = "user_suspicion" | "agent_analysis"
  | "user_reported_clinical" | "clinical_document";

type MedicalConceptRef = {
  system: string;                         // 如 ICD/SNOMED；不可编造编码
  code: string;
  display?: string;
};

type EvidenceRef = {
  evidenceType: "body_fact" | "body_observation"
    | "symptom_episode" | "document_evidence";
  evidenceId: string;
  version?: string;
};

type SourceClaimRef = {
  claimId: string;                        // 引用具版本的权威来源或结构化声明
  sourceKind: SourceKind;
  sourceId: string;
  sourceVersion?: string;
  statement: string;                      // 原始措辞或准确引用（含“考虑”等不确定性）
  assertedAt?: string;
  recordedAt: string;
  evidenceAdmissionId?: string;           // 文件类来源适用
  validityAtAssessment: "current" | "corrected" | "withdrawn" | "ineligible"; // 评估时来源快照；当前资格在来源权威查询
};

type DiagnosisEvidenceLink = {
  evidenceType: "body_fact" | "body_observation"
    | "symptom_episode" | "document_evidence";
  evidenceId: string;
  evidenceVersion?: string;
  role: "supporting" | "counter" | "context";
  rationale: string;                     // 相关性说明，不等于未经证实的病因
  analyzedByRunId: string;
};

type DiagnosisAssessment = {             // 无独立 AI 评估的临床来源条目可为空
  level: DiagnosisLevel;
  rationale: string;
  informationGaps: string[];
  differentialNotes: string[];
  basedOnRunId: string;
};

type HandlingGuidance = {
  summary: string;                        // 允许观察、检查、就医；非必然训练处方
  actions: string[];
  cautionNotes: string[];
};

type ConditionEntry = {
  conditionId: string;
  userId: string;
  medicalName: string;                    // 具名医学疾病/综合征，不是纯症状标题
  conceptRef?: MedicalConceptRef;
  qualifiers?: { regionIds?: string[]; laterality?: string; notes?: string };

  assessment?: DiagnosisAssessment;      // AI suspected/likely；不保存“clinical_diagnosis”等级
  sourceClaimRefs: SourceClaimRef[];      // 确认时的来源快照；当前资格查询原始权威
  evidenceLinks: DiagnosisEvidenceLink[];
  causeAnalysis: { summary: string; rationale: string };
  informationGaps: string[];
  handlingPlan: HandlingGuidance;

  disposition: "adopted" | "inactive";     // 只表示当前采纳关系
  latestDecisionId: string;
  lastAnalysisRunId?: string;
  createdAt: string;
  updatedAt: string;
};
~~~

**来源与确定性约束：**

1. suspected / likely 是本次 AI 对证据支持程度的定性判断，不采用未校准百分比；不表示用户选择、临床确诊或诊断排名。两个候选可以同时 suspected；若资料不足，Run 可以 insufficient_evidence 结束而不创建提案。
2. 临床诊断声明保存医生原始措辞与来源；“医生考虑 A”和“医生明确诊断 A”不能在字段映射中一律提升为“确诊”。用户转述医生诊断与有采纳文件支持的声明必须区分。临床来源本身不强行生成 AI assessment。
3. 上传报告不等于诊断声明：OCR/文件证据必须经既有证据管线准入；只有明确记载临床医生诊断的资料才能产生 clinical_document 类型的诊断声明。影像、化验数值可作为其他诊断的分析证据，但不能冒充医生诊断。
4. 新证据先存入拥有其真值的来源领域。未经正式分析与用户确认，**不能自动追加/改写 ConditionEntry 的正式证据链、AI 判断或当前来源解释**；只可生成派生的复核提示。
5. 所有 ConditionEntry 的医学名称、原因分析、信息缺口和处理方向应如实填写；不强迫制造医学名称来填满 Diagnosis。纯症状/未明确病名的记录留在 BodyState。
6. medicalName 与概念编码分离；无可核实编码时可只用医学名称。名称同义规范化可以保留 conditionId；实质不同医学诊断应新建条目，保留替代与鉴别历史。

### 3.1 正式分析审计对象

正式分析采用 **ConditionAnalysisRun → ConditionProposal → ConditionProposalDecision → DiagnosisRevision**。不可变的是创建时基础记录、提案内容、决定与追加事件；Run/Proposal 的 state 是可回放投影。

~~~ts
type ConditionAnalysisRun = {
  runId: string;
  userId: string;
  requestMessageId: string;             // 用户明确请求正式分析
  trigger: "new_analysis" | "partial_update" | "full_reanalysis";
  scope: {
    conditionIds: string[];
    evidenceRefs: EvidenceRef[];
    description: string;
  };
  basedOnBodyStateRevision: number;
  basedOnDiagnosisRevision: number;
  inputEvidenceRefs: EvidenceRef[];
  agentHypotheses: { statement: string; evidenceRefs: EvidenceRef[] }[]; // 创建快照
  startedAt: string;
  // 以下为追加事件推导的只读投影：
  evidenceRefs: EvidenceRef[];
  hypotheses: { statement: string; evidenceRefs: EvidenceRef[]; addedAt: string }[];
  proposalIds: string[];
  state: "open" | "committed" | "closed_without_change";
  outcome?: "proposals_created" | "insufficient_evidence" | "safety_blocked";
  committedDiagnosisRevision?: number;
  settledAt?: string;
};

type ConditionProposal = {
  proposalId: string;
  runId: string;
  targetConditionId?: string;           // create 时不存在
  changeKind: "create" | "update" | "set_disposition" | "update_relation";
  content: {
    proposedEntry?: Partial<ConditionEntry>; // create/update 的明确目标内容
    changeSet?: { field: string; before: unknown; after: unknown }[];
    newDisposition?: "adopted" | "inactive";
    dispositionReason?: "user_withdrawal" | "superseded_by_new_conclusion"
      | "no_longer_applicable" | "readopted";
    relationChange?: ConditionRelation;
  };
  contentFingerprint: string;           // 覆盖操作类型、目标、全部实质内容及来源版本
  evidenceRefs: EvidenceRef[];
  counterEvidenceRefs: EvidenceRef[];
  supersedesProposalId?: string;
  createdAt: string;
  // 状态为事件投影：
  state: "pending" | "decided" | "superseded" | "expired";
  expiredReason?: "timeout" | "base_revision_changed"
    | "run_closed" | "safety_blocked";
  supersededReason?: "revised_after_supplement" | "evidence_changed";
  supersededByProposalId?: string;
};

type ConditionProposalDecision = {
  decisionId: string;
  proposalId: string;
  runId: string;
  contentFingerprint: string;
  decidedEvidenceRefs: EvidenceRef[];
  decision: "accept" | "reject";
  toolCallId: string;
  decidedAt: string;
};

type ConditionRelation = {
  fromConditionId: string;
  toConditionId: string;
  kind: "differential_with" | "supersedes" | "coexisting";
  basisEvidenceRefs: EvidenceRef[];
  decisionId?: string;
};

type DispositionChange = {
  conditionId: string;
  from: "adopted" | "inactive";
  to: "adopted" | "inactive";
  reason: "user_withdrawal" | "superseded_by_new_conclusion"
    | "no_longer_applicable" | "readopted";
  decisionId: string;
  evidenceRefs: EvidenceRef[];
  changedAt: string;
};

type ConditionAnalysisEvent = {
  eventId: string;
  runId: string;
  sequence: number;                     // 同一 Run 单调递增、可重放
  at: string;
  event:
    | { kind: "run_started" }
    | { kind: "evidence_added"; evidenceRefs: EvidenceRef[]; sourceMessageId: string }
    | { kind: "hypothesis_added"; statement: string; evidenceRefs: EvidenceRef[] }
    | { kind: "proposal_created"; proposalId: string; supersedesProposalId?: string }
    | { kind: "proposal_superseded"; proposalId: string; byProposalId: string;
        reason: "revised_after_supplement" | "evidence_changed" }
    | { kind: "proposal_expired"; proposalId: string;
        reason: "timeout" | "base_revision_changed" | "run_closed" | "safety_blocked" }
    | { kind: "decision_recorded"; decisionId: string; proposalId: string }
    | { kind: "decision_rechecked"; decisionId: string; proposalId: string;
        checkedEvidenceRefs: EvidenceRef[]; result: "unaffected" | "affected" }
    | { kind: "insufficient_evidence"; summary: string }
    | { kind: "safety_blocked"; summary: string }
    | { kind: "committed"; diagnosisRevision: number }
    | { kind: "closed_without_change" };
};
~~~

- Run 只能由用户明确要求新分析、部分更新或全量重分析启动。普通对话中的初步推测，以及系统自动的相关性检查，不创建正式 Run。
- ConditionProposal 的 create 必须有具名医学诊断；update 是**同一医学概念**的内容修订，不能借名称替换改变诊断身份；不同概念新建 conditionId，并根据准确决定建立关系或停用旧条目。
- update_relation 只在目标医学身份真实存在、明确关联依据与权限成立时提交；differential_with 只表示鉴别，coexisting 不能仅凭共用 Episode 推定。
- 提案被用户明确 reject 且没有与之相关的新证据时，不得反复提出语义相同的内容指纹。新证据需以实际证据 ID/版本及关联理由识别，不能只凭任意 revision 增加突破去重。
- AI 运行开始后新形成的假设只以 hypothesis_added 事件追加，不修改启动快照，也不进入 BodyHypothesis 或直接进入 Diagnosis。
- 基础记录、Proposal 内容与 Decision 只追加；投影可重建。无提案且证据不足时写 insufficient_evidence；安全阻断用 safety_blocked 终结。一个 Run 只有一个终结事件；提交和安全阻断竞争时按原子写入次序确定胜者，安全提示不受提交结果限制。

### 3.2 用户选择与数据映射

**confirm_condition 只负责用户采纳意愿，不负责设置 AI 判断强度，也不能制造临床来源。**

| 提案类型 | 用户决定 | 提交效果 |
| --- | --- | --- |
| create | accept | 新建 adopted ConditionEntry，保留提案内已有证据评估和来源 |
| create | reject | 不创建 ConditionEntry；仅审计拒绝 |
| update | accept | 仅更新同一 conditionId 明确批准的诊断字段，不改变其采纳状态（除非另有 set_disposition 提案） |
| update | reject | 原条目及来源不变，拒绝本次变更 |
| set_disposition | accept | 严格应用提案写明的目标 adopted/inactive 与原因，记录 DispositionChange |
| set_disposition | reject | 不改变目标条目 |
| update_relation | accept | 写入提案内准确的相关诊断关系；不暗中修改任一条目的 disposition |
| update_relation | reject | 不写入关系 |
| 任意 | 继续补充或关闭 | 不产生 Decision；提案保持 pending，可恢复 |

用户采纳 suspected 不会把它变成 likely；用户采纳临床来源诊断也不代表重新进行医学确诊。用户对已有诊断的更新选择 reject 不等于撤回之前的 adopted 结论。

每个决定必须绑定用户身份、proposalId、runId、内容指纹、相关证据版本、交互工具调用与决定时间。提交必须严格遵守提案目标，禁止拿一项内容的授权提交另一项修改。

### 3.3 多提案确认与原子提交

一次 Run 可以提出多个不同的具名医学诊断、对当前条目的更新/停用和诊断关系变更。各提案**逐项确认、全部有效决定整体一次提交**：

1. 所有提案处于 decided、superseded 或 expired 时才允许结算；用户明确「其余先不处理」可以让未决定项 expired（run_closed）并结算有效决定。
2. 已决定但未提交的用户选择持久留档，不能先局部改变 Diagnosis。关闭 Chat 不等于拒绝，pending 可从原 Run 恢复；超时使未决定提案 expired，不伪装成 reject。
3. 用户继续补充时，新事实先按 BodyState 规则写入，并加入当前 Run 的证据；实质修改某提案时创建新提案并将旧提案 superseded，不就地改写旧内容。
4. 提交前，每个已决定提案（包含 reject）对其决定之后新加入的相关证据完成 recheck，记录覆盖范围；unaffected 保留决定，affected 使旧提案与决定失效并重新确认。未完成复核时 Go 拒绝提交。
5. 通过证据准入、来源声明、医学身份、授权、Safety、expectedDiagnosisRevision 等校验后，所有生效决定组成**一个原子补丁**，产生最多一次 DiagnosisRevision。无实际内容变化时 closed_without_change，不生成空修订。
6. 另有事务修改 Diagnosis 时拒绝过期补丁、保留审计并重新核验受影响提案；不能静默覆盖。BodyState 无关修订号变化不应自动被当作新的诊断反证。
7. 运行中出现需要停止分析的严重风险（含部分提案已经决定），立即进入 safety_blocked 终结：pending 提案 expired、已决定仅留档但不生效，本 Run 永不产生 DiagnosisRevision；终结后的决定/事件/提交均拒绝。普通 Trainer 训练限制不自动阻断正式诊断分析；风险处理后须由用户发起新 Run。

所有事件按同一 Run 的单调序号回放，能重现结果、证据、提案、决定、过期/取代原因和是否有效。具体事务锁/超时时长属工程契约（DC-018/050/051/070）。

### 3.4 局部更新、整体修订

- Diagnosis 只有一份。局部更新只影响被明确授权的 conditionId 和关系，不重建或删除不相关条目；一次原子事务只产生一次 DiagnosisRevision。
- DiagnosisRevision 保存 previous/current revision、精确 patch、决定 ID、Run/DirectEdit、当时 BodyState/证据快照及版本引用、提交时间与幂等键；可回放历史采纳集合，不把技术 revision 作为用户默认 UI。
- 相同医学概念、术语规范化或新证据复核：保留 conditionId，按正式提案确认内容；实质不同疾病或综合征：创建新 conditionId。两个诊断共享 Episode 不自动建立 replaces 或 coexisting。
- 当前采纳状态只有 adopted/inactive；inactive 保留曾被采纳的条目和历史，不意味着已治愈、已被医学排除。**撤回、替代、不再适用是独立历史原因，不是状态枚举。** inactive 重新采纳仍用原 conditionId，必须有针对当前内容的新分析和用户确认。
- 已纳入诊断的 sourceClaimRefs、assessment、evidenceLinks 更新也属于正式内容变更，不能因新增资料而自动悄悄修改；动态来源有效性查询与复核提示可以即时反映新证据资格，不重写旧历史快照；SourceClaimRef.validityAtAssessment 保留诊断当时所知来源资格，不作为来源当前资格的第二份权威。

### 3.5 用户直接更正与撤回

用户明确要求更正**非实质展示元数据**或撤回某个准确 ConditionEntry，可经轻量确认走 ConditionDirectEdit，不强迫启动完整分析：

~~~ts
type ConditionDirectEdit = {
  editId: string;
  userId: string;
  conditionId: string;
  editKind: "correct_metadata" | "withdraw";
  changes: { field: string; before: unknown; after: unknown }[];
  reason: string;
  requestMessageId: string;
  confirmationToolCallId: string;
  confirmedAt: string;
  basedOnDiagnosisRevision: number;
  resultingDiagnosisRevision: number;
};
~~~

- 用户明确「从诊断里撤回 X」并核对目标后，withdraw → disposition = inactive，审计原因 user_withdrawal；原诊断与 BodyState 均不删除。
- metadata 更正不能悄悄改变医学诊断身份、病因分析、来源、AI 判断、证据链或处理建议；涉及这些实质内容必须经正式分析及逐项确认。无法唯一定位条目时 Ask User。
- 用户说「右肩已经好了」属于 BodyState 的症状时间事实，不是 Diagnosis 的 mark_resolved。不得借直接编辑通道重建一个通用生命周期。
- Agent 不得主动调用直接编辑来实现自己的诊断推测；取消、关闭、超时不修改 Diagnosis。正确授权后一次直接编辑仅改变目标条目并生成一次修订。
- 与正在进行的 Run 并发时执行基线修订校验；旧 Run 的决定不得覆盖直接编辑结果。

### 3.6 结论、来源、采纳状态与复核

| 维度 | 权威内容 | 禁止混用 |
| --- | --- | --- |
| AI 判断 assessment.level | suspected / likely | 用户采纳按钮、医疗文件来源、概率百分比 |
| 来源声明 SourceClaimRef | 原始医学表述、来源类型、证据版本与有效性 | 用较新文件覆盖旧声明、把数值当临床确诊 |
| 采纳 disposition | adopted / inactive | 症状是否持续、诊断已被医学排除、撤回原因 |
| 症状过程 SymptomEpisode | unknown / ongoing / ended（BodyState 派生） | ConditionEntry 通用 lifecycle |
| 复核提示 reviewRecommended | 根据新证据与已评估范围派生 | ConditionEntry 医学状态，正式分析 Run |
| Run/Proposal state | 追加审计派生的执行状态 | 用户是否采纳医学诊断 |

**复核提示**：新 BodyState 事实或来源资格变化可以自动触发相关性检查。相关证据合并提示；用户暂不处理只收起当前提示，不视为拒绝诊断；新增实质证据可再次提醒。正式复核在新 Run 中记录已评估的证据版本，诊断无变化也可完成复核但不生成空 DiagnosisRevision。无需新增 DiagnosisReviewTask；提示的“暂不处理”仅保留最小的 Chat/UI 收起记录，不作为诊断医学状态。真正安全风险遵从 SafetyState，不等待用户授权复核。

**临床来源冲突**：独立保留医生原始措辞、时间及来源。较新的「倾向 B」不自动否定较早的「考虑 A」。Agent 可以比较与建议复核，不能自行宣布任何诊断被医学排除。

### 3.7 事件、授权与审计

- 唯一写入权威在后端业务规则，Agent 只提供可核验的候选分析与工具调用；用户对准确 Proposal/DirectEdit 作出授权后才允许提交。
- 所有真实持久化写入带 userId、授权引用、幂等键及必要的 expectedRevision，重复请求不得重复创建诊断、事件或修订。
- Run、Proposal、Decision 和 AnalysisEvent 的基础审计只追加，状态为可回放投影；与目标内容指纹不匹配的决定、过期 Run、越权 conditionId 必须拒绝。
- 医学安全阻断优先于正常分析提交；训练安全限制和 Diagnosis 的内容修订独立。
- 纠错/撤回诊断采纳不等于删除个人健康数据；源资料或派生记录真正清除、脱敏和传播受 DC-033/035 约束，未完成不能声称擦除成功。

### 3.8 新诊断契约与旧运行时对照（As-Is）

- [DiagnosisAnalysisRecord](../../../apps/api/internal/model/diagnosis_analysis.go) 仍是按一次分析形成的旧历史工件；并不等价于一个用户一份 Diagnosis + 具名 ConditionEntry + 可重放 Run/Proposal/Decision 的目标聚合。
- [DiagnosisCandidateRecord](../../../apps/api/internal/model/diagnosis_analysis.go) 的 Confidence/Assessment 等旧字段不等于本模型的 AI 证据级别与独立采纳决定。
- [DiagnosisApplicationService](../../../apps/api/internal/service/diagnosis_application_service.go) 旧的 commitHypotheses 流程把 Agent 假设写入 BodyStateHypothesis，与本模型的领域归属冲突（IG-05）。
- 旧的 description-only 条目、single source、clinical_diagnosis、用户按钮控制 likely、lifecycle 与 endTarget 必须作为兼容迁移对象处理，不能宣称当前运行时已遵循 V2。
- 具体持久表、DTO、事件迁移与回归测试应单独实施；本文只批准业务逻辑，不批准未评审的破坏性数据迁移。

### 3.9 历史、身份与待实施边界

| 场景 | V2 已确定的业务原则 | 待技术方案或跨域契约 |
| --- | --- | --- |
| 症状结束后复发 | BodyState 建立新 Episode；Diagnosis 不自动更新，同一医学概念可以继续引用多次经历 | Episode ID 演化及引用适配 DC-030/066 |
| 名称同义与不同侧别 | 同一医学概念且身份匹配才保留 conditionId；实质不同概念新建 | 稳定概念匹配与旧条目合并/拆分 DC-047/067 |
| 重新采纳 | inactive→adopted 用当前内容重新分析、明确确认，保留撤回历史 | 拒绝指纹与再采纳去重 DC-024/068 |
| 多份临床文件有不同诊断 | 声明并存，保留原始不确定性；正式诊断按需复核 | 来源声明映射、证据资格撤销 DC-048/069 |
| 多提案/多设备并发 | 确认逐项、一次原子提交，过期拒绝，严重安全阻断零提交 | Run 恢复与竞态 DC-049～051/070 |
| 删除来源资料 | 不以 soft-delete 冒充删除，不静默继续信任已失效证据 | 隐私清除及派生传播 DC-033/035 |

**文档结论**：V2 以具名医学诊断为 ConditionEntry 唯一业务定位；BodyState 为所有身体事实和真实症状病程的来源。Diagnosis 的主要变化是经过用户授权的医学结论修订，而非症状状态复制。