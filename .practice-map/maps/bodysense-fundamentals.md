---
id: bodysense-fundamentals
title: BodySense 五语言进阶（Go / Python / JavaScript / TypeScript / React）
status: active
level: beginner
language: go, python, javascript, typescript, react
created_at: 2026-07-13
updated_at: 2026-07-29
---

# Goal

用 BodySense 这个真实项目当教材，把 **Go / Python / JavaScript / TypeScript / React** 五门语言的基础和工程心智打扎实：
不是背语法，而是能**读懂**项目里的核心代码、能**改**已有功能、最终能**独立实现**一个未完成的待办并合并 PR。

# Why

- 你手里就有一个 ~41k 行、三语言、结构清晰的真实工程，比任何教程都真实。
- 三端各自代表一类典型范式：Go（分层、并发、持久化）、Python（FastAPI、async、Pydantic）、JavaScript/TypeScript/React（流读取、类型契约、Reducer 与状态边界）。
- 学完能直接兑现价值：项目里有一批**明确、有限、可闭环**的待办（见 `docs/learning/05-practice-tasks.md`），学一个就能交付一个。

# Milestones

- **M1 · 读懂五语言基础**（阅读为主）
  - 读 `docs/learning/01-go-fundamentals.md` → 能说清 package main / 分层 / `:=` vs `=` / 多返回值+error / struct tag / 依赖注入。
  - 读 `docs/learning/02-python-fundamentals.md` → 能说清 async/await / Pydantic / 类型提示 / 异常链。
  - 读 `docs/learning/03-react-fundamentals.md` → 能说清 `UI=f(state)` / 各 Hook 用途 / Zustand vs TanStack Query（客户端态 vs 服务端态）。
  - 读 `docs/learning/06-javascript-typescript-streaming.md` → 能说清字节块、文本行、协议事件、运行时校验和 reducer 的分层。
  - 自测：每篇末尾的 self-check 全部能口头回答。
- **M2 · 读懂一条闭环**（阅读 + 画图）
  - 读 `docs/learning/04-closed-loop-features.md`，选"登录闭环"手画一遍时序图（前端→Go→DB）。
  - 目标：能指出每一步"为什么这么做"（如 `json:"-"` 防密码泄漏、JWT alg 校验防混淆攻击）。
- **M3 · 热身改一处**（动手，L0）
  - 完成新版 `P1 调试日志` 或 `P2 姿态校验`。目标：跑通“改代码→测试绿→复盘”的完整循环。
- **M4 · 建立安全边界**（动手，L1）
  - 完成 `P3 StreamEvent 运行时解析` + `P4 流分块测试`。目标：不再把类型断言误当作数据校验。
- **M5 · 修异步正确性**（动手，L2）
  - 完成 `P5 异步连接池` + `P6 CPU 阻塞下沉`。目标：区分同步 I/O 和 CPU 工作如何阻塞事件循环。
- **M6 · 打通流式恢复**（动手，L3）
  - 完成 `P7 统一事件校验` → `P8 事件续传与取消`。目标：跨 TS/React/Go 保持事件正确性。
- **M7 · 独立交付**（动手，L4）
  - 完成 `P9` 的一个纵向切片，从问题定义、契约、测试到用户验收独立闭环。

# Current Focus

**M1 · 读懂五语言基础** —— 先读 `docs/learning/01~03`，再读 `06-javascript-typescript-streaming.md`，通过各篇 self-check。
读的时候遇到不懂的概念，随时对我说 `Explain <概念>`。

# Exercises

练习任务的完整清单、难度分层、验收标准见 → `docs/learning/05-practice-tasks.md`

推荐顺序（与里程碑对应）：
- L0：`P1` 调试日志 · `P2` 姿态校验
- L1：`P3` 运行时事件解析 · `P4` 流分块测试
- L2：`P5` 异步连接池 · `P6` CPU 阻塞下沉
- L3：`P7` 统一事件校验 · `P8` 续传与取消
- L4：`P9` 独立纵向切片

# Session Log

## 2026-07-13

- 创建学习方案与 practice map，状态置为 `active`。
- 已产出配套教材：`docs/learning/01~04`（Go/Python/React/闭环，含真实代码逐行注释）+ `05`（练习任务清单）。
- 当前焦点定为 M1（读懂三端基础），起点为 Go 基础文档。

## 2026-07-29

- 同步最新 `origin/dev` 后重新核对学习资料与代码。
- 原 P6–P9 的体态工具、姿态估计、多模态和契约基础已经落地，不再列为待实现。
- 新增 JavaScript/TypeScript 流式链路教材，并把路线扩展为五语言。
- 当前仍处于 M1，但完成标准增加“能解释从网络字节到可信 React 状态”的完整分层。

# Next Step

读 `docs/learning/06-javascript-typescript-streaming.md` 的第 1–4 节，然后不看文档画出“字节块 → 文本 → 协议 → unknown → StreamEvent → reducer”的流程；下一次用 `Review` 让我校准。
