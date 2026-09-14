/**
 * SSE 事件处理器 —— 解析 SSE 文本行，分发给对应的回调函数。
 *
 * ===== 背景知识 =====
 * SSE（Server-Sent Events）是浏览器原生支持的服务端推送协议。
 * 后端返回的响应格式是纯文本，每一行要么是 "event: xxx"，要么是 "data: xxx"。
 * 一个完整的 SSE 消息由一对 event + data 组成，例如：
 *
 *   event: message.text.delta
 *   data: {"type":"message.text.delta","payload":{"text":"你好"}}
 *
 * 这个文件做的事情就是：把这些文本行解析出来，根据事件类型调用对应的回调函数。
 *
 * 深入笔记（Thought Forest 文件名）：
 * - web-streams-and-incremental-text-decoding.md
 * - ndjson-sse-and-streaming-protocol-boundaries.md
 * - abortcontroller-and-async-cancellation.md
 * - typescript-static-types-and-runtime-validation.md
 *
 * 分层提醒：本文件处理“字节块 → 文本行 → 协议事件”。JSON 能解析并不等于
 * 已通过 StreamEvent 运行时校验；可信类型边界应在事件进入 reducer 前建立。
 */

import { parseStreamEvent } from "@bodysense/contracts";

// ======================== 类型导入 ========================
import type {
  ConversationCreatedEvent,
  MessagePersistedEvent,
  MessageCreatedEvent,
  MessageTextDeltaEvent,
  ToolCallEvent,
  ToolResultEvent,
  ExtractedInfoUpsertEvent,
  LifestyleContextUpsertEvent,
  PhaseChangedEvent,
  CitationAddedEvent,
  KnowledgeGapEvent,
  RedFlagDetectedEvent,
  MessageCompletedEvent,
  MessageFailedEvent,
  TitleGeneratedEvent,
  StreamDoneEvent,
  StreamErrorEvent,
  RunStartedEvent,
  RunResumedEvent,
  RunInterruptedEvent,
  RunCompletedEvent,
  RunFailedEvent,
  RunCancelledEvent,
  InteractionRequiredEvent,
  InteractionAnsweredEvent,
  InteractionExpiredEvent,
  StreamEvent,
} from "@bodysense/contracts";

// ======================== SSEHandlers 接口 ========================
// 这个接口定义了所有可能的 SSE 事件回调函数。
// 每个回调都是可选的（用 ? 标记），调用方可以只传自己关心的事件。
//
// 用法示例：
//   const handlers: SSEHandlers = {
//     onTextDelta: (data) => { console.log('AI 说：', data.payload.text) },
//     onDone: () => { console.log('流结束了') },
//   };
export interface SSEHandlers {
  onConversationCreated?: (data: ConversationCreatedEvent) => void; // 新会话被创建时触发
  onRunStarted?: (data: RunStartedEvent) => void; // 一次 consultation run 开始时触发
  onRunResumed?: (data: RunResumedEvent) => void; // 被中断的 run 恢复时触发
  onRunInterrupted?: (data: RunInterruptedEvent) => void; // run 被中断时触发
  onRunCompleted?: (data: RunCompletedEvent) => void; // run 完成时触发
  onRunFailed?: (data: RunFailedEvent) => void; // run 失败时触发
  onRunCancelled?: (data: RunCancelledEvent) => void; // 用户显式取消 run 时触发
  onMessagePersisted?: (data: MessagePersistedEvent) => void; // 用户消息持久化完成时触发
  onMessageCreated?: (data: MessageCreatedEvent) => void; // AI 回复消息（空占位）创建时触发
  onTextDelta?: (data: MessageTextDeltaEvent) => void; // AI 输出一个文字片段时触发（高频，用于打字机效果）
  onToolCall?: (data: ToolCallEvent) => void; // AI 决定调用工具时触发
  onToolResult?: (data: ToolResultEvent) => void; // 工具返回结果时触发
  onExtractedInfo?: (data: ExtractedInfoUpsertEvent) => void; // 从对话中提取出用户信息时触发
  onLifestyleContext?: (data: LifestyleContextUpsertEvent) => void;
  onPhaseChange?: (data: PhaseChangedEvent) => void; // 对话阶段切换时触发（如：问诊 → 建议）
  onCitation?: (data: CitationAddedEvent) => void; // AI 引用了知识来源时触发
  onKnowledgeGap?: (data: KnowledgeGapEvent) => void; // 发现知识缺口时触发
  onRedFlag?: (data: RedFlagDetectedEvent) => void; // 检测到安全风险时触发
  onMessageCompleted?: (data: MessageCompletedEvent) => void; // AI 回复完整结束时触发
  onMessageFailed?: (data: MessageFailedEvent) => void; // AI 回复失败时触发
  onTitleGenerated?: (data: TitleGeneratedEvent) => void; // 会话标题生成完成时触发
  onInteractionRequired?: (data: InteractionRequiredEvent) => void; // 需要用户交互（如确认）时触发
  onInteractionAnswered?: (data: InteractionAnsweredEvent) => void; // 用户回答了交互请求时触发
  onInteractionExpired?: (data: InteractionExpiredEvent) => void;
  onDone?: (data: StreamDoneEvent) => void; // 整个 SSE 流正常结束时触发
  onStreamError?: (data: StreamErrorEvent) => void; // 流级别错误发生时触发
  onError?: (error: Error) => void; // 本地解析/网络错误时触发（非服务端事件）
}

