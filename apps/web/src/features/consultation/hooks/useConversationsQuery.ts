import { useQuery } from '@tanstack/react-query';
import { consultationApi } from '../services/consultationService';
import { consultationKeys } from '../services/consultationQueryKeys';

export function useConversationsQuery() {
  return useQuery({
    queryKey: consultationKeys.conversations(),
    queryFn: () => consultationApi.listConversations({ limit: 50 }),
    select: (data) => data.conversations,
  });
}
