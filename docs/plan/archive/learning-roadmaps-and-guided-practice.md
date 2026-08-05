# BodySense 五语言学习路线与引导式实践计划

> 状态：已完成  
> 日期：2026-07-29  
> 范围：Thought Forest 学习路线、BodySense 教学文档、典型源码注释、Practice Map

## 目标

以最新 `origin/dev` 为准，把 BodySense 变成一条可持续的 Go、Python、JavaScript、TypeScript、React 学习路径：

1. 知识点在 Thought Forest 中有稳定入口和原子笔记。
2. 典型业务源码说明“知识如何在真实场景中使用”，并能跳转到笔记。
3. 练习任务来自当前代码缺口，按读懂、局部修改、跨端交付逐级增加难度。
4. `.practice-map` 记录当前焦点和下一次练习。

## 实施批次

### A. 知识库

- 新建 TypeScript 七阶段学习路线。
- 从现有 `typescript-advanced-types.md` 抽出高频原子概念。
- 为流式问诊补 Web Streams、NDJSON/SSE、取消模型笔记。
- 更新 JavaScript、React、TypeScript、前端与 BodySense MOC 入口。
- 保留旧笔记与 aliases，不破坏旧链接。

### B. 项目教材

- 校正 `docs/learning/` 中已经失效的代码路径和“待实现”状态。
- 增加 JavaScript/TypeScript 学习章节与跨语言契约章节。
- 重写实践任务，使其对应 2026-07-29 的最新代码。
- 更新现有 `bodysense-fundamentals` Practice Map，不创建平行学习计划。

### C. 教学注释

选取以下教学走廊：

- TS 契约：`packages/contracts/src/stream-events.ts`
- React 状态：`ActiveTurnContext.tsx`
- JS 流解析：`useSSEProcessor.ts`
- Go 事件持久化：`runtime_event_service.go`
- Python 事件模型与流式路由：`stream_event.py`、`runtime.py`

注释只解释关键机制、边界与设计原因；详细内容指向 Thought Forest 笔记，不做逐行复述。

## 验收

- Thought Forest：`npm run kb:audit -- --changed-only`、`npm run kb:drift`
- 前端：`pnpm nx run web:lint`、`pnpm nx run web:typecheck`
- Contracts：`pnpm nx run @bodysense/contracts:test`
- Go：`go vet ./...`、`go test ./...`
- Python：`uv run ruff check .`、`uv run pytest`
- 所有新增笔记可从相邻 MOC 找到，代码引用的笔记文件真实存在。
