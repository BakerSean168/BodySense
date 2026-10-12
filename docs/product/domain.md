# BodySense · 领域模型与数据契约

**本文件定义数据契约与不变量（`INV-*`）。** 业务规则（为什么、什么场景）见 [产品需求](./requirements.md)，执行步骤见 [Agent 工作流](./agent-workflows.md)。业务数据以用户为隔离边界；当前状态是可查询的投影，内部修订用于审计、一致性与恢复。

字段均为**逻辑契约**；实际持久化结构按类型和服务边界落实，不要求照搬为单一表。

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

## 2. BodyState

~~~ts
type BodyState = {
  userId: string;
  currentRevision: number;       // 内部字段
  currentFacts: BodyFact[];
  observations: BodyObservation[];
  hypotheses: BodyHypothesis[];
  concernGroups: DerivedConcernGroup[];  // 组织投影，不单独持有身体事实
  safetyState: SafetyState;
};

type BodyFact = {
  id: string;
  statement: string;
  bodyRegionIds: string[];
  subject: "self";
  kind: "symptom" | "health_history" | "activity" | "lifestyle" | "training_feedback";
  source: { kind: "user_message" | "direct_edit" | "structured_answer"; id: string };
  validFrom?: string;
  validTo?: string;
  observedAt?: string;
  recordedAt: string;
  timePrecision: "exact" | "day" | "range" | "approximate" | "unknown";
  lifecycle: "active" | "resolved" | "historical";
  correctedFactId?: string;
  supersedesFactId?: string;
};

type BodyHypothesis = {          // 只保存用户本人提出的怀疑（R-BS-06）
  id: string;
  statement: string;             // 如「我怀疑是偏头痛」
  source: { kind: "user"; messageId: string };
  relatedFactIds: string[];
  bodyRegionIds: string[];
  recordedAt: string;
};

type SafetyState = {             // 风险状态的权威来源（R-SAFE-03）
  restrictions: SafetyRestriction[];
};

type SafetyRestriction = {
  restrictionId: string;
  riskLevel: string;             // 风险等级；取值集合待定（Q-18）
  reason: string;                // 面向用户的原因与就医提示
  scope:
    | { kind: "plan"; planId: string }
    | { kind: "exercise"; planId: string; exerciseIds: string[] };
  validFrom: string;
  validUntil?: string;           // on_expiry：到期时间，必填；其他策略：复核时点，为空表示无预定复核时点
  liftPolicy: "on_expiry" | "user_reverification" | "professional_clearance";  // 解除策略，见下文
  liftConditions: string[];      // 如「经医生检查确认可恢复训练」
  liftedAt?: string;
  liftedReason?: string;         // 解除时记录满足了哪个条件及来源
  liftBasis?:                    // 解除依据，必须与 liftPolicy 一致
    | { kind: "expiry" }                                              // 仅 on_expiry
    | { kind: "user_reverification"; checkInteractionId: string }     // 用户经复核问答重新确认症状
    | { kind: "professional_clearance";
        sourceKind: "user_message" | "clinical_document"; sourceId: string };  // 用户记录的专业放行及来源
  sourceIds: string[];           // 触发该限制的事实 / 观察 / 分析
};
~~~

- SafetyState 持有风险状态；Trainer 在开始和继续执行时检查当前有效的限制并把关（R-SAFE-03）。限制不修改 `TrainingPlan.status` 或计划内容（INV-08）。
- 范围为 `exercise` 时只拦截指定动作。
- **解除规则**（R-SAFE-04、INV-20）：
  - `on_expiry`：到达 `validUntil` 自动解除（`liftBasis.kind = expiry`）。只有这一策略允许到期自动解除。
  - `user_reverification`：到达 `validUntil` 只触发复核提示，限制继续生效；用户通过复核问答重新确认症状、结果满足 `liftConditions` 后才解除。
  - `professional_clearance`：到达 `validUntil` 同样只触发复核提示，限制继续生效；用户记录专业人员（如医生）的放行并给出来源（用户消息或经 [证据管线](../architecture/health-document-evidence-pipeline.md) 采纳的文件证据）后才解除。
  - 需要就医检查的警示类风险（红旗征象 / 建议就医）默认 `professional_clearance`。其余风险等级的取值及其默认策略见 [Q-18](./open-questions.md)。
  - Agent 不能自行解除限制，也不能放宽已设定的 `liftPolicy`；每次解除必须写入与策略一致的 `liftBasis`。

### 2.1 写入语义与证据分类

满足 R-BS-02 的用户自述，经过结构验证后直接保存，无需事前人工批准。系统必须核查来源消息、用户身份、主体归属、时间语义、字段可解释性及幂等标识。

