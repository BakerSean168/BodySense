# 06 · JavaScript 与 TypeScript：从流字节到可信业务事件

> 对照真实源码：
>
> - `apps/web/src/features/consultation/hooks/useSSEProcessor.ts`
> - `apps/web/src/features/consultation/services/consultationService.ts`
> - `packages/contracts/src/stream-events.ts`
> - `apps/web/src/features/consultation/runtime/activeTurnReducer.ts`

## 1. 先看完整分层

```text
HTTP 响应字节
  -> ReadableStream reader
  -> TextDecoder 增量解码
  -> 行/事件协议解析
  -> JSON.parse 得到 unknown
  -> StreamEvent 运行时校验
  -> reducer 根据 event.type 更新状态
  -> React 渲染
```

每层只解决一个问题。最常见的错误，是把“能 JSON.parse”误认为“已经是可信的 StreamEvent”。

Thought Forest 对照：

- `web-streams-and-incremental-text-decoding.md`
- `ndjson-sse-and-streaming-protocol-boundaries.md`
- `abortcontroller-and-async-cancellation.md`
- `typescript-static-types-and-runtime-validation.md`

## 2. JavaScript：网络块不是文本行

`reader.read()` 返回 `{ done, value }`，其中 `value` 是 `Uint8Array`。一次读取可能：

- 只得到半个 UTF-8 字符；
- 得到半行 JSON；
- 一次得到多条事件；
- 在任意位置抛出网络错误。

因此 `useSSEProcessor.ts` 使用：

```ts
const decoder = new TextDecoder();
let buffer = '';

const { done, value } = await reader.read();
buffer += decoder.decode(value, { stream: true });
const lines = buffer.split('\n');
buffer = lines.pop() ?? '';
```

`stream: true` 允许解码器暂存跨块的多字节字符；`buffer` 暂存跨块的半行。这是两个不同层次的缓冲。

## 3. TypeScript：泛型事件信封

`StreamEventBase` 把协议中稳定和变化的部分分开：

```ts
interface StreamEventBase<
  TChannel extends StreamChannel,
  TType extends string,
  TPayload extends Record<string, unknown>,
> {
  channel: TChannel;
  type: TType;
  payload: TPayload;
}
```

- `TChannel` 限制事件所属频道。
- `TType` 保留具体事件名的字面量类型。
- `TPayload` 让每种事件拥有自己的 payload。

当所有具体事件组成联合后，`event.type` 就成为判别字段：

```ts
if (event.type === 'message.text.delta') {
  event.payload.delta; // 已收窄到字符串
}
```

对照笔记：

- `typescript-generics-keyof-and-indexed-access.md`
- `typescript-discriminated-unions-and-exhaustiveness.md`
- `typescript-unknown-vs-any.md`

## 4. `unknown`、`never` 和断言

- `unknown`：数据存在，但使用前必须证明形状。
- `never`：在当前控制流中不应再有任何可能；可用于穷尽检查。
- `as StreamEvent`：只有编译期断言，没有运行时证明。

当前 `listRunEvents` 中的断言是很好的练习入口：先把服务端 JSON 当作 `unknown`，检查 envelope 和最小字段，再交给业务代码。

## 5. React：事件进入 reducer

`ActiveTurnContext` 使用 action 联合和 `useReducer`：

```ts
type TurnAction =
  | { type: 'DISPATCH_EVENT'; event: StreamEvent }
  | { type: 'RESET_TURN' }
  | { type: 'HYDRATE_TURN'; state: ActiveTurnState };
```

这让“允许发生哪些状态转换”成为显式集合。Context 又拆成 state 和 actions：

- 读取 state 的组件随状态变化重渲染。
- 只使用稳定 actions 的组件不需要因 state 改变而重渲染。

## 6. 自测

1. 为什么不能对每个 `reader.read()` 的 value 直接执行 `JSON.parse`？
2. `TextDecoder` 的内部字节缓冲与字符串 `buffer` 分别解决什么？
3. `TType extends string` 为什么仍能保留 `'run.started'` 字面量？
4. `JSON.parse(raw) as StreamEvent` 缺少了哪一步？
5. 为什么 reducer action 很适合用可辨识联合？

## 7. 最小练习

给 `useSSEProcessor.test.ts` 增加一个测试：把“你”字的三个 UTF-8 字节拆到两个 chunk，同时把 JSON 行拆到三个 chunk。最终只能分发一个内容正确的事件。

