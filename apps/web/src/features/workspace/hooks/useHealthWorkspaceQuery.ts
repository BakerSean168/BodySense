import { useQuery } from "@tanstack/react-query";
import {
  healthWorkspaceQueryKey,
  healthWorkspaceQueryOptions,
} from "../api/workspaceQueryOptions";

export { healthWorkspaceQueryKey };

export function useHealthWorkspaceQuery() {
  return useQuery(healthWorkspaceQueryOptions());
}
