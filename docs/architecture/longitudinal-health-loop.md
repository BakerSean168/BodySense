# BodySense · 长期身体状态业务事件流

~~~text
用户陈述 / 测量 / 自测 / 训练反馈
          │
          ▼
解析归属、时间、证据和真实变化
          ├─ 明确事实 → BodyState 自动记录
          └─ 关键歧义 → ask_user 澄清
                            │
                            ▼
                 BodyState 当前投影
                            │
             ┌──────────────┴──────────────┐
             ▼                             ▼
    用户请求正式分析                 计划相关性检查
             │                             │
             ▼                             ▼
    证据与安全审查                   Agent 可提出建议
             │                             │
             ▼                             │
   confirm_condition：用户逐个决定         │
   每个病情提案（三选一 / 继续补充）       │
             │                             │
             ▼                             ▼
   一次原子提交更新 Diagnosis        等待用户选择
                                           │
                                         同意
                                           │
                                           ▼
                                修改指定 TrainingPlan
                                           │
                                           ▼
                                  训练执行与反馈
                                           │
                                           └──→ BodyState
~~~

- BodyState 是身体事实及时间变化的权威聚合。
- Diagnosis 是一份当前综合结果，内部病情可以定向更新，整体原子修订；病情条目只有经用户在 confirm_condition 中决定后才写入，授权与确认规则见 [agent-workflows §3.2](../product/agent-workflows.md#32-diagnosis-正式分析与病情确认) 与 [§4 交互工具状态](../product/agent-workflows.md#4-交互工具状态)，本文不另行定义。
- Plan 影响分析只读；未授权时现行训练内容保持不变。
- 安全提醒与执行限制具有独立的优先级，不是隐式的训练内容变更。
- 普通知识解释不触发本人病历写入或正式诊断发布。

具体契约见 [领域模型](../product/domain.md)、[Agent 工作流](../product/agent-workflows.md)。
