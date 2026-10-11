# BodySense · 用户健康交互路径

用户可直接从问知识、描述身体变化、查看当前状态、请求综合诊断或执行训练开始，无必须完成的全局线性向导。

- 清晰的本人身体陈述 → 自动保存 BodyState。
- 有关键歧义 → Agent 发起针对性澄清。
- 用户明确请求正式分析 → 更新唯一综合 Diagnosis 的相应病情条目。
- 用户要求修改计划 → 更新对应 TrainingPlan。
- Agent 发现计划风险 → 提议调整，用户决定是否应用。
- 训练反馈与身体变化 → 进入 BodyState 及后续分析上下文。

每一步均可能单独结束或在另一日继续；安全规则始终优先于普通训练执行。

详细路径见 [业务事件流](./longitudinal-health-loop.md) 与 [Agent 工作流](../product/agent-workflows.md)。
