import { describe, expect, it, vi } from "vitest";
import { loadPinnedReferenceMuscles } from "./referenceAtlasLoader";

it("does not parse downloaded muscle bytes after cancellation while waiting for skin", async () => {
  const abort = new AbortController();
  let release: () => void = () => undefined;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  const response = new Response(new Uint8Array(8));
  const read = vi.spyOn(response, "arrayBuffer");
  const promise = loadPinnedReferenceMuscles({
    signal: abort.signal,
    parseAfter: gate,
    fetchImpl: vi.fn().mockResolvedValue(response),
  });
  await vi.waitFor(() => expect(read).toHaveBeenCalled());
  abort.abort();
  release();
  await expect(promise).rejects.toMatchObject({ name: "AbortError" });
});

import { BufferGeometry, MeshBasicMaterial } from "three";
import { parseReferenceAtlasSurface } from "./referenceAtlasLoader";

function invalidSecondPrimitive() {
  const json = {
    accessors: [
      { bufferView: 0, componentType: 5126, count: 3, type: "VEC3" },
      { bufferView: 1, componentType: 5123, count: 3, type: "SCALAR" },
    ],
    bufferViews: [
      { buffer: 0, byteOffset: 0, byteLength: 36 },
      { buffer: 0, byteOffset: 36, byteLength: 6 },
    ],
    meshes: [
      {
        primitives: [
          { attributes: { POSITION: 0 }, indices: 1 },
          { attributes: { POSITION: 999 }, indices: 1 },
        ],
      },
    ],
    nodes: [{ name: "body-shell", mesh: 0 }],
  };
  const encoded = new TextEncoder().encode(JSON.stringify(json));
  const jsonLength = Math.ceil(encoded.length / 4) * 4;
  const buffer = new ArrayBuffer(12 + 8 + jsonLength + 8 + 44);
  const view = new DataView(buffer);
  view.setUint32(0, 0x46546c67, true);
  view.setUint32(4, 2, true);
  view.setUint32(8, buffer.byteLength, true);
  view.setUint32(12, jsonLength, true);
  view.setUint32(16, 0x4e4f534a, true);
  const jsonBytes = new Uint8Array(buffer, 20, jsonLength);
  jsonBytes.fill(32);
  jsonBytes.set(encoded);
  const binary = 20 + jsonLength;
  view.setUint32(binary, 44, true);
  view.setUint32(binary + 4, 0x004e4942, true);
  new Float32Array(buffer, binary + 8, 9).set([0, 0, 0, 1, 0, 0, 0, 1, 0]);
  new Uint16Array(buffer, binary + 44, 3).set([0, 1, 2]);
  return buffer;
}

describe("reference GLB ownership", () => {
  it("releases both attached and in-progress geometry and the shared material when parsing fails", () => {
    const geometry = vi.spyOn(BufferGeometry.prototype, "dispose");
    const material = vi.spyOn(MeshBasicMaterial.prototype, "dispose");
    try {
      expect(() =>
        parseReferenceAtlasSurface(invalidSecondPrimitive()),
      ).toThrow("accessor is unsupported");
      expect(geometry).toHaveBeenCalledTimes(2);
      expect(material).toHaveBeenCalledTimes(1);
    } finally {
      geometry.mockRestore();
      material.mockRestore();
    }
  });
});
