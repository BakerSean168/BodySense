# BodySense · 系统架构

## 产品领域与业务契约

**产品业务规范以 [docs/product](../product/README.md) 为准。** 它定义 BodyState、Diagnosis、TrainingPlan、Agent 操作权限、当前界面和验收标准。

- [领域模型](../product/domain/README.md)
- [Agent 工作流](../product/agent-workflows.md)
- [UI 与交互](../product/interface.md)
- [领域状态实现边界](./longitudinal-body-state-domain.md)
- [业务事件流](./longitudinal-health-loop.md)

## 运行与数据

| 主题 | 文档 |
| --- | --- |
| 当前服务与数据边界 | [Current Longitudinal System](./current-longitudinal-system.md) |
| Agent 工具运行 | [Agent Tool Calling Runtime](./agent-tool-calling-runtime.md) |
| Agent 上下文 | [Context Engineering](./context-engineering-architecture.md) |
| 业务/AI 事件与流协议 | [Stream Event Contract](./stream-event-contract-runtime.md) |
| 诊断证据及安全判定 | [Diagnosis Decision Authority](./diagnosis-decision-authority.md) |
| 训练方案生成治理 | [Treatment Agent Configuration](./treatment-agent-configuration.md) |
| 身体部位本体 | [Body Region Ontology](./body-region-ontology.md) |
| 3D 解剖资源 | [3D Body Explorer](./body-explorer-3d-anatomy.md) |
| 健康文件证据 | [Health Document Evidence Pipeline](./health-document-evidence-pipeline.md) |

## 开发与交付

- [技术方案](./technical-approach.md)
- [部署架构](./deployment-architecture.md)
- [Release 生命周期](./release-lifecycle-v3.md)
- [交付平台](./delivery-platform-v3.md)
- [模型网关](./model-gateway-routing.md)
- [真实业务 LLM 接入调研（Chatwoot / FastGPT / Onyx）](./business-llm-integration-case-studies.md)
- [OpenRouter / Groq 免费渠道连通性记录](../runbooks/openrouter-groq-free-connectivity.md)
- [知识库生命周期](./knowledge-lifecycle-architecture.md)

运行细节以真实代码、API 契约和数据库迁移为准。产品规格中的逻辑对象、字段、工具名是要实现和验证的目标。