// ======================== Exhaustive event dispatch ========================
//
// StreamEvent is generated from the canonical JSON Schema. Keeping dispatch as
// an exhaustive switch means a newly-added protocol variant cannot silently
// bypass the live/replay consumer: TypeScript must see either a handler or an
// explicit intentional no-op before the build is green.
function assertNever(event: never): never {
  throw new Error(`Unhandled validated StreamEvent: ${JSON.stringify(event)}`);
}

function dispatchStreamEvent(event: StreamEvent, handlers: SSEHandlers): void {
  switch (event.type) {
    case "conversation.created":
      handlers.onConversationCreated?.(event);
      return;
    case "run.started":
      handlers.onRunStarted?.(event);
      return;
    case "run.resumed":
      handlers.onRunResumed?.(event);
      return;
    case "run.interrupted":
      handlers.onRunInterrupted?.(event);
      return;
    case "run.completed":
      handlers.onRunCompleted?.(event);
      return;
    case "run.failed":
      handlers.onRunFailed?.(event);
      return;
    case "run.cancelled":
      handlers.onRunCancelled?.(event);
      return;
    case "message.persisted":
      handlers.onMessagePersisted?.(event);
      return;
    case "message.created":
      handlers.onMessageCreated?.(event);
      return;
    case "message.text.delta":
      handlers.onTextDelta?.(event);
      return;
    case "message.completed":
      handlers.onMessageCompleted?.(event);
      return;
    case "message.failed":
      handlers.onMessageFailed?.(event);
      return;
    case "tool.call":
      handlers.onToolCall?.(event);
      return;
    case "tool.result":
      handlers.onToolResult?.(event);
      return;
    case "state.extracted_info.upsert":
      handlers.onExtractedInfo?.(event);
      return;
    case "state.lifestyle_context.upsert":
      handlers.onLifestyleContext?.(event);
      return;
    case "state.phase.changed":
      handlers.onPhaseChange?.(event);
      return;
    case "state.interaction.required":
      handlers.onInteractionRequired?.(event);
      return;
    case "state.interaction.answered":
      handlers.onInteractionAnswered?.(event);
      return;
    case "state.interaction.expired":
      handlers.onInteractionExpired?.(event);
      return;
    case "source.citation.added":
      handlers.onCitation?.(event);
      return;
    case "source.knowledge_gap":
      handlers.onKnowledgeGap?.(event);
      return;
    case "safety.red_flag.detected":
      handlers.onRedFlag?.(event);
      return;
    case "title.generated":
      handlers.onTitleGenerated?.(event);
      return;
    case "stream.done":
      handlers.onDone?.(event);
      return;
    case "stream.error":
      handlers.onStreamError?.(event);
      return;

    // Valid public events intentionally not projected by this dispatcher yet.
    // They remain visible in the exhaustive switch so protocol growth cannot be
    // mistaken for an unknown-event fallback.
    case "source.answer_attribution.added":
    case "safety.output_reviewed":
    case "safety.output_rejected":
    case "usage.reported":
    case "job.created":
    case "job.progress":
    case "job.completed":
    case "job.failed":
      return;
    default:
      return assertNever(event);
  }
}

