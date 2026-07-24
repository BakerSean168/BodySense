# Agent Skills

项目专属的 AI Agent 技能包。每个 skill 是一个独立目录，包含 `SKILL.md`（技能描述和执行指令）和可选的辅助脚本。

## 安装

将 skill 目录复制或软链接到本机的 agent skills 目录：

- **Claude Code**：`~/.claude/skills/`
- **Codex**：`~/.codex/skills/`（若未设置 `$CODEX_HOME`）
- **QoderWork**：通过 QoderWork 设置安装

```bash
# 示例：安装 validate-local-deploy skill
ln -s "$(pwd)/tools/agent-skills/validate-local-deploy" ~/.claude/skills/bodysense-validate-local-deploy
```

## 当前 Skills

| Skill | 用途 |
|-------|------|
| `validate-local-deploy` | 验证 Docker Compose 本地环境能否正常启动 |
| `validate-rag-pipeline` | 检查 RAG 知识库质量、召回率、答案准确率 |
| `validate-docs-code` | 验证 PRD/技术方案与实际代码的一致性 |
| `coding-coach` | 教练式编码辅导:概念讲解、分层提示、代码评审、刻意练习,并用 `.practice-map/` 持久化学习计划(来自 cc-switch) |
