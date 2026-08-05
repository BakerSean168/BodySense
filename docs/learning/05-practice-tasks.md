# 05 · 练习任务清单（基于 2026-07-29 最新代码）

> 这些练习来自当前代码的真实改进空间。一次只做一个任务；先自己实现，再用测试证明。
> 已完成能力不再伪装成“待办”：体态档案工具、姿态几何估计、问诊多模态输入和跨语言事件契约已进入代码，改为读码与增强练习。

## 难度梯度

| 层级 | 目标 | 任务 |
|---|---|---|
| L0 读懂与热身 | 单文件、结果明确 | P1、P2 |
| L1 安全边界 | 1–2 个文件和针对性测试 | P3、P4 |
| L2 异步正确性 | 理解运行时行为 | P5、P6 |
| L3 跨层功能 | 跨前端/Go/Python 边界 | P7、P8 |
| L4 独立设计 | 先写方案，再交付闭环 | P9 |

推荐顺序：**P1 → P2 → P3 → P4 → P5 → P6 → P7 → P8 → P9**。

---

## L0 · 读懂与热身

### P1 · 清理流式链路调试日志（JavaScript / TypeScript / React）

**练习目标**：区分纯函数、业务副作用和开发期诊断。

涉及文件：

- `apps/web/src/features/consultation/runtime/activeTurnReducer.ts`
- `apps/web/src/features/consultation/hooks/useSSEProcessor.ts`
- `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`
- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`

约束：

- reducer 内不得调用 `console.*`。
- 不改变事件处理结果。
- 开发期日志通过一个最小工具统一开关，不创建复杂日志框架。

验收：

- reducer 纯度测试通过。
- 生产构建不包含目标 debug 文本。
- `web:lint`、`web:typecheck` 和现有测试通过。

### P2 · 为姿态指标补业务范围校验（Python）

**练习目标**：理解类型提示和运行时校验的区别。

涉及文件：

- `apps/ai-service/src/models/posture.py`
- `apps/ai-service/tests/unit/test_pose_estimator.py` 或新增模型测试

任务：

- 为角度、比例或百分比设计合理的 Pydantic 约束。
- 明确通用 `PostureMetric` 是否能使用统一范围；不能时，应在具体指标生成边界校验，而不是写一个错误的万能范围。

验收：

- 合法现有 fixture 不受影响。
- 至少两个非法边界会稳定失败。
- `ruff` 和相关 pytest 通过。

---

## L1 · 安全边界

### P3 · 从 `unknown` 解析回放事件（TypeScript）

**练习目标**：联合类型、类型守卫、运行时校验、错误上下文。

当前问题：

`consultationService.listRunEvents` 对 `ids`、`payload` 和最终事件使用类型断言。断言不会验证服务端 JSON。

涉及文件：

- `apps/web/src/features/consultation/services/consultationService.ts`
- `packages/contracts/src/stream-events.ts`
- `apps/web/src/features/consultation/services/__tests__/consultationService.test.ts`

约束：

- 外部数据从 `unknown` 开始。
- 校验器放在网络适配层或 contracts 包，不散落进组件。
- 错误应指出记录序号或事件类型，便于诊断。

验收：

- 合法 fixture 解析成功。
- 缺少 `seq`、非法 `channel`、非对象 `payload` 会失败。
- 不用 `as unknown as StreamEvent` 绕过检查。

### P4 · 强化流分块测试（JavaScript）

**练习目标**：ReadableStream、TextDecoder、缓冲区和协议边界。

涉及文件：

- `apps/web/src/features/consultation/hooks/useSSEProcessor.ts`
- `apps/web/src/features/consultation/hooks/useSSEProcessor.test.ts`

需要覆盖：

- 一个 UTF-8 中文字符被切成多个字节块。
- JSON 行被切成三个块。
- `\r\n` 行尾。
- 最后一条记录没有换行。
- 读取过程中抛错。

加分项：验证流结束时是否需要调用 `decoder.decode()` 冲刷剩余字节。

---

## L2 · 异步正确性

### P5 · 知识库改异步数据库连接池（Python）

**练习目标**：事件循环、同步 I/O、连接池、异步上下文管理器。

当前证据：

- `knowledge_library.py` 的公开方法是 `async def`。
- 内部仍使用同步 `psycopg.connect()` 和同步 cursor。

约束：

- 对外方法签名和返回形状不变。
- 使用 `psycopg_pool.AsyncConnectionPool` 或等价的明确方案。
- 生命周期包含初始化、借用、归还和关闭。
- 不把同步连接藏进另一个 `async def`。

验收：

- 并发查询不会因一个同步数据库调用阻塞整个事件循环。
- 知识库单元测试与集成测试通过。
- 连接池关闭有测试或可观察验证。

### P6 · 把本地 embedding 移出事件循环（Python）

**练习目标**：区分 I/O 阻塞与 CPU 阻塞。

涉及文件：

- `apps/ai-service/src/rag/embedding.py`
- `apps/ai-service/tests/unit/test_embedding.py`

任务：

- 将同步 `model.encode(...)` 放入 `asyncio.to_thread` 或受控 executor。
- 保持批量和单条接口行为一致。
- 测试异常传播和返回值顺序。

验收：生成 embedding 时，另一个简单协程仍能得到调度。

---

## L3 · 跨层功能

### P7 · 流式事件运行时校验贯穿前端入口（TS / React）

**练习目标**：把 P3 的解析器接入直播流与回放流，形成单一可信入口。

涉及文件：

- `packages/contracts/src/stream-events.ts`
- `useSSEProcessor.ts`
- `consultationService.ts`
- 对应测试

约束：

- 同一验证逻辑服务直播和回放。
- 协议错误转换为可观察的 stream error，不让 reducer 接收非法事件。
- 不在每个组件重复检查。

验收：同一个坏 fixture 在直播和回放入口都以一致方式失败。

### P8 · 事件续传与取消的端到端演练（React / Go）

**练习目标**：AbortController、最后序号、幂等回放、重复事件处理。

任务：

1. 前端记录最后成功消费的 `seq`。
2. 模拟网络中断并取消旧 reader。
3. 调用 run events 接口从 `after_seq` 回放。
4. 验证重复事件不会重复更新界面。

涉及文件：

- `useAssistantChatRuntime.ts`
- `useSSEProcessor.ts`
- `consultationService.ts`
- Go `runtime_event_service.go` 及相关 handler/repository

验收：测试中在任意分块位置断开，恢复后的最终状态与未断开完全一致。

---

## L4 · 独立设计

### P9 · 设计并交付一个新的健康旅程纵向切片

候选方向：

- 某类训练提醒与完成反馈。
- 复诊前后的关键指标对比。
- 体态发现到训练动作的可解释关联。

交付顺序：

1. 写一页问题定义和非目标。
2. 画 TS/Go/Python 数据契约。
3. 先写一个跨端黄金 fixture。
4. 实现最窄可用纵向切片。
5. 补失败、取消、权限和可观察性。
6. 用真实用户流程验收。

限制：第一版最多新增一个主流程、一个数据模型和一个页面入口。

## 教练使用方式

- `Explain <概念>`：先建立心智模型。
- `Hint P3`：只给下一层提示，不直接贴完整答案。
- `Review <文件>`：按“现象、原因、修改、自检”评审。
- 每完成一个任务，更新 `.practice-map/maps/bodysense-fundamentals.md` 的 Session Log、Current Focus 和 Next Step。

What you learned：练习清单必须追随当前代码；已经交付的功能更适合成为读码样本和增强任务。

Next rep：先做 P1 或 P2，完成一次小改动、测试、复盘的完整循环。
