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
| [领域待决](./domain-decisions.md) | 可访问材料中的待决项子集；原 83 项待合流 | DC-* |
| [实现差异](./implementation-gaps.md) | 源码静态证据与目标契约差异 | IG-* |
| [开放问题](./open-questions.md) | 规格内的矛盾与待决定事项 | `Q-*` |
| [决策记录](./decisions.md) | 已决定问题的结论、原因与写入位置（规则本身仍在上述各文件） | `Q-*`、`GAP-*` |

文中的逻辑结构与工具名称是实现目标，不表示所有能力已经部署。

## 核心对象一览

- **BodyState 身体状态**：每位用户一份；明确的本人陈述自动记录，有歧义才追问；用户本人的怀疑记为用户假设（R-BS-01～06）。
- **Diagnosis 综合诊断**：每位用户一份；只包含用户在 Chat 确认工具中决定采纳的病情条目（「大概率是」「先保留」或临床来源的「纳入」），来源与结论分开记录（R-DX-01～13）。
- **TrainingPlan 训练计划**：可有多份，每份是完整处方；Agent 建议需用户「接受调整」才生效（R-TP-01～05）。
- **交互空间**：3D 人体持续作为底层桌面，Chat、Person、Diagnosis、TrainingPlan、Trainer 为浮动窗口（[interface](./interface.md)）。

## 设计参照

Penpot **BodySense · Product Design** 的 Product 页是视觉基准：[桌面原型](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) · [移动端原型](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=0)。设计稿中的 Assistant / Assessment / Profile / Training 等旧名见 [术语表 §3](./glossary.md#3-penpot--旧代码名称对照)。
