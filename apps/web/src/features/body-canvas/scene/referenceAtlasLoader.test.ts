import { BufferGeometry, Mesh, MeshBasicMaterial } from "three";
import { describe, expect, it, vi } from "vitest";
import {
  disposeReferenceAtlasSurface,
  loadPinnedReferenceMuscles,
  parseReferenceAtlasMeshes,
  parseReferenceAtlasSurface,
} from "./referenceAtlasLoader";

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

/** Builds a GLB with `pairs` independent (position, index) triangle pairs. */
function buildGlb(
  json: object,
  pairs: number,
  fill?: (buffer: ArrayBuffer, binaryOffset: number) => void,
): ArrayBuffer {
  const binLength = pairs * 44;
  const encoded = new TextEncoder().encode(JSON.stringify(json));
  const jsonLength = Math.ceil(encoded.length / 4) * 4;
  const buffer = new ArrayBuffer(12 + 8 + jsonLength + 8 + binLength);
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
  view.setUint32(binary, binLength, true);
  view.setUint32(binary + 4, 0x004e4942, true);
  const content = binary + 8;
  for (let pair = 0; pair < pairs; pair += 1) {
    new Float32Array(buffer, content + pair * 44, 9).set([
      0, 0, 0, 1, 0, 0, 0, 1, 0,
    ]);
    new Uint16Array(buffer, content + pair * 44 + 36, 3).set([0, 1, 2]);
  }
  fill?.(buffer, content);
  return buffer;
}

function triangleJson(pairs: number): {
  accessors: object[];
  bufferViews: object[];
  buffers: object[];
} {
  const accessors: object[] = [];
  const bufferViews: object[] = [];
  for (let pair = 0; pair < pairs; pair += 1) {
    accessors.push(
      {
        bufferView: pair * 2,
        componentType: 5126,
        count: 3,
        type: "VEC3",
        min: [0, 0, 0],
        max: [1, 1, 0],
      },
      {
        bufferView: pair * 2 + 1,
        componentType: 5123,
        count: 3,
        type: "SCALAR",
      },
    );
    bufferViews.push(
      { buffer: 0, byteOffset: pair * 44, byteLength: 36 },
      { buffer: 0, byteOffset: pair * 44 + 36, byteLength: 6 },
    );
  }
  return { accessors, bufferViews, buffers: [{ byteLength: pairs * 44 }] };
}

function glbWithNodes(
  nodes: object[],
  meshes: object[],
  pairs: number,
): ArrayBuffer {
  return buildGlb(
    {
      asset: { version: "2.0" },
      scene: 0,
      scenes: [{ nodes: nodes.map((_, index) => index) }],
      ...triangleJson(pairs),
      meshes,
      nodes,
    },
    pairs,
  );
}

function invalidSecondPrimitive(): ArrayBuffer {
  return buildGlb(
    {
      asset: { version: "2.0" },
      scene: 0,
      scenes: [{ nodes: [0] }],
      ...triangleJson(1),
      meshes: [
        {
          primitives: [
            { attributes: { POSITION: 0 }, indices: 1 },
            { attributes: { POSITION: 999 }, indices: 1 },
          ],
        },
      ],
      nodes: [{ name: "body-shell", mesh: 0 }],
    },
    1,
  );
}

describe("reference GLB ownership", () => {
  it("releases the shared material when GLB parsing fails mid-geometry", async () => {
    const geometry = vi.spyOn(BufferGeometry.prototype, "dispose");
    const material = vi.spyOn(MeshBasicMaterial.prototype, "dispose");
    try {
      await expect(
        parseReferenceAtlasSurface(invalidSecondPrimitive()),
      ).rejects.toThrow("bufferView");
      expect(material).toHaveBeenCalledTimes(1);
      // Until parsing completes, the geometries under construction belong
      // to GLTFLoader; at this stage the adapter owns (and releases) only
      // the shared placeholder material.
      expect(geometry).not.toHaveBeenCalled();
    } finally {
      geometry.mockRestore();
      material.mockRestore();
    }
  });

  it("releases parsed geometry and the shared material when no mesh is accepted", async () => {
    const geometry = vi.spyOn(BufferGeometry.prototype, "dispose");
    const material = vi.spyOn(MeshBasicMaterial.prototype, "dispose");
    const bytes = glbWithNodes(
      [{ name: "not-a-shell", mesh: 0 }],
      [{ primitives: [{ attributes: { POSITION: 0 }, indices: 1 }] }],
      1,
    );
    try {
      await expect(parseReferenceAtlasSurface(bytes)).rejects.toThrow(
        "Vanatome regional GLB contains no body-shell surface",
      );
      expect(geometry).toHaveBeenCalledTimes(1);
      expect(material).toHaveBeenCalledTimes(1);
    } finally {
      geometry.mockRestore();
      material.mockRestore();
    }
  });
});