| 分类 | 含义 | 权威边界 |
| --- | --- | --- |
| 用户自述（BodyFact） | 用户明确报告的现象、健康史或生活习惯 | 自动记录不等于临床客观核实 |
| Observation | 有方法及来源的检查、自测、测量或报告观察；报告识别值只有按 [证据管线](../architecture/health-document-evidence-pipeline.md) 规则采纳后才形成 Observation（R-PS-03） | 可查看方法与来源；未核实 / 低于阈值的值不是可信临床证据 |
| Hypothesis（BodyHypothesis） | 用户本人提出的病因怀疑或可能疾病，`source = user`，自动记录 | 不具备确诊权威；AI 提出的假设只存在于 ConditionAnalysisRun / ConditionProposal 审计，不写入 BodyState |
| Evidence | 支持或反驳假设的文献、报告、测量等 | 必须有可追溯来源 |

存在不可消除的关键歧义时，只对相关字段创建 Clarification；同一条消息的其他明确内容可以先提交（R-BS-03）。

### 2.2 时间与更正操作

| 操作 | 当前投影 | 审计和时间线 |
| --- | --- | --- |
| Add | 新增事实 | 记录来源与适用时间 |
| TemporalChange | 更新有效状态 | 保留旧状态的真实时间区间 |
| Correct | 更正错误记录 | 保留纠错痕迹，错误数据不当病程 |
| Duplicate | 无实质改变或增加证据 | 不能制造多份相同事件 |
| Clarify | 暂不写入模糊字段 | 记录待回答的问题、目标和来源 |

- 至少区分身体状态有效时间、观察时间、记录时间、原始证据形成时间。描述「最近」「去年」「一周前」时保留原始精度并使用用户时区；不能为了填充字段而编造日期（R-BS-05）。
- 一个语义完整的事务可以增删改多个 BodyFact，但只形成一次 BodyStateRevision。

## 3. Diagnosis

~~~ts
type Diagnosis = {
  userId: string;                      // 每位用户唯一
  currentRevision: number;             // 仅内部使用
  conditionEntries: ConditionEntry[];
  crossConditionNotes: string[];
  safetySummary: SafetySummary;
  updatedAt: string;
};

type ConditionEntry = {
  conditionId: string;                 // 同一病情长期稳定
  name: string;                        // 可以是描述性健康问题，不必是疾病名（R-DX-11）

  // ① 结论来自哪里（R-DX-08）
  source:
    | "clinical_document"              // 经证据管线采纳的临床文件
    | "user_reported_clinical"         // 用户转述医生的诊断，无文件支持
    | "agent_analysis"                 // Agent 正式分析得出
    | "user_suspicion";                // 源自用户本人的怀疑（BodyHypothesis）
  sourceRefs: {
    documentEvidenceIds: string[];     // source = clinical_document 时必填，且须为明确记载该临床诊断的文件证据（INV-15）
    factIds: string[];                 // 如用户转述诊断的 BodyFact
    hypothesisIds: string[];           // source = user_suspicion 时必填
  };

  // ② 结论强度
  conclusionStatus:
    | "likely"                         // 用户选择「大概率是这个」
    | "suspected"                      // 用户选择「可能是，先保留」
    | "clinical_diagnosis";            // 仅 source 为 clinical_document / user_reported_clinical 时允许

  causeAnalysis: {
    summary: string;                   // 可以明确写「原因不明 / 证据不足」（R-DX-11）
    supportingEvidenceIds: string[];
  };
  handlingPlan: {
    summary: string;                   // 处理方向：可以是观察、进一步检查、就医，不一定是训练处方（R-DX-11）
    actions: string[];
    cautionNotes: string[];
  };

  // ③ 用户确认记录（R-DX-03、R-DX-04）
  confirmation: {
    decision: "likely" | "keep_as_suspected" | "include_clinical";
    decisionId: string;                // → ConditionProposalDecision
    proposalId: string;
    toolCallId: string;                // Chat 病情确认工具调用
    decidedAt: string;
    scope: "name_cause_handling";
  };

  // ④ 两个独立维度（R-DX-01）
  lifecycle: "active" | "monitoring" | "resolved";      // 病情的真实病程
  disposition: {                                       // 该解释是否仍被采纳
    state: "adopted" | "withdrawn_by_user" | "no_longer_applicable";
    reason?: string;                   // 撤回 / 不再适用时必填
    changedAt: string;
    decisionId?: string;               // 对应的用户决定：撤回 → ConditionDirectEdit（§3.5）；不再适用 → `endTarget = mark_no_longer_applicable` 的 `end` 提案上的 `accept_end` 决定（ConditionProposalDecision）
  };

  relatedConcernIds: string[];
  relatedBodyRegionIds: string[];
  basisFactIds: string[];
  basisObservationIds: string[];
  supportingEvidenceIds: string[];
  counterEvidenceIds: string[];
  analysisSummary: string;
  informationGaps: string[];           // conclusionStatus = suspected 时必填
  lastAnalysisRunId: string;
  lastDirectEditId?: string;           // 最近一次用户直接编辑（§3.5）
  basedOnBodyStateRevision: number;
};
~~~

