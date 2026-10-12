# BodySense · 产品文档

BodySense 是持续理解个人身体状况的 Agent 应用。用户通过自然表达积累身体信息，按需获得有依据的综合诊断，管理与执行多份训练计划。

**本目录是业务规则的唯一来源（Single Source of Truth）。** 设计稿、旧代码或其他文档与本目录冲突时，以本目录为准；本目录内部的未决问题列在 [开放问题](./open-questions.md)。

## 文档分工

每条规则只有一个归属文件，其他文件通过规则编号引用：

| 文档 | 负责什么 | 编号 |
| --- | --- | --- |
| [产品需求](./requirements.md) | 做什么、为什么、业务规则与典型场景 | `R-*`、`G-*` |
| [领域模型](./domain/README.md) | BodyState、Diagnosis、TrainingPlan/Trainer、SafetyState、Person/Chat 的属性、状态与不变量 | `INV-*` |
| [Agent 工作流](./agent-workflows.md) | 记录、澄清、正式分析、病情确认、计划修改的流程与工具 | — |
| [UI 与交互](./interface.md) | 3D 人体桌面、业务窗口、交互工具、平台要求 | — |
| [验收规范](./acceptance.md) | 测试矩阵（引用规则编号）与实施顺序 | `BS/DX/TP/UI/SEC-*` |
| [术语表](./glossary.md) | 标准术语、避免使用的说法、Penpot / 旧代码名称对照 | — |
| [领域待决](./domain-decisions.md) | 由正式规格和源码独立重建的 70 项（19 项 P0），并非 Hark 原始 83 项的恢复副本 | DC-* |
| [实现差异](./implementation-gaps.md) | 源码静态证据与目标契约差异 | IG-* |
| [开放问题](./open-questions.md) | 规格内的矛盾与待决定事项 | `Q-*` |
| [决策记录](./decisions.md) | 已决定问题的结论、原因与写入位置（规则本身仍在上述各文件） | `Q-*`、`GAP-*` |

文中的逻辑结构与工具名称是实现目标，不表示所有能力已经部署。Diagnosis V2 的新医学身份和确认语义取代原 V1 规则；Training V2 与 Safety 的批量业务决策也已登记。历史决策仍留存但不作为新实现依据。

## 核心对象一览

- **BodyState 身体状态**：每位用户一份；明确的本人陈述自动记录，五类 BodyFact 与症状 Episode 分离、历史和更正独立；关键冲突在实时 Chat 主动 Ask User；用户自报观察与文件证据分级，用户本人怀疑独立记录（R-BS-01～10、BS-D1～D10）。
- **Diagnosis V2 综合诊断**：每位用户一份；当前显示经用户采纳的具名医学疾病/综合征诊断（ConditionEntry），停用历史保留。AI suspected/likely 证据评估、医生原始声明、多来源、BodyState 多对多证据及用户 adopted/inactive 分离；逐项确认、一次原子修订（R-DX-01～17、DX-D5～D15）。
- **TrainingPlan / Trainer（TP-D1～D12）**：多份独立完整处方；Draft 是尚未采纳的计划预览，接受后才生效。PlanProposal 统一创建/调整建议，计划修订与真实执行日志分离；每组完成或跳过须有明确用户行为；默认频率而非自动日历排期（R-TP-01～10、R-EX-01～08）。
- **SafetyState（SAFE-D1～D8）**：BodyState 内唯一风险权威；advisory/restricted/urgent 分级与范围匹配，Trainer 严格执行门禁，专业放行和复核解除按来源校验，不改写计划内容或状态（R-SAFE-01～08）。
- **交互空间**：3D 人体持续作为底层桌面，Chat、Person、Diagnosis、TrainingPlan、Trainer 为浮动窗口（[interface](./interface.md)）。

## 设计参照

Penpot **BodySense · Product Design** 的 Product 页是视觉基准：[桌面原型](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) · [移动端原型](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=0)。设计稿中的 Assistant / Assessment / Profile / Training 等旧名见 [术语表 §3](./glossary.md#3-penpot--旧代码名称对照)。
