import { useCallback, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { workspaceKeys } from "@/features/workspace/api/workspaceQueryOptions";
import { consultationKeys } from "../services/consultationQueryKeys";
import type {
  ConsultationPhase,
  ConsultationThread,
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

  // The server lazily creates the user's single Conversation on the first
  // turn; record its identity so every consumer resolves the same thread.
  const registerConversation = useCallback(
    (conversationId: string) => {
      queryClient.setQueryData(consultationKeys.conversation(), {
        conversationId,
      });
    },
    [queryClient],
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
      // Go persists state events before they reach the browser. Refresh the
      // living BodyState immediately instead of waiting for stream.done; an
      // intake turn may pause for user input and never emit stream.done yet.
      void queryClient.invalidateQueries({ queryKey: workspaceKeys.all });
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
    reconcileMessageId,
    updateExtractedInfo,
    updatePhase,
    finishStream,
    retryThread,
  };
}
