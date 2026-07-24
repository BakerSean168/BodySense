---
id: bodysense-fundamentals
title: BodySense 三语言基础（Go / Python / React）
status: active
level: beginner
language: go, python, typescript
created_at: 2026-07-13
updated_at: 2026-07-13
---

# Goal

用 BodySense 这个真实项目当教材，把 **Go / Python / React** 三门语言的基础打扎实：
不是背语法，而是能**读懂**项目里的核心代码、能**改**已有功能、最终能**独立实现**一个未完成的待办并合并 PR。

# Why

- 你手里就有一个 ~41k 行、三语言、结构清晰的真实工程，比任何教程都真实。
- 三端各自代表一类典型后端/AI/前端范式：Go（分层 + GORM + JWT）、Python（FastAPI + async + Pydantic）、React（Hooks + Zustand + TanStack Query）。
- 学完能直接兑现价值：项目里有一批**明确、有限、可闭环**的待办（见 `docs/learning/05-practice-tasks.md`），学一个就能交付一个。

# Milestones

- **M1 · 读懂三端基础**（阅读为主）
  - 读 `docs/learning/01-go-fundamentals.md` → 能说清 package main / 分层 / `:=` vs `=` / 多返回值+error / struct tag / 依赖注入。
  - 读 `docs/learning/02-python-fundamentals.md` → 能说清 async/await / Pydantic / 类型提示 / 异常链。
  - 读 `docs/learning/03-react-fundamentals.md` → 能说清 `UI=f(state)` / 各 Hook 用途 / Zustand vs TanStack Query（客户端态 vs 服务端态）。
  - 自测：每篇末尾的 self-check 全部能口头回答。
- **M2 · 读懂一条闭环**（阅读 + 画图）
  - 读 `docs/learning/04-closed-loop-features.md`，选"登录闭环"手画一遍时序图（前端→Go→DB）。
  - 目标：能指出每一步"为什么这么做"（如 `json:"-"` 防密码泄漏、JWT alg 校验防混淆攻击）。
- **M3 · 热身改一处**（动手，L0）
  - 完成 `P1 日志开关` 或 `P2 类型标注`（见 05 清单）。目标：跑通"改代码→测试绿→提交"的完整循环。
- **M4 · 修一个缺陷**（动手，L1）
  - 完成 `P3 异步连接池`（最关键）+ `P5 CPU 阻塞下沉`。目标：真正理解"阻塞事件循环"的两种形态。
- **M5 · 扩一个功能**（动手，L2）
  - 完成 `P6 体态 Agent 工具` 或 `P7 姿态估计几何量化`。目标：复用已有骨架独立加功能。
- **M6 · 打通跨端**（动手，L3）
  - 完成 `P9 契约测试扩展` → `P8 诊断台多模态输入`。目标：改动三端并保持契约一致，这是工程能力的分水岭。

# Current Focus

**M1 · 读懂三端基础** —— 先从 Go 开始（`docs/learning/01-go-fundamentals.md`），按顺序读完三篇并通过每篇末尾的 self-check。
读的时候遇到不懂的概念，随时对我说 `Explain <概念>`。

# Exercises

练习任务的完整清单、难度分层、验收标准见 → `docs/learning/05-practice-tasks.md`

推荐顺序（与里程碑对应）：
- L0 热身：`P1` 日志开关 · `P2` 类型标注
- L1 修缺陷：`P3` 异步连接池 · `P5` CPU 阻塞下沉 · `P4` 真实 embedding
- L2 扩功能：`P6` 体态 Agent 工具 · `P7` 姿态估计几何量化
- L3 跨端：`P9` 契约测试扩展 · `P8` 诊断台多模态输入

# Session Log

## 2026-07-13

- 创建学习方案与 practice map，状态置为 `active`。
- 已产出配套教材：`docs/learning/01~04`（Go/Python/React/闭环，含真实代码逐行注释）+ `05`（练习任务清单）。
- 当前焦点定为 M1（读懂三端基础），起点为 Go 基础文档。

# Next Step

打开 `docs/learning/01-go-fundamentals.md`，读到"依赖注入 / NewXxx 构造器"一节，然后回来对我说一句你对 `:=` 和 `=` 区别的理解，我来帮你校准（`Review`）。
