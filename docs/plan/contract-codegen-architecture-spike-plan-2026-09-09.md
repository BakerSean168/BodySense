# BodySense Contract Codegen Architecture Spike Plan

> 文档状态：SPIKE COMPLETE / 推荐已形成 / 未开始生产迁移
> 创建日期：2026-09-09
> 目标：在不改动生产行为的前提下，用 BodySense 的真实契约做可重复实验，比较 OpenAPI + Orval、OpenAPI + Hey API、Protobuf + Buf/Connect/Protovalidate，以及当前 JSON/JSON Schema 路线，确定各协议边界最适合的单一真值（Source of Truth）与 codegen 方案。
> 实施结果：已在独立 worktree 中完成依赖安装、codegen、mutation、runtime/bundle/wire/Chromium benchmark 与评分；生产代码、生产 transport 和 canonical worktree 均未修改。

---

## 0. Executive Summary

这次不应该把问题简化成“Orval vs Hey API vs Protobuf 谁最好”。真正需要拆成两层：

1. **Contract architecture**：谁是协议的唯一真值，如何做跨语言生成、运行时校验、breaking-change gate。
2. **Client/codegen tooling**：在同一份 contract 下，哪一个工具生成的前端 API 层最适合 BodySense。

本 Spike 计划比较四条路线：

| 编号 | 路线 | Canonical source | TS/Web | Go | Python | Runtime validation |
|---|---|---|---|---|---|---|
| B0 | 当前基线 | 多份手写定义 + JSON Schema/fixtures | 手写 TS / parser | 手写 DTO | Pydantic | 手写 / Pydantic |
| O1 | OpenAPI + Orval | OpenAPI 3.1 | Orval: TS + Zod + Fetch/TanStack | oapi-codegen | datamodel-code-generator / OpenAPI Generator | Zod + Pydantic/Go boundary |
| O2 | OpenAPI + Hey API | OpenAPI 3.1 | Hey API: SDK + Zod + TanStack | oapi-codegen | datamodel-code-generator / OpenAPI Generator | Zod + Pydantic/Go boundary |
| P1 | Protobuf + Buf | `.proto` | Protobuf-ES / Connect-Web | protobuf-go / connect-go | protobuf Python / gRPC or Connect where applicable | Protovalidate |
| J1 | JSON Schema-first control | JSON Schema 2020-12 | generated/compiled validator + TS | schema conformance | Pydantic/generated model | JSON Schema validator |

**预期不是选出一个全项目通吃的赢家。** 更合理的最终结果很可能是：

- Browser-facing REST：OpenAPI-first；
- Go ↔ Python 内部服务 RPC：Proto-first 值得重点评估；
- Public SSE / JSON artifact：JSON Schema 可能仍比强行 Proto 化更自然；
- 前端 OpenAPI generator：Orval 与 Hey API 二选一。

此外，本计划明确把以下三条作为“待验证假设”，而不是先验结论：

- codegen **大概率降低 contract drift 和人工重复维护**；
- codegen **可能降低 AI token 消耗，但只有在 generated code 被排除出 Agent 常规上下文时才成立**；若 Agent 大量读取生成文件，token 反而可能上升；
- Orval/Hey 本身**不会天然提升运行时性能**；Zod runtime validation 通常会增加少量 CPU 开销。真正可能明显改变 wire size / serialization 成本的是 Protobuf transport，但它同时带来迁移成本。

> **Spike result (2026-09-09):** H1-H6 已完成验证。最终结果不是单一 winner，而是按边界采用 OpenAPI / JSON Schema / Proto 的混合策略。详细证据见 `docs/architecture/contract-codegen-spike-results-2026-09-09.md`，决策见 `docs/adr/0014-adopt-boundary-specific-contract-codegen-strategy.md`。

---

## 1. 为什么现在值得做这个 Spike

### 1.1 BodySense 已经进入“contract duplication 成本开始显性化”的阶段

当前仓库里同一个协议事实已经存在多种表达。

以 `StreamEvent` 为例：

- `packages/contracts/src/stream-events.ts`：334 LOC，TS 静态类型；
- `packages/contracts/src/stream-event-parser.ts`：194 LOC，TS runtime validator；
- `packages/contracts/schemas/stream-event.v1.schema.json`：1037 LOC，JSON Schema；
- `apps/api/internal/dto/stream_event.go`：50 LOC，Go DTO；
- `apps/ai-service/src/models/stream_event.py`：93 LOC，Pydantic model；
- shared fixtures + Go/Python parity tests 再做跨语言兜底。

这套体系并不是“不工程化”，相反它已经有很强的 parity discipline；历史 `t0-cross-language-contract-testing-plan.md` 也明确记录过真实 drift 被 fixture 覆盖不足放过的问题。现在的核心问题是：

> 能否把“靠多份定义 + parity tests 保持一致”进一步收敛为“一个 canonical contract + 自动生成 + conformance/breaking gate”？

### 1.2 `HealthWorkspace` 是第二个典型重复点

当前：

- Go：`apps/api/internal/dto/health_workspace.go`；
- TS：`apps/web/src/features/workspace/types/workspace.ts`；
- Web API：`apps/web/src/features/workspace/api/workspaceApi.ts` 使用 `request<HealthWorkspace>()`；
- `request<T>` / `expectJson<T>` 的泛型只提供编译期视图，不等于 runtime validation。

