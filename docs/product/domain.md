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
│  ├─ BodyHypothesis[]        AI 或用户提出的解释性假设
│  ├─ ConcernGroups           由相关信息形成的组织范围
│  ├─ SafetyState
│  └─ BodyStateRevision[]
├─ Diagnosis (1)              用户确认过的病情综合诊断（含用户选择保留的疑似病情）
│  ├─ ConditionEntry[]        稳定身份，支持多项局部更新
│  ├─ SafetySummary
│  └─ DiagnosisRevision[]
└─ TrainingPlan (0..N)
   ├─ CurrentPlanContent
   ├─ TrainingPlanRevision[]
   ├─ PlanChangeProposal[]
   └─ TrainingLog[]
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
~~~

### 2.1 写入语义与证据分类

满足 R-BS-02 的用户自述，经过结构验证后直接保存，无需事前人工批准。系统必须核查来源消息、用户身份、主体归属、时间语义、字段可解释性及幂等标识。

| 分类 | 含义 | 权威边界 |
| --- | --- | --- |
| 用户自述（BodyFact） | 用户明确报告的现象、健康史或生活习惯 | 自动记录不等于临床客观核实 |
| Observation | 有方法及来源的检查、自测、测量或报告观察 | 可查看方法与来源 |
| Hypothesis | 病因解释或可能疾病 | 不得通过自动 Fact 写入获得确诊权威 |
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
  name: string;
  certainty:
    | "clinically_documented"
    | "user_reported_clinical_diagnosis"
    | "user_confirmed_agent_analysis"    // 用户选择「大概率是这个」
    | "user_kept_as_suspected";          // 用户选择「可能是，但不确定，先保留」
  causeAnalysis: {
    summary: string;                  // 明确的原因解释，注明证据边界
    supportingEvidenceIds: string[];
  };
  handlingPlan: {
    summary: string;                  // 该病情的处理方向，不等于训练动作清单
    actions: string[];
    cautionNotes: string[];
  };
  confirmation: {
    confirmationToolCallId: string;   // Chat 病情确认工具的记录
    confirmedAt: string;
    confirmedScope: "name_cause_handling";
    decision: "likely" | "keep_as_suspected";   // 用户在确认工具中的选择
  };
  lifecycle: "active" | "monitoring" | "resolved";
  relatedConcernIds: string[];
  relatedBodyRegionIds: string[];
  basisFactIds: string[];
  basisObservationIds: string[];
  supportingEvidenceIds: string[];
  counterEvidenceIds: string[];
  analysisSummary: string;
  informationGaps: string[];
  lastAnalysisId: string;
  basedOnBodyStateRevision: number;
};
~~~

### 3.1 病情确认结果与数据映射

规则见 R-DX-02～R-DX-05、R-DX-08。写入前必须通过来源验证、证据检查、临床确定性语义、用户确认记录和安全策略校验。已有条目发生实质变化同样需要用户在确认工具中核对变更内容。

| 用户选择 | 数据结果 |
| --- | --- |
| 大概率是这个 | 创建/更新 ConditionEntry，`certainty = user_confirmed_agent_analysis`，`decision = likely` |
| 可能是，但不确定，先保留 | 创建/更新 ConditionEntry，`certainty = user_kept_as_suspected`，`decision = keep_as_suspected`，并必须写明 `informationGaps`（还需要明确什么） |
| 不是这个，不用保留 | 不创建条目；若是已有条目，按「解释被否定」结束该条目。在 DiagnosisAnalysis 审计中记录用户否定 |
| 继续补充 / 取消 / 超时 | 不写入 Diagnosis；提案保持待定，相关 BodyState 事实不丢失 |

- 疑似条目可在后续正式分析中经同一确认工具升级为「大概率是」，或被用户否定后结束；状态变化只在用户确认后生效。
- 只有对应医疗材料或可靠来源才能标为 `clinically_documented`。
- 历史 DiagnosisAnalysis 保留为不可变审计/证据资料；当前 Diagnosis 是单一对外聚合视图。

### 3.2 局部更新、整体修订

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
- 每个修订保存精确的 BodyState 输入快照/修订和有效证据引用。
- 新身体事实不自动改变 Diagnosis；相关条目可标记复核建议，等待用户请求后重新分析（R-DX-06）。
- 病情缓解或解释被否定时，应明确结束或更新相应条目，不能悄悄删除导致当前记录自相矛盾。

## 4. TrainingPlan

~~~ts
type TrainingPlan = {
  planId: string;
  userId: string;
  title: string;
  goal: string;
  relatedConditionIds: string[];
  relatedConcernIds: string[];
  status: "active" | "paused" | "completed";
  currentAcceptedRevisionId: string;
  currentContent: {
    sessions: TrainingSession[];
    // 一个 Plan 是完整方案；每个 Session 按顺序组织动作，不把单个部位模块当作 Plan
    exerciseStages: {
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
    frequency: { sessionsPerWeek?: number; description?: string };
    constraints: string[];
  };
};

type PlanChangeProposal = {
  proposalId: string;
  planId: string;
  basedOnRevisionId: string;
  initiatedBy: "user" | "agent";
  reason: string;
  proposedChanges: PlanPatch[];
  state: "pending" | "accepted" | "rejected" | "expired";
  userAuthorizationId?: string;
};
~~~

- 每份 Plan 独立修订；每个动作与训练记录关联明确的 planId（R-TP-01）。
- `currentContent` 必须能直接呈现 R-TP-02 要求的完整处方：热身 → 正式训练 → 必要放松，逐个动作名称、组数、次数/时长、必要休息及训练频率。
- 同一份计划的动作/组次/频率/排程只有一个当前有效权威副本。
- 用户明确请求的修改（R-TP-03）直接提交并保留该请求的授权来源。
- Agent 建议（R-TP-04）仅创建 Proposal；接受时检查对应方案、用户身份、底版修订和决定状态，才允许提交。未响应、拒绝、过期或取消的 Proposal 不产生内容修订。
- 安全执行阻断是独立状态，不等同于修改计划内容（R-SAFE-02）。
- TrainingLog 和身体变化回写只触发影响检查，不直接修改计划（R-TP-05）。

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
| INV-02 | 用户自述不会被模型转化为临床确诊；Observation / Hypothesis 与 Fact 不混同 |
| INV-03 | Correction 与 TemporalChange 是不同业务操作 |
| INV-04 | Diagnosis 的局部补丁不会删除或改写无关病情 |
| INV-05 | 同一 Diagnosis 原子补丁最多产生一个整体新修订 |
| INV-06 | Diagnosis 条目的创建/实质更新需要可追溯的用户请求，以及用户对病名、原因和处理方案的确认选择（「大概率是」或「先保留」）；单纯请求分析不代表接受 Agent 结论 |
| INV-07 | 未经用户授权绝不改变 TrainingPlan 的当前有效内容 |
| INV-08 | 训练安全阻断与计划内容修订的权限独立 |
| INV-09 | 工具、事件和网络重试具备幂等、错误提示和可恢复性 |
| INV-10 | 当前页面不暴露技术修订号、历史版本管理或后端运行编号 |
