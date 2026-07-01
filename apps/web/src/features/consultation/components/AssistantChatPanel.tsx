import { AssistantRuntimeProvider, useThread, useComposerRuntime, ThreadPrimitive, MessagePrimitive, useMessage, type ThreadAssistantMessagePart, type ThreadMessageLike } from "@assistant-ui/react";
import { MarkdownTextPrimitive } from "@assistant-ui/react-markdown";
import remarkGfm from "remark-gfm";
import { useAssistantChatRuntime } from "../hooks/useAssistantChatRuntime";
import { ActiveTurnProvider, useActiveTurnActions, useActiveTurnState } from "../context/ActiveTurnContext";
import React, { useRef, useEffect, useState, useCallback, useMemo } from "react";
import type { Citation, ConsultationPhase, ExtractedInfo } from "../types/consultation";
import type { ActiveTurnState } from "../runtime/activeTurnReducer";
import { buildAssistantMessagePartsViewModel } from "../runtime/assistantMessagePartsViewModel";
import { selectIsComposerLocked } from "../runtime/activeTurnSelectors";
import { StreamingAssistantTurn } from "./StreamingAssistantTurn";
import { AskUserCard } from "./AskUserCard";
import { consultationApi } from "../services/consultationService";
import { StreamingTurnToolCalls } from "./StreamingTurnToolCalls";
import { RedFlagBanner } from "./RedFlagBanner";

interface AssistantChatPanelProps {
  conversationId: string | null;
  initialMessages?: ThreadMessageLike[];
  initialActiveTurn?: ActiveTurnState | null;
  initialExtractedInfo?: ExtractedInfo[];
  isDraft?: boolean;
  clientDraftId?: string | null;
  onExtractedInfoUpdate?: (info: ExtractedInfo[]) => void;
  onPhaseChange?: (phase: ConsultationPhase) => void;
  onCitation?: (citation: Citation) => void;
  onConversationCreated?: (conversationId: string) => void;
  onTitleGenerated?: (title: string) => void;
  onMessagePersisted?: (clientMessageId: string, messageId: string) => void;
}

/**
 * Bridge that syncs the adapter's active turn state into ActiveTurnContext.
 */
function ActiveTurnBridge({ bridgeRef }: { bridgeRef: React.MutableRefObject<((state: ActiveTurnState) => void) | null> }) {
  const { hydrateTurn } = useActiveTurnActions();
  bridgeRef.current = hydrateTurn;
  return null;
}

/**
 * Chat panel powered by assistant-ui runtime with ActiveTurnState.
 *
 * The runtime drives all SSE streaming via ConsultationChatAdapter.
 * Streaming content is rendered by StreamingAssistantTurn from ActiveTurnState.
 * Historical messages are rendered via assistant-ui's thread state.
 */
export function AssistantChatPanel({
  conversationId,
  initialMessages = [],
  initialActiveTurn = null,
  initialExtractedInfo: _initialExtractedInfo = [],
  isDraft,
  clientDraftId,
  onExtractedInfoUpdate,
  onPhaseChange,
  onCitation,
  onConversationCreated,
  onTitleGenerated,
  onMessagePersisted,
}: AssistantChatPanelProps) {
  const extractedInfoRef = useRef<ExtractedInfo[]>(_initialExtractedInfo);

  useEffect(() => {
    extractedInfoRef.current = _initialExtractedInfo;
  }, [_initialExtractedInfo]);

  const activeTurnRef = useRef<((state: ActiveTurnState) => void) | null>(null);
  const isResumingRef = useRef<boolean>(false);

  const adapterOptions = useMemo(() => ({
    onExtractedInfoUpdate: (info: ExtractedInfo) => {
      const existing = extractedInfoRef.current;
      const idx = existing.findIndex((e) => e.body_part === info.body_part);
      if (idx >= 0) {
        const updated = [...existing];
        updated[idx] = { ...updated[idx], ...info };
        extractedInfoRef.current = updated;
      } else {
        extractedInfoRef.current = [...existing, info];
      }
      onExtractedInfoUpdate?.(extractedInfoRef.current);
    },
    onPhaseChange: (_from: string, to: string) => {
      onPhaseChange?.(to as ConsultationPhase);
    },
    onCitation,
    onConversationCreated,
    onTitleGenerated,
    onMessagePersisted,
    onActiveTurnUpdate: (state: ActiveTurnState) => {
      activeTurnRef.current?.(state);
    },
    isDraft,
    clientDraftId: clientDraftId ?? undefined,
    isResumingRef,
  }), [onExtractedInfoUpdate, onPhaseChange, onCitation, onConversationCreated, onTitleGenerated, onMessagePersisted, isDraft, clientDraftId]);

  const { runtime } = useAssistantChatRuntime(conversationId, initialMessages, adapterOptions);

  return (
    <ActiveTurnProvider>
      <ActiveTurnBridge bridgeRef={activeTurnRef} />
      <AssistantRuntimeProvider runtime={runtime}>
        <InitialActiveTurnHydrator initialActiveTurn={initialActiveTurn} />
        <ChatContent conversationId={conversationId} isResumingRef={isResumingRef} />
      </AssistantRuntimeProvider>
    </ActiveTurnProvider>
  );
}

