import { AssistantRuntimeProvider, useThread, useComposerRuntime, useThreadRuntime, ThreadPrimitive, MessagePrimitive, useMessage, type ThreadAssistantMessagePart, type ThreadMessageLike } from "@assistant-ui/react";
import { MarkdownTextPrimitive } from "@assistant-ui/react-markdown";
import remarkGfm from "remark-gfm";
import { useAssistantChatRuntime } from "../hooks/useAssistantChatRuntime";
import { ActiveTurnProvider, useActiveTurnActions, useActiveTurnState } from "../context/ActiveTurnContext";
import React, { useRef, useEffect, useState, useCallback, useMemo } from "react";
import type { Citation, ConsultationPhase, ExtractedInfo, HealthFeatures } from "../types/consultation";
import type { ActiveTurnState } from "../runtime/activeTurnReducer";
import { buildAssistantMessagePartsViewModel } from "../runtime/assistantMessagePartsViewModel";
import { selectIsComposerLocked } from "../runtime/activeTurnSelectors";
import { useUploadStore } from "@/stores/uploadStore";
import { consultationAttachmentBuffer } from "../hooks/useAssistantChatRuntime";
import { StreamingAssistantTurn } from "./StreamingAssistantTurn";
import { StreamingTurnToolCalls } from "./StreamingTurnToolCalls";
import { RedFlagBanner } from "./RedFlagBanner";
import { AskUserStatusCard } from "./AskUserStatusCard";
import { useConsultationStore } from "@/stores/consultationStore";

