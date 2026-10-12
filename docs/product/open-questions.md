# BodySense · 开放问题与规格矛盾

> 新一轮独立重建的问题见 [domain-decisions.md](./domain-decisions.md)（原始登记共 70 项、其中 19 项 P0；**已包含本轮已决事项，并非剩余待决数**），与 Hark 未推送的原 83 项尚未核对，不得宣称已逐条合流。BodyState BS-D1～D10 与 Diagnosis V2 DX-D5～D15 已决原则见 [decisions.md](./decisions.md)，旧数据迁移与隐私删除细则仍待决。

**状态：2026-10-13 更新，待后续产品 / 实现评审。** 以下问题在决定前不得由实现方自行补全为新规则；决定后把结论写入对应规格文件，在 [决策记录](./decisions.md) 中登记，并从本表删除。Q-01～Q-14、Q-16、Q-20、Q-21 已决定，Q-15、Q-18 已按 TP-D/SAFE-D 批量确定，见决策记录。

| ID | 问题 | 当前规格的处理 | 涉及位置 |
| --- | --- | --- | --- |
| Q-17 | **仅 Diagnosis 提案期限待定。** Training 的 PlanProposal 已确定最长 7×24 小时未决定自动过期，并校验关联上下文；ConditionAnalysisRun 内的提案有效期限另行确定，不依附训练期限。 | Training 已按 TP-D4 结案；Diagnosis 仍保持未定 | domain/diagnosis §3.3、domain/training §4.2 |
| Q-19 | **Diagnosis V2 确认工具的实际组件与文案。** 业务语义已经决定：具名医学诊断的 create/update/set_disposition/update_relation 提案分别展示准确变更，用户 accept/reject/继续补充；AI suspected/likely 只读，停用理由清楚显示，多提案逐项决定整体提交。仍需 Penpot 确定文案、信息密度、窄屏、已决定未提交、补充后过期/重新确认和无临床来源情形。 | 业务与权限按 [Diagnosis V2](./domain/diagnosis.md) 已批准；仅组件与交互展示待设计，不再考虑 V1 的 likely / keep_as_suspected / include_clinical / mark_resolved 选择 | interface §4、agent-workflows §3.2 |