这正适合验证 OpenAPI -> generated client -> Zod runtime parse 是否能把重复层收敛掉。

### 1.3 Go ↔ Python 已经存在真实的手写跨语言内部协议

`apps/api/internal/service/ai_client.go` 手写：

- `StartConsultationTurnRequest`；
- `ResumeConsultationInterruptRequest`；
- NDJSON event parsing；
- `validateConsultationInternalEvent()`；

Python `apps/ai-service/src/api/routes/runtime.py` 又维护对应的 Pydantic request model 和 stream output。

这个边界比普通 REST 更适合测试 Proto/Buf 的跨语言收益。

---

## 2. 外部调研结论：哪些能力已经成熟

本计划只采用官方文档能明确确认的能力作为实验前提。

### 2.1 Orval

官方能力：

- OpenAPI -> Zod schemas；
- OpenAPI -> typed Fetch/Axios client；
- TanStack Query / SWR 等 client generation；
- Fetch client 支持 Zod response `runtimeValidation`；
- Zod 4，并提供更 tree-shakeable 的 Mini variant 选项。

官方参考：

- https://orval.dev/docs/guides/zod/
- https://orval.dev/docs/guides/client-with-zod/
- https://orval.dev/docs/guides/react-query/
- https://orval.dev/docs/reference/configuration/output/

### 2.2 Hey API (`@hey-api/openapi-ts`)

官方能力：

- OpenAPI -> TypeScript types / typed SDK；
- Zod plugin；
- TanStack Query plugin；
- plugin-based output composition；
- 官方强调同一 spec 产生 deterministic generated output。

注意：官网当前同时展示 Python 示例/标识和“Python is next / coming soon”文案，因此 **本 Spike 不把 Hey API 作为 Python 生成链的前提**。Python leg 单独用成熟工具评测。

官方参考：

- https://heyapi.dev/

### 2.3 OpenAPI 的 Go / Python 生成链

为了公平比较 O1/O2，Orval/Hey 只负责前端 leg，跨语言架构还需要另外两端：

- Go：`oapi-codegen`，支持从 OpenAPI 生成 Go models、clients、以及包括 Gin 在内的 server boilerplate；
- Python：`datamodel-code-generator` 可从 OpenAPI / JSON Schema 生成 Pydantic v2 models；OpenAPI Generator 也有稳定 Python client generator。

参考：

- https://github.com/oapi-codegen/oapi-codegen
- https://github.com/koxudaxi/datamodel-code-generator
- https://openapi-generator.tech/docs/generators/python/

### 2.4 OpenAPI governance

单有 generator 不足以防 drift，还需要：

- schema lint：Redocly CLI；
- breaking diff：oasdiff；
- generated output no-diff gate。

参考：

- https://redocly.com/docs/cli/commands/lint
- https://www.oasdiff.com/docs/breaking-changes

### 2.5 Protobuf + Buf

Buf 官方支持：

- `buf generate`：统一运行多语言 codegen plugins；
- `buf breaking`：与 Git/BSR/其他 baseline 比较兼容性；
- breaking rule 可按 FILE / PACKAGE / WIRE_JSON / WIRE 选择；
- Buf + protobuf plugins 可生成 Go、TypeScript、Python 等代码。

参考：

- https://buf.build/docs/generate/usage/
- https://buf.build/docs/breaking/

### 2.6 Connect / Connect-Query

Connect 以 `.proto` 为 API 定义，可生成 Go、TS、Python client/server，浏览器可使用 Connect-Web；Connect-Query 可与 TanStack Query 集成。

重要限制：Connect-Query 当前主要面向 unary RPC；streaming 应使用更底层 Connect transport / Connect-Web 能力，不能拿 Connect-Query 直接替换 BodySense 的 SSE/NDJSON streaming 后就宣称“等价”。

参考：

- https://connectrpc.com/
- https://connectrpc.com/docs/query/getting-started/

### 2.7 Protovalidate

Proto 静态 schema 本身不等于所有业务 validation。Protovalidate 提供 schema annotations + runtime validation，当前支持 Go、Python、TypeScript/JavaScript 等语言。

参考：

- https://github.com/bufbuild/protovalidate
- https://protovalidate.com/

---

## 3. Spike 的核心问题

完成实验后必须能够回答以下问题，而不是只给“感觉更优雅”的结论：

1. **Drift**：改一个 contract 时，哪条路线最早、最确定地抓到不兼容？
2. **Source of Truth**：是否真的只需人工维护一份 wire contract？还有哪些不可消除的 domain/view model 是合理重复？
3. **Runtime trust**：真实网络 JSON / message 在哪里从 `unknown/untrusted` 变成 trusted typed value？
4. **DX**：加字段、改 enum、新增 endpoint/event 时，人工要改多少处？
5. **AI efficiency**：使用同一 Agent 完成同类 contract change，输入/输出 token、tool calls、repair iterations 是否下降？
6. **Generated-code tax**：生成物的 LOC、bundle size、typecheck/build 时间是否反而上升？
7. **Runtime performance**：Zod/Protovalidate validation 开销是多少？Proto 是否在真实 BodySense payload 下有足够的 wire/CPU 收益？
8. **Migration fit**：为了收益需要侵入多少现有 Gin / React Query / FastAPI / SSE / NDJSON 架构？
9. **Debuggability**：浏览器 Network 面板、curl、日志、错误信息是否仍清晰？
10. **Evolution**：可选字段、nullable、enum、version、unknown field、stream event 新 variant 如何演进？

