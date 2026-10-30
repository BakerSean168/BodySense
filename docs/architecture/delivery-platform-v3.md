# BodySense Delivery Platform — 现行边界

> 本页替代历史 V3/VNext 阶段执行记录。实际权威是 GitHub Actions 工作流、`scripts/delivery` 契约与可验证的运行证据；历史流水和性能调试见 Git/Issues/PR。

## 流水线

`Local focused tests → PR Fast / Check → main Main Full → scheduled Main Full → Nightly Candidate → manual Stable → manual Production`。

- PR 使用全仓 Lint + 受影响 Web / Go / AI / Contracts / PostgreSQL 检查并行，最终 `Check` 对 SHA、Run、Attempt、Manifest 和 selected jobs 做 fail-closed 汇聚。PR 不运行完整 E2E。
- Main push 完整回归（E2E 三分片、PG18 恢复与 Release Safety），**只验证，不构建候选**。
- Scheduled Main Full 全选。成功后独立的 `Publish Nightly Candidate` 下载**该次** `Check` 回执，验证所有必需任务真实成功，然后构建四种候选镜像（Web、API、AI Service、Runtime）和按 SHA 版本化的 CDN 资产；相同 SHA 的成功候选直接复用。也可人工 dispatch `Publish Nightly Candidate`，绑定当前 main HEAD 上成功的全量 CI（schedule 或手动 dispatch 的 `ci.yml`），其余回执与 Digest 门禁相同。
- 发布候选必须有 `candidate-set-v1.json`：source SHA、CI Run ID、delivery Manifest Digest、migration head、四镜像原始 Digest、静态资源归属。任何缺件、错配或错误 Skip 均不构成可发布候选。
- `Publish Stable` 仅人工调度。给定 Nightly Run ID 与稳定 `vX.Y.Z`，验证候选，原 Digest 晋升为稳定版本，生成 `release-manifest.json`，**不重新 build**、不部署生产。
- `Deploy Production` 独立手动调度，受 `production` 环境保护。严格检验 GitHub Release、Tag、SHA、CI、Manifest、OCI Digest；Watcher 再负责备份、PG18 迁移、健康和回滚。

## 不变量

1. 源 SHA、Run/Attempt、测试回执、镜像 Digest 和 CDN 静态资产属于同一发布身份；绿色 workflow 不等于产物存在。
2. 可选择较旧的仍可达 `main` 的**成功 Nightly** 候选发布；新提交不自动覆盖旧版本。碰撞的不可变版本 Tag 必须拒绝。
3. CI 和 CD 分开，候选、Stable、Production 三阶段各有独立门禁。历史 Stable 的安全回滚路径保留。
4. 测试、格式、依赖/Schema 变更优先在相关 Lane 定向执行；不要为方便开发生成本地生产 Docker 镜像或在 Oracle2 囤积构建缓存。

使用方法见 [Release](../operations/release.md)、[Testing](../operations/testing.md)。发布实现文件：`.github/workflows/ci.yml`、`candidate-publish.yml`、`release-publish.yml`、`deploy-production.yml`；候选与 Release 约束定义在 `scripts/delivery/`。