字段组合约束：

- `source` 只描述来源类型，由证据决定，**用户点击确认不能改变来源**；「临床文件记载」即 `source = clinical_document`，只能由经 [证据管线](../architecture/health-document-evidence-pipeline.md) 采纳的文件证据获得（R-PS-03）。
- `conclusionStatus = clinical_diagnosis` 只允许与 `confirmation.decision = include_clinical` 及临床来源（`clinical_document` / `user_reported_clinical`）同时出现；`agent_analysis` 与 `user_suspicion` 只能是 `likely` 或 `suspected`。
- **临床文件证据 ≠ 临床诊断**：`source = clinical_document` 且 `conclusionStatus = clinical_diagnosis` 只在被采纳的文件证据**明确记载由临床医生作出的该诊断**时成立（如诊断证明、病历或出院小结中的诊断）。化验值、影像测量值等只是支持某个假设的文件证据，只能作为 `agent_analysis` 提案的证据（`evidenceIds` / `supportingEvidenceIds`），不能使条目来源变为 `clinical_document`，也不能把结论升级为 `clinical_diagnosis`（INV-15）。
- `user_reported_clinical` 表示「用户说医生这样诊断」，不等于有文件支持；`include_clinical` 只表示用户同意纳入个人病情管理，不是对医生诊断的再次判断。
- 撤回或不再适用时条目及其历史保留，`lifecycle` 不因此改写；BodyState 中的症状事实保持不变。用户否定 AI 解释不等于医学上已被排除。

### 3.1 正式分析审计对象

