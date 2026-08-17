import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { errorMessage } from "@/lib/api-client";
import { consultationApi } from "../services/consultationService";
import { consultationKeys } from "../services/consultationQueryKeys";
import { consultationThreadQueryOptions } from "../services/consultationQueryOptions";
import type {
  Conversation,
  ConversationListResponse,
  ConsultationThread,
} from "../types/consultation";

interface UseConversationActionsOptions {
  conversations: Conversation[];
  routeConversationId: string | null;
  navigateTo: (conversationId?: string | null) => void;
}

export function useConversationActions({
  conversations,
  routeConversationId,
  navigateTo,
}: UseConversationActionsOptions) {
  const queryClient = useQueryClient();

  const prefetchConversation = useCallback(
    (conversationId: string) => {
      if (!conversationId || conversationId === routeConversationId) return;
      const queryKey = consultationKeys.thread(conversationId);
      if (queryClient.getQueryState(queryKey)?.fetchStatus === "fetching")
        return;
      if (queryClient.getQueryData(queryKey)) return;
      void queryClient.prefetchQuery(
        consultationThreadQueryOptions(conversationId),
      );
    },
    [queryClient, routeConversationId],
  );

  const deleteConversation = useCallback(
    async (conversationId: string) => {
      try {
        await consultationApi.deleteConversation(conversationId);
        queryClient.setQueryData<ConversationListResponse>(
          consultationKeys.conversations(),
          (old) =>
            old
              ? {
                  ...old,
                  conversations: old.conversations.filter(
                    (conversation) => conversation.id !== conversationId,
                  ),
                }
              : old,
        );
        queryClient.removeQueries({
          queryKey: consultationKeys.thread(conversationId),
        });
        if (routeConversationId === conversationId) navigateTo();
      } catch (error) {
        toast.error(errorMessage(error, "删除会话失败"));
      }
    },
    [navigateTo, queryClient, routeConversationId],
  );

  const deleteAllConversations = useCallback(async () => {
    try {
      await Promise.all(
        conversations.map((conversation) =>
          consultationApi.deleteConversation(conversation.id),
        ),
      );
      queryClient.setQueryData<ConversationListResponse>(
        consultationKeys.conversations(),
        { conversations: [], next_cursor: null, has_more: false },
      );
      queryClient.removeQueries({
        queryKey: [...consultationKeys.all, "thread"],
      });
      navigateTo();
    } catch (error) {
      toast.error(errorMessage(error, "清空会话失败"));
    }
  }, [conversations, navigateTo, queryClient]);

  const pinConversation = useCallback(
    async (conversationId: string, pinned: boolean) => {
      try {
        await consultationApi.pinConversation(conversationId, pinned);
        const pinnedAt = pinned ? new Date().toISOString() : null;
        queryClient.setQueryData<ConversationListResponse>(
          consultationKeys.conversations(),
          (old) =>
            old
              ? {
                  ...old,
                  conversations: old.conversations.map((conversation) =>
                    conversation.id === conversationId
                      ? { ...conversation, pinned, pinned_at: pinnedAt }
                      : conversation,
                  ),
                }
              : old,
        );
        queryClient.setQueryData<ConsultationThread>(
          consultationKeys.thread(conversationId),
          (old) =>
            old
              ? {
                  ...old,
                  conversation: {
                    ...old.conversation,
                    pinned,
                    pinned_at: pinnedAt,
                  },
                }
              : old,
        );
      } catch (error) {
        toast.error(errorMessage(error, "更新置顶状态失败"));
      }
    },
    [queryClient],
  );

  const renameConversation = useCallback(
    async (conversationId: string, title: string) => {
      try {
        await consultationApi.renameTitle(conversationId, title);
        queryClient.setQueryData<ConversationListResponse>(
          consultationKeys.conversations(),
          (old) =>
            old
              ? {
                  ...old,
                  conversations: old.conversations.map((conversation) =>
                    conversation.id === conversationId
                      ? { ...conversation, title }
                      : conversation,
                  ),
                }
              : old,
        );
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
      } catch (error) {
        toast.error(errorMessage(error, "重命名会话失败"));
      }
    },
    [queryClient],
  );

  const shareConversation = useCallback(async (conversationId: string) => {
    try {
      const result = await consultationApi.shareConversation(conversationId);
      await navigator.clipboard.writeText(result.shareUrl);
      toast.success("分享链接已复制到剪贴板");
    } catch (error) {
      toast.error(errorMessage(error, "分享失败，请稍后重试"));
    }
  }, []);

  const unshareConversation = useCallback(async (conversationId: string) => {
    try {
      await consultationApi.unshareConversation(conversationId);
      toast.success("已取消分享");
    } catch (error) {
      toast.error(errorMessage(error, "取消分享失败，请稍后重试"));
    }
  }, []);

  return {
    prefetchConversation,
    deleteConversation,
    deleteAllConversations,
    pinConversation,
    renameConversation,
    shareConversation,
    unshareConversation,
  };
}
