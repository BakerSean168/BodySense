import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { workspaceApi } from "../api/workspaceApi";
import { workspaceKeys } from "../api/workspaceQueryOptions";
import { useBodyStateCommand } from "./useBodyStateCommand";

afterEach(() => vi.restoreAllMocks());

describe("useBodyStateCommand", () => {
  it("invalidates the workspace projection after a successful command", async () => {
    vi.spyOn(workspaceApi, "reviewObservation").mockResolvedValue({});
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const wrapper = ({ children }: PropsWithChildren) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useBodyStateCommand(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        type: "reviewObservation",
        observationId: "observation-1",
        expectedRevision: 5,
        reviewState: "confirmed",
      });
    });

    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: workspaceKeys.all }),
    );
  });
});
