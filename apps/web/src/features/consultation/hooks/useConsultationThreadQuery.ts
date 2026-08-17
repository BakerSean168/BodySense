import { useQuery } from "@tanstack/react-query";
import { consultationThreadQueryOptions } from "../services/consultationQueryOptions";

export function useConsultationThreadQuery(conversationId: string | null) {
  const resolvedId = conversationId ?? "__disabled__";
  return useQuery({
    ...consultationThreadQueryOptions(resolvedId),
    enabled: Boolean(conversationId),
    placeholderData: (previousData) => previousData,
  });
}
