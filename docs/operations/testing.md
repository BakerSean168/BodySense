# BodySense 测试与 CI

现行唯一主工作流：`.github/workflows/ci.yml`。它生成 SHA/Run/Attempt 绑定的 Manifest，测试并行，最终 `Check` 逐项验证必需结果；缺少、取消和非法 Skip 均失败。GitHub main 只要求 `Check`，仍要求与 main 保持同步。

| 时机 | 任务 | 说明 |
| --- | --- | --- |
| 本地编辑 | 最近的 Unit/Component/Go/Python 测试、必要 Nx affected | **不默认**启动完整 E2E 或 prod-like Docker |
| PR Fast | 全仓 OXFmt/Oxlint + Changed Scope；受影响 Web、Go API、AI Service、Contracts、PG18 相关测试并行 | 不运行跨服务 Playwright Full E2E、灾难恢复和发布构建 |
| Main Full | 全面 Web/API/AI/Contracts 验证、PG18 集成与恢复、Playwright 三分片与合并报告、Release Safety | Main 成功**不触发 Candidate** |
| Scheduled Main Full（Nightly 资格） | 同 Main Full，全选任务 + 最终 `Check` 完整回执 | 只有全部必需任务成功且回执身份一致才可被 CD 使用 |

`CI Check` 回执不仅记录成功，还记录精确 source SHA、GitHub Run ID/Attempt、Manifest Digest 和全部 Job 结果。测试分片的全部成功与结果合并是完整回归所必需的，不允许用可见绿色代替实测。

**基线（2026-10-26）**：VNext v2 合入 main（PR #475/#477）；Main Full 16/16 成功约 7 分 42 秒（run #40331012685）；PR Fast 冷缓存约 194 秒、重跑约 222 秒。两次 PR 通过不代表各类别长期 P95 已达到 300 秒统计目标。

阅读源代码、测试和 workflow 是理解具体 target 的首选；历史计划与 Review 记录在 GitHub Issues/PRs 中。