---

## 4. 统一实验样本：必须用 BodySense 真实边界

不使用 Petstore 作为最终评分样本。Petstore 只允许用于工具安装 sanity check。

### R1. REST Read：`GET /api/v1/health-workspace`

目的：测试大型嵌套 response、nullable/optional、时间、UUID、Raw JSON、数组、domain projection。

当前锚点：

- Go DTO：`apps/api/internal/dto/health_workspace.go`
- Web TS：`apps/web/src/features/workspace/types/workspace.ts`
- Web client：`apps/web/src/features/workspace/api/workspaceApi.ts`
- route：`apps/api/cmd/server/main.go`

为什么选它：这是最能体现“Go DTO -> 手写 TS DTO -> generic fetch”重复成本的真实接口。

### R2. REST Command：BodyState command / Outcome mutation

从当前 workspace API 中选一个同时包含：

- request body；
- path/query 参数；
- optimistic revision；
- 正常 response；
- 409/4xx error semantics；

的 mutation。实施 Spike 时先在 `addFact`、`recordOutcome`、`updateLifestyleCurrent` 中选形状最完整的一个。

目的：防止只测 read model，忽略 error / mutation / request validation。

### S1. Public stream：`StreamEvent`

锚点：

- `packages/contracts/src/stream-events.ts`
- `packages/contracts/src/stream-event-parser.ts`
- `packages/contracts/schemas/stream-event.v1.schema.json`
- `apps/api/internal/dto/stream_event.go`
- `apps/ai-service/src/models/stream_event.py`

目的：测试 discriminated union、closed envelope、runtime validation、forward/backward compatibility。

### I1. Internal Go -> Python runtime

锚点：

- `apps/api/internal/service/ai_client.go`
- `apps/ai-service/src/api/routes/runtime.py`
- `apps/ai-service/src/models/stream_event.py`

样本：

- `StartConsultationTurnRequest`；
- 一小段 NDJSON StreamEvent sequence。

目的：这里是 Proto-first 最有潜在价值的真实边界。

---

## 5. 公平性原则：不要把“schema”和“transport”混在一次比较里

Protobuf 容易因为顺手把 JSON/HTTP 换成 binary RPC 而得到性能优势，但这样无法知道收益来自：

- IDL/codegen；
- runtime validator；
- binary serialization；
- transport protocol；

哪一层。

因此 P1 必须拆成两个阶段：

### P1-A：Proto as IDL / codegen only

只验证：

- `.proto` 是否能作为单一真值；
- Go/TS/Python 生成质量；
- Buf breaking；
- Protovalidate；

不改 BodySense 现有生产 transport。

### P1-B：Transport experiment（独立评分）

只在 isolated benchmark harness 里比较：

- JSON/HTTP vs Protobuf binary；
- NDJSON stream vs Connect/gRPC-style server streaming；

禁止直接把 transport benchmark 结果算到 codegen/DX 分数里。

---

## 6. OpenAPI Source-of-Truth 也要单独验证：Spec-first vs Go-first

用户日常企业开发经常看到“后端写 Go -> Swagger 文档 -> 前端照文档手写”，因此 Spike 不能只验证 generator，还要验证 contract ownership。

### O-SF：Spec-first（主候选）

```text
contracts/openapi/bodysense-spike.yaml
          |
          +--> oapi-codegen --> Go generated DTO/interface
          +--> Orval/Hey   --> TS/Zod/client/hooks
          +--> Python gen  --> Pydantic model/client
          +--> docs        --> API docs
```

人工只改 OpenAPI；generated files 禁止手改。

### O-GF：Go-first（对照组）

```text
Go handler / DTO / annotations
        |
        +--> generated OpenAPI
                 |
                 +--> TS/Zod/client
                 +--> Python models/client
```

需要回答：

- Go code/annotation 是否足够表达 required/nullable/enum/oneOf/error response？
- handler 实际返回值是否可能和 annotation 漂移？
- runtime/contract test 能否自动抓住？

**默认不因“少维护一个 YAML”就自动偏向 Go-first。** 如果 annotation 本身成为另一套重复描述，就不算真正降低维护成本。

---

## 7. 评分模型（100 分）

### 7.1 权重

| 维度 | 权重 | 为什么重要 |
|---|---:|---|
| Contract correctness & drift prevention | 25 | 核心目的，必须优先于代码漂亮 |
| Source-of-truth & evolution | 15 | 是否真的减少重复并可安全演进 |
| Runtime trust / validation | 10 | Type Erasure 后真实边界必须有证据 |
| Human DX / maintenance cost | 10 | 日常新增/变更成本 |
| AI efficiency / token cost | 10 | BodySense 高 AI 协作密度，必须实测 |
| Runtime / wire performance | 10 | 不预设 codegen 会更快 |
| Migration fit / architecture intrusion | 10 | 避免为 codegen 重写半个系统 |
| Build / CI / bundle cost | 5 | generator 和 validators 也有成本 |
| Debuggability / observability | 5 | curl、Network、错误定位、日志体验 |
| **Total** | **100** | |

