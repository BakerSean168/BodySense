# BodySense · 开放问题与规格矛盾

> 新一轮独立重建的问题见 [domain-decisions.md](./domain-decisions.md)（原始登记共 70 项、其中 19 项 P0；**已包含本轮已决事项，并非剩余待决数**），与 Hark 未推送的原 83 项尚未核对，不得宣称已逐条合流。BodyState BS-D1～D10 与 Diagnosis V2 DX-D5～D15 已决原则见 [decisions.md](./decisions.md)，旧数据迁移与隐私删除细则仍待决。

**状态：2026-10-13 更新，待后续产品 / 实现评审。** 以下问题在决定前不得由实现方自行补全为新规则；决定后把结论写入对应规格文件，在 [决策记录](./decisions.md) 中登记，并从本表删除。Q-01～Q-14、Q-16、Q-20、Q-21 已决定，Q-18 部分决定，见决策记录。

| ID | 问题 | 当前规格的处理 | 涉及位置 |
| --- | --- | --- | --- |
| Q-15 | **Penpot Workflow/Training Draft 状态的含义。** Today / Move / Feedback / Paused / Completed 已对应 Trainer 执行状态，Draft 含义不明（训练前预览？计划草稿？）。需在 Penpot 中核对。 | 保留该状态，不删除、不赋予业务含义 | interface §5.1、glossary §3 |
| Q-17 | **超时时长。** 病情提案（ConditionAnalysisRun 内）与计划提案（PlanChangeProposal）多久算超时？两者是否相同？ | 已定义超时后的行为，未定义时长 | domain §3.3、§4 |
| Q-18 | **SafetyRestriction 的风险等级取值与策略映射（剩余部分）。** `riskLevel` 有哪些等级、各自对执行的影响？除警示 / 建议就医类（已定为默认 `professional_clearance`）外，哪些风险对应 `on_expiry` / `user_reverification`，默认有效期或复核时点多长？解除策略本身已决定，见决策记录。 | `riskLevel` 取值未定义；`liftPolicy` 与解除规则已定义 | domain §2、requirements R-SAFE-03～04 |
| Q-19 | **Diagnosis V2 确认工具的实际组件与文案。** 业务语义已经决定：具名医学诊断的 create/update/set_disposition/update_relation 提案分别展示准确变更，用户 accept/reject/继续补充；AI suspected/likely 只读，停用理由清楚显示，多提案逐项决定整体提交。仍需 Penpot 确定文案、信息密度、窄屏、已决定未提交、补充后过期/重新确认和无临床来源情形。 | 业务与权限按 [Diagnosis V2](./domain/diagnosis.md) 已批准；仅组件与交互展示待设计，不再考虑 V1 的 likely / keep_as_suspected / include_clinical / mark_resolved 选择 | interface §4、agent-workflows §3.2 |