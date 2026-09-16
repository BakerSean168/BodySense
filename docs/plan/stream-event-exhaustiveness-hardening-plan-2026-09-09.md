# BodySense StreamEvent Exhaustiveness Hardening Plan

> 文档状态：PLAN ONLY / 待实施
> 创建日期：2026-09-09
> 目标：把 Web 内部 `StreamEvent` reducer 从“已知事件也可能被 permissive default 静默吞掉”收敛为“网络边界负责 runtime unknown，内部 reducer 对已知 `StreamEvent` 做 compile-time exhaustive handling”。
> 明确约束：本阶段只记录问题、设计验收与实施步骤；不改生产代码、不改协议、不新增/删除事件类型。

---

## 0. Executive Summary

当前 BodySense 已经有一条正确的 runtime trust-boundary：

```text
SSE / JSON
  -> JSON.parse(): unknown
  -> parseStreamEvent(input: unknown)
  -> known StreamEvent
  -> reduceActiveTurnEvent(...)
```

`parseStreamEvent()` 会拒绝不在 `EVENT_SPECS` 中的 `event.type`，因此正常 Web SSE 链路中的“契约外未知事件”应该在 reducer 之前就被拦截。

但是 `reduceActiveTurnEvent(current, event: StreamEvent)` 当前仍保留：

```ts
default:
  break;
```

这会产生两个问题：

1. 给 `StreamEvent` union 新增一个**已知** variant 时，即使 reducer 作者忘记处理，TypeScript 也可能继续通过；
2. reducer 注释把 `default` 描述为“处理契约之外未知事件”，但当前正常入口已经由 `parseStreamEvent()` 拒绝 unknown type，职责出现重叠/错位。

推荐方向：

- runtime unknown / protocol compatibility：继续由 `parseStreamEvent()` 与版本策略负责；
- reducer 内部已知 `StreamEvent`：显式列出所有 handled / intentional no-op variants；
- terminal `default` 使用 `assertNever(event)`（或等价 `event satisfies never`）建立 compile-time exhaustiveness gate。

这不是修复已确认线上 bug，而是一个**类型安全与协议演进硬化项**。

---

## 1. 当前事实基线

### 1.1 Canonical TypeScript union

文件：

```text
packages/contracts/src/stream-events.ts
```

`StreamEvent` 是 discriminated union，每个成员都有 literal `type`。

当前 union / parser `EVENT_SPECS` 共覆盖 **33 个已知 public event type**。

### 1.2 Runtime parser 已经拒绝 unknown event type

文件：

```text
packages/contracts/src/stream-event-parser.ts
```

关键逻辑：

```ts
if (typeof input.type !== "string" || !(input.type in EVENT_SPECS)) {
  throw new StreamEventParseError(
    `unsupported public event type ${String(input.type)}`,
  );
}
```

因此正常网络路径的职责已经是：

```text
unknown network value
  -> parser runtime validation
  -> only known StreamEvent enters reducer
```

相关入口：

```text
apps/web/src/features/consultation/hooks/useSSEProcessor.ts
apps/web/src/features/consultation/services/consultationService.ts
```

例如 SSE 路径当前明确执行：

```ts
const data: unknown = JSON.parse(dataStr);
const event = parseStreamEvent(data);
```

### 1.3 Reducer 当前不是 exhaustive consumer

文件：

```text
apps/web/src/features/consultation/runtime/activeTurnReducer.ts
```

当前签名：

```ts
export function reduceActiveTurnEvent(
  current: ActiveTurnState,
  event: StreamEvent,
): ReduceResult
```

switch 尾部：

```ts
default:
  break;
```

然后统一：

```ts
return { state: next, effects };
```

因此这里不会像一个 `(): string` 且所有 case 都直接 return 的 toy function 一样，被 `noImplicitReturns` 间接保护。

### 1.4 当前 33 个已知事件中，Reducer 仅显式列出 24 个

