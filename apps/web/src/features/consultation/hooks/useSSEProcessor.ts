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

// ======================== 类型导入 ========================
// 从 consultation 类型文件中导入所有 SSE 事件的 TypeScript 类型。
// 这些类型定义了每个事件的 data 结构（有哪些字段、字段是什么类型）。
import type {
  SSEConversationCreated, // conversation.created 事件的数据类型（新会话创建）
  SSEMessagePersisted, // message.persisted 事件的数据类型（用户消息持久化完成）
  SSEMessageCreated, // message.created 事件的数据类型（AI 回复消息创建）
  SSETextDelta, // message.text.delta 事件的数据类型（AI 逐字输出的片段）
  SSEToolCall, // tool.call 事件的数据类型（AI 调用工具）
  SSEToolResult, // tool.result 事件的数据类型（工具返回结果）
  SSEExtractedInfo, // state.extracted_info.upsert 事件的数据类型（提取的用户信息）
  SSEPhaseChange, // state.phase.changed 事件的数据类型（对话阶段变化）
  SSECitation, // source.citation.added 事件的数据类型（引用来源）
  SSEKnowledgeGap, // source.knowledge_gap 事件的数据类型（知识缺口提示）
  SSERedFlag, // safety.red_flag.detected 事件的数据类型（安全风险标记）
  SSEMessageCompleted, // message.completed 事件的数据类型（AI 回复完成）
  SSEMessageFailed, // message.failed 事件的数据类型（AI 回复失败）
  SSETitleGenerated, // title.generated 事件的数据类型（会话标题生成完成）
  SSEStreamDone, // stream.done 事件的数据类型（整个 SSE 流结束）
  SSEStreamError, // stream.error 事件的数据类型（流级别错误）
  StreamEvent, // 所有 SSE 事件的基础类型（通用结构）
} from "../types/consultation";

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
  onConversationCreated?: (data: SSEConversationCreated) => void; // 新会话被创建时触发
  onRunStarted?: (data: StreamEvent) => void; // 一次 consultation run 开始时触发
  onRunResumed?: (data: StreamEvent) => void; // 被中断的 run 恢复时触发
  onRunInterrupted?: (data: StreamEvent) => void; // run 被中断时触发
  onRunCompleted?: (data: StreamEvent) => void; // run 完成时触发
  onRunFailed?: (data: StreamEvent) => void; // run 失败时触发
  onMessagePersisted?: (data: SSEMessagePersisted) => void; // 用户消息持久化完成时触发
  onMessageCreated?: (data: SSEMessageCreated) => void; // AI 回复消息（空占位）创建时触发
  onTextDelta?: (data: SSETextDelta) => void; // AI 输出一个文字片段时触发（高频，用于打字机效果）
  onToolCall?: (data: SSEToolCall) => void; // AI 决定调用工具时触发
  onToolResult?: (data: SSEToolResult) => void; // 工具返回结果时触发
  onExtractedInfo?: (data: SSEExtractedInfo) => void; // 从对话中提取出用户信息时触发
  onPhaseChange?: (data: SSEPhaseChange) => void; // 对话阶段切换时触发（如：问诊 → 建议）
  onCitation?: (data: SSECitation) => void; // AI 引用了知识来源时触发
  onKnowledgeGap?: (data: SSEKnowledgeGap) => void; // 发现知识缺口时触发
  onRedFlag?: (data: SSERedFlag) => void; // 检测到安全风险时触发
  onMessageCompleted?: (data: SSEMessageCompleted) => void; // AI 回复完整结束时触发
  onMessageFailed?: (data: SSEMessageFailed) => void; // AI 回复失败时触发
  onTitleGenerated?: (data: SSETitleGenerated) => void; // 会话标题生成完成时触发
  onInteractionRequired?: (data: StreamEvent) => void; // 需要用户交互（如确认）时触发
  onInteractionAnswered?: (data: StreamEvent) => void; // 用户回答了交互请求时触发
  onDone?: (data: SSEStreamDone) => void; // 整个 SSE 流正常结束时触发
  onStreamError?: (data: SSEStreamError) => void; // 流级别错误发生时触发
  onError?: (error: Error) => void; // 本地解析/网络错误时触发（非服务端事件）
}