// ======================== processSSELine 函数 ========================
// 作用：处理 SSE 流中的**单行文本**，判断它是 event 行还是 data 行，然后做相应处理。
//
// SSE 协议规定：
//   - "event: xxx" 行声明接下来的 data 属于什么类型的事件
//   - "data: xxx" 行携带事件的实际数据（JSON 字符串）
//   - 一个空行表示一条消息结束
//
// 参数：
//   line    —— SSE 流中的一行文本
//   state   —— 一个可变的状态对象，用来在多行之间记住当前事件类型
//              为什么需要它？因为 event 行和 data 行是分开的两行，
//              我们需要先记住 event 类型，等读到 data 行时才知道该调用哪个回调。
//   handlers —— 调用方传入的所有事件回调函数
export interface SSEParseState {
  /** Last event: line type remembered across lines. */
  currentEvent: string;
  /** Highest StreamEvent.seq observed so far (for after_seq resume). */
  maxSeq: number;
}

export function processSSELine(
  line: string,
  state: SSEParseState, // 可变状态：记住最近一次读到的事件类型 + maxSeq
  handlers: SSEHandlers, // 所有事件的回调函数集合
): void {
  // 去掉行首尾的空白字符（如空格、\r 等）
  const trimmed = line.trim();

  // --------- 处理 event 行 ---------
  // SSE 格式："event: message.text.delta"
  //            ^^^^^^  ^^^^^^^^^^^^^^^^
  //            前缀     事件类型名
  if (trimmed.startsWith("event: ")) {
    // slice(7) 跳过 "event: " 这 7 个字符，拿到事件类型名
    // 例如 "event: message.text.delta" → "message.text.delta"
    state.currentEvent = trimmed.slice(7).trim();
    // 🔍 追踪所有 event 行（高频事件只在关注时打开）
    if (
      state.currentEvent === "conversation.created" ||
      state.currentEvent === "title.generated"
    ) {
      console.debug(`[SSE] ① event 行 → 记住事件类型: "${state.currentEvent}"`);
    }
    return; // event 行只需要记住类型，不做其他处理，直接返回
  }

  // --------- 处理 data 行 ---------
  // SSE 格式："data: {"type":"message.text.delta","payload":{"text":"你好"}}"
  //            ^^^^^^  ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  //            前缀     JSON 数据
  if (trimmed.startsWith("data: ")) {
    // slice(6) 跳过 "data: " 这 6 个字符，拿到 JSON 字符串
    const dataStr = trimmed.slice(6);

    try {
      // JSON.parse 只证明语法正确；真正的可信边界由共享 contracts parser 建立。
      const data: unknown = JSON.parse(dataStr);
      const event = parseStreamEvent(data);
      if (state.currentEvent && state.currentEvent !== event.type) {
        throw new Error(
          `SSE event/data type mismatch: event=${state.currentEvent} data=${event.type}`,
        );
      }
      const eventType = event.type;

      // Monotonic seq tracking enables GET .../events?after_seq=N resume.
      if (typeof event.seq === "number" && event.seq > state.maxSeq) {
        state.maxSeq = event.seq;
      }

      if (
        eventType === "conversation.created" ||
        eventType === "title.generated"
      ) {
        console.debug(`[SSE] ② data 行解析成功 → 分发事件: ${eventType}`, {
          conversation_id: event.ids.conversation_id,
          payload: event.payload,
        });
      }
      dispatchStreamEvent(event, handlers);
    } catch (err) {
      const protocolError = new Error(
        `Invalid consultation SSE event: ${err instanceof Error ? err.message : String(err)}`,
      );
      console.warn("[SSE] protocol validation failed:", dataStr, protocolError);
      handlers.onError?.(protocolError);
    }
  }

  // 空行和其他格式的行（如注释行 ": xxx"）会被静默忽略
}

