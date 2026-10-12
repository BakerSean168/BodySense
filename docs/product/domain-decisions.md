# BodySense · 领域契约待决登记（恢复工作分支）

**状态：部分恢复，尚非完整决策登记表。** Hark 的五份 domain-drafts 分析和原先的 83 项去重清单未推送、当前不可访问。下表仅覆盖已知且可由现行规格或明确交接信息支持的问题，不能宣称等同于其 21 项 P0 或全部 83 项。临时编号 DC-* 用于避免与 Hark 草稿 D-0xx 重号；获得原清单后统一映射、去重和编号，不凭空补齐条目。

**决策规则**：只有产品负责人确定后，才能修改相关 `domain/*.md` 的规范性状态/字段，并同步需求、流程、验收；`pending` 不是系统运行状态，不得交给实现者猜测。已经批准的语义继续以各领域契约为准。

| ID | 优先级 | 待决定事项（不预设答案） | 决策将影响 | 原有问题 |
| --- | --- | --- | --- | --- |
| DC-001 | P0 | 风险等级 `riskLevel` 的完整枚举、严重程度和执行行为 | SafetyRestriction 与 Trainer 门禁 | Q-18 |
| DC-002 | P0 | 症状/临床风险信号如何映射等级、升级及降级，何种情况阻断 Diagnosis | 安全分类器与 Agent | Q-18 |
| DC-003 | P0 | 限制的作用范围是否需要全训练 `all_plans`，以及继承与优先级规则 | SafetyState.scope | 新 |
| DC-004 | P0 | 解除条件需要哪些结构化字段、证据类型、有效期和后端核验 | liftConditions / liftBasis | Q-18 |
| DC-005 | P0 | 安全阻断后的新运行何时允许启动；用户如何获知与恢复 | Run/Diagnosis 与安全策略 | 新；现有 INV-23 只定义原运行不可恢复 |
| DC-006 | P0 | 正在执行的 Trainer 收到新安全限制时暂停、跳过或终止的粒度及记录 | TrainingSessionLog / SafetyRestriction | 新 |
| DC-007 | P0 | Habits 信息是否遵从 BodyState 的自动记录，是否还有候选审核 | Person / BodyFact | 新 |
| DC-008 | P0 | Records 中不同记录类型是否自动入库、何时需要确认 | Person / BodyState | 新 |
| DC-009 | P0 | 临床/结束提案以及多提案确认卡片如何展示与操作 | Chat / Diagnosis UI | Q-19 |
| DC-010 | P0 | 训练频率、排期、漏练、补练与用户时区的业务语义 | TrainingPlan 排程与训练记录 | 新 |
| DC-011 | P0 | 动作身份的稳定规则；同名动作、跨计划与修订的标识如何关联 | exerciseId / 历史回放 | 新 |
| DC-012 | P0 | 计划暂停、完成、恢复与重开有哪些合法转换、授权和审计 | TrainingPlan.status | 新 |
| DC-013 | P0 | `skipped` 是跳过组、动作还是整次课；提前结束如何归类 | Session/Set 记录 | 新 |
| DC-014 | P0 | Penpot 中 Draft 究竟是计划草稿、训练预览还是其他状态 | Trainer/UI | Q-15 |
| DC-015 | P0 | Profile 身份字段、必填性、编辑和私密展示要求 | Person / Profile | 新 |
| DC-016 | P0 | 单用户 Conversation 是一份还是多份；会话与待回复工具的关联 | Chat/Agent 运行时 | 新 |
| DC-017 | P0 | 有 pending 决定时普通输入能否继续、怎样绑定为补充信息 | Chat 输入、ask_user 恢复 | 新 |
| DC-018 | P1 | 病情/计划提案超时时长、配置和提醒策略 | Proposal 生命周期 | Q-17 |
| DC-019 | P1 | BodyFact 的 `historical` 是真实历史性质还是生命周期状态 | BodyState.lifecycle | 新 |
| DC-020 | P1 | 用户怀疑（BodyHypothesis）的更正、撤回、证据更新和历史 | BodyHypothesis | 新 |
| DC-021 | P1 | 病情缓解后再发作，复用 conditionId 还是创建 episode/新条目 | Diagnosis 病程 | 新 |
| DC-022 | P1 | 训练当天跨时区、跨午夜时，`date` 应如何归属 | TrainingSessionLog.date | 新 |
| DC-023 | P1 | 资料、报告、器材的完整字段、删除/停用和历史保留 | Person 资产 | 新 |
| DC-024 | P1 | 已撤回或不再适用的病情，未来重新采纳的条件及审计方式 | ConditionEntry.disposition | 新 |

**已确定但不可改写的边界**：确有危险风险时终结当前 Run 的 INV-23、安全限制不能静默修改 TrainingPlan 的 INV-08、用户拒绝更新不撤回整个诊断的 INV-18、临床文件与模型分析不同来源的 INV-11/15、提案关闭不等于拒绝，均已是正式契约。上表只决策这些规则尚未覆盖的细节。

原 [open-questions.md](./open-questions.md) 的 Q-15、Q-17、Q-18、Q-19 仍然保留，作为合并前已有问题的来源；待原 83 项可访问后，再决定是否以单一登记表替换。
