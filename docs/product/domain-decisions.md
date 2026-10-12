# BodySense · 领域契约独立重建决策登记

**状态：独立重建；BodyState/Diagnosis、Training/Safety、Person/Chat + 跨领域的核心产品语义均已收敛（PC-D1～D9、CD-D1～D4），物理迁移、隐私擦除、临床审核及独立剩余议题另行跟踪；未与 Hark 原草稿核对。** GitHub 未保存 Hark 所述的 `ec817b8` 和五份 `work/domain-drafts/`；此表依据已合并的产品规格、当前 Go/React/SQL 源码重新梳理决策问题。条目与 Hark 所说的 83 项、21 项 P0 **没有逐条对应关系**，不得把独立重建的数量作为原草稿恢复率。DC-* 是本轮独立编号，后续如找到原文件再以语义、来源和影响范围去重映射。

**本表仍保留全部 70 个原始议题（含 19 个 P0），因此这些总数并非「仍待决定」数量。** Diagnosis DX-D5～D15 已决原则见 [decisions.md](./decisions.md) 与 [Diagnosis V2](./domain/diagnosis.md)；旧 Diagnosis V1 的描述性病名、通用 lifecycle、用户按钮调整 suspected/likely、临床确诊等级和 mark_resolved 均已退役，不能用旧表述覆盖 V2。 BS-D1～D10、TP-D1～D12、SAFE-D1～D8 与 PC-D1～D9、CD-D1～D4 的已决原则见 [decisions.md](./decisions.md)。DC-001～006、010～014 的**产品语义**已批量确立；DC-022、052～065 的核心行为也已定，其中物理 DTO、旧数据迁移、安全阈值专业审核与并发实现仍需工程验证。与旧系统迁移、隐私清除等独立细则仍按原 ID 跟踪。

**决策归属**：P0 原为需要产品负责人拍板的业务或隐私约束；其中部分已根据 2026-10-13 的一揽子决策结案，不再视为待决。P1/P2 混合业务语义与工程迁移问题：例如 DC-028/029/030/038/044/046/052/053/064/070 可以先由工程方提供实现方案和验证，但不得突破已获批准的不变量；涉及用户可见行为或权限改变时仍需产品批准。任何未决定的业务语义都不能写成正式默认值。`pending` 在本表中表示决策尚未结案，而非产品的业务状态。

