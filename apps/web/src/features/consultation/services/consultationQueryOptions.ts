import { queryOptions } from "@tanstack/react-query";
import { consultationApi } from "./consultationService";
import { consultationKeys } from "./consultationQueryKeys";

/** The user's single Conversation (get-or-create on the server). */
export function conversationQueryOptions() {
  return queryOptions({
    queryKey: consultationKeys.conversation(),
    queryFn: () => consultationApi.getConversation(),
    staleTime: Infinity,
  });
}

export function consultationThreadQueryOptions(conversationId: string) {
  return queryOptions({
    queryKey: consultationKeys.thread(conversationId),
    queryFn: () => consultationApi.getConsultationThread(conversationId),
    staleTime: 30_000,
  });
}

export function diagnosisHistoryQueryOptions(limit = 20) {
  return queryOptions({
    queryKey: consultationKeys.diagnosisHistory(limit),
    queryFn: () => consultationApi.listDiagnosisHistory(limit),
    staleTime: 30_000,
  });
}
