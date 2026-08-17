import { useQuery } from "@tanstack/react-query";
import { conversationsQueryOptions } from "../services/consultationQueryOptions";

export function useConversationsQuery() {
  return useQuery({
    ...conversationsQueryOptions(50),
    select: (data) => data.conversations,
  });
}
