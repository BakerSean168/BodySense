# BodySense 开发流程

## 最短路径

`Issue（需求与验收） → 单写入者分支 → 定向测试 → PR Fast → 单一 Check → 合入 main`

先读取真实代码、契约、变更涉及的边界；需求来自 GitHub Issue，业务语义来自 [产品规格](../product/README.md)。无需每次开发都完整构建后端/容器或执行全仓测试。

- Web：`pnpm nx run web:test` 或对应 Vitest 文件，必要时 Storybook Browser。
- API：`cd apps/api && go test ./internal/相关包/...`；涉及 PG18 时使用真实隔离的集成测试。
- AI：`cd apps/ai-service && uv run pytest tests/相关文件.py`。
- Contracts：`pnpm contracts:verify`（跨语言/Schema 更改时）。
- 影响范围：`pnpm nx affected -t lint,typecheck,test --base=origin/main --head=HEAD --exclude=bodysense`（提交前）。
- 仓库根格式门禁：`pnpm format:check` / `pnpm lint:oxlint`，通常交给 PR CI；不要默认在本地执行全部 E2E。

同步修改对应回归测试，验证真实行为；UI 改动还需视觉与键盘/焦点证据。每个完成项只在 Issue 或 PR 记录验证结果、提交 SHA 和剩余限制，禁止另起 Active Plan。

并行仅允许不同拥有者的独立工作树/文件范围，合并冲突时先同步 main。PR 的 Required Check 是 `Check`；CI 具体分层见 [Testing](testing.md)，发布见 [Release](release.md)。
