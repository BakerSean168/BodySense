# BodySense · 领域契约独立重建决策登记

**状态：独立重建，BodyState/Diagnosis 与 Training/Safety 的指定业务原则已批量确定，其余产品/隐私与工程细节继续跟踪；未与 Hark 原草稿核对。** GitHub 未保存 Hark 所述的 `ec817b8` 和五份 `work/domain-drafts/`；此表依据已合并的产品规格、当前 Go/React/SQL 源码重新梳理决策问题。条目与 Hark 所说的 83 项、21 项 P0 **没有逐条对应关系**，不得把独立重建的数量作为原草稿恢复率。DC-* 是本轮独立编号，后续如找到原文件再以语义、来源和影响范围去重映射。

**本表仍保留全部 70 个原始议题（含 19 个 P0），因此这些总数并非「仍待决定」数量。** Diagnosis DX-D5～D15 已决原则见 [decisions.md](./decisions.md) 与 [Diagnosis V2](./domain/diagnosis.md)；旧 Diagnosis V1 的描述性病名、通用 lifecycle、用户按钮调整 suspected/likely、临床确诊等级和 mark_resolved 均已退役，不能用旧表述覆盖 V2。 BS-D1～D10、TP-D1～D12 与 SAFE-D1～D8 的已决原则见 [decisions.md](./decisions.md)。DC-001～006、010～014 的**产品语义**已批量确立；DC-022、052～065 的核心行为也已定，其中物理 DTO、旧数据迁移、安全阈值专业审核与并发实现仍需工程验证。与旧系统迁移、隐私清除等独立细则仍按原 ID 跟踪。

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
| DC-015 | P0 | Profile 身份字段、必填性、编辑和私密展示要求 | Person / Profile | 新 |
| DC-016 | P0 | 单用户 Conversation 是一份还是多份；会话与待回复工具的关联 | Chat/Agent 运行时 | 新 |
| DC-017 | P0 | 有 pending 决定时普通输入能否继续、怎样绑定为补充信息 | Chat 输入、ask_user 恢复 | 新 |
| DC-018 | P1 | 病情/计划提案超时时长、配置和提醒策略 | Proposal 生命周期 | Q-17 |
| DC-019 | P1 | **已决（BS-D1）**：historical 只属于时间语义，不再是 BodyFact 的生命周期状态；记录状态独立为 asserted/superseded/withdrawn | BodyFact 目标契约 | 新 |
| DC-020 | P1 | **原则已决（BS-D9）**：仅用户本人假设；可按原始陈述更正、撤回并保留关联事实/观察；AI 旧数据迁移仍待 DC-029 | BodyHypothesis | 新 |
| DC-021 | P1 | **Diagnosis V2 原则已决（DX-D5/D8）**：症状缓解/复发属于 BodyState SymptomEpisode；ConditionEntry 是具名医学诊断概念，不持有通用 lifecycle，重新评估不自动修改 Diagnosis；Episode 引用物理迁移待 DC-066 | Diagnosis / BodyState 边界 | DX-D5/D8、DC-066 |
| DC-022 | P1 | **业务原则已定（TP-D6）**：session trainingDate 按开始时 IANA 时区快照，跨午夜不拆、同日多次独立记录；工程映射/测试另行验收 | TrainingSessionLog.date | 新 |
| DC-023 | P1 | 资料、报告、器材的完整字段、删除/停用和历史保留 | Person 资产 | 新 |
| DC-024 | P1 | **原则已决（DX-D7/D8）**：已停用的医学诊断同概念重新采纳沿用 conditionId，需新分析、准确确认与历史审计；具体拒绝指纹与并发复用待 DC-068/070 | disposition 与重采纳 | DX-D7/D8 |

