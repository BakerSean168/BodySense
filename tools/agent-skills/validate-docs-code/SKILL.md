# validate-docs-code

验证 BodySense 的项目文档（PRD、技术方案）与实际代码实现的一致性。

## 触发条件

- 修改了 `docs/` 目录下的任何文档
- 完成了较大的功能开发后
- 用户主动要求验证文档一致性

## 执行步骤

1. **检查技术方案中的版本与实际依赖的一致性**
   ```bash
   # 前端依赖版本
   cd apps/web
   cat package.json | grep -E '"react"|"vite"|"tailwindcss"|"zustand"|"@tanstack/react-query"'

   # Go 依赖版本
   cd apps/api
   grep -E 'gin-gonic|gorm' go.mod

   # Python 依赖版本
   cd apps/ai-service
   grep -E 'fastapi|langchain' pyproject.toml
   ```

2. **检查技术方案中的 API 接口是否已实现**
   - 读取 `docs/technical-approach.md` 中 "5. API 接口设计" 章节定义的接口
   - 对照 `apps/api/internal/handler/` 中的路由注册
   - 报告已实现和未实现的接口

3. **检查技术方案中的数据模型是否与代码一致**
   - 读取 `docs/technical-approach.md` 中 "4. 数据库设计" 章节
   - 对照 `apps/api/internal/model/` 中的结构体定义
   - 检查字段名、类型、关系是否匹配

4. **检查 PRD 中的页面是否在原型和代码中都有对应**
   - 读取 PRD 第 9 节"页面与流程概览"中的 7 个核心页面
   - 对照 `apps/web/src/pages/` 中的页面组件
   - 报告覆盖情况

## 成功标准

- 技术方案中的版本号与实际依赖版本一致（允许 patch 版本差异）
- API 接口定义与代码路由注册一致
- 数据模型字段定义与代码 struct 一致
- PRD 中的 7 个核心页面在代码中都有对应

## 失败处理

- 版本不一致：更新文档或升级依赖，使两者对齐
- 接口不一致：标记为待实现或更新文档移除已废弃的接口
- 数据模型不一致：以代码为准更新文档，或以文档为准修改代码（需确认）
- 页面缺失：记录为待开发任务
