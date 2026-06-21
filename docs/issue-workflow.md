# GitHub Issue 开发流程

本文档描述 BodySense 项目中基于 GitHub Issue 的完整开发流程：从领取 Issue 到提交 PR 并自动关闭 Issue。

---

## 整体流程概览

```
找 Issue → 认领确认 → 创建分支 → 本地开发 → 提交代码 → 推送并开 PR → 审查合并 → Issue 自动关闭
```

---

## 1. 获取 Issue

### 网页方式

进入仓库 → **Issues** 标签页 → 使用筛选条件：

```text
is:issue is:open                          # 所有未关闭的 Issue
is:issue is:open label:"good first issue" # 适合新手的任务
is:issue is:open label:"bug"              # Bug 修复
is:issue is:open assignee:@me             # 分配给我的
```

### GitHub CLI 方式

```bash
# 查看仓库所有 open 的 Issue
gh issue list

# 查看某个 Issue 的详情和评论
gh issue view 123 --comments
```

### API 方式

```bash
curl -H "Accept: application/vnd.github+json" \
  https://api.github.com/repos/OWNER/REPO/issues/123
```

---

## 2. 认领与确认

在开始写代码之前，先做好确认工作：

1. **阅读 Issue 描述**：理解需求、复现步骤、期望行为
2. **检查是否已有人在做**：看评论区和 assignee
3. **评论表明意图**：留言 "I'd like to work on this issue."
4. **Assign 给自己**：如果有权限，直接 assign；开源项目先等维护者确认
5. **在 `docs/plan/` 下写实施计划**：参考 `docs/plan/active/issue-2-auth-jwt.md` 的格式

---

## 3. 创建分支

### 分支命名规范

本项目的分支策略为 `main` ← `dev` ← `feature/*`。分支命名应包含 Issue 编号和简短描述：

```bash
# 从 dev 拉出新分支
git checkout dev
git pull origin dev
git checkout -b feat/123-short-description
```

命名模式参考：

| 类型 | 分支名示例 | 适用场景 |
|------|-----------|---------|
| 新功能 | `feat/123-user-auth` | 新增功能 |
| Bug 修复 | `fix/456-login-crash` | 修复 Bug |
| 文档 | `docs/789-api-guide` | 纯文档修改 |
| 基础设施 | `chore/101-ci-setup` | CI/CD、工具链 |

### 无写权限时（Fork 模式）

```bash
gh repo fork OWNER/REPO --clone
cd REPO
git checkout -b feat/123-short-description
```

---

## 4. 本地开发与验证

### 开发阶段

修改代码时遵循项目的架构约定（参见 AGENT.md）：

- **前端** (`apps/web`)：Feature-based 结构，PascalCase 组件，camelCase 工具函数
- **后端** (`apps/api`)：Go standard layout，snake_case 命名
- **AI 服务** (`apps/ai-service`)：Python 分层结构，PEP 8 规范
- **共享包** (`packages/*`)：`@bodysense/*` 前缀

### 验证命令

提交前务必通过相关检查：

```bash
# 前端 - lint 和类型检查
pnpm nx lint web
pnpm nx typecheck web

# 后端 - 编译检查和测试
cd apps/api && go vet ./... && go test ./...

# AI 服务 - lint 和测试
cd apps/ai-service && uv run ruff check . && uv run pytest

# 全项目 lint（Nx 编排）
pnpm nx run-many -t lint
```

---

## 5. 提交代码

### Commit 规范

本项目使用 Conventional Commits，由 Husky + commitlint 强制执行。格式：

```text
<type>(<scope>): <description>

[可选的 body]

[可选的 footer]
```

**type** 可选值：`feat`、`fix`、`docs`、`style`、`refactor`、`test`、`chore`、`perf`、`ci`、`build`

**scope** 可选值：`web`、`api`、`ai`、`docker`、`docs`、`deps`

示例：

```text
feat(api): 实现用户注册和登录接口
feat(web): 添加登录页面和 auth store
test(api): 补充 auth service 单元测试
docs(docs): 添加认证模块知识库文档
```

### 提交步骤

```bash
git add .
git commit -m "feat(api): 实现 JWT 鉴权中间件"
```

> **提示**：commit message 中可以提及 Issue 编号（如 `Refs #123`），但自动关闭 Issue 的关键字应放在 PR 描述中，而非 commit message 中。

