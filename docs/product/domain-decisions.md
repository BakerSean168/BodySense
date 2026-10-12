# BodySense · 领域契约独立重建决策登记

**状态：独立重建，待产品评审，未与 Hark 原草稿核对。** GitHub 未保存 Hark 所述的 `ec817b8` 和五份 `work/domain-drafts/`；此表依据已合并的产品规格、当前 Go/React/SQL 源码重新梳理决策问题。条目与 Hark 所说的 83 项、21 项 P0 **没有逐条对应关系**，不得把独立重建的数量作为原草稿恢复率。DC-* 是本轮独立编号，后续如找到原文件再以语义、来源和影响范围去重映射。

**本表仍保留全部 70 个原始议题（含 19 个 P0），因此这些总数并非「仍待决定」数量。** Diagnosis DX-D5～D15 已决原则见 [decisions.md](./decisions.md) 与 [Diagnosis V2](./domain/diagnosis.md)；旧 Diagnosis V1 的描述性病名、通用 lifecycle、用户按钮调整 suspected/likely、临床确诊等级和 mark_resolved 均已退役，不能用旧表述覆盖 V2。 BS-D1～D10 的已决原则见 [decisions.md](./decisions.md)，对应 DC-007/008/019/020/027/028/031 在下面标注状态；与旧系统迁移、隐私清除等独立细则仍按原 ID 跟踪。

**决策归属**：P0 均为需要产品负责人拍板的业务或隐私约束。P1/P2 混合业务语义与工程迁移问题：例如 DC-028/029/030/038/044/046/052/053/064/070 可以先由工程方提供实现方案和验证，但不得突破已获批准的不变量；涉及用户可见行为或权限改变时仍需产品批准。任何未决定的业务语义都不能写成正式默认值。`pending` 在本表中表示决策尚未结案，而非产品的业务状态。

