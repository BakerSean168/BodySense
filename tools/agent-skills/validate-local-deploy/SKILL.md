# validate-local-deploy

验证 BodySense 的 Docker Compose 本地开发环境能否正常启动并通过健康检查。

## 触发条件

- 修改了 `docker/` 目录下的任何文件
- 修改了任何 `Dockerfile`
- 修改了 `.env` 相关配置文件
- 用户主动要求验证本地部署

## 执行步骤

1. **构建并启动开发环境**
   ```bash
   cd docker
   docker compose -f docker-compose.yml --profile dev up -d --build
   ```

2. **等待健康检查通过**（最多等待 120 秒）
   ```bash
   # 检查 PostgreSQL
   docker compose -f docker-compose.yml --profile dev ps postgres-dev
   # 检查 Redis
   docker compose -f docker-compose.yml --profile dev ps redis-dev
   ```

3. **验证服务连通性**
   ```bash
   # 验证数据库连接
   docker exec bodysense-dev-db pg_isready -U bodysense
   # 验证 Redis 连接
   docker exec bodysense-dev-redis redis-cli -a bodysense123 ping
   ```

4. **清理**
   ```bash
   docker compose -f docker-compose.yml --profile dev down
   ```

## 成功标准

- 所有容器状态为 `healthy`
- PostgreSQL 可接受连接
- Redis 返回 `PONG`

## 失败处理

- 如果容器启动失败，检查 `docker compose logs <service>` 获取错误详情
- 如果是端口冲突，报告冲突端口并建议替代端口
- 如果是镜像拉取失败，检查网络连接和镜像仓库可达性