当前存在 **9 个已知 `StreamEvent` variant** 没有显式 case，而是落入 `default`：

```text
job.created
job.progress
job.completed
job.failed
safety.output_reviewed
safety.output_rejected
source.answer_attribution.added
state.interaction.expired
usage.reported
```

这非常重要：实施不能简单把：

```ts
default:
  break;
```

机械替换为：

```ts
default:
  return assertNever(event);
```

否则当前代码会立即 typecheck 失败。

必须先对这 9 类已知事件逐一做**显式 disposition**：

```text
HANDLE
或
INTENTIONAL_NOOP
```

只有这样，terminal branch 才能真正 narrowed to `never`。

---

## 2. 要解决的核心问题

### 2.1 `noImplicitReturns` 不是 exhaustiveness

BodySense 根 tsconfig 当前启用：

```json
"noImplicitReturns": true
```

它回答的是：

```text
所有可达路径是否都有 return？
```

它不回答：

```text
Discriminated union 的所有 variant 是否都被处理？
```

对于当前 reducer：

```ts
switch (...) {
  ...
  default:
    break;
}

return { state: next, effects };
```

任何新 variant 都仍然有统一返回路径，因此 `noImplicitReturns` 无法提供 union evolution gate。

### 2.2 普通 `default` 会主动吞掉新 known variant

如果未来新增：

```ts
| SafetyEmergencyRequiredEvent
```

而 reducer 未增加 case，当前行为是：

```text
new known event
  -> default
  -> no state/effect change
  -> typecheck can still pass
```

这正是本计划要消除的 silent-loss 风险。

### 2.3 Runtime unknown 与 Compile-time known variant 必须分层

正确职责建议固定为：

```text
Network / protocol boundary
  unknown
  -> parseStreamEvent
  -> reject/version/compatibility policy

Internal domain reducer
  StreamEvent
  -> exhaustive switch
  -> every known variant consciously handled or ignored
```

不要让一个 permissive reducer `default` 同时承担两层职责。

---

## 3. 推荐目标形态

### 3.1 Shared exhaustive helper

候选实现：

```ts
function assertNever(value: never): never {
  throw new Error(`Unhandled StreamEvent: ${JSON.stringify(value)}`);
}
```

是否放为局部 helper 还是共享 utility，在实施阶段根据复用情况决定。

默认优先局部放在 reducer 附近，避免为了一个 helper 过早扩大公共 API。

### 3.2 所有已知 no-op variant 必须显式写出

概念目标：

```ts
switch (event.type) {
  case "conversation.created":
    // handle
    break;

  // ... handled variants ...

  case "job.created":
  case "job.progress":
  case "job.completed":
  case "job.failed":
  case "safety.output_reviewed":
  case "safety.output_rejected":
  case "source.answer_attribution.added":
  case "state.interaction.expired":
  case "usage.reported":
    // Explicitly intentional no-op in ActiveTurn reducer.
    // IMPORTANT: preserve current processed/sequence semantics unless
    // characterization tests prove another behavior is intended.
    break;

  default:
    return assertNever(event);
}
```

上面只是**目标结构示意**，不是本阶段生产修改。

### 3.3 不要顺手改变 `processed` 语义

当前 default/no-op 分支不设置：

```ts
processed = true;
```

因此这些事件当前不会更新 reducer 的 `lastSeq` / `sequenceRunId`。

实施 exhaustive hardening 时，必须先用测试记录现状，再决定：

```text
A. 显式 no-op 且 processed=false（纯行为保持）
B. 显式 no-op 但 processed=true（改变 seq consumption 语义）
```

本计划默认 **A：先保持行为**。

任何 B 类改变必须另立行为理由和 regression test，不能夹带在类型安全重构中。

---

## 4. `assertNever` 与 `satisfies never` 的选择

### 4.1 `assertNever(event)`

优点：

