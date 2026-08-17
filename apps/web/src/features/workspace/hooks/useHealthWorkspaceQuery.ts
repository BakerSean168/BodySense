import { useQuery } from "@tanstack/react-query";
import { workspaceApi } from "../services/workspaceService";

export const healthWorkspaceQueryKey = ["health-workspace"] as const;

export function useHealthWorkspaceQuery() {
  return useQuery({
    queryKey: healthWorkspaceQueryKey,
    queryFn: workspaceApi.get,
    staleTime: 10_000,
    refetchOnWindowFocus: false,
  });
}
