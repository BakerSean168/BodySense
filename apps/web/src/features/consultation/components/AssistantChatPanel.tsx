import {
  AssistantRuntimeProvider,
  useAui,
  useAuiState,
  ThreadPrimitive,
  MessagePrimitive,
  type ThreadAssistantMessagePart,
  type ThreadMessageLike,
} from "@assistant-ui/react";
import { MarkdownTextPrimitive } from "@assistant-ui/react-markdown";
import { ArrowUp, ImagePlus, Plus, X } from "lucide-react";
import remarkGfm from "remark-gfm";
import { useAssistantChatRuntime } from "../hooks/useAssistantChatRuntime";
import {
  ActiveTurnProvider,
  useActiveTurnActions,
  useActiveTurnState,
} from "../context/ActiveTurnContext";
import React, {
  useRef,
  useEffect,
  useState,
  useCallback,
  useMemo,
} from "react";
import type {
  Citation,
  ConsultationPhase,
  ExtractedInfo,
} from "../types/consultation";
import {
  EXECUTION_LOST_USER_MESSAGE,
  type ActiveTurnState,
} from "../runtime/activeTurnReducer";
import { buildAssistantMessagePartsViewModel } from "../runtime/assistantMessagePartsViewModel";
import {
  selectIsComposerLocked,
  shouldApplyInitialActiveTurn,
} from "../runtime/activeTurnSelectors";
import { useUploadStore } from "@/stores/uploadStore";
import { consultationAttachmentBuffer } from "../hooks/useAssistantChatRuntime";
import { consultationApi } from "../services/consultationService";
import { StreamingAssistantTurn } from "./StreamingAssistantTurn";
import { FailedRunStatusCard } from "./FailedRunStatusCard";
import { CancelledRunStatusCard } from "./CancelledRunStatusCard";
import { StreamingTurnToolCalls } from "./StreamingTurnToolCalls";
import { RedFlagBanner } from "./RedFlagBanner";
import { AskUserStatusCard } from "./AskUserStatusCard";
import { useConsultationStore } from "@/stores/consultationStore";
import {
  Message,
  MessageContent,
  MessageResponse,
} from "@/components/ai-elements/message";
import {
  Source,
  SourceList,
  Sources,
} from "@/components/ai-elements/sources";