- compile-time exhaustive proof；
- 若某个内部调用绕过 parser 并真的把非法值送进 reducer，runtime fail-fast；
- 错误位置直观；
- 团队可读性较强。

缺点：

- 理论上多了一条 runtime throw path；
- error serialization 需要避免非常大的 payload（可只输出 `type`）。

### 4.2 `event satisfies never`

示例：

```ts
default: {
  event satisfies never;
  break;
}
```

优点：

- 纯 compile-time intent；
- 不增加 runtime error policy。

缺点：

- 如果非法值绕过 boundary，仍可能 silent no-op；
- 对不熟悉 TS `satisfies never` 的维护者可读性略低。

### 4.3 本计划默认推荐

优先：

```text
assertNever
```

理由：内部 reducer 的参数已经声明为 trusted `StreamEvent`；如果这个 invariant 被绕过，fail-fast 比静默忽略更容易发现架构违规。

最终实现前仍需做 direct-caller audit，确认没有任何调用方依赖“伪造 unknown event 后 reducer 静默忽略”。

---

## 5. 实施阶段计划

### Phase A — Characterize current behavior

1. 枚举 `StreamEvent["type"]` / `EVENT_SPECS` 全部 known variants；
2. 枚举 reducer switch 的全部 case；
3. 固化差集检查：当前基线为 33 known / 24 explicit / 9 implicit no-op；
4. 对 9 个 implicit no-op variant 写 characterization tests，确认：
   - state 是否完全不变；
   - effects 是否为空；
   - `processed=false` 对 `lastSeq` 的现有影响；
5. 审计所有 `reduceActiveTurnEvent()` caller，确认是否都传 trusted `StreamEvent`，以及测试中是否存在 intentionally forged values。

### Phase B — Make intentional no-op explicit

把 9 个 known variants 逐一分为：

```text
HANDLE
INTENTIONAL_NOOP
```

对 `INTENTIONAL_NOOP` 使用显式 case，不能继续依赖 `default`。

若审计发现其中某个事件其实应该影响 ActiveTurn，例如：

```text
state.interaction.expired
safety.output_rejected
```

必须另开行为修复项；本 hardening PR 不应把类型安全改造和业务语义变更混为一体。

### Phase C — Add exhaustiveness gate

新增局部 helper：

```ts
function assertNever(value: never): never {
  throw new Error(...);
}
```

switch terminal branch：

```ts
default:
  return assertNever(event);
```

或者如果 reducer 返回结构不适合直接 return helper，则使用等价结构，只要 terminal value 必须是 `never`。

### Phase D — Mutation verification

做一个**只用于验证、不提交 production variant**的 mutation experiment：

临时向 `StreamEvent` union 增加：

```ts
type TestOnlyFutureEvent = StreamEventBase<
  "state",
  "state.test_future",
  { value: string }
>;
```

预期：

```text
web typecheck FAIL
```

失败位置必须能追踪到 reducer exhaustive gate，而不是依赖 unrelated test / noImplicitReturns。

然后增加 explicit case，预期：

```text
web typecheck PASS
```

实验结束后回滚 test-only protocol mutation。

### Phase E — Preserve runtime boundary behavior

确保以下 parser tests 仍然成立：

```text
unknown event.type -> StreamEventParseError
wrong channel      -> StreamEventParseError
malformed payload  -> StreamEventParseError
```

也就是说：

```text
runtime unknown policy remains in parser
```

不能因为 reducer 变 strict，就删除/弱化 parser boundary。

---

## 6. 验收标准

### 6.1 Compile-time acceptance

必须证明：

```text
Add known StreamEvent variant
+ forget reducer disposition
=> typecheck fails at exhaustiveness gate
```

不是仅仅因为函数漏 return，也不是碰巧某个测试类型错了。

### 6.2 Runtime acceptance

必须证明：

```text
network unknown type
=> rejected before reducer
```

同时：

```text
all current valid events
=> existing reducer behavior preserved
```

### 6.3 Intentional no-op acceptance

