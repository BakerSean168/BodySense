# BodySense · 目标领域契约与当前实现差异

**代码对照基线**：以 `main` 合并 PR #245 后的旧运行时代码（7472989）为最初对照，进一步静态核对 GCP Dev `13d236f37` 的领域资料与相关运行时代码；目标产品规则已升级至 Diagnosis V2、Training/Safety V2、阶段三 Person/Chat + 跨域契约。本页只记录可追溯的静态代码证据，不代表运行时 E2E 已复现；本次更新不修改业务代码、迁移或生产配置。

| ID | 严重性 | 源码证据 | 当前观察 / 与目标契约的关系 | 处理与回归依据 |
| --- | --- | --- | --- | --- |
| IG-01 | P1 | [TrainingExecutionPanel.tsx](../../apps/web/src/features/training/components/TrainingExecutionPanel.tsx)（handleNextExercise，约 698–712 行） | 最后一个动作的「下一个」遍历全部 exercises，将各项 exerciseStates 全设为 true 并提示全部完成；未逐一检查用户是否完成。与 R-EX-04 不符 | 后续代码 PR：仅明确完成/跳过才能产生记录；回归 TP-12 |
| IG-02 | P1 | [training_service.go](../../apps/api/internal/service/training_service.go)（约 202/244/297/435 行） | GetTodayTask、CheckIn、UpdateLog 使用 localDate(time.Now())，localDate 沿用进程时区；没有以当前用户时区计算日期。与 TrainingSessionLog.date 约定不符 | 后续代码 PR：引入可信用户时区与跨日测试；回归 TP-13；跨午夜规则 DC-022 |
| IG-03 | 待分类 | [training_service.go](../../apps/api/internal/service/training_service.go)（UpdateLogWithFeedback）与 [treatment_service.go](../../apps/api/internal/service/treatment_service.go)（状态重算） | 保存反馈会调用 RecordOutcome，响应可能得到 TreatmentStatusPaused；旧 Treatment 暂停的具体触发条件需与新 TrainingPlan.status/SafetyState 权威边界逐一核对。**不能据此宣称每次保存必定暂停。** | 增加状态组合契约测试；决定迁移语义后修正，回归 TP-15 |
| IG-04 | P0 安全边界 | [body_state_service.go](../../apps/api/internal/service/body_state_service.go)（ResolveSafetyState，约 654–671 行） | 允许基于 resolution 和任意 note 将安全状态设为 resolved/cleared_by_review，函数本身未验证 professional_clearance 或结构化 liftBasis。不同于目标 R-SAFE-04/INV-20 | 未完成安全策略与审计前不得以本代码作为已满足解除契约的依据；回归 SAFE-01/02，依赖 DC-001/004 |
| IG-05 | P1 领域归属 | [diagnosis_application_service.go](../../apps/api/internal/service/diagnosis_application_service.go)（commitHypotheses，约 458–481 行）与 [body_state_service.go](../../apps/api/internal/service/body_state_service.go)（AddDiagnosisHypothesis） | 旧 Diagnosis 代码将模型候选假设存入 BodyStateHypothesis，和新 R-BS-06 / INV-02 的「AI 假设只在分析运行审计」冲突 | 已解决（PR4 slice 3）：commitHypotheses / AddDiagnosisHypothesis / body_state_hypotheses 与 /api/v1/body-state/hypotheses 已删除；Agent 假设以创建快照写入 diagnosis_analyses.decision_trace.agent_hypotheses（只审计、无 BodyState 权威）；后续 hypothesis_added 事件表待 Run 事件化 |
| IG-06 | P0 旧功能退役与隐私 | [share_service.go](../../apps/api/internal/service/share_service.go) 与 [routes.go](../../apps/api/internal/transport/httpapi/routes.go) | 现有 ConversationShare 快照及公开分享 URL；**PC-D8 已明确目标产品不提供公开分享**，但尚未验证旧令牌是否仍可访问、是否有历史分享数据 | 独立代码/数据治理 PR：盘点令牌与快照、验证撤销/停用及缓存清理；回归 PC3-19；本次只更新契约，不默默删服务 |
| IG-07 | P1 Chat UX | [activeTurnSelectors.ts](../../apps/web/src/features/consultation/runtime/activeTurnSelectors.ts) 与 [AssistantChatPanel.tsx](../../apps/web/src/features/consultation/components/AssistantChatPanel.tsx) | interrupted 已锁住普通 Composer，但**PC-D5 要求 Ask User 整体替换底部普通输入区**，回答只属于当前交互而非另一个普通消息；现有卡片加锁定输入的形态尚未验收为最终 UI | 按 PC-D5 改造输入区与工具控件、恢复/过期/跨端串行路径；回归 UI-08、PC3-03～05 |
| IG-08 | P1 领域模型 | [body_state.go](../../apps/api/internal/model/body_state.go)（BodyStateFact/BodyState）、[body_state_repository.go](../../apps/api/internal/repository/body_state_repository.go)（UpsertFact/TransitionFact） | 现有 Fact 的 LifecycleState/ReviewState/Trend 仍混合当前陈述和实际变化；仓库目标契约已将 assertionStatus 与症状 Episode 派生状态分开，但尚未见新 Episode 完整持久关联及状态投影 | 后续独立代码/迁移 PR，先做旧数据表征测试并明确 DC-025/026/030；验证 BS-16～27，不在文档 PR 中直接更改库表 |
| IG-09 | P1 来源与隐私 | [body_state_reasoning.go](../../apps/api/internal/model/body_state_reasoning.go)、[body_state.go](../../apps/api/internal/model/body_state.go) | 现有假设面向 AI、观察仍使用旧 review_state；**PC-D6/D7、CD-D4 已明确业务删除和历史准入语义**，但旧来源分级/原话片段和衍生擦除仍无完整运行时验证 | 按 DC-027～029、DC-033/035 做来源/记录迁移、独立来源保护及隐私擦除传播；验证 BS-28/31/32、PC3-10/17 |

