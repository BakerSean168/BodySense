# BodySense · Person / Chat + 跨领域契约评审稿（2026-10-13）

> **状态：候选设计，待产品决策；不是当前权威规格或已实现 Schema。**
>
> **代码/文档基线：** GCP Dev `7769b50e8`（`docs/domain-contract-completion-recovery-20261012`）。本文在独立工作树维护，可以与 Training / SafetyState 并行。不得重开已确认的 BS-D1～D10 和 DX-D5～D15。
>
> **本轮目标：** 完成 Person / Chat 对象、生命周期、权限、数据归属和跨领域删除/回放的候选领域契约；不修改数据库、Go API、Agent、React 或生产配置。文中“建议”的方案未经用户批准不能直接视为正式决定。

## 1. 已批准的边界与现场证据

| 边界 | 已确定规则 | 依据 |
| --- | --- | --- |
| Person | Records、Equipment、Media、Reports、Habits 五类视图，Profile 在标题区/Identity Popover；Person 不持有第二份身体真值 | R-PS-01～05、INV-28/34、BS-D8 |
| BodyState | 明确的本人陈述自动按事实类别记录；真正身体变化、更正、撤回陈述、隐私擦除彼此区分 | BS-D1～D10 |
| Diagnosis V2 | 只管理具名医学诊断；正式分析和逐项确认、整体原子提交；新证据不自动修改已采纳诊断 | DX-D5～D15、INV-12/38 |
| Training/Safety | PlanChangeProposal 未经准确授权不改处方；Trainer 日志有独立权威；SafetyState 归 BodyState，不能被 Chat 或计划覆盖 | INV-07/08/26 |
| Chat | 关闭窗口不是接受/拒绝；PendingInteraction 稳定可恢复；运行状态不是业务决定 | INV-29、agent-workflows §4 |
| 文件证据 | 上传/OCR 完成不等于候选准入，更不等于临床诊断 | R-PS-03、INV-24/33 |

**As-Is 静态核对：**

- `apps/api/internal/model/user.go` 负责账户身份；`user_profile.go` 当前仅 Gender、BirthDate、派生 AgeYears。旧字段存在不代表完整 Profile 已获批准。
- `model/conversation.go`、`message.go` 已有多会话、消息顺序、Run 关联、软删除等字段；`model/agent_interaction.go` 与 `service/agent_interaction_service.go` 有 pending/answered/cancelled/expired、重复回答判定及 CAS。
- `service/conversation_service.go` 删除会话时在数据库事务里撤销分享链接并软删除 Conversation；这不证明整个 Chat 历史、领域事实或外部副本已被擦除。
- `service/upload_service.go` 的 DeleteUpload 删除原始存储对象与上传 manifest；此路径没有证明对已生成 Observation、医学分析事件、缩略图/缓存等做了完整传播。
- `service/share_service.go`、`transport/httpapi/routes.go` 有令牌访问的公开 ConversationShare 快照；属于待决定隐私功能（DC-039）。
- `apps/web/src/features/consultation/runtime/activeTurnSelectors.ts` 会在 interrupted 时锁 Composer；不能据此宣称普通输入框已经可以向原 Run “继续补充”（IG-07）。

以上未经过线上数据/对象存储/浏览器 E2E 核验，**不能宣称实际数据已经或尚未泄露**。

## 2. 领域所有权：优先建立清楚的单一权威

~~~text
User/AuthIdentity ── Profile(0..1)
                 ├─ EquipmentAsset(0..N)
                 ├─ UserAsset(0..N) ─ ExtractionRun ─ EvidenceCandidate
                 ├─ Conversation(0..N) ─ Message(0..N)
                 │                     └ AgentRun ─ PendingInteraction
                 ├─ BodyState(1) ─ Fact / Episode / Observation / SafetyState
                 ├─ Diagnosis(1) ─ ConditionEntry / AnalysisRun / Proposal / Decision
                 └─ TrainingPlan(0..N) ─ Revision / Proposal / TrainingSessionLog

PersonView = 按当前用户读取 Profile + Equipment + Assets + BodyState 投影；
不另存第二份 Records、Habits 或 Diagnosis。
~~~

