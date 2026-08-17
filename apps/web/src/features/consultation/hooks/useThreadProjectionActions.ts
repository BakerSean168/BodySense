import { useCallback, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { workspaceKeys } from "@/features/workspace";
import { consultationKeys } from "../services/consultationQueryKeys";
import type {
  ConsultationPhase,
  ConsultationThread,
  ConversationListResponse,
  ExtractedInfo,
} from "../types/consultation";

interface UseThreadProjectionActionsOptions {
  routeConversationId: string | null;
  activeConversationIdRef: RefObject<string | null>;
}

export function useThreadProjectionActions({
  routeConversationId,
  activeConversationIdRef,
}: UseThreadProjectionActionsOptions) {
  const queryClient = useQueryClient();
  const resolveConversationId = useCallback(
    () => activeConversationIdRef.current ?? routeConversationId,
    [activeConversationIdRef, routeConversationId],
  );

  const registerConversation = useCallback(
    (conversationId: string) => {
      const now = new Date().toISOString();
      queryClient.setQueryData<ConversationListResponse>(
        consultationKeys.conversations(),
        (old) => ({
          conversations: [
            {
              id: conversationId,
              title: "",
              title_status: "pending",
              status: "active",
              pinned: false,
              pinned_at: null,
              default_model: null,
              last_message_at: now,
              message_count: 0,
              metadata: {},
              created_at: now,
              updated_at: now,
            },
            ...(old?.conversations ?? []).filter(
              (conversation) => conversation.id !== conversationId,
            ),
          ],
          next_cursor: old?.next_cursor ?? null,
          has_more: old?.has_more ?? false,
        }),
      );
    },
    [queryClient],
  );

  const updateTitle = useCallback(
    (title: string) => {
      const conversationId = resolveConversationId();
      if (!conversationId) return;
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(conversationId),
        (old) =>
          old
            ? {
                ...old,
                conversation: {
                  ...old.conversation,
                  title,
                  title_status: "generated",
                },
              }
            : old,
      );
      queryClient.setQueryData<ConversationListResponse>(
        consultationKeys.conversations(),
        (old) =>
          old
            ? {
                ...old,
                conversations: old.conversations.map((conversation) =>
                  conversation.id === conversationId
                    ? { ...conversation, title, title_status: "generated" }
                    : conversation,
                ),
              }
            : old,
      );
    },
    [queryClient, resolveConversationId],
  );

  const reconcileMessageId = useCallback(
    (clientMessageId: string, messageId: string) => {
      const conversationId = resolveConversationId();
      if (!conversationId) return;
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(conversationId),
        (old) =>
          old
            ? {
                ...old,
                messages: old.messages.map((message) =>
                  message.id === clientMessageId
                    ? { ...message, id: messageId }
                    : message,
                ),
              }
            : old,
      );
    },
    [queryClient, resolveConversationId],
  );

  const updateExtractedInfo = useCallback(
    async (info: ExtractedInfo[]) => {
      const conversationId = resolveConversationId();
      if (!conversationId) return;
      await queryClient.cancelQueries({
        queryKey: consultationKeys.thread(conversationId),
      });
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(conversationId),
        (old) => (old ? { ...old, extracted_info: info } : old),
      );
    },
    [queryClient, resolveConversationId],
  );

  const updatePhase = useCallback(
    async (phase: ConsultationPhase) => {
      const conversationId = resolveConversationId();
      if (!conversationId) return;
      const queryKey = consultationKeys.thread(conversationId);
      if (
        queryClient.getQueryData<ConsultationThread>(queryKey)?.phase === phase
      )
        return;
      await queryClient.cancelQueries({ queryKey });
      queryClient.setQueryData<ConsultationThread>(queryKey, (old) =>
        old ? { ...old, phase } : old,
      );
    },
    [queryClient, resolveConversationId],
  );

  const finishStream = useCallback(() => {
    const conversationId = resolveConversationId();
    if (!conversationId) return;
    void queryClient.invalidateQueries({
      queryKey: consultationKeys.thread(conversationId),
    });
    void queryClient.invalidateQueries({
      queryKey: consultationKeys.conversations(),
    });
    void queryClient.invalidateQueries({ queryKey: workspaceKeys.all });
  }, [queryClient, resolveConversationId]);

  const retryThread = useCallback(() => {
    if (!routeConversationId) return;
    void queryClient.invalidateQueries({
      queryKey: consultationKeys.thread(routeConversationId),
    });
  }, [queryClient, routeConversationId]);

  return {
    registerConversation,
    updateTitle,
    reconcileMessageId,
    updateExtractedInfo,
    updatePhase,
    finishStream,
    retryThread,
  };
}
