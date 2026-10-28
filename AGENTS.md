# BodySense — Agent Instructions

本文件是仓库唯一的通用 AI Agent 工程规范。其他助手入口仅引用本文件。用户明确指令优先；不得绕过安全门禁。

## 事实与边界

- **产品语义**：以 [docs/product](docs/product/README.md) 的业务规则、领域、交互及验收为准；产品规则尚未确定时，不得根据旧代码或历史设计自行创造业务规则。
- **技术现状**：以正在执行的代码、测试、配置和契约为准；[docs/architecture](docs/architecture/README.md) 和 ADR 解释跨组件决策。如技术文档与代码不符，修正文档，不以旧文档覆盖代码。
- Web 为 React/TS，API 为 Go，AI Service 为 Python，跨语言契约在 packages/contracts、tools/contracts。保持 Web、Server、AI、Contracts 的责任边界；用户确认、健康判断、安全语义、持久化和人工中断不可由模型猜测。
- 先定位真实文件、符号、调用链和测试，再动手。变更应直接解决根因；避免额外抽象、重复契约、兼容层和临时治理框架。

## 实施与交付

- **GitHub Issue 是任务、方案、验收、交接的唯一活动记录。** 不在 docs/ 新增 Active Plan、任务台账、过程报告、重复 Prompt、流水日志或逐 PR 审查快照。只有跨组件的持久设计决策、可重复的操作步骤和产品规则才进入仓库文档。
- 每个工作树只允许一个写入者；其他 Agent 可独立分析或负责不重叠的 PR/工作树。开始前查看 Git 状态及活跃 Issue，避免覆盖他人改动。
- 在专用分支中实施；提交遵循 Conventional Commits；通过 PR、受保护分支和唯一必需检查 `Check` 集成。不得直接推送 main、force-push 共享分支或临时关闭保护。独立任务不必串行等待其他 PR 的 Main CI；同一文件冲突先协调。
- 复杂任务中断时在 Issue 留 **分支、提交、已执行验证、阻塞、下一步** 的短检查点。不要让另一份文档与 Issue 争夺权威。
- 不为提高 CI 通过率而跳过必要测试、伪造完成回执、替换不一致的 SHA/Digest 或扩大 Skip 白名单。

## 开发与验证（T3 Code 式小步快验）

1. 阅读相关代码与测试，明确影响范围。优先修一个行为并跑**最近的测试**；修复协议、状态或逻辑缺陷时增加聚焦回归测试。
2. Web / TS：`pnpm nx run <project>:<target>`，按需运行 Vitest 或 Storybook Browser；Go API：在 `apps/api` 用 `go test` 或对应 package test；AI：在 `apps/ai-service` 用 `uv run pytest` / Ruff；Contract、数据库迁移、跨服务变更验证其实际边界。
3. 提交前以 changed/affected 为主：`pnpm nx affected -t lint,typecheck,test --base=origin/main --head=HEAD --exclude=bodysense`，同时按需 `pnpm format:check`、`pnpm lint:oxlint`。CI 的 Lint 与受影响分支并行；本地**默认不跑全仓 E2E、生产形态 Docker 打包、整套测试**。有针对性的发布或数据库变更才执行专项验收。
4. PR Fast 不跑完整跨服务 E2E；Main Full 执行全面回归（包含 Playwright 三分片、PG18 集成与恢复、Release Safety），最后 Check fail-closed。职责和命令参见 [Testing](docs/operations/testing.md)。
5. UI 改动提交可复现的视觉/交互前后证据，涉及 A11y、键盘、空态、异常和重试时覆盖相关状态。不可将“Job skipped”或“资格检查绿色”视为完整构建/发布成功。

## CD / 环境安全

- Main push **不自动构建 Candidate**。成功的定时 Main Full 才能产生 Nightly 候选；同 SHA 可复用已构建的候选。候选绑定 source CI、完整 Check 回执、四镜像 Digest 和静态 CDN 资源。
- Stable 必须人工选定已成功构建的 Nightly Run 与不可变版本号，**只晋升原 Digest，不重新编译**；发布 Stable 不等于生产部署。
- 生产发布通过独立手动 workflow 与 GitHub `production` 环境门禁；迁移、备份、资源归属、数据库回滚和人工签核不能由开发 Agent 自行放宽或静默执行。详情见 [Release](docs/operations/release.md)。
- 开发不得使用真实凭据、生产数据或用户持久配置做测试；遇到需要操作真实生产、清空数据、修改 Secret 或门禁的任务，应保留安全检查并取得明确授权。

## 文档入口

[文档索引](docs/README.md) · [本地开发](docs/operations/development.md) · [测试](docs/operations/testing.md) · [发布](docs/operations/release.md) · [产品契约](docs/product/README.md) · [架构决策](docs/architecture/README.md)

历史尝试从 Git、Issue、PR 检索；不要恢复平行文档规范。