---

## 6. 推送并创建 Pull Request

### 推送分支

```bash
git push -u origin feat/123-short-description
```

### 创建 PR

使用 GitHub CLI：

```bash
gh pr create \
  --title "feat: 用户注册登录与 JWT 鉴权" \
  --body "$(cat <<'EOF'
## Summary
实现用户注册、登录和 JWT 鉴权的完整功能。

## Related Issues
Fixes #123

## Changes
- 新增 auth service 和 handler
- 新增 JWT 中间件
- 新增前端登录/注册页面
- 新增 auth store
EOF
)"
```

或在 GitHub 网页上点击 **Compare & pull request** 按钮创建。

---

## 7. 自动关闭 Issue（关键）

### 核心机制

GitHub 会在 PR **合并到默认分支**时，自动关闭 PR 描述中引用的 Issue。触发自动关闭的关键字：

```text
close / closes / closed
fix / fixes / fixed
resolve / resolves / resolved
```

### 用法

在 PR 描述（body）中写入：

```text
# 关闭单个 Issue
Fixes #123

# 关闭多个 Issue（每个关键字单独一行或逗号分隔）
Fixes #1
Fixes #2

# 或写成一行
Fixes #1, Fixes #2

# 跨仓库引用（如果 Issue 在另一个仓库）
Fixes OWNER/REPO#123
```

> **注意**：关键字必须写在 PR 描述中才会生效。写在 PR 标题或评论中不会触发自动关闭。

### 完整示例

假设你的 PR 同时解决了 Issue #1（开发基础设施）和 Issue #2（认证鉴权）：

```bash
gh pr create \
  --title "feat: 搭建开发基础设施并实现用户认证" \
  --body "$(cat <<'EOF'
## Summary
搭建项目开发基础设施，并实现用户注册/登录 + JWT 鉴权功能。

## Related Issues
Fixes #1
Fixes #2

## Changes
- 搭建 Docker 开发环境（PostgreSQL + Redis）
- 配置 Nx monorepo 和项目骨架
- 实现 Go 后端 auth service、handler、middleware
- 实现 React 前端登录/注册页面和 auth store

## Testing
- [x] 后端单元测试通过
- [x] 前端 lint 和类型检查通过
EOF
)"
```

PR 合并后，Issue #1 和 #2 会自动被关闭。

---

## 8. PR 审查与合并

创建 PR 后的典型流程：

1. **CI 自动跑测试**（如果配置了 GitHub Actions）
2. **Reviewer 审查代码**，提出修改意见
3. **根据反馈继续修改**，在同一个分支提交并推送：
   ```bash
   git add .
   git commit -m "fix(api): 修复 review 中发现的问题"
   git push
   ```
4. PR 会自动更新，无需重新创建
5. **Review 通过后合并**
6. 合并后 Issue 自动关闭（如果 PR 描述中有 `Fixes #N`）

### 合并后的收尾

```bash
# 切回 dev 分支并拉取最新
git checkout dev
git pull origin dev

# 删除已合并的本地分支
git branch -d feat/123-short-description

# 将实施计划归档
# 把 docs/plan/active/issue-123-xxx.md 移到 docs/plan/archive/
```

---

## 常见问题

### Q: Issue 在 PR 合并后没有自动关闭？

检查以下几点：
- PR 描述中是否使用了正确的关键字（`Fixes`、`Closes`、`Resolves`）
- PR 是否合并到了仓库的**默认分支**（通常是 `main` 或 `master`）
- 关键字和 Issue 编号之间是否有空格（`Fixes #123` ✅ `Fixes#123` ❌）
- 如果 PR 是通过 squash merge 合并的，检查 squash 后的 commit message 是否保留了关键字

### Q: 一个 PR 能关闭多个 Issue 吗？

可以。在 PR 描述中为每个 Issue 写一行关键字即可：

```text
Fixes #1
Fixes #2
Fixes #3
```

### Q: 想关闭的 Issue 和 PR 不在同一个仓库？

使用完整的仓库引用：

```text
Fixes OWNER/REPO#123
```

### Q: 分支命名带 Issue 编号，PR 描述也要写 Fixes 吗？

是的。分支名中的 Issue 编号只是方便人识别，GitHub 不会根据分支名自动关闭 Issue。必须在 PR 描述中使用关键字。