| ID | 优先级 | 议题与本轮已决原则（未标已决者仍待处理） | 决策将影响 | 原有问题 |
| --- | --- | --- | --- | --- |
| DC-001 | P0 | **产品门禁已定/医学规则待验证（SAFE-D2）**：产品执行等级为 advisory/restricted/urgent，分别提醒/匹配阻断/全训练阻断；医学规则专业验证独立；工程映射/测试另行验收 | SafetyRestriction 与 Trainer 门禁 | Q-18 |
| DC-002 | P0 | **产品门禁已定/医学规则待验证（SAFE-D4）**：必须按有版本的结构化规则及来源证据确认风险；Agent 不得自由文本降级；真正 urgent 才能中断正式分析；工程映射/测试另行验收 | 安全分类器与 Agent | Q-18 |
| DC-003 | P0 | **业务原则已定（SAFE-D3）**：作用范围 all_training/plan/exercise，按 exerciseId 判断，同一范围多个限制取最严格门禁；工程映射/测试另行验收 | SafetyState.scope | 新 |
| DC-004 | P0 | **业务原则已定（SAFE-D5）**：on_expiry 仅审核过的低风险明确期限；用户复核或专业放行需满足结构化条件与来源，非自动策略到期仅提醒；工程映射/测试另行验收 | liftConditions / liftBasis | Q-18 |
| DC-005 | P0 | **业务原则已定（SAFE-D7）**：真正 urgent 的 Diagnosis Run 终结并不可恢复；风险处理后用户新发起，普通 scoped restriction 不终结分析；工程映射/测试另行验收 | Run/Diagnosis 与安全策略 | 新；现有 INV-23 只定义原运行不可恢复 |
| DC-006 | P0 | **业务原则已定（SAFE-D6）**：正在执行的匹配 restricted 暂停，urgent 安全终止；保持真实 Action，不伪造 skipped；工程映射/测试另行验收 | TrainingSessionLog / SafetyRestriction | 新 |
| DC-007 | P0 | **原则已决（BS-D8）**：明确本人习惯自动写入 BodyFact；仅关键歧义/特定证据准入另行处理，Habits 不复制事实 | Person / BodyFact | 新 |
| DC-008 | P0 | **原则已决（BS-D8）**：明确的本人身体陈述自动写入所属权威领域；Records 不绕过 Diagnosis 确认，也不持有第二份事实 | Person / BodyState | 新 |
| DC-009 | P0 | **Diagnosis V2 原则已决（DX-D15）**：具名医学提案逐项 accept/reject/继续补充，一次原子提交；卡片具体文案、组件与响应式展示仍待 Q-19/Penpot | Chat / Diagnosis UI | Q-19 |
| DC-010 | P0 | **业务原则已定（TP-D6）**：默认 frequency_only，calendar 必须显式安排，缺练与补练不自动记录；工程映射/测试另行验收 | TrainingPlan 排程与训练记录 | 新 |
| DC-011 | P0 | **业务原则已定（TP-D7）**：exerciseId/planExerciseId/templateId/sessionLogId 身份分离且可跨修订追溯，同名不当作同 ID；工程映射/测试另行验收 | exerciseId / 历史回放 | 新 |
| DC-012 | P0 | **业务原则已定（TP-D1）**：计划 active/paused/completed 由用户显式转换、可重新启用，状态事件与内容修订分离；工程映射/测试另行验收 | TrainingPlan.status | 新 |
| DC-013 | P0 | **业务原则已定（TP-D8）**：跳过只支持用户明确的 set/exercise 粒度，提前结束另用 endedReason；无自动组完成；工程映射/测试另行验收 | Session/Set 记录 | 新 |
| DC-014 | P0 | **业务原则已定（TP-D2）**：Draft 为尚未接受的训练计划预览/创建提案，不是已生效 Plan 或 Trainer Session；工程映射/测试另行验收 | Trainer/UI | Q-15 |
| DC-015 | P0 | **业务已决（PC-D1）**：极简可选 Profile 与偏好；身份弹层编辑，身体时间事实属于 BodyState；敏感资料展示按归属/授权；DTO/脱敏实施另验收 | Person / Profile | 新 |
| DC-016 | P0 | **业务已决（PC-D4/D9）**：每用户唯一持续 Conversation，按安全游标滚动清理近期消息；历史多会话与 Checkpoint 迁移待工程验证 | Chat/Agent 运行时 | 新 |
| DC-017 | P0 | **业务已决（PC-D5）**：单活动 Run，Ask User 替换普通输入区，补充解释仅通过当前工具回答并恢复原 interaction/run；并发/刷新测试待验收 | Chat 输入、ask_user 恢复 | 新 |
| DC-018 | P1 | 病情/计划提案超时时长、配置和提醒策略 | Proposal 生命周期 | Q-17 |
| DC-019 | P1 | **已决（BS-D1）**：historical 只属于时间语义，不再是 BodyFact 的生命周期状态；记录状态独立为 asserted/superseded/withdrawn | BodyFact 目标契约 | 新 |
| DC-020 | P1 | **原则已决（BS-D9）**：仅用户本人假设；可按原始陈述更正、撤回并保留关联事实/观察；AI 旧数据迁移仍待 DC-029 | BodyHypothesis | 新 |
| DC-021 | P1 | **Diagnosis V2 原则已决（DX-D5/D8）**：症状缓解/复发属于 BodyState SymptomEpisode；ConditionEntry 是具名医学诊断概念，不持有通用 lifecycle，重新评估不自动修改 Diagnosis；Episode 引用物理迁移待 DC-066 | Diagnosis / BodyState 边界 | DX-D5/D8、DC-066 |
| DC-022 | P1 | **业务原则已定（TP-D6）**：session trainingDate 按开始时 IANA 时区快照，跨午夜不拆、同日多次独立记录；工程映射/测试另行验收 | TrainingSessionLog.date | 新 |
| DC-023 | P1 | **业务已决（PC-D2/D3/D7）**：独立器材资源、Media/Reports 统一 UserAsset，原文件移除与健康数据擦除不同；物理 Schema 和迁移待工程验证 | Person 资产 | 新 |
| DC-024 | P1 | **原则已决（DX-D7/D8）**：已停用的医学诊断同概念重新采纳沿用 conditionId，需新分析、准确确认与历史审计；具体拒绝指纹与并发复用待 DC-068/070 | disposition 与重采纳 | DX-D7/D8 |

