import { useCallback, useRef, useState } from "react";
import { useAuthStore } from "@/stores/authStore";
import { readBodyFactCommit } from "@/features/profile/services/bodyFactReceipt";
import {
  useLocalRuntime,
  type ChatModelAdapter,
  type ChatModelRunResult,
  type ThreadMessageLike,
} from "@assistant-ui/react";

/** Minimal thread handle used to resume HITL runs under assistant-ui 0.15. */
export type ConsultationThreadController = {
  resumeRun: (config: {
    parentId: string | null;
    stream: () => AsyncGenerator<ChatModelRunResult, void, unknown>;
  }) => void;
  getState: () => { messages: readonly { id?: string }[] };
};
import { consultationApi } from "../services/consultationService";
import { consumeSSEStream, type SSEHandlers } from "./useSSEProcessor";
import { recoverDurableRunEvents } from "../runtime/durableRunRecovery";
import { reportClientDiagnostic } from "@/lib/clientDiagnostics";
import {
  reduceActiveTurnEvent,
  INITIAL_ACTIVE_TURN_STATE,
  resetActiveTurnState,
  type ActiveTurnState,
  type ActiveTurnEffect,
} from "../runtime/activeTurnReducer";
import type {
  ExtractedInfo,
  Citation,
  SSEMessageCompleted,
  RedFlagEvent,
  StreamEvent,
  PendingInteraction,
  ConsultationSpatialContext,
} from "../types/consultation";

/** Ephemeral image attachments for the next user turn (Phase 3-B2).
 * ChatInput pushes upload_ids here; the model adapter drains them when
 * composing the StartRun parts. Not a global store — single in-flight turn.
 */
export type ConsultationImageAttachment = {
  uploadId: string;
  mimeType?: string;
  imageUrl?: string;
};

export const consultationAttachmentBuffer: {
  next: ConsultationImageAttachment[];
} = {
  next: [],
};

/** Ephemeral Body Explorer context for the next user turn only. */
export const consultationSpatialContextBuffer: {
  next: ConsultationSpatialContext | null;
} = {
  next: null,
};

export interface ConsultationAdapterOptions {
  onConversationCreated?: (conversationId: string) => void;
  onMessagePersisted?: (clientMessageId: string, messageId: string) => void;
  onExtractedInfoUpdate?: (info: ExtractedInfo) => void;
  onPhaseChange?: (from: string, to: string) => void;
  onRedFlag?: (flag: RedFlagEvent) => void;
  onCitation?: (citation: Citation) => void;
  onMessageCompleted?: (data: SSEMessageCompleted) => void;
  onInteractionRequired?: (interaction: PendingInteraction) => void;
  /** Called on each dispatch with the full active turn state. */
  onActiveTurnUpdate?: (state: ActiveTurnState) => void;
  /** Called when the stream has fully completed or errored/aborted. */
  onStreamFinished?: () => void;
  onBodyFactCommitted?: () => void;
}

