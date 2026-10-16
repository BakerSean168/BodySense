import { useQuery } from "@tanstack/react-query";
import { conversationQueryOptions } from "../services/consultationQueryOptions";

/** Resolves the user's single Conversation (created on first use by the API). */
export function useConversationQuery() {
  return useQuery(conversationQueryOptions());
}
