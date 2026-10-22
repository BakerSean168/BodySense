import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useUploadStore } from "@/stores/uploadStore";
import { usePersonWindowData } from "./usePersonWindowData";

const original = useUploadStore.getState();

afterEach(() => {
  act(() => useUploadStore.setState(original));
});

function createControlledFetch() {
  let resolveNext: () => void = () => {
    throw new Error("No pending fetch to complete");
  };
  const fetchUploads = vi.fn(
    () =>
      new Promise<void>((resolve) => {
        resolveNext = resolve;
      }),
  );
  const complete = async (hasFetched: boolean, error: string | null) => {
    await act(async () => {
      useUploadStore.setState({ uploads: [], hasFetched, error });
      resolveNext();
    });
  };
  return { fetchUploads, complete };
}

describe("usePersonWindowData", () => {
  it("handles Store-caught read errors and a successful retry", async () => {
    const fetch = createControlledFetch();
    useUploadStore.setState({
      uploads: [],
      hasFetched: false,
      error: null,
      fetchUploads: fetch.fetchUploads,
    });

    const { result } = renderHook(() => usePersonWindowData());
    expect(result.current.loading).toBe(true);
    await fetch.complete(false, "HTTP 503");
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBe("部分文件暂时无法读取。");

    act(() => {
      void result.current.retry();
    });
    await fetch.complete(true, null);
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(fetch.fetchUploads).toHaveBeenCalledTimes(2);
  });

  it("defers file reads until Media/Reports is opened", async () => {
    const fetch = createControlledFetch();
    useUploadStore.setState({
      uploads: [],
      hasFetched: false,
      error: null,
      fetchUploads: fetch.fetchUploads,
    });
    const { result, rerender } = renderHook(
      ({ enabled }) => usePersonWindowData(enabled),
      { initialProps: { enabled: false } },
    );

    expect(fetch.fetchUploads).not.toHaveBeenCalled();
    rerender({ enabled: true });
    await fetch.complete(true, null);
    expect(result.current.loading).toBe(false);
    expect(fetch.fetchUploads).toHaveBeenCalledTimes(1);

    rerender({ enabled: false });
    rerender({ enabled: true });
    expect(fetch.fetchUploads).toHaveBeenCalledTimes(1);
  });
});
