import { useQuery } from '@tanstack/react-query';
import { consultationApi } from '../services/consultationService';
import { consultationKeys } from '../services/consultationQueryKeys';

export function useConsultationSessionQuery(conversationId: string | null) {
  return useQuery({
    queryKey: conversationId
      ? consultationKeys.session(conversationId)
      : consultationKeys.sessionEmpty(),
    queryFn: () => consultationApi.getConsultation(conversationId!),
    enabled: !!conversationId,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    refetchOnMount: false,
  });
}