| ID | 优先级 | 议题与本轮已决原则（未标已决者仍待处理） | 决策将影响 | 原有问题 |
| --- | --- | --- | --- | --- |
| DC-001 | P0 | 风险等级 `riskLevel` 的完整枚举、严重程度和执行行为 | SafetyRestriction 与 Trainer 门禁 | Q-18 |
| DC-002 | P0 | 症状/临床风险信号如何映射等级、升级及降级，何种情况阻断 Diagnosis | 安全分类器与 Agent | Q-18 |
| DC-003 | P0 | 限制的作用范围是否需要全训练 `all_plans`，以及继承与优先级规则 | SafetyState.scope | 新 |
| DC-004 | P0 | 解除条件需要哪些结构化字段、证据类型、有效期和后端核验 | liftConditions / liftBasis | Q-18 |
| DC-005 | P0 | 安全阻断后的新运行何时允许启动；用户如何获知与恢复 | Run/Diagnosis 与安全策略 | 新；现有 INV-23 只定义原运行不可恢复 |
| DC-006 | P0 | 正在执行的 Trainer 收到新安全限制时暂停、跳过或终止的粒度及记录 | TrainingSessionLog / SafetyRestriction | 新 |
| DC-007 | P0 | **原则已决（BS-D8）**：明确本人习惯自动写入 BodyFact；仅关键歧义/特定证据准入另行处理，Habits 不复制事实 | Person / BodyFact | 新 |
| DC-008 | P0 | **原则已决（BS-D8）**：明确的本人身体陈述自动写入所属权威领域；Records 不绕过 Diagnosis 确认，也不持有第二份事实 | Person / BodyState | 新 |
| DC-009 | P0 | **Diagnosis V2 原则已决（DX-D15）**：具名医学提案逐项 accept/reject/继续补充，一次原子提交；卡片具体文案、组件与响应式展示仍待 Q-19/Penpot | Chat / Diagnosis UI | Q-19 |
| DC-010 | P0 | 训练频率、排期、漏练、补练与用户时区的业务语义 | TrainingPlan 排程与训练记录 | 新 |
| DC-011 | P0 | 动作身份的稳定规则；同名动作、跨计划与修订的标识如何关联 | exerciseId / 历史回放 | 新 |
| DC-012 | P0 | 计划暂停、完成、恢复与重开有哪些合法转换、授权和审计 | TrainingPlan.status | 新 |
| DC-013 | P0 | `skipped` 是跳过组、动作还是整次课；提前结束如何归类 | Session/Set 记录 | 新 |
| DC-014 | P0 | Penpot 中 Draft 究竟是计划草稿、训练预览还是其他状态 | Trainer/UI | Q-15 |
| DC-015 | P0 | Profile 身份字段、必填性、编辑和私密展示要求 | Person / Profile | 新 |
| DC-016 | P0 | 单用户 Conversation 是一份还是多份；会话与待回复工具的关联 | Chat/Agent 运行时 | 新 |
| DC-017 | P0 | 有 pending 决定时普通输入能否继续、怎样绑定为补充信息 | Chat 输入、ask_user 恢复 | 新 |
| DC-018 | P1 | 病情/计划提案超时时长、配置和提醒策略 | Proposal 生命周期 | Q-17 |
| DC-019 | P1 | **已决（BS-D1）**：historical 只属于时间语义，不再是 BodyFact 的生命周期状态；记录状态独立为 asserted/superseded/withdrawn | BodyFact 目标契约 | 新 |
| DC-020 | P1 | **原则已决（BS-D9）**：仅用户本人假设；可按原始陈述更正、撤回并保留关联事实/观察；AI 旧数据迁移仍待 DC-029 | BodyHypothesis | 新 |
| DC-021 | P1 | **Diagnosis V2 原则已决（DX-D5/D8）**：症状缓解/复发属于 BodyState SymptomEpisode；ConditionEntry 是具名医学诊断概念，不持有通用 lifecycle，重新评估不自动修改 Diagnosis；Episode 引用物理迁移待 DC-066 | Diagnosis / BodyState 边界 | DX-D5/D8、DC-066 |
| DC-022 | P1 | 训练当天跨时区、跨午夜时，`date` 应如何归属 | TrainingSessionLog.date | 新 |
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
| DC-041 | P1 | 器材可用性（是否拥有/可借用/停用）与训练计划选动作的关系，以及默认可见性 | Equipment / TrainingPlan | DC-023 |
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
| DC-052 | P1 | PlanPatch 各操作（添加/移除/改组次/改频率/排期）逻辑类型、路径和冲突边界 | PlanPatch | R-TP-03/04 |
| DC-053 | P1 | TrainingPlanRevision 的 ID、父修订、精确 diff、授权记录和历史回放契约 | TrainingPlanRevision | INV-07/09 |
| DC-054 | P1 | 旧打卡布尔值 `is_checked_in`、旧日粒度 `Exercises` 如何转为组级完成事实，历史不可推断部分如何展示 | TrainingSessionLog 迁移 | IG-01 |
| DC-055 | P1 | 旧 Treatment 的 `review_recommended` 和 `paused` 如何映射到新“复核建议”与独立 SafetyState，而不篡改 Plan.status | 训练计划迁移 | IG-03 |
| DC-056 | P1 | 同一用户同一计划/模板一天能否多次训练或多设备并行训练，日志如何唯一定位 | TrainingSessionLog 身份 | DC-010 |
| DC-057 | P1 | 训练暂停/恢复的计时语义、离线恢复、跨设备计时与超时恢复 | Trainer 执行状态 | R-EX-03 |
| DC-058 | P1 | 用户误点完成或跳过后是否可撤销/更正，变更事实怎样保留追加审计 | exerciseRecords | INV-26 |
| DC-059 | P1 | `ended` 时如何区分正常完成、主动提前结束、取消和安全终止；是否新增结束原因 | TrainingSessionLog | DC-013 |
| DC-060 | P1 | 用户暂停或完成的 TrainingPlan 能否再次激活，多个计划同时处于 active 的规则 | TrainingPlan.status | DC-012 |
| DC-061 | P1 | 已接受计划更改频率/课模板后，历史日志及未来排期如何与修订关联 | 计划排期 / Revision | DC-010 |
| DC-062 | P1 | 多个限制覆盖同一计划/动作时 UI 展示及解除顺序如何合并，是否存在最严格优先策略 | SafetyRestriction 范围 | DC-003 |
| DC-063 | P1 | 导致限制的 BodyFact/Observation 更正后，限制需不需要触发复核，谁可以发起解除 | SafetyState 来源变化 | DC-004 |
| DC-064 | P1 | 并发新增安全风险与解除旧限制时，如何确保单条限制级别的 CAS / 不可清除新风险 | SafetyState 并发安全 | INV-09/20 |
| DC-065 | P1 | 专业放行的用户转述与文件来源出现矛盾/失效时如何提示、驳回或请求复核 | professional_clearance | DC-004 |
| DC-066 | P1 | **领域边界已决（BS-D3/D4、DX-D5）**：症状 Episode 的 ID/时间与复发只在 BodyState；Diagnosis 引用其证据版本，具体重放、引用重定向和旧数据迁移仍待 DC-030 | Diagnosis ↔ SymptomEpisode | DC-021 / BS-D3/D4 |
| DC-067 | P1 | **原则已决（DX-D8）**：不同医学概念不应直接合并为一个 ConditionEntry，同一概念保留稳定 ID；历史条目误合并、拆分、现有下游引用映射待工程方案和必要的用户授权 | Diagnosis 条目身份 | DX-D8/INV-39 |
| DC-068 | P1 | **原则已决（DX-D7/D8）**：inactive → adopted 需新分析及用户接受，旧撤回史保留；与历史拒绝的指纹去重和证据新鲜度具体计算待技术方案 | Diagnosis 重新采纳 | DX-D7/D8 |
| DC-069 | P1 | **业务语义已决（DX-D12）**：多临床声明独立保留且不覆盖，医生原始不确定性和有效来源分别展示；结构化映射与原资料撤销传播待实现 | Diagnosis 临床历史 | DX-D12 |
| DC-070 | P1 | 直接编辑与另一设备的正式分析同时提交时，诊断修订冲突应如何安全恢复并通知用户 | Diagnosis 并发 | INV-09/12 |