### 7.2 每项使用 0-5 原始分，再乘权重

- 5：明显优于 baseline，且无重大新风险；
- 4：优于 baseline，有小成本；
- 3：收益/成本接近；
- 2：需要明显妥协；
- 1：关键场景较差；
- 0：不满足硬要求。

### 7.3 硬门槛（任何一项失败则不能成为默认路线）

- 能 deterministic regenerate；
- canonical schema 可以进入 Git review；
- 有 breaking-change strategy；
- generated code 可被明确标记并禁止手改；
- runtime boundary 有可验证方案；
- CI 能发现“schema 改了但 generated output 未同步”；
- 真实 BodySense 样本能表达，不依赖大量 `any` / `unknown` 逃生口；
- 不降低现有 safety/authority-relevant payload 的验证强度。

---

## 8. Contract drift 测试矩阵

每条路线必须在同一组 mutation 下测试“在哪一阶段报错”。

记录列：

```text
mutation
-> schema lint
-> breaking gate
-> codegen
-> compile/typecheck
-> runtime validator
-> integration/E2E
-> 是否静默通过
```

### M1. required field rename

例如 `generated_at` -> `generated_time`。

期望：breaking gate 在 merge 前失败，或者 generated consumers 编译失败；不能只靠浏览器运行后发现 `undefined`。

### M2. field type change

`priority: integer` -> `string`。

期望：schema breaking check + generated type compile/runtime validation 至少一层稳定失败。

### M3. enum value removal

例如删除一个已有 lifecycle/status value。

期望：breaking check 明确报告 consumer 兼容风险。

### M4. request optional -> required

这是典型 client-breaking change。

期望：CI gate 在生成客户端之前或同时抓到。

### M5. nullable -> non-null

验证 OpenAPI / Proto / generated language 对 nullable/presence 的真实语义，尤其 Go pointer、Python Optional、TS `null | undefined`。

### M6. add optional field

这是正常演进控制样本。

期望：不应出现大量无意义 breakage。

### M7. unknown response field

验证 closed-vs-open schema 策略：

- Zod object 默认/strict 行为；
- JSON Schema `additionalProperties`；
- Proto unknown fields；

不能只看“通过/不通过”，还要记录 forward compatibility 的设计取舍。

### M8. malformed nested payload

例如 `StreamEvent.payload.has_red_flags = "yes"`。

期望：runtime trust boundary 明确失败，并提供可定位的 path/message。

### M9. new StreamEvent variant

新增一个 event `type/channel/payload`。

检查：

- canonical schema 只改一次是否足够；
- Go/Python/TS 是否自动得到一致表示；
- exhaustive consumer 是否被编译器/CI 提醒；
- fixture/conformance 是否需要人工补充（合理）。

### M10. Proto-specific wire mutation

仅 P1：

- same field number change wire type；
- reuse removed field number；
- rename field；
- add field；

由 `buf breaking` 按 FILE/PACKAGE/WIRE_JSON/WIRE 分别记录结果。

---

## 9. AI Token / Agent Efficiency：必须实测，不能靠 LOC 猜

### 9.1 为什么 codegen 可能降低 token

当前 Agent 处理 contract change 时可能需要读取/修改：

```text
Go DTO
TS type
TS parser
JSON Schema
Python Pydantic
fixtures
parity tests
API client
consumer
```

单一 schema 后，理论上人工/Agent 主要处理：

```text
canonical schema
+ small handwritten adapter/domain model
+ tests
```

但 generated code 往往很长。如果 Agent 索引/打开全部 generated files，token 可能比 baseline 更高。

### 9.2 两套 token 指标

#### A. 真实 Agent telemetry（主指标）

在相同模型、相同 prompt、相同初始 commit 下，做 3 个 task，每个 candidate 至少重复 3 次：

- T1：新增一个 optional response field 并在 UI 消费；
- T2：增加一个 enum variant 并正确处理；
- T3：新增一个 request field + runtime rule。

记录：

- input tokens；
- cached input tokens（若 provider 暴露）；
- output tokens；
- tool calls；
- files opened/read；
- files modified；
- first-green 前 repair iterations；
- wall-clock time；
- first attempt CI pass rate。

同一 task 的 prompt 不允许针对工具特别提示答案，否则对比失真。

#### B. Context-surface proxy（辅指标）

若 Agent telemetry 不完整，则统计：

- 为完成任务必须人工理解的 source LOC / token；
- canonical contract token；
- handwritten adapter token；
- generated token（分别统计“全部”和“按规范排除后”）；
- Git diff 中 handwritten vs generated token。

### 9.3 强制做一个“generated context exclusion”对照

每个 codegen candidate 至少测：

1. Agent 可以无差别读取 generated dir；
2. Agent 默认忽略 generated dir，只有 debug 时按需进入。

