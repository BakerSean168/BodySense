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
│  └─ DiagnosisRevision[]
├─ ConditionAnalysisRun[]     正式分析执行审计（不可变），含 AI 假设与证据引用
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
  validUntil?: string;           // 为空表示直到满足解除条件
  liftConditions: string[];      // 如「经医生检查确认可恢复训练」
  liftedAt?: string;
  liftedReason?: string;         // 解除时记录满足了哪个条件及来源
  sourceIds: string[];           // 触发该限制的事实 / 观察 / 分析
};
~~~

- SafetyState 持有风险状态；Trainer 在开始和继续执行时检查当前有效的限制并把关（R-SAFE-03）。限制不修改 `TrainingPlan.status` 或计划内容（INV-08）。
- 范围为 `exercise` 时只拦截指定动作；`validUntil` 到期或解除条件被满足并记录后，限制不再生效。

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
    documentEvidenceIds: string[];     // source = clinical_document 时必填
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
    decisionId?: string;               // 撤回 / 不再适用对应的用户决定
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
  basedOnBodyStateRevision: number;
};
~~~

字段组合约束：

- `source` 只描述来源类型，由证据决定，**用户点击确认不能改变来源**；「临床文件记载」即 `source = clinical_document`，只能由经 [证据管线](../architecture/health-document-evidence-pipeline.md) 采纳的文件证据获得（R-PS-03）。
- `conclusionStatus = clinical_diagnosis` 只允许与 `confirmation.decision = include_clinical` 及临床来源（`clinical_document` / `user_reported_clinical`）同时出现；`agent_analysis` 与 `user_suspicion` 只能是 `likely` 或 `suspected`。
- `user_reported_clinical` 表示「用户说医生这样诊断」，不等于有文件支持；`include_clinical` 只表示用户同意纳入个人病情管理，不是对医生诊断的再次判断。
- 撤回或不再适用时条目及其历史保留，`lifecycle` 不因此改写；BodyState 中的症状事实保持不变。用户否定 AI 解释不等于医学上已被排除。

### 3.1 正式分析审计对象