function InitialActiveTurnHydrator({
  initialActiveTurn,
}: {
  initialActiveTurn: ActiveTurnState | null;
}) {
  const { hydrateTurn, resetTurn } = useActiveTurnActions();

  useEffect(() => {
    if (initialActiveTurn) {
      hydrateTurn(initialActiveTurn);
      return;
    }
    resetTurn();
  }, [initialActiveTurn, hydrateTurn, resetTurn]);

  return null;
}

interface ChatContentProps {
  conversationId?: string | null;
  isResumingRef?: React.MutableRefObject<boolean>;
}

/**
 * Inner chat content — reads thread state from assistant-ui and active turn
 * from ActiveTurnContext. StreamingAssistantTurn renders the live turn;
 * ThreadPrimitive.Messages renders historical messages.
 */
function ChatContent({ conversationId = null, isResumingRef }: ChatContentProps) {
  const thread = useThread();
  const composerRuntime = useComposerRuntime();
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const [inputText, setInputText] = useState('');

  const { markInteractionAnswered } = useActiveTurnActions();
  const activeTurn = useActiveTurnState();
  const isComposerLocked = selectIsComposerLocked(activeTurn);

  // AskUserCard States
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [interactionError, setInteractionError] = useState<string | null>(null);

  // Auto-scroll to bottom
  useEffect(() => {
    const container = messagesContainerRef.current;
    if (container) {
      container.scrollTo({
        top: container.scrollHeight,
        behavior: 'smooth',
      });
    }
  }, [thread.messages, activeTurn]);

  const handleSend = useCallback(() => {
    if (!inputText.trim() || isComposerLocked) return;
    composerRuntime.setText(inputText.trim());
    composerRuntime.send();
    setInputText('');
  }, [inputText, isComposerLocked, composerRuntime]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleResume = useCallback(async (interactionId: string, answer: unknown) => {
    if (isResumingRef) {
      isResumingRef.current = true;
    }
    const result = await consultationApi.resumeInteraction(
      conversationId!,
      interactionId,
      answer,
    );
    markInteractionAnswered(interactionId);
    if (result.answer_text) {
      composerRuntime.setText(result.answer_text);
      composerRuntime.send();
    }
    return result;
  }, [conversationId, composerRuntime, markInteractionAnswered, isResumingRef]);

  const handleInteractionAnswer = useCallback(
    async (answer: unknown) => {
      const interaction = activeTurn.pendingInteraction;
      if (!interaction || !conversationId) return;

      setIsSubmitting(true);
      setInteractionError(null);
      try {
        await handleResume(interaction.id, answer);
      } catch (err) {
        setInteractionError(err instanceof Error ? err.message : '提交失败，请重试');
      } finally {
        setIsSubmitting(false);
      }
    },
    [activeTurn.pendingInteraction, conversationId, handleResume],
  );

  const hasPendingInteraction =
    activeTurn.pendingInteraction && activeTurn.pendingInteraction.status === 'pending';

  return (
    <ThreadPrimitive.Root className="flex flex-col h-full">
      {/* Messages area */}
      <ThreadPrimitive.Viewport ref={messagesContainerRef} className="flex-1 overflow-y-auto p-4 space-y-4">
        <ThreadPrimitive.Empty>
          <div className="flex items-center justify-center h-full text-gray-400">
            <div className="text-center">
              <p className="text-lg font-medium">开始咨询</p>
              <p className="text-sm mt-1">
                描述你的体态问题，AI 助手会帮助你分析
              </p>
            </div>
          </div>
        </ThreadPrimitive.Empty>

        <ThreadPrimitive.Messages
          components={{
            UserMessage: CustomUserMessage,
            AssistantMessage: CustomAssistantMessage,
          }}
        />

        {/* Streaming assistant turn from ActiveTurnState */}
        <StreamingAssistantTurn
          conversationId={conversationId}
          onResume={handleResume}
        />
      </ThreadPrimitive.Viewport>

      {/* Input area */}
      {hasPendingInteraction ? (
        <div className="p-4 border-t border-[#E5E3DF] bg-[#FBFBFA]">
          <AskUserCard
            question={activeTurn.pendingInteraction!.question}
            onSubmit={handleInteractionAnswer}
            isSubmitting={isSubmitting}
            error={interactionError}
            onRetry={() => setInteractionError(null)}
          />
        </div>
      ) : (
        <div className="flex items-end gap-2 p-4 border-t border-[#E5E3DF] bg-[#FBFBFA]">
          <textarea
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="描述您的症状、体态问题或身体感受..."
            disabled={isComposerLocked}
            rows={1}
            className="flex-1 resize-none rounded-2xl border border-[#D6D3CD] px-4 py-3 text-sm bg-white
                       focus:outline-none focus:ring-2 focus:ring-primary-600 focus:border-transparent
                       disabled:bg-[#F7F5F0] disabled:text-gray-400
                       placeholder:text-gray-400"
            style={{ maxHeight: '120px' }}
          />
          <button
            onClick={handleSend}
            disabled={isComposerLocked || !inputText.trim()}
            className="flex-shrink-0 rounded-full bg-[#CD7B67] px-6 py-3 text-sm font-semibold text-white
                       hover:bg-[#B65E49] focus:outline-none focus:ring-2 focus:ring-primary-600 focus:ring-offset-2
                       disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors duration-300 shadow-sm shadow-[#CD7B67]/15"
          >
            发送
          </button>
        </div>
      )}
    </ThreadPrimitive.Root>
  );
}

function CustomUserMessage() {
  const metadata = useMessage((state) => state.metadata);
  
  // If this message has metadata indicating it's an interaction answer, hide it!
  const isInteractionAnswer = (metadata as any)?.is_interaction_answer;
  if (isInteractionAnswer) {
    return null;
  }

  return (
    <MessagePrimitive.Root className="flex justify-end">
      <div className="max-w-[80%] rounded-[20px] px-4 py-3 bg-primary-700 text-[#FBFBFA] rounded-br-[4px] shadow-sm shadow-[#2a4b3a]/10">
        <div className="whitespace-pre-wrap text-sm leading-relaxed font-medium">
          <MessagePrimitive.Content />
        </div>
      </div>
    </MessagePrimitive.Root>
  );
}

/**
 * Renders a completed assistant message using assistant-ui's built-in markdown renderer.
 * Streaming display is handled by StreamingAssistantTurn separately.
 */
function CustomAssistantMessage() {
  const content = useMessage((state) => state.content) as readonly ThreadAssistantMessagePart[];
  const isLast = useMessage((state) => state.isLast);
  const activeTurn = useActiveTurnState();

  const viewModel = useMemo(
    () => buildAssistantMessagePartsViewModel(content),
    [content],
  );
  const [isRedFlagDismissed, setIsRedFlagDismissed] = useState(false);

  useEffect(() => {
    setIsRedFlagDismissed(false);
  }, [content]);

  const isMessageActive =
    isLast && (activeTurn.status === 'streaming' || activeTurn.status === 'interrupted');

  if (isMessageActive) {
    return null;
  }

  return (
    <MessagePrimitive.Root className="flex justify-start">
      <div className="max-w-[80%] rounded-[20px] px-4 py-3 bg-[#F7F5F0] text-[#1A221E] rounded-bl-[4px] border border-[#E5E3DF]">
        <div className="flex flex-col gap-3">
          <StreamingTurnToolCalls toolCalls={viewModel.toolCalls} />

          <div className="text-sm leading-relaxed font-medium prose-markdown">
            <MessagePrimitive.Content
              components={{
                Text: () => <MarkdownTextPrimitive smooth={false} remarkPlugins={[remarkGfm]} />,
                Source: () => null,
                File: () => null,
                Image: () => null,
                Reasoning: () => null,
                data: { Fallback: () => null },
                tools: { Fallback: () => null },
              }}
            />
          </div>

          {viewModel.citations.length > 0 && (
            <div className="rounded-xl px-3 py-2 bg-[#EEF2EE] border border-[#D4DDD4]">
              <p className="text-xs font-semibold text-[#5A7A64] mb-1">参考知识</p>
              <div className="flex flex-wrap gap-1.5">
                {viewModel.citations.map((c) => (
                  <span
                    key={c.title}
                    className="inline-block rounded-full bg-white px-2.5 py-0.5 text-xs text-[#3D5A47] border border-[#C8D8CC]"
                    title={c.summary || c.snippet || c.content || ''}
                  >
                    {c.title}
                  </span>
                ))}
              </div>
            </div>
          )}

          {viewModel.knowledgeGaps.length > 0 && (
            <div className="rounded-xl px-3 py-2 bg-[#FFF8F0] border border-[#F0D4B0]">
              <div className="flex items-start gap-2">
                <svg className="w-4 h-4 text-[#D4864A] mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div>
                  <p className="text-xs font-semibold text-[#A06030]">知识库提示</p>
                  <p className="text-xs text-[#8B6A4A] mt-0.5">
                    知识库中暂未收录「{viewModel.knowledgeGaps.map((gap) => gap.query).join('」「')}」的专项资料，以下建议仅供参考。
                  </p>
                </div>
              </div>
            </div>
          )}

          {viewModel.redFlag?.has_red_flags && !isRedFlagDismissed && (
            <RedFlagBanner
              redFlags={viewModel.redFlag.flags}
              onAcknowledge={() => setIsRedFlagDismissed(true)}
            />
          )}
        </div>
      </div>
    </MessagePrimitive.Root>
  );
}
