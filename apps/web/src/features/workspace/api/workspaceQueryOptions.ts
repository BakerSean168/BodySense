import { queryOptions } from "@tanstack/react-query";
import { workspaceApi } from "./workspaceApi";

export const workspaceKeys = {
  all: ["workspace"] as const,
  health: () => [...workspaceKeys.all, "health"] as const,
};

export function healthWorkspaceQueryOptions() {
  return queryOptions({
    queryKey: workspaceKeys.health(),
    queryFn: workspaceApi.get,
    staleTime: 10_000,
    refetchOnWindowFocus: false,
  });
}

export const healthWorkspaceQueryKey = workspaceKeys.health();