所有已知但 ActiveTurn 不关心的事件都必须：

- 有显式 case；
- 有注释说明为什么属于该 reducer 的 no-op；
- 至少有一组 characterization evidence 防止未来误改；
- 不再依赖 catch-all `default` 表达“我不关心”。

### 6.4 Recommended verification commands

实施时至少运行：

```bash
pnpm nx run @bodysense/web:typecheck --skip-nx-cache
pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/consultation/runtime/activeTurnReducer.test.ts
pnpm exec vitest run packages/contracts/src/stream-event-parser.test.ts
pnpm curriculum:check
git diff --check
```

若 contract mutation experiment 修改 `packages/contracts`，还应运行对应 contracts typecheck/test target。

---

## 7. 风险与非目标

### 非目标 1：不在此 PR 改 protocol compatibility policy

本计划不决定：

```text
未来 v2 event
旧客户端应该 ignore / reject / downgrade / negotiate
```

这是协议版本治理问题，应在 contract codegen / versioning 方案中单独处理。

### 非目标 2：不在此 PR 把 SSE 改成 Proto/Connect

本 hardening 与 transport 无关。

### 非目标 3：不顺手修所有 `as` / parser / payload typing

这些属于 runtime schema/codegen 方案，已经记录在：

```text
docs/plan/contract-codegen-architecture-spike-plan-2026-09-09.md
```

### 风险 1：显式 no-op 暴露真实遗漏

9 个 implicit no-op 中可能存在“其实应该处理”的事件。

如果发现，说明当前 permissive default 正在隐藏真实业务缺口；应单独立项，而不是为了让 exhaustive check 变绿而盲目标成 no-op。

### 风险 2：测试绕过 parser

当前部分 reducer tests 使用 `as never` / `as StreamEvent` 构造测试值。

实施前必须区分：

```text
合法 fixture 的 test convenience cast
vs
故意依赖 unknown event permissive behavior
```

后者若存在，需要先明确它是否违背新边界设计。

---

## 8. 与 Contract Codegen Spike 的关系

两项工作解决不同问题：

```text
Contract Codegen Spike
= 如何减少 Go / TS / Python contract duplication 与 drift

Exhaustiveness Hardening
= 已经进入 trusted TS domain 后，新增 union variant 如何强制消费者显式处理
```

即使未来使用 OpenAPI / Proto / generated TS union，仍然需要 exhaustive consumer discipline。

Codegen 只能让：

```text
new variant 自动进入 generated union
```

真正让消费者“不能忘记处理”的仍然是：

```text
never / exhaustive switch gate
```

两者是互补关系。

---

## 9. 推荐实施顺序

```text
1. Characterize 9 implicit no-op variants
2. Audit reducer callers / parser ownership
3. Explicitly disposition all known variants
4. Add assertNever gate
5. Mutation-test new variant -> compile failure
6. Run focused reducer + parser tests
7. Write short ADR/comment clarifying boundary ownership
8. Only then consider merge
```

---

## 10. Done Definition

只有同时满足以下条件才算完成：

```text
[ ] Every current StreamEvent variant has explicit reducer disposition
[ ] No known variant relies on catch-all default
[ ] Terminal branch requires event: never
[ ] Adding a hypothetical new union member makes typecheck fail
[ ] Runtime unknown event is still rejected at parseStreamEvent boundary
[ ] Existing ActiveTurn state/effect behavior is preserved
[ ] No accidental change to processed/lastSeq semantics
[ ] Focused tests + typecheck + diff check pass
```

结论：当前问题不是“必须使用名为 `assertNever` 的函数”，而是 reducer 必须拥有一个**显式 `never` proof point**。对 BodySense 当前边界设计，`assertNever(event)` 是首选实现，因为 network unknown 已经由 `parseStreamEvent()` 负责，内部 trusted reducer 更适合 fail-closed / exhaustive，而不是继续用 permissive `default: break` 静默吞掉新 known variant。