| 概念 | 状态权威与责任 | 禁止持有 |
| --- | --- | --- |
| User/AuthIdentity | 登录主体、邮箱与鉴权 | 身体疾病事实 |
| Profile | 稳定的个人展示资料、界面偏好 | 身高体重测量、症状、生活习惯、医学结论 |
| EquipmentAsset | 器材拥有/借用/可访问及有效期 | 训练处方和医学风险状态 |
| UserAsset | 原始文件单一资产身份；Media/Reports 多视图 | 文件候选的医学准入、BodyObservation |
| EvidenceCandidate | 提取来源、定位、处理版本及准入 | 用户授权采纳医学诊断 |
| Chat | 消息、Agent Run、待答交互与恢复 | BodyFact / ConditionEntry / TrainingPlan 的第二权威 |
| 其他健康领域 | 按已批准契约持有唯一事实和修订 | 直接修改其他领域的权威对象 |

## 3. Person 候选契约

### PC-D1 · Profile 与偏好（DC-015/040）

**建议：极简且可选。** 逻辑字段：`userId` 唯一、`displayName?`、`birthDate?`、`genderOrSexMetadata?`、`locale?`、`timeZone?`、`unitPreference?`、服务端 `updatedAt/profileRevision`。出生日期和性别相关资料属于敏感可选字段；不得推定其医学含义。年龄按时间派生，不持久成第二权威；邮箱/口令/角色属于 User/Auth，不重复。

时间变化的测量、病史、习惯全归 BodyState。时区建议采用 IANA 名称并保留事件当时的时间上下文；变更时区只改变以后显示或排程规则，不重写既往 TrainingSessionLog 的训练日。单位设置只转换展示，不改原始 Observation 值和单位。用户应能修改、清空可选 Profile 字段，并有后端逐字段校验。

**待决定：** 昵称是否独立于身份账户；是否真需要性别/生日等字段；哪些资料可以被公开展示。产品未批准前不增加必填属性。

### PC-D2 · EquipmentAsset（DC-023/041）

~~~ts
type EquipmentAsset = {
  equipmentId: string; userId: string; displayName: string;
  accessKind: "owned" | "borrowed" | "facility_access";
  availability: "available" | "unavailable" | "retired";
  validFrom?: string; validUntil?: string; sourceRef?: SourceRef;
  createdAt: string; updatedAt: string;
};
~~~

器材是**用户可使用资源**，而不是身体问题或训练动作。资源变为 unavailable 时只触发计划影响检查/调整建议；不得直接修改 TrainingPlan。具有过去/现在时间意义的器材使用陈述要保留原阶段，不能用一个最新布尔值抹平历史。旧 BodyFact 类目 `equipment.available` 的迁移需要去重，不能新增第二权威。

### PC-D3 · 同一 UserAsset 的 Media/Reports 多视图（DC-023/034/042）

~~~ts
type UserAsset = {
  assetId: string; userId: string; facets: ("media" | "report")[];
  originalName: string; mimeType: string; byteSize: number;
  contentHash: string; privateStorageRef: string;
  state: "available" | "deleting" | "deleted";
  createdAt: string; deletedAt?: string;
};
type ExtractionRun = {
  runId: string; assetId: string; mechanismVersion: string;
  processingState: "queued" | "processing" | "succeeded" | "failed";
  retryOfRunId?: string;
};
type EvidenceCandidate = {
  candidateId: string; runId: string; locator: string;
  admissibility: "admissible" | "needs_review" | "rejected";
  decisionRef?: string;
};
~~~

一份文件可同时出现在 Media/Reports，但只有一个 assetId、源文件与删除身份。**asset.state、processingState、admissibility 是三条独立状态轴**。重处理创建新的 ExtractionRun，不覆写旧的准入决定。现有健康文件证据管线继续作为唯一的解析/准入规则；不新建“上传即诊断”工作流。UserAsset 不直接拥有已采纳 BodyObservation。

## 4. Chat 候选契约

### PC-D4 · Conversation 生命周期（DC-016/036/043）

