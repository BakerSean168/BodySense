# BodySense · Person / Chat + 跨领域契约（阶段三）

> **状态：产品业务契约已批准（PC-D1～PC-D9、CD-D1～CD-D4；2026-10-13）。** 本文定义目标逻辑模型、授权、生命周期和不可违反的边界，不代表 Go、Python、数据库、前端或生产环境已完成迁移。阶段二 BodyState、Diagnosis V2、Training/Trainer、SafetyState 的已批准契约仍是各自领域的唯一权威；参见 [领域总览](./README.md)、[需求](../requirements.md)、[Agent 工作流](../agent-workflows.md)、[验收](../acceptance.md)。

## 1. 权威归属与 Person 组合视图（PC-D1～PC-D3）

| 对象 | 权威内容 | 不得持有 |
| --- | --- | --- |
| User / Auth | 用户登录身份、凭据、角色及访问授权 | 第二份身体档案 |
| Profile | 可选基础个人资料、用户偏好（PC-D1） | 身高体重的时间事实、症状、病史、训练习惯或医学结论 |
| EquipmentAsset | 用户拥有、借用或可使用的器材及可用状态（PC-D2） | TrainingPlan 处方、Trainer 的实际训练事实 |
| UserAsset | 上传原始文件的唯一身份、私有存储与文件删除生命周期（PC-D3） | OCR 准入决定、BodyObservation、Diagnosis |
| ExtractionRun / EvidenceCandidate / Admission | 版本化的文件感知、解析、证据来源定位与医学证据准入 | 用户采纳的医学诊断结论 |
| Conversation / Chat Runtime | 唯一持续对话、短期消息、运行、Ask User 与清理游标（PC-D4～D9） | 第二份 BodyState、Diagnosis 或训练处方 |
| BodyState | 身体事实、Observation、Episode、用户本人 Hypothesis 及 SafetyState | Agent 推测出的临床确诊 |
| Diagnosis | 用户唯一一份综合诊断、具名 ConditionEntry、正式分析与授权修订 | 无来源的身体事实或第二份 SafetyState |
| TrainingPlan / Trainer | 分别保存已接受处方与实际执行记录 | 以聊天文字伪造执行完成、绕过安全门禁 |

**Person 是只读聚合视图和经授权的操作入口，不是额外的健康数据聚合。** Person 保留 **Records / Equipment / Media / Reports / Habits** 五个分类；身份资料在窗口标题区、通过 Identity Popover 编辑，不设 Profile 第六标签页。Records 与 Habits 查询 BodyState；Equipment 查询 EquipmentAsset；Media 与 Reports 是 UserAsset 的分类视图。同一明确本人陈述由所属领域校验并提交，不因它来自 Chat 或 Person 而改变写入权威。

### 1.1 Profile：极简可选资料（PC-D1）

逻辑字段：`userId`（唯一）、`displayName?`、`avatarAssetId?`、`birthDate?`、`gender?`（如提供，明确其自报语义，不作为未经证实的医学/生理判定）、`preferences? { locale?, timeZone?, unitPreference? }`、`updatedAt`、内部 `revision`。头像指向私有资产时受相同归属/删除校验。新字段默认可选，不因缺少资料阻断 Chat 或健康服务；年龄从可用出生日期按需要计算，不持久成第二权威。账户邮箱、口令、角色仍由 User/Auth 管理。

- 时区使用 IANA 标识；默认值可以存在，但缺省/推断必须可识别。相对日期解释需保留事件当时的时区/语境；更改偏好不重写既有训练日、Observation 原始测量时间或历史发生日期。
- 单位偏好仅改变展示/输入解释，不篡改 BodyObservation 的原始数值、单位及来源。必要的医学相关生理信息应具有其本身的明确含义和来源，不从普通 Profile.gender 自动推断。
- 用户可查看、更改、清空可选字段；后端按 userId 授权、字段验证和并发修订，普通界面不显示技术 revision。Profile 与 Auth 数据不可无授权互相复制。

### 1.2 EquipmentAsset：可使用资源（PC-D2）

~~~ts
type EquipmentAsset = {
  equipmentId: string;
  userId: string;
  displayName: string;
  accessKind: "owned" | "borrowed" | "facility_access";
  availability: "available" | "unavailable" | "retired";
  validFrom?: string;
  validUntil?: string;
  sourceRef?: SourceRef;
  createdAt: string;
  updatedAt: string;
};
~~~