旧代码中的 DiagnosisAnalysis（按问诊生成的候选诊断）名称保留给旧实现；新规格拆为三个对象（术语对照见 [术语表 §3](./glossary.md#3-penpot--旧代码名称对照)）：

~~~ts
type ConditionAnalysisRun = {          // 一次正式分析执行，不可变审计
  runId: string;
  userId: string;
  requestMessageId: string;            // 授权分析的用户请求（R-DX-02）
  scope: { conditionIds: string[]; concernIds: string[]; description: string };
  basedOnBodyStateRevision: number;
  basedOnDiagnosisRevision: number;
  evidenceIds: string[];
  agentHypotheses: { statement: string; evidenceIds: string[] }[];  // AI 假设只存在这里
  outcome: "proposals_created" | "insufficient_evidence" | "safety_blocked";
  proposalIds: string[];
  state: "open" | "committed" | "closed_without_change";
  committedDiagnosisRevision?: number;
  startedAt: string;
  settledAt?: string;
};

type ConditionProposal = {
  proposalId: string;
  runId: string;
  targetConditionId?: string;          // 为空表示新增条目
  changeKind: "create" | "update" | "end";   // end：标记缓解或不再适用
  content: Pick<ConditionEntry,
    "name" | "source" | "sourceRefs" | "conclusionStatus" | "causeAnalysis"
    | "handlingPlan" | "lifecycle" | "informationGaps">;
  contentFingerprint: string;          // 对规范化内容计算；算法由实现决定
  evidenceIds: string[];               // 本提案引用的证据
  counterEvidenceIds: string[];
  state: "pending" | "decided" | "superseded" | "expired";
  expiredReason?: "timeout" | "base_revision_changed" | "run_closed";
};

type ConditionProposalDecision = {     // 用户决定的独立审计记录（追加写入，不修改）
  decisionId: string;
  proposalId: string;
  runId: string;
  contentFingerprint: string;          // 决定针对的准确内容
  evidenceIds: string[];               // 决定时提案引用的证据
  decision: "likely" | "keep_as_suspected" | "include_clinical" | "rejected";
  toolCallId: string;
  decidedAt: string;
};
~~~

- 「继续补充」不是决定，不产生 ConditionProposalDecision；提案保持 `pending`。
- **不重复提议**：存在 `rejected` 决定时，Agent 不得再次提出相同 `contentFingerprint`（或同一目标条目上语义相同）的提案，除非本次运行引用了该决定 `evidenceIds` 之外、且与该提案相关的新证据 ID。仅 BodyState 修订号变化不算新证据。
- 历史 ConditionAnalysisRun / Proposal / Decision 不可改写；当前 Diagnosis 是单一对外聚合视图。

### 3.2 用户选择与数据映射

规则见 R-DX-02～R-DX-05、R-DX-08。写入前必须通过来源验证、证据检查、来源/结论组合约束、用户确认记录和安全策略校验。已有条目发生实质变化同样需要用户在确认工具中核对变更内容。

| 提案来源 | 用户选择 | 决定值 | 提交后的 ConditionEntry |
| --- | --- | --- | --- |
| `agent_analysis` / `user_suspicion` | 大概率是这个 | `likely` | `conclusionStatus = likely`，`disposition = adopted` |
| `agent_analysis` / `user_suspicion` | 可能是，但不确定，先保留 | `keep_as_suspected` | `conclusionStatus = suspected`，必须写明 `informationGaps` |
| `clinical_document` / `user_reported_clinical` | 纳入我的病情管理 | `include_clinical` | `conclusionStatus = clinical_diagnosis`，`source` 保持原来源 |
| 任意，新增提案 | 不是这个 / 不纳入 | `rejected` | 不创建条目；只记录决定 |
| 任意，针对已有条目 | 不是这个 | `rejected` | `disposition = withdrawn_by_user`，记录原因和时间；条目与历史保留（另见 [Q-16](./open-questions.md)） |
| `changeKind = end` | 用户同意 | 按提案内容 | `lifecycle = resolved` 或 `disposition = no_longer_applicable` |
| 任意 | 继续补充 / 关闭界面 | — | 不写入；提案保持 `pending`，可恢复 |

- **升级路径**：疑似条目可在后续正式分析中经同一确认工具改为 `likely`；`user_suspicion` / `agent_analysis` / `user_reported_clinical` 条目在出现经管线采纳的临床文件后，可经新提案把同一 `conditionId` 的 `source` 改为 `clinical_document`、`conclusionStatus` 改为 `clinical_diagnosis`，用户选择 `include_clinical` 后生效；旧来源与旧结论在修订历史中保留。
- 用户选择「大概率是」只表示用户接受把这一可能性记为「大概率」，不是临床诊断（R-DX-04）。

### 3.3 多提案确认与原子提交

一次 ConditionAnalysisRun 可以产生多个 ConditionProposal，每个提案独立决定，最终**一次性**提交（R-DX-12）：

- **提交时机**：运行中所有提案都已结算（`decided`、`superseded` 或 `expired`）时，把全部会改变 Diagnosis 的决定（`likely` / `keep_as_suspected` / `include_clinical`，以及对已有条目的 `rejected` 与 `end`）合成**一个**原子补丁，只产生一次 DiagnosisRevision；没有任何改变时运行记为 `closed_without_change`，不产生修订。
- **提交前**：已作出的决定保存在 ConditionProposalDecision 中，不会丢失，但 Diagnosis 不变。
- **关闭界面 / 离开对话 / 再次进入**：运行保持 `open`，未决定提案保持 `pending` 并可在同一运行中恢复；已决定的不需要重新选择。
- **继续补充**：只影响被补充的那个提案；新信息按 R-BS-01 写入 BodyState。Agent 若据此修订提案，旧提案记为 `superseded`，新提案在同一运行内等待决定；其他提案的已有决定保留。
- **超时**：运行的未决定提案记为 `expired`（`timeout`），随后按上述规则提交已作出的决定。超时时长见 [Q-17](./open-questions.md)。
- **用户结束本次确认**（如「其余先不处理」）：未决定提案记为 `expired`（`run_closed`），已作出的决定照常提交。
- **底版变化**：提交时 Diagnosis 当前修订与 `basedOnDiagnosisRevision` 不一致，补丁被拒绝；受影响提案记为 `expired`（`base_revision_changed`），Agent 重新核验后再展示，不静默合并。

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
| INV-06 | Diagnosis 条目的创建/实质更新需要可追溯的用户请求，以及用户对病名、原因和处理方案的确认决定（`likely` / `keep_as_suspected` / `include_clinical`，见 §3.2）；单纯请求分析不代表接受 Agent 结论 |
| INV-07 | 未经用户授权绝不改变 TrainingPlan 的当前有效内容 |
| INV-08 | 训练安全阻断与计划内容修订的权限独立：风险状态由 BodyState SafetyState 持有，Trainer 执行门禁，`TrainingPlan.status` 与内容不因此改变 |
| INV-09 | 工具、事件和网络重试具备幂等、错误提示和可恢复性 |
| INV-10 | 当前页面不暴露技术修订号、历史版本管理或后端运行编号 |
| INV-11 | `ConditionEntry.source` 只由证据来源决定，用户确认不能把条目变为 `clinical_document`；`clinical_diagnosis` 只与临床来源同时出现 |
| INV-12 | 一次 ConditionAnalysisRun 的全部生效决定在一个原子补丁中提交；提交前 Diagnosis 不变，已作出的决定不丢失 |
| INV-13 | 已被用户拒绝的提案指纹，无相关新证据 ID 时不得再次提出 |
| INV-14 | 撤回 / 不再适用只改变 `disposition`，条目与历史保留，`lifecycle` 与 BodyState 事实不被改写 |
