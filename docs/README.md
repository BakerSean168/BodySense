# BodySense 文档索引

**当前原则：产品规则、技术事实、操作步骤各只有一个权威来源；实施状态在 GitHub Issue/PR，而不是仓库内的 Active Plan。**

| 主题 | 唯一入口 | 备注 |
| --- | --- | --- |
| 产品规则、BodyState/Diagnosis/TrainingPlan、Agent、界面与验收 | [product/README.md](product/README.md) | **业务语义权威**，与代码有差异时标记待实施 |
| 系统架构和跨组件边界 | [architecture/README.md](architecture/README.md) | 技术现状以运行代码、配置和测试为准 |
| 架构决策记录 | [adr/](adr/) | 历史提案仅作为决策背景，不能覆盖产品规则 |
| 本地开发和 AI Agent 开发方式 | [operations/development.md](operations/development.md)、[AGENTS.md](../AGENTS.md) | Issue 为实施与交接真源 |
| 测试与 CI | [operations/testing.md](operations/testing.md) | PR Fast / Main Full / 单一 Check |
| Nightly / Stable / Production | [operations/release.md](operations/release.md) | 发布与部署分离 |
| 运维、安全与灾备 | [runbooks/](runbooks/)、[security/](security/) | 操作时以实际环境和配置复核 |
| 设计基准 | [design/README.md](design/README.md) | 对照现行产品词汇表 |
| 独立教学资料 | [learning/README.md](learning/README.md) | 学习内容，不属于项目执行规范 |

**不再维护**：按日期堆积的开发计划、任务队列、重构过程记录、重复 Agent 提示词及历史审查快照。这些在 Git 历史、Issue 和 PR 中可追溯。不要把旧计划当成当前授权的开发或发布步骤。