只有第二种明显降低 token 时，结论应写成：

> codegen + generated-context hygiene 降低 Agent token；不是 codegen 单独降低 token。

### 9.4 Generated code policy（若未来采用）

正式实施时应考虑：

- `generated/` 目录统一；
- 文件头 `DO NOT EDIT`；
- formatter/linter 对 generated code 单独策略；
- code review 默认折叠 generated diff，只 review schema + generator version + semantic diff；
- Agent/ForgeFlow 搜索与上下文优先排除 generated files；
- debugging 任务允许显式读取 generated implementation。

---

## 10. Runtime / Wire 性能评测

### 10.1 先建立正确预期

- Orval / Hey API 是 codegen；**不应该预设比手写 fetch 更快**。
- Zod runtime validation 会增加 CPU 工作，但可以换取 fail-fast contract safety。
- Protobuf binary 可能减少 payload size / serialization 成本，但真实收益取决于 BodySense payload、gzip、浏览器与 stream 形态。
- 任何性能结论必须基于相同 payload semantic content。

### 10.2 Web bundle

对 B0 / O1 / O2 / P1-browser 分别测：

- `vite build` 总产物；
- generated client chunk raw / gzip / brotli；
- Zod runtime / generated schema 增量；
- tree-shaking 后实际 route chunk 增量；
- cold build + warm build；
- TypeScript typecheck 时间。

特别测试 Orval Zod normal vs Mini variant。

### 10.3 Runtime validation microbenchmark

使用相同 `HealthWorkspace` fixture：

- 1 KB / 10 KB / 100 KB 近似 payload；
- valid；
- missing field；
- wrong nested type；

测：

- current custom validator（适用处）；
- generated Zod (Orval)；
- generated Zod (Hey)；
- JSON Schema validator（J1）；
- Protovalidate（对应 Proto message）。

指标：ops/s、p50/p95 per parse、heap/allocation proxy；Node 和真实 Chromium 各跑一次，避免只有 Node 微基准。

### 10.4 Stream throughput

使用现有 `stream-events.v1.json` 扩展/复制形成：

- 100 events；
- 1,000 events；
- 10,000 events；

比较：

- JSON parse + current parser；
- JSON parse + generated Zod；
- JSON Schema validator；
- Proto deserialize + Protovalidate。

指标：

- total processing time；
- per-event latency；
- memory peak；
- invalid-event fail-fast latency。

### 10.5 Wire size

对同一 semantic payload 测：

- JSON raw；
- JSON gzip；
- Protobuf binary raw；
- Protobuf + transport compression（若候选实际支持/启用）；

不要只展示“Proto 比裸 JSON 小”这种没有生产意义的数字。

### 10.6 End-to-end latency

只在 P1-B transport experiment 做：

- loopback unary：1000 calls；
- server streaming：固定事件数；

测 p50/p95/p99、CPU、bytes transferred。

不把 localhost 几百微秒的差异夸大成生产收益。

---

## 11. Human DX / Maintenance 评测

对每条路线完成相同的四个“维护动作”：

1. add optional field；
2. add enum value；
3. add endpoint/RPC；
4. make one intentionally breaking change then fix migration。

记录：

- handwritten files touched；
- handwritten LOC；
- generated LOC（单列，不和 handwritten 混在一起）；
- commands required；
- config complexity；
- error message clarity；
- editor autocomplete quality；
- generated name/readability；
- 是否需要 custom template/mutator；
- domain adapter 是否清晰；
- 新人从 contract 找到 consumer 的导航成本。

### 11.1 一个关键指标：Manual Definition Count

目标不是“总文件最少”，而是：

> 同一个 wire fact 有多少份需要人亲自维护的定义？

例如 `WorkspaceAction.priority` 应尽量从：

```text
Go DTO + TS interface + doc + validator
```

收敛为：

```text
canonical schema (1 manual definition)
-> generated language artifacts
```

### 11.2 不要错误消灭 domain model

以下重复是合理的，不能计为 codegen 失败：

```text
Generated API DTO
        -> adapter
Frontend Domain Model / ViewModel
```

wire contract 和 UI/domain model 是不同职责。

---

## 12. Candidate O1：OpenAPI + Orval 详细实验

### O1-0. Tool sanity

isolated temp dir 安装 pin 版本，记录：

- Node/pnpm；
- Orval；
- Zod；
- oapi-codegen；
- Python generator；
- Redocly；
- oasdiff。

禁止 `latest` 留在最终配置。

### O1-1. 手工构造最小 OpenAPI 3.1 spike spec

只覆盖 R1 + R2，不覆盖整个 BodySense。

要求 schema 精确表达：

- required / optional；
- nullable；
- enum；
- UUID；
- date-time；
- `additionalProperties`；
- 200 + 4xx error；
- nested arrays/objects。

### O1-2. Orval frontend outputs

至少测三种输出：

A. Zod only；
B. Fetch + generated types；
C. Fetch/TanStack + Zod runtime validation。

检查：

- generated model 可读性；
- response parse 是否真的执行；
- custom mutator 是否会绕过 runtimeValidation；
- TanStack query key / hook API 是否适合当前 BodySense query layer；
- Zod Mini 对 bundle 的影响。

