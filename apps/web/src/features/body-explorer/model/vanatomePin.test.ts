import { describe, expect, it, vi } from "vitest";
import {
  resolvePinnedVanatomeSystemSource,
  VANATOME_ATLAS_BUILD_ID,
  VANATOME_ATLAS_RELEASE,
} from "./vanatomePin";

function response(value: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: vi.fn().mockResolvedValue(value),
  } as unknown as Response;
}

describe("Vanatome pinned lightweight source", () => {
  it("accepts only the pinned release/build and resolves relative bundle URLs", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      response({
        atlas: {
          version: VANATOME_ATLAS_RELEASE,
          buildId: VANATOME_ATLAS_BUILD_ID,
        },
        systems: [{ id: "regional-anatomy", bundleId: "regional" }],
        bundles: [
          {
            id: "regional",
            modelUrl: "../../models/regional.glb",
            bytes: 1234,
            sha256: "abc",
          },
        ],
      }),
    ) as unknown as typeof fetch;

    await expect(
      resolvePinnedVanatomeSystemSource("regional-anatomy", {
        catalogUrl:
          "https://atlas.example/releases/1.4.0/catalog.json",
        fetchImpl,
      }),
    ).resolves.toEqual({
      catalogUrl: "https://atlas.example/releases/1.4.0/catalog.json",
      modelUrl: "https://atlas.example/models/regional.glb",
      bytes: 1234,
      sha256: "abc",
    });
  });

  it("rejects an atlas that changes under the pinned URL", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      response({
        atlas: { version: "1.4.1", buildId: "unexpected" },
        systems: [],
        bundles: [],
      }),
    ) as unknown as typeof fetch;

    await expect(
      resolvePinnedVanatomeSystemSource("regional-anatomy", {
        catalogUrl:
          "https://atlas.example/releases/1.4.0/catalog.json",
        fetchImpl,
      }),
    ).rejects.toThrow(/Unexpected Vanatome atlas/);
  });
});
