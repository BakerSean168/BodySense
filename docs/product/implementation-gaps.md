# BodySense · 目标领域契约与当前实现差异

**基线**：`main` 合并 PR #245 后的目标业务契约（7472989）。本页只记录可追溯的静态代码证据，不代表运行时 E2E 已复现；不授权本 PR 修改业务代码、迁移或生产配置。

| ID | 严重性 | 源码证据 | 当前观察 / 与目标契约的关系 | 处理与回归依据 |
| --- | --- | --- | --- | --- |
| IG-01 | P1 | [TrainingExecutionPanel.tsx](../../apps/web/src/features/training/components/TrainingExecutionPanel.tsx)（handleNextExercise，约 698–712 行） | 最后一个动作的「下一个」遍历全部 exercises，将各项 exerciseStates 全设为 true 并提示全部完成；未逐一检查用户是否完成。与 R-EX-04 不符 | 后续代码 PR：仅明确完成/跳过才能产生记录；回归 TP-12 |
| IG-02 | P1 | [training_service.go](../../apps/api/internal/service/training_service.go)（约 202/244/297/435 行） | GetTodayTask、CheckIn、UpdateLog 使用 localDate(time.Now())，localDate 沿用进程时区；没有以当前用户时区计算日期。与 TrainingSessionLog.date 约定不符 | 后续代码 PR：引入可信用户时区与跨日测试；回归 TP-13；跨午夜规则 DC-022 |
| IG-03 | 待分类 | [training_service.go](../../apps/api/internal/service/training_service.go)（UpdateLogWithFeedback）与 [treatment_service.go](../../apps/api/internal/service/treatment_service.go)（状态重算） | 保存反馈会调用 RecordOutcome，响应可能得到 TreatmentStatusPaused；旧 Treatment 暂停的具体触发条件需与新 TrainingPlan.status/SafetyState 权威边界逐一核对。**不能据此宣称每次保存必定暂停。** | 增加状态组合契约测试；决定迁移语义后修正，回归 TP-15 |
| IG-04 | P0 安全边界 | [body_state_service.go](../../apps/api/internal/service/body_state_service.go)（ResolveSafetyState，约 654–671 行） | 允许基于 resolution 和任意 note 将安全状态设为 resolved/cleared_by_review，函数本身未验证 professional_clearance 或结构化 liftBasis。不同于目标 R-SAFE-04/INV-20 | 未完成安全策略与审计前不得以本代码作为已满足解除契约的依据；回归 SAFE-01/02，依赖 DC-001/004 |
| IG-05 | P1 领域归属 | [diagnosis_application_service.go](../../apps/api/internal/service/diagnosis_application_service.go)（commitHypotheses，约 458–481 行）与 [body_state_service.go](../../apps/api/internal/service/body_state_service.go)（AddDiagnosisHypothesis） | 旧 Diagnosis 代码将模型候选假设存入 BodyStateHypothesis，和新 R-BS-06 / INV-02 的「AI 假设只在分析运行审计」冲突 | 后续迁移至 Run 追加假设事件；保留历史来源，回归 BS-14 |
| IG-06 | 产品/隐私待决 | [share_service.go](../../apps/api/internal/service/share_service.go) 与 [routes.go](../../apps/api/internal/transport/httpapi/routes.go) | 现有 ConversationShare 快照及公开分享 URL；新产品规格没有明确此功能的用户体验、权限及隐私边界。存在 ≠ 必须删除 | 明确功能归属/隐私后决定保留、退役或独立迁移；不得在本 PR 默默删除 |
| IG-07 | P1 可用性 | [activeTurnSelectors.ts](../../apps/web/src/features/consultation/runtime/activeTurnSelectors.ts)（selectIsComposerLocked 约 99–102 行）及 [AssistantChatPanel.tsx](../../apps/web/src/features/consultation/components/AssistantChatPanel.tsx)（约 487、552 行） | interrupted 状态锁定整个 Composer；当前「继续补充」不能通过普通消息输入路径完成。其他交互卡片可回答，但补充内容的业务归属和方式未确定 | 需要产品决定 DC-017；UI-08 测试继续补充通道，不强行规定必须使用通用 Composer |

## 交付要求与执行隔离

- 上述项均为**静态代码阅读**，未在此分支运行浏览器或数据库 E2E；P0/P1 是审查排序，不等于已确认生产安全事件。
- 每一修复需独立代码 PR，给出 Go 事务/Agent/React/UI 对应的正反例与回归测试；不得在领域文档 PR 中混合修复。
- 当前已经批准的业务规则以 [requirements.md](./requirements.md)、[domain](./domain/README.md)、[acceptance.md](./acceptance.md) 为准；产品语义待决项以 [domain-decisions.md](./domain-decisions.md) 为准。
- 后续核实若发现代码路径已有独立权威校验或实际保护，须更新本表为 fixed / partial / not reproduced，并附测试证据，不能保留推断。
