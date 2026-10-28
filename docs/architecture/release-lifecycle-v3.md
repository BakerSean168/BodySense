# Release 生命周期：Nightly、Stable 与生产隔离

发布状态有意拆分为四个独立的事实：**Main 代码通过 CI、完整 Nightly 构建成功、Stable 手动签发、Production 手动选择并部署**。

```text
PR Fast / Check → Main Full（无产物）
                         └→ Scheduled Main Full（全选）
                            → Nightly Candidate（原 SHA、四镜像 Digest、CDN、Manifest）
                            → 人工验收 → Publish Stable（不可变版本，原 Digest）
                            → Deploy Production（人工审批、Watcher 迁移与回滚）
```

Main 的源码集成不是自动发布授权。候选构建必须证明其源 Scheduled CI 真实通过全量 Check，并在产物元数据中绑定测试回执与 Manifest。复用同 SHA 候选不代表已有新候选；只有成功的 Nightly Candidate Run 可作为 Stable 输入。

Stable 按显式 `candidate_run_id` 和 `release_tag=vX.Y.Z` 晋升；四镜像 Digest 保持不变，部署位置也不改变。生产只接受正式 Published Release 的完整 Release Manifest，经独立受控环境门禁进行 tag selection；部署 Watcher负责数据库备份、迁移、健康检测和回滚。旧版 Stable 可按严格的历史来源验证回滚。

Prepare Release / release-please 是**可选的版本文件与 CHANGELOG 准备工具**，不是 Stable 发布触发器。详见 [Release Runbook](../operations/release.md)、[Delivery Platform](delivery-platform-v3.md) 和 [ADR 0008](../adr/0008-adopt-delivery-platform-v3.md)。
