import React, { useRef, useEffect, useState, useCallback } from "react";
import { useAui, useAuiState, ThreadPrimitive } from "@assistant-ui/react";
import {
  useAssistantChatRuntime,
  consultationAttachmentBuffer,
  consultationSpatialContextBuffer,
} from "../../hooks/useAssistantChatRuntime";
import {
  useActiveTurnActions,
  useActiveTurnState,
} from "../../context/ActiveTurnContext";
import { selectIsComposerLocked } from "../../runtime/activeTurnSelectors";
import type {
  ConsultationSpatialContext,
  PendingInteraction,
} from "../../types/consultation";
import { useUploadStore } from "@/stores/uploadStore";
import { useConsultationStore } from "@/stores/consultationStore";
import { consultationApi } from "../../services/consultationService";
import { BODY_STATE_SUGGESTION_DECLINE } from "../BodyStateSuggestionCard";
import { AskUserCard } from "../AskUserCard";
import { StreamingAssistantTurn } from "../StreamingAssistantTurn";
import { ChatInputArea, type PendingChatImage } from "./ChatComposer";
import { CustomAssistantMessage, CustomUserMessage } from "./ChatMessages";

interface ChatContentProps {
  conversationId: string;
  onResumeInteraction: ReturnType<
    typeof useAssistantChatRuntime
  >["resumeInteraction"];
  spatialContext?: ConsultationSpatialContext | null;
  onClearSpatialContext?: () => void;
  composerFocusKey?: number;
}

/**
 * Inner chat content — reads thread state from assistant-ui and active turn
 * from ActiveTurnContext. StreamingAssistantTurn renders the live turn;
 * ThreadPrimitive.Messages renders historical messages.
 */
export function ChatContent({
  conversationId,
  onResumeInteraction,
  spatialContext,
  onClearSpatialContext,
  composerFocusKey = 0,
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

  const { hydrateTurn } = useActiveTurnActions();
  const activeTurn = useActiveTurnState();
  const isComposerLocked = selectIsComposerLocked(activeTurn);

  // AskUserCard States
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [interactionError, setInteractionError] = useState<string | null>(null);
  const lastAnswer = useRef<{
    interactionId: string;
    answer: unknown;
    editQuote?: string;
  } | null>(null);

  useEffect(() => {
    lastAnswer.current = null;
    setInteractionError(null);
  }, [activeTurn.pendingInteraction?.id]);

  const [isCancellingRun, setIsCancellingRun] = useState(false);
  const [cancelRunError, setCancelRunError] = useState<string | null>(null);

  useEffect(() => {
    if (composerFocusKey > 0 && !isComposerLocked) {
      textareaRef.current?.focus({ preventScroll: true });
    }
  }, [composerFocusKey, isComposerLocked]);

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

    consultationSpatialContextBuffer.next = spatialContext ?? null;
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
    onClearSpatialContext?.();
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
    onClearSpatialContext,
    spatialContext,
  ]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleResume = useCallback(
    async (interaction: PendingInteraction, answer: unknown) => {
      await onResumeInteraction(
        threadRuntime,
        interaction.id,
        answer,
        interaction,
      );
    },
    [onResumeInteraction, threadRuntime],
  );

  const handleInteractionAnswer = useCallback(
    async (answer: unknown) => {
      const interaction = activeTurn.pendingInteraction;
      if (!interaction || isSubmitting) return false;
      lastAnswer.current = { interactionId: interaction.id, answer };

      setIsSubmitting(true);
      setInteractionError(null);
      try {
        await handleResume(interaction, answer);
        return true;
      } catch (err) {
        setInteractionError(
          err instanceof Error ? err.message : "提交失败，请重试",
        );
        return false;
      } finally {
        setIsSubmitting(false);
      }
    },
    [activeTurn.pendingInteraction, handleResume, isSubmitting],
  );

  // "改一改再说": decline the suggestion (Go writes nothing), then hand the
  // user's own words back to the composer so they can restate them.
  const handleSuggestionEdit = useCallback(
    async (quote: string) => {
      const declined = await handleInteractionAnswer(
        BODY_STATE_SUGGESTION_DECLINE,
      );
      if (!declined) {
        if (lastAnswer.current) lastAnswer.current.editQuote = quote;
        return;
      }
      setInputText(quote);
      requestAnimationFrame(() =>
        textareaRef.current?.focus({ preventScroll: true }),
      );
    },
    [handleInteractionAnswer],
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
    <ThreadPrimitive.Root className="chat-thread flex h-full min-h-0 flex-col bg-[#171717] text-[#ececec]">
      <ThreadPrimitive.Viewport
        ref={messagesContainerRef}
        className="chat-thread-viewport custom-scrollbar min-h-0 flex-1 overflow-y-auto"
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
              <div className="chat-empty-icon mb-5 flex size-11 items-center justify-center rounded-[16px] border border-white/[0.08] bg-white/[0.035] text-[#83d4aa] shadow-[0_8px_24px_rgba(0,0,0,0.18)]">
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
              <h2 className="chat-empty-title text-xl font-medium tracking-[-0.02em] text-white/90">
                有什么身体变化想告诉我？
              </h2>
              <p className="chat-empty-text mt-2 max-w-sm text-sm leading-6 text-white/42">
                描述不适、体态或最近的变化。
              </p>
            </div>
          </ThreadPrimitive.Empty>

          <ThreadPrimitive.Messages
            components={{
              UserMessage: CustomUserMessage,
              AssistantMessage: CustomAssistantMessage,
            }}
          />

          <StreamingAssistantTurn pendingInteractionInComposer />
        </div>
      </ThreadPrimitive.Viewport>

      <div className="bc-chat-composer-area shrink-0 bg-[linear-gradient(to_top,#171717_78%,rgba(23,23,23,0))] px-4 pb-4 pt-7 sm:px-5">
        <div className="mx-auto w-full max-w-[760px]">
          {canCancelRun ? (
            <div className="bc-chat-run-status mb-2.5 flex items-center justify-between gap-3 rounded-xl border border-white/[0.07] bg-white/[0.035] px-3 py-2">
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
          {hasPendingInteraction && activeTurn.pendingInteraction ? (
            <div
              className="bc-pending-composer max-h-[48dvh] overflow-y-auto"
              data-testid="pending-interaction-composer"
            >
              <AskUserCard
                key={activeTurn.pendingInteraction.id}
                title="请补充这条信息"
                question={activeTurn.pendingInteraction.question}
                onSubmit={(answer) => void handleInteractionAnswer(answer)}
                isSubmitting={isSubmitting}
                error={interactionError}
                onRetry={() => {
                  const failed = lastAnswer.current;
                  if (
                    failed &&
                    failed.interactionId === activeTurn.pendingInteraction?.id
                  ) {
                    if (failed.editQuote !== undefined)
                      void handleSuggestionEdit(failed.editQuote);
                    else void handleInteractionAnswer(failed.answer);
                  }
                }}
                onEdit={(quote) => void handleSuggestionEdit(quote)}
              />
            </div>
          ) : (
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
              spatialContext={spatialContext}
              onClearSpatialContext={onClearSpatialContext}
            />
          )}
        </div>
      </div>
    </ThreadPrimitive.Root>
  );
}