| DC-025 | P1 | 旧 `BodyStateFact.ReviewState` 和 `Trend` 是否进入新 BodyFact，还是只作为兼容投影 | BodyFact / 历史迁移 | [BodyState As-Is](./domain/bodystate.md#28-当前-go-实现映射as-is不是新规格) |
| DC-026 | P1 | 旧 `Kind` 类目（`lifestyle.*`、`history.*`、`equipment.available`）如何对齐新 `symptom/activity/lifestyle` 并保持检索稳定 | BodyFact 类目与旧数据 | 同上 |
| DC-027 | P1 | **部分已决（BS-D9）**：原始值/单位、规范化值/单位、测量方法/条件、时间、来源与纠错链均为逻辑字段；具体单位校验/量纲/准入回评规则待实现契约 | BodyObservation 逻辑 Schema | [BodyState](./domain/bodystate.md) |
| DC-028 | P1 | **原则已决（BS-D9）**：Observation 的陈述状态、来源等级与文档候选准入分离；旧 unverified/confirmed/rejected 数据的逐项迁移及重新准入策略仍待技术方案 | 证据准入与推理资格 | [证据管线](../architecture/health-document-evidence-pipeline.md) |
| DC-029 | P1 | 旧 AI Hypothesis 历史记录如何迁移到分析审计；用户假设如何保留可撤回的证据关联 | BodyHypothesis 与 Run 迁移 | IG-05 |
| DC-030 | P1 | BodyStateRevision 的结构化变更操作、不可变依据、关联 sourceId 和重放保留策略 | BodyStateRevision | [旧模型](../../apps/api/internal/model/body_state.go) |
| DC-031 | P1 | **已决（BS-D1/D3/D4）**：不设 historical 生命周期；事实纠错与真实变化分离，Episode 已结束后新发作为新身份而非恢复旧事实 | BodyFact / SymptomEpisode | DC-019 延伸 |
| DC-032 | P2 | ConcernGroup 的稳定分组键、合并/拆分、派生更新及展示顺序应如何定义 | DerivedConcernGroup | INV-25 |
| DC-033 | P1 | **业务已决（PC-D6/D9、CD-D1）**：消息安全滚动清理前持久化必要原话片段、结构化事实和来源版本；撤回、纠错、隐私擦除不同；跨缓存/审计的物理清理待验证 | BodyFact provenance / 隐私 | G-04 |
| DC-034 | P1 | **业务已决（PC-D3、CD-D4）**：唯一 UserAsset 的文件状态、ExtractionRun 任务状态和 EvidenceAdmission 准入状态分离；原件不可用提示与重处理 UI 待实现 | Person 资产 UI | UserUpload |
| DC-035 | P0 | **产品语义已决（PC-D7、CD-D4）**：区分移除原件并保留历史准入与擦除关联健康数据；原件不可用时高要求业务必须重新核验。衍生/缓存/审计及合规擦除的真实执行仍为上线门槛 | Person 隐私与证据完整性 | R-PS-03 / G-04 |
| DC-036 | P1 | **业务已决（PC-D4/D5/D6/D9）**：不归档多会话；唯一 Conversation 持久，Ask User 可恢复；已保存领域来源片段后滚动清理旧消息；迁移/游标/物理擦除待实现 | Conversation 生命周期 | [Person/Chat](./domain/person-chat.md) |
| DC-037 | P1 | **业务边界已定（PC-D5/D9）**：Message、AgentRun、PendingInteraction、领域 Proposal 状态相互分离；完整交互边界和清理游标已定，流式 DTO/错误重试物理实现另验收 | Message 状态机 | 同上 |
| DC-038 | P1 | **业务原则已定（PC-D5、CD-D3）**：Ask User 回答只属于原交互/Run；超时/底版过时不得重复消费授权，用户决定与领域提案的真实提交分离；CAS/跨设备恢复待技术验证 | 互动状态机 | 同上 |
| DC-039 | P0 | **产品已决（PC-D8）**：目标不提供健康对话公开分享；旧 ConversationShare 公共接口、令牌、快照需审计及退役治理（现网是否仍可用未验证） | 健康对话隐私 / 分享 | IG-06 |
| DC-040 | P1 | **业务已决（PC-D1）**：偏好作为极简 Profile 的逻辑子结构；IANA 时区/语言/显示单位变更不重写历史测量值或执行日；DTO 与时间解释验收待实现 | Person / 时间语义 | DC-015/022 |
| DC-041 | P1 | **业务已决（TP-D11、PC-D2）**：器材资源归 Equipment，使用习惯归 BodyState；器材变化只触发计划影响复核，不改已接受处方；迁移与测试待实施 | Equipment / TrainingPlan | DC-023 |
| DC-042 | P1 | **业务已决（PC-D3）**：统一 assetId 和唯一文件副本；Media/Reports 为同一资产的不同视图，不重复存储 | Person 资产身份 | DC-023 |
| DC-043 | P2 | **不纳入目标产品（PC-D4）**：多会话标题生成、置顶/归档等旧元数据无新 UI 需求；旧 Schema/接口清理及迁移由工程处理 | Conversation 元数据 | [旧模型](../../apps/api/internal/model/conversation.go) |
| DC-044 | P1 | **契约已决（CD-D2）**：只有权威领域真实提交回执才可报告「已保存」，ToolCall succeeded 不算；具体 event/commit DTO 与可靠传播待工程实现 | Agent 工具与领域授权 | INV-09 |
| DC-045 | P1 | **业务已决（PC-D5、CD-D3）**：工具交互在唯一活动 Chat Run 中串行呈现，Domain Proposal 决定独立持久；重复回答幂等、过期不可复用授权，顺序/锁实现待验证 | PendingInteraction | DC-017 |
| DC-046 | P1 | **业务已决（PC-D4/D5）**：不允许同一用户创建并行 Conversation，跨设备对唯一 Chat Run 做后端串行仲裁、明确冲突/恢复；锁和迁移协议待工程实现 | Conversation / Run | G-04 |
| DC-047 | P1 | **原则已决（DX-D8）**：conditionId 对应具名医学概念；同义规范化可保留、实质不同概念须新建；侧别/限定信息和同名去重的精确匹配机制仍待工程方案 | Diagnosis 身份 | DX-D8，DC-067 |
| DC-048 | P1 | **原则已决（DX-D6/D11/D12）**：来源多记录并存、临床原话与证据版本可追踪、来源资格变化仅建议复核；具体迁移与证据撤销传播仍待技术协议 | ConditionEntry 来源链 | DX-D6/11/12 |
| DC-049 | P1 | **原则已决（CD-D2）**：各领域内部原子，跨域可靠事件/结果分项回执，不伪造跨域事务；局部失败恢复/一致性技术方案待验证 | ConditionDirectEdit | INV-19 |
| DC-050 | P1 | **原则已决（DX-D15）**：补充证据后只重新询问受影响的已决定提案；内容指纹和交互卡的版本映射、网络中断恢复协议仍待工程细化 | ConditionProposal | INV-17 |
| DC-051 | P1 | ConditionAnalysisRun 基础修订过期后，是结束旧 Run 还是创建新 Run，如何展示已决但未生效的记录 | Diagnosis 运行恢复 | INV-12/16 |
| DC-052 | P1 | **业务原则已定（TP-D5）**：PlanPatch 逻辑动作和稳定 ID 目标已确定；具体物理 DTO 与兼容迁移待工程实现；工程映射/测试另行验收 | PlanPatch | R-TP-03/04 |
| DC-053 | P1 | **业务原则已定（TP-D5）**：Revision 不可变快照 + patch + source + authorization + parent；历史日志固定原 Revision；工程映射/测试另行验收 | TrainingPlanRevision | INV-07/09 |
| DC-054 | P1 | **业务原则已定（TP-D10）**：旧打卡值不能被推断成整次组完成，有真实明细才迁入 Action，否则保留为来源摘要；工程映射/测试另行验收 | TrainingSessionLog 迁移 | IG-01 |
| DC-055 | P1 | **业务原则已定（TP-D11）**：旧 Treatment 复核/暂停不能直接当新计划状态；复核提示派生、安全门禁独立；工程映射/测试另行验收 | 训练计划迁移 | IG-03 |
| DC-056 | P1 | **业务原则已定（TP-D10）**：每次执行不同 sessionLogId，同日多次允许；同用户最多一个 in_progress；工程映射/测试另行验收 | TrainingSessionLog 身份 | DC-010 |
| DC-057 | P1 | **业务原则已定（TP-D9）**：暂停期间不累计计时；断线和界面关闭不自动结束，恢复原 Session、未证实离线时长不当作训练；工程映射/测试另行验收 | Trainer 执行状态 | R-EX-03 |
| DC-058 | P1 | **业务原则已定（TP-D8）**：误触完成/跳过通过追加 corrected Action 更正，原事实留审计；工程映射/测试另行验收 | exerciseRecords | INV-26 |
| DC-059 | P1 | **业务原则已定（TP-D9）**：Session endedReason 区分 finished/stopped_early/safety_stopped，不能自动补全动作记录；工程映射/测试另行验收 | TrainingSessionLog | DC-013 |
| DC-060 | P1 | **业务原则已定（TP-D1）**：可以有多个 active Plan，completed→active 由用户显式重新启用、历史不覆盖；工程映射/测试另行验收 | TrainingPlan.status | DC-012 |
| DC-061 | P1 | **业务原则已定（TP-D6）**：已执行历史关联当次修订；未来显式课次需跟新修订一致，不自动生成补练；工程映射/测试另行验收 | 计划排期 / Revision | DC-010 |
| DC-062 | P1 | **业务原则已定（SAFE-D3）**：有效限制独立并存、按最严格门禁生效、逐条解除不误放行；工程映射/测试另行验收 | SafetyRestriction 范围 | DC-003 |
| DC-063 | P1 | **业务原则已定（SAFE-D5）**：证据更正只触发风险复评，不能因来源变化自行清除限制；工程映射/测试另行验收 | SafetyState 来源变化 | DC-004 |
| DC-064 | P1 | **业务原则已定（SAFE-D5）**：按 restrictionId 条件更新与 SafetyState 修订校验，避免解除覆盖并发新风险；工程映射/测试另行验收 | SafetyState 并发安全 | INV-09/20 |
| DC-065 | P1 | **业务原则已定（SAFE-D5）**：专业放行必须匹配具体限制和有效来源，用户转述保留等级、未准入文件不放行；工程映射/测试另行验收 | professional_clearance | DC-004 |
| DC-066 | P1 | **领域边界已决（BS-D3/D4、DX-D5）**：症状 Episode 的 ID/时间与复发只在 BodyState；Diagnosis 引用其证据版本，具体重放、引用重定向和旧数据迁移仍待 DC-030 | Diagnosis ↔ SymptomEpisode | DC-021 / BS-D3/D4 |
| DC-067 | P1 | **原则已决（DX-D8）**：不同医学概念不应直接合并为一个 ConditionEntry，同一概念保留稳定 ID；历史条目误合并、拆分、现有下游引用映射待工程方案和必要的用户授权 | Diagnosis 条目身份 | DX-D8/INV-39 |
| DC-068 | P1 | **原则已决（DX-D7/D8）**：inactive → adopted 需新分析及用户接受，旧撤回史保留；与历史拒绝的指纹去重和证据新鲜度具体计算待技术方案 | Diagnosis 重新采纳 | DX-D7/D8 |
| DC-069 | P1 | **业务语义已决（DX-D12）**：多临床声明独立保留且不覆盖，医生原始不确定性和有效来源分别展示；结构化映射与原资料撤销传播待实现 | Diagnosis 临床历史 | DX-D12 |
| DC-070 | P1 | **原则已定（CD-D2/D3、DX-D15）**：过期基线拒绝覆盖、保留原决定，按受影响提案重新核验；并发协议/提示细节待工程验证 | Diagnosis 并发 | INV-09/12 |


## 2026-10-13 产品领域决策状态汇总

- **Training 核心业务闭合**：TP-D1～D12 已将计划草案、完整处方、稳定 ID、频率与明确排期、状态、修订、逐组执行、用户明确结束、时区、反馈与授权统一。对应 P0 DC-010～014 不再逐题要求产品选择；DC-022、052～061 余下主要为 DTO、历史迁移、计时与执行工程验证。
- **Safety 产品策略闭合**：SAFE-D1～D8 已将 advisory/restricted/urgent、all_training/plan/exercise、最严格生效、安全暂停/安全终止、解除与复核审计、并发保护统一。对应 P0 DC-001～006 的**产品行为**已决定；**医学分类规则、阈值、临床适用性及专业验证仍是上线门槛**，不能由此宣称临床安全验证完成。
- **Person/Chat + 跨域产品原则闭合**：PC-D1～D9、CD-D1～D4 确定极简 Profile、统一资产、唯一 Chat/串行 Ask User、滚动历史、来源片段、两类删除与历史准入资格。原 DC-015～017、023、033～046 的业务选项已决定；旧会话/令牌治理、物理清理、数据擦除与临床安全审核仍是独立工程门槛。
- **独立剩余议题**：Diagnosis 提案有效期限 Q-17、医学确认组件具体视觉 Q-19，以及 Diagnosis/BodyState 的物理迁移、跨域幂等传播和隐私擦除合规实施；不再将这些称为阶段三的核心产品问题。
- **表格解释**：保留原 70 条及 P0/P1/P2 分级作为历史登记；本表 P0 的原始数量不是未决数量。标为「业务原则已定」的记录，工程实现和专业验证不自动视为已完成。

**已确定但不可改写的边界**：确有危险风险时终结当前 Run 的 INV-23、安全限制不能静默修改 TrainingPlan 的 INV-08、用户拒绝更新不撤回整个诊断的 INV-18、临床文件与模型分析不同来源的 INV-11/15、提案关闭不等于拒绝，均已是正式契约。上表只决策这些规则尚未覆盖的细节。

原 [open-questions.md](./open-questions.md) 的 Q-15、Q-18 已在 TP/SAFE 决策中结案，阶段三的 PC/CD 决策也已回填。Q-17 仅 Diagnosis 提案期限及 Q-19 的交互视觉细节仍开放，不能默认为阶段三未完成。临床安全规则上线前须专业审核，尚无验证的删除/迁移能力不得按已实现对外宣称。

## 仍待独立产品/专业评审的事项（阶段三已闭合）

阶段三 PC-D1～D9、CD-D1～D4 不再列入「尚待产品决定」清单。保留原登记号和优先级便于工程追踪，但**P0 是历史风险分类，不代表未决定**。

| ID | 剩余事项 | 边界 |
| --- | --- | --- |
| DC-009 / Q-19 | Diagnosis V2 医学确认组件、文案、信息密度与窄屏展示 | 业务确认与 Ask User 串行语义已定；Penpot/UI 单独定稿 |
| DC-018 / Q-17 | Diagnosis 正式提案具体过期时限 | 不受 TrainingProposal 的 7 天规则自动约束 |
| DC-035（实施） | 隐私数据真正擦除、历史准入保留、衍生来源传播与合法保留期限 | PC-D7、CD-D4 的产品语义已定，待完整执行与合规验收 |
| DC-039（迁移） | 旧公开分享令牌/快照的关闭和清除 | PC-D8 已决定不保留新功能，需核查运行态并完成退役 |

**决策落地路径**：先决定业务行为 → 更新所属领域属性与状态转换 → 更新 Agent 工作流与 UI → 补充正反例验收 → 实现 Go 权限与幂等校验。临床风险分类仍有**专业规则审核**门槛，不因产品门禁等级已决定而跳过。
