# T0 亮点方案（三）：跨语言契约测试（Go / Python / TS 三方一致）

> 文档状态：调研 + 增强方案（待评审）
> 创建日期：2026-07-10
> 关联：`docs/project-review-2026-07-10.md` §1 T0-3
> 真值来源：当前代码。文中 `file:line` 为撰写时锚点，实施前以最新代码为准。

---

## 0. 一句话定位

三语言微服务最易出的问题是**契约漂移**（Go 发的 JSON 和 Python/TS 解析的不是同一形状）。本项目用一份共享 `stream-event.v1.schema.json` + 一份 `stream-events.v1.json` fixture，让 Go 与 Python **各自加载同一份 fixture** 做一致性（parity）测试——这是资深工程师才会做的事。

---

## 1. 现状架构剖析

### 1.1 单一真值：一份 schema、一份 fixture

```text
packages/contracts/
├── schemas/stream-event.v1.schema.json   # JSON Schema（draft 2020-12）
│     必填 version/seq/channel/type/ids/payload；channel 为固定 enum；
│     ids 为封闭对象（additionalProperties:false），7 个可空 id 字段
├── fixtures/stream-events.v1.json         # 3 条代表性事件（message/job/state）
├── src/stream-events.ts                   # TS 类型 + 导出（前端消费）
└── src/index.ts                           # export * from './stream-events'
```

### 1.2 三方各自对同一 fixture 做 parity 测试

| 语言 | 测试 | 断言要点 |
|---|---|---|
| Python | `apps/ai-service/tests/unit/test_stream_event.py:36` `test_stream_event_fixture_parity` | `StreamEvent.model_validate(item)` 逐条解析 fixture；断言 `events[1].channel=="job"`、`ids.job_id=="job-1"`、`events[2].ids.interaction_id=="interaction-1"` |
| Go | `apps/api/internal/dto/stream_event_test.go` `TestStreamEventFixtureParity` | `json.Unmarshal` 同一 fixture；断言 3 条、`Channel=="job"`、`IDs.JobID=="job-1"`；另有 `TestNewStreamEventIncludesJobID` 验证构造→序列化→反序列化 round-trip |
| TS | `packages/contracts/src/stream-events.ts`（类型层） | 前端按同一类型消费；编译期保证字段名/可空性一致 |

### 1.3 为什么这套是"对的"

1. **fixture 即"黄金样本"**：不是各写各的 mock，而是**同一份字节**喂给三端解析器——真正验证"我发的你能收"。
2. **schema 显式封闭**：`additionalProperties:false` + `ids` 封闭对象，意味着**新增字段是破坏性变更**，会被 schema 校验挡下，逼你走版本化（`version: const 1`）。
3. **round-trip 测试**：Go 侧 `NewStreamEvent → Marshal → Unmarshal` 证明构造器与线格式自洽（如 `job_id` 不会因 `omitempty` 丢失）。
4. **snake_case 跨语言统一**：fixture 用 `conversation_id`/`job_id`，Go tag、Pydantic alias、TS 字段三处对齐。

---

## 2. 现存不足与风险

- 🟡 **C-1 fixture 覆盖窄**：只有 3 条（message.text.delta / job.progress / state.interaction.required）。而实际事件类型远多于此（run.started/interrupted/resumed、tool.*、source/citation、safety、usage、stream.done...）。**很多真实事件没进 parity 网**。
- 🟡 **C-2 schema 未在 CI 强制校验 fixture**：Go/Python 各自读 fixture 做断言，但**没有一步用 JSON Schema 去校验 fixture 本身合法**（schema 与 fixture 可能各自漂移）。TS 侧仅编译期，无运行时 schema 校验。
- 🟡 **C-3 无"schema ↔ 类型"生成/校验闭环**：Go struct、Pydantic model、TS interface 三处**手写**，与 schema 之间靠人肉保持一致，缺"从 schema 生成"或"用 schema 校验类型输出"的自动闭环。
- 🟢 **C-4 无版本演进演练**：`version: const 1` 已就位，但没有 v1→v2 的迁移/双读示例，讲不了"契约如何安全演进"。
- 🟢 **C-5 CI 未显式聚合三方契约门禁**：三方测试散落在各自 test 套件里，缺一个"contract"聚合门禁与失败可读性。

---

## 3. 增强方案（把"3 条 fixture"升级为"全事件契约门禁"）

### Phase A：schema 校验 fixture（0.5 天，堵住 C-2）

**目标**：让 fixture **首先**必须通过 JSON Schema 校验，再谈三方解析。

1. Python：新增 `test_fixture_matches_schema`——用 `jsonschema` 加载 `stream-event.v1.schema.json`，对 fixture 每条 `validate()`。
2. Go：新增 `TestFixtureMatchesSchema`——用 `santhosh-tekuri/jsonschema` 校验同一 fixture。
3. **验收**：故意往 fixture 加一个 schema 未定义字段 → 两端测试同时红。