器材实体表示「用户能否使用这一资源」，不是使用次数或生活习惯。「每周用哑铃训练三次」属于有时间语义的 BodyFact（`lifestyle` 或适当类别），不重复写入 EquipmentAsset。用户可新增、编辑、标记暂不可用和停用器材；停用不伪造过去的器材使用历史。器材状态变化只触发对相关 TrainingPlan 的**影响检查/修改建议**，不会自动修改已接受的处方或 Trainer 日志。

### 1.3 统一文件资产、多分类视图（PC-D3）

~~~ts
type UserAsset = {
  assetId: string;
  userId: string;
  facets: ("media" | "report")[]; // 展示分类，同一文件可同时具有两种视图
  originalName: string;
  mimeType: string;
  byteSize: number;
  contentHash: string;
  privateStorageRef?: string;
  state: "available" | "deleting" | "deleted";
  createdAt: string;
  deletedAt?: string;
};
type DocumentExtractionRun = {
  runId: string;
  assetId: string;
  mechanismVersion: string;
  processingState: "queued" | "processing" | "succeeded" | "failed";
  retryOfRunId?: string;
};
type EvidenceCandidate = {
  candidateId: string;
  extractionRunId: string;
  sourceLocator: string; // 页、文字框或表格单元格等
  admissibility: "admissible" | "needs_review" | "rejected";
  admissionDecisionId?: string;
};
~~~

一份医疗报告照片可同时显示在 Media/Reports，但具有**一个 assetId、一个原始存储身份和一个删除目标**。上传完成、OCR/抽取成功、证据准入、正式 Diagnosis 采纳分别是不同事件。重处理创建新 ExtractionRun，保留对应配置和源证据版本，不原地篡改旧准入决定；准入遵循 [健康文件证据管线](../../architecture/health-document-evidence-pipeline.md) 与 ADR 0013。分析从 Chat 发起，不另建「上传即诊断」流程。

## 2. Chat：每用户唯一 Conversation（PC-D4～PC-D5）

### 2.1 唯一身份、串行执行

每个 User 有且只有 **一份稳定的 Conversation**（目标后端唯一约束）；首次需要时可惰性创建。不存在新建会话、会话列表、会话标题/置顶/归档和切换会话等用户功能。用户关闭/收起 Chat 或更换设备，不删除 Conversation，不隐式接受/拒绝任何业务提案。旧多会话及其记录必须经过显式数据迁移、去重、来源/Run/互动映射后再退役，不能仅把多份历史指向同一 ID。

~~~text
User (1) ── Conversation (1)
              ├─ Message[]                 滚动近期消息，seq 单调递增
              ├─ AgentRun[]                按轮独立执行；至多一个活动中的普通 Chat Run
              ├─ PendingInteraction?       当前占用输入区的 Ask User 工具
              ├─ WorkingContext?           当前任务的短期摘要；不是第二份 BodyState
              └─ ChatRetentionCursor       处理、上下文与物理清理边界
~~~

一次用户输入开始独立 AgentRun；Run 结束后释放通道，下次输入仍进入同一 Conversation。后端必须对同一用户/Conversation 的活动 Run 和待回答交互施加并发约束；多个设备提交竞争时返回当前 Run/可恢复交互及明确的忙碌或冲突结果，不创建第二条会话或并发覆盖状态。

### 2.2 Ask User 阻断式工具交互（PC-D5）

~~~text
idle ──用户发送──> running
running ──正常完成──> completed ──> idle
running ──ask_user──> waiting_for_user
waiting_for_user ──有效回答──> resume(original run) ──> running
waiting_for_user ──断线/关闭窗口──> durable pending ──> 恢复同一 interaction
waiting_for_user ──用户停止──> cancelled
waiting_for_user ──过期/底版失效──> stale/expired ──> 不继承旧授权
~~~

- `waiting_for_user` 时**普通聊天输入区由 Ask User 工具选项替换**，禁止自由发送另一个普通 Chat 消息。自定义解释或补充材料若被允许，应作为当前 Ask User 的答案控件，精确绑定 `interactionId / runId / toolCallId`。
- 回答、恢复、取消、超时等由持久状态确定；重复提交相同答案幂等，不同答案竞争时拒绝或要求重新确认，不能双重执行。UI 的 loading/hidden/collapsed 不等于 Run/Proposal 状态。
- Diagnosis 正式分析可拥有多个 ConditionProposal，但**逐项串行呈现确认工具**。工具答复只是 ProposalDecision；已决定但未提交可在刷新后恢复，全部有效决定按 Diagnosis V2 一次原子提交。新证据影响已有决定时按 DX-D15 仅重新确认受影响提案；真正 urgent 的安全阻断遵守零提交终结。
- 若处于阻断交互，用户不通过普通输入框开启其他 AgentRun。用户可按明确的停止/取消流程结束当前运行；关闭 Chat 窗口不等于拒绝提案或擦除决定。