// ======================== consumeSSEStream 函数 ========================
// 作用：读取一个 SSE 格式的 HTTP Response，持续解析其中的事件并分发给回调。
//
// 这是一个 async 函数（返回 Promise），因为它需要异步地从流中读取数据。
//
// 参数：
//   response —— fetch() 返回的 Response 对象，body 是一个 ReadableStream
//   handlers —— 调用方传入的所有事件回调函数
//
// 为什么不用浏览器内置的 EventSource？
// 因为 EventSource 只支持 GET 请求，而我们的 SSE 是通过 POST 请求发起的
// （需要在 body 中传参数），所以必须手动解析 SSE 流。
export async function consumeSSEStream(
  response: Response,
  handlers: SSEHandlers,
  signal?: AbortSignal,
): Promise<number> {
  // 从 Response 的 body 中获取 ReadableStream 的 reader。
  // reader 是逐块读取流数据的工具。
  // 用 ?. 是因为 body 可能为 null（理论上不会，但防御性编程）。
  const reader = response.body?.getReader();

  // 如果拿不到 reader（比如 body 为空），触发 onError 回调并退出
  if (!reader) {
    handlers.onError?.(new Error("No response body"));
    return 0;
  }

  // TextDecoder 用于把二进制数据（Uint8Array）解码成字符串。
  // 流中读到的每一块数据都是 Uint8Array，需要解码才能当文本处理。
  const abortReader = () => {
    void reader.cancel().catch(() => undefined);
  };
  if (signal?.aborted) {
    abortReader();
  } else {
    signal?.addEventListener("abort", abortReader, { once: true });
  }

  const decoder = new TextDecoder();

  // state 对象：在多行解析之间保持状态（记住当前事件类型）
  // 用对象而不是普通变量，是因为 processSSELine 需要修改它，
  // 对象是引用传递，函数内的修改会影响到外层。
  const state: SSEParseState = { currentEvent: "", maxSeq: 0 };

  // buffer：文本缓冲区。
  // 因为流中读到的数据块不一定刚好在换行符处断开，
  // 可能一块数据的末尾是半行，下一块数据的开头是后半行，
  // 所以需要用 buffer 暂存还没处理完的部分。
  let buffer = "";

  try {
    // 持续循环读取，直到流结束
    while (true) {
      // reader.read() 返回 { done: boolean, value: Uint8Array }
      //   done = true 表示流已结束，没有更多数据
      //   value 是这一块的二进制数据
      const { done, value } = await reader.read();

      // 流结束，跳出循环
      if (done) break;

      // 把二进制块解码为字符串，追加到 buffer 末尾。
      // { stream: true } 告诉 TextDecoder：
      // "这是一块流数据，末尾可能有不完整的多字节字符，先别急着解码它，
      //  等下一块数据来了再一起解码。"
      // 例如中文字符"你"占 3 字节，如果一块数据刚好在第 2 字节断开，
      // 没有 stream: true 就会产生乱码。
      buffer += decoder.decode(value, { stream: true });

      // 按换行符 \n 分割 buffer，得到多行文本。
      // 例如 "event: foo\ndata: bar\nbaz" → ["event: foo", "data: bar", "baz"]
      const lines = buffer.split("\n");

      // pop() 取出并移除数组最后一个元素。
      // 最后一个元素可能是不完整的行（还没读到换行符），放回 buffer 等待下次处理。
      // 例如上面的 "baz" 就是不完整的行，保留在 buffer 中。
      // 如果最后一行刚好是完整的（以 \n 结尾），split 后末尾会产生空字符串 ""，
      // pop() 取出 "" 放回 buffer，不影响后续处理。
      buffer = lines.pop() || "";

      // 逐行处理：把每一行交给 processSSELine 解析
      for (const line of lines) {
        processSSELine(line, state, handlers);
      }
    }

    // 循环结束后，buffer 中可能还有最后一行数据（流的最后一行没有 \n 结尾）
    // trim() 后如果非空，说明还有内容需要处理
    if (buffer.trim()) {
      processSSELine(buffer, state, handlers);
    }
  } catch (err) {
    // A durable terminal watcher may intentionally cancel the live reader.
    // That is convergence, not a transport failure.
    if (!signal?.aborted) {
      handlers.onError?.(err instanceof Error ? err : new Error(String(err)));
    }
  } finally {
    signal?.removeEventListener("abort", abortReader);
  }
  return state.maxSeq;
}

/**
 * Dispatch a batch of durable StreamEvents (e.g. from GET .../events?after_seq=N)
 * through the same handler map used for live SSE, skipping seq already seen.
 */
export function dispatchReplayEvents(
  events: readonly StreamEvent[],
  handlers: SSEHandlers,
  state: SSEParseState = { currentEvent: "", maxSeq: 0 },
): SSEParseState {
  const sorted = [...events].sort((a, b) => (a.seq ?? 0) - (b.seq ?? 0));
  for (const event of sorted) {
    if (typeof event.seq === "number" && event.seq <= state.maxSeq) {
      continue; // de-dupe
    }
    if (typeof event.seq === "number" && event.seq > state.maxSeq) {
      state.maxSeq = event.seq;
    }
    dispatchStreamEvent(event, handlers);
  }
  return state;
}