interface AssistantChatPanelProps {
  conversationId: string;
  onConversationCreated?: (id: string) => void;
  initialMessages?: ThreadMessageLike[];
  initialActiveTurn?: ActiveTurnState | null;
  initialExtractedInfo?: ExtractedInfo[];
  onHealthFeaturesUpdate?: (healthFeatures: HealthFeatures) => void;
  onExtractedInfoUpdate?: (info: ExtractedInfo[]) => void;
  onPhaseChange?: (phase: ConsultationPhase) => void;
  onCitation?: (citation: Citation) => void;
  onTitleGenerated?: (title: string) => void;
  onMessagePersisted?: (clientMessageId: string, messageId: string) => void;
  onStreamFinished?: () => void;
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
  onHealthFeaturesUpdate,
  onExtractedInfoUpdate,
  onPhaseChange,
  onCitation,
  onTitleGenerated,
  onMessagePersisted,
  onConversationCreated,
  onStreamFinished,
}: AssistantChatPanelProps) {
  const extractedInfoRef = useRef<ExtractedInfo[]>(_initialExtractedInfo);

  useEffect(() => {
    extractedInfoRef.current = _initialExtractedInfo;
  }, [_initialExtractedInfo]);

  const activeTurnRef = useRef<((state: ActiveTurnState) => void) | null>(null);

  const adapterOptions = useMemo(() => ({
    onConversationCreated: onConversationCreated
      ? (id: string) => {
          console.debug('[SSE] ⓪ AssistantChatPanel → 桥接 onConversationCreated', { id });
          onConversationCreated(id);
        }
      : undefined,
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
    onHealthFeaturesUpdate,
    onPhaseChange: (_from: string, to: string) => {
      onPhaseChange?.(to as ConsultationPhase);
    },
    onCitation,
    onTitleGenerated: onTitleGenerated
      ? (title: string) => {
          console.debug('[SSE] ⓪ AssistantChatPanel → 桥接 onTitleGenerated', { title });
          onTitleGenerated(title);
        }
      : undefined,
    onMessagePersisted,
    onStreamFinished,
    onActiveTurnUpdate: (state: ActiveTurnState) => {
      activeTurnRef.current?.(state);
    },
  }), [onConversationCreated, onExtractedInfoUpdate, onHealthFeaturesUpdate, onPhaseChange, onCitation, onTitleGenerated, onMessagePersisted, onStreamFinished]);

  const { runtime, resumeInteraction } = useAssistantChatRuntime(
    conversationId,
    initialMessages,
    adapterOptions,
  );

  return (
    <ActiveTurnProvider>
      <ActiveTurnBridge bridgeRef={activeTurnRef} />
      <AssistantRuntimeProvider runtime={runtime}>
        <InitialActiveTurnHydrator initialActiveTurn={initialActiveTurn} />
        <ChatContent conversationId={conversationId} onResumeInteraction={resumeInteraction} />
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

interface PendingChatImage {
  uploadId: string;
  previewUrl: string;
  mimeType: string;
  name: string;
}

interface ChatInputAreaProps {
  inputText: string;
  setInputText: (text: string) => void;
  isComposerLocked: boolean;
  handleKeyDown: (e: React.KeyboardEvent) => void;
  handleSend: () => void;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  isCentered?: boolean;
  pendingImages: PendingChatImage[];
  onAddImages: (files: FileList | null) => void;
  onRemoveImage: (uploadId: string) => void;
  isUploadingImage: boolean;
}

function ChatInputArea({
  inputText,
  setInputText,
  isComposerLocked,
  handleKeyDown,
  handleSend,
  textareaRef,
  isCentered = false,
  pendingImages,
  onAddImages,
  onRemoveImage,
  isUploadingImage,
}: ChatInputAreaProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const canSend = Boolean(inputText.trim() || pendingImages.length > 0);

  return (
    <div className="flex w-full flex-col gap-2">
      {pendingImages.length > 0 && (
        <div className="flex flex-wrap gap-2 px-1">
          {pendingImages.map((image) => (
            <div
              key={image.uploadId}
              className="relative h-16 w-16 overflow-hidden rounded-lg border border-[#D6D3CD] bg-white"
            >
              <img src={image.previewUrl} alt={image.name} className="h-full w-full object-cover" />
              <button
                type="button"
                onClick={() => onRemoveImage(image.uploadId)}
                disabled={isComposerLocked}
                className="absolute right-0.5 top-0.5 rounded-full bg-black/60 px-1.5 text-[10px] text-white"
                aria-label="移除图片"
              >
                ×
              </button>
            </div>
          ))}
        </div>
      )}
      <div className="flex items-end gap-2 w-full">
        <input
          ref={fileInputRef}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          multiple
          className="hidden"
          onChange={(e) => {
            onAddImages(e.target.files);
            e.target.value = '';
          }}
        />
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={isComposerLocked || isUploadingImage || pendingImages.length >= 3}
          className="flex-shrink-0 rounded-full border border-[#D6D3CD] bg-white px-3 py-3.5 text-sm text-[#5D6B63]
                     hover:bg-[#F7F5F0] disabled:cursor-not-allowed disabled:opacity-50"
          title="上传体态/症状照片"
        >
          {isUploadingImage ? '…' : '图片'}
        </button>
        <textarea
          ref={textareaRef}
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="描述您的症状、体态问题或身体感受，也可附上照片..."
          disabled={isComposerLocked}
          rows={1}
          className={`flex-1 resize-none rounded-2xl border border-[#D6D3CD] px-4 py-3.5 text-sm bg-white
                     focus:outline-none focus:ring-2 focus:ring-primary-600 focus:border-transparent
                     disabled:bg-[#F7F5F0] disabled:text-gray-400
                     placeholder:text-gray-400 ${isCentered ? "shadow-md" : "shadow-sm"}`}
          style={{ maxHeight: '150px' }}
        />
        <button
          onClick={handleSend}
          disabled={isComposerLocked || !canSend || isUploadingImage}
          className="flex-shrink-0 rounded-full bg-[#CD7B67] px-6 py-3.5 text-sm font-semibold text-white
                     hover:bg-[#B65E49] focus:outline-none focus:ring-2 focus:ring-primary-600 focus:ring-offset-2
                     disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors duration-300 shadow-sm shadow-[#CD7B67]/15"
        >
          发送
        </button>
      </div>
    </div>
  );
}

interface ChatContentProps {
  conversationId: string;
  onResumeInteraction: ReturnType<typeof useAssistantChatRuntime>['resumeInteraction'];
}

/**
 * Inner chat content — reads thread state from assistant-ui and active turn
 * from ActiveTurnContext. StreamingAssistantTurn renders the live turn;
 * ThreadPrimitive.Messages renders historical messages.
 */
function ChatContent({ conversationId, onResumeInteraction }: ChatContentProps) {
  const thread = useThread();
  const threadRuntime = useThreadRuntime();
  const composerRuntime = useComposerRuntime();
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const draftMessage = useConsultationStore((state) => state.draftMessage);
  const setDraftMessage = useConsultationStore((state) => state.setDraftMessage);
  const clearDraftMessage = useConsultationStore((state) => state.clearDraftMessage);

  const [inputText, setInputText] = useState(() => {
    return conversationId === 'new' ? draftMessage : '';
  });
  const [pendingImages, setPendingImages] = useState<PendingChatImage[]>([]);
  const [isUploadingImage, setIsUploadingImage] = useState(false);
  const uploadFile = useUploadStore((s) => s.uploadFile);

  // Sync store draft when user types, only for new conversations
  useEffect(() => {
    if (conversationId === 'new') {
      setDraftMessage(inputText);
    }
  }, [inputText, conversationId, setDraftMessage]);

  // When conversationId changes, if it becomes 'new', load draft. Otherwise clear.
  useEffect(() => {
    if (conversationId === 'new') {
      setInputText(draftMessage);
    } else {
      setInputText('');
    }
  }, [conversationId, draftMessage]);

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

  const handleAddImages = useCallback(
    async (files: FileList | null) => {
      if (!files || files.length === 0 || isComposerLocked) return;
      const remaining = Math.max(0, 3 - pendingImages.length);
      const selected = Array.from(files).slice(0, remaining);
      if (selected.length === 0) return;
      setIsUploadingImage(true);
      try {
        const uploaded: PendingChatImage[] = [];
        for (const file of selected) {
          const record = await uploadFile(file, 'consultation_photo');
          uploaded.push({
            uploadId: record.id,
            previewUrl: URL.createObjectURL(file),
            mimeType: record.mime_type || file.type,
            name: file.name,
          });
        }
        setPendingImages((prev) => [...prev, ...uploaded].slice(0, 3));
      } catch (err) {
        console.error('consultation image upload failed', err);
      } finally {
        setIsUploadingImage(false);
      }
    },
    [isComposerLocked, pendingImages.length, uploadFile],
  );

  const handleRemoveImage = useCallback((uploadId: string) => {
    setPendingImages((prev) => {
      const target = prev.find((item) => item.uploadId === uploadId);
      if (target?.previewUrl.startsWith('blob:')) {
        URL.revokeObjectURL(target.previewUrl);
      }
      return prev.filter((item) => item.uploadId !== uploadId);
    });
  }, []);

  const handleSend = useCallback(() => {
    if (isComposerLocked || isUploadingImage) return;
    if (!inputText.trim() && pendingImages.length === 0) return;

    consultationAttachmentBuffer.next = pendingImages.map((image) => ({
      uploadId: image.uploadId,
      mimeType: image.mimeType,
      imageUrl: image.previewUrl.startsWith('blob:') ? undefined : image.previewUrl,
    }));

    const text =
      inputText.trim() ||
      (pendingImages.length > 0
        ? '请结合我附上的照片，分析与体态/不适相关的可见信息，并给出谨慎建议。'
        : '');
    composerRuntime.setText(text);
    composerRuntime.send();
    if (conversationId === 'new') {
      clearDraftMessage();
    }
    setInputText('');
    setPendingImages((prev) => {
      for (const image of prev) {
        if (image.previewUrl.startsWith('blob:')) URL.revokeObjectURL(image.previewUrl);
      }
      return [];
    });
  }, [
    inputText,
    isComposerLocked,
    isUploadingImage,
    pendingImages,
    composerRuntime,
    conversationId,
    clearDraftMessage,
  ]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleResume = useCallback(async (interactionId: string, answer: unknown) => {
    markInteractionAnswered(interactionId);
    onResumeInteraction(threadRuntime, interactionId, answer);
  }, [markInteractionAnswered, onResumeInteraction, threadRuntime]);

  const handleInteractionAnswer = useCallback(
    async (answer: unknown) => {
      const interaction = activeTurn.pendingInteraction;
      if (!interaction) return;

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
    [activeTurn.pendingInteraction, handleResume],
  );

  const hasPendingInteraction =
    activeTurn.pendingInteraction && activeTurn.pendingInteraction.status === 'pending';

  // Determine if the conversation has no user or assistant messages
  const isEmptyConversation =
    thread.messages.filter((m) => m.role === 'user' || m.role === 'assistant').length === 0;

  // Render centered Layout for new/empty conversations
  if (isEmptyConversation && !hasPendingInteraction) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center p-6 bg-white h-full relative overflow-y-auto">
        <div className="w-full max-w-3xl text-center space-y-6 -translate-y-12">
          {/* Posture/Health logo/decoration */}
          <div className="flex justify-center">
            <div className="w-16 h-16 rounded-2xl bg-gradient-to-tr from-primary-100 to-primary-50 flex items-center justify-center text-primary-700 shadow-md shadow-primary-700/5 border border-primary-200/50">
              <svg className="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
            </div>
          </div>

          <div className="space-y-3">
            {/* Title */}
            <h2 className="text-2xl md:text-3xl font-display font-bold text-[#1A221E] tracking-tight">
              开始一次新的健康咨询
            </h2>
            
            {/* Subtitle */}
            <p className="text-sm md:text-base text-[#4A554E] max-w-lg mx-auto leading-relaxed font-medium">
              描述你的问题，我会帮你逐步分析并给出建议。
            </p>
          </div>

          {/* Input Box Centered */}
          <div className="w-full text-left max-w-3xl mx-auto">
            <ChatInputArea
              inputText={inputText}
              setInputText={setInputText}
              isComposerLocked={isComposerLocked}
              handleKeyDown={handleKeyDown}
              handleSend={handleSend}
              textareaRef={textareaRef}
              isCentered={true}
              pendingImages={pendingImages}
              onAddImages={handleAddImages}
              onRemoveImage={handleRemoveImage}
              isUploadingImage={isUploadingImage}
            />
          </div>
        </div>
      </div>
    );
  }

  // Render normal Chatting Layout with messages list and bottom input box
  return (
    <ThreadPrimitive.Root className="flex flex-col h-full bg-white">
      {/* Messages area */}
      <ThreadPrimitive.Viewport ref={messagesContainerRef} className="flex-1 overflow-y-auto p-4 space-y-4">
        <ThreadPrimitive.Empty>
          <div className="hidden" />
        </ThreadPrimitive.Empty>

        <ThreadPrimitive.Messages
          components={{
            UserMessage: CustomUserMessage,
            AssistantMessage: CustomAssistantMessage,
          }}
        />

        {/* Streaming assistant turn from ActiveTurnState */}
        <StreamingAssistantTurn
          onInteractionSubmit={hasPendingInteraction ? handleInteractionAnswer : undefined}
          isInteractionSubmitting={isSubmitting}
          interactionError={interactionError}
          onInteractionRetry={() => setInteractionError(null)}
        />
      </ThreadPrimitive.Viewport>

      {/* Input area */}
      <div className="p-4 border-t border-[#E5E3DF] bg-[#FBFBFA]">
        {hasPendingInteraction && (
          <p className="mb-3 text-xs font-medium text-[#5F6F86]">
            正在等待你完成上方追问，当前输入框暂时锁定。
          </p>
        )}
        <ChatInputArea
              inputText={inputText}
              setInputText={setInputText}
              isComposerLocked={isComposerLocked}
              handleKeyDown={handleKeyDown}
              handleSend={handleSend}
              textareaRef={textareaRef}
              pendingImages={pendingImages}
              onAddImages={handleAddImages}
              onRemoveImage={handleRemoveImage}
              isUploadingImage={isUploadingImage}
            />
      </div>
    </ThreadPrimitive.Root>
  );
}

function CustomUserMessage() {
  const metadata = useMessage((state) => state.metadata) as
    | { custom?: { is_interaction_answer?: boolean } }
    | undefined;

  // If this message has metadata indicating it's an interaction answer, hide it!
  const isInteractionAnswer = metadata?.custom?.is_interaction_answer === true;
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
  const metadata = useMessage((state) => state.metadata) as
    | { custom?: { interaction_history?: boolean; interaction?: import('../types/consultation').InteractionHistoryItem } }
    | undefined;
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

  const historicalInteraction = metadata?.custom?.interaction;
  if (metadata?.custom?.interaction_history && historicalInteraction) {
    return <AskUserStatusCard interaction={historicalInteraction} />;
  }

  if (!viewModel.hasRenderableContent) {
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
                Image: (props) => {
                  const src =
                    (props as { image?: string }).image ||
                    (props as { src?: string }).src ||
                    '';
                  if (!src) return null;
                  return (
                    <img
                      src={src}
                      alt="用户上传"
                      className="mt-2 max-h-48 rounded-lg border border-[#E5E3DF] object-contain"
                    />
                  );
                },
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
