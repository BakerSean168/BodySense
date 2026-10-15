# BodySense · BodyState 领域契约

## 2. BodyState

**性质：目标业务逻辑模型（BS-D1～D10 已确认的原则）；不代表当前 Go 数据库/API 已实现。** 业务规则见 [requirements `2.2](../requirements.md#22-bodystate-身体状态)，运行时不变量见 [domain/README](./README.md#6-运行时不变量)，具体证据采纳见 [健康文件证据管线](../../architecture/health-document-evidence-pipeline.md)。

BodyState 每位用户恰有一份，持有**用户本人真实陈述、可追溯观察、用户提出的假设、症状经历关系及安全状态**。它不是 Diagnosis，也不是 Conversation、Person 的第二份身体事实存储。

~~~ts
type BodyState = {
  userId: string;
  currentRevision: number;            // 领域事务修订，普通 UI 不展示
  currentFacts: BodyFact[];
  symptomEpisodes: SymptomEpisode[];  // 稳定身份与关联；state 为派生投影
  observations: BodyObservation[];
  hypotheses: BodyHypothesis[];
  concernGroups: DerivedConcernGroup[]; // 只读组织投影
  safetyState: SafetyState;            // 详细限制契约由 safety.md 定义
};
~~~

### 2.1 证据归属与自动写入

| 对象 | 权威内容 | 不得混入 |
| --- | --- | --- |
| BodyFact | 用户明确陈述的症状、健康史、活动、习惯和训练反馈；表示「用户报告如此」，非医学客观证实 | AI 对病因的解释、他人的健康情况、知识问答 |
| BodyObservation | 有方法、来源及时间的测量或观察（含用户自报测量、自测、设备及经过准入的文件观察） | 未准入 OCR 候选、未经证实的临床结论 |
| BodyHypothesis | 用户本人明确提出的病因猜测或可能疾病 | Agent 自己提出的假设；后者只进入 ConditionAnalysisRun / 提案审计 |
| SymptomEpisode | 同一段症状经历的事实关联、时间边界和推导状态 | 独立且未经来源支持的身体症状、医学病名 |
| DerivedConcernGroup | 相关 Episode、Fact、Observation、Hypothesis 的组织视图 | 第二份身体事实、独立诊断 |
| SafetyState | 当前有效的风险限制及来源 | TrainingPlan.status 或 Diagnosis 生命周期 |

明确本人陈述经 Agent 提取、Go 验证后直接保存，不逐项请求批准（R-BS-01～03）。同一消息中的明确内容可先保存，关键歧义进入 Ask User；**事实自动入库与自动认可医学结论是两回事**。Person 的 Records/Habits 是权威聚合的视图/编辑入口，不复制身体真值；是否写入由事实类型与来源规则决定，不由所在 UI 标签页决定。

### 2.2 公共来源、时间与记录状态

~~~ts
type BodyFactKind =
  | "symptom" | "health_history" | "activity"
  | "lifestyle" | "training_feedback";

type UserStatementSource = {
  kind: "user_message" | "direct_edit" | "structured_answer";
  id: string;                         // 稳定来源引用
};

type TemporalValue = {
  start?: string;                      // 用户陈述支持的真实发生/生效时间
  end?: string;
  precision: "exact" | "day" | "range" | "approximate" | "unknown";
  originalExpression?: string;        // 「大约上周」「很久以前」等
  timezone?: string;                   // 相对时间解释采用当时用户时区
};

type AssertionStatus = "asserted" | "superseded" | "withdrawn";

type BodyFact = {
  id: string;
  userId: string;
  subject: "self";
  kind: BodyFactKind;
  statement: string;                   // 用户原意，不把 Agent 推断伪装成用户话语
  bodyRegionIds: string[];             // 非局部事实可为空
  source: UserStatementSource;
  temporal: TemporalValue;
  observedAt?: string;                 // 用户/系统观察时间（有依据时）
  recordedAt: string;                  // 系统保存时间，不能代替发生时间
  assertionStatus: AssertionStatus;
  supersedesFactId?: string;           // 仅纠错继承；真实变化不是事实纠错
  details: FactDetails;                // 依 kind 分型，见 `2.3
};
~~~