### 2.3 滚动历史窗口与长期记忆（PC-D6、PC-D9）

**Conversation 长期存在，聊天原始 Message 不永久存在。** 模型上下文与 UI 历史均采用滚动窗口；BodyState 等领域持有真正长期信息。语义提取在日常消息处理时及时进行，不能只等压缩才写入事实；普通寒暄、知识问答和无需长期存储的 AI 回复不强行生成事实。

~~~ts
type ChatRetentionCursor = {
  conversationId: string;
  processedThroughSeq: number; // 语义处理已完成且必要领域提交确认
  contextStartSeq: number;      // 模型仍使用原始消息的起始 seq
  prunedThroughSeq: number;     // 原始消息和受控运行副本的安全清理边界
  updatedAt: string;
};
~~~

- `prunedThroughSeq <= processedThroughSeq`；物理清理不得越过当前上下文、正在执行的 Run、仍待答的 Interaction、未结算的提案、未完成提交或恢复依赖。截断只能在完整的用户/助手/工具交互边界发生，不能留下孤立工具调用或工具结果；消息序号不得因清理重新编号。
- 先抽取明确身体信息，再由所属领域校验并获得真实提交回执，再推进 `processedThroughSeq`；没有长期价值的已处理消息可明确标记无需入库。不应依靠自然语言摘要作为医学权威。
- **PC-D6 方案 B**：保存结构化事实与**必要的原始陈述片段**，附准确来源类型、版本、时间和解释语境。原 Message 被删后，长期证据指向所属领域持有的持久 SourceEvidence，而不是悬空 Message ID；不把完整聊天历史改名为永久证据仓库。
- `contextStartSeq` 根据 Token 预算、近期完整交互与当前 Run 的恢复需求推进；按需读取领域事实及短期 WorkingContext，而非累积永久无限摘要。UI 近期窗口保留合理的连续消息，具体数量/保留期限属后续可调整实现参数，不在业务层规定固定天数或条数。
- `prunedThroughSeq` 仅在数据库 Message、运行时 Checkpoint、临时摘要、原文缓存、检索索引和受控副本的清理/引用替换完成并核验后推进。物理清理异步失败应可重试；**模型窗口截断、UI 不显示和数据真正擦除是三回事**。对涉及隐私擦除的敏感原文，不能靠不可变审计或旧 Checkpoint 将其偷偷留存。
- 对来源纠错、用户撤回和隐私擦除分别保留合法领域操作与最小审计；撤回（withdrawn）不表示已擦除，擦除也不等于症状已康复或诊断已停用。

## 3. 文件删除、证据历史与隐私擦除（PC-D7～PC-D8、CD-D4）

### 3.1 两种不同的删除操作（PC-D7 方案 B）

| 用户操作 | 文件/证据处理 | 不得发生 |
| --- | --- | --- |
| **移除原始文件** | 使 UserAsset 原件不可用，移除源 PDF/照片与不再需要的原文 OCR/完整识别缓存；保留已合法采纳的结构化 Observation、必要来源片段、历史准入/版本标识和最小可追溯性，并向用户明确告知留存范围 | 暗中保留整份原始文件副本；将历史观察一并静默删除；假称原件仍可核验 |
| **删除文件及关联健康数据** | 经用户明确选择范围后，阻止源和衍生内容再用于推理，随后可重试地清理原始对象、派生 OCR/缩略图/索引/缓存、涉及的结构化证据及敏感审计载荷，并更新依赖投影/复核提示 | 仅做 soft-delete 或 withdrawn 就宣称隐私擦除完成；删除不相关或独立来源的合法事实 |

文件原件删除不会让已合法准入的结构化健康观察自动失去历史资格，但会改变**当前原件可用性**。涉及隐私删除必须提供明确作用范围和执行状态，所需受控复制品均纳入清理；如果存在依法必须保留的材料，其例外和期限必须依据实际合规要求，而非由 Agent 随意假设。

### 3.2 历史准入 ≠ 原件现时可核验（CD-D4 方案 A）

对文档来源至少区分 `historicalAdmission: admissible / needs_review / rejected` 和 `sourceAvailability: original_available / original_unavailable / erased`（逻辑维度；具体表设计可不同）。

