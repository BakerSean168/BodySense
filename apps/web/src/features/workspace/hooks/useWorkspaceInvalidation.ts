import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { workspaceKeys } from "../api/workspaceQueryOptions";

export function useWorkspaceInvalidation() {
  const queryClient = useQueryClient();
  return useCallback(
    () => queryClient.invalidateQueries({ queryKey: workspaceKeys.all }),
    [queryClient],
  );
}