### Phase B：扩展 fixture 至"全事件覆盖"（1–1.5 天，堵住 C-1）

**目标**：让**每一类对外事件**都有一条黄金样本进 parity 网。

1. 枚举现有所有 `channel/type`（从 `runtime.go` 的 `sendNewEvent(... "run.started" ...)` 等调用点归纳）：`run.started/interrupted/resumed/failed`、`message.created/persisted/text.delta`、`tool.call/result`、`source/citation`、`safety.red_flag`、`usage`、`state.interaction.required/answered`、`state.health_features.upsert`、`stream.done/error`、`job.progress/completed`。
2. 每类补一条 fixture（覆盖各自 payload 形状与用到的 ids 字段）。
3. 三方 parity 测试改为**遍历 fixture 全量**逐条解析，而非硬编码索引断言。
4. **治理**：加一个"fixture 覆盖率"检查——列出代码里出现过的 event type 集合 vs fixture 覆盖集合，缺失即失败（防止新增事件类型忘了加 fixture）。
5. **验收**：新增一个 event type 但漏加 fixture → CI 红并指名缺哪个 type。

### Phase C：schema ↔ 类型闭环（1–2 天，堵住 C-3，最出彩）

选其一（按投入产出）：

- **C-方案1（校验输出，推荐先做）**：Go/Python 在**构造事件后**用 schema 校验其序列化结果（至少在测试/dev 断言）。即"我发的每条事件都 schema-valid"，直接消灭"发了 schema 外字段"。
- **C-方案2（生成类型）**：以 `stream-event.v1.schema.json` 为源，用 `quicktype`/`datamodel-code-generator` 生成 TS/Python 类型，CI 校验"生成物与手写类型无 diff"。把手写三处收敛为"一处 schema + 生成"。

**验收**：改 schema 后不同步类型 → CI 红（生成 diff 或运行时校验失败）。

### Phase D：契约版本演进演练（0.5 天，堵住 C-4/C-5）

1. 写一份 `docs/adr/xxxx-stream-event-versioning.md`：v1→v2 的"加字段=升版本 + 双读兼容"策略，配一个 v2 fixture 与"v1 客户端读 v2 事件"的容错测试（未知字段忽略、未知 type 跳过）。
2. CI 加一个 `contract` 聚合 target（Nx）：一处跑齐三方契约测试 + schema 校验 + 覆盖率检查，失败信息可读。
3. **验收**：`nx run contracts:verify`（或等价）一条命令绿灯 = 三端契约一致。

---

## 4. 变更契约时的标准流程（团队规范，务必遵守）

> 任何改动"对外事件结构"的 PR（如 T0-1 Phase B 的多字段中断、T0-2 Phase C 的最终文本事件）都必须走这套：

```text
1. 改 packages/contracts/schemas/stream-event.v1.schema.json（或升 v2）
2. 补/改 packages/contracts/fixtures/stream-events.v1.json（加黄金样本）
3. 同步三处类型：Go dto、Pydantic model、TS interface
4. 跑三方 parity + schema 校验 + 覆盖率：全绿方可合并
5. 破坏性变更 → 升 version 并提供双读兼容
```

这条流程本身就是**面试可讲的工程纪律**。

---

## 5. 面试叙事

> "多语言微服务最怕契约漂移。我把 SSE 事件的结构固化成**一份 JSON Schema + 一份 fixture**作为单一真值，Go 和 Python **各自加载同一份 fixture** 做一致性测试，还有构造→序列化→反序列化的 round-trip。我进一步把它做成**门禁**：fixture 先过 schema 校验、**每类事件都要有黄金样本**（漏加即 CI 红）、schema 改了类型不同步也 CI 红。契约的**版本演进**我用 `version` 常量 + 双读兼容来保证平滑升级。"

命中考点：契约测试、单一真值、JSON Schema、跨语言一致性、CI 门禁、向后兼容演进。

---

## 6. 落地任务清单

```text
A1 test(ai/api): fixture 必须先通过 JSON Schema 校验
B1 feat(contracts): 扩 fixture 至全事件类型覆盖
B2 test(ai/api): parity 测试遍历全量 fixture
B3 test(contracts): event-type 覆盖率检查（代码出现的 type ⊆ fixture 覆盖）
C1 test(api/ai): 构造事件后 schema 校验其序列化输出
C2 build(contracts): 从 schema 生成 TS/Python 类型 + no-diff 校验（可选）
D1 docs(adr): stream-event 版本演进策略 + v2 双读兼容测试
D2 ci: 新增 contract 聚合 target（Nx affected 友好）
```

## 7. 风险与回滚

- A/B/C1 均为**加测试**，对运行时零侵入，回滚=删测试。
- C2（生成类型）改动构建流程，风险略高，作为可选项；先做 C1（校验输出）已能拿下大部分价值。
- 扩 fixture 后若某历史事件形状与 schema 冲突，说明**已存在契约漂移**——这正是该测试要暴露的问题，应修代码而非放宽 schema。