**已确定但不可改写的边界**：确有危险风险时终结当前 Run 的 INV-23、安全限制不能静默修改 TrainingPlan 的 INV-08、用户拒绝更新不撤回整个诊断的 INV-18、临床文件与模型分析不同来源的 INV-11/15、提案关闭不等于拒绝，均已是正式契约。上表只决策这些规则尚未覆盖的细节。

原 [open-questions.md](./open-questions.md) 的 Q-15、Q-17、Q-18、Q-19 仍然保留，作为 PR #245 的正式待决来源；新一轮独立发现的详情进入本表。提交合并前需由产品负责人明确 P0 的范围和取舍；未获批准的业务语义不得由 Agent 根据旧代码自动填充。

## P0 决策评审稿（不预选答案）

下面只列**需要明确产品意图**的 P0，候选方向不是已获批准的行为，也没有绑定旧代码。可以一次按领域讨论，不需要把技术实现细节交给产品负责人选择。

| ID | 必须明确的问题 | 可比较的方向 |
| --- | --- | --- |
| DC-001 | 风险等级 | 少量可解释等级 / 细分等级；无论哪种都需稳定、可测试的门禁行为 |
| DC-002 | 如何判断危险症状 | 基于明确规则的信号分类 / 证据与规则结合；AI 模型本身不担任最终解除权威 |
| DC-003 | 风险作用范围 | 单计划、单动作 / 再加入全训练阻断；需要界定重叠时行为 |
| DC-004 | 解除条件 | 有结构的规则和来源 / 少量枚举化条件；必须兼容已批准的三种解除策略 |
| DC-005 | 风险处理后的分析 | 手动重新请求 / 明确复核合格后才允许请求；原被阻断 Run 永不复活 |
| DC-006 | 训练中突然出现风险 | 自动暂停当前 Session / 中止当前动作并要求明确恢复；任何方案都不能继续执行受限动作 |
| DC-007 | Habits 如何写入 | **已选择**：明确本人习惯自动记录，关键歧义主动 Ask User；文件证据仍遵循来源准入；见 BS-D8 |
| DC-008 | Records 如何写入 | **已选择**：明确本人陈述自动进入所属权威领域；诊断结论仍需其原有确认；Person 不存第二份真值；见 BS-D8 |
| DC-009 | 多提案确认交互 | 同一 Chat 中逐项处理 / 结构化批量浏览后逐项决定；临床、结束、普通提案保持不同语义 |
| DC-010 | 排期与漏练补练 | 仅每周频率 / 具体日历排期、漏练和补练规则 |
| DC-011 | 动作身份 | 计划修订内稳定 ID / 跨修订可追踪 ID；不能继续用名称充当身份 |
| DC-012 | 计划生命周期 | 暂停/恢复/完成/重开允许的状态图；手动与自动事件有哪些权威 |
| DC-013 | 跳过粒度 | 当前组 / 当前动作 / 整次课；结束和跳过是否独立事件 |
| DC-014 | Penpot Draft | 预览 / 未接受计划草稿 / 其他表现；先核对实际交互再选 |
| DC-015 | Profile 身份 | 最小必需资料 / 扩展可选资料；哪些字段可编辑或只展示 |
| DC-016 | 对话数量 | 一个长期 Conversation / 多 Conversation 同时长期存在；需要明确 pending 互动如何归属 |
| DC-017 | 等待决定时继续补充 | 在卡片内补充 / 普通输入框仍能发送绑定原 Run 的内容 / 混合机制 |
| DC-035 | 报告与派生证据删除 | 保留不可识别的证明链 / 清除并撤销衍生证据 / 按用户指令与合规边界区分 |
| DC-039 | 旧公开对话分享 | 明确保留且需要授权、过期、撤回、脱敏 / 停用并处理历史分享；不能在未决定前把私有健康数据公开为默认能力 |

**决策落地路径**：先确认业务行为 → 更新相应领域属性与合法状态转换 → 更新 Agent 工作流和 UI 状态 → 补充正反例验收 → 实现 Go 权限与并发校验。缺少安全解除和隐私授权的情形不得因前端按钮可见而放宽规则。