1. 仅「移除原始文件」：历史准入结论与结构化 BodyObservation **可以继续作为明确注明历史来源、原件不可用的健康证据使用**，其可信范围不扩大。不能拿历史准入冒充「现在有可重新查验的原始临床文件」。
2. 某项新业务操作若要求可核验的临床原件（例如专业放行证据必须符合指定来源条件），原件已不可用时**不得自动放行**；需补充当下符合要求的独立证据。用户转述、留存数值、历史 OCR 片段不提升为可核验医疗文件。
3. 真正擦除关联健康数据后，受擦除范围的来源和证据**立即停止新使用**，历史可见性按脱敏/最小审计规则处理；同一观察若另有独立有效来源不能连带误删。
4. 证据资格和来源可用性的变化可以触发 Diagnosis 复核提示及 SafetyState 证据检查，但**不得自动修改已确认 ConditionEntry、推断疾病治愈或解除任何现有安全限制**。诊断正式更新仍遵守 DX-D5～D15；安全限制的解除仍遵守 SAFE-D1～D8。

### 3.3 可验证、可恢复的擦除协议

~~~text
requested -> authorized + scoped -> blocked_from_new_use
         -> propagating:
            original objects + thumbnails + OCR/evidence payload
            caches / search indexes / snapshots / runtime checkpoints
            affected BodyState evidence + derived projections
            Diagnosis reviewRecommended + conservative Safety recheck
         -> completed | retryable_failed (the source remains blocked)
~~~

先阻止新使用，后在授权范围内执行多步物理清理；对象存储与数据库不共享事务，不可把单步删除成功说成「所有副本已擦除」。持久删除任务需支持幂等重试与逐步核验，保留不含敏感原文的最小操作回执。旧 ConversationShare 对外公开功能**不纳入新目标产品（PC-D8）**，旧接口、令牌与历史快照仍须在迁移中审计、关闭/撤销及纳入适用的擦除传播；不能宣称已在运行环境关闭。

## 4. 跨领域引用、提交与授权（CD-D1～CD-D3）

### 4.1 统一来源身份（CD-D1）

~~~ts
type SourceRef = {
  userId: string;
  sourceKind:
    | "user_statement_evidence" | "user_asset" | "evidence_candidate"
    | "body_fact" | "body_observation" | "symptom_episode"
    | "training_session" | "direct_user_action";
  sourceId: string;
  sourceVersion?: string;
  observedAt?: string;
  recordedAt: string;
  locator?: string; // 医疗文件页/框/表格，或训练动作/组等
};
~~~

这是跨域**语义协议**而非必须强制所有 API 使用同一个万能 DTO。每个 SourceRef 验证 userId、稳定目标及适用的版本/位置；聊天来源在消息清理前转换为持久且最小的 `user_statement_evidence`，不得引用已物理删除的孤立消息。Diagnosis 对 BodyFact/Observation/Episode/文档证据使用版本化引用，分别标记 supporting/counter/context；不能由同一 Episode 推出医学因果，也不能重复计权。

### 4.2 业务写入与事件传播（CD-D2）

- Go 领域服务分别校验用户身份、授权内容、当前修订、来源资格与幂等键并提交。**ToolCall succeeded、Agent 说「已保存」、前端工具卡结束均不代表领域数据写入成功**；只有实际持久化成功的 DomainCommitReceipt（或等价的受确认领域结果，含目标/修订/事件标识）才能告知用户成功。
- 一次语义操作在**所属领域内部**原子提交（BodyState 单次事实修订、Diagnosis 一次 Run 全部有效决定一次整体 Patch、TrainingPlan 授权修订、Trainer 真实动作记录）。跨领域不得谎称存在一个总原子事务；明确局部成功和失败，安全依赖不可用时按 Safety 门禁保守处理。
- 跨域影响使用事务 Outbox 或等价的**持久、可靠、幂等、可重放**事件机制；消费者仅执行获允许的投影重算、影响检查、复核建议，不可用事件自动接受诊断或更改训练处方。网络重复、乱序或异步失败不产生重复业务修订。

### 4.3 精确授权（CD-D3）

~~~ts
type DomainCommand = {
  userId: string;
  targetDomain: string;
  targetId?: string;
  operation: string;
  sourceRefs: SourceRef[];
  idempotencyKey: string;
  expectedRevision?: string;
  authorization?: {
    interactionId?: string;
    proposalId?: string;
    contentFingerprint?: string;
  };
};
~~~

Chat 仅发起操作和保存交互状态；Go 验证工具回答是否确实属于当前 Run/目标/提案/证据版本。普通明确本人事实依既定 BS-D8 自动入库，**不增加一轮无意义确认**；关键更正/关系冲突通过 Ask User 取得精确授权；正式 Diagnosis 提案、计划 Proposal 及安全放行分别遵守自身更高授权门槛。不能把对 A 病名、A 计划或旧内容指纹的许可复用给 B；并发底版过期时拒绝陈旧提交并按领域规则重新核验。