### O1-3. Go leg

用 oapi-codegen 生成：

- models；
- Gin server interface（或 strict interface，按当前架构最小侵入方案）；

只编译 isolated spike，不接管现有 router。

检查：

- `time.Time` / UUID / nullable 映射；
- `map[string]any` / Raw JSON 表达；
- model reuse；
- 是否需要大量 custom type mapping。

### O1-4. Python leg

从同一 OpenAPI 生成 Pydantic v2 model，比较当前 Python Pydantic 风格。

### O1-5. Governance

- Redocly lint；
- oasdiff against baseline；
- regenerate + `git diff --exit-code`；
- malformed runtime fixture；
- mutation matrix M1-M9。

---

## 13. Candidate O2：OpenAPI + Hey API 详细实验

完全复用 O1 的同一 OpenAPI spec、同一 fixtures、同一 mutations。

只替换 frontend generator；Go/Python legs 保持相同，确保比较的是 Orval vs Hey API，而不是整个架构一起变化。

### O2-1. 生成组合

至少：

- `@hey-api/sdk`；
- Zod plugin；
- `@tanstack/react-query` plugin；

检查 generated output 是否天然拆分得适合 BodySense Nx workspace。

### O2-2. Runtime validation 行为

不要因“生成了 Zod”就认为 response 已自动验证。

明确追踪 generated SDK call -> response -> schema parse 的实际执行路径；若需要 wrapper/manual `.parse()`，记录为 DX/ownership 成本。

### O2-3. Generated output quality

和 Orval 逐项比较：

- public API surface；
- naming；
- tree-shaking；
- hook/query options；
- error model；
- custom fetch/auth interceptor；
- file count/LOC；
- diff stability；
- config complexity。

### O2-4. 同样运行 M1-M9、AI tasks、bundle/runtime benchmarks

这样最后可以给出真正的 Orval vs Hey API scorecard。

---

## 14. Candidate P1：Protobuf + Buf + Protovalidate + Connect 详细实验

### P1-0. Scope

不把整个 REST API Proto 化。

优先 I1（Go ↔ Python runtime），再用 R1 的一个最小 projection 验证 browser codegen 体验。

### P1-1. `.proto` canonical contract

为 spike 定义：

- `StartConsultationTurnRequest`；
- `StreamEvent` envelope + 3-5 个代表性 variants；
- 一个 unary `GetHealthWorkspaceSubset` 仅用于 browser/client DX 对比。

### P1-2. Buf generate

生成：

- Go；
- TypeScript/Protobuf-ES；
- Python；

要求所有 generated dirs deterministic。

### P1-3. Buf breaking

运行：

- FILE；
- PACKAGE；
- WIRE_JSON；
- WIRE；

用 M10 明确记录不同 rule set 对 BodySense 的实际含义。

### P1-4. Protovalidate

至少表达：

- string min length；
- integer range；
- enum/presence；
- 一个跨字段 rule（若当前 contract 真实需要）。

验证 Go/Python/TS 对同一 invalid fixture 的 conformance。

### P1-5. Connect browser DX

只做 unary spike：

- Connect-Web；
- Connect-Query / TanStack Query；

比较当前 React Query 写法。

Streaming 不使用 Connect-Query 作为实验工具；另开 P1-B transport benchmark。

### P1-6. Python transport

若测试 Connect Python / gRPC Python，只在 isolated harness 中使用，不替换现有 FastAPI runtime。

### P1-7. Migration-cost accounting

必须把以下工作算入成本：

- `.proto` package/version convention；
- Buf config；
- generated package wiring；
- gateway/Connect handlers；
- JSON compatibility；
- existing REST clients；
- streaming semantic changes；
- observability/curl/debug workflow。

不能只比较最终 generated model 有多漂亮。

---

## 15. Candidate J1：JSON Schema-first Control

为什么保留这个候选：BodySense public `StreamEvent` 已经有一份 JSON Schema 2020-12。如果只因为 Proto 的 codegen 体验更漂亮就改 transport，可能是在解决错误的问题。

J1 只针对 S1，测试：

```text
stream-event.v1.schema.json (canonical)
  -> TS static types / validator
  -> Python Pydantic/generated model
  -> Go conformance/generated model (若工具质量可接受)
```

重点回答：

- 能否删除/减少 194 LOC 手写 TS parser；
- 能否减少 334 LOC TS contract 手写部分；
- 是否可以保留 SSE/JSON debug 体验；
- codegen quality 是否足以避免“生成代码比手写更难维护”。

如果 J1 在 StreamEvent 上胜出，不影响 REST 选择 OpenAPI、内部 RPC 选择 Proto。

---

## 16. CI Gate 设计（所有 winner 必须有）

未来若采用，目标 pipeline：

```text
Canonical contract changed
        |
        v
Schema lint
        |
        v
Breaking-change check vs main
        |
        v
Code generation
        |
        v
Generated no-diff / compile
        |
        v
Cross-language conformance fixtures
        |
        v
Runtime invalid-fixture tests
        |
        v
Integration / E2E
```

### OpenAPI candidate

建议门禁：

