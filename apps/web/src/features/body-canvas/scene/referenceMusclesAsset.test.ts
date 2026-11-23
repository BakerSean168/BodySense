import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { Group, Mesh } from "three";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import {
  disposeReferenceAtlasSurface,
  parseReferenceAtlasMeshes,
} from "./referenceAtlasLoader";
import { bakeSkinnedGeometry } from "./referenceRigSkinning";

/**
 * Bake-level regression for the Meshopt-compressed muscles asset.
 *
 * The shipped GLB stores position/normal as quantized, interleaved,
 * Meshopt-encoded attributes plus a compensating node transform. three's
 * `applyMatrix4` (used by the rig bake) mutates raw attribute arrays and
 * cannot dequantize, so unless the parser materializes plain Float32
 * attributes first, baked geometry comes out systematically displaced
 * (measured on this asset during the #635 evaluation: a tendon mesh
 * shifted by -2.0001 in Y). The golden values below were produced by
 * running the *uncompressed* asset through this same production parser
 * and bake; the compressed asset must reproduce them within quantization
 * noise (measured: bounds within 1.4e-5, per-point nearest-neighbour
 * distance at most 3.5e-5 — five orders of magnitude below the corruption
 * signature). Like the pinned byte counts, the golden values are pinned
 * to this asset revision: regenerate them if the asset is re-exported.
 */
const BOUNDS_TOLERANCE = 1e-3;

const GOLDEN_BAKES: Array<{
  name: string;
  vertexCount: number;
  min: [number, number, number];
  max: [number, number, number];
  positionSum: number;
}> = [
  {
    name: "Sternocostal head of pectoralis major muscle.r__tendon",
    vertexCount: 128,
    min: [-0.19310323894023895, 1.3139679431915283, -0.015892066061496735],
    max: [-0.14124543964862823, 1.3592184782028198, 0.03468095138669014],
    positionSum: 149.9252818073728,
  },
  {
    // Mirrored twin (negative-determinant node): exercises the winding
    // restoration in the bake as well as the quantization path.
    name: "Sternocostal head of pectoralis major muscle.l__tendon",
    vertexCount: 128,
    min: [0.14124546945095062, 1.3139679431915283, -0.015892192721366882],
    max: [0.19310325384140015, 1.3592184782028198, 0.03468085080385208],
    positionSum: 192.65802615351276,
  },
  {
    // Largest mesh in the asset (6,773 vertices).
    name: "Iliotibial tract.r__tendon",
    vertexCount: 6773,
    min: [-0.16116535663604736, 0.3923715651035309, -0.05350679159164429],
    max: [-0.08975373953580856, 0.9977180361747742, 0.028128178790211678],
    positionSum: 4087.064606867934,
  },
];

describe("Meshopt-compressed reference muscles asset", () => {
  let group: Group;

  beforeAll(async () => {
    const path = resolve(
      dirname(fileURLToPath(import.meta.url)),
      "../../../../public/anatomy/z-anatomy-muscles-v1.glb",
    );
    const bytes = readFileSync(path);
    const arrayBuffer = bytes.buffer.slice(
      bytes.byteOffset,
      bytes.byteOffset + bytes.byteLength,
    ) as ArrayBuffer;
    group = await parseReferenceAtlasMeshes(arrayBuffer, {
      accept: (name) => /__(muscle|tendon)$/.test(name),
      label: "Reference muscles asset regression",
      emptyMessage: "Reference muscles GLB contains no meshes",
    });
    group.updateMatrixWorld(true);
  });

  afterAll(() => {
    disposeReferenceAtlasSurface(group);
  });

  it("decodes to the full inventory with materialized Float32 attributes", () => {
    let muscles = 0;
    let tendons = 0;
    let vertices = 0;
    let triangles = 0;
    for (const child of group.children) {
      const mesh = child as Mesh;
      if (mesh.name.endsWith("__muscle")) muscles += 1;
      if (mesh.name.endsWith("__tendon")) tendons += 1;
      const geometry = mesh.geometry;
      const position = geometry.getAttribute("position");
      vertices += position.count;
      triangles += (geometry.getIndex()?.count ?? 0) / 3;
      // The materialization contract the bake depends on: plain Float32
      // arrays, never the quantized interleaved storage of the file.
      expect(position.array).toBeInstanceOf(Float32Array);
      expect(geometry.getAttribute("normal")?.array).toBeInstanceOf(
        Float32Array,
      );
    }
    expect(group.children).toHaveLength(497);
    expect(muscles).toBe(274);
    expect(tendons).toBe(223);
    expect(vertices).toBe(180646);
    expect(triangles).toBe(325378);
  });

  for (const golden of GOLDEN_BAKES) {
    it(`bakes ${golden.name} to the pre-compression geometry`, () => {
      const mesh = group.children.find(
        (child) => child.name === golden.name,
      ) as Mesh;
      expect(mesh).toBeDefined();
      const baked = bakeSkinnedGeometry(mesh);
      try {
        const position = baked.getAttribute("position");
        expect(position.count).toBe(golden.vertexCount);
        const min = [Infinity, Infinity, Infinity];
        const max = [-Infinity, -Infinity, -Infinity];
        let sum = 0;
        const array = position.array as Float32Array;
        for (let index = 0; index < array.length; index += 1) {
          sum += array[index];
          const axis = index % 3;
          if (array[index] < min[axis]) min[axis] = array[index];
          if (array[index] > max[axis]) max[axis] = array[index];
        }
        for (let axis = 0; axis < 3; axis += 1) {
          expect(Math.abs(min[axis] - golden.min[axis])).toBeLessThanOrEqual(
            BOUNDS_TOLERANCE,
          );
          expect(Math.abs(max[axis] - golden.max[axis])).toBeLessThanOrEqual(
            BOUNDS_TOLERANCE,
          );
        }
        // The component sum is order-independent (the compressor reorders
        // vertices); quantization noise on it measured below 0.01 even for
        // the largest mesh, while a dequantization failure shifts it by
        // whole units per vertex.
        const sumTolerance = Math.max(0.01, golden.vertexCount * 1e-5);
        expect(Math.abs(sum - golden.positionSum)).toBeLessThanOrEqual(
          sumTolerance,
        );
      } finally {
        baked.dispose();
      }
    });
  }
});