| DC-025 | P1 | 旧 `BodyStateFact.ReviewState` 和 `Trend` 是否进入新 BodyFact，还是只作为兼容投影 | BodyFact / 历史迁移 | [BodyState As-Is](./domain/bodystate.md#28-当前-go-实现映射as-is不是新规格) |
| DC-026 | P1 | 旧 `Kind` 类目（`lifestyle.*`、`history.*`、`equipment.available`）如何对齐新 `symptom/activity/lifestyle` 并保持检索稳定 | BodyFact 类目与旧数据 | 同上 |
| DC-027 | P1 | **部分已决（BS-D9）**：原始值/单位、规范化值/单位、测量方法/条件、时间、来源与纠错链均为逻辑字段；具体单位校验/量纲/准入回评规则待实现契约 | BodyObservation 逻辑 Schema | [BodyState](./domain/bodystate.md) |
| DC-028 | P1 | **原则已决（BS-D9）**：Observation 的陈述状态、来源等级与文档候选准入分离；旧 unverified/confirmed/rejected 数据的逐项迁移及重新准入策略仍待技术方案 | 证据准入与推理资格 | [证据管线](../architecture/health-document-evidence-pipeline.md) |
| DC-029 | P1 | 旧 AI Hypothesis 历史记录如何迁移到分析审计；用户假设如何保留可撤回的证据关联 | BodyHypothesis 与 Run 迁移 | IG-05 |
| DC-030 | P1 | BodyStateRevision 的结构化变更操作、不可变依据、关联 sourceId 和重放保留策略 | BodyStateRevision | [旧模型](../../apps/api/internal/model/body_state.go) |
| DC-031 | P1 | **已决（BS-D1/D3/D4）**：不设 historical 生命周期；事实纠错与真实变化分离，Episode 已结束后新发作为新身份而非恢复旧事实 | BodyFact / SymptomEpisode | DC-019 延伸 |
| DC-032 | P2 | ConcernGroup 的稳定分组键、合并/拆分、派生更新及展示顺序应如何定义 | DerivedConcernGroup | INV-25 |
| DC-033 | P1 | 用户删除或撤回原始消息后，事实来源和审计如何最小化保留与脱敏 | BodyFact provenance / 隐私 | G-04 |
| DC-034 | P1 | Media/Reports 在上传、OCR 处理中、分析失败与重试时展示何种状态，如何区别证据准入状态 | Person 资产 UI | UserUpload |
| DC-035 | P0 | 删除医疗报告/源图像时，已采纳的衍生证据与原始资料如何清除、撤回或继续受控留档 | Person 隐私与证据完整性 | R-PS-03 / G-04 |
| DC-036 | P1 | 归档、删除 Conversation 后 pending 互动、运行恢复、证据消息关联与历史如何处理 | Conversation 生命周期 | [Person/Chat](./domain/person-chat.md) |
| DC-037 | P1 | Message 的草稿、流式、失败、重试、撤回是否需要持久状态，以及与 Run 状态如何隔离 | Message 状态机 | 同上 |
| DC-038 | P1 | AgentInteraction 的 answered/cancelled/expired 如何映射被底版取代的业务提案、重试和跨设备恢复 | 互动状态机 | 同上 |
| DC-039 | P0 | 旧 ConversationShare 公开链接是否继续保留；若保留，需怎样的显式授权、敏感内容处理、撤回和过期机制 | 健康对话隐私 / 分享 | IG-06 |
| DC-040 | P1 | 用户时区、语言、单位制等偏好属于 Profile 还是独立 Preferences；变更何时对旧记录生效 | Person / 时间语义 | DC-015/022 |
| DC-041 | P1 | **训练侧已定（TP-D11）**：器材变化仅触发训练影响复核；器材实体自身生命周期仍归 Person DC-023；工程映射/测试另行验收 | Equipment / TrainingPlan | DC-023 |
| DC-042 | P1 | 同一上传文件同时是 Media 和 Report 时，是一个资源多视图还是复制两条记录 | Person 资产身份 | DC-023 |
| DC-043 | P2 | Conversation 标题生成失败、重算、用户手动改名、置顶的状态与来源如何展示 | Conversation 元数据 | [旧模型](../../apps/api/internal/model/conversation.go) |
| DC-044 | P1 | Agent ToolCall succeeded 是否必须附带业务写入 commitId/revision 才能向用户报告“已保存” | Agent 工具与领域授权 | INV-09 |
| DC-045 | P1 | 同一 Run 有多个 pending 工具确认时的顺序、去重及互相失效策略 | PendingInteraction | DC-017 |
| DC-046 | P1 | 用户与 Agent 同时编辑不同 Conversation 时，何种运行锁和用户可见的冲突反馈 | Conversation / Run | G-04 |
| DC-047 | P1 | **原则已决（DX-D8）**：conditionId 对应具名医学概念；同义规范化可保留、实质不同概念须新建；侧别/限定信息和同名去重的精确匹配机制仍待工程方案 | Diagnosis 身份 | DX-D8，DC-067 |
| DC-048 | P1 | **原则已决（DX-D6/D11/D12）**：来源多记录并存、临床原话与证据版本可追踪、来源资格变化仅建议复核；具体迁移与证据撤销传播仍待技术协议 | ConditionEntry 来源链 | DX-D6/11/12 |
| DC-049 | P1 | 用户一次直接更正同时影响 BodyState 和 Diagnosis，跨聚合提交如何界定原子性/部分失败补偿 | ConditionDirectEdit | INV-19 |
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
| DC-070 | P1 | 直接编辑与另一设备的正式分析同时提交时，诊断修订冲突应如何安全恢复并通知用户 | Diagnosis 并发 | INV-09/12 |


## 2026-10-13 Training / Safety 批量决策状态

- **Training 核心业务闭合**：TP-D1～D12 已将计划草案、完整处方、稳定 ID、频率与明确排期、状态、修订、逐组执行、用户明确结束、时区、反馈与授权统一。对应 P0 DC-010～014 不再逐题要求产品选择；DC-022、052～061 余下主要为 DTO、历史迁移、计时与执行工程验证。
- **Safety 产品策略闭合**：SAFE-D1～D8 已将 advisory/restricted/urgent、all_training/plan/exercise、最严格生效、安全暂停/安全终止、解除与复核审计、并发保护统一。对应 P0 DC-001～006 的**产品行为**已决定；**医学分类规则、阈值、临床适用性及专业验证仍是上线门槛**，不能由此宣称临床安全验证完成。
- **未一并结案的非 Training 问题**：Profile/Conversation/Privacy（特别是 DC-015～017、DC-035/039）、Diagnosis 提案有效期 Q-17、Diagnosis/BodyState 物理迁移与跨领域隐私擦除继续单独跟踪。
- **表格解释**：保留原 70 条及 P0/P1/P2 分级作为历史登记；本表 P0 的原始数量不是未决数量。标为「业务原则已定」的记录，工程实现和专业验证不自动视为已完成。

**已确定但不可改写的边界**：确有危险风险时终结当前 Run 的 INV-23、安全限制不能静默修改 TrainingPlan 的 INV-08、用户拒绝更新不撤回整个诊断的 INV-18、临床文件与模型分析不同来源的 INV-11/15、提案关闭不等于拒绝，均已是正式契约。上表只决策这些规则尚未覆盖的细节。

原 [open-questions.md](./open-questions.md) 的 Q-15 与 Q-18 已在 TP-D2/SAFE-D2～D5 中结案；Q-17 仅 Diagnosis 提案期限及 Q-19 的交互视觉细节仍开放，新一轮独立发现的详情进入本表。剩余未结案 P0 仍需产品负责人明确；临床安全规则上线前须专业审核，未获批准的业务语义不得由 Agent 根据旧代码自动填充。

## 尚待产品决定的 P0（不含已定 Training/Safety）

以下为**剩余**业务、隐私与交互 P0；Training/Safety 已按 TP-D1～D12 与 SAFE-D1～D8 一揽子结案，不再次逐题列出旧候选。

| ID | 仍待明确的行为 | 候选边界 |
| --- | --- | --- |
| DC-009 | 诊断确认组件的具体 UI、信息密度与窄屏操作 | 业务语义已按 DX-D15 定，剩视觉与交互 |
| DC-015 | Profile 身份字段的编辑、必填与隐私显示 | 最小必要身份 / 扩展可选资料 |
| DC-016 | 长期 Conversation 数量、归档/删除与 pending 关联 | 一份长期会话 / 多份会话并行 |
| DC-017 | 待决定工具期间如何继续补充文本/证据 | 卡片内补充 / 绑定原 Run 的 Chat 输入 / 组合 |
| DC-035 | 医疗报告与派生证据的实际删除范围 | 正确清除、脱敏和证据引用处理，不能把业务撤回当成隐私删除 |
| DC-039 | 旧对话分享的保留、关闭、撤回与敏感内容隔离 | 明确授权后保留 / 停用并治理历史分享 |

**决策落地路径**：先决定业务行为 → 更新所属领域属性与状态转换 → 更新 Agent 工作流与 UI → 补充正反例验收 → 实现 Go 权限与幂等校验。临床风险分类仍有**专业规则审核**门槛，不因产品门禁等级已决定而跳过。
