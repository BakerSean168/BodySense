import type { ChatModelRunResult } from "@assistant-ui/react";
import { useAuthStore } from "@/stores/authStore";
import { readBodyFactCommit } from "@/features/profile/services/bodyFactReceipt";
import { consultationApi } from "../services/consultationService";
import { consumeSSEStream, type SSEHandlers } from "../hooks/useSSEProcessor";
import { recoverDurableRunEvents } from "./durableRunRecovery";
import { reportClientDiagnostic } from "@/lib/clientDiagnostics";
import {
  reduceActiveTurnEvent,
  INITIAL_ACTIVE_TURN_STATE,
  resetActiveTurnState,
  type ActiveTurnState,
} from "./activeTurnReducer";
import type { StreamEvent, PendingInteraction } from "../types/consultation";
import type { ConsultationAdapterOptions } from "./consultationRuntime.types";
import { applyConsultationEffects } from "./consultationEffects";
import {
  buildStreamingSnapshot,
  buildCompletedSnapshot,
} from "./chatModelSnapshots";

export async function* streamConsultationRun(
  context: {
    conversationId: string;
    optionsRef: { current: ConsultationAdapterOptions };
    setIsStreaming: (value: boolean) => void;
  },
  startRequest: () => Promise<Response>,
  preserveUnstartedState = false,
  resumedInteraction?: PendingInteraction,
): AsyncGenerator<ChatModelRunResult> {
  const { conversationId, optionsRef, setIsStreaming } = context;
  let streamError: Error | null = null;
  let streamFinished = false;
  let wakeQueueConsumer: (() => void) | null = null;
  let lastYieldSignature: string | null = null;
  const pendingResults: ChatModelRunResult[] = [];
  // A resume stream starts from the interaction it answers, so its answered
  // milestone (and any BodyState receipt on it) lands on that interaction.
  let reducerState: ActiveTurnState = resumedInteraction
    ? { ...INITIAL_ACTIVE_TURN_STATE, pendingInteraction: resumedInteraction }
    : INITIAL_ACTIVE_TURN_STATE;
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

    const signature = JSON.stringify([result.content, result.metadata]);
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
          hasConversationCreatedCb: !!optionsRef.current.onConversationCreated,
        },
      );
    }
    applyConsultationEffects(result.effects, optionsRef);
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
