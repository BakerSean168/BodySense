# BodySense · 开放问题与规格矛盾

> 新一轮独立重建的问题见 [domain-decisions.md](./domain-decisions.md)（原始登记共 70 项、其中 19 项 P0；**已包含本轮已决事项，并非剩余待决数**），与 Hark 未推送的原 83 项尚未核对，不得宣称已逐条合流。BodyState BS-D1～D10 与 Diagnosis V2 DX-D5～D15 已决原则见 [decisions.md](./decisions.md)，旧数据迁移与隐私删除细则仍待决。

**状态：2026-10-13 更新。** Person / Chat 与跨领域 PC-D1～D9、CD-D1～D4 的产品行为已确定，见 [领域契约](./domain/person-chat.md) 与 [决策记录](./decisions.md)；不再以多会话、并行普通输入或原件移除自动撤销历史准入为候选。剩余 Q-17（Diagnosis 提案期限）与 Q-19（医学确认控件实际视觉）分别按原领域后续评审；临床规则验证、迁移和物理擦除合规实施是工程/专业门槛。Q-01～Q-16、Q-18、Q-20、Q-21 既有业务原则已决定，未决问题不可由实现方擅自填默认值。

| ID | 问题 | 当前规格的处理 | 涉及位置 |
| --- | --- | --- | --- |
| Q-17 | **仅 Diagnosis 提案期限待定。** Training 的 PlanProposal 已确定最长 7×24 小时未决定自动过期，并校验关联上下文；ConditionAnalysisRun 内的提案有效期限另行确定，不依附训练期限。 | Training 已按 TP-D4 结案；Diagnosis 仍保持未定 | domain/diagnosis §3.3、domain/training §4.2 |
| Q-19 | **Diagnosis V2 确认工具的实际组件与文案。** 业务语义已经决定：具名医学诊断的 create/update/set_disposition/update_relation 提案分别展示准确变更，用户 accept/reject/继续补充；AI suspected/likely 只读，停用理由清楚显示，多提案逐项决定整体提交。仍需 Penpot 确定文案、信息密度、窄屏、已决定未提交、补充后过期/重新确认和无临床来源情形。 | 业务与权限按 [Diagnosis V2](./domain/diagnosis.md) 已批准；仅组件与交互展示待设计，不再考虑 V1 的 likely / keep_as_suspected / include_clinical / mark_resolved 选择 | interface §4、agent-workflows §3.2 |
