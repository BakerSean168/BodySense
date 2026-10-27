# BodySense 运行与部署架构

## 运行端

- 前端 React/Vite Web，后端 Go API，Python AI Service，Runtime 包含服务配置、Gateway 等依赖；PostgreSQL 18（pgvector）为生产数据库。
- 镜像发布到阿里云 ACR，生产运行在阿里云主机，由 Docker Compose 与本地部署 Watcher 消费经过验证的一组镜像身份。
- Web 静态哈希资产先发布到 Cloudflare R2/CDN 的**精确源 SHA 路径**，Web 镜像必须指向对应版本；解剖 atlas 单独版本化，不在每次发行包中重复复制。
- `staging-latest`、`prod-latest` 是可变通道指针，候选和稳定版本的`sha-<SHA>` / `vX.Y.Z` 不可变；Watcher 必须在四种镜像的源 SHA 收敛后才部署，不能部署混合版本。

## 交付路径

- PR Fast 和 Main Full 负责验证。Main push 通过后不自动产生候选。
- Scheduled Full 和完整 Check 回执决定 Nightly 能否构建镜像/静态资源。构建一次、按 Digest 晋升，不从另一个 SHA 重建。
- 显式人工 Stable 提升版本，显式人工 Production 审批后发布 `prod-latest`。生产部署前验明 Tag、Git SHA、各镜像 Digest、源 CI Run 和候选证明。
- 数据库迁移、数据备份、健康检查、失败恢复由主机 Watcher 和对应生产脚本控制。CI 或开发端不得直接执行危险清理。

**权威运维入口**：[Release](../operations/release.md)、[Testing](../operations/testing.md)、[Delivery Platform](delivery-platform-v3.md)、[生产 PG18 DR](../runbooks/production-postgres-dr.md)、[静态资源 CDN](../runbooks/static-asset-cdn.md)。实施细节以 `.github/workflows`、`docker/`、`scripts/production-deploy-watch.sh` 的现行代码为准。
