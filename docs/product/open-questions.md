# BodySense · 开放问题与规格矛盾

**状态：2026-10-12 文档整理时记录，待产品负责人（Arlon）决定。** 以下问题在决定前不得由实现方自行补全为新规则；决定后把结论写入对应规格文件，并从本表删除。

## 1. 规格内部矛盾 / 缺口

| ID | 问题 | 涉及位置 |
| --- | --- | --- |
| Q-01 | **否定与「不再适用」没有对应的生命周期值。** R-DX-01 允许「标记缓解或不再适用」，domain §3.1 规定「不是这个」时「按解释被否定结束该条目」，但 `ConditionEntry.lifecycle` 只有 `active / monitoring / resolved`。被否定或不再适用的条目用哪个值？是否需要 `refuted` / `not_applicable`？ | requirements R-DX-01、domain §3 / §3.1 |
| Q-02 | **用户否定记录存在哪里？** domain 说「在 DiagnosisAnalysis 审计中记录用户否定」，acceptance DX-07c 要求无新证据时不再重复提议，但 DiagnosisAnalysis 没有数据契约，「同一提案」「新证据」的判定也未定义。 | domain §3.1、acceptance DX-07c |
| Q-03 | **临床来源条目是否也走确认工具？** `certainty` 包含 `clinically_documented` 与 `user_reported_clinical_diagnosis`，但 `confirmation.decision` 只有 `likely / keep_as_suspected`，且 `confirmation` 是必填字段。用户说「医生告诉我患偏头痛」（BS-05）或上传诊断报告时，是先进入 BodyState，还是经确认工具进入 Diagnosis？选哪个 decision？ | domain §3、acceptance BS-05 |
| Q-04 | **用户提出的假设写到哪里？** R-BS-02 场景「我怀疑是偏头痛 → 记录用户提出的可能性」，domain 有 `BodyHypothesis[]`，但 `BodyFact.kind` 无对应值，`record_body_state` 只写事实。用户假设是自动写入 BodyHypothesis 还是作为 BodyFact？AI 自己提出的假设是否持久化？ | requirements §2.2、domain §2、agent-workflows §3.1 |
| Q-05 | **`TrainingSession` 与 `exerciseStages` 的关系未定义。** `currentContent` 同时有 `sessions: TrainingSession[]` 与顶层 `exerciseStages`，`TrainingSession` 无类型定义。多 Session 计划（如 A/B 日）的阶段放在哪里？ | domain §4 |
| Q-06 | **提案「取消 / 关闭」没有状态值。** domain 正文与 agent-workflows §4 提到取消/关闭，`PlanChangeProposal.state` 只有 `pending / accepted / rejected / expired`。关闭后是保持 pending 还是记为 rejected/expired？ | domain §4、agent-workflows §4 |
| Q-07 | **Trainer 细节缺失。** 产品规格只在窗口表中描述 Trainer。已归档的 `feature_spec_training_schedule.md` §3 有更细的规则（显示当前动作、组次、计时、完成/跳过与恢复；训练记录按计划/日期/动作定位，不根据点击推断完成），Penpot 也有 Workflow/Training Today/Draft/Move/Feedback/Paused/Completed 状态（其中 Draft 含义不明）。是否把这些写入 requirements / interface？ | interface §2、archive/product-legacy |
| Q-08 | **资料上传与 OCR 证据缺少产品规格。** 旧「上传 → 证据 → 结果」流程已废弃（上传归 Person Media/Reports，分析在 Chat 中发起），但新规格未说明报告识别值如何被用户核实、何时可作为 Diagnosis 证据。已归档的 `feature_spec_health_document_ingestion.md` 与仍有效的 `architecture/health-document-evidence-pipeline.md`、ADR 0011/0013 仍以 Assessment 为证据消费者。 | interface §3、architecture、ADR |
| Q-09 | **训练计划安全阻断的状态位置。** R-SAFE-02 / INV-08 要求执行阻断独立于内容修订，但 `TrainingPlan.status` 只有 `active / paused / completed`，阻断由 BodyState `SafetyState` 还是计划自身持有未说明。 | domain §2 / §4 |

## 2. 规格与仓库其他文档的冲突

| ID | 问题 | 涉及位置 |
| --- | --- | --- |
| Q-10 | **真值顺序冲突。** `AGENT.md` 规定「当前代码 > 配置 > docs/product」，而负责人规则是 docs/product 为业务规则唯一来源（代码正在按新规格重构）。建议在 AGENT.md 中区分：技术事实以代码为准，业务规则以 docs/product 为准。本次整理未改动该规则。 | AGENT.md「真值顺序」 |
| Q-11 | **架构文档缺少病情确认步骤。** `architecture/longitudinal-health-loop.md`、`health-journey-workflow.md`、`longitudinal-body-state-domain.md` 写作「用户请求正式分析 → 更新 Diagnosis」，没有 confirm_condition 与三选一结果。 | docs/architecture |
| Q-12 | **工程文档仍以旧模型描述现有实现。** `architecture/current-longitudinal-system.md`、`technical-approach.md`、`treatment-*`、`diagnosis-*`、`agent-platform-role-governance.md`、ADR 0004/0009、`CONTEXT.md` 等使用 Consultation Session、Treatment、Assessment、DiagnosisAnalysis candidates。它们描述的是当前代码，因此保留；重构落地后需要逐篇更新或归档。 | docs/architecture、docs/adr、CONTEXT.md |
| Q-13 | **DiagnosisAnalysis 同名异义。** 旧代码中 DiagnosisAnalysis 是按问诊生成的候选诊断；新规格中仅指不可变分析审计记录。实现时建议改名或明确区分。 | domain §3.1、旧代码 |
| Q-14 | **Person 与 Profile 的边界。** Penpot 的 Profile 窗口 = Person；领域对象 Profile = 基础资料。design/README 把分类写为「Profile / Records / Equipment / Media / Reports / Habits」，interface 只列五类。Profile（身份）是否作为 Person 的第六个标签页？ | interface §3、design/README |