```text
redocly lint
-> oasdiff breaking
-> contracts:generate
-> git diff --exit-code
-> Go/Python/TS typecheck/test
-> runtime invalid-fixture suite
```

### Proto candidate

建议门禁：

```text
buf lint
-> buf breaking --against main
-> buf generate
-> git diff --exit-code
-> Go/Python/TS compile/test
-> Protovalidate conformance
```

### Generated artifact policy

Spike 必须同时评估两种模式：

- generated files committed；
- generated files build-time only。

评分重点：

- clone 后可复现性；
- CI 时间；
- PR review noise；
- AI context noise；
- debugging convenience。

---

## 17. 决策时需要特别防止的错误结论

### 错误 1：“generated LOC 少/多 = 好/坏”

Generated LOC 不是主要维护成本；应该看 **handwritten contract LOC + change fanout + Agent context**。

### 错误 2：“Zod 有 runtime check，所以一定更优”

需要测实际 validation CPU、bundle、错误策略以及是否所有 client path 都执行 parse。

### 错误 3：“Proto payload 更小，所以整个系统应该 Proto 化”

需要把 transport migration、浏览器 debug、SSE semantics、gateway、FastAPI integration 全算进去。

### 错误 4：“一个 repo 只能有一个 IDL”

正确目标是：**每个 wire boundary 只有一个 canonical contract**。REST、stream、internal RPC 可以使用不同但各自唯一的 IDL。

### 错误 5：“前后端所有模型都必须 generated”

Generated wire DTO 和 handwritten domain/view model 可以同时存在；adapter 是合理边界。

### 错误 6：“AI 一定更省 token”

Generated code 如果被 Agent 默认索引可能增加 token。必须用真实 telemetry 证明。

### 错误 7：“性能提升”把 build-time / runtime / wire 混为一谈

本计划分别记录：

- codegen/build speed；
- runtime validation speed；
- frontend bundle；
- network wire size；
- E2E latency。

---

## 18. 实施阶段拆分（未来执行，不在本次进行）

### Phase 0 — Baseline Capture（0.5 天）

产物：

- selected contract inventory；
- exact toolchain versions；
- current tests/build timings；
- current manual definition count；
- current bundle baseline；
- fixtures snapshot；
- baseline AI task traces。

### Phase 1 — OpenAPI canonical fixture（0.5-1 天）

产物：

- isolated OpenAPI 3.1 spec covering R1/R2；
- lint + oasdiff baseline；
- no production wiring。

### Phase 2 — Orval spike（0.5-1 天）

产物：

- generated Zod；
- generated Fetch/TanStack variant；
- runtime validation evidence；
- mutation/bundle/DX metrics。

### Phase 3 — Hey API spike（0.5-1 天）

同 O1 的全部指标。

### Phase 4 — OpenAPI multi-language leg（0.5-1 天）

产物：

- oapi-codegen Go model/interface；
- generated Python Pydantic model；
- cross-language fixture conformance；
- spec-first vs Go-first comparison。

### Phase 5 — Proto/Buf codegen spike（1-1.5 天）

产物：

- `.proto` sample；
- Go/TS/Python generated artifacts；
- Buf lint/breaking report；
- Protovalidate conformance；
- Connect unary browser experiment。

### Phase 6 — Stream/JSON Schema control（0.5-1 天）

产物：

- generated/schema-driven StreamEvent alternative；
- current parser parity；
- streaming validation benchmark。

### Phase 7 — AI + performance benchmark（1-2 天）

产物：

- token telemetry table；
- generated-context-on/off comparison；
- bundle/build table；
- runtime parse/validate table；
- wire/transport table。

### Phase 8 — ADR / recommendation（0.5 天）

最终不直接 merge migration，而是先出：

```text
docs/adr/<date>-contract-source-of-truth.md
```

内容必须回答：

- REST winner；
- Stream winner；
- Internal RPC winner；
- frontend generator winner；
- generated-file policy；
- CI breaking policy；
- migration sequencing；
- 哪些区域保持现状。

---

## 19. 实验目录与隔离策略

正式执行 Spike 时优先使用独立 worktree/临时实验目录，避免污染生产依赖：

```text
experiments/contract-codegen/   # 若决定将 benchmark harness 纳入 repo
```

或：

```text
/tmp/bodysense-contract-codegen-spike/
```

若需要真实 import/build 集成，使用专门 spike branch/worktree，不直接修改 main 工作树。

所有 generated output 在实验中明确隔离：

```text
<spike>/gen/orval/
<spike>/gen/heyapi/
<spike>/gen/openapi-go/
<spike>/gen/openapi-python/
<spike>/gen/proto-go/
<spike>/gen/proto-ts/
<spike>/gen/proto-python/
```

这样方便：

- `du`/LOC/token 对比；
- bundle 单独导入；
- 防止 candidate 互相污染；
- 一键删除回滚。

---

## 20. 数据记录模板

每个 candidate 最后填写同一张表：

