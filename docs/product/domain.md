# BodySense · 领域模型与数据契约

**业务数据以用户为隔离边界。当前状态是可查询的投影，内部修订用于审计、一致性与恢复。**

## 1. 聚合关系

~~~text
User
├─ Profile                    基础身份与偏好
├─ BodyState (1)              当前身体事实及真实病程
│  ├─ BodyFact[]              用户自述、行为和生活习惯
│  ├─ BodyObservation[]       测量、自测和有来源的观察
│  ├─ BodyHypothesis[]        AI 或用户提出的解释性假设
│  ├─ ConcernGroups          由相关信息形成的组织范围
│  ├─ SafetyState
│  └─ BodyStateRevision[]
├─ Diagnosis (1)              当前综合诊断
│  ├─ ConditionEntry[]        稳定身份，支持多项局部更新
│  ├─ UnresolvedConcern[]     缺乏足够证据的问题
│  ├─ SafetySummary
│  └─ DiagnosisRevision[]
└─ TrainingPlan (0..N)
   ├─ CurrentPlanContent
   ├─ TrainingPlanRevision[]
   ├─ PlanChangeProposal[]
   └─ TrainingLog[]
~~~

一个病情可以关联多个 Concern，一个 Concern 可关联多种候选解释。Concern 是便于组织事实的派生分组，不是另一份身体真值，不要求用户创建或管理独立问题实体与报告。

Conversation 记录对话和工具活动；当前身体事实的权威来源是 BodyState。Profile 保存相对稳定的身份信息；测量值、生活习惯和伤病状态属于具有时间语义的 BodyState。

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

字段为**逻辑契约**；实际持久化结构应根据类型和服务边界落实，不要求照搬为单一表。

### 2.1 写入权限

明确来自本人且语义清晰的用户自述，经过结构验证后直接保存；无需事前人工批准。系统必须核查来源消息、用户身份、主体归属、时间语义、字段可解释性及幂等标识。

- **用户自述**：用户明确报告的现象、健康史或生活习惯。自动记录不等于临床客观核实。
- **Observation**：有方法及来源的检查、自测、测量或报告观察。
- **Hypothesis**：病因解释或可能疾病，不得通过自动 Fact 写入获得确诊权威。
- **Evidence**：支持或反驳假设的文献、报告、测量等，有可追溯来源。

只有存在不可消除的关键歧义时，才对相关字段启动 Clarification；同一条消息的其他明确内容可以先提交。

### 2.2 时间与更正

| 操作 | 当前投影 | 审计和时间线 |
| --- | --- | --- |
| Add | 新增事实 | 记录来源与适用时间 |
| TemporalChange | 更新有效状态 | 保留旧状态的真实时间区间 |
| Correct | 更正错误记录 | 保留纠错痕迹，错误数据不当病程 |
| Duplicate | 无实质改变或增加证据 | 不能制造多份相同事件 |
| Clarify | 暂不写入模糊字段 | 记录待回答的问题、目标和来源 |

至少区分身体状态有效时间、观察时间、记录时间、原始证据形成时间。描述「最近」「去年」「一周前」时保留原始精度并使用用户时区。不能为了填充数据库字段而编造日期。

一个语义完整的事务可以增删改多个 BodyFact，但只形成一次 BodyStateRevision。并发提交必须使用乐观锁或等效机制避免丢失修改。

## 3. Diagnosis

~~~ts
type Diagnosis = {
  userId: string;                      // 每位用户唯一
  currentRevision: number;             // 仅内部使用
  conditionEntries: ConditionEntry[];
  unresolvedConcerns: UnresolvedConcern[];
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
    | "ai_supported_possibility"
    | "unresolved";
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

### 3.1 正式诊断的授权与审查

正式持久化的 Diagnosis 创建、更新由用户的明确请求/授权触发。Agent 在聊天中给出的初步分析不自动发布为正式 Diagnosis。用户说「重新分析右肩」本身就是请求，不要求重复点选按钮。

提交前必须经过来源验证、证据检查、临床确定性语义和安全策略校验。证据不足时可不产生 ConditionEntry，并记录待明确的问题。

### 3.2 局部更新、整体修订

~~~text
Diagnosis current revision R
    ├─ A  unchanged
    ├─ B  replace content after authorized, verified analysis
    └─ C  unchanged
             |
             v
Diagnosis current revision R+1
    ├─ A  same stable identity/content
    ├─ B  same stable identity / updated assessment
    └─ C  same stable identity/content
~~~

- 一次原子事务可更新、增加或结束一个或多个病情条目。
- **整体修订每个成功事务仅增加一次**，与涉及的条目数量无关。
- 只更新明确在补丁范围内的 ConditionEntry，其他条目不得重建或删除。
- 空变更不制造新修订。
- 每个修订保存精确的 BodyState 输入快照/修订和有效证据引用。
- 新身体事实不自动发表正式诊断；相关项可标记复核建议，等待用户请求后重评估。
- 历史 DiagnosisAnalysis 保留为不可变审计/证据资料，当前 Diagnosis 为单一对外聚合视图。

病情被缓解或解释被否定时，应明确结束或更新相应条目，不能悄悄删除导致用户当前记录自相矛盾。

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
    exercises: ExercisePrescription[];
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

- 一位用户有多份 Plan，彼此独立修订；每个动作与训练记录关联明确的 planId。
- 同一份计划的动作/组次/频率/排程具有一个当前有效权威副本，避免重复来源产生冲突。
- **用户明确请求**的、范围清晰的修改可以直接提交，保留该请求的授权来源；范围不清或影响重大的修改需要先澄清。
- **Agent 建议**仅创建 Proposal；接受时检查对应方案、用户身份、底版修订和决定状态，才允许提交。
- 未响应、拒绝、过期或取消 Proposal 时，不产生内容修订。
- 安全执行阻断是独立状态，不等同于修改计划内容。
- TrainingLog 和身体变化回写只触发影响检查，不直接修改计划。

## 5. 一致性、审计与权限

- Go 业务服务持有持久化状态和授权判定；Python Agent 提供结构化输入、推理与工具调用；Web 显示当前状态并提交用户动作。
- 每项写操作绑定 userId、sourceId、idempotencyKey、expectedRevision（适用时）与业务权限。
- 工具执行需要报告成功/失败，不允许前端在事务未提交时宣称已保存。
- 异步事件重复、乱序或网络重试不造成重复数据或重复修订。
- 训练提案的授权必须对应**准确的提案身份及内容**，不能重用对其他计划的授权。
- 病程与业务修订可检索，但不把内部编号或版本管理列为普通页面的默认入口。
