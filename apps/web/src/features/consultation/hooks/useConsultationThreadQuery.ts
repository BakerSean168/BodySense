import { useQuery } from '@tanstack/react-query';
import { consultationApi } from '../services/consultationService';
import { consultationKeys } from '../services/consultationQueryKeys';

export function useConsultationThreadQuery(conversationId: string | null) {
  return useQuery({
    queryKey: conversationId
      ? consultationKeys.thread(conversationId)
      : consultationKeys.threadEmpty(),
    queryFn: () => consultationApi.getConsultationThread(conversationId!),
    enabled: !!conversationId,
  });
}