旧代码中的 DiagnosisAnalysis（按问诊生成的候选诊断）名称保留给旧实现；新规格拆为三个对象（术语对照见 [术语表 §3](./glossary.md#3-penpot--旧代码名称对照)），审计事件由 ConditionAnalysisEvent 追加记录：

~~~ts
type ConditionAnalysisRun = {          // 一次正式分析执行 = 基础记录 + 由 ConditionAnalysisEvent 推导的投影
  // —— 基础记录：创建运行时写入一次，之后不修改 ——
  runId: string;
  userId: string;
  requestMessageId: string;            // 授权分析的用户请求（R-DX-02）
  trigger: "new_analysis" | "partial_update" | "full_reanalysis";  // 用户请求的分析类型（R-DX-02）
  scope: { conditionIds: string[]; concernIds: string[]; description: string };
  basedOnBodyStateRevision: number;
  basedOnDiagnosisRevision: number;
  inputEvidenceIds: string[];          // 运行开始时的输入证据
  agentHypotheses: { statement: string; evidenceIds: string[] }[];  // 创建时的 AI 假设快照；之后形成的假设由 hypothesis_added 追加
  startedAt: string;

  // —— 投影：完全由事件推导（映射见下文「投影字段来源」） ——
  evidenceIds: string[];               // inputEvidenceIds + 各 evidence_added
  hypotheses: { statement: string; evidenceIds: string[]; addedAt: string }[];
                                       // agentHypotheses 快照（addedAt = startedAt）+ 各 hypothesis_added；AI 假设只存在于此（及提案审计）
  outcome?: "proposals_created" | "insufficient_evidence" | "safety_blocked";  // 结算前且尚无提案时为空；安全阻断可在已有提案后发生（§3.3）
  proposalIds: string[];
  state: "open" | "committed" | "closed_without_change";
  committedDiagnosisRevision?: number;
  settledAt?: string;
};

type ConditionProposal = {             // 基础记录 + 投影
  // —— 基础记录：创建提案时写入一次，之后不修改 ——
  proposalId: string;
  runId: string;
  targetConditionId?: string;          // 为空表示新增条目
  changeKind: "create" | "update" | "end";   // end：结束已有条目，结束结果由 content.endTarget 明确指定
  content: Pick<ConditionEntry,
    "name" | "source" | "sourceRefs" | "conclusionStatus" | "causeAnalysis"
    | "handlingPlan" | "lifecycle" | "informationGaps"> & {
    endTarget?: "mark_resolved"                  // → lifecycle = resolved
              | "mark_no_longer_applicable";     // → disposition.state = no_longer_applicable
                                         // changeKind = end 时必填；create / update 时必须为空
    endReason?: string;                  // changeKind = end 时必填：为什么建议结束
  };
  contentFingerprint: string;          // 对规范化内容（含 endTarget）计算；算法由实现决定
  evidenceIds: string[];               // 本提案引用的证据
  counterEvidenceIds: string[];
  supersedesProposalId?: string;       // 本提案取代的旧提案
  createdAt: string;

  // —— 投影：完全由事件推导 ——
  state: "pending" | "decided" | "superseded" | "expired";
  expiredReason?: "timeout" | "base_revision_changed" | "run_closed" | "safety_blocked";
  supersededReason?: "revised_after_supplement" | "evidence_changed";  // 见 §3.3
  supersededByProposalId?: string;
};

type ConditionProposalDecision = {     // 用户决定的独立审计记录（追加写入，不修改）
  decisionId: string;
  proposalId: string;
  runId: string;
  contentFingerprint: string;          // 决定针对的准确内容
  evidenceIds: string[];               // 决定时提案引用的证据
  decision: "likely" | "keep_as_suspected" | "include_clinical"   // 仅用于 create / update 提案
    | "accept_end"                     // 仅用于 end 提案：同意提案写明的 endTarget
    | "rejected";                      // 任意提案
  toolCallId: string;
  decidedAt: string;
};

type ConditionAnalysisEvent = {        // 追加写入的审计事件：写入后不修改、不删除
  eventId: string;
  runId: string;
  at: string;
  event:
    | { kind: "run_started" }            // 运行基础记录已写入；state → open
    | { kind: "evidence_added"; evidenceIds: string[]; sourceMessageId: string }
    | { kind: "hypothesis_added"; statement: string; evidenceIds: string[] }  // 运行开始后形成的 AI 假设；时间即 at
    | { kind: "insufficient_evidence"; summary: string }  // 无提案结束：证据不足（R-DX-10）
    | { kind: "safety_blocked"; summary: string }         // 终结：需停止分析的严重风险（R-DX-10，§3.3）；可在已有提案后写入
    | { kind: "proposal_created"; proposalId: string; supersedesProposalId?: string }
    | { kind: "proposal_superseded"; proposalId: string; byProposalId: string;
        reason: "revised_after_supplement" | "evidence_changed" }
    | { kind: "proposal_expired"; proposalId: string;
        reason: "timeout" | "base_revision_changed" | "run_closed" | "safety_blocked" }
    | { kind: "decision_recorded"; decisionId: string; proposalId: string }
    | { kind: "decision_rechecked"; decisionId: string; proposalId: string;
        checkedEvidenceIds: string[]; result: "unaffected" | "affected" }  // §3.3
    | { kind: "committed"; diagnosisRevision: number }
    | { kind: "closed_without_change" };
};
~~~

- 「继续补充」不是决定，不产生 ConditionProposalDecision；提案保持 `pending`。
- **结束提案必须写明结束目标**：`changeKind = end` 时 `targetConditionId` 必填，`content.endTarget` 与 `content.endReason` 必填，`content` 其余字段为目标条目的当前内容（供用户核对）；`endTarget = mark_resolved` 时 `content.lifecycle = resolved`，`endTarget = mark_no_longer_applicable` 时 `content.lifecycle` 保持目标条目当前值（INV-14）。`endTarget` 计入 `contentFingerprint`，同一条目上仅 `endTarget` 不同的两个结束提案指纹不同、语义也不同。
- **不重复提议**：存在 `rejected` 决定时，Agent 不得再次提出相同 `contentFingerprint`（或同一目标条目上语义相同）的提案，除非本次运行引用了该决定 `evidenceIds` 之外、且与该提案相关的新证据 ID。仅 BodyState 修订号变化不算新证据。以 `safety_blocked` 终结的运行中的决定只留档、不生效（INV-23），不作为本规则的依据。
- **AI 假设只追加、无权威**：运行开始后 Agent 新形成的假设（如因 `evidence_added` 引出）以 `hypothesis_added` 事件追加记录内容、证据 ID 与时间（`at`），不修改基础记录中的 `agentHypotheses` 快照；其 `evidenceIds` 须属于写入时该运行的 `evidenceIds`。AI 假设任何时候都不写入 BodyState（INV-02），也不直接进入 Diagnosis——只有经用户决定并提交的提案才改变 Diagnosis。
- **不可变的是基础记录与审计事件**（INV-16）：
  - ConditionAnalysisRun 基础记录：`runId`、`userId`、`requestMessageId`、`trigger`、`scope`、`basedOnBodyStateRevision`、`basedOnDiagnosisRevision`、`inputEvidenceIds`、`agentHypotheses` 快照、`startedAt`，创建运行时写入；
  - ConditionProposal 基础记录：`proposalId`、`runId`、`targetConditionId`、`changeKind`、`content`（含 `endTarget`、`endReason`）、`contentFingerprint`、`evidenceIds`、`counterEvidenceIds`、`supersedesProposalId`、`createdAt`，创建提案时写入；修订提案内容即创建新提案并取代旧提案；
  - ConditionProposalDecision 整条记录，以及每个 ConditionAnalysisEvent。
  以上写入后不修改、不删除。
- **状态是投影**：投影字段只由基础记录与事件推导，可随新事件更新；更新投影不改写已写入的记录或事件。投影字段来源：

  | 投影字段 | 推导来源 |
  | --- | --- |
  | Run `state` | `run_started` → `open`；`committed` → `committed`；`closed_without_change`、`insufficient_evidence`、`safety_blocked` → `closed_without_change` |
  | Run `outcome` | 有 `safety_blocked` → `safety_blocked`（无论此前是否有提案）；有 `insufficient_evidence` → `insufficient_evidence`；否则有 `proposal_created` → `proposals_created`；皆无时为空 |
  | Run `evidenceIds` | `inputEvidenceIds` ∪ 各 `evidence_added.evidenceIds` |
  | Run `hypotheses` | `agentHypotheses` 快照（`addedAt = startedAt`），再按事件顺序追加各 `hypothesis_added`（`addedAt = at`） |
  | Run `proposalIds` | 各 `proposal_created.proposalId` |
  | Run `committedDiagnosisRevision` | `committed.diagnosisRevision` |
  | Run `settledAt` | 终结事件（`committed`、`closed_without_change`、`insufficient_evidence`、`safety_blocked`）的 `at` |
  | Proposal `state` | `proposal_created` → `pending`；`decision_recorded` → `decided`；`proposal_superseded` → `superseded`；`proposal_expired` → `expired` |
  | Proposal `expiredReason` | `proposal_expired.reason` |
  | Proposal `supersededReason`、`supersededByProposalId` | `proposal_superseded.reason`、`.byProposalId` |
  | 决定是否仍生效 | `decision_recorded` 后生效；对应 `decision_rechecked.result = affected` 后失效（§3.3）；运行以 `safety_blocked` 终结时，该运行全部决定失效（INV-23） |

- **事件顺序约束**（由 Go 后端强制，不依赖 Agent 提示）：
  - `run_started` 是运行的第一个事件；
  - 终结事件（`committed`、`closed_without_change`、`insufficient_evidence`、`safety_blocked`）只能有一个；终结后该运行的任何事件（含决定、复核、`hypothesis_added`）与补丁 / 提交请求一律被拒绝；
  - `insufficient_evidence` 只能在运行尚未产生任何 `proposal_created` 时写入；
  - `safety_blocked` 可在运行 `open` 期间任何时刻写入，包括已有提案、部分或全部已决定但尚未提交时。已有提案时，在同一原子事务中先为每个 `pending` 提案写入 `proposal_expired`（`safety_blocked`），再写入 `safety_blocked`；已决定提案保持 `decided`，决定留档但不生效；不写入 `committed`，不产生 DiagnosisRevision（§3.3「运行中安全阻断」、INV-23）；
  - **终结竞争**：同一运行的终结写入串行化（如对运行事件序号使用乐观锁），先写入的终结事件胜出，另一个被拒绝：`safety_blocked` 先写入时提交被拒、不产生修订；`committed` 先写入时 `safety_blocked` 被拒，已提交的修订保留，安全提示仍按 [Agent 工作流 §5](./agent-workflows.md#5-安全处理) 照常展示；
  - **重复阻断幂等**：运行已以 `safety_blocked` 终结时，重复的 `safety_blocked` 请求（网络重试或 Agent 重复调用）返回已有结果，不追加事件。
- **可回放**：按 `at`（同一时刻按写入顺序）依次回放基础记录与 ConditionAnalysisEvent，必须重现每个 Run / Proposal 的全部投影字段（含 `hypotheses`）与决定是否仍生效；不存在只能由投影本身保存、无法从事件推导的状态（INV-22）。
- 当前 Diagnosis 是单一对外聚合视图。

### 3.2 用户选择与数据映射

规则见 R-DX-02～R-DX-05、R-DX-08。写入前必须通过来源验证、证据检查、来源/结论组合约束、用户确认记录和安全策略校验。已有条目发生实质变化同样需要用户在确认工具中核对变更内容。

| 提案来源 | 用户选择 | 决定值 | 提交后的 ConditionEntry |
| --- | --- | --- | --- |
| `agent_analysis` / `user_suspicion` | 大概率是这个 | `likely` | `conclusionStatus = likely`，`disposition = adopted` |
| `agent_analysis` / `user_suspicion` | 可能是，但不确定，先保留 | `keep_as_suspected` | `conclusionStatus = suspected`，必须写明 `informationGaps` |
| `clinical_document` / `user_reported_clinical` | 纳入我的病情管理 | `include_clinical` | `conclusionStatus = clinical_diagnosis`，`source` 保持原来源 |
| 任意，新增提案 | 不是这个 / 不纳入 | `rejected` | 不创建条目；只记录决定 |
| 任意，针对已有条目（`update` / `end`） | 不是这个 / 不纳入 | `rejected` | 只拒绝本次变更：条目内容、`conclusionStatus`、`lifecycle`、`disposition` 均不变；决定照常留档（用于不重复提议） |
| 任意，`changeKind = end`，`endTarget = mark_resolved` | 同意结束（标记为已缓解） | `accept_end` | `lifecycle = resolved`；`disposition`、`conclusionStatus`、`confirmation` 及其他内容不变 |
| 任意，`changeKind = end`，`endTarget = mark_no_longer_applicable` | 同意结束（不再适用） | `accept_end` | `disposition.state = no_longer_applicable`，`reason = content.endReason`，`changedAt` 为提交时间，`decisionId` 指向该决定；`lifecycle` 及其他内容不变（INV-14） |
| 任意 | 继续补充 / 关闭界面 | — | 不写入；提案保持 `pending`，可恢复 |

- **结束提案只应用被批准的结束目标**：提交时按决定所针对内容（`contentFingerprint`）中的 `endTarget` 写入，不得改为另一种结束结果，也不得同时改写其他字段；`accept_end` 只对 `end` 提案有效，`likely` / `keep_as_suspected` / `include_clinical` 不能用于 `end` 提案（INV-21）。
- **不再适用**（`disposition = no_longer_applicable`）只来自被 `accept_end` 的 `mark_no_longer_applicable` 结束提案；**标记缓解**（`lifecycle = resolved`）可来自被 `accept_end` 的 `mark_resolved` 结束提案，也可来自用户明确请求的直接编辑（[§3.5](#35-用户直接更正撤回与标记缓解) `mark_resolved`）。
- **撤回整条条目**（`disposition = withdrawn_by_user`）只来自用户的明确请求，经 [§3.5](#35-用户直接更正撤回与标记缓解) 直接编辑路径；对提案选「不是这个」不会撤回已有条目（INV-18）。
- **升级路径**：疑似条目可在后续正式分析中经同一确认工具改为 `likely`；`user_suspicion` / `agent_analysis` / `user_reported_clinical` 条目在出现经管线采纳、且明确记载该诊断的临床文件后，可经新提案把同一 `conditionId` 的 `source` 改为 `clinical_document`、`conclusionStatus` 改为 `clinical_diagnosis`，用户选择 `include_clinical` 后生效；旧来源与旧结论在修订历史中保留。
- 用户选择「大概率是」只表示用户接受把这一可能性记为「大概率」，不是临床诊断（R-DX-04）。

### 3.3 多提案确认与原子提交

一次 ConditionAnalysisRun 可以产生多个 ConditionProposal，每个提案独立决定，最终**一次性**提交（R-DX-12）：

- **提交时机**：运行仍为 `open`（未被安全阻断或以其他方式终结），运行中所有提案都已结算（`decided`、`superseded` 或 `expired`），且每个已决定提案都已完成下文的新证据复核时，把全部会改变 Diagnosis 的决定（`likely` / `keep_as_suspected` / `include_clinical`，以及结束提案上的 `accept_end`；`rejected` 不改变 Diagnosis）合成**一个**原子补丁，只产生一次 DiagnosisRevision；没有任何改变时运行记为 `closed_without_change`，不产生修订。
- **提交前**：已作出的决定保存在 ConditionProposalDecision 中，不会丢失，但 Diagnosis 不变。
- **关闭界面 / 离开对话 / 再次进入**：运行保持 `open`，未决定提案保持 `pending` 并可在同一运行中恢复；已决定的不需要重新选择。
- **继续补充**：被补充的提案不作决定；新信息按 R-BS-01 写入 BodyState，并作为新证据加入本运行（`evidence_added`）。Agent 若据此修订该提案，旧提案记为 `superseded`（`revised_after_supplement`），新提案在同一运行内等待决定。其他提案的已有决定按下条复核。
- **新证据复核已决定提案**：运行中加入新证据后（继续补充，或用户在同一确认过程中提供的其他新信息 / 资料），提交前必须把每个已决定提案（含 `rejected`）与其决定之后加入本运行的全部证据 ID 逐一复核，并记录 `decision_rechecked`（INV-17）：
  - `unaffected`：新证据与该提案无关，也不改变其内容指纹——原决定照常生效；
  - `affected`：新证据与该提案相关（应列入其证据或反证）或会改变其内容指纹——该提案记为 `superseded`（`evidence_changed`），原决定保留为审计但不再生效；Agent 在同一运行内重新提出提案（新证据 ID 满足 §3.1 不重复提议的例外），用户须重新决定；
  - 复核由 Agent 执行，结果与覆盖的证据 ID 留档；提交时 Go 业务规则校验覆盖完整性：任何已决定提案存在未复核的新证据 ID 时补丁不得提交，受影响的决定绝不未经复核提交。
- **超时**：运行的未决定提案记为 `expired`（`timeout`），随后按上述规则提交已作出的决定。超时时长见 [Q-17](./open-questions.md)。
- **用户结束本次确认**（如「其余先不处理」）：未决定提案记为 `expired`（`run_closed`），已作出的决定照常提交。
- **底版变化**：提交时 Diagnosis 当前修订与 `basedOnDiagnosisRevision` 不一致，补丁被拒绝；受影响提案记为 `expired`（`base_revision_changed`），Agent 重新核验后再展示，不静默合并。
- **运行中安全阻断**（R-DX-10、R-SAFE-01）：
  - 只有需要停止分析的严重风险（警示 / 需尽快就医类，即红旗信号）才中断运行：立即展示医学安全提示，停止该运行的后续确认（不再展示或记录后续提案的决定）。只影响训练执行的普通限制（SafetyState 中仅由 Trainer 把关的限制）不中断正式分析。
  - 已有提案的运行同样以 `safety_blocked` 终结：未决定提案记为 `expired`（`safety_blocked`）；已决定提案与决定留档供审计，但永不生效。
  - Diagnosis 不变：该运行不产生 DiagnosisRevision，也没有部分提交。
  - 风险得到适当处理后，由用户重新发起正式分析（新的 ConditionAnalysisRun）；被阻断的运行不自动恢复，其提案与决定不被新运行继承，新运行的提案须用户重新决定。
  - 由 Go 后端强制：对投影 `outcome = safety_blocked` 的运行，任何补丁 / 提交以及终结事件之后的任何事件都被拒绝；提交与阻断的竞争、重复阻断按 §3.1「事件顺序约束」处理（INV-23）。

### 3.4 局部更新、整体修订

~~~text
Diagnosis current revision R
    ├─ A  unchanged
    ├─ B  replace content after authorized, verified, user-confirmed analysis
    └─ C  unchanged
             |
             v
Diagnosis current revision R+1
    ├─ A  same stable identity/content
    ├─ B  same stable identity / updated analysis
    └─ C  same stable identity/content
~~~

- 一次原子事务可更新、增加或结束一个或多个病情条目；每个成功事务整体修订只增加一次，与涉及的条目数量无关。
- 只更新明确在补丁范围内的 ConditionEntry，其他条目不得重建或删除；空变更不制造新修订。
- 每个修订保存精确的 BodyState 输入快照/修订、ConditionAnalysisRun 与有效证据引用。
- 新身体事实不自动改变 Diagnosis；相关条目可标记复核建议，等待用户请求后重新分析（R-DX-06）。
- 病情缓解、不再适用或被用户撤回时，通过 `lifecycle` / `disposition` 明确表达，不能悄悄删除导致当前记录自相矛盾。

### 3.5 用户直接更正、撤回与标记缓解

规则见 R-DX-13。用户明确、直接地要求更正或撤回某个病情条目、或把它标记为已缓解时，不启动正式分析（不创建 ConditionAnalysisRun），而是走独立、可审计的直接编辑路径：

~~~ts
type ConditionDirectEdit = {           // 用户决定审计事件：追加写入，不修改
  editId: string;
  userId: string;                      // 谁
  conditionId: string;
  editKind: "correct" | "withdraw" | "mark_resolved";
  changes: { field: string; before: unknown; after: unknown }[];  // 准确的变更内容
  reason: string;                      // 用户给出的原因；未说明时记录用户原始请求
  requestMessageId: string;            // 用户的明确请求
  confirmationToolCallId: string;      // 轻量确认，如「确认撤回『X』？」
  confirmedAt: string;                 // 何时
  basedOnDiagnosisRevision: number;
  resultingDiagnosisRevision: number;
};
~~~

| 用户请求 | `editKind` | 确认后写入 |
| --- | --- | --- |
| 「把肩峰撞击从诊断里去掉」 | `withdraw` | `disposition = withdrawn_by_user`，`reason`、`changedAt` 必填，`decisionId` 指向该 `editId`；条目与历史保留，`lifecycle` 不变（INV-14） |
| 「右肩已经好了」 | `mark_resolved` | `lifecycle = resolved`，`disposition` 不变；该陈述同时作为身体事实按 R-BS-01 写入 BodyState |
| 「病名里的左右写反了」 | `correct` | 只改用户明确指出的字段 |

- 只能由用户的明确请求触发。Agent 自己认为应更正、撤回或结束某个条目时，只能建议重新分析，经正式分析与确认工具提出（R-DX-06），不得使用此路径（INV-19）。
- 用户确认前不写入；取消或关闭确认不产生修改。确认绑定准确的条目与变更内容；请求可能对应多个条目时先用 ask_user 澄清。
- 每次确认的直接编辑只改目标条目，产生一次 DiagnosisRevision（INV-04、INV-05）。
- 更正不能改变 `source`、`conclusionStatus` 或 `confirmation`（INV-11）；需要重新判断原因或处理方案时，应发起正式分析。
- 存在进行中的运行时，直接编辑会改变其底版，该运行提交时按 §3.3「底版变化」处理。

## 4. TrainingPlan

~~~ts
type TrainingPlan = {
  planId: string;
  userId: string;
  title: string;
  goal: string;
  relatedConditionIds: string[];
  relatedConcernIds: string[];
  status: "active" | "paused" | "completed";   // 计划自身状态；安全限制不改写它
  currentAcceptedRevisionId: string;
  currentContent: {
    sessions: PlanSessionTemplate[];  // 单次课计划只有一个模板；A/B 日等多课计划有多个
    frequency: { sessionsPerWeek?: number; description?: string };
    constraints: string[];
  };
};

type PlanSessionTemplate = {          // 计划中「一次课」的模板，不是实际执行
  templateId: string;
  title?: string;                     // 如「A 日 · 下肢」
  stages: {
    phase: "warmup" | "main" | "cooldown";
    exercises: {
      exerciseId: string;
      name: string;
      sets: number;
      repetitions?: number;
      durationSeconds?: number;
      restSeconds?: number;
    }[];
  }[];
};

type TrainingSessionLog = {           // 一次实际执行的训练记录（Trainer 产生）
  sessionLogId: string;
  planId: string;
  planRevisionId: string;
  templateId: string;
  date: string;                       // 用户时区的训练日期
  startedAt: string;
  endedAt?: string;
  state: "in_progress" | "paused" | "ended";
  exerciseRecords: {
    exerciseId: string;
    setNumber: number;
    outcome: "completed" | "skipped";
    actualRepetitions?: number;
    actualDurationSeconds?: number;
    recordedAt: string;               // 来自用户的明确完成 / 跳过操作或陈述
  }[];
  feedback?: string;                  // 主观反馈；身体变化另按 R-BS-01 写入 BodyState
};

type PlanChangeProposal = {
  proposalId: string;
  planId: string;
  basedOnRevisionId: string;
  initiatedBy: "user" | "agent";
  reason: string;
  proposedChanges: PlanPatch[];
  state: "pending" | "accepted" | "rejected" | "expired";
  expiredReason?: "timeout" | "base_revision_changed";
  userAuthorizationId?: string;
};
~~~

- 每份 Plan 独立修订；每个动作与训练记录关联明确的 planId（R-TP-01）。
- `currentContent` 必须能直接呈现 R-TP-02 要求的完整处方：每个 PlanSessionTemplate 按热身 → 正式训练 → 必要放松排列，逐个动作名称、组数、次数/时长、必要休息，加上整体训练频率。
- PlanSessionTemplate 是计划内容；TrainingSessionLog 是执行事实，二者不混用。执行记录不回写模板。
- TrainingSessionLog 按 planId、日期、exerciseId 定位；只有用户明确的完成 / 跳过操作或陈述才形成 `exerciseRecords`，不根据点击、浏览或演示播放推断完成（R-EX-04）。
- 同一份计划的动作/组次/频率/排程只有一个当前有效权威副本。
- 用户明确请求的修改（R-TP-03）直接提交并保留该请求的授权来源。
- Agent 建议（R-TP-04）仅创建 Proposal；接受时检查对应方案、用户身份、底版修订和决定状态，才允许提交。Proposal 状态：
  - 关闭界面或离开对话：保持 `pending`，可恢复；
  - 用户明确选择「暂不修改」：`rejected`；
  - 超时或计划底版修订已变化：`expired`（超时时长见 [Q-17](./open-questions.md)）。
  只有 `accepted` 产生内容修订。
- 安全执行限制由 BodyState `SafetyState` 持有，不等同于修改计划内容（R-SAFE-02、R-SAFE-03）。
- TrainingSessionLog 和身体变化回写只触发影响检查，不直接修改计划（R-TP-05）。

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
