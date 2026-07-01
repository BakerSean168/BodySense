import { useRef, useState } from 'react';
import {
  useLocalRuntime,
  type ChatModelAdapter,
  type ChatModelRunResult,
  type ThreadRuntime,
  type ThreadMessageLike,
} from '@assistant-ui/react';
import { consultationApi } from '../services/consultationService';
import { consumeSSEStream } from './useSSEProcessor';
import {
  reduceActiveTurnEvent,
  INITIAL_ACTIVE_TURN_STATE,
  resetActiveTurnState,
  type ActiveTurnState,
  type ActiveTurnEffect,
} from '../runtime/activeTurnReducer';
import type {
  ExtractedInfo,
  Citation,
  SSERedFlag,
  SSEMessageCompleted,
  StreamEvent,
  PendingInteraction,
} from '../types/consultation';

export interface ConsultationAdapterOptions {
  onConversationCreated?: (conversationId: string) => void;
  onMessagePersisted?: (clientMessageId: string, messageId: string) => void;
  onExtractedInfoUpdate?: (info: ExtractedInfo) => void;
  onPhaseChange?: (from: string, to: string) => void;
  onRedFlag?: (flag: SSERedFlag['payload']) => void;
  onCitation?: (citation: Citation) => void;
  onTitleGenerated?: (title: string) => void;
  onMessageCompleted?: (data: SSEMessageCompleted) => void;
  onInteractionRequired?: (interaction: PendingInteraction) => void;
  /** Called on each dispatch with the full active turn state. */
  onActiveTurnUpdate?: (state: ActiveTurnState) => void;
}