**建议允许一个用户拥有多段长期 Conversation**。UI 收起不是生命周期动作；`active`、`archived` 与 `deleting/deleted` 建议明确区分：

| 操作 | Conversation | 对领域状态与 pending 的影响 |
| --- | --- | --- |
| 创建、恢复 | active | 新建或恢复消息与 Run，不制造 BodyFact |
| 收起窗口、切换会话 | 不变 | 不拒绝 pending，不改变训练/医学提案 |
| 归档 | archived | 不删除消息、Run、领域授权；原 pending 仍有稳定身份 |
| 删除 | deleting → deleted（需确认清理完成的判据） | 撤销分享、终止该会话后续运行与待答恢复；**未提交提案不得被当成用户拒绝或接受**；已持久化健康事实是否一并删除需要用户明确选择 |

Message 顺序可用 seq/turnId/runId/parentMessageId 回放；Message streaming/failed、AgentRun interrupted/finished、业务 Proposal accepted/rejected 各自独立。会话重命名或置顶不改变领域对象。

### PC-D5 · PendingInteraction、继续补充与并发（DC-017/038/045/046）

~~~ts
type PendingInteraction = {
  interactionId: string; userId: string; conversationId: string;
  runId: string; originatingMessageId?: string; toolCallId: string;
  kind: "clarification" | "domain_proposal";
  target?: {
    domain: string; entityId: string; proposalId?: string;
    expectedRevision?: string; contentFingerprint?: string;
  };
  status: "pending" | "answered" | "cancelled" | "expired";
  expiresAt?: string; answerRef?: string;
};
~~~

**推荐机制：**

1. 同一工具卡可点击“继续补充”，打开**绑定原 interactionId + runId** 的自由输入；该输入按内容进入 BodyState 等权威域，再通知原 Run 重新核验。原提案若受新证据影响，按 Diagnosis V2 产生 superseded 和重新确认。
2. 普通 Composer 若允许补充，也必须在 UI 上明确标记绑定哪条交互，并由后端核验；不允许将另一个 Conversation 的消息隐式代答。
3. 关闭/刷新/切换设备从服务端的 interactionId/Run 和领域状态恢复；每个待答卡独立定位，不能总把答案交给“最新一张卡”。
4. 对重复答案幂等，对相同 interactionId 的不同答案返回冲突。answered 是运行时输入已记录，不等于 Diagnosis 提案 accepted 或 TrainingPlan 处方已提交。
5. 原 Run 终结、目标内容/修订失效时，不可拿旧答案套用新提案。业务 stale 应保留具体原因，不另造一个全局“卡片状态”覆盖领域状态。
6. 同一个 Conversation 不应有两个可相互抢占身份的活动 Run；不同 Conversation 可以独立聊天，但共享的领域写入仍需 revision/CAS。是否允许同会话同时开新话题属于本决策最后的 UI/运行时取舍。

## 5. 跨领域规范（CD-D1～D3）

### CD-D1 · 来源、版本与传播图

~~~ts
type SourceRef = {
  userId: string;
  sourceKind: "message" | "user_asset" | "evidence_candidate" |
              "body_fact" | "body_observation" | "symptom_episode" |
              "training_session" | "direct_user_action";
  sourceId: string; sourceVersion?: string;
  observedAt?: string; recordedAt: string; locator?: string;
};
type DomainCommand = {
  userId: string; targetDomain: string; targetId?: string;
  operation: string; sourceRefs: SourceRef[];
  idempotencyKey: string; expectedRevision?: string;
  authorization?: { interactionId?: string; proposalId?: string;
                    contentFingerprint?: string };
};
type DomainCommitReceipt = {
  commandId: string; domain: string; targetId: string;
  committed: boolean; resultingRevision?: string; eventIds: string[];
};
~~~

这是一套**逻辑约束**，不要求所有 Go API/领域 Patch 硬塞同一 DTO。每条来源关联需可校验归属与版本，文件证据需可定位页/框，训练反馈可定位 session/动作/组，Episode 证据可定位原 Fact 与有效版本。ConditionEntry 与证据是多对多 supporting/counter/context，不能因共享同一症状而自动推导因果或重复计权。**不新建 BodyConcern。**