export function useAssistantChatRuntime(
  conversationId: string,
  initialMessages: ThreadMessageLike[] = [],
  options: ConsultationAdapterOptions = {},
) {
  const [isStreaming, setIsStreaming] = useState(false);
  const optionsRef = useRef(options);
  optionsRef.current = options;

  // Ref to stabilize resumeInteraction — avoids re-creating on every render
  type StreamRunFn = (
    startRequest: () => Promise<Response>,
    preserveUnstartedState?: boolean,
  ) => AsyncGenerator<ChatModelRunResult>;
  const streamConsultationRunRef = useRef<StreamRunFn | null>(null);

  async function* streamConsultationRun(
    startRequest: () => Promise<Response>,
    preserveUnstartedState = false,
  ): AsyncGenerator<ChatModelRunResult> {
    let streamError: Error | null = null;
    let streamFinished = false;
    let wakeQueueConsumer: (() => void) | null = null;
    let lastYieldSignature: string | null = null;
    const pendingResults: ChatModelRunResult[] = [];
    let reducerState: ActiveTurnState = INITIAL_ACTIVE_TURN_STATE;
    let maxSeq = 0;
    let sawStreamDone = false;
    const networkFailure: { current: Error | null } = { current: null };
    let startDurableWatcher: (() => void) | null = null;
    let durableWatcherController: AbortController | null = null;
    let liveReaderController: AbortController | null = null;
    let durableWatcherPromise: Promise<void> | null = null;

    function notifyQueueConsumer() {
      const wake = wakeQueueConsumer;
      wakeQueueConsumer = null;
      wake?.();
    }

    function enqueueResult(result: ChatModelRunResult | null) {
      if (!result) {
        return;
      }

      const signature = JSON.stringify(result.content);
      if (signature === lastYieldSignature) {
        return;
      }

      lastYieldSignature = signature;
      pendingResults.push(result);
      notifyQueueConsumer();
    }

    function waitForQueuedResult() {
      if (pendingResults.length > 0 || streamFinished) {
        return Promise.resolve();
      }

      return new Promise<void>((resolve) => {
        wakeQueueConsumer = resolve;
      });
    }

    function applyEffects(effects: ActiveTurnEffect[]) {
      for (const effect of effects) {
        switch (effect.type) {
          case "conversation_created":
            console.debug(
              "[SSE] ④ applyEffects → 触发 onConversationCreated 回调",
              {
                conversationId: effect.conversationId,
                hasCallback: !!optionsRef.current.onConversationCreated,
              },
            );
            optionsRef.current.onConversationCreated?.(effect.conversationId);
            break;
          case "message_persisted":
            optionsRef.current.onMessagePersisted?.(
              effect.clientMessageId,
              effect.messageId,
            );
            break;
          case "extracted_info_updated":
            optionsRef.current.onExtractedInfoUpdate?.(effect.info);
            break;
          case "phase_changed":
            optionsRef.current.onPhaseChange?.(effect.from, effect.to);
            break;
          case "red_flag":
            optionsRef.current.onRedFlag?.(effect.flags);
            break;
          case "citation_added":
            optionsRef.current.onCitation?.(effect.citation);
            break;
          case "interaction_required":
            optionsRef.current.onInteractionRequired?.(effect.interaction);
            break;
          case "interaction_answered":
            break;
          case "message_completed":
            optionsRef.current.onMessageCompleted?.(effect.data);
            break;
          case "stream_error":
            break;
          default:
            break;
        }
      }
    }

    const receivedFactCommits = new Set<string>();
    function dispatch(event: StreamEvent) {
      if (
        event.type === "tool.result" &&
        event.payload.tool === "record_body_state"
      ) {
        const receipt = readBodyFactCommit(event.payload.result);
        if (
          receipt &&
          receipt.user_id === useAuthStore.getState().user?.id &&
          !receivedFactCommits.has(receipt.id)
        ) {
          receivedFactCommits.add(receipt.id);
          optionsRef.current.onBodyFactCommitted?.();
        }
      }
      if (typeof event.seq === "number") {
        maxSeq = Math.max(maxSeq, event.seq);
      }
      const result = reduceActiveTurnEvent(reducerState, event);
      reducerState = result.state;
      // 🔍 追踪关键事件的 effect 产生情况
      if (event.type === "conversation.created") {
        console.debug(
          `[SSE] ④ dispatch(${event.type}) → effects 数量: ${result.effects.length}`,
          {
            effects: result.effects.map((e) => e.type),
            hasConversationCreatedCb:
              !!optionsRef.current.onConversationCreated,
          },
        );
      }
      applyEffects(result.effects);
      optionsRef.current.onActiveTurnUpdate?.(reducerState);
      startDurableWatcher?.();

      switch (event.type) {
        case "message.text.delta":
        case "state.interaction.required":
          enqueueResult(buildStreamingSnapshot(reducerState));
          break;
        case "message.completed":
        case "stream.done":
          enqueueResult(buildCompletedSnapshot(reducerState));
          break;
        default:
          break;
      }
    }

    try {
      const response = await startRequest();
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const handlers: SSEHandlers = {
        onConversationCreated: (data) => {
          console.debug(
            "[SSE] ②-SSE handler 收到 conversation.created → 准备 dispatch",
            {
              conversation_id: data.ids.conversation_id,
              run_id: data.ids.run_id,
            },
          );
          dispatch(data);
        },
        onMessagePersisted: (data) => dispatch(data),
        onRunStarted: (data) => dispatch(data),
        onRunResumed: (data) => dispatch(data),
        onRunInterrupted: (data) => dispatch(data),
        onRunCompleted: (data) => dispatch(data),
        onRunFailed: (data) => dispatch(data),
        onRunCancelled: (data) => dispatch(data),
        onMessageCreated: (data) => dispatch(data),
        onTextDelta: (data) => dispatch(data),
        onToolCall: (data) => dispatch(data),
        onToolResult: (data) => dispatch(data),
        onExtractedInfo: (data) => dispatch(data),
        onPhaseChange: (data) => dispatch(data),
        onRedFlag: (data) => dispatch(data),
        onCitation: (data) => dispatch(data),
        onKnowledgeGap: (data) => dispatch(data),
        onInteractionRequired: (data) => dispatch(data),
        onInteractionAnswered: (data) => dispatch(data),
        onInteractionExpired: (data) => dispatch(data),
        onMessageCompleted: (data) => dispatch(data),
        onMessageFailed: (data) => dispatch(data),
        onDone: (data) => {
          sawStreamDone = true;
          dispatch(data);
        },
        onStreamError: (data) => {
          streamError = new Error(data.payload.message);
          dispatch(data);
          streamFinished = true;
          notifyQueueConsumer();
        },
        onError: (err: Error) => {
          // Network/read failure — attempt durable after_seq resume below.
          networkFailure.current = err;
          reportClientDiagnostic({
            category: "chat.transport",
            event: "sse_read_failed",
            severity: "warn",
            code: err.name,
            message: err.message,
            phase: "live_stream",
            conversationId:
              reducerState.conversationId ||
              (conversationId !== "new" ? conversationId : null),
            runId: reducerState.runId,
          });
          streamFinished = true;
          notifyQueueConsumer();
        },
      };

      durableWatcherController = new AbortController();
      liveReaderController = new AbortController();

      startDurableWatcher = () => {
        if (durableWatcherPromise) return;
        const convId =
          reducerState.conversationId ||
          (conversationId !== "new" ? conversationId : null);
        const runId = reducerState.runId;
        if (!convId || !runId || !durableWatcherController) return;

        const watcherSignal = durableWatcherController.signal;
        reportClientDiagnostic({
          category: "chat.transport",
          event: "durable_watcher_started",
          severity: "info",
          phase: "live_stream",
          conversationId: convId,
          runId,
        });
        durableWatcherPromise = recoverDurableRunEvents({
          afterSeq: maxSeq,
          fetchPage: (afterSeq) =>
            consultationApi.listRunEvents(convId, runId, {
              afterSeq,
              limit: 200,
            }),
          handlers,
          signal: watcherSignal,
        })
          .then((recovered) => {
            maxSeq = Math.max(maxSeq, recovered.maxSeq);
            reportClientDiagnostic({
              category: "chat.transport",
              event: "durable_watcher_terminal",
              severity: "info",
              code: recovered.terminalType,
              phase: "live_stream",
              conversationId: convId,
              runId,
            });
            // The durable event log is the terminal authority. Once it has a
            // terminal event, stop waiting for a proxy/SSE socket to notice.
            liveReaderController?.abort();
            streamFinished = true;
            notifyQueueConsumer();
          })
          .catch((error) => {
            if (watcherSignal.aborted) return;
            const recoveryError =
              error instanceof Error
                ? error
                : new Error("durable run recovery failed");
            reportClientDiagnostic({
              category: "chat.transport",
              event: "durable_watcher_failed",
              severity: "error",
              code: recoveryError.name,
              message: recoveryError.message,
              phase: "live_stream",
              conversationId: convId,
              runId,
            });
            streamError = recoveryError;
            liveReaderController?.abort();
            streamFinished = true;
            notifyQueueConsumer();
          });
      };

      const streamPromise = consumeSSEStream(
        response,
        handlers,
        liveReaderController.signal,
      )
        .then((seq) => {
          maxSeq = Math.max(maxSeq, seq);
        })
        .finally(() => {
          streamFinished = true;
          notifyQueueConsumer();
        });

      while (!streamFinished || pendingResults.length > 0) {
        if (pendingResults.length === 0) {
          await waitForQueuedResult();
          continue;
        }

        const nextResult = pendingResults.shift();
        if (nextResult) {
          yield nextResult;
        }
      }

      await streamPromise;

      // A clean EOF without stream.done is not success. Cancellation and some
      // reverse proxies close the live response before their durable terminal
      // reaches the browser. The eager watcher is preferred, but a guaranteed
      // fallback recovery starts here if it never attached for any reason.
      if (!sawStreamDone && !streamError) {
        if (durableWatcherPromise) {
          await durableWatcherPromise;
          networkFailure.current = null;
        } else {
          const convId =
            reducerState.conversationId ||
            (conversationId !== "new" ? conversationId : null);
          const runId = reducerState.runId;
          if (convId && runId) {
            const fallbackController = new AbortController();
            durableWatcherController = fallbackController;
            const disconnectError = networkFailure.current;
            reportClientDiagnostic({
              category: "chat.transport",
              event: "fallback_recovery_started",
              severity: "warn",
              code: disconnectError?.name,
              message: disconnectError?.message,
              phase: "post_disconnect",
              conversationId: convId,
              runId,
            });

            // Re-open the queue while durable replay is dispatching so missing
            // deltas can continue rendering instead of appearing only on reload.
            streamFinished = false;
            durableWatcherPromise = recoverDurableRunEvents({
              afterSeq: maxSeq,
              fetchPage: (afterSeq) =>
                consultationApi.listRunEvents(convId, runId, {
                  afterSeq,
                  limit: 200,
                }),
              handlers,
              signal: fallbackController.signal,
            })
              .then((recovered) => {
                maxSeq = Math.max(maxSeq, recovered.maxSeq);
                networkFailure.current = null;
                reportClientDiagnostic({
                  category: "chat.transport",
                  event: "fallback_recovery_terminal",
                  severity: "info",
                  code: recovered.terminalType,
                  phase: "post_disconnect",
                  conversationId: convId,
                  runId,
                });
              })
              .catch((error) => {
                const recoveryError =
                  error instanceof Error
                    ? error
                    : new Error("durable run recovery failed");
                reportClientDiagnostic({
                  category: "chat.transport",
                  event: "fallback_recovery_failed",
                  severity: "error",
                  code: recoveryError.name,
                  message: recoveryError.message,
                  phase: "post_disconnect",
                  conversationId: convId,
                  runId,
                });
                streamError = recoveryError;
              })
              .finally(() => {
                streamFinished = true;
                notifyQueueConsumer();
              });

            while (!streamFinished || pendingResults.length > 0) {
              if (pendingResults.length === 0) {
                await waitForQueuedResult();
                continue;
              }
              const nextResult = pendingResults.shift();
              if (nextResult) yield nextResult;
            }
            await durableWatcherPromise;
          } else if (networkFailure.current) {
            streamError = networkFailure.current;
          } else {
            streamError = new Error(
              "实时连接在终态确认前结束，且没有可恢复的运行标识",
            );
          }
        }
      }

      if (streamError) {
        throw streamError;
      }
    } finally {
      startDurableWatcher = null;
      durableWatcherController?.abort();
      liveReaderController?.abort();
      setIsStreaming(false);
      optionsRef.current.onStreamFinished?.();
      if (
        reducerState.status === "completed" ||
        (reducerState.status === "idle" && !preserveUnstartedState)
      ) {
        optionsRef.current.onActiveTurnUpdate?.(resetActiveTurnState());
      }
    }
  }

  // Keep ref current so resumeInteraction always calls the latest streamConsultationRun
  streamConsultationRunRef.current = streamConsultationRun;

  const adapter: ChatModelAdapter = {
    async *run({ messages }): AsyncGenerator<ChatModelRunResult> {
      const lastMessage = messages[messages.length - 1];
      const content = lastMessage.content
        .filter((p): p is { type: "text"; text: string } => p.type === "text")
        .map((p) => p.text)
        .join("");

      const attachments = consultationAttachmentBuffer.next.splice(
        0,
        consultationAttachmentBuffer.next.length,
      );
      const spatialContext = consultationSpatialContextBuffer.next;
      consultationSpatialContextBuffer.next = null;
      const parts: Array<{
        type: string;
        text?: string;
        upload_id?: string;
        mime_type?: string;
        image_url?: string;
      }> = [];
      const text = content.trim();
      if (text) {
        parts.push({ type: "text", text });
      } else if (attachments.length > 0) {
        parts.push({
          type: "text",
          text: "请结合我附上的照片，分析与体态/不适相关的可见信息，并给出谨慎建议。",
        });
      }
      for (const image of attachments.slice(0, 3)) {
        parts.push({
          type: "image",
          upload_id: image.uploadId,
          ...(image.mimeType ? { mime_type: image.mimeType } : {}),
          ...(image.imageUrl ? { image_url: image.imageUrl } : {}),
        });
      }
      if (parts.length === 0) {
        parts.push({ type: "text", text: content || " " });
      }

      setIsStreaming(true);

      yield* streamConsultationRun(() =>
        consultationApi.startConsultationRun({
          conversationId: conversationId === "new" ? null : conversationId,
          clientMessageId: `tmp_${crypto.randomUUID()}`,
          requestId: crypto.randomUUID(),
          message: {
            role: "user",
            parts,
            ...(spatialContext
              ? { metadata: { body_explorer_context: spatialContext } }
              : {}),
          },
        }),
      );
    },
  };

  const runtime = useLocalRuntime(adapter, {
    initialMessages,
  });

  const resumeRequest = useRef<{
    conversationId: string;
    interactionId: string;
    content: string;
    requestId: string;
  } | null>(null);

  const resumeInteraction = useCallback(
    (
      threadRuntime: ConsultationThreadController,
      interactionId: string,
      answer: unknown,
    ): Promise<void> => {
      const content = JSON.stringify(answer);
      if (
        resumeRequest.current?.conversationId !== conversationId ||
        resumeRequest.current.interactionId !== interactionId ||
        resumeRequest.current.content !== content
      ) {
        resumeRequest.current = {
          conversationId,
          interactionId,
          content,
          requestId: crypto.randomUUID(),
        };
      }
      const requestId = resumeRequest.current.requestId;
      setIsStreaming(true);
      return new Promise<void>((resolve, reject) => {
        try {
          threadRuntime.resumeRun({
            parentId: threadRuntime.getState().messages.at(-1)?.id ?? null,
            stream: async function* () {
              try {
                const runFn = streamConsultationRunRef.current;
                if (!runFn)
                  throw new Error("Conversation runtime is not ready.");
                yield* runFn(
                  () =>
                    consultationApi.resumeInteractionStream(
                      conversationId,
                      interactionId,
                      { requestId, answer },
                    ),
                  true,
                );
                resolve();
              } catch (error) {
                // The pending composer owns retry feedback. An unacknowledged
                // HTTP failure must not fabricate an answered interaction.
                reject(error);
              } finally {
                setIsStreaming(false);
              }
            },
          });
        } catch (error) {
          setIsStreaming(false);
          reject(error);
        }
      });
    },
    [conversationId],
  );

  return { runtime, isStreaming, resumeInteraction };
}

function buildStreamingSnapshot(
  state: ActiveTurnState,
): ChatModelRunResult | null {
  if (!state.text) {
    return null;
  }

  return {
    content: [{ type: "text", text: state.text }],
  };
}

function buildCompletedSnapshot(
  state: ActiveTurnState,
): ChatModelRunResult | null {
  if (state.status !== "completed") {
    return null;
  }

  if (state.finalParts.length > 0) {
    return {
      content: [...state.finalParts],
    };
  }

  return buildStreamingSnapshot(state);
}