| 来源/事件 | 下游只允许 | 不允许 |
| --- | --- | --- |
| 明确本人症状/习惯 | BodyState 真实保存；必要时触发 Episode / 诊断相关性与安全检查 | 聊天直接创建/升级医学诊断 |
| 经过准入的报告候选 | 写入有来源的 BodyObservation，建议正式诊断复核 | OCR 即确诊、自动改 ConditionEntry |
| 用户接受指定诊断提案 | 原 Run 留 Decision；有效决定一次 Diagnosis 原子修订 | Ask User 完成即宣称诊断已提交 |
| 器材、身体事实或训练反馈改变 | 检查相关计划并提出准确建议；SafetyState独立执行门禁 | 静默修改已接受处方或伪造训练完成 |
| 医疗文件原件移除 | 标记原件不可用，保留有限历史准入，按新操作要求重新核验 | 擅自清除已有诊断或自动解除限制 |
| 医疗来源真正擦除 | 立刻阻止再使用，擦除关联数据与缓存，触发相关性/安全复核 | 假称软删除=擦除，自动停用诊断或自动解除限制 |

## 5. 用户界面必须表达的状态

| UI | 必须显示/区分 | 不显示/不提供 |
| --- | --- | --- |
| Person 标题/Identity Popover | 可选资料、可编辑身份与偏好、正确空态 | 第六个 Profile 标签、内部修订号 |
| Person 五分类 | Records/Habits 权威数据、Equipment 可用性、Media/Reports 唯一资产与状态 | 演示假数据、第二份 BodyState |
| Reports | 上传/处理中/失败/重试与证据 admissible/needs_review/rejected 独立；历史准入但原件不可用有明确提示 | OCR 成功即确诊、原件已删却展示为可核验 |
| Chat 默认状态 | 唯一近期消息窗口、普通输入区、当前 Run 进度 | 会话列表、新建会话、归档/置顶 |
| Chat 等待 Ask User | 输入区整体转为准确的选项/自定义解释控件；刷新与重新连接可恢复 | 同时自由普通发送、用新的提案替换旧授权 |
| Chat 删除/清理 | 自动滚动，必要的持久事实独立留存；明确区分文件移除与数据擦除 | 把历史窗口截断称作个人信息已擦除 |

## 6. 旧代码差异与迁移门禁（As-Is，非完成声明）

- Go `UserProfile` 仅有 Gender/BirthDate/派生 AgeYears；目标可选显示字段/偏好、授权及历史时间快照尚需补全。旧 `UserUpload` 同时存 OCR/姿态分析状态，尚未证明具有 UserAsset 单一身份与来源删除传播链。
- Go `Conversation` 存在多会话标题、置顶、软删除及分享形态；`MessageRepository.ListByConversationID` 可读取整段原始历史。目标单会话、三个保留游标、历史清理及旧会话迁移**尚未实现**。
- Python Consultation Thread 对发往模型的历史做长度截取；LangGraph PostgreSQL Checkpointer 仍持久保存运行消息及恢复信息。**模型上下文截取不等于 Message/Checkpoint 真正清理**。
- 前端已有 `interrupted` 锁 Composer 与 Ask User 卡片机制，但尚须验证「Ask User 直接替换底部输入区」、跨设备串行及待答恢复的目标行为。
- 当前 `UploadService.DeleteUpload` 删除原存储对象与上传记录；未证实能区分两类用户删除意图或可靠清理/保留各类衍生证据。老公开分享端点/快照需单独退役与令牌治理。
- 必须先有旧数据来源映射、字段/状态兼容、版本/删除/事务表征测试，再分阶段迁移；不得以本契约「已批准」宣称生产实现/医学安全或个人数据擦除已经验收。

## 7. 验收与独立遗留问题

阶段三的 PC/CD 场景见 [acceptance.md](../acceptance.md) 中 `PC3-*`、`CD3-*` 测试；域内契约仍由 [BodyState](./bodystate.md)、[Diagnosis](./diagnosis.md)、[Training](./training.md)、[Safety](./safety.md) 拥有。

**本阶段产品建模已闭合，但下列项目不属于新的阶段三产品决策：** Diagnosis 正式提案有效期限（Q-17，仍单独待定）、Diagnosis 确认工具 Penpot 组件/窄屏/文案（Q-19，交互视觉待设计）、临床安全阈值与专业审核、旧数据物理迁移和隐私合规保留规则。它们不得被实现方以默认值冒充已获批准。