### CD-D2 · 权威提交与最终一致性

1. Chat/Person 收到输入只作意图识别/参数呈现；Go 验证 userId、sourceId/版本、idempotencyKey、expectedRevision（适用时）、领域权限。
2. 明确本人事实进入 BodyState；具名医学诊断更新进入 Diagnosis Run/Proposal/Decision；计划建议进入 PlanChangeProposal；Trainer 负责组级执行日志；SafetyState 负责风险事实。
3. 工具调用成功≠领域提交成功。只有领域服务返回带状态和修订/事件 ID 的 **DomainCommitReceipt**，才能向用户声明已保存。
4. 跨聚合影响采用可靠 outbox/等价持久事件传播与去重，消费者只做允许的投影重算、影响检查和复核提醒；不能让通知自行接受诊断或修改训练处方。
5. 若一次自然语言请求导致多领域写入，不可假称存在全域原子事务。应明确局部成功、失败重试、补偿和用户可见的每项结果。**Diagnosis 一次 Run 内的原子 Patch 仍不可拆**。

### CD-D3 · 跨域命令的允许边界

| 用户/系统动作 | 可以做 | 绝不可做 |
| --- | --- | --- |
| Chat 新增本人症状、习惯 | BodyState 幂等保存，必要时触发相关复核 | 聊天直接建立医学诊断 |
| Person Records/Habits 更正 | 调用 BodyState 领域更正 | 在 Person 保存第二份事实 |
| 报告候选被准入 | 有来源的 Observation，Diagnosis 复核建议 | 直接升级为临床确诊 |
| 用户接受诊断提案 | 在原 Run 留 Decision，所有有效决定原子提交 | 借用其他 proposalId 授权 |
| 训练中完成动作并反馈疼痛 | Trainer 记真实完成；BodyState 记疼痛，来源关联 | 完成事实重复保存、自动推断病因 |
| 器材不可用/身体事实变化 | 计划影响检查及 Proposal | Agent 静默修改已接受处方 |
| SafetyState 出现限制 | Trainer 门禁，必要时提示/阻断 | 隐式改 Plan.status 或放宽解除 |
| 来源被隐私删除 | 撤销其证据资格、派生复核与重算 | 自动停用 ConditionEntry 或自动解除安全限制 |

## 6. 隐私与删除（P0，须产品批准）

### PC-D6 · 三种操作的不同语义（DC-033/036）

- **更正**：更正当初记录的事实，保留修订原因；不产生虚构身体变化。
- **撤回陈述**：原陈述不再有效，按 BodyState 既有 assertionStatus 与 Episode 推导处理；不是擦除。
- **真正删除**：明确消除用户要求范围内的消息/原文件及有关衍生健康数据；不能通过 withdrawn、UI 隐藏、Conversation 软删冒充已完成删除。

**建议 UI 将「删除聊天历史」与「删除关联健康资料及派生数据」区分。** 删除 Conversation 默认不静默删除早已合法提交的 BodyState 事实，但必须告知用户；用户显式要求真正删除原始资料及派生健康信息时，执行完整可追踪清理。隐私保留的法定例外、范围和期限不可由模型擅自设定。

### PC-D7 · 报告与衍生证据删除（DC-035）

~~~text
requested
 -> scoped/validated
 -> blocked_from_new_use
 -> propagation_in_progress:
      revoke controlled shares + block extraction jobs
      purge original object, thumbnails, OCR, candidate payloads
      purge sensitive search indexes, caches, derived summaries
      invalidate/de-link adopted evidence provenance
      recompute affected BodyState projections / evidence eligibility
      emit Diagnosis reviewRecommended without changing ConditionEntry
      recheck Safety conservatively; NEVER auto-lift restrictions
      verify required steps and record minimal non-sensitive receipt
 -> completed | retryable_failed (source stays blocked)
~~~