// ======================== EVENT_MAP 事件映射表 ========================
// 这是一个"翻译字典"：把后端发来的事件类型字符串（左边）
// 映射到 SSEHandlers 接口中的回调函数名（右边）。
//
// 例如后端发来 "event: message.text.delta"，查表得到 'onTextDelta'，
// 然后就会调用 handlers.onTextDelta(data)。
//
// 为什么需要这个映射？
// 因为后端用的是带点号的事件名（如 message.text.delta），
// 而前端回调用的是驼峰命名（如 onTextDelta），风格不同，需要翻译。
const EVENT_MAP: Record<string, keyof SSEHandlers> = {
  "conversation.created": "onConversationCreated", // 新会话创建
  "run.started": "onRunStarted", // run 开始
  "run.resumed": "onRunResumed", // run 恢复
  "run.interrupted": "onRunInterrupted", // run 中断
  "run.completed": "onRunCompleted", // run 完成
  "run.failed": "onRunFailed", // run 失败
  "message.persisted": "onMessagePersisted", // 用户消息已持久化
  "message.created": "onMessageCreated", // AI 消息占位已创建
  "message.text.delta": "onTextDelta", // AI 文字片段（高频）
  "tool.call": "onToolCall", // 工具调用
  "tool.result": "onToolResult", // 工具结果
  "state.extracted_info.upsert": "onExtractedInfo", // 提取信息
  "state.phase.changed": "onPhaseChange", // 阶段变化
  "source.citation.added": "onCitation", // 引用来源
  "source.knowledge_gap": "onKnowledgeGap", // 知识缺口
  "safety.red_flag.detected": "onRedFlag", // 安全风险
  "message.completed": "onMessageCompleted", // 消息完成
  "message.failed": "onMessageFailed", // 消息失败
  "title.generated": "onTitleGenerated", // 标题生成
  "state.interaction.required": "onInteractionRequired", // 需要交互
  "state.interaction.answered": "onInteractionAnswered", // 交互已回答
  "stream.done": "onDone", // 流结束
  "stream.error": "onStreamError", // 流错误
};

// 回调函数的统一类型签名：接收一个 StreamEvent，无返回值。
type HandlerFn = (data: StreamEvent) => void;

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
      // 把 JSON 字符串解析成 JavaScript 对象
      const data = JSON.parse(dataStr);

      // 类型断言：把解析出来的对象当作 StreamEvent 类型
      const event = data as StreamEvent;

      // 优先使用 data 中自带的 type 字段，如果没有则回退到 state 中记住的 event 类型。
      // 为什么有这个回退？因为有些 SSE 实现只在 event 行声明类型，data 中不带 type 字段。
      const eventType = event.type || state.currentEvent;

      // Monotonic seq tracking enables GET .../events?after_seq=N resume.
      if (typeof event.seq === "number" && event.seq > state.maxSeq) {
        state.maxSeq = event.seq;
      }

      // 用事件类型在映射表中查找对应的回调函数名
      // 例如 "message.text.delta" → "onTextDelta"
      const handlerKey = EVENT_MAP[eventType];

      if (handlerKey) {
        // 用回调函数名从 handlers 对象中取出实际的函数
        const handler = handlers[handlerKey];
        if (handler) {
          // 🔍 追踪 conversation.created 和 title.generated 事件
          if (
            eventType === "conversation.created" ||
            eventType === "title.generated"
          ) {
            console.debug(`[SSE] ② data 行解析成功 → 分发事件: ${eventType}`, {
              handlerKey,
              conversation_id: (event as StreamEvent).ids?.conversation_id,
              payload: (event as StreamEvent).payload,
            });
          }
          // 调用回调函数，把事件数据传进去
          // 类型断言为 HandlerFn 是因为 TypeScript 无法自动推断这里调用的安全性
          (handler as HandlerFn)(event);
        } else if (
          eventType === "conversation.created" ||
          eventType === "title.generated"
        ) {
          console.warn(
            `[SSE] ⚠️ 事件 ${eventType} 的 handlerKey="${handlerKey}" 在 handlers 中为 ${handler}!`,
          );
        }
      } else if (
        eventType === "conversation.created" ||
        eventType === "title.generated"
      ) {
        console.warn(
          `[SSE] ⚠️ 事件 ${eventType} 在 EVENT_MAP 中未找到 handlerKey!`,
        );
      }
      // 如果 handlerKey 为 undefined（事件类型不在映射表中），
      // 说明这是一个我们不认识的事件，静默忽略即可。
    } catch (err) {
      // JSON 解析失败（后端发来了格式错误的数据），打印警告但不中断流处理
      console.warn("[SSE] malformed JSON payload:", dataStr, err);
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
    // 捕获所有异常（网络断开、流读取错误等），触发 onError 回调。
    // 确保 err 是 Error 类型，如果不是则用 String() 转换后包装成 Error。
    handlers.onError?.(err instanceof Error ? err : new Error(String(err)));
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
    const eventType = event.type;
    const handlerKey = EVENT_MAP[eventType];
    if (!handlerKey) continue;
    const handler = handlers[handlerKey];
    if (handler) {
      (handler as HandlerFn)(event);
    }
  }
  return state;
}
