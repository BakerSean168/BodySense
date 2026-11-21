import {
  BufferAttribute,
  type BufferGeometry,
  Float32BufferAttribute,
  type Mesh,
  Uint16BufferAttribute,
} from "three";
import { jointIndex, smooth, type JointId } from "./referenceRigJoints";

function restoreMirroredTriangleWinding(geometry: BufferGeometry): void {
  // Several right-side Vanatome shell nodes are mirrored with a negative scale.
  // Baking that matrix into geometry reverses triangle winding. Keep the source
  // indexed representation and swap the second/third index of every triangle;
  // this avoids tripling the vertex payload just to repair face orientation.
  const index = geometry.getIndex();
  if (index) {
    for (let offset = 0; offset + 2 < index.count; offset += 3) {
      const second = index.getX(offset + 1);
      index.setX(offset + 1, index.getX(offset + 2));
      index.setX(offset + 2, second);
    }
    index.needsUpdate = true;
    return;
  }

  // Defensive fallback for a future non-indexed atlas build.
  for (const attribute of Object.values(geometry.attributes)) {
    if (!(attribute instanceof BufferAttribute)) continue;
    for (let vertex = 0; vertex + 2 < attribute.count; vertex += 3) {
      for (let component = 0; component < attribute.itemSize; component += 1) {
        const second = attribute.getComponent(vertex + 1, component);
        attribute.setComponent(
          vertex + 1,
          component,
          attribute.getComponent(vertex + 2, component),
        );
        attribute.setComponent(vertex + 2, component, second);
      }
    }
    attribute.needsUpdate = true;
  }
}

/** Deterministic artist-authored skinning for this pinned reference atlas, not measured anatomy. */
export function skinInfluence(x: number, y: number): [number, number, number] {
  const side = x >= 0 ? "L" : "R";
  const blend = (
    a: JointId,
    b: JointId,
    weight: number,
  ): [number, number, number] => [
    jointIndex.get(a)!,
    jointIndex.get(b)!,
    smooth(weight),
  ];
  const arm = Math.abs(x) > 0.185 && y > 0.63 && y < 1.45;
  if (arm) {
    if (y > 1.3)
      return blend("chest", `upperArm${side}`, (Math.abs(x) - 0.155) / 0.09);
    if (y > 1.065)
      return blend(`forearm${side}`, `upperArm${side}`, (y - 1.065) / 0.09);
    return blend(`hand${side}`, `forearm${side}`, (y - 0.81) / 0.075);
  }
  if (y < 0.975) {
    if (y > 0.83) return blend(`thigh${side}`, "pelvis", (y - 0.83) / 0.145);
    if (y > 0.38)
      return blend(`calf${side}`, `thigh${side}`, (y - 0.38) / 0.12);
    return blend(`foot${side}`, `calf${side}`, (y - 0.055) / 0.08);
  }
  if (y > 1.495) return blend("neck", "head", (y - 1.495) / 0.05);
  if (y > 1.415) return blend("chest", "neck", (y - 1.415) / 0.08);
  if (y > 1.18) return blend("spine", "chest", (y - 1.18) / 0.1);
  return blend("pelvis", "spine", (y - 0.99) / 0.14);
}

export const SKIN_COLOR = "#f4f1e8";
export const SKIN_OPACITY = 0.22;
export const SKIN_SELECTED_OPACITY = 0.58;

/** Bakes an atlas mesh into rig space with the deterministic reference skinning. */
export function bakeSkinnedGeometry(object: Mesh): BufferGeometry {
  const mirrored = object.matrixWorld.determinant() < 0;
  const geometry = object.geometry.clone().applyMatrix4(object.matrixWorld);
  if (mirrored) {
    // The left-side Vanatome meshes reuse the right-side meshes under a
    // negative X scale. applyMatrix4 already reflects their normals through
    // the normal matrix, so they stay outward; only the triangle winding
    // flips. Restore the winding and leave the normals alone: inverting them
    // as well lit the whole left half from inside (dark, split down the
    // midline).
    restoreMirroredTriangleWinding(geometry);
  }
  for (const key of Object.keys(geometry.attributes))
    if (key !== "position" && key !== "normal") geometry.deleteAttribute(key);
  if (!geometry.getAttribute("normal")) geometry.computeVertexNormals();
  const position = geometry.getAttribute("position");
  const indices = new Uint16Array(position.count * 4);
  const weights = new Float32Array(position.count * 4);
  for (let i = 0; i < position.count; i++) {
    const [a, b, weight] = skinInfluence(position.getX(i), position.getY(i));
    indices[i * 4] = a;
    indices[i * 4 + 1] = b;
    weights[i * 4] = 1 - weight;
    weights[i * 4 + 1] = weight;
  }
  geometry.setAttribute("skinIndex", new Uint16BufferAttribute(indices, 4));
  geometry.setAttribute("skinWeight", new Float32BufferAttribute(weights, 4));
  return geometry;
}