- **阻止再使用先于异步物理清理**；删除工作必须持久、逐步可重试、可幂等。DB 与对象存储不共享事务，不把“删除了对象”说成“所有副本已删”。
- 如果某 Observation 仅有被删报告这一来源，不能继续作为可信文档证据；独立存在的其他有效来源不能跟着误删。BodyState 当前投影重算，但删除证据不自动等于疾病消失。
- 已采纳 ConditionEntry 不因来源删除而擅自改 disposition；其来源资格和复核提示必须更新。SafetyState 不依靠缺失证据自动解除已有高风险限制。
- 已有 Run/Proposal/Revision 审计如包含健康原文，真正隐私擦除应去除敏感 payload 并保留适度 tombstone/redacted 事件。**不可变审计不能成为保存已删除个人信息的理由**；擦除后历史回放允许只显示脱敏状态，不能恢复原文。
- 系统可控制的分享快照纳入清理；无法承诺撤回第三方已下载的内容。何种最小匿名化审计可保留、是否受法律/合规强制，须单独批准。
- 只有已批准清理范围内各步骤核验成功，才返回 completed；失败应可重试且已删来源不得重新参与 AI 推理。

### PC-D8 · 健康会话公开分享（DC-039）

**建议默认退役公开分享功能**，对已有 share Token 单独盘点并定义撤销/迁移路径；**本次仅提出建议，不修改现有 API 或令牌**。若仍要保留，最少要求用户显式创建、选择/预览可分享字段、风险提示、可撤销/过期及删除传播。用户未批准前不能将旧功能存在视为未来产品的授权。

## 7. 与并行 Training / SafetyState 的契约

| 阶段三提供 | 阶段二消费 | 已确定底线 |
| --- | --- | --- |
| 时区偏好及写入时的时间上下文 | TrainingSessionLog.date / 排程 | 不使用服务端日期；跨午夜归属仍由 DC-022 决定 |
| EquipmentAsset 可用性变更 | TrainingPlan 影响检查 | 不直接改 Plan |
| PendingInteraction / 精确授权身份 | PlanChangeProposal | 基于 planId、proposalId、fingerprint、revision 校验 |
| SessionLog SourceRef | BodyState.training_feedback | 完成事实仅在 Trainer；反馈另归 BodyState |
| 来源资格/删除事件 | SafetyState + Diagnosis | 安全保守、不自动解除、不自动改诊断 |
| DomainCommitReceipt / 幂等事件 | 所有领域写入 | 不以 ToolCall.succeeded 假称已保存 |

**不在本阶段制定** Training 动作身份、跳过粒度、计划重开、Safety riskLevel 或 professional_clearance 的具体阈值；对应 DC-010～014、DC-001～006/062～065 留在阶段二。

## 8. 决策包（推荐方案，未批准）

| ID | 关联决策 | 推荐收敛方向 | 优先级 |
| --- | --- | --- | --- |
| PC-D1 | DC-015/040 | Profile 最小可选字段、展示偏好与时间事实分开 | P0 |
| PC-D2 | DC-023/041 | 器材为独立可用资产，影响计划但不自动改计划 | P1 |
| PC-D3 | DC-023/034/042 | 文件一个 assetId、多分类视图，解析/准入分离 | P1 |
| PC-D4 | DC-016/036 | 多 Conversation、可归档；删除显式处理 pending | P0 |
| PC-D5 | DC-017/038/045 | 原 interaction/run 锚定自由补充；业务决定独立 | P0 |
| PC-D6 | DC-033/036 | 纠错、撤回、删除分离，聊天删除和健康数据删除分离 | P0 |
| PC-D7 | DC-035 | 源删除后停止使用、逐级清理/重算、最小审计；不会隐式改诊断/解除安全 | P0 |
| PC-D8 | DC-039 | 默认退役公开分享；现存令牌单独审计处理 | P0 |
| CD-D1 | DC-048/066 | 统一来源身份/版本引用，不创建第二事实库 | P1 |
| CD-D2 | DC-044/049/070 | 域内原子提交、跨域可靠事件、真实提交回执 | P1 |
| CD-D3 | DC-038/045/050 | PendingInteraction 与领域授权各持一份准确身份，不跨 Run 复用 | P1 |