describe("reference GLB parsing via GLTFLoader", () => {
  it("restores the original node names GLTFLoader sanitizes and applies node transforms", async () => {
    const bytes = glbWithNodes(
      [
        {
          name: "body-shell__Angle of mouth.l",
          mesh: 0,
          translation: [0.2, 1.35, 0],
          scale: [-1, 1, 1],
        },
        { name: "body-shell__Angle of mouth.r", mesh: 1 },
      ],
      [
        { primitives: [{ attributes: { POSITION: 0 }, indices: 1 }] },
        { primitives: [{ attributes: { POSITION: 2 }, indices: 3 }] },
      ],
      2,
    );
    const group = await parseReferenceAtlasSurface(bytes);
    try {
      expect(group.children.map((child) => child.name)).toEqual([
        "body-shell__Angle of mouth.l",
        "body-shell__Angle of mouth.r",
      ]);
      const [left, right] = group.children as Mesh[];
      expect(left.position.toArray()).toEqual([0.2, 1.35, 0]);
      expect(left.scale.toArray()).toEqual([-1, 1, 1]);
      // One shared placeholder material for every mesh, as before.
      expect(left.material).toBeInstanceOf(MeshBasicMaterial);
      expect(left.material).toBe(right.material);
      const geometry = left.geometry;
      expect(geometry.getAttribute("position").count).toBe(3);
      expect(geometry.getAttribute("position").array).toBeInstanceOf(
        Float32Array,
      );
      // The fixture has no NORMAL accessor; normals are computed, as before.
      expect(geometry.getAttribute("normal")).toBeDefined();
      expect(geometry.getIndex()).not.toBeNull();
    } finally {
      disposeReferenceAtlasSurface(group);
    }
  });

  it("filters muscle and tendon nodes by their original names", async () => {
    const bytes = glbWithNodes(
      [
        { name: "Foo bar.r__muscle", mesh: 0 },
        { name: "Foo bar.r__tendon", mesh: 1 },
        { name: "Unrelated", mesh: 2 },
      ],
      [
        { primitives: [{ attributes: { POSITION: 0 }, indices: 1 }] },
        { primitives: [{ attributes: { POSITION: 2 }, indices: 3 }] },
        { primitives: [{ attributes: { POSITION: 4 }, indices: 5 }] },
      ],
      3,
    );
    const group = await parseReferenceAtlasMeshes(bytes, {
      accept: (name) => /__(muscle|tendon)$/.test(name),
      label: "test muscles",
      emptyMessage: "test muscles GLB contains no meshes",
    });
    try {
      expect(group.children.map((child) => child.name)).toEqual([
        "Foo bar.r__muscle",
        "Foo bar.r__tendon",
      ]);
    } finally {
      disposeReferenceAtlasSurface(group);
    }
  });

  it("emits one mesh per primitive for multi-primitive nodes, under the node name and transform", async () => {
    const bytes = glbWithNodes(
      [{ name: "multi prim node", mesh: 0, translation: [1, 2, 3] }],
      [
        {
          primitives: [
            { attributes: { POSITION: 0 }, indices: 1 },
            { attributes: { POSITION: 2 }, indices: 3 },
          ],
        },
      ],
      2,
    );
    const group = await parseReferenceAtlasMeshes(bytes, {
      accept: () => true,
      label: "test multi-primitive",
      emptyMessage: "test multi-primitive GLB contains no meshes",
    });
    try {
      expect(group.children).toHaveLength(2);
      for (const child of group.children) {
        expect(child.name).toBe("multi prim node");
        expect((child as Mesh).position.toArray()).toEqual([1, 2, 3]);
      }
    } finally {
      disposeReferenceAtlasSurface(group);
    }
  });
});
