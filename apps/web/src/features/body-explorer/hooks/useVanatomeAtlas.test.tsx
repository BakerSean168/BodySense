import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useVanatomeAtlas } from "./useVanatomeAtlas";

const api = vi.hoisted(() => ({
  initial: vi.fn(),
  system: vi.fn(
    (_id: string, _options?: { signal?: AbortSignal }) =>
      new Promise<{
        modelUrl: string;
        structures: { id: string; system: string }[];
      }>(() => {}),
  ),
}));
vi.mock("../adapters/vanatomeAdapter", () => ({
  loadPinnedVanatomeAtlas: api.initial,
  normalizeVanatomeError: (error: Error) => ({
    kind: "unknown",
    message: error.message,
    retryable: true,
  }),
}));

describe("optional atlas system ownership", () => {
  it("aborts optional system metadata/prefetch when its viewer unmounts", async () => {
    api.initial.mockResolvedValue({
      atlases: [{ structures: [{ id: "body-shell", system: "regional" }] }],
      catalog: { systems: [] },
      loadSystem: api.system,
    });
    const options = {
      onFatalError: vi.fn(),
      reportViewerDiagnostic: vi.fn(),
      setLoadState: vi.fn(),
      setViewerError: vi.fn(),
    };
    const { result, unmount } = renderHook(() => useVanatomeAtlas(options));
    await waitFor(() => expect(result.current.loaded).not.toBeNull());
    act(() => {
      void result.current.ensureSystem("skeletal");
    });
    const signal = api.system.mock.calls[0]?.[1]?.signal;
    expect(signal).toBeInstanceOf(AbortSignal);
    unmount();
    expect(signal?.aborted).toBe(true);
  });
});

it("keeps an optional system failure retryable while the initial atlas stays usable", async () => {
  api.system
    .mockReset()
    .mockRejectedValueOnce(new Error("optional layer failed"))
    .mockResolvedValueOnce({
      modelUrl: "/skeleton.glb",
      structures: [{ id: "bone", system: "skeletal" }],
    });
  api.initial.mockResolvedValue({
    atlases: [{ structures: [{ id: "body-shell", system: "regional" }] }],
    catalog: { systems: [] },
    loadSystem: api.system,
  });
  const options = {
    onFatalError: vi.fn(),
    reportViewerDiagnostic: vi.fn(),
    setLoadState: vi.fn(),
    setViewerError: vi.fn(),
  };
  const { result, unmount } = renderHook(() => useVanatomeAtlas(options));
  await waitFor(() => expect(result.current.loaded).not.toBeNull());
  await act(async () => {
    await result.current.ensureSystem("skeletal");
  });
  expect(result.current.failedSystemId).toBe("skeletal");
  expect(options.onFatalError).not.toHaveBeenCalled();
  await act(async () => {
    await result.current.ensureSystem("skeletal");
  });
  expect(result.current.failedSystemId).toBeNull();
  expect(result.current.loadedSystems.has("skeletal")).toBe(true);
  unmount();
});
