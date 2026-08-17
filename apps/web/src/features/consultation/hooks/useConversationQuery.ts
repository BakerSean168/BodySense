import { useQuery } from "@tanstack/react-query";
import { consultationApi } from "../services/consultationService";
import { consultationKeys } from "../services/consultationQueryKeys";

export function useConversationQuery(conversationId: string | null) {
  return useQuery({
    queryKey: conversationId
      ? consultationKeys.conversation(conversationId)
      : consultationKeys.conversationEmpty(),
    queryFn: () => consultationApi.getConversation(conversationId!),
    enabled: !!conversationId,
  });
}