interface AssistantChatPanelProps {
  conversationId: string;
  onConversationCreated?: (id: string) => void;
  initialMessages?: ThreadMessageLike[];
  initialActiveTurn?: ActiveTurnState | null;
  initialExtractedInfo?: ExtractedInfo[];
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
function ActiveTurnBridge({
  bridgeRef,
}: {
  bridgeRef: React.MutableRefObject<((state: ActiveTurnState) => void) | null>;
}) {
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

  const adapterOptions = useMemo(
    () => ({
      onConversationCreated: onConversationCreated
        ? (id: string) => {
            console.debug(
              "[SSE] ⓪ AssistantChatPanel → 桥接 onConversationCreated",
              { id },
            );
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
      onPhaseChange: (_from: string, to: string) => {
        onPhaseChange?.(to as ConsultationPhase);
      },
      onCitation,
      onTitleGenerated: onTitleGenerated
        ? (title: string) => {
            console.debug(
              "[SSE] ⓪ AssistantChatPanel → 桥接 onTitleGenerated",
              { title },
            );
            onTitleGenerated(title);
          }
        : undefined,
      onMessagePersisted,
      onStreamFinished,
      onActiveTurnUpdate: (state: ActiveTurnState) => {
        activeTurnRef.current?.(state);
      },
    }),
    [
      onConversationCreated,
      onExtractedInfoUpdate,
      onPhaseChange,
      onCitation,
      onTitleGenerated,
      onMessagePersisted,
      onStreamFinished,
    ],
  );

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
        <ChatContent
          conversationId={conversationId}
          onResumeInteraction={resumeInteraction}
        />
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
  const currentTurn = useActiveTurnState();
  const currentTurnRef = useRef(currentTurn);
  currentTurnRef.current = currentTurn;

  useEffect(() => {
    const current = currentTurnRef.current;
    if (!shouldApplyInitialActiveTurn(current, initialActiveTurn)) {
      return;
    }
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
  pendingImages,
  onAddImages,
  onRemoveImage,
  isUploadingImage,
}: ChatInputAreaProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const canSend = Boolean(inputText.trim() || pendingImages.length > 0);

  return (
    <div className="w-full">
      <input
        ref={fileInputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        multiple
        className="hidden"
        onChange={(event) => {
          onAddImages(event.target.files);
          event.target.value = "";
        }}
      />

      <div className="rounded-[24px] border border-white/[0.08] bg-[#2a2a2a] p-2 shadow-[0_12px_34px_rgba(0,0,0,0.28)] transition-[border-color,background-color,box-shadow] duration-200 focus-within:border-white/[0.14] focus-within:bg-[#2d2d2d] focus-within:shadow-[0_16px_38px_rgba(0,0,0,0.34)]">
        {pendingImages.length > 0 ? (
          <div className="flex flex-wrap gap-2 px-1 pb-2 pt-1">
            {pendingImages.map((image) => (
              <div
                key={image.uploadId}
                className="relative size-16 overflow-hidden rounded-xl border border-white/10 bg-black/20"
              >
                <img
                  src={image.previewUrl}
                  alt={image.name}
                  className="h-full w-full object-cover"
                />
                <button
                  type="button"
                  onClick={() => onRemoveImage(image.uploadId)}
                  disabled={isComposerLocked}
                  className="absolute right-1 top-1 inline-flex size-5 items-center justify-center rounded-full bg-black/70 text-white/80 transition-colors hover:bg-black hover:text-white disabled:opacity-40"
                  aria-label="移除图片"
                >
                  <X className="size-3" aria-hidden="true" />
                </button>
              </div>
            ))}
          </div>
        ) : null}

        <textarea
          ref={textareaRef}
          value={inputText}
          onChange={(event) => setInputText(event.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="和 BodySense 说说你的身体感受…"
          disabled={isComposerLocked}
          rows={1}
          className="max-h-[140px] min-h-11 w-full resize-none bg-transparent px-3 py-2 text-[14px] leading-6 text-[#f2f2f2] outline-none placeholder:text-white/35 disabled:cursor-not-allowed disabled:text-white/35"
        />

        <div className="flex items-center justify-between gap-2 px-1 pb-0.5">
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={
              isComposerLocked || isUploadingImage || pendingImages.length >= 3
            }
            className="inline-flex size-8 items-center justify-center rounded-full text-white/55 transition-colors hover:bg-white/[0.07] hover:text-white/85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#75d5a7]/45 disabled:cursor-not-allowed disabled:opacity-35"
            title="添加图片"
            aria-label="添加图片"
          >
            {isUploadingImage ? (
              <span className="size-3.5 animate-spin rounded-full border border-white/35 border-t-white/80" />
            ) : pendingImages.length > 0 ? (
              <ImagePlus className="size-4" aria-hidden="true" />
            ) : (
              <Plus className="size-4" aria-hidden="true" />
            )}
          </button>

          <button
            type="button"
            onClick={handleSend}
            disabled={isComposerLocked || !canSend || isUploadingImage}
            className="inline-flex size-8 items-center justify-center rounded-full bg-[#f2f2f2] text-[#171717] transition-[transform,background-color,opacity] duration-150 hover:bg-white active:scale-95 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#75d5a7]/50 disabled:cursor-not-allowed disabled:bg-white/10 disabled:text-white/25"
            aria-label="发送"
            title="发送"
          >
            <ArrowUp className="size-4" strokeWidth={2.3} aria-hidden="true" />
          </button>
        </div>
      </div>
    </div>
  );
}

interface ChatContentProps {
  conversationId: string;
  onResumeInteraction: ReturnType<
    typeof useAssistantChatRuntime
  >["resumeInteraction"];
}

/**
 * Inner chat content — reads thread state from assistant-ui and active turn
 * from ActiveTurnContext. StreamingAssistantTurn renders the live turn;
 * ThreadPrimitive.Messages renders historical messages.
 */
function ChatContent({
  conversationId,
  onResumeInteraction,
}: ChatContentProps) {
  const aui = useAui();
  const threadMessages = useAuiState((state) => state.thread.messages);
  const threadRuntime = aui.thread;
  const composerRuntime = aui.composer;
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const draftMessage = useConsultationStore((state) => state.draftMessage);
  const setDraftMessage = useConsultationStore(
    (state) => state.setDraftMessage,
  );
  const clearDraftMessage = useConsultationStore(
    (state) => state.clearDraftMessage,
  );

  const [inputText, setInputText] = useState(() => {
    return conversationId === "new" ? draftMessage : "";
  });
  const [pendingImages, setPendingImages] = useState<PendingChatImage[]>([]);
  const [isUploadingImage, setIsUploadingImage] = useState(false);
  const uploadFile = useUploadStore((s) => s.uploadFile);

  // Sync store draft when user types, only for new conversations
  useEffect(() => {
    if (conversationId === "new") {
      setDraftMessage(inputText);
    }
  }, [inputText, conversationId, setDraftMessage]);

  // When conversationId changes, if it becomes 'new', load draft. Otherwise clear.
  useEffect(() => {
    if (conversationId === "new") {
      setInputText(draftMessage);
    } else {
      setInputText("");
    }
  }, [conversationId, draftMessage]);

  const { markInteractionAnswered, hydrateTurn } = useActiveTurnActions();
  const activeTurn = useActiveTurnState();
  const isComposerLocked = selectIsComposerLocked(activeTurn);

  // AskUserCard States
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [interactionError, setInteractionError] = useState<string | null>(null);

  const [isCancellingRun, setIsCancellingRun] = useState(false);
  const [cancelRunError, setCancelRunError] = useState<string | null>(null);

  // Auto-scroll to bottom
  useEffect(() => {
    const container = messagesContainerRef.current;
    if (container) {
      container.scrollTo({
        top: container.scrollHeight,
        behavior: "smooth",
      });
    }
  }, [threadMessages, activeTurn]);

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
          const record = await uploadFile(file, "consultation_photo");
          uploaded.push({
            uploadId: record.id,
            previewUrl: URL.createObjectURL(file),
            mimeType: record.mime_type || file.type,
            name: file.name,
          });
        }
        setPendingImages((prev) => [...prev, ...uploaded].slice(0, 3));
      } catch (err) {
        console.error("consultation image upload failed", err);
      } finally {
        setIsUploadingImage(false);
      }
    },
    [isComposerLocked, pendingImages.length, uploadFile],
  );

  const handleRemoveImage = useCallback((uploadId: string) => {
    setPendingImages((prev) => {
      const target = prev.find((item) => item.uploadId === uploadId);
      if (target?.previewUrl.startsWith("blob:")) {
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
      imageUrl: image.previewUrl.startsWith("blob:")
        ? undefined
        : image.previewUrl,
    }));

    const text =
      inputText.trim() ||
      (pendingImages.length > 0
        ? "请结合我附上的照片，分析与体态/不适相关的可见信息，并给出谨慎建议。"
        : "");
    composerRuntime.setText(text);
    composerRuntime.send();
    if (conversationId === "new") {
      clearDraftMessage();
    }
    setInputText("");
    setPendingImages((prev) => {
      for (const image of prev) {
        if (image.previewUrl.startsWith("blob:"))
          URL.revokeObjectURL(image.previewUrl);
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
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleResume = useCallback(
    async (interactionId: string, answer: unknown) => {
      markInteractionAnswered(interactionId);
      onResumeInteraction(threadRuntime, interactionId, answer);
    },
    [markInteractionAnswered, onResumeInteraction, threadRuntime],
  );

  const handleInteractionAnswer = useCallback(
    async (answer: unknown) => {
      const interaction = activeTurn.pendingInteraction;
      if (!interaction) return;

      setIsSubmitting(true);
      setInteractionError(null);
      try {
        await handleResume(interaction.id, answer);
      } catch (err) {
        setInteractionError(
          err instanceof Error ? err.message : "提交失败，请重试",
        );
      } finally {
        setIsSubmitting(false);
      }
    },
    [activeTurn.pendingInteraction, handleResume],
  );

  const hasPendingInteraction =
    activeTurn.pendingInteraction &&
    activeTurn.pendingInteraction.status === "pending";

  const canCancelRun =
    Boolean(activeTurn.runId) &&
    (activeTurn.status === "streaming" || activeTurn.status === "interrupted");

  const handleCancelRun = useCallback(async () => {
    if (!activeTurn.runId || !canCancelRun || isCancellingRun) return;
    setIsCancellingRun(true);
    setCancelRunError(null);
    try {
      await consultationApi.cancelRun(activeTurn.runId);
      hydrateTurn({
        ...activeTurn,
        status: "cancelled",
        pendingInteraction: null,
        error: "cancelled_by_user",
      });
    } catch (err) {
      setCancelRunError(
        err instanceof Error ? err.message : "取消失败，请稍后重试",
      );
    } finally {
      setIsCancellingRun(false);
    }
  }, [activeTurn, canCancelRun, hydrateTurn, isCancellingRun]);

  // Determine if the conversation has no user or assistant messages
  const isEmptyConversation =
    threadMessages.filter(
      (message) => message.role === "user" || message.role === "assistant",
    ).length === 0;

  return (
    <ThreadPrimitive.Root className="flex h-full min-h-0 flex-col bg-[#171717] text-[#ececec]">
      <ThreadPrimitive.Viewport
        ref={messagesContainerRef}
        className="custom-scrollbar min-h-0 flex-1 overflow-y-auto"
      >
        <div className="mx-auto flex min-h-full w-full max-w-[760px] flex-col gap-6 px-5 py-7 sm:px-6">
          <ThreadPrimitive.Empty>
            <div
              className={
                isEmptyConversation
                  ? "flex flex-1 flex-col items-center justify-center px-4 pb-10 text-center"
                  : "hidden"
              }
            >
              <div className="mb-5 flex size-11 items-center justify-center rounded-[16px] border border-white/[0.08] bg-white/[0.035] text-[#83d4aa] shadow-[0_8px_24px_rgba(0,0,0,0.18)]">
                <svg
                  className="size-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={1.6}
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M3.75 12h3l1.5-4.5 3 9 2.25-6 1.5 1.5h5.25"
                  />
                </svg>
              </div>
              <h2 className="text-xl font-medium tracking-[-0.02em] text-white/90">
                有什么身体变化想告诉我？
              </h2>
              <p className="mt-2 max-w-sm text-sm leading-6 text-white/42">
                描述不适、体态或最近的变化。BodySense 会把对话与右侧身体状态持续关联。
              </p>
            </div>
          </ThreadPrimitive.Empty>

          <ThreadPrimitive.Messages
            components={{
              UserMessage: CustomUserMessage,
              AssistantMessage: CustomAssistantMessage,
            }}
          />

          <StreamingAssistantTurn
            onInteractionSubmit={
              hasPendingInteraction ? handleInteractionAnswer : undefined
            }
            isInteractionSubmitting={isSubmitting}
            interactionError={interactionError}
            onInteractionRetry={() => setInteractionError(null)}
          />
        </div>
      </ThreadPrimitive.Viewport>

      <div className="shrink-0 bg-[linear-gradient(to_top,#171717_78%,rgba(23,23,23,0))] px-4 pb-4 pt-7 sm:px-5">
        <div className="mx-auto w-full max-w-[760px]">
          {canCancelRun ? (
            <div className="mb-2.5 flex items-center justify-between gap-3 rounded-xl border border-white/[0.07] bg-white/[0.035] px-3 py-2">
              <p className="text-xs leading-5 text-white/48">
                {activeTurn.status === "interrupted"
                  ? "正在等待你的补充信息。"
                  : "BodySense 正在处理这条消息。"}
              </p>
              <button
                type="button"
                onClick={() => void handleCancelRun()}
                disabled={isCancellingRun}
                className="shrink-0 rounded-lg px-2.5 py-1 text-xs font-medium text-red-300 transition-colors hover:bg-red-400/10 hover:text-red-200 disabled:cursor-not-allowed disabled:opacity-40"
              >
                {isCancellingRun ? "取消中…" : "停止"}
              </button>
            </div>
          ) : null}
          {cancelRunError ? (
            <p className="mb-2.5 px-1 text-xs font-medium text-red-300">
              {cancelRunError}
            </p>
          ) : null}
          {hasPendingInteraction ? (
            <p className="mb-2.5 px-1 text-xs text-white/42">
              完成上方追问后可以继续输入。
            </p>
          ) : null}
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
      </div>
    </ThreadPrimitive.Root>
  );
}
function CustomUserMessage() {
  const metadata = useAuiState((state) => state.message.metadata) as
    { custom?: { is_interaction_answer?: boolean } } | undefined;

  // If this message has metadata indicating it's an interaction answer, hide it!
  const isInteractionAnswer = metadata?.custom?.is_interaction_answer === true;
  if (isInteractionAnswer) {
    return null;
  }

  return (
    <MessagePrimitive.Root className="w-full">
      <Message from="user">
        <MessageContent>
          <div className="whitespace-pre-wrap">
            <MessagePrimitive.Content />
          </div>
        </MessageContent>
      </Message>
    </MessagePrimitive.Root>
  );
}

/**
 * Renders a completed assistant message using assistant-ui's built-in markdown renderer.
 * Streaming display is handled by StreamingAssistantTurn separately.
 */
function CustomAssistantMessage() {
  const content = useAuiState(
    (state) => state.message.content,
  ) as readonly ThreadAssistantMessagePart[];
  const isLast = useAuiState((state) => state.message.isLast);
  const metadata = useAuiState((state) => state.message.metadata) as
    | {
        custom?: {
          interaction_history?: boolean;
          interaction?: import("../types/consultation").InteractionHistoryItem;
          consultation_status?: string;
          consultation_error?: { code?: string; message?: string } | null;
        };
      }
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
    isLast &&
    (activeTurn.status === "streaming" || activeTurn.status === "interrupted");

  if (isMessageActive) {
    return null;
  }

  const historicalInteraction = metadata?.custom?.interaction;
  if (metadata?.custom?.interaction_history && historicalInteraction) {
    return <AskUserStatusCard interaction={historicalInteraction} />;
  }

  if (
    metadata?.custom?.consultation_status === "failed" &&
    metadata.custom.consultation_error?.code === "execution_lost"
  ) {
    return <FailedRunStatusCard message={EXECUTION_LOST_USER_MESSAGE} />;
  }

  if (metadata?.custom?.consultation_status === "aborted") {
    return <CancelledRunStatusCard />;
  }

  if (!viewModel.hasRenderableContent) {
    return null;
  }

  return (
    <MessagePrimitive.Root className="w-full">
      <Message from="assistant">
        <MessageContent className="gap-3">
          <StreamingTurnToolCalls toolCalls={viewModel.toolCalls} />

          <MessageResponse className="conversation-prose prose-markdown">
            <MessagePrimitive.Content
              components={{
                Text: () => (
                  <MarkdownTextPrimitive
                    smooth={false}
                    remarkPlugins={[remarkGfm]}
                  />
                ),
                Source: () => null,
                File: () => null,
                Image: (props) => {
                  const src =
                    (props as { image?: string }).image ||
                    (props as { src?: string }).src ||
                    "";
                  if (!src) return null;
                  return (
                    <img
                      src={src}
                      alt="用户上传"
                      className="mt-2 max-h-48 rounded-xl border border-white/10 object-contain"
                    />
                  );
                },
                Reasoning: () => null,
                data: { Fallback: () => null },
                tools: { Fallback: () => null },
              }}
            />
          </MessageResponse>

          <Sources count={viewModel.citations.length}>
            <SourceList>
              {viewModel.citations.map((citation) => (
                <Source
                  key={citation.title}
                  title={
                    citation.summary ||
                    citation.snippet ||
                    citation.content ||
                    ""
                  }
                >
                  <p className="font-medium text-white/78">{citation.title}</p>
                  {citation.summary || citation.snippet ? (
                    <p className="mt-0.5 line-clamp-2 text-white/42">
                      {citation.summary || citation.snippet}
                    </p>
                  ) : null}
                </Source>
              ))}
            </SourceList>
          </Sources>

          {viewModel.knowledgeGaps.length > 0 ? (
            <div className="rounded-xl border border-amber-300/10 bg-amber-300/[0.045] px-3 py-2.5">
              <p className="text-xs font-medium text-amber-200/75">
                还缺少一些专项资料
              </p>
              <p className="mt-1 text-xs leading-5 text-amber-100/48">
                当前知识库暂未覆盖「
                {viewModel.knowledgeGaps.map((gap) => gap.query).join("」「")}
                」，相关建议会保持更谨慎。
              </p>
            </div>
          ) : null}

          {viewModel.redFlag?.has_red_flags && !isRedFlagDismissed ? (
            <RedFlagBanner
              redFlags={viewModel.redFlag.flags}
              onAcknowledge={() => setIsRedFlagDismissed(true)}
            />
          ) : null}
        </MessageContent>
      </Message>
    </MessagePrimitive.Root>
  );
}