| Metric | B0 | O1 Orval | O2 Hey API | P1 Proto | J1 JSON Schema |
|---|---:|---:|---:|---:|---:|
| manual wire definitions | | | | | |
| handwritten contract LOC | | | | | |
| generated LOC | | | | | |
| M1-M10 detected before runtime | | | | | |
| invalid fixture rejection | | | | | |
| codegen cold ms | | | | | |
| typecheck delta ms | | | | | |
| build delta ms | | | | | |
| web bundle gzip delta | | | | | |
| validation p50/p95 | | | | | |
| 1k stream events total ms | | | | | |
| payload raw/gzip bytes | | | | | |
| agent input tokens | | | | | |
| agent output tokens | | | | | |
| agent tool calls | | | | | |
| repair iterations | | | | | |
| files manually touched/change | | | | | |
| migration intrusion score | | | | | |
| weighted score /100 | | | | | |

同时附 qualitative notes，不允许只看总分。

---

## 21. 初步假设（只能用于指导实验，不能当结论）

### H1 — REST/frontend

Orval 很可能是 BodySense 当前 React + TanStack Query + runtime Zod 的高适配候选，尤其 Fetch `runtimeValidation` 能直接验证“generated client 是否真的建立 runtime trust boundary”。

### H2 — Hey API

Hey API 可能在生成代码的模块化、plugin composition 和 SDK API surface 上更优，需要用真实 BodySense OpenAPI 才能判断；不能只看 landing page demo。

### H3 — Internal Go/Python

Proto + Buf 的最大价值可能不是浏览器 REST，而是 `Go -> Python AI runtime`：三语言 generated types、breaking gate、Protovalidate 会直接减少当前手写 request/event contract 重复。

### H4 — Public StreamEvent

当前 JSON Schema + fixtures 已经很接近正确方向。最小风险改进可能是“把 JSON Schema 真正提升为 generated/static/runtime canonical source”，而不是改成 Proto transport。

### H5 — AI token

对 BodySense 这种 AI 高参与开发，**manual contract surface 的减少**应能降低 Agent reasoning/context 负担；但只有把 generated code 从默认上下文排除后才可能稳定转化为 token savings。

### H6 — Performance

OpenAPI codegen 的主要收益应是 correctness/DX，而不是 runtime speed；Proto 的性能收益只有在真实 wire benchmark 足够明显且能抵消迁移成本时才值得作为 transport 方案。

---

## 22. 最终 Recommendation 的判定规则

### 可以建议“采用”

满足：

- weighted score 比 B0 高 >= 10 分；
- drift prevention / runtime trust 无倒退；
- 至少 2 个真实 BodySense contract 样本成功；
- generated code deterministic；
- CI gate 清晰；
- 不需要大量 custom templates/patches；
- Agent token 或 human change fanout 至少一个出现可测量改善；
- migration 可分阶段，不要求 big-bang rewrite。

### 只建议“局部采用”

例如：

- Orval 只用于 Web REST；
- Proto 只用于 Go/Python internal runtime；
- JSON Schema 保留用于 StreamEvent。

这很可能比“一统全部协议”更优。

### 建议“保持现状”

若：

- generator 需要大量 workaround；
- runtime validation 变弱；
- generated bundle/CI/Agent noise 显著变差；
- BodySense 特殊 streaming contract 表达很差；
- migration cost 明显高于 drift/maintenance 收益。

保持当前 schema + fixture + parity tests 也是合法结果，不允许为了“现代工具”强行迁移。

---

## 23. Spike 完成后的下一步

本 Spike 已在独立 worktree 中完成，候选实验不直接合入 production。

本轮已经完成：

1. Phase 0 baseline 与统一 fixtures；
2. O1 / O2 同 spec 对照；
3. OpenAPI Go/Python spec-first 与 Go-first control；
4. P1 Proto/Buf/Protovalidate/Connect；
5. J1 StreamEvent JSON Schema-first；
6. Node + Chromium runtime、bundle、wire benchmark；
7. AI context-surface proxy；
8. weighted scorecard；
9. architecture result report + ADR。

下一步只有在用户明确批准生产迁移后，才创建新的 migration Active Plan，并按 REST -> StreamEvent -> Internal IDL -> selective transport 的顺序逐边界实施。**不得把本 Spike 的 candidate branch 直接当 production patch 合并。**

---

## 24. Definition of Done（Spike，而非迁移）

Spike 只有在以下全部完成时才算结束：

- [x] B0 baseline 被量化；
- [x] Orval 用 R1/R2 跑通并记录 runtime validation；
- [x] Hey API 用同一 R1/R2 跑通；
- [x] OpenAPI Go/Python generation 跑通；
- [x] Redocly + oasdiff mutation matrix 完成；
- [x] Proto/Buf 三语言 codegen 跑通；
- [x] Buf breaking matrix 完成；
- [x] Protovalidate cross-language invalid fixtures 完成；
- [x] Connect unary browser experiment 完成；
- [x] StreamEvent JSON Schema control 完成；
- [x] runtime/bundle/wire benchmark 完成；
- [x] AI token/context experiment 完成；
- [x] weighted scorecard 完成；
- [x] ADR 给出按边界的推荐，而不是强制单一技术栈；
- [x] 没有任何 production transport/behavior change 混进 Spike。

AI token/context 项使用计划 §9.2 允许的 context-surface proxy：Pixel Control Plane 在最终测量时未监听 `127.0.0.1:8320`，因此没有伪造真实 Agent input/output telemetry。
