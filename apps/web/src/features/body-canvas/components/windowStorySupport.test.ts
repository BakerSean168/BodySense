import { afterEach, describe, expect, it, vi } from "vitest";
import preview from "../../../../.storybook/preview";

afterEach(() => vi.unstubAllGlobals());

describe("Storybook transport isolation", () => {
  it("blocks app API reads and writes before rendering, permits assets, and restores fetch", async () => {
    const transport = vi.fn().mockResolvedValue(new Response("asset"));
    vi.stubGlobal("fetch", transport);
    const restore = preview.beforeEach();
    try {
      await expect(fetch("/api/v1/workspace")).rejects.toThrow(
        "已隔离真实 API",
      );
      await expect(
        fetch(new URL("https://backend.example/api/v1/profile"), {
          method: "PATCH",
        }),
      ).rejects.toThrow("已隔离真实 API");
      await expect(
        fetch(
          new Request("https://backend.example/api/v1/uploads", {
            method: "DELETE",
          }),
        ),
      ).rejects.toThrow("已隔离真实 API");
      expect(transport).not.toHaveBeenCalled();
      await fetch("/assets/model.glb");
      expect(transport).toHaveBeenCalledExactlyOnceWith(
        "/assets/model.glb",
        undefined,
      );
    } finally {
      restore();
    }
    expect(window.fetch).toBe(transport);
  });
});