| IG-10 | P1 Diagnosis V2 契约迁移 | [diagnosis_analysis.go](../../apps/api/internal/model/diagnosis_analysis.go)、[diagnosis_application_service.go](../../apps/api/internal/service/diagnosis_application_service.go) | 旧 Analysis/Candidate 及用户确认逻辑并非 V2 目标的具名医学 ConditionEntry、双态 disposition、多来源临床声明、AI suspected/likely 与 accept/reject 独立决策；未看到 V2 全套表/事件/Schema 已迁移的验证 | 独立迁移 PR：旧病名/描述性条目分类、状态/决定映射、API/事务/前端契约与历史兼容；先完成表征测试并验证 DXV2-01～28。不把文档修改宣称为运行时实现 |

## 2026-10-13 · 阶段三新增的运行时差距

| ID | 严重性 | 源码证据 | 与已批准目标的差距 | 处理与回归依据 |
| --- | --- | --- | --- | --- |
| IG-11 | P1 Chat 身份与串行 | [conversation.go](../../apps/api/internal/model/conversation.go)、[conversation_service.go](../../apps/api/internal/service/conversation_service.go) | 多 Conversation、Title/Pinned/软删结构和 UI 流程仍存在；新目标每用户只有唯一稳定 Conversation 且普通 Run 后端串行 | 旧数据/Run/互动/引用兼容迁移，唯一约束、跨设备并发恢复；PC3-01/02/04/05 |
| IG-12 | P0 隐私与滚动消息 | [message_repository.go](../../apps/api/internal/repository/message_repository.go)、[consultation_thread.py](../../apps/ai-service/src/runtime/consultation_thread.py)、[checkpointing.py](../../apps/ai-service/src/runtime/checkpointing.py) | 目前模型输入截断≠原 Message/Checkpoint 物理清理；尚无三游标、最小原话证据转存、完整工具边界保护及缓存可核验清理 | 实现持久游标/事务边界和消息+Checkpoint/索引清理，再回归 PC3-06～10、CD3-05/07 |
| IG-13 | P0 医疗文件/资产删除 | [user_profile.go](../../apps/api/internal/model/user_profile.go)、[user_upload.go](../../apps/api/internal/model/user_upload.go)、[upload_service.go](../../apps/api/internal/service/upload_service.go) | 现有 UserProfile、UserUpload 与 DeleteUpload 未证明统一 UserAsset 多视图、Profile 偏好及原件移除/衍生健康数据擦除的两种独立 API 和 CD-D4 历史准入保护 | 分资产/证据身份、文件状态与准入；实现并验证双删除纵切、最小缓存/衍生清理和独立来源；PC3-11～18、CD3-06/07 |
| IG-14 | P1 跨域一致性/权限 | [body_state_service.go](../../apps/api/internal/service/body_state_service.go)、[diagnosis_application_service.go](../../apps/api/internal/service/diagnosis_application_service.go)、[agent_interaction.go](../../apps/api/internal/model/agent_interaction.go) | 当前各服务具备部分来源/修订/幂等机制，但未证明全链路 SourceRef、DomainCommitReceipt、可靠 Outbox 或等价可重放事件以及精确授权与部分失败可见性全部满足 CD-D1～D3 | 分域原子提交/真实回执、持久幂等传播、并发冲突与跨用户鉴权；CD3-01～08、PC3-20 |

## 交付要求与执行隔离

- 上述项均为**静态代码阅读**，未在此分支运行浏览器或数据库 E2E；P0/P1 是审查排序，不等于已确认生产安全事件。
- 每一修复需独立代码 PR，给出 Go 事务/Agent/React/UI 对应的正反例与回归测试；不得在领域文档 PR 中混合修复。
- 当前已经批准的业务规则以 [requirements.md](./requirements.md)、[domain](./domain/README.md)、[acceptance.md](./acceptance.md) 为准；产品语义待决项以 [domain-decisions.md](./domain-decisions.md) 为准。
- 后续核实若发现代码路径已有独立权威校验或实际保护，须更新本表为 fixed / partial / not reproduced，并附测试证据，不能保留推断。