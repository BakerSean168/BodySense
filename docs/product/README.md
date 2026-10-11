# BodySense · 产品文档

BodySense 是持续理解个人身体状况的 Agent 应用。用户通过自然表达积累身体信息，按需获得有依据的综合分析，管理与执行训练计划。

## 文档导航

| 文档 | 说明 |
| --- | --- |
| [产品需求](./requirements.md) | 产品目标、用户任务、业务规则与边界 |
| [领域模型](./domain.md) | BodyState、Diagnosis、ConditionEntry、TrainingPlan 与数据契约 |
| [Agent 工作流](./agent-workflows.md) | 信息提取、澄清、正式分析、计划修改和确认 |
| [UI 与交互](./interface.md) | 3D 人体桌面、浮动窗口、Person 与互动工具 |
| [验收标准](./acceptance.md) | 端到端场景、不可破坏约束与实施顺序 |

这些文档共同构成产品规格。文中的逻辑结构与工具名称是实现目标，不表示所有能力已经部署。

## 核心原则

- **BodyState**：每位用户唯一且持续更新；明确的本人身体陈述默认自动记录，保留时间与来源。
- **Diagnosis**：每位用户唯一的当前综合诊断，由多个可单独更新的病情条目组成；整体作为一个修订单元。
- **TrainingPlan**：每位用户可有多份独立计划；仅按明确用户请求或已接受的 Agent 建议更新具体计划内容。
- **Agent**：通过对话和交互工具完成提问、分析和业务操作；后端负责校验、持久化和权限。
- **交互空间**：3D 人体持续作为底层桌面，Chat 和业务内容使用可移动、可收起的浮动窗口。
- **展示状态**：默认呈现当前身体信息、综合诊断和训练计划，不要求用户管理内部版本。

身体自述、客观观察、模型推测、临床诊断具有不同证据等级。BodySense 不能代替专业医疗评估。

## 设计参照

Penpot：[桌面原型](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) · [移动端原型](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=0)。

业务对象和用户任务决定交互功能，具体视觉组成与状态参照可编辑原型。
