# BodySense · Diagnosis 领域契约

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

- `source` 只描述来源类型，由证据决定，**用户点击确认不能改变来源**；「临床文件记载」即 `source = clinical_document`，只能由经 [证据管线](../../architecture/health-document-evidence-pipeline.md) 采纳的文件证据获得（R-PS-03）。
- `conclusionStatus = clinical_diagnosis` 只允许与 `confirmation.decision = include_clinical` 及临床来源（`clinical_document` / `user_reported_clinical`）同时出现；`agent_analysis` 与 `user_suspicion` 只能是 `likely` 或 `suspected`。
- **临床文件证据 ≠ 临床诊断**：`source = clinical_document` 且 `conclusionStatus = clinical_diagnosis` 只在被采纳的文件证据**明确记载由临床医生作出的该诊断**时成立（如诊断证明、病历或出院小结中的诊断）。化验值、影像测量值等只是支持某个假设的文件证据，只能作为 `agent_analysis` 提案的证据（`evidenceIds` / `supportingEvidenceIds`），不能使条目来源变为 `clinical_document`，也不能把结论升级为 `clinical_diagnosis`（INV-15）。
- `user_reported_clinical` 表示「用户说医生这样诊断」，不等于有文件支持；`include_clinical` 只表示用户同意纳入个人病情管理，不是对医生诊断的再次判断。
- 撤回或不再适用时条目及其历史保留，`lifecycle` 不因此改写；BodyState 中的症状事实保持不变。用户否定 AI 解释不等于医学上已被排除。

### 3.1 正式分析审计对象

旧代码中的 DiagnosisAnalysis（按问诊生成的候选诊断）名称保留给旧实现；新规格拆为三个对象（术语对照见 [术语表 §3](../glossary.md#3-penpot--旧代码名称对照)），审计事件由 ConditionAnalysisEvent 追加记录：

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
  - **终结竞争**：同一运行的终结写入串行化（如对运行事件序号使用乐观锁），先写入的终结事件胜出，另一个被拒绝：`safety_blocked` 先写入时提交被拒、不产生修订；`committed` 先写入时 `safety_blocked` 被拒，已提交的修订保留，安全提示仍按 [Agent 工作流 §5](../agent-workflows.md#5-安全处理) 照常展示；
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
- **超时**：运行的未决定提案记为 `expired`（`timeout`），随后按上述规则提交已作出的决定。超时时长见 [Q-17](../open-questions.md)。
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

### 3.6 病情状态索引与尚未解决的生命周期

| 独立维度 | 已确定的枚举 | 明确的转换与权限 |
| --- | --- | --- |
| 来源 source | clinical_document / user_reported_clinical / agent_analysis / user_suspicion | 临床来源须有相应证据；用户选择不能制造临床事实 |
| 结论 conclusionStatus | likely / suspected / clinical_diagnosis | 新结论通过正式提案 + confirm_condition；疑似升级仍保留旧结论历史 |
| 病程 lifecycle | active / monitoring / resolved | mark_resolved 结束提案被接受或用户直接确认缓解才修改 |
| 解释采纳 disposition | adopted / withdrawn_by_user / no_longer_applicable | 用户明确请求可撤回，正式结束提案可标不再适用；拒绝一次更新不撤回条目 |
| 分析运行 state | open / committed / closed_without_change，另有 outcome | 由追加事件推导，严重安全阻断不产生 DiagnosisRevision |
| 提案 state | pending / decided / superseded / expired | 关闭 Chat 不等于拒绝；新证据影响已决定结果时取代并重新征询 |

**复发与再次采纳未定**：病情从 resolved 后再次出现，复用 conditionId、引入 Episode 还是建立相关的新条目，见 DC-021；已撤回/不再适用的解释重新采纳，见 DC-024。未决前不得自动重置 lifecycle/disposition，更不能覆盖旧的用户决定和证据。

### 3.7 事件、授权与审计

新增/更新/结束病情：用户请求正式分析 → Run 固定 BodyState/Diagnosis 底版 → Agent 提案 → 用户对每条提案决定 → 复核新证据及风险 → 一次原子补丁 → 一个 DiagnosisRevision。每个阶段引用准确的 runId/proposalId/contentFingerprint/evidenceIds；拒绝/过期/安全阻断不能偷偷提交。

直接更正/撤回/缓解只在用户明确请求后以轻量确认提交，记录 ConditionDirectEdit；不能用 Agent 推测调用该路径。临床诊断与普通 AI 分析来源及用户接受行为彼此独立，安全阻断或证据不足必须如实展示。

**UI 状态**：来源、结论强度、当前病程、解释是否采纳、提案等待决定/已决定未提交/过时、分析无结果/安全阻断、当前条目与历史。验收沿用 DX-01～14，新增 DX-15/16；复发与重新采纳用例待 DC-021/024 决策。
