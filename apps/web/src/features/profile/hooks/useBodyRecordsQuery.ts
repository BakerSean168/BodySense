import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@/stores/authStore";
import { readBodyRecords } from "../services/bodyRecordsService";

export function useBodyRecordsQuery() {
  const owner = useAuthStore((state) => state.user?.id);
  return useQuery({
    queryKey: ["workspace", "body-records", owner],
    enabled: Boolean(owner),
    queryFn: () => {
      if (!owner) throw new Error("Body records owner unavailable");
      return readBodyRecords(owner);
    },
    staleTime: 10_000,
  });
}