- `2.2 的 `asserted` 表示该**用户陈述记录**仍有效，不表示症状仍持续，更不表示医学已确诊。
- `superseded` 表示旧陈述被纠正后的新陈述代替；`withdrawn` 表示用户明确撤回其陈述。二者有审计链，不能伪造症状缓解。
- **历史**由发生时间、结束时间和事件关联表达；不再有 BodyFact 的统一 `active / resolved / historical` 生命周期（BS-D1）。
- 一条「昨天头痛」不能推出今天依旧头痛或已经康复；「今天没有头痛」是有效的阴性陈述，不能自动终结所有相关 Episode。
- **隐私删除不是 withdrawn**：PC-D6/D7 已明确结构化事实与必要原话片段、原件移除与健康衍生数据擦除的区别；源真正擦除按 [Person/Chat §3](./person-chat.md) 先阻止使用再清理/脱敏并复核。DC-033/035 的物理传播、备份及合规执行仍待工程验收。

### 2.3 五种 BodyFact 的差异化字段和状态（BS-D2）

| kind | `details` 最小业务内容（逻辑字段，允许未提供） | 状态/归属 |
| --- | --- | --- |
| `symptom` | `symptomType`、`presence: present/absent`、`laterality`、`severity`（原始量表/单位）、`pattern: continuous/intermittent/unknown`、`triggers`、`relievingFactors`、`occurrenceContext`、可选的明确病程陈述 `episodeCourseReport` | 用户每次明确描述均为事实；持续、缓解、复发由 Episode 推导 |
| `health_history` | `historyKind`、事件描述、所述发生时间/结果、来源上下文 | 既往损伤、手术、用户转述医生诊断等历史陈述；通常是事件，不强制状态机 |
| `activity` | `activityType`、持续时间/运动量/强度（有依据时） | 某次具体活动事件；长期习惯归 lifestyle |
| `lifestyle` | `habitType`、频率/规律/典型时长、适用时期 | 事实有效时间段表达习惯调整、停止与未知；不覆盖旧时期 |
| `training_feedback` | `sessionLogId`、`exerciseId`（可选）、反馈类别与原文 | 一次用户主观反馈；正式完成/跳过仍由 Trainer 的 TrainingSessionLog 权威持有 |

字段不由 Agent 依据习惯擅自填满；无来源则保留未提供。症状的同一次/多次发作，不由部位和固定天数判断；同一句训练反馈若明确包含身体症状，可同时形成来源共享但职责不同的 symptom Fact，且避免复制 Trainer 已有的完成记录。

明确病程陈述作为 **symptom Fact 自身可选的来源解释字段**：`episodeCourseReport = explicit_current_report | explicit_resolution`；前者与 `presence=present` 对应，后者与 `presence=absent` 对应，但反向推断不成立，普通“现在没有疼”**绝非**“本次病程已结束”。保存 Fact、将 Fact 组织到 Episode、将明确报告纳入 Episode 状态证据是三个独立写入动作。当前 v9 工程仅开放经过本人直接编辑的明确陈述及其精确关联的用户授权提交；Chat/Agent 提取的 High 可信病程关联仍须完成独立验证，不得复用直接编辑命令绕过审核。见 [v9 证据准入验收说明](../../plan/active/2026-10-13-episode-status-admission.md)。

### 2.4 SymptomEpisode：身份、关联及状态（BS-D3～D7）

~~~ts
type EpisodeState = "unknown" | "ongoing" | "ended";

type SymptomEpisode = {
  id: string;
  userId: string;
  symptomType: string;                // 分组特征；并非医学病因
  bodyRegionIds: string[];
  laterality: "left" | "right" | "bilateral" | "not_applicable" | "unknown";
  createdAt: string;
  identityStatus: "current" | "superseded" | "invalidated"; // 组织身份，不是病程状态
  projection: {
    state: EpisodeState;
    stateAsOf?: string;                // 最近支持该结论的业务时间
    onset: TemporalValue;
    end?: TemporalValue;
    reason: "explicit_current_report" | "explicit_resolution"
      | "insufficient_evidence" | "conflicting_evidence";
    basisFactIds: string[];
    basedOnRevision: number;
  };
};

type EpisodeFactLink = {
  episodeId: string;
  factId: string;
  role: "onset" | "observation" | "change" | "resolution";
  associationMethod: "explicit" | "inferred" | "user_corrected";
  basisSourceIds: string[];
};

type EpisodeRelation = {
  fromEpisodeId: string;
  toEpisodeId: string;
  kind: "recurrence_reported" | "similar_symptom"
    | "merge_replacement" | "split_replacement";
  basisFactIds: string[];
  authorizationId?: string;          // 合并/拆分必须绑定准确授权
};
~~~

链接是独立、可审计的关联；Episode **不持有症状正文、疼痛程度或独立的医学结论**。用户未明确提到的 onset/end 不能补造。一个症状 Fact 至多属于一段互斥经历的**主关联**；与其他经历或 Concern 的关联须另行标明，不能偷偷复制事实。

| 依据 | Episode 结果 | 必须遵守 |
| --- | --- | --- |
| 明确仍持续 | `ongoing` | 是「截至该陈述时间持续」，不是保证查询当下仍持续 |
| 明确本次症状完全结束 | `ended` | 必须引用具体有效结束依据；改善、短暂不疼不等于结束 |
| 不知目前是否持续 | `unknown` | 不靠无新消息、固定时间跨度自动结束 |
| 已结束后明确再次发作 | **新 Episode**，关联旧经历 | 不修改旧 Episode 的结束事实；关联不证明同一医学病因 |
| 原结束依据被用户明确纠错 | 原 Episode 重算 | 不伪造一次结束后复发 |
| 直接影响状态的未解决实质冲突 | `unknown` + `conflicting_evidence` | 保留冲突依据及上次有支持的状态；局部历史冲突不必降级整段 |
| 唯一支持事实被撤回 | 重算；无有效症状依据时组织身份 invalidated 并退出当前投影 | invalidated 不等于病程结束；不继续保留无依据的 ended/ongoing |

Episode 当前投影只能由有效 BodyFact、可追溯链接与领域变更事件**确定性重建**；允许保存缓存供读，但不允许直接写 `state = ended` 而不提供依据。重建按**业务时间与精度**组织，按**修订顺序**回放当时已知事实；补录过去的事实不能倒改历史修订当时的知识。

#### 关联与授权

- **High**：用户明确指代、时间/症状特征兼容、无竞争候选，可自动关联或创建明确的新经历；`High` 是依据强度而非医疗诊断概率。
- **Medium / Low**：明确的 BodyFact 照常写入；不强行关联，只有关系歧义会实质影响病程才发起 Ask User。未关联 Fact 仍有完整来源，且参与安全识别。
- **关键冲突**：实时会话中由 Agent 优先调用 Ask User，给出「旧记录说错 / 新表述说错 / 确实结束后再发生 / 条件不同 / 自行补充 / 暂不确定」等与上下文匹配的选项。用户回答构成对**精确更正目标和内容**的授权，无需同一动作再次重复确认。
- **交互未完成**：网络中断、取消、无回答等由 Chat/PendingInteraction 持有独立待决身份；冲突直接影响 Episode 状态时才暂时推导为 unknown，并保留冲突依据。恢复时按目标修订重新核验；`pending` 不是 Episode 的病程状态。
- **合并/拆分/关键边界更正**：要求用户对准确范围显式授权；原组织 ID/Fact 不删除，合并或拆分可创建新 Episode 身份并保留旧→新沿革、引用映射和修订审计；各新 Episode 独立重算，不复制旧状态。若 Diagnosis 已引用旧 ID，仅提示/标记受影响分析复核，不静默改写已确认的 ConditionEntry。
- 来源冲突需比较主体、症状类型、部位、侧别、有效时间与触发/观察条件；时间条件不同或主观症状与客观观察不同不必然冲突。出现紧急风险时，不得等候关联澄清而跳过 Safety 判断。

### 2.5 BodyObservation、BodyHypothesis、ConcernGroup 与证据分级（BS-D9）

~~~ts
type BodyObservation = {
  id: string;
  userId: string;
  kind: string;
  statement?: string;                 // 非数值观察
  rawValue?: string;
  rawUnit?: string;
  normalizedValue?: number;
  normalizedUnit?: string;
  method?: string;
  measurementContext?: string;
  source: { kind: "user_report" | "self_test" | "device" | "document" | "structured_observation"; id: string; version?: string };
  observedAt?: TemporalValue;
  recordedAt: string;
  assertionStatus: AssertionStatus;
  supersedesObservationId?: string;
  evidenceRefId?: string;             // 文件/设备证据，视来源可选
};

type BodyHypothesis = {
  id: string;
  userId: string;
  statement: string;
  source: { actor: "user"; reference: UserStatementSource }; // 用户消息/直接编辑/结构化回答；排除 AI 假设
  relatedFactIds: string[];
  relatedObservationIds: string[];
  bodyRegionIds: string[];
  assertionStatus: AssertionStatus;
  supersedesHypothesisId?: string;
  recordedAt: string;
};

type DerivedConcernGroup = {
  id?: string;                        // 派生展示标识，不是独立权威
  relatedFactIds: string[];
  relatedEpisodeIds: string[];
  relatedObservationIds: string[];
  relatedHypothesisIds: string[];
  displayTitle?: string;
};
~~~

- 用户明确自报「我体重 67 公斤」可形成带 `source=user_report` 的个人测量观察，**不等于有医疗机构、设备或检验报告支持**；有来源的方法与时间应保留，缺失方法不编造。
- 文档/图片 OCR 提取的值，只有按 [证据管线](../../architecture/health-document-evidence-pipeline.md) 准入后才成为相应的 BodyObservation；未准入候选留在来源资料处理记录，不伪装成可信观察。文件候选的 `admissible/needs_review/rejected` 与 Observation 的记录状态、来源等级是**不同维度**；自报测量不因没有 OCR 流程而被误标为可信临床证据。
- Hypothesis 的纠正与撤回使用与 BodyFact 一致的陈述记录机制，原事实和观察不会因用户撤回猜测而被删除；AI 提出的病因推测只存于 ConditionAnalysisRun 的审计事件。
- ConcernGroup 只组织相关事实、经历、观察及用户猜测；不独立得出结论。分组键稳定策略、自动重组和可见顺序仍属于 DC-032 的实现/产品细节。

### 2.6 事务、审计与跨领域边界（BS-D1～D10）

| 业务操作 | 目标及效果 | 授权与历史 |
| --- | --- | --- |
| `FactRecorded / ObservationRecorded / HypothesisRecorded` | 保存明确的新陈述、测量或用户猜测 | 有来源、主体、时间、幂等校验；普通本人陈述不逐项确认 |
| `FactCorrected / FactWithdrawn` | 旧陈述 superseded/withdrawn；更正可创建新 Fact | 用户明确更正/撤回；不制造真实病程变化 |
| `TemporalChangeRecorded` | 追加真正发生的变化事实，触发 Episode 重算 | 与更正分离，旧事实依旧能说明过去 |
| `EpisodeCreated / EpisodeFactLinked` | 创建经历或 High 关联 | Go 校验候选、证据及用户归属 |
| `EpisodeFactUnlinked / EpisodesMerged / EpisodeSplit / EpisodeBoundaryCorrected` | 修正关系或时间边界 | 关键历史修改需具体用户授权；旧 ID 与跨域引用可追踪 |
| `EvidenceInvalidated` | 来源撤销/更正后移除不再适用的证据资格 | 重新计算受影响投影；不静默修改 Diagnosis 或降低 Safety 限制 |
| `PrivacyErasureRequested` | 用户请求清除个人健康信息 | 与撤回陈述和仅移除原始文件分离；按 PC-D7、CD-D4 先阻断已删除来源，再清理/脱敏受影响的来源及衍生信息，可靠复核；软删除不等于擦除完成 |

**一次语义事务只增加一次 BodyStateRevision**，即便包含多条 Fact 与 Episode 变更。逻辑修订对象至少包含 §revision§、§userId§、§sourceId§、§actor: user/agent/system§、§recordedAt§、§idempotencyKey§、§changes[]§（每项含类型、目标身份与变更依据）、§authorizationId?§；已提交审计不可静默篡改。网络重试不得生成重复修订，并发更正必须校验预期修订或同等事务锁。隐私删除的实际擦除/脱敏规则优先于「永久保留明文审计」的实现习惯。

| 交界对象 | 唯一权威及传播规则 |
| --- | --- |
| Person / Habits / Records | Person 提供视图和经授权的录入、更正入口；实际身体陈述/观察写回 BodyState，病情条目仍归 Diagnosis |
| Chat / Ask User | Chat 持有会话、问答及待回复工具的状态；BodyState 只持有事实、冲突依据和完成澄清后的领域更改 |
| TrainingPlan / Trainer | 已执行组次、完成/跳过由 TrainingSessionLog 持有；BodyFact 只保留训练主观反馈或明确的身体变化，并引用训练记录 |
| Diagnosis | 以 Fact/Observation/Episode 与医学诊断形成多对多证据引用，具体具名病名、采纳与医学评估归 Diagnosis；BodyState 变动只建议复核，不自动改写用户已确认的 ConditionEntry，不新增 BodyConcern |
| SafetyState | BodyState 持有风险约束；具体限制范围与解除由 [safety.md](./safety.md) 定义；来源更正不自动放宽限制 |

### 2.7 UI 派生状态与验收

必须能够区分用户陈述与测量来源、目前已知/不明的症状经历、结束的依据、过去曾有的症状、被更正或撤回的陈述、未完成 Ask User、保存成功/失败、报告待核实以及跨领域复核需要。普通用户默认不见内部 revision；`ongoing` 应显示为「截至某次报告仍有」，而非把陈旧记录说成现在已复核。

验收包括 [acceptance.md](../acceptance.md) 中 BS-01～15 及本轮补充的 BS-16～32。现有实现的兼容、Schema 迁移和缺陷修复不属于本次文档评审。

### 2.8 当前 Go 实现映射（As-Is，不是新规格）

| 旧持久对象 | 已存在的字段/状态 | 新契约迁移注意 |
| --- | --- | --- |
| [BodyState](../../../apps/api/internal/model/body_state.go) | UserID、CurrentRevision、SafetyState(JSON)、CreatedAt/UpdatedAt；Facts/Observations/Hypotheses 为读取投影 | 当前没有 SymptomEpisode 的完整目标关联与推导契约；不能因旧 JSON 设计将 Concern 变为第二份真值 |
| [BodyStateFact](../../../apps/api/internal/model/body_state.go) | ID、UserID、ConcernKey、Kind、BodyRegion/ID、Value、Details、Origin、ReviewState、LifecycleState、Trend、SourceKey、Provenance、ObservedAt、ValidFrom/Until、SupersedesFactID、ExcludedFromReasoning、Created/UpdatedRevision 与时间 | 旧 ReviewState/LifecycleState/Trend 不等于本轮 assertionStatus + 独立时间/episode 状态；兼容映射待 DC-025/026 |
| [BodyStateObservation](../../../apps/api/internal/model/body_state.go) | Kind、Method、Value(JSON)、Condition、SourceKey、Provenance、ObservedAt、ReviewState、LifecycleState、SupersedesObservationID、ExcludedFromReasoning、修订和时间 | 旧 unverified/confirmed/rejected 与来源文件准入、临床可信等级不是同一维度（DC-027/028） |
| BodyStateHypothesis（已删除） | 旧 AI 假设表 body_state_hypotheses 及 /api/v1/body-state/hypotheses 已在 PR4 移除；Agent 假设只保存在 diagnosis_analyses.decision_trace.agent_hypotheses | 用户自述假设见 body_hypotheses（BodyHypothesis） |
| [BodyStateRevision](../../../apps/api/internal/model/body_state.go) | ID、UserID、Revision、ChangeType、Source、Changes(JSON)、CreatedAt | 有修订基线，但目标的结构化关联/授权/回放协议需独立验证（DC-030） |

以上是静态实现对照，不代表代码已经满足新产品规则；见 [数据库基线迁移](../../../apps/api/migrations/000001_vnext_baseline.up.sql) 与 [实施差异](../implementation-gaps.md)。

### 2.9 仍未完全批准或尚待工程契约的内容

| 范围 | 本轮已确定 | 仍然开放 |
| --- | --- | --- |
| 事实分类、历史与 Episode | 五类事实、独立陈述状态、病程推导、High 关联/按需 Ask User、合并拆分授权 | 旧 ReviewState/Trend/Kind 迁移 DC-025/026；Episode 完整物理 Schema、身份映射与重放实现 DC-030/032 |
| Observation | 自报/设备/文件来源区分、原始值/单位、文件准入独立于记录状态 | 具体数值标准化、方法目录、纠错与资格重评协议 DC-027/028 |
| Hypothesis | 用户本人假设，更正/撤回与保留证据关联 | 旧 AI 假设记录迁移 DC-029 |
| ConcernGroup | 派生组织视图，不复制事实 | 组键、分合与展示顺序 DC-032 |
| 隐私清除 | 纠错、撤回、仅移除原件、清除个人健康数据均已区分（PC-D6/D7、CD-D4） | 真实清除/脱敏/缓存/审计、下游传播和完成回执的实现与合规核验 DC-033/035 |
| Safety | 安全状态属于 BodyState，不能自行解除 | 风险等级、限制范围、解除依据的细节由 safety.md / DC-001～006 定义 |