批准建议：PC-D1～D5 可合成一轮日常产品语义评审；**PC-D6～D8 作为隐私 P0 单独确认**；CD-D1～D3 可由工程方细化但不得改变产品授权规则。任何带建议的选项未获用户确认前不得进入权威契约。

## 9. 可执行验收规格（拟新增，均未执行）

| ID | 场景 | 必须验证 |
| --- | --- | --- |
| PC-01 | Chat 描述每周跑步习惯后打开 Person Habits | 一条权威 BodyFact，Person 无副本，重复消息幂等 |
| PC-02 | Profile 改时区/单位 | 不改旧 Observation 原单位，也不重写旧训练日期 |
| PC-03 | 同一报告在 Media 和 Reports | 同一 assetId，OCR 完成与准入为不同状态 |
| PC-04 | 器材设为不可用 | Plan 当前修订不变，可以产生受影响计划建议 |
| PC-05 | A/B 会话切换，A 有 pending 工具卡 | 身份和 Run 不变，B 不能代答，关闭/收起不拒绝 |
| PC-06 | 同 interaction 回答重复/答案冲突 | 相同答案幂等，不同答案拒绝，过时 proposal 不得继承旧授权 |
| PC-07 | Diagnosis 一份 Run 两个提案，已决定一项后刷新 | 已决定留档但未提交；待决定恢复；新证据只重问受影响者 |
| PC-08 | 删除仍有待确认 PlanChangeProposal 的会话 | 不自动接受、不提交 Plan；用户知晓 pending 的后果 |
| PC-09 | 删除已提供诊断证据的报告 | 证据不可再用、临床资格重算、复核提示，不自动撤销 ConditionEntry/解除 Safety |
| PC-10 | 删除源对象成功、衍生数据库步骤失败 | 状态仍 retryable_failed，能恢复，源不能用于新分析 |
| PC-11 | 报告还有独立的另一来源支持 Observation | 不能误删其他来源，重算只涉及真正受影响证据 |
| PC-12 | 越权读取/操作别人的 Conversation、Asset、Interaction | Go 后端拒绝；不依赖前端过滤 |
| PC-13 | ToolCall succeeded 但 DomainCommitReceipt 未提交 | UI 不报告“已保存”，没有假 Revision |
| PC-14 | 公开分享撤销后重新访问令牌 | 服务端撤销可验证，受控快照纳入清理 |
| PC-15 | 擦除包含健康文字的来源后回放 | 只显示 redacted/tombstone，不重新泄露已擦除原文 |
| PC-16 | Person 记录“医生告诉我患 X” | 来源是用户转述，不冒充临床文件，也不绕过 Diagnosis 采纳 |

上述先写成测试契约，决策获批后增加 Go 权限/事务测试、文件与事件集成测试、浏览器端中断恢复 E2E。

## 10. 批准后工程迁移（尚未开始）

1. 锁定旧代码 Characterization tests：Conversation 软删与 share 撤销、Upload 真实删除范围、Interaction CAS、当前 Token 权限。
2. 将已批准 PC-/CD- 决定分别更新到 `domain/person-chat.md`、`domain/README.md`、`requirements.md`、`agent-workflows.md`、`decisions.md`、`acceptance.md`；保留本稿作为评审记录。
3. 先引入兼容的 Profile/资产/Conversation/Interaction DTO 与来源引用、真实提交回执；旧 API/历史数据有迁移路径。
4. 优先跑通“报告删除→停止使用→持久任务→衍生证据清理→安全保守/诊断复核→回执”的隐私纵切；再跑通“原 Run 的自由补充→领域事实写入→提案新证据复核→恢复”的 Chat 纵切。
5. 做跨域重试、并发、多设备、鉴权、来源证据撤销与无来源情形的全链路验收。独立代码 PR，不在本轮文档 PR 中静默修复。

**证据缺口：** 还未扫描真实线上数据、私有对象存储、搜索缓存和旧共享令牌；Hark 所述未推送领域草稿尚未找到。禁止声称清理已完成、分享无人使用或所有遗留规则已经逐条合并。