export function useAssistantChatRuntime(
  conversationId: string,
  initialMessages: ThreadMessageLike[] = [],
  options: ConsultationAdapterOptions = {}
) {
  const [isStreaming, setIsStreaming] = useState(false);
  const optionsRef = useRef(options);
  optionsRef.current = options;

  async function* streamConsultationRun(
    startRequest: () => Promise<Response>,
  ): AsyncGenerator<ChatModelRunResult> {
    let streamError: Error | null = null;
    let streamFinished = false;
    let wakeQueueConsumer: (() => void) | null = null;
    let lastYieldSignature: string | null = null;
    const pendingResults: ChatModelRunResult[] = [];
    let reducerState: ActiveTurnState = INITIAL_ACTIVE_TURN_STATE;

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
          case 'message_persisted':
            optionsRef.current.onMessagePersisted?.(
              effect.clientMessageId,
              effect.messageId,
            );
            break;
          case 'extracted_info_updated':
            optionsRef.current.onExtractedInfoUpdate?.(effect.info);
            break;
          case 'phase_changed':
            optionsRef.current.onPhaseChange?.(effect.from, effect.to);
            break;
          case 'red_flag':
            optionsRef.current.onRedFlag?.(effect.flags as SSERedFlag['payload']);
            break;
          case 'citation_added':
            optionsRef.current.onCitation?.(effect.citation);
            break;
          case 'interaction_required':
            optionsRef.current.onInteractionRequired?.(effect.interaction);
            break;
          case 'interaction_answered':
            break;
          case 'message_completed':
            optionsRef.current.onMessageCompleted?.(effect.data as SSEMessageCompleted);
            break;
          case 'stream_error':
            break;
          default:
            break;
        }
      }
    }

    function dispatch(event: StreamEvent) {
      const result = reduceActiveTurnEvent(reducerState, event);
      reducerState = result.state;
      applyEffects(result.effects);
      optionsRef.current.onActiveTurnUpdate?.(reducerState);

      switch (event.type) {
        case 'message.text.delta':
        case 'state.interaction.required':
          enqueueResult(buildStreamingSnapshot(reducerState));
          break;
        case 'message.completed':
        case 'stream.done':
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

      const streamPromise = consumeSSEStream(response, {
        onMessagePersisted: (data) => dispatch(data as unknown as StreamEvent),
        onMessageCreated: (data) => dispatch(data as unknown as StreamEvent),
        onTextDelta: (data) => dispatch(data as unknown as StreamEvent),
        onToolCall: (data) => dispatch(data as unknown as StreamEvent),
        onToolResult: (data) => dispatch(data as unknown as StreamEvent),
        onExtractedInfo: (data) => dispatch(data as unknown as StreamEvent),
        onPhaseChange: (data) => dispatch(data as unknown as StreamEvent),
        onRedFlag: (data) => dispatch(data as unknown as StreamEvent),
        onCitation: (data) => dispatch(data as unknown as StreamEvent),
        onKnowledgeGap: (data) => dispatch(data as unknown as StreamEvent),
        onTitleGenerated: (data) => {
          optionsRef.current.onTitleGenerated?.(data.payload.title);
        },
        onInteractionRequired: (data) => dispatch(data as unknown as StreamEvent),
        onInteractionAnswered: (data) => dispatch(data as unknown as StreamEvent),
        onMessageCompleted: (data) => dispatch(data as unknown as StreamEvent),
        onMessageFailed: (data) => dispatch(data as unknown as StreamEvent),
        onDone: () => {},
        onStreamError: (data) => {
          streamError = new Error(data.payload.message);
        },
        onError: (err) => {
          streamError = err;
        },
      }).finally(() => {
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

      if (streamError) {
        throw streamError;
      }
    } finally {
      setIsStreaming(false);
      if (
        reducerState.status === 'completed' ||
        reducerState.status === 'failed' ||
        reducerState.status === 'idle'
      ) {
        optionsRef.current.onActiveTurnUpdate?.(resetActiveTurnState());
      }
    }
  }

  const adapter: ChatModelAdapter = {
    async *run({ messages }): AsyncGenerator<ChatModelRunResult> {
      const lastMessage = messages[messages.length - 1];
      const content = lastMessage.content
        .filter((p): p is { type: 'text'; text: string } => p.type === 'text')
        .map((p) => p.text)
        .join('');

      const clientMessageId = `tmp_${crypto.randomUUID()}`;
      const requestId = crypto.randomUUID();
      setIsStreaming(true);

      let activeId = conversationId;
      if (conversationId === 'new') {
        try {
          const session = await consultationApi.createConsultation();
          activeId = session.conversation_id;
          optionsRef.current.onConversationCreated?.(activeId);
        } catch (err) {
          setIsStreaming(false);
          throw err;
        }
      }

      yield* streamConsultationRun(() =>
        consultationApi.sendConsultationMessage(activeId, {
          clientMessageId,
          requestId,
          message: {
            role: 'user',
            parts: [{ type: 'text', text: content }],
          },
        }),
      );
    },
  };

  const runtime = useLocalRuntime(adapter, {
    initialMessages,
  });

  const resumeInteraction = (
    threadRuntime: ThreadRuntime,
    interactionId: string,
    answer: unknown,
  ) => {
    setIsStreaming(true);
    threadRuntime.resumeRun({
      parentId: threadRuntime.getState().messages.at(-1)?.id ?? null,
      stream: async function* () {
        yield* streamConsultationRun(() =>
          consultationApi.resumeInteractionStream(conversationId, interactionId, {
            requestId: crypto.randomUUID(),
            answer,
          }),
        );
      },
    });
  };

  return { runtime, isStreaming, resumeInteraction };
}

function buildStreamingSnapshot(state: ActiveTurnState): ChatModelRunResult | null {
  if (!state.text) {
    return null;
  }

  return {
    content: [{ type: 'text', text: state.text }],
  };
}

function buildCompletedSnapshot(state: ActiveTurnState): ChatModelRunResult | null {
  if (state.status !== 'completed') {
    return null;
  }

  if (state.finalParts.length > 0) {
    return {
      content: [...state.finalParts],
    };
  }

  return buildStreamingSnapshot(state);
}
