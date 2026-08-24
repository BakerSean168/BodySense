export interface ProvisionalChatPromotionInput {
  chatSessionKey: string;
  pendingConversationId: string | null;
  displayedConversationId: string | null;
  activeTurnRunId: string | null | undefined;
  latestAssistantStatus: string | null;
}

const TERMINAL_ASSISTANT_STATUSES = new Set(["completed", "failed", "aborted"]);

/**
 * Promote the provisional `new` assistant-ui runtime only after the server
 * thread has caught up to a terminal assistant projection. This keeps the
 * first live SSE run mounted while it is active, but lets the durable server
 * history become authoritative once that run has settled.
 */
export function shouldPromoteProvisionalChatSession({
  chatSessionKey,
  pendingConversationId,
  displayedConversationId,
  activeTurnRunId,
  latestAssistantStatus,
}: ProvisionalChatPromotionInput): boolean {
  if (chatSessionKey !== "new") return false;
  if (
    !pendingConversationId ||
    pendingConversationId !== displayedConversationId
  ) {
    return false;
  }
  if (activeTurnRunId) return false;
  return Boolean(
    latestAssistantStatus &&
    TERMINAL_ASSISTANT_STATUSES.has(latestAssistantStatus),
  );
}
