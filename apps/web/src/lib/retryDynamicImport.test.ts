import { describe, expect, it, vi } from "vitest";
import { retryDynamicImport } from "./retryDynamicImport";

describe("retryDynamicImport", () => {
  it("retries one transient failure and returns the next successful import", async () => {
    const loader = vi
      .fn<() => Promise<{ value: string }>>()
      .mockRejectedValueOnce(
        new TypeError("Failed to fetch dynamically imported module"),
      )
      .mockResolvedValueOnce({ value: "loaded" });

    await expect(
      retryDynamicImport(loader, { attempts: 2, delayMs: 0 }),
    ).resolves.toEqual({ value: "loaded" });
    expect(loader).toHaveBeenCalledTimes(2);
  });

  it("surfaces a persistent failure after the bounded attempt count", async () => {
    const loader = vi
      .fn<() => Promise<never>>()
      .mockRejectedValue(new Error("persistent chunk failure"));

    await expect(
      retryDynamicImport(loader, { attempts: 2, delayMs: 0 }),
    ).rejects.toThrow("persistent chunk failure");
    expect(loader).toHaveBeenCalledTimes(2);
  });

  it("rejects invalid retry settings", async () => {
    await expect(
      retryDynamicImport(async () => ({ ok: true }), {
        attempts: 0,
        delayMs: 0,
      }),
    ).rejects.toThrow("attempts must be >= 1");

    await expect(
      retryDynamicImport(async () => ({ ok: true }), {
        attempts: 1,
        delayMs: -1,
      }),
    ).rejects.toThrow("delayMs must be >= 0");
  });
